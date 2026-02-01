//go:build integration

package redis

import (
	"context"
	"errors"
	"fmt"
	"os"
	"testing"
	"time"
)

// getRedisAddress returns the Redis address from environment or default.
func getRedisAddress() string {
	host := os.Getenv("REDIS_HOST")
	if host == "" {
		host = "localhost"
	}
	port := os.Getenv("REDIS_PORT")
	if port == "" {
		port = "6379"
	}
	return fmt.Sprintf("%s:%s", host, port)
}

func setupRedisContainer(t *testing.T) (*Client, func()) {
	t.Helper()

	addr := getRedisAddress()
	t.Logf("Connecting to Redis at %s", addr)

	client, err := New(WithAddress(addr))
	if err != nil {
		t.Fatalf("failed to create redis client: %v", err)
	}

	// Verify connection
	ctx := context.Background()
	if err := client.Ping(ctx); err != nil {
		client.Close()
		t.Fatalf("failed to ping redis: %v", err)
	}

	// Flush database to ensure clean state for each test
	if err := client.client.FlushDB(ctx).Err(); err != nil {
		client.Close()
		t.Fatalf("failed to flush redis: %v", err)
	}

	cleanup := func() {
		// Flush again on cleanup to leave clean state
		_ = client.client.FlushDB(context.Background()).Err()
		_ = client.Close()
	}

	return client, cleanup
}

func TestIntegration_ClientPing(t *testing.T) {
	client, cleanup := setupRedisContainer(t)
	defer cleanup()

	ctx := context.Background()
	if err := client.Ping(ctx); err != nil {
		t.Errorf("Ping() error = %v", err)
	}
}

func TestIntegration_ClientStats(t *testing.T) {
	client, cleanup := setupRedisContainer(t)
	defer cleanup()

	stats := client.Stats()
	if stats == nil {
		t.Error("Stats() returned nil")
	}
}

func TestIntegration_ClientInit(t *testing.T) {
	client, cleanup := setupRedisContainer(t)
	defer cleanup()

	ctx := context.Background()
	if err := client.Init(ctx); err != nil {
		t.Errorf("Init() error = %v", err)
	}
}

func TestIntegration_ClientCheck(t *testing.T) {
	client, cleanup := setupRedisContainer(t)
	defer cleanup()

	ctx := context.Background()
	if err := client.Check(ctx); err != nil {
		t.Errorf("Check() error = %v", err)
	}
}

func TestIntegration_CacheBasicOperations(t *testing.T) {
	client, cleanup := setupRedisContainer(t)
	defer cleanup()

	ctx := context.Background()
	cache := NewCache(client)

	// Test Set and Get
	key := "test:key"
	value := []byte("test value")

	if err := cache.Set(ctx, key, value); err != nil {
		t.Fatalf("Set() error = %v", err)
	}

	got, err := cache.Get(ctx, key)
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if string(got) != string(value) {
		t.Errorf("Get() = %q, want %q", got, value)
	}

	// Test cache miss
	miss, err := cache.Get(ctx, "nonexistent")
	if err != nil {
		t.Errorf("Get() for nonexistent key should not error, got %v", err)
	}
	if miss != nil {
		t.Errorf("Get() for nonexistent key should return nil, got %v", miss)
	}

	// Test Delete
	if err := cache.Delete(ctx, key); err != nil {
		t.Fatalf("Delete() error = %v", err)
	}

	deleted, err := cache.Get(ctx, key)
	if err != nil {
		t.Errorf("Get() after delete should not error, got %v", err)
	}
	if deleted != nil {
		t.Errorf("Get() after delete should return nil, got %v", deleted)
	}
}

