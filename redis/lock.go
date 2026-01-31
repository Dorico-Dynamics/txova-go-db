// Package redis provides Redis client utilities for the Txova platform.
package redis

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"log/slog"
	"time"

	"github.com/redis/go-redis/v9"
)

// Default lock settings.
const (
	// DefaultLockTTL is the default TTL for locks (30 seconds).
	DefaultLockTTL = 30 * time.Second
	// DefaultLockRetryDelay is the default delay between lock acquisition attempts.
	DefaultLockRetryDelay = 50 * time.Millisecond
	// DefaultLockRetryCount is the default number of lock acquisition attempts.
	DefaultLockRetryCount = 100
)

// Lock represents a distributed lock.
type Lock struct {
	client    *Client
	key       string
	owner     string
	ttl       time.Duration
	logger    *slog.Logger
	acquired  bool
	expiresAt time.Time
}

// Locker provides distributed locking operations.
type Locker struct {
	client     *Client
	logger     *slog.Logger
	keyPrefix  string
	defaultTTL time.Duration
	retryDelay time.Duration
	retryCount int
}

// LockerOption is a functional option for configuring the Locker.
type LockerOption func(*Locker)

// WithLockKeyPrefix sets a prefix for all lock keys.
func WithLockKeyPrefix(prefix string) LockerOption {
	return func(l *Locker) {
		l.keyPrefix = prefix
	}
}

// WithDefaultLockTTL sets the default TTL for locks.
func WithDefaultLockTTL(ttl time.Duration) LockerOption {
	return func(l *Locker) {
		l.defaultTTL = ttl
	}
}

// WithLockRetryDelay sets the delay between lock acquisition attempts.
func WithLockRetryDelay(delay time.Duration) LockerOption {
	return func(l *Locker) {
		l.retryDelay = delay
	}
}

// WithLockRetryCount sets the number of lock acquisition attempts.
func WithLockRetryCount(count int) LockerOption {
	return func(l *Locker) {
		l.retryCount = count
	}
}

// WithLockerLogger sets the logger for the locker.
func WithLockerLogger(logger *slog.Logger) LockerOption {
	return func(l *Locker) {
		l.logger = logger
	}
}

// NewLocker creates a new Locker instance.
func NewLocker(client *Client, opts ...LockerOption) *Locker {
	locker := &Locker{
		client:     client,
		logger:     slog.Default(),
		keyPrefix:  "lock",
		defaultTTL: DefaultLockTTL,
		retryDelay: DefaultLockRetryDelay,
		retryCount: DefaultLockRetryCount,
	}

	for _, opt := range opts {
		opt(locker)
	}

	return locker
}

// lockKey builds the full lock key.
func (l *Locker) lockKey(resource string) string {
	return l.keyPrefix + ":" + resource
}

// generateOwner generates a unique owner identifier.
func generateOwner() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		// Fallback to timestamp-based ID if crypto/rand fails
		return hex.EncodeToString([]byte(time.Now().String()))
	}
	return hex.EncodeToString(b)
}

// Acquire attempts to acquire a lock on the given resource.
// Returns a Lock if successful, or an error if the lock could not be acquired.
func (l *Locker) Acquire(ctx context.Context, resource string) (*Lock, error) {
	return l.AcquireWithTTL(ctx, resource, l.defaultTTL)
}

// AcquireWithTTL attempts to acquire a lock with a custom TTL.
func (l *Locker) AcquireWithTTL(ctx context.Context, resource string, ttl time.Duration) (*Lock, error) {
	key := l.lockKey(resource)
	owner := generateOwner()

	ok, err := l.client.client.SetNX(ctx, key, owner, ttl).Result()
	if err != nil {
		l.logger.Error("lock acquire error", "key", key, "error", err)
		return nil, FromRedisError(err)
	}

	if !ok {
		l.logger.Debug("lock acquisition failed", "key", key, "reason", "already held")
		return nil, LockFailed("lock is already held by another owner")
	}

	lock := &Lock{
		client:    l.client,
		key:       key,
		owner:     owner,
		ttl:       ttl,
		logger:    l.logger,
		acquired:  true,
		expiresAt: time.Now().Add(ttl),
	}

	l.logger.Debug("lock acquired", "key", key, "ttl", ttl)
	return lock, nil
}

// AcquireWithRetry attempts to acquire a lock, retrying if it's already held.
// It will retry up to the configured retry count with the configured delay between attempts.
func (l *Locker) AcquireWithRetry(ctx context.Context, resource string) (*Lock, error) {
	return l.AcquireWithRetryAndTTL(ctx, resource, l.defaultTTL)
}

// AcquireWithRetryAndTTL attempts to acquire a lock with retry and custom TTL.
func (l *Locker) AcquireWithRetryAndTTL(ctx context.Context, resource string, ttl time.Duration) (*Lock, error) {
	var lastErr error

	for i := range l.retryCount {
		lock, err := l.AcquireWithTTL(ctx, resource, ttl)
		if err == nil {
			return lock, nil
		}

		lastErr = err

		// Only retry on lock contention, not on connection errors
		if !IsLockFailed(err) {
			return nil, err
		}

		// Check context before sleeping
		select {
		case <-ctx.Done():
			return nil, Wrap(CodeTimeout, "lock acquisition cancelled", ctx.Err())
		default:
		}

		l.logger.Debug("lock retry", "key", l.lockKey(resource), "attempt", i+1, "max", l.retryCount)
		time.Sleep(l.retryDelay)
	}

	return nil, Wrapf(CodeLockFailed, lastErr, "failed to acquire lock after %d attempts", l.retryCount)
}

