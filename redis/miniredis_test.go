package redis

import (
	"context"
	"errors"
	"log/slog"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	goredis "github.com/redis/go-redis/v9"
)

// newTestClient creates a Client connected to a miniredis server for testing.
func newTestClient(t *testing.T) (*Client, *miniredis.Miniredis) {
	t.Helper()
	mr := miniredis.RunT(t)

	client := &Client{
		client: goredis.NewClient(&goredis.Options{
			Addr: mr.Addr(),
		}),
		config: DefaultConfig(),
		logger: slog.Default(),
	}
	client.config.Addresses = []string{mr.Addr()}

	return client, mr
}

// =============================================================================
// Cache Tests with Miniredis
// =============================================================================

func TestCache_SetAndGet(t *testing.T) {
	t.Parallel()
	client, _ := newTestClient(t)
	cache := NewCache(client)

	ctx := context.Background()

	// Test Set
	err := cache.Set(ctx, "test-key", []byte("test-value"))
	if err != nil {
		t.Fatalf("Set() error = %v", err)
	}

	// Test Get
	value, err := cache.Get(ctx, "test-key")
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if string(value) != "test-value" {
		t.Errorf("Get() = %q, want %q", string(value), "test-value")
	}
}

func TestCache_SetWithTTL(t *testing.T) {
	t.Parallel()
	client, mr := newTestClient(t)
	cache := NewCache(client)

	ctx := context.Background()

	err := cache.SetWithTTL(ctx, "ttl-key", []byte("value"), 5*time.Second)
	if err != nil {
		t.Fatalf("SetWithTTL() error = %v", err)
	}

	// Verify key exists
	value, err := cache.Get(ctx, "ttl-key")
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if string(value) != "value" {
		t.Errorf("Get() = %q, want %q", string(value), "value")
	}

	// Fast-forward time
	mr.FastForward(6 * time.Second)

	// Key should be expired
	value, err = cache.Get(ctx, "ttl-key")
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if value != nil {
		t.Errorf("Get() after expiration = %q, want nil", string(value))
	}
}

func TestCache_GetMiss(t *testing.T) {
	t.Parallel()
	client, _ := newTestClient(t)
	cache := NewCache(client)

	ctx := context.Background()

	value, err := cache.Get(ctx, "nonexistent-key")
	if err != nil {
		t.Fatalf("Get() error = %v, want nil for cache miss", err)
	}
	if value != nil {
		t.Errorf("Get() = %v, want nil for cache miss", value)
	}
}

func TestCache_Exists(t *testing.T) {
	t.Parallel()
	client, _ := newTestClient(t)
	cache := NewCache(client)

	ctx := context.Background()

	// Key doesn't exist
	exists, err := cache.Exists(ctx, "test-key")
	if err != nil {
		t.Fatalf("Exists() error = %v", err)
	}
	if exists {
		t.Error("Exists() = true, want false for nonexistent key")
	}

	// Set the key
	_ = cache.Set(ctx, "test-key", []byte("value"))

	// Key exists
	exists, err = cache.Exists(ctx, "test-key")
	if err != nil {
		t.Fatalf("Exists() error = %v", err)
	}
	if !exists {
		t.Error("Exists() = false, want true for existing key")
	}
}

func TestCache_Delete(t *testing.T) {
	t.Parallel()
	client, _ := newTestClient(t)
	cache := NewCache(client)

	ctx := context.Background()

	// Set keys
	_ = cache.Set(ctx, "key1", []byte("value1"))
	_ = cache.Set(ctx, "key2", []byte("value2"))

	// Delete keys
	err := cache.Delete(ctx, "key1", "key2")
	if err != nil {
		t.Fatalf("Delete() error = %v", err)
	}

	// Verify deletion
	value, _ := cache.Get(ctx, "key1")
	if value != nil {
		t.Error("key1 should be deleted")
	}
	value, _ = cache.Get(ctx, "key2")
	if value != nil {
		t.Error("key2 should be deleted")
	}
}