func TestIntegration_CacheJSON(t *testing.T) {
	client, cleanup := setupRedisContainer(t)
	defer cleanup()

	ctx := context.Background()
	cache := NewCache(client)

	type TestData struct {
		Name  string `json:"name"`
		Value int    `json:"value"`
	}

	key := "test:json"
	data := TestData{Name: "test", Value: 42}

	if err := cache.SetJSON(ctx, key, data); err != nil {
		t.Fatalf("SetJSON() error = %v", err)
	}

	var got TestData
	found, err := cache.GetJSON(ctx, key, &got)
	if err != nil {
		t.Fatalf("GetJSON() error = %v", err)
	}
	if !found {
		t.Fatal("GetJSON() should return true for existing key")
	}
	if got.Name != data.Name || got.Value != data.Value {
		t.Errorf("GetJSON() = %+v, want %+v", got, data)
	}
}

func TestIntegration_CacheGetOrSet(t *testing.T) {
	client, cleanup := setupRedisContainer(t)
	defer cleanup()

	ctx := context.Background()
	cache := NewCache(client)

	key := "test:getorset"
	computeCalled := 0

	compute := func(ctx context.Context) ([]byte, error) {
		computeCalled++
		return []byte("computed value"), nil
	}

	// First call should compute
	val1, err := cache.GetOrSet(ctx, key, compute)
	if err != nil {
		t.Fatalf("GetOrSet() error = %v", err)
	}
	if string(val1) != "computed value" {
		t.Errorf("GetOrSet() = %q, want %q", val1, "computed value")
	}
	if computeCalled != 1 {
		t.Errorf("compute called %d times, want 1", computeCalled)
	}

	// Second call should use cache
	val2, err := cache.GetOrSet(ctx, key, compute)
	if err != nil {
		t.Fatalf("GetOrSet() error = %v", err)
	}
	if string(val2) != "computed value" {
		t.Errorf("GetOrSet() = %q, want %q", val2, "computed value")
	}
	if computeCalled != 1 {
		t.Errorf("compute called %d times, want 1 (should use cache)", computeCalled)
	}
}

func TestIntegration_CacheMGet(t *testing.T) {
	client, cleanup := setupRedisContainer(t)
	defer cleanup()

	ctx := context.Background()
	cache := NewCache(client)

	// Set multiple values
	_ = cache.Set(ctx, "key1", []byte("value1"))
	_ = cache.Set(ctx, "key2", []byte("value2"))

	// MGet
	results, err := cache.MGet(ctx, "key1", "key2", "key3")
	if err != nil {
		t.Fatalf("MGet() error = %v", err)
	}

	if string(results["key1"]) != "value1" {
		t.Errorf("MGet()[key1] = %q, want %q", results["key1"], "value1")
	}
	if string(results["key2"]) != "value2" {
		t.Errorf("MGet()[key2] = %q, want %q", results["key2"], "value2")
	}
	if results["key3"] != nil {
		t.Errorf("MGet()[key3] = %v, want nil", results["key3"])
	}
}

func TestIntegration_CacheMSet(t *testing.T) {
	client, cleanup := setupRedisContainer(t)
	defer cleanup()

	ctx := context.Background()
	cache := NewCache(client)

	values := map[string][]byte{
		"mset1": []byte("value1"),
		"mset2": []byte("value2"),
	}

	if err := cache.MSet(ctx, values); err != nil {
		t.Fatalf("MSet() error = %v", err)
	}

	val1, _ := cache.Get(ctx, "mset1")
	val2, _ := cache.Get(ctx, "mset2")

	if string(val1) != "value1" {
		t.Errorf("MSet key1 = %q, want %q", val1, "value1")
	}
	if string(val2) != "value2" {
		t.Errorf("MSet key2 = %q, want %q", val2, "value2")
	}
}

func TestIntegration_CacheExists(t *testing.T) {
	client, cleanup := setupRedisContainer(t)
	defer cleanup()

	ctx := context.Background()
	cache := NewCache(client)

	_ = cache.Set(ctx, "exists-key", []byte("value"))

	exists, err := cache.Exists(ctx, "exists-key")
	if err != nil {
		t.Fatalf("Exists() error = %v", err)
	}
	if !exists {
		t.Error("Exists() should return true for existing key")
	}

	exists, err = cache.Exists(ctx, "nonexistent-key")
	if err != nil {
		t.Fatalf("Exists() error = %v", err)
	}
	if exists {
		t.Error("Exists() should return false for nonexistent key")
	}
}

