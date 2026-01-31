// Package redis provides Redis client utilities for the Txova platform.
package redis

import (
	"context"
	"encoding/json"
	"log/slog"
	"time"
)

// Default cache settings.
const (
	// DefaultCacheTTL is the default TTL for cached items (15 minutes).
	DefaultCacheTTL = 15 * time.Minute
)

// Cache provides caching operations on top of the Redis client.
type Cache struct {
	client     *Client
	logger     *slog.Logger
	defaultTTL time.Duration
	keyPrefix  string
}

// CacheOption is a functional option for configuring the Cache.
type CacheOption func(*Cache)

// WithDefaultTTL sets the default TTL for cache entries.
func WithDefaultTTL(ttl time.Duration) CacheOption {
	return func(c *Cache) {
		c.defaultTTL = ttl
	}
}

// WithKeyPrefix sets a prefix for all cache keys.
func WithKeyPrefix(prefix string) CacheOption {
	return func(c *Cache) {
		c.keyPrefix = prefix
	}
}

// WithCacheLogger sets the logger for the cache.
func WithCacheLogger(logger *slog.Logger) CacheOption {
	return func(c *Cache) {
		c.logger = logger
	}
}

// NewCache creates a new Cache instance.
func NewCache(client *Client, opts ...CacheOption) *Cache {
	cache := &Cache{
		client:     client,
		logger:     slog.Default(),
		defaultTTL: DefaultCacheTTL,
	}

	for _, opt := range opts {
		opt(cache)
	}

	return cache
}

// prefixKey adds the configured prefix to the key.
func (c *Cache) prefixKey(key string) string {
	if c.keyPrefix == "" {
		return key
	}
	return c.keyPrefix + ":" + key
}

// Get retrieves a value from the cache.
// Returns (nil, nil) for cache miss (key not found).
// Returns (nil, error) for actual errors.
// Returns (value, nil) for cache hit.
func (c *Cache) Get(ctx context.Context, key string) ([]byte, error) {
	prefixedKey := c.prefixKey(key)
	result, err := c.client.client.Get(ctx, prefixedKey).Bytes()
	if err != nil {
		redisErr := FromRedisError(err)
		if IsNotFound(redisErr) {
			c.logger.Debug("cache miss", "key", prefixedKey)
			return nil, nil
		}
		c.logger.Error("cache get error", "key", prefixedKey, "error", err)
		return nil, redisErr
	}

	c.logger.Debug("cache hit", "key", prefixedKey)
	return result, nil
}

// GetJSON retrieves a JSON-encoded value from the cache and unmarshals it.
// Returns false for cache miss, true for cache hit.
func (c *Cache) GetJSON(ctx context.Context, key string, dest any) (bool, error) {
	data, err := c.Get(ctx, key)
	if err != nil {
		return false, err
	}
	if data == nil {
		return false, nil
	}

	if err := json.Unmarshal(data, dest); err != nil {
		return false, SerializationWrap("failed to unmarshal cached value", err)
	}

	return true, nil
}

// Set stores a value in the cache with the default TTL.
func (c *Cache) Set(ctx context.Context, key string, value []byte) error {
	return c.SetWithTTL(ctx, key, value, c.defaultTTL)
}

// SetWithTTL stores a value in the cache with a custom TTL.
// If ttl is 0, the value is stored without expiration.
func (c *Cache) SetWithTTL(ctx context.Context, key string, value []byte, ttl time.Duration) error {
	prefixedKey := c.prefixKey(key)
	err := c.client.client.Set(ctx, prefixedKey, value, ttl).Err()
	if err != nil {
		c.logger.Error("cache set error", "key", prefixedKey, "error", err)
		return FromRedisError(err)
	}

	c.logger.Debug("cache set", "key", prefixedKey, "ttl", ttl)
	return nil
}

// SetJSON stores a JSON-encoded value in the cache with the default TTL.
func (c *Cache) SetJSON(ctx context.Context, key string, value any) error {
	return c.SetJSONWithTTL(ctx, key, value, c.defaultTTL)
}

// SetJSONWithTTL stores a JSON-encoded value in the cache with a custom TTL.
func (c *Cache) SetJSONWithTTL(ctx context.Context, key string, value any, ttl time.Duration) error {
	data, err := json.Marshal(value)
	if err != nil {
		return SerializationWrap("failed to marshal value for cache", err)
	}

	return c.SetWithTTL(ctx, key, data, ttl)
}