func TestCache_TTL(t *testing.T) {
	t.Parallel()
	client, _ := newTestClient(t)
	cache := NewCache(client)

	ctx := context.Background()

	// Set with TTL
	_ = cache.SetWithTTL(ctx, "ttl-key", []byte("value"), time.Minute)

	ttl, err := cache.TTL(ctx, "ttl-key")
	if err != nil {
		t.Fatalf("TTL() error = %v", err)
	}
	if ttl <= 0 || ttl > time.Minute {
		t.Errorf("TTL() = %v, want > 0 and <= 1m", ttl)
	}

	// TTL for nonexistent key
	ttl, err = cache.TTL(ctx, "nonexistent")
	if err != nil {
		t.Fatalf("TTL() error = %v", err)
	}
	// Redis returns -2 for nonexistent keys (as duration: -2ns or -2ms depending on implementation)
	if ttl >= 0 {
		t.Errorf("TTL() for nonexistent = %v, want negative value", ttl)
	}
}

func TestCache_Expire(t *testing.T) {
	t.Parallel()
	client, _ := newTestClient(t)
	cache := NewCache(client)

	ctx := context.Background()

	// Set key without TTL
	_ = cache.SetWithTTL(ctx, "expire-key", []byte("value"), 0)

	// Set expiration
	ok, err := cache.Expire(ctx, "expire-key", time.Minute)
	if err != nil {
		t.Fatalf("Expire() error = %v", err)
	}
	if !ok {
		t.Error("Expire() = false, want true")
	}

	// Expire on nonexistent key
	ok, err = cache.Expire(ctx, "nonexistent", time.Minute)
	if err != nil {
		t.Fatalf("Expire() error = %v", err)
	}
	if ok {
		t.Error("Expire() = true for nonexistent key, want false")
	}
}

func TestCache_MGet(t *testing.T) {
	t.Parallel()
	client, _ := newTestClient(t)
	cache := NewCache(client)

	ctx := context.Background()

	// Set keys
	_ = cache.Set(ctx, "mget1", []byte("value1"))
	_ = cache.Set(ctx, "mget2", []byte("value2"))

	// MGet
	values, err := cache.MGet(ctx, "mget1", "mget2", "mget3")
	if err != nil {
		t.Fatalf("MGet() error = %v", err)
	}
	if string(values["mget1"]) != "value1" {
		t.Errorf("MGet()[mget1] = %q, want %q", string(values["mget1"]), "value1")
	}
	if string(values["mget2"]) != "value2" {
		t.Errorf("MGet()[mget2] = %q, want %q", string(values["mget2"]), "value2")
	}
	if values["mget3"] != nil {
		t.Errorf("MGet()[mget3] = %v, want nil", values["mget3"])
	}
}

func TestCache_MSetWithTTL_Miniredis(t *testing.T) {
	t.Parallel()
	client, _ := newTestClient(t)
	cache := NewCache(client)

	ctx := context.Background()

	values := map[string][]byte{
		"mset1": []byte("value1"),
		"mset2": []byte("value2"),
	}

	err := cache.MSetWithTTL(ctx, values, time.Minute)
	if err != nil {
		t.Fatalf("MSetWithTTL() error = %v", err)
	}

	// Verify
	got, _ := cache.Get(ctx, "mset1")
	if string(got) != "value1" {
		t.Errorf("Get(mset1) = %q, want %q", string(got), "value1")
	}
}

func TestCache_DeleteByPattern(t *testing.T) {
	t.Parallel()
	client, _ := newTestClient(t)
	cache := NewCache(client)

	ctx := context.Background()

	// Set keys with pattern
	_ = cache.Set(ctx, "pattern:key1", []byte("value1"))
	_ = cache.Set(ctx, "pattern:key2", []byte("value2"))
	_ = cache.Set(ctx, "other:key", []byte("other"))

	// Delete by pattern
	deleted, err := cache.DeleteByPattern(ctx, "pattern:*")
	if err != nil {
		t.Fatalf("DeleteByPattern() error = %v", err)
	}
	if deleted != 2 {
		t.Errorf("DeleteByPattern() = %d, want 2", deleted)
	}

	// Verify pattern keys deleted
	value, _ := cache.Get(ctx, "pattern:key1")
	if value != nil {
		t.Error("pattern:key1 should be deleted")
	}

	// Verify other key not deleted
	value, _ = cache.Get(ctx, "other:key")
	if string(value) != "other" {
		t.Error("other:key should not be deleted")
	}
}