func TestIntegration_CacheTTL(t *testing.T) {
	client, cleanup := setupRedisContainer(t)
	defer cleanup()

	ctx := context.Background()
	cache := NewCache(client)

	_ = cache.SetWithTTL(ctx, "ttl-key", []byte("value"), 5*time.Minute)

	ttl, err := cache.TTL(ctx, "ttl-key")
	if err != nil {
		t.Fatalf("TTL() error = %v", err)
	}
	if ttl < 4*time.Minute || ttl > 5*time.Minute {
		t.Errorf("TTL() = %v, expected around 5 minutes", ttl)
	}
}

func TestIntegration_CacheExpire(t *testing.T) {
	client, cleanup := setupRedisContainer(t)
	defer cleanup()

	ctx := context.Background()
	cache := NewCache(client)

	_ = cache.Set(ctx, "expire-key", []byte("value"))

	ok, err := cache.Expire(ctx, "expire-key", 10*time.Minute)
	if err != nil {
		t.Fatalf("Expire() error = %v", err)
	}
	if !ok {
		t.Error("Expire() should return true for existing key")
	}

	ok, err = cache.Expire(ctx, "nonexistent-expire", 10*time.Minute)
	if err != nil {
		t.Fatalf("Expire() error = %v", err)
	}
	if ok {
		t.Error("Expire() should return false for nonexistent key")
	}
}

func TestIntegration_CacheDeleteByPattern(t *testing.T) {
	client, cleanup := setupRedisContainer(t)
	defer cleanup()

	ctx := context.Background()
	cache := NewCache(client, WithKeyPrefix("app"))

	// Set keys with pattern
	_ = cache.Set(ctx, "user:1", []byte("user1"))
	_ = cache.Set(ctx, "user:2", []byte("user2"))
	_ = cache.Set(ctx, "other:1", []byte("other"))

	deleted, err := cache.DeleteByPattern(ctx, "user:*")
	if err != nil {
		t.Fatalf("DeleteByPattern() error = %v", err)
	}
	if deleted != 2 {
		t.Errorf("DeleteByPattern() deleted %d, want 2", deleted)
	}

	// Verify user keys deleted
	val, _ := cache.Get(ctx, "user:1")
	if val != nil {
		t.Error("user:1 should be deleted")
	}

	// Verify other key still exists
	val, _ = cache.Get(ctx, "other:1")
	if val == nil {
		t.Error("other:1 should still exist")
	}
}

func TestIntegration_LockAcquireRelease(t *testing.T) {
	client, cleanup := setupRedisContainer(t)
	defer cleanup()

	ctx := context.Background()
	locker := NewLocker(client)

	lock, err := locker.Acquire(ctx, "test-resource")
	if err != nil {
		t.Fatalf("Acquire() error = %v", err)
	}

	if !lock.IsHeld() {
		t.Error("lock should be held")
	}

	// Try to acquire again - should fail
	_, err = locker.Acquire(ctx, "test-resource")
	if err == nil {
		t.Error("second Acquire() should fail")
	}
	if !IsLockFailed(err) {
		t.Errorf("error should be LockFailed, got %v", err)
	}

	// Release
	if err := lock.Release(ctx); err != nil {
		t.Fatalf("Release() error = %v", err)
	}

	if lock.IsHeld() {
		t.Error("lock should not be held after release")
	}

	// Now should be able to acquire again
	lock2, err := locker.Acquire(ctx, "test-resource")
	if err != nil {
		t.Fatalf("Acquire() after release error = %v", err)
	}
	_ = lock2.Release(ctx)
}

