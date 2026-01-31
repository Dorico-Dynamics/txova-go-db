// Package redis provides Redis client utilities for the Txova platform.
package redis

import (
	"context"
	"errors"
	"log/slog"
	"strconv"
	"time"

	"github.com/redis/go-redis/v9"
)

// Default rate limiting settings.
const (
	// DefaultRateLimitWindow is the default window duration.
	DefaultRateLimitWindow = time.Minute
	// DefaultRateLimitMax is the default maximum requests per window.
	DefaultRateLimitMax = 100
)

// RateLimitResult contains the result of a rate limit check.
type RateLimitResult struct {
	// Allowed indicates whether the request is allowed.
	Allowed bool
	// Remaining is the number of requests remaining in the current window.
	Remaining int64
	// ResetAt is when the current window resets.
	ResetAt time.Time
	// Total is the maximum number of requests allowed per window.
	Total int64
}

// RateLimiter provides rate limiting operations.
type RateLimiter struct {
	client    *Client
	logger    *slog.Logger
	keyPrefix string
	window    time.Duration
	maxReqs   int64
	burst     int64
}

// RateLimiterOption is a functional option for configuring the RateLimiter.
type RateLimiterOption func(*RateLimiter)

// WithRateLimitKeyPrefix sets a prefix for all rate limit keys.
func WithRateLimitKeyPrefix(prefix string) RateLimiterOption {
	return func(r *RateLimiter) {
		r.keyPrefix = prefix
	}
}

// WithRateLimitWindow sets the time window for rate limiting.
func WithRateLimitWindow(window time.Duration) RateLimiterOption {
	return func(r *RateLimiter) {
		r.window = window
	}
}

// WithRateLimitMax sets the maximum requests per window.
func WithRateLimitMax(maxReqs int64) RateLimiterOption {
	return func(r *RateLimiter) {
		r.maxReqs = maxReqs
	}
}

// WithRateLimitBurst sets the burst allowance (extra requests allowed).
func WithRateLimitBurst(burst int64) RateLimiterOption {
	return func(r *RateLimiter) {
		r.burst = burst
	}
}

// WithRateLimiterLogger sets the logger for the rate limiter.
func WithRateLimiterLogger(logger *slog.Logger) RateLimiterOption {
	return func(r *RateLimiter) {
		r.logger = logger
	}
}

// NewRateLimiter creates a new RateLimiter instance.
func NewRateLimiter(client *Client, opts ...RateLimiterOption) *RateLimiter {
	rl := &RateLimiter{
		client:    client,
		logger:    slog.Default(),
		keyPrefix: "ratelimit",
		window:    DefaultRateLimitWindow,
		maxReqs:   DefaultRateLimitMax,
		burst:     0,
	}

	for _, opt := range opts {
		opt(rl)
	}

	return rl
}

// rateLimitKey builds the full rate limit key.
func (r *RateLimiter) rateLimitKey(identifier string) string {
	return r.keyPrefix + ":" + identifier
}

// Allow checks if a request is allowed for the given identifier (user ID, IP, etc.)
// using a fixed window rate limiting algorithm.
func (r *RateLimiter) Allow(ctx context.Context, identifier string) (*RateLimitResult, error) {
	return r.AllowN(ctx, identifier, 1)
}

// AllowN checks if N requests are allowed for the given identifier.
func (r *RateLimiter) AllowN(ctx context.Context, identifier string, n int64) (*RateLimitResult, error) {
	key := r.rateLimitKey(identifier)
	maxAllowed := r.maxReqs + r.burst

	// Use a Lua script for atomic increment and expiry check
	script := redis.NewScript(`
		local key = KEYS[1]
		local max = tonumber(ARGV[1])
		local window = tonumber(ARGV[2])
		local n = tonumber(ARGV[3])

		local current = redis.call("GET", key)
		local is_new_window = (current == false)
		if is_new_window then
			current = 0
		else
			current = tonumber(current)
		end

		local allowed = 0
		local remaining = max - current

		if current + n <= max then
			redis.call("INCRBY", key, n)
			-- Only set TTL when creating a new window to keep fixed window behavior
			if is_new_window then
				redis.call("PEXPIRE", key, window)
			end
			allowed = 1
			remaining = max - current - n
		end

		local ttl = redis.call("PTTL", key)
		if ttl < 0 then
			ttl = window
		end

		return {allowed, remaining, ttl, max}
	`)

	windowMs := r.window.Milliseconds()
	result, err := script.Run(ctx, r.client.client, []string{key}, maxAllowed, windowMs, n).Slice()
	if err != nil {
		r.logger.Error("rate limit check error", "key", key, "error", err)
		return nil, FromRedisError(err)
	}

	allowedVal, _ := result[0].(int64) //nolint:errcheck // Lua script always returns int64
	remaining, _ := result[1].(int64)  //nolint:errcheck // Lua script always returns int64
	ttlMs, _ := result[2].(int64)      //nolint:errcheck // Lua script always returns int64
	total, _ := result[3].(int64)      //nolint:errcheck // Lua script always returns int64

	resetAt := time.Now().Add(time.Duration(ttlMs) * time.Millisecond)

	rlResult := &RateLimitResult{
		Allowed:   allowedVal == 1,
		Remaining: remaining,
		ResetAt:   resetAt,
		Total:     total,
	}

	if !rlResult.Allowed {
		r.logger.Debug("rate limit exceeded", "key", key, "remaining", remaining)
	}

	return rlResult, nil
}

// SlidingWindowAllow checks if a request is allowed using a sliding window algorithm.
// This provides smoother rate limiting than fixed windows.
func (r *RateLimiter) SlidingWindowAllow(ctx context.Context, identifier string) (*RateLimitResult, error) {
	return r.SlidingWindowAllowN(ctx, identifier, 1)
}