func TestCache_GetOrSet(t *testing.T) {
	t.Parallel()
	client, _ := newTestClient(t)
	cache := NewCache(client)

	ctx := context.Background()

	computeCalls := 0
	compute := func(ctx context.Context) ([]byte, error) {
		computeCalls++
		return []byte("computed"), nil
	}

	// First call - cache miss, should compute
	value, err := cache.GetOrSet(ctx, "getorset-key", compute)
	if err != nil {
		t.Fatalf("GetOrSet() error = %v", err)
	}
	if string(value) != "computed" {
		t.Errorf("GetOrSet() = %q, want %q", string(value), "computed")
	}
	if computeCalls != 1 {
		t.Errorf("compute called %d times, want 1", computeCalls)
	}

	// Second call - cache hit, should not compute
	value, err = cache.GetOrSet(ctx, "getorset-key", compute)
	if err != nil {
		t.Fatalf("GetOrSet() error = %v", err)
	}
	if string(value) != "computed" {
		t.Errorf("GetOrSet() = %q, want %q", string(value), "computed")
	}
	if computeCalls != 1 {
		t.Errorf("compute called %d times, want 1 (should be cached)", computeCalls)
	}
}

func TestCache_GetOrSetJSON(t *testing.T) {
	t.Parallel()
	client, _ := newTestClient(t)
	cache := NewCache(client)

	ctx := context.Background()

	type Data struct {
		ID   int    `json:"id"`
		Name string `json:"name"`
	}

	computeCalls := 0
	compute := func(ctx context.Context) (any, error) {
		computeCalls++
		return Data{ID: 42, Name: "test"}, nil
	}

	// First call
	var dest Data
	err := cache.GetOrSetJSON(ctx, "json-key", &dest, compute)
	if err != nil {
		t.Fatalf("GetOrSetJSON() error = %v", err)
	}
	if dest.ID != 42 || dest.Name != "test" {
		t.Errorf("GetOrSetJSON() dest = %+v, want {ID:42, Name:test}", dest)
	}
	if computeCalls != 1 {
		t.Errorf("compute called %d times, want 1", computeCalls)
	}

	// Second call - cache hit
	var dest2 Data
	err = cache.GetOrSetJSON(ctx, "json-key", &dest2, compute)
	if err != nil {
		t.Fatalf("GetOrSetJSON() error = %v", err)
	}
	if dest2.ID != 42 {
		t.Errorf("GetOrSetJSON() dest2.ID = %d, want 42", dest2.ID)
	}
	if computeCalls != 1 {
		t.Errorf("compute called %d times, want 1", computeCalls)
	}
}

// =============================================================================
// Lock Tests with Miniredis
// =============================================================================

func TestLocker_AcquireAndRelease(t *testing.T) {
	t.Parallel()
	client, _ := newTestClient(t)
	locker := NewLocker(client)

	ctx := context.Background()

	// Acquire lock
	lock, err := locker.Acquire(ctx, "test-resource")
	if err != nil {
		t.Fatalf("Acquire() error = %v", err)
	}
	if lock == nil {
		t.Fatal("Acquire() returned nil lock")
	}
	if !lock.IsHeld() {
		t.Error("lock.IsHeld() = false, want true")
	}

	// Release lock
	err = lock.Release(ctx)
	if err != nil {
		t.Fatalf("Release() error = %v", err)
	}
	if lock.IsHeld() {
		t.Error("lock.IsHeld() after release = true, want false")
	}
}

func TestLocker_AcquireConflict(t *testing.T) {
	t.Parallel()
	client, _ := newTestClient(t)
	locker := NewLocker(client)

	ctx := context.Background()

	// Acquire first lock
	lock1, err := locker.Acquire(ctx, "conflict-resource")
	if err != nil {
		t.Fatalf("Acquire() error = %v", err)
	}

	// Try to acquire same resource - should fail
	_, err = locker.Acquire(ctx, "conflict-resource")
	if err == nil {
		t.Error("Acquire() should fail when lock already held")
	}
	if !IsLockFailed(err) {
		t.Errorf("error should be LockFailed, got %v", err)
	}

	// Release first lock
	_ = lock1.Release(ctx)

	// Now acquisition should succeed
	lock2, err := locker.Acquire(ctx, "conflict-resource")
	if err != nil {
		t.Fatalf("Acquire() after release error = %v", err)
	}
	if lock2 == nil {
		t.Error("Acquire() after release returned nil")
	}
	_ = lock2.Release(ctx)
}