func TestIntegration_LockExtend(t *testing.T) {
	client, cleanup := setupRedisContainer(t)
	defer cleanup()

	ctx := context.Background()
	locker := NewLocker(client)

	lock, err := locker.AcquireWithTTL(ctx, "extend-resource", 5*time.Second)
	if err != nil {
		t.Fatalf("Acquire() error = %v", err)
	}
	defer lock.Release(ctx)

	// Extend
	if err := lock.Extend(ctx, time.Minute); err != nil {
		t.Fatalf("Extend() error = %v", err)
	}

	// Check TTL was extended
	ttl, err := lock.TTL(ctx)
	if err != nil {
		t.Fatalf("TTL() error = %v", err)
	}
	if ttl < 50*time.Second {
		t.Errorf("TTL() = %v, expected > 50s after extend", ttl)
	}
}

func TestIntegration_LockVerify(t *testing.T) {
	client, cleanup := setupRedisContainer(t)
	defer cleanup()

	ctx := context.Background()
	locker := NewLocker(client)

	lock, err := locker.Acquire(ctx, "verify-resource")
	if err != nil {
		t.Fatalf("Acquire() error = %v", err)
	}

	held, err := lock.Verify(ctx)
	if err != nil {
		t.Fatalf("Verify() error = %v", err)
	}
	if !held {
		t.Error("Verify() should return true while lock is held")
	}

	_ = lock.Release(ctx)

	held, err = lock.Verify(ctx)
	if err != nil {
		t.Fatalf("Verify() after release error = %v", err)
	}
	if held {
		t.Error("Verify() should return false after release")
	}
}

func TestIntegration_LockWithLock(t *testing.T) {
	client, cleanup := setupRedisContainer(t)
	defer cleanup()

	ctx := context.Background()
	locker := NewLocker(client)

	executed := false
	err := locker.WithLock(ctx, "withlock-resource", func(ctx context.Context) error {
		executed = true
		return nil
	})

	if err != nil {
		t.Fatalf("WithLock() error = %v", err)
	}
	if !executed {
		t.Error("function should have been executed")
	}

	// Verify lock was released
	lock, err := locker.Acquire(ctx, "withlock-resource")
	if err != nil {
		t.Fatalf("Acquire() after WithLock should succeed, got %v", err)
	}
	_ = lock.Release(ctx)
}

func TestIntegration_LockTryAcquire(t *testing.T) {
	client, cleanup := setupRedisContainer(t)
	defer cleanup()

	ctx := context.Background()
	locker := NewLocker(client)

	// First try should succeed
	lock, err := locker.TryAcquire(ctx, "try-resource")
	if err != nil {
		t.Fatalf("TryAcquire() error = %v", err)
	}
	if lock == nil {
		t.Fatal("TryAcquire() should return lock")
	}

	// Second try should return nil without error
	lock2, err := locker.TryAcquire(ctx, "try-resource")
	if err != nil {
		t.Fatalf("TryAcquire() second call error = %v", err)
	}
	if lock2 != nil {
		t.Error("TryAcquire() should return nil when lock is held")
	}

	_ = lock.Release(ctx)
}

func TestIntegration_RateLimiterAllow(t *testing.T) {
	client, cleanup := setupRedisContainer(t)
	defer cleanup()

	ctx := context.Background()
	rl := NewRateLimiter(client,
		WithRateLimitMax(5),
		WithRateLimitWindow(time.Minute),
	)

	// Should allow first 5 requests
	for i := range 5 {
		result, err := rl.Allow(ctx, "user1")
		if err != nil {
			t.Fatalf("Allow() error = %v", err)
		}
		if !result.Allowed {
			t.Errorf("request %d should be allowed", i+1)
		}
		if result.Remaining != int64(4-i) {
			t.Errorf("Remaining = %d, want %d", result.Remaining, 4-i)
		}
	}

	// 6th request should be denied
	result, err := rl.Allow(ctx, "user1")
	if err != nil {
		t.Fatalf("Allow() error = %v", err)
	}
	if result.Allowed {
		t.Error("6th request should be denied")
	}
	if result.Remaining != 0 {
		t.Errorf("Remaining = %d, want 0", result.Remaining)
	}
}