// TryAcquire attempts to acquire a lock without blocking.
// Returns (nil, nil) if the lock is already held.
func (l *Locker) TryAcquire(ctx context.Context, resource string) (*Lock, error) {
	return l.TryAcquireWithTTL(ctx, resource, l.defaultTTL)
}

// TryAcquireWithTTL attempts to acquire a lock without blocking, with custom TTL.
// Returns (nil, nil) if lock is already held (not an error condition for try-acquire).
//
//nolint:nilnil // Intentional: (nil, nil) means "lock not acquired, no error" for try semantics.
func (l *Locker) TryAcquireWithTTL(ctx context.Context, resource string, ttl time.Duration) (*Lock, error) {
	lock, err := l.AcquireWithTTL(ctx, resource, ttl)
	if err != nil {
		if IsLockFailed(err) {
			return nil, nil
		}
		return nil, err
	}
	return lock, nil
}

// WithLock executes a function while holding a lock.
// The lock is automatically released after the function completes.
func (l *Locker) WithLock(ctx context.Context, resource string, fn func(ctx context.Context) error) error {
	return l.WithLockAndTTL(ctx, resource, l.defaultTTL, fn)
}

// WithLockAndTTL executes a function while holding a lock with custom TTL.
func (l *Locker) WithLockAndTTL(ctx context.Context, resource string, ttl time.Duration, fn func(ctx context.Context) error) error {
	lock, err := l.AcquireWithRetryAndTTL(ctx, resource, ttl)
	if err != nil {
		return err
	}
	defer func() {
		if releaseErr := lock.Release(ctx); releaseErr != nil {
			l.logger.Warn("failed to release lock", "key", lock.key, "error", releaseErr)
		}
	}()

	return fn(ctx)
}

// Release releases the lock.
// Returns an error if the lock is not held or if the release fails.
func (lock *Lock) Release(ctx context.Context) error {
	if !lock.acquired {
		return LockNotHeld("lock was never acquired")
	}

	// Use Lua script for atomic ownership check and delete
	script := redis.NewScript(`
		if redis.call("get", KEYS[1]) == ARGV[1] then
			return redis.call("del", KEYS[1])
		else
			return 0
		end
	`)

	result, err := script.Run(ctx, lock.client.client, []string{lock.key}, lock.owner).Int64()
	if err != nil {
		lock.logger.Error("lock release error", "key", lock.key, "error", err)
		return FromRedisError(err)
	}

	if result == 0 {
		lock.logger.Warn("lock release failed", "key", lock.key, "reason", "not held or owner mismatch")
		return LockNotHeld("lock is not held by this owner")
	}

	lock.acquired = false
	lock.logger.Debug("lock released", "key", lock.key)
	return nil
}

// Extend extends the lock's TTL.
// Returns an error if the lock is not held.
func (lock *Lock) Extend(ctx context.Context, ttl time.Duration) error {
	if !lock.acquired {
		return LockNotHeld("lock was never acquired")
	}

	// Use Lua script for atomic ownership check and expire
	script := redis.NewScript(`
		if redis.call("get", KEYS[1]) == ARGV[1] then
			return redis.call("pexpire", KEYS[1], ARGV[2])
		else
			return 0
		end
	`)

	ttlMs := ttl.Milliseconds()
	result, err := script.Run(ctx, lock.client.client, []string{lock.key}, lock.owner, ttlMs).Int64()
	if err != nil {
		lock.logger.Error("lock extend error", "key", lock.key, "error", err)
		return FromRedisError(err)
	}

	if result == 0 {
		lock.acquired = false
		lock.logger.Warn("lock extend failed", "key", lock.key, "reason", "not held or owner mismatch")
		return LockNotHeld("lock is not held by this owner")
	}

	lock.ttl = ttl
	lock.expiresAt = time.Now().Add(ttl)
	lock.logger.Debug("lock extended", "key", lock.key, "ttl", ttl)
	return nil
}

// TTL returns the remaining TTL of the lock.
func (lock *Lock) TTL(ctx context.Context) (time.Duration, error) {
	ttl, err := lock.client.client.PTTL(ctx, lock.key).Result()
	if err != nil {
		return 0, FromRedisError(err)
	}
	return ttl, nil
}

// Key returns the lock key.
func (lock *Lock) Key() string {
	return lock.key
}

// Owner returns the lock owner identifier.
func (lock *Lock) Owner() string {
	return lock.owner
}

// IsHeld returns whether the lock is currently held by this owner.
func (lock *Lock) IsHeld() bool {
	return lock.acquired
}

// ExpiresAt returns the expected expiration time of the lock.
func (lock *Lock) ExpiresAt() time.Time {
	return lock.expiresAt
}

// IsExpired returns whether the lock has likely expired based on local time.
// Note: This is an approximation and may not reflect the actual server state.
func (lock *Lock) IsExpired() bool {
	return time.Now().After(lock.expiresAt)
}

// Verify checks if the lock is still held by this owner on the server.
func (lock *Lock) Verify(ctx context.Context) (bool, error) {
	if !lock.acquired {
		return false, nil
	}

	owner, err := lock.client.client.Get(ctx, lock.key).Result()
	if err != nil {
		if IsNotFound(FromRedisError(err)) {
			lock.acquired = false
			return false, nil
		}
		return false, FromRedisError(err)
	}

	isHeld := owner == lock.owner
	if !isHeld {
		lock.acquired = false
	}
	return isHeld, nil
}