func TestLocker_TryAcquire(t *testing.T) {
	t.Parallel()
	client, _ := newTestClient(t)
	locker := NewLocker(client)

	ctx := context.Background()

	// TryAcquire on available resource
	lock, err := locker.TryAcquire(ctx, "try-resource")
	if err != nil {
		t.Fatalf("TryAcquire() error = %v", err)
	}
	if lock == nil {
		t.Fatal("TryAcquire() returned nil")
	}

	// TryAcquire on held resource - should return nil, nil
	lock2, err := locker.TryAcquire(ctx, "try-resource")
	if err != nil {
		t.Fatalf("TryAcquire() on held lock error = %v", err)
	}
	if lock2 != nil {
		t.Error("TryAcquire() on held lock should return nil")
	}

	_ = lock.Release(ctx)
}

func TestLocker_AcquireWithRetry(t *testing.T) {
	t.Parallel()
	client, _ := newTestClient(t)
	locker := NewLocker(client,
		WithLockRetryCount(5),
		WithLockRetryDelay(10*time.Millisecond),
	)

	ctx := context.Background()

	// Acquire first lock
	lock1, _ := locker.Acquire(ctx, "retry-resource")

	// Start goroutine to release lock after delay
	go func() {
		time.Sleep(30 * time.Millisecond)
		_ = lock1.Release(ctx)
	}()

	// AcquireWithRetry should succeed after lock is released
	lock2, err := locker.AcquireWithRetry(ctx, "retry-resource")
	if err != nil {
		t.Fatalf("AcquireWithRetry() error = %v", err)
	}
	if lock2 == nil {
		t.Error("AcquireWithRetry() returned nil")
	}
	_ = lock2.Release(ctx)
}

func TestLocker_WithLock(t *testing.T) {
	t.Parallel()
	client, _ := newTestClient(t)
	locker := NewLocker(client)

	ctx := context.Background()

	executed := false
	err := locker.WithLock(ctx, "withlock-resource", func(ctx context.Context) error {
		executed = true
		return nil
	})
	if err != nil {
		t.Fatalf("WithLock() error = %v", err)
	}
	if !executed {
		t.Error("WithLock() function not executed")
	}
}

func TestLocker_WithLock_FunctionError(t *testing.T) {
	t.Parallel()
	client, _ := newTestClient(t)
	locker := NewLocker(client)

	ctx := context.Background()

	fnErr := errors.New("function error")
	err := locker.WithLock(ctx, "withlock-error", func(ctx context.Context) error {
		return fnErr
	})
	if !errors.Is(err, fnErr) {
		t.Errorf("WithLock() error = %v, want %v", err, fnErr)
	}
}

func TestLock_Extend(t *testing.T) {
	t.Parallel()
	client, _ := newTestClient(t)
	locker := NewLocker(client)

	ctx := context.Background()

	lock, _ := locker.AcquireWithTTL(ctx, "extend-resource", time.Second)

	// Extend the lock
	err := lock.Extend(ctx, time.Minute)
	if err != nil {
		t.Fatalf("Extend() error = %v", err)
	}

	// Verify TTL was extended
	ttl, err := lock.TTL(ctx)
	if err != nil {
		t.Fatalf("TTL() error = %v", err)
	}
	if ttl < 50*time.Second {
		t.Errorf("TTL() = %v, want > 50s", ttl)
	}

	_ = lock.Release(ctx)
}

func TestLock_Verify(t *testing.T) {
	t.Parallel()
	client, _ := newTestClient(t)
	locker := NewLocker(client)

	ctx := context.Background()

	lock, _ := locker.Acquire(ctx, "verify-resource")

	// Verify lock is held
	held, err := lock.Verify(ctx)
	if err != nil {
		t.Fatalf("Verify() error = %v", err)
	}
	if !held {
		t.Error("Verify() = false, want true")
	}

	// Release and verify
	_ = lock.Release(ctx)

	held, err = lock.Verify(ctx)
	if err != nil {
		t.Fatalf("Verify() after release error = %v", err)
	}
	if held {
		t.Error("Verify() after release = true, want false")
	}
}

func TestLock_TTL(t *testing.T) {
	t.Parallel()
	client, _ := newTestClient(t)
	locker := NewLocker(client)

	ctx := context.Background()

	lock, _ := locker.AcquireWithTTL(ctx, "ttl-resource", time.Minute)

	ttl, err := lock.TTL(ctx)
	if err != nil {
		t.Fatalf("TTL() error = %v", err)
	}
	if ttl <= 0 || ttl > time.Minute {
		t.Errorf("TTL() = %v, want > 0 and <= 1m", ttl)
	}

	_ = lock.Release(ctx)
}