func TestIntegration_RateLimiterBurst(t *testing.T) {
	client, cleanup := setupRedisContainer(t)
	defer cleanup()

	ctx := context.Background()
	rl := NewRateLimiter(client,
		WithRateLimitMax(5),
		WithRateLimitBurst(2),
		WithRateLimitWindow(time.Minute),
	)

	// Should allow 7 requests (5 + 2 burst)
	for i := range 7 {
		result, err := rl.Allow(ctx, "burst-user")
		if err != nil {
			t.Fatalf("Allow() error = %v", err)
		}
		if !result.Allowed {
			t.Errorf("request %d should be allowed (with burst)", i+1)
		}
	}

	// 8th should be denied
	result, _ := rl.Allow(ctx, "burst-user")
	if result.Allowed {
		t.Error("8th request should be denied")
	}
}

func TestIntegration_RateLimiterReset(t *testing.T) {
	client, cleanup := setupRedisContainer(t)
	defer cleanup()

	ctx := context.Background()
	rl := NewRateLimiter(client, WithRateLimitMax(2))

	// Use up the limit
	_, _ = rl.Allow(ctx, "reset-user")
	_, _ = rl.Allow(ctx, "reset-user")

	result, _ := rl.Allow(ctx, "reset-user")
	if result.Allowed {
		t.Error("should be rate limited")
	}

	// Reset
	if err := rl.Reset(ctx, "reset-user"); err != nil {
		t.Fatalf("Reset() error = %v", err)
	}

	// Should allow again
	result, _ = rl.Allow(ctx, "reset-user")
	if !result.Allowed {
		t.Error("should be allowed after reset")
	}
}

func TestIntegration_RateLimiterGetStatus(t *testing.T) {
	client, cleanup := setupRedisContainer(t)
	defer cleanup()

	ctx := context.Background()
	rl := NewRateLimiter(client, WithRateLimitMax(10))

	// Use 3 requests
	_, _ = rl.Allow(ctx, "status-user")
	_, _ = rl.Allow(ctx, "status-user")
	_, _ = rl.Allow(ctx, "status-user")

	status, err := rl.GetStatus(ctx, "status-user")
	if err != nil {
		t.Fatalf("GetStatus() error = %v", err)
	}

	if status.Remaining != 7 {
		t.Errorf("Remaining = %d, want 7", status.Remaining)
	}
	if status.Total != 10 {
		t.Errorf("Total = %d, want 10", status.Total)
	}
}

func TestIntegration_RateLimiterSlidingWindow(t *testing.T) {
	client, cleanup := setupRedisContainer(t)
	defer cleanup()

	ctx := context.Background()
	rl := NewRateLimiter(client,
		WithRateLimitMax(5),
		WithRateLimitWindow(time.Minute),
	)

	// Should allow first 5 requests
	for i := range 5 {
		result, err := rl.SlidingWindowAllow(ctx, "sliding-user")
		if err != nil {
			t.Fatalf("SlidingWindowAllow() error = %v", err)
		}
		if !result.Allowed {
			t.Errorf("request %d should be allowed", i+1)
		}
	}

	// 6th should be denied
	result, _ := rl.SlidingWindowAllow(ctx, "sliding-user")
	if result.Allowed {
		t.Error("6th request should be denied")
	}
}

func TestIntegration_SessionCreateAndGet(t *testing.T) {
	client, cleanup := setupRedisContainer(t)
	defer cleanup()

	ctx := context.Background()
	store := NewSessionStore(client)

	session, err := store.Create(ctx, "user123",
		WithDeviceID("device1"),
		WithDeviceInfo("Chrome on macOS"),
		WithIPAddress("192.168.1.1"),
	)
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	if session.ID == "" {
		t.Error("session ID should not be empty")
	}
	if session.UserID != "user123" {
		t.Errorf("UserID = %q, want %q", session.UserID, "user123")
	}
	if session.DeviceID != "device1" {
		t.Errorf("DeviceID = %q, want %q", session.DeviceID, "device1")
	}

	// Get session
	got, err := store.Get(ctx, session.ID)
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if got.ID != session.ID {
		t.Errorf("Get() ID = %q, want %q", got.ID, session.ID)
	}
	if got.UserID != "user123" {
		t.Errorf("Get() UserID = %q, want %q", got.UserID, "user123")
	}
}