// SlidingWindowAllowN checks if N requests are allowed using a sliding window algorithm.
func (r *RateLimiter) SlidingWindowAllowN(ctx context.Context, identifier string, n int64) (*RateLimitResult, error) {
	key := r.rateLimitKey(identifier + ":sliding")
	now := time.Now()
	maxAllowed := r.maxReqs + r.burst

	// Use sorted set with timestamps for sliding window
	script := redis.NewScript(`
		local key = KEYS[1]
		local max = tonumber(ARGV[1])
		local window_ms = tonumber(ARGV[2])
		local now_ms = tonumber(ARGV[3])
		local n = tonumber(ARGV[4])

		-- Remove expired entries
		local min_time = now_ms - window_ms
		redis.call("ZREMRANGEBYSCORE", key, 0, min_time)

		-- Count current entries
		local current = redis.call("ZCARD", key)

		local allowed = 0
		local remaining = max - current

		if current + n <= max then
			-- Add new entries with current timestamp
			for i = 1, n do
				redis.call("ZADD", key, now_ms, now_ms .. ":" .. i .. ":" .. math.random(1000000))
			end
			redis.call("PEXPIRE", key, window_ms)
			allowed = 1
			remaining = max - current - n
		end

		-- Calculate reset time (when oldest entry expires)
		local oldest = redis.call("ZRANGE", key, 0, 0, "WITHSCORES")
		local reset_ms = window_ms
		if oldest[2] then
			reset_ms = tonumber(oldest[2]) + window_ms - now_ms
			if reset_ms < 0 then
				reset_ms = 0
			end
		end

		return {allowed, remaining, reset_ms, max}
	`)

	windowMs := r.window.Milliseconds()
	nowMs := now.UnixMilli()

	result, err := script.Run(ctx, r.client.client, []string{key}, maxAllowed, windowMs, nowMs, n).Slice()
	if err != nil {
		r.logger.Error("sliding window rate limit error", "key", key, "error", err)
		return nil, FromRedisError(err)
	}

	allowed := result[0].(int64) == 1 //nolint:errcheck // Lua script always returns int64
	remaining := result[1].(int64)    //nolint:errcheck // Lua script always returns int64
	resetMs := result[2].(int64)      //nolint:errcheck // Lua script always returns int64
	total := result[3].(int64)        //nolint:errcheck // Lua script always returns int64

	resetAt := now.Add(time.Duration(resetMs) * time.Millisecond)

	rlResult := &RateLimitResult{
		Allowed:   allowed,
		Remaining: remaining,
		ResetAt:   resetAt,
		Total:     total,
	}

	if !allowed {
		r.logger.Debug("sliding window rate limit exceeded", "key", key, "remaining", remaining)
	}

	return rlResult, nil
}

// Reset resets the rate limit for the given identifier.
func (r *RateLimiter) Reset(ctx context.Context, identifier string) error {
	key := r.rateLimitKey(identifier)
	slidingKey := r.rateLimitKey(identifier + ":sliding")

	pipe := r.client.client.Pipeline()
	pipe.Del(ctx, key)
	pipe.Del(ctx, slidingKey)
	_, err := pipe.Exec(ctx)
	if err != nil {
		return FromRedisError(err)
	}

	r.logger.Debug("rate limit reset", "identifier", identifier)
	return nil
}

// GetStatus returns the current rate limit status without incrementing.
func (r *RateLimiter) GetStatus(ctx context.Context, identifier string) (*RateLimitResult, error) {
	key := r.rateLimitKey(identifier)
	maxAllowed := r.maxReqs + r.burst

	pipe := r.client.client.Pipeline()
	getCmd := pipe.Get(ctx, key)
	ttlCmd := pipe.PTTL(ctx, key)
	_, err := pipe.Exec(ctx)

	// Handle case where key doesn't exist
	current := int64(0)
	ttlMs := r.window.Milliseconds()

	if err == nil || errors.Is(err, redis.Nil) {
		if val, getErr := getCmd.Result(); getErr == nil {
			parsed, parseErr := strconv.ParseInt(val, 10, 64)
			if parseErr == nil {
				current = parsed
			}
		}
		if ttl, ttlErr := ttlCmd.Result(); ttlErr == nil && ttl > 0 {
			ttlMs = ttl.Milliseconds()
		}
	} else {
		return nil, FromRedisError(err)
	}

	remaining := maxAllowed - current
	if remaining < 0 {
		remaining = 0
	}

	return &RateLimitResult{
		Allowed:   remaining > 0,
		Remaining: remaining,
		ResetAt:   time.Now().Add(time.Duration(ttlMs) * time.Millisecond),
		Total:     maxAllowed,
	}, nil
}

// UserRateLimiter creates a rate limiter for user-based limits.
func UserRateLimiter(client *Client, maxReqs int64, window time.Duration, opts ...RateLimiterOption) *RateLimiter {
	defaultOpts := []RateLimiterOption{
		WithRateLimitKeyPrefix("ratelimit:user"),
		WithRateLimitMax(maxReqs),
		WithRateLimitWindow(window),
	}
	return NewRateLimiter(client, append(defaultOpts, opts...)...)
}

// IPRateLimiter creates a rate limiter for IP-based limits.
func IPRateLimiter(client *Client, maxReqs int64, window time.Duration, opts ...RateLimiterOption) *RateLimiter {
	defaultOpts := []RateLimiterOption{
		WithRateLimitKeyPrefix("ratelimit:ip"),
		WithRateLimitMax(maxReqs),
		WithRateLimitWindow(window),
	}
	return NewRateLimiter(client, append(defaultOpts, opts...)...)
}