// =============================================================================
// RateLimiter Tests with Miniredis
// =============================================================================

func TestRateLimiter_Allow(t *testing.T) {
	t.Parallel()
	client, _ := newTestClient(t)
	limiter := NewRateLimiter(client,
		WithRateLimitMax(5),
		WithRateLimitWindow(time.Minute),
	)

	ctx := context.Background()

	// First 5 requests should be allowed
	for i := 0; i < 5; i++ {
		result, err := limiter.Allow(ctx, "user1")
		if err != nil {
			t.Fatalf("Allow() error = %v", err)
		}
		if !result.Allowed {
			t.Errorf("Allow() #%d = not allowed, want allowed", i+1)
		}
		if result.Remaining != int64(4-i) {
			t.Errorf("Allow() #%d Remaining = %d, want %d", i+1, result.Remaining, 4-i)
		}
	}

	// 6th request should be denied
	result, err := limiter.Allow(ctx, "user1")
	if err != nil {
		t.Fatalf("Allow() error = %v", err)
	}
	if result.Allowed {
		t.Error("Allow() #6 = allowed, want denied")
	}
}

func TestRateLimiter_AllowN(t *testing.T) {
	t.Parallel()
	client, _ := newTestClient(t)
	limiter := NewRateLimiter(client,
		WithRateLimitMax(10),
		WithRateLimitWindow(time.Minute),
	)

	ctx := context.Background()

	// Request 5 at once
	result, err := limiter.AllowN(ctx, "user2", 5)
	if err != nil {
		t.Fatalf("AllowN() error = %v", err)
	}
	if !result.Allowed {
		t.Error("AllowN(5) = not allowed, want allowed")
	}
	if result.Remaining != 5 {
		t.Errorf("AllowN(5) Remaining = %d, want 5", result.Remaining)
	}

	// Request 6 more - should be denied
	result, err = limiter.AllowN(ctx, "user2", 6)
	if err != nil {
		t.Fatalf("AllowN() error = %v", err)
	}
	if result.Allowed {
		t.Error("AllowN(6) = allowed, want denied")
	}
}

func TestRateLimiter_WithBurst(t *testing.T) {
	t.Parallel()
	client, _ := newTestClient(t)
	limiter := NewRateLimiter(client,
		WithRateLimitMax(5),
		WithRateLimitBurst(3),
		WithRateLimitWindow(time.Minute),
	)

	ctx := context.Background()

	// Should allow 8 requests (5 + 3 burst)
	for i := 0; i < 8; i++ {
		result, err := limiter.Allow(ctx, "burst-user")
		if err != nil {
			t.Fatalf("Allow() error = %v", err)
		}
		if !result.Allowed {
			t.Errorf("Allow() #%d = not allowed, want allowed", i+1)
		}
	}

	// 9th request should be denied
	result, err := limiter.Allow(ctx, "burst-user")
	if err != nil {
		t.Fatalf("Allow() error = %v", err)
	}
	if result.Allowed {
		t.Error("Allow() #9 = allowed, want denied")
	}
}

func TestRateLimiter_Reset(t *testing.T) {
	t.Parallel()
	client, _ := newTestClient(t)
	limiter := NewRateLimiter(client,
		WithRateLimitMax(2),
		WithRateLimitWindow(time.Minute),
	)

	ctx := context.Background()

	// Use up the limit
	_, _ = limiter.Allow(ctx, "reset-user")
	_, _ = limiter.Allow(ctx, "reset-user")

	result, _ := limiter.Allow(ctx, "reset-user")
	if result.Allowed {
		t.Error("Allow() should be denied after limit reached")
	}

	// Reset
	err := limiter.Reset(ctx, "reset-user")
	if err != nil {
		t.Fatalf("Reset() error = %v", err)
	}

	// Should be allowed again
	result, _ = limiter.Allow(ctx, "reset-user")
	if !result.Allowed {
		t.Error("Allow() after reset = not allowed, want allowed")
	}
}