func TestIntegration_SessionDelete(t *testing.T) {
	client, cleanup := setupRedisContainer(t)
	defer cleanup()

	ctx := context.Background()
	store := NewSessionStore(client)

	session, _ := store.Create(ctx, "user456")

	if err := store.Delete(ctx, session.ID); err != nil {
		t.Fatalf("Delete() error = %v", err)
	}

	_, err := store.Get(ctx, session.ID)
	if err == nil {
		t.Error("Get() after Delete should return error")
	}
	if !IsNotFound(err) {
		t.Errorf("error should be NotFound, got %v", err)
	}
}

func TestIntegration_SessionListByUserID(t *testing.T) {
	client, cleanup := setupRedisContainer(t)
	defer cleanup()

	ctx := context.Background()
	store := NewSessionStore(client)

	// Create multiple sessions for same user
	_, _ = store.Create(ctx, "multi-user", WithDeviceID("device1"))
	_, _ = store.Create(ctx, "multi-user", WithDeviceID("device2"))
	_, _ = store.Create(ctx, "other-user", WithDeviceID("device3"))

	sessions, err := store.ListByUserID(ctx, "multi-user")
	if err != nil {
		t.Fatalf("ListByUserID() error = %v", err)
	}
	if len(sessions) != 2 {
		t.Errorf("ListByUserID() returned %d sessions, want 2", len(sessions))
	}
}

func TestIntegration_SessionDeleteByUserID(t *testing.T) {
	client, cleanup := setupRedisContainer(t)
	defer cleanup()

	ctx := context.Background()
	store := NewSessionStore(client)

	_, _ = store.Create(ctx, "delete-user", WithDeviceID("device1"))
	_, _ = store.Create(ctx, "delete-user", WithDeviceID("device2"))

	deleted, err := store.DeleteByUserID(ctx, "delete-user")
	if err != nil {
		t.Fatalf("DeleteByUserID() error = %v", err)
	}
	if deleted < 2 {
		t.Errorf("DeleteByUserID() deleted %d, want >= 2", deleted)
	}

	sessions, _ := store.ListByUserID(ctx, "delete-user")
	if len(sessions) != 0 {
		t.Errorf("sessions remaining = %d, want 0", len(sessions))
	}
}

func TestIntegration_SessionExists(t *testing.T) {
	client, cleanup := setupRedisContainer(t)
	defer cleanup()

	ctx := context.Background()
	store := NewSessionStore(client)

	session, _ := store.Create(ctx, "exists-user")

	exists, err := store.Exists(ctx, session.ID)
	if err != nil {
		t.Fatalf("Exists() error = %v", err)
	}
	if !exists {
		t.Error("Exists() should return true for existing session")
	}

	exists, err = store.Exists(ctx, "nonexistent-session")
	if err != nil {
		t.Fatalf("Exists() error = %v", err)
	}
	if exists {
		t.Error("Exists() should return false for nonexistent session")
	}
}

func TestIntegration_SessionExtend(t *testing.T) {
	client, cleanup := setupRedisContainer(t)
	defer cleanup()

	ctx := context.Background()
	store := NewSessionStore(client)

	session, _ := store.CreateWithTTL(ctx, "extend-user", time.Minute)

	if err := store.Extend(ctx, session.ID, 2*time.Hour); err != nil {
		t.Fatalf("Extend() error = %v", err)
	}

	got, _ := store.Get(ctx, session.ID)
	if got.ExpiresAt.Before(time.Now().Add(time.Hour)) {
		t.Error("ExpiresAt should be extended")
	}
}