// Delete removes a value from the cache.
func (c *Cache) Delete(ctx context.Context, keys ...string) error {
	if len(keys) == 0 {
		return nil
	}

	prefixedKeys := make([]string, len(keys))
	for i, key := range keys {
		prefixedKeys[i] = c.prefixKey(key)
	}

	err := c.client.client.Del(ctx, prefixedKeys...).Err()
	if err != nil {
		c.logger.Error("cache delete error", "keys", prefixedKeys, "error", err)
		return FromRedisError(err)
	}

	c.logger.Debug("cache delete", "keys", prefixedKeys)
	return nil
}

// Exists checks if a key exists in the cache.
func (c *Cache) Exists(ctx context.Context, key string) (bool, error) {
	prefixedKey := c.prefixKey(key)
	count, err := c.client.client.Exists(ctx, prefixedKey).Result()
	if err != nil {
		return false, FromRedisError(err)
	}
	return count > 0, nil
}

// GetOrSet retrieves a value from the cache, or computes and stores it if not found.
// The compute function is only called on cache miss.
func (c *Cache) GetOrSet(ctx context.Context, key string, compute func(ctx context.Context) ([]byte, error)) ([]byte, error) {
	return c.GetOrSetWithTTL(ctx, key, c.defaultTTL, compute)
}

// GetOrSetWithTTL retrieves a value from the cache, or computes and stores it if not found.
func (c *Cache) GetOrSetWithTTL(ctx context.Context, key string, ttl time.Duration, compute func(ctx context.Context) ([]byte, error)) ([]byte, error) {
	// Try to get from cache first
	value, err := c.Get(ctx, key)
	if err != nil {
		return nil, err
	}
	if value != nil {
		return value, nil
	}

	// Cache miss - compute the value
	value, err = compute(ctx)
	if err != nil {
		return nil, err
	}

	// Store in cache (don't fail if caching fails)
	if setErr := c.SetWithTTL(ctx, key, value, ttl); setErr != nil {
		c.logger.Warn("failed to cache computed value", "key", key, "error", setErr)
	}

	return value, nil
}

// GetOrSetJSON retrieves a JSON value from the cache, or computes and stores it if not found.
func (c *Cache) GetOrSetJSON(ctx context.Context, key string, dest any, compute func(ctx context.Context) (any, error)) error {
	return c.GetOrSetJSONWithTTL(ctx, key, c.defaultTTL, dest, compute)
}

// GetOrSetJSONWithTTL retrieves a JSON value from the cache, or computes and stores it if not found.
func (c *Cache) GetOrSetJSONWithTTL(ctx context.Context, key string, ttl time.Duration, dest any, compute func(ctx context.Context) (any, error)) error {
	// Try to get from cache first
	found, err := c.GetJSON(ctx, key, dest)
	if err != nil {
		return err
	}
	if found {
		return nil
	}

	// Cache miss - compute the value
	value, err := compute(ctx)
	if err != nil {
		return err
	}

	// Store in cache (don't fail if caching fails)
	if setErr := c.SetJSONWithTTL(ctx, key, value, ttl); setErr != nil {
		c.logger.Warn("failed to cache computed value", "key", key, "error", setErr)
	}

	// Marshal and unmarshal to populate dest
	data, err := json.Marshal(value)
	if err != nil {
		return SerializationWrap("failed to marshal computed value", err)
	}
	if err := json.Unmarshal(data, dest); err != nil {
		return SerializationWrap("failed to unmarshal computed value", err)
	}

	return nil
}

// MGet retrieves multiple values from the cache.
// Returns a map of key to value, with nil values for cache misses.
func (c *Cache) MGet(ctx context.Context, keys ...string) (map[string][]byte, error) {
	if len(keys) == 0 {
		return make(map[string][]byte), nil
	}

	prefixedKeys := make([]string, len(keys))
	for i, key := range keys {
		prefixedKeys[i] = c.prefixKey(key)
	}

	results, err := c.client.client.MGet(ctx, prefixedKeys...).Result()
	if err != nil {
		return nil, FromRedisError(err)
	}

	values := make(map[string][]byte, len(keys))
	hits := 0
	for i, key := range keys {
		if results[i] != nil {
			if str, ok := results[i].(string); ok {
				values[key] = []byte(str)
				hits++
			} else {
				values[key] = nil
			}
		} else {
			values[key] = nil
		}
	}

	c.logger.Debug("cache mget", "keys", len(keys), "hits", hits, "misses", len(keys)-hits)
	return values, nil
}