func TestRateLimiter_GetStatus(t *testing.T) {
	t.Parallel()
	client, _ := newTestClient(t)
	limiter := NewRateLimiter(client,
		WithRateLimitMax(10),
		WithRateLimitWindow(time.Minute),
	)

	ctx := context.Background()

	// Check status before any requests
	status, err := limiter.GetStatus(ctx, "status-user")
	if err != nil {
		t.Fatalf("GetStatus() error = %v", err)
	}
	if status.Remaining != 10 {
		t.Errorf("GetStatus() Remaining = %d, want 10", status.Remaining)
	}

	// Make some requests
	_, _ = limiter.Allow(ctx, "status-user")
	_, _ = limiter.Allow(ctx, "status-user")

	// Check status - should not increment
	status, err = limiter.GetStatus(ctx, "status-user")
	if err != nil {
		t.Fatalf("GetStatus() error = %v", err)
	}
	if status.Remaining != 8 {
		t.Errorf("GetStatus() Remaining = %d, want 8", status.Remaining)
	}

	// Verify GetStatus didn't increment
	status2, _ := limiter.GetStatus(ctx, "status-user")
	if status2.Remaining != 8 {
		t.Errorf("GetStatus() incremented counter: Remaining = %d, want 8", status2.Remaining)
	}
}

func TestRateLimiter_SlidingWindow(t *testing.T) {
	t.Parallel()
	client, _ := newTestClient(t)
	limiter := NewRateLimiter(client,
		WithRateLimitMax(5),
		WithRateLimitWindow(time.Minute),
	)

	ctx := context.Background()

	// First 5 requests should be allowed
	for i := 0; i < 5; i++ {
		result, err := limiter.SlidingWindowAllow(ctx, "sliding-user")
		if err != nil {
			t.Fatalf("SlidingWindowAllow() error = %v", err)
		}
		if !result.Allowed {
			t.Errorf("SlidingWindowAllow() #%d = not allowed, want allowed", i+1)
		}
	}

	// 6th request should be denied
	result, err := limiter.SlidingWindowAllow(ctx, "sliding-user")
	if err != nil {
		t.Fatalf("SlidingWindowAllow() error = %v", err)
	}
	if result.Allowed {
		t.Error("SlidingWindowAllow() #6 = allowed, want denied")
	}
}

func TestRateLimiter_Convenience(t *testing.T) {
	t.Parallel()
	client, _ := newTestClient(t)

	t.Run("UserRateLimiter", func(t *testing.T) {
		t.Parallel()
		limiter := UserRateLimiter(client, 100, time.Minute)
		if limiter.keyPrefix != "ratelimit:user" {
			t.Errorf("keyPrefix = %q, want %q", limiter.keyPrefix, "ratelimit:user")
		}
		if limiter.maxReqs != 100 {
			t.Errorf("maxReqs = %d, want 100", limiter.maxReqs)
		}
	})

	t.Run("IPRateLimiter", func(t *testing.T) {
		t.Parallel()
		limiter := IPRateLimiter(client, 50, time.Hour)
		if limiter.keyPrefix != "ratelimit:ip" {
			t.Errorf("keyPrefix = %q, want %q", limiter.keyPrefix, "ratelimit:ip")
		}
		if limiter.maxReqs != 50 {
			t.Errorf("maxReqs = %d, want 50", limiter.maxReqs)
		}
	})
}

// =============================================================================
// Session Tests with Miniredis
// =============================================================================

func TestSessionStore_CreateAndGet(t *testing.T) {
	t.Parallel()
	client, _ := newTestClient(t)
	store := NewSessionStore(client)

	ctx := context.Background()

	// Create session
	session, err := store.Create(ctx, "user123",
		WithDeviceID("device1"),
		WithDeviceInfo("iPhone 15"),
		WithIPAddress("192.168.1.1"),
	)
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if session == nil {
		t.Fatal("Create() returned nil")
	}
	if session.UserID != "user123" {
		t.Errorf("UserID = %q, want %q", session.UserID, "user123")
	}
	if session.DeviceID != "device1" {
		t.Errorf("DeviceID = %q, want %q", session.DeviceID, "device1")
	}

	// Get session
	retrieved, err := store.Get(ctx, session.ID)
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if retrieved.ID != session.ID {
		t.Errorf("Get() ID = %q, want %q", retrieved.ID, session.ID)
	}
	if retrieved.UserID != "user123" {
		t.Errorf("Get() UserID = %q, want %q", retrieved.UserID, "user123")
	}
}