func TestIntegration_SessionCount(t *testing.T) {
	client, cleanup := setupRedisContainer(t)
	defer cleanup()

	ctx := context.Background()
	store := NewSessionStore(client)

	_, _ = store.Create(ctx, "count-user")
	_, _ = store.Create(ctx, "count-user")
	_, _ = store.Create(ctx, "count-user")

	count, err := store.Count(ctx, "count-user")
	if err != nil {
		t.Fatalf("Count() error = %v", err)
	}
	if count != 3 {
		t.Errorf("Count() = %d, want 3", count)
	}
}

func TestIntegration_SessionUpdate(t *testing.T) {
	client, cleanup := setupRedisContainer(t)
	defer cleanup()

	ctx := context.Background()
	store := NewSessionStore(client)

	session, _ := store.Create(ctx, "update-user")
	session.DeviceInfo = "Updated Info"

	if err := store.Update(ctx, session); err != nil {
		t.Fatalf("Update() error = %v", err)
	}

	got, _ := store.Get(ctx, session.ID)
	if got.DeviceInfo != "Updated Info" {
		t.Errorf("DeviceInfo = %q, want %q", got.DeviceInfo, "Updated Info")
	}
}

func TestIntegration_CacheGetOrSetJSON(t *testing.T) {
	client, cleanup := setupRedisContainer(t)
	defer cleanup()

	ctx := context.Background()
	cache := NewCache(client)

	type Data struct {
		Value int `json:"value"`
	}

	key := "getorsetjson-key"
	computeCalled := 0

	var dest Data
	err := cache.GetOrSetJSON(ctx, key, &dest, func(ctx context.Context) (any, error) {
		computeCalled++
		return Data{Value: 42}, nil
	})
	if err != nil {
		t.Fatalf("GetOrSetJSON() error = %v", err)
	}
	if dest.Value != 42 {
		t.Errorf("Value = %d, want 42", dest.Value)
	}
	if computeCalled != 1 {
		t.Errorf("compute called %d times, want 1", computeCalled)
	}

	// Second call should use cache
	var dest2 Data
	err = cache.GetOrSetJSON(ctx, key, &dest2, func(ctx context.Context) (any, error) {
		computeCalled++
		return Data{Value: 99}, nil
	})
	if err != nil {
		t.Fatalf("GetOrSetJSON() second call error = %v", err)
	}
	if dest2.Value != 42 {
		t.Errorf("cached Value = %d, want 42", dest2.Value)
	}
	if computeCalled != 1 {
		t.Errorf("compute called %d times, want 1", computeCalled)
	}
}

func TestIntegration_LockAcquireWithRetry(t *testing.T) {
	client, cleanup := setupRedisContainer(t)
	defer cleanup()

	ctx := context.Background()
	locker := NewLocker(client,
		WithLockRetryCount(3),
		WithLockRetryDelay(10*time.Millisecond),
	)

	// Acquire lock
	lock1, _ := locker.Acquire(ctx, "retry-resource")

	// Try to acquire with retry (should fail after retries)
	_, err := locker.AcquireWithRetry(ctx, "retry-resource")
	if err == nil {
		t.Error("AcquireWithRetry() should fail when lock is held")
	}

	// Release first lock
	_ = lock1.Release(ctx)

	// Now should succeed
	lock2, err := locker.AcquireWithRetry(ctx, "retry-resource")
	if err != nil {
		t.Fatalf("AcquireWithRetry() after release error = %v", err)
	}
	_ = lock2.Release(ctx)
}

func TestIntegration_LockWithLockError(t *testing.T) {
	client, cleanup := setupRedisContainer(t)
	defer cleanup()

	ctx := context.Background()
	locker := NewLocker(client)

	testErr := errors.New("test error")
	err := locker.WithLock(ctx, "error-resource", func(ctx context.Context) error {
		return testErr
	})

	if !errors.Is(err, testErr) {
		t.Errorf("WithLock() should propagate function error, got %v", err)
	}

	// Lock should still be released
	lock, err := locker.Acquire(ctx, "error-resource")
	if err != nil {
		t.Fatalf("Acquire() after WithLock error should succeed, got %v", err)
	}
	_ = lock.Release(ctx)
}