// MSet stores multiple values in the cache with the default TTL.
func (c *Cache) MSet(ctx context.Context, values map[string][]byte) error {
	return c.MSetWithTTL(ctx, values, c.defaultTTL)
}

// MSetWithTTL stores multiple values in the cache with a custom TTL.
func (c *Cache) MSetWithTTL(ctx context.Context, values map[string][]byte, ttl time.Duration) error {
	if len(values) == 0 {
		return nil
	}

	// Use pipeline for atomic-like operations with TTL
	pipe := c.client.client.Pipeline()

	for key, value := range values {
		prefixedKey := c.prefixKey(key)
		pipe.Set(ctx, prefixedKey, value, ttl)
	}

	_, err := pipe.Exec(ctx)
	if err != nil {
		c.logger.Error("cache mset error", "count", len(values), "error", err)
		return FromRedisError(err)
	}

	c.logger.Debug("cache mset", "count", len(values), "ttl", ttl)
	return nil
}

// DeleteByPattern deletes all keys matching the given pattern.
// Use with caution as SCAN can be slow on large datasets.
func (c *Cache) DeleteByPattern(ctx context.Context, pattern string) (int64, error) {
	prefixedPattern := c.prefixKey(pattern)
	var deleted int64

	iter := c.client.client.Scan(ctx, 0, prefixedPattern, 100).Iterator()
	var keysToDelete []string

	for iter.Next(ctx) {
		keysToDelete = append(keysToDelete, iter.Val())

		// Delete in batches of 100
		if len(keysToDelete) >= 100 {
			count, err := c.client.client.Del(ctx, keysToDelete...).Result()
			if err != nil {
				return deleted, FromRedisError(err)
			}
			deleted += count
			keysToDelete = keysToDelete[:0]
		}
	}

	if err := iter.Err(); err != nil {
		return deleted, FromRedisError(err)
	}

	// Delete remaining keys
	if len(keysToDelete) > 0 {
		count, err := c.client.client.Del(ctx, keysToDelete...).Result()
		if err != nil {
			return deleted, FromRedisError(err)
		}
		deleted += count
	}

	c.logger.Debug("cache delete by pattern", "pattern", prefixedPattern, "deleted", deleted)
	return deleted, nil
}

// TTL returns the remaining TTL for a key.
// Returns -2 if key does not exist, -1 if key has no expiration.
func (c *Cache) TTL(ctx context.Context, key string) (time.Duration, error) {
	prefixedKey := c.prefixKey(key)
	ttl, err := c.client.client.TTL(ctx, prefixedKey).Result()
	if err != nil {
		return 0, FromRedisError(err)
	}
	return ttl, nil
}

// Expire sets a new TTL on an existing key.
// Returns false if the key does not exist.
func (c *Cache) Expire(ctx context.Context, key string, ttl time.Duration) (bool, error) {
	prefixedKey := c.prefixKey(key)
	ok, err := c.client.client.Expire(ctx, prefixedKey, ttl).Result()
	if err != nil {
		return false, FromRedisError(err)
	}
	return ok, nil
}

// KeyBuilder provides utilities for building cache keys.
type KeyBuilder struct {
	service string
}

// NewKeyBuilder creates a new KeyBuilder for a service.
func NewKeyBuilder(service string) *KeyBuilder {
	return &KeyBuilder{service: service}
}

// Key builds a cache key in the format: {service}:{entity}:{id}.
func (b *KeyBuilder) Key(entity, id string) string {
	return b.service + ":" + entity + ":" + id
}

// KeyWithParts builds a cache key with multiple parts.
func (b *KeyBuilder) KeyWithParts(parts ...string) string {
	if len(parts) == 0 {
		return b.service
	}

	result := b.service
	for _, part := range parts {
		result += ":" + part
	}
	return result
}

// Pattern builds a cache key pattern for use with DeleteByPattern.
func (b *KeyBuilder) Pattern(entity string) string {
	return b.service + ":" + entity + ":*"
}