func TestSessionStore_GetWithTouch(t *testing.T) {
	t.Parallel()
	client, _ := newTestClient(t)
	store := NewSessionStore(client)

	ctx := context.Background()

	session, _ := store.Create(ctx, "user123")
	originalLastActive := session.LastActive

	time.Sleep(10 * time.Millisecond)

	// Get with touch
	retrieved, err := store.GetWithTouch(ctx, session.ID, true)
	if err != nil {
		t.Fatalf("GetWithTouch() error = %v", err)
	}
	if !retrieved.LastActive.After(originalLastActive) {
		t.Error("GetWithTouch() should update LastActive")
	}

	// Get without touch
	time.Sleep(10 * time.Millisecond)
	lastActive := retrieved.LastActive
	retrieved2, _ := store.GetWithTouch(ctx, session.ID, false)
	if retrieved2.LastActive.After(lastActive) {
		t.Error("GetWithTouch(false) should not update LastActive")
	}
}

func TestSessionStore_Delete(t *testing.T) {
	t.Parallel()
	client, _ := newTestClient(t)
	store := NewSessionStore(client)

	ctx := context.Background()

	session, _ := store.Create(ctx, "user123")

	// Delete
	err := store.Delete(ctx, session.ID)
	if err != nil {
		t.Fatalf("Delete() error = %v", err)
	}

	// Get should fail
	_, err = store.Get(ctx, session.ID)
	if err == nil {
		t.Error("Get() after delete should return error")
	}
	if !IsNotFound(err) {
		t.Errorf("Get() error should be NotFound, got %v", err)
	}
}

func TestSessionStore_DeleteByUserID(t *testing.T) {
	t.Parallel()
	client, _ := newTestClient(t)
	store := NewSessionStore(client)

	ctx := context.Background()

	// Create multiple sessions
	s1, _ := store.Create(ctx, "user456")
	s2, _ := store.Create(ctx, "user456")
	s3, _ := store.Create(ctx, "user789") // Different user

	// Delete by user ID
	deleted, err := store.DeleteByUserID(ctx, "user456")
	if err != nil {
		t.Fatalf("DeleteByUserID() error = %v", err)
	}
	if deleted < 2 {
		t.Errorf("DeleteByUserID() = %d, want >= 2", deleted)
	}

	// Verify sessions deleted
	_, err = store.Get(ctx, s1.ID)
	if !IsNotFound(err) {
		t.Error("session s1 should be deleted")
	}
	_, err = store.Get(ctx, s2.ID)
	if !IsNotFound(err) {
		t.Error("session s2 should be deleted")
	}

	// Other user's session should remain
	_, err = store.Get(ctx, s3.ID)
	if err != nil {
		t.Errorf("session s3 should not be deleted: %v", err)
	}
}

func TestSessionStore_ListByUserID(t *testing.T) {
	t.Parallel()
	client, _ := newTestClient(t)
	store := NewSessionStore(client)

	ctx := context.Background()

	// Create sessions
	_, _ = store.Create(ctx, "listuser", WithDeviceID("device1"))
	_, _ = store.Create(ctx, "listuser", WithDeviceID("device2"))

	// List
	sessions, err := store.ListByUserID(ctx, "listuser")
	if err != nil {
		t.Fatalf("ListByUserID() error = %v", err)
	}
	if len(sessions) != 2 {
		t.Errorf("ListByUserID() len = %d, want 2", len(sessions))
	}
}

func TestSessionStore_Exists(t *testing.T) {
	t.Parallel()
	client, _ := newTestClient(t)
	store := NewSessionStore(client)

	ctx := context.Background()

	// Check nonexistent
	exists, err := store.Exists(ctx, "nonexistent")
	if err != nil {
		t.Fatalf("Exists() error = %v", err)
	}
	if exists {
		t.Error("Exists() = true for nonexistent session")
	}

	// Create and check
	session, _ := store.Create(ctx, "user123")
	exists, err = store.Exists(ctx, session.ID)
	if err != nil {
		t.Fatalf("Exists() error = %v", err)
	}
	if !exists {
		t.Error("Exists() = false for existing session")
	}
}

func TestSessionStore_Extend(t *testing.T) {
	t.Parallel()
	client, _ := newTestClient(t)
	store := NewSessionStore(client)

	ctx := context.Background()

	session, _ := store.CreateWithTTL(ctx, "user123", time.Minute)
	originalExpiry := session.ExpiresAt

	// Extend
	err := store.Extend(ctx, session.ID, time.Hour)
	if err != nil {
		t.Fatalf("Extend() error = %v", err)
	}

	// Verify extended
	retrieved, _ := store.Get(ctx, session.ID)
	if !retrieved.ExpiresAt.After(originalExpiry) {
		t.Error("Extend() should update ExpiresAt")
	}
}

func TestSessionStore_Count(t *testing.T) {
	t.Parallel()
	client, _ := newTestClient(t)
	store := NewSessionStore(client)

	ctx := context.Background()

	// Count before
	count, err := store.Count(ctx, "countuser")
	if err != nil {
		t.Fatalf("Count() error = %v", err)
	}
	if count != 0 {
		t.Errorf("Count() = %d, want 0", count)
	}

	// Create sessions
	_, _ = store.Create(ctx, "countuser")
	_, _ = store.Create(ctx, "countuser")

	// Count after
	count, err = store.Count(ctx, "countuser")
	if err != nil {
		t.Fatalf("Count() error = %v", err)
	}
	if count != 2 {
		t.Errorf("Count() = %d, want 2", count)
	}
}

func TestSessionStore_Update(t *testing.T) {
	t.Parallel()
	client, _ := newTestClient(t)
	store := NewSessionStore(client)

	ctx := context.Background()

	session, _ := store.Create(ctx, "user123", WithIPAddress("1.1.1.1"))

	// Update IP
	session.IPAddress = "2.2.2.2"
	err := store.Update(ctx, session)
	if err != nil {
		t.Fatalf("Update() error = %v", err)
	}

	// Verify update
	retrieved, _ := store.Get(ctx, session.ID)
	if retrieved.IPAddress != "2.2.2.2" {
		t.Errorf("IPAddress = %q, want %q", retrieved.IPAddress, "2.2.2.2")
	}
}

func TestSessionStore_WithSessionData(t *testing.T) {
	t.Parallel()
	client, _ := newTestClient(t)
	store := NewSessionStore(client)

	ctx := context.Background()

	customData := map[string]string{"role": "admin"}
	session, err := store.Create(ctx, "user123", WithSessionData(customData))
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if session.Data == nil {
		t.Error("session.Data should not be nil")
	}
}

// =============================================================================
// Client Tests with Miniredis
// =============================================================================

func TestClient_PingAndClose(t *testing.T) {
	t.Parallel()
	client, _ := newTestClient(t)

	ctx := context.Background()

	// Ping
	err := client.Ping(ctx)
	if err != nil {
		t.Fatalf("Ping() error = %v", err)
	}

	// Close
	err = client.Close()
	if err != nil {
		t.Fatalf("Close() error = %v", err)
	}
}

func TestClient_Init(t *testing.T) {
	t.Parallel()
	client, _ := newTestClient(t)

	ctx := context.Background()

	err := client.Init(ctx)
	if err != nil {
		t.Fatalf("Init() error = %v", err)
	}
}

func TestClient_Check(t *testing.T) {
	t.Parallel()
	client, _ := newTestClient(t)

	ctx := context.Background()

	err := client.Check(ctx)
	if err != nil {
		t.Fatalf("Check() error = %v", err)
	}
}

// =============================================================================
// Additional Coverage Tests
// =============================================================================

func TestSessionStore_ListByUserID_Empty(t *testing.T) {
	t.Parallel()
	client, _ := newTestClient(t)
	store := NewSessionStore(client)

	ctx := context.Background()

	// List for non-existent user
	sessions, err := store.ListByUserID(ctx, "nonexistent-user")
	if err != nil {
		t.Fatalf("ListByUserID() error = %v", err)
	}
	if len(sessions) != 0 {
		t.Errorf("ListByUserID() len = %d, want 0", len(sessions))
	}
}

func TestCache_GetJSON_NotFound_Miniredis(t *testing.T) {
	t.Parallel()
	client, _ := newTestClient(t)
	cache := NewCache(client)

	ctx := context.Background()

	type Data struct {
		Name string
	}
	var data Data
	found, err := cache.GetJSON(ctx, "nonexistent-key", &data)
	if err != nil {
		t.Fatalf("GetJSON() error = %v", err)
	}
	if found {
		t.Error("GetJSON() should return found=false for nonexistent key")
	}
}
