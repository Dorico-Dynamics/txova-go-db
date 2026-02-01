package redis

import (
	"context"
	"log/slog"
	"testing"
	"time"
)

func TestNewLocker(t *testing.T) {
	t.Parallel()

	client, err := New()
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	t.Run("with defaults", func(t *testing.T) {
		t.Parallel()
		locker := NewLocker(client)

		if locker.client != client {
			t.Error("client not set correctly")
		}
		if locker.keyPrefix != "lock" {
			t.Errorf("keyPrefix = %q, want %q", locker.keyPrefix, "lock")
		}
		if locker.defaultTTL != DefaultLockTTL {
			t.Errorf("defaultTTL = %v, want %v", locker.defaultTTL, DefaultLockTTL)
		}
		if locker.retryDelay != DefaultLockRetryDelay {
			t.Errorf("retryDelay = %v, want %v", locker.retryDelay, DefaultLockRetryDelay)
		}
		if locker.retryCount != DefaultLockRetryCount {
			t.Errorf("retryCount = %d, want %d", locker.retryCount, DefaultLockRetryCount)
		}
		if locker.logger == nil {
			t.Error("logger is nil")
		}
	})

	t.Run("with options", func(t *testing.T) {
		t.Parallel()
		logger := slog.Default()
		locker := NewLocker(client,
			WithLockKeyPrefix("mylock"),
			WithDefaultLockTTL(time.Minute),
			WithLockRetryDelay(100*time.Millisecond),
			WithLockRetryCount(50),
			WithLockerLogger(logger),
		)

		if locker.keyPrefix != "mylock" {
			t.Errorf("keyPrefix = %q, want %q", locker.keyPrefix, "mylock")
		}
		if locker.defaultTTL != time.Minute {
			t.Errorf("defaultTTL = %v, want %v", locker.defaultTTL, time.Minute)
		}
		if locker.retryDelay != 100*time.Millisecond {
			t.Errorf("retryDelay = %v, want %v", locker.retryDelay, 100*time.Millisecond)
		}
		if locker.retryCount != 50 {
			t.Errorf("retryCount = %d, want 50", locker.retryCount)
		}
		if locker.logger != logger {
			t.Error("logger not set correctly")
		}
	})
}

func TestLocker_lockKey(t *testing.T) {
	t.Parallel()

	client, _ := New()

	t.Run("default prefix", func(t *testing.T) {
		t.Parallel()
		locker := NewLocker(client)
		if got := locker.lockKey("myresource"); got != "lock:myresource" {
			t.Errorf("lockKey() = %q, want %q", got, "lock:myresource")
		}
	})

	t.Run("custom prefix", func(t *testing.T) {
		t.Parallel()
		locker := NewLocker(client, WithLockKeyPrefix("custom"))
		if got := locker.lockKey("myresource"); got != "custom:myresource" {
			t.Errorf("lockKey() = %q, want %q", got, "custom:myresource")
		}
	})
}

func TestGenerateOwner(t *testing.T) {
	t.Parallel()

	owner1 := generateOwner()
	owner2 := generateOwner()

	if owner1 == "" {
		t.Error("generateOwner() returned empty string")
	}
	if owner2 == "" {
		t.Error("generateOwner() returned empty string")
	}
	if owner1 == owner2 {
		t.Error("generateOwner() should return unique values")
	}

	// Check length (32 hex chars for 16 bytes)
	if len(owner1) != 32 {
		t.Errorf("owner length = %d, want 32", len(owner1))
	}
}

func TestDefaultLockConstants(t *testing.T) {
	t.Parallel()

	if DefaultLockTTL != 30*time.Second {
		t.Errorf("DefaultLockTTL = %v, want %v", DefaultLockTTL, 30*time.Second)
	}
	if DefaultLockRetryDelay != 50*time.Millisecond {
		t.Errorf("DefaultLockRetryDelay = %v, want %v", DefaultLockRetryDelay, 50*time.Millisecond)
	}
	if DefaultLockRetryCount != 100 {
		t.Errorf("DefaultLockRetryCount = %d, want 100", DefaultLockRetryCount)
	}
}

func TestLockerOptions(t *testing.T) {
	t.Parallel()

	client, _ := New()

	t.Run("WithLockKeyPrefix", func(t *testing.T) {
		t.Parallel()
		locker := NewLocker(client, WithLockKeyPrefix("test-prefix"))
		if locker.keyPrefix != "test-prefix" {
			t.Errorf("keyPrefix = %q, want %q", locker.keyPrefix, "test-prefix")
		}
	})

	t.Run("WithDefaultLockTTL", func(t *testing.T) {
		t.Parallel()
		locker := NewLocker(client, WithDefaultLockTTL(2*time.Minute))
		if locker.defaultTTL != 2*time.Minute {
			t.Errorf("defaultTTL = %v, want %v", locker.defaultTTL, 2*time.Minute)
		}
	})

	t.Run("WithLockRetryDelay", func(t *testing.T) {
		t.Parallel()
		locker := NewLocker(client, WithLockRetryDelay(200*time.Millisecond))
		if locker.retryDelay != 200*time.Millisecond {
			t.Errorf("retryDelay = %v, want %v", locker.retryDelay, 200*time.Millisecond)
		}
	})

	t.Run("WithLockRetryCount", func(t *testing.T) {
		t.Parallel()
		locker := NewLocker(client, WithLockRetryCount(25))
		if locker.retryCount != 25 {
			t.Errorf("retryCount = %d, want 25", locker.retryCount)
		}
	})

	t.Run("WithLockerLogger", func(t *testing.T) {
		t.Parallel()
		logger := slog.Default()
		locker := NewLocker(client, WithLockerLogger(logger))
		if locker.logger != logger {
			t.Error("logger not set correctly")
		}
	})
}

func TestLock_Properties(t *testing.T) {
	t.Parallel()

	lock := &Lock{
		key:       "test:key",
		owner:     "test-owner-123",
		ttl:       time.Minute,
		acquired:  true,
		expiresAt: time.Now().Add(time.Minute),
	}

	if lock.Key() != "test:key" {
		t.Errorf("Key() = %q, want %q", lock.Key(), "test:key")
	}
	if lock.Owner() != "test-owner-123" {
		t.Errorf("Owner() = %q, want %q", lock.Owner(), "test-owner-123")
	}
	if !lock.IsHeld() {
		t.Error("IsHeld() should be true")
	}
	if lock.IsExpired() {
		t.Error("IsExpired() should be false")
	}
}

func TestLock_IsExpired(t *testing.T) {
	t.Parallel()

	t.Run("not expired", func(t *testing.T) {
		t.Parallel()
		lock := &Lock{
			expiresAt: time.Now().Add(time.Hour),
		}
		if lock.IsExpired() {
			t.Error("IsExpired() should be false")
		}
	})

	t.Run("expired", func(t *testing.T) {
		t.Parallel()
		lock := &Lock{
			expiresAt: time.Now().Add(-time.Hour),
		}
		if !lock.IsExpired() {
			t.Error("IsExpired() should be true")
		}
	})
}

func TestLock_Release_NotAcquired(t *testing.T) {
	t.Parallel()

	client, _ := New()
	lock := &Lock{
		client:   client,
		key:      "test:key",
		owner:    "test-owner",
		acquired: false,
		logger:   slog.Default(),
	}

	err := lock.Release(context.Background())
	if err == nil {
		t.Error("Release() should return error when lock not acquired")
	}
	if !IsLockNotHeld(err) {
		t.Errorf("error should be LockNotHeld, got %v", err)
	}
}

func TestLock_Extend_NotAcquired(t *testing.T) {
	t.Parallel()

	client, _ := New()
	lock := &Lock{
		client:   client,
		key:      "test:key",
		owner:    "test-owner",
		acquired: false,
		logger:   slog.Default(),
	}

	err := lock.Extend(context.Background(), time.Minute)
	if err == nil {
		t.Error("Extend() should return error when lock not acquired")
	}
	if !IsLockNotHeld(err) {
		t.Errorf("error should be LockNotHeld, got %v", err)
	}
}

func TestLock_Verify_NotAcquired(t *testing.T) {
	t.Parallel()

	client, _ := New()
	lock := &Lock{
		client:   client,
		key:      "test:key",
		owner:    "test-owner",
		acquired: false,
		logger:   slog.Default(),
	}

	held, err := lock.Verify(context.Background())
	if err != nil {
		t.Errorf("Verify() error = %v", err)
	}
	if held {
		t.Error("Verify() should return false when lock not acquired")
	}
}

func TestLocker_AcquireErrors(t *testing.T) {
	t.Parallel()

	// Use a non-existent Redis address to test connection errors
	client, _ := New(WithAddress("localhost:59999"))
	locker := NewLocker(client)

	// Without Redis running, acquire should fail with connection error
	_, err := locker.Acquire(context.Background(), "test-resource")
	if err == nil {
		t.Error("Acquire() should fail without Redis")
	}
}

func TestLocker_TryAcquireErrors(t *testing.T) {
	t.Parallel()

	// Use a non-existent Redis address to test connection errors
	client, _ := New(WithAddress("localhost:59999"))
	locker := NewLocker(client)

	// Without Redis running, TryAcquire should fail with connection error
	lock, err := locker.TryAcquire(context.Background(), "test-resource")
	if err == nil && lock != nil {
		t.Error("TryAcquire() should not succeed without Redis")
	}
}

func TestLocker_WithLockError(t *testing.T) {
	t.Parallel()

	// Use a non-existent Redis address to test connection errors
	client, _ := New(WithAddress("localhost:59999"))
	locker := NewLocker(client, WithLockRetryCount(1), WithLockRetryDelay(time.Millisecond))

	fnCalled := false
	err := locker.WithLock(context.Background(), "test-resource", func(ctx context.Context) error {
		fnCalled = true
		return nil
	})

	// Without Redis, should fail before calling fn
	if err == nil {
		t.Error("WithLock() should fail without Redis")
	}
	if fnCalled {
		t.Error("fn should not be called when lock acquisition fails")
	}
}

func TestLocker_AcquireWithRetry_ContextCancel(t *testing.T) {
	t.Parallel()

	// Use a non-existent Redis address to test connection errors
	client, _ := New(WithAddress("localhost:59999"))
	locker := NewLocker(client, WithLockRetryCount(1000), WithLockRetryDelay(time.Millisecond))

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	_, err := locker.AcquireWithRetry(ctx, "test-resource")
	if err == nil {
		t.Error("AcquireWithRetry() should fail with cancelled context")
	}
}

func TestLocker_AcquireWithRetryAndTTL(t *testing.T) {
	t.Parallel()

	// Use a non-existent Redis address to test connection errors
	client, _ := New(WithAddress("localhost:59999"))
	locker := NewLocker(client, WithLockRetryCount(2), WithLockRetryDelay(time.Millisecond))

	// Without Redis, should fail after retries
	_, err := locker.AcquireWithRetryAndTTL(context.Background(), "test-resource", time.Minute)
	if err == nil {
		t.Error("AcquireWithRetryAndTTL() should fail without Redis")
	}
}

func TestLocker_TryAcquireWithTTL(t *testing.T) {
	t.Parallel()

	// Use a non-existent Redis address to test connection errors
	client, _ := New(WithAddress("localhost:59999"))
	locker := NewLocker(client)

	// Without Redis, should return error
	lock, err := locker.TryAcquireWithTTL(context.Background(), "test-resource", time.Minute)
	if err == nil && lock != nil {
		t.Error("TryAcquireWithTTL() should not succeed without Redis")
	}
}

func TestLocker_WithLockAndTTL(t *testing.T) {
	t.Parallel()

	// Use a non-existent Redis address to test connection errors
	client, _ := New(WithAddress("localhost:59999"))
	locker := NewLocker(client, WithLockRetryCount(1), WithLockRetryDelay(time.Millisecond))

	fnCalled := false
	err := locker.WithLockAndTTL(context.Background(), "test-resource", time.Minute, func(ctx context.Context) error {
		fnCalled = true
		return nil
	})

	if err == nil {
		t.Error("WithLockAndTTL() should fail without Redis")
	}
	if fnCalled {
		t.Error("fn should not be called when lock acquisition fails")
	}
}

func TestLock_ExpiresAt(t *testing.T) {
	t.Parallel()

	expiresAt := time.Now().Add(time.Hour)
	lock := &Lock{
		expiresAt: expiresAt,
	}

	if !lock.ExpiresAt().Equal(expiresAt) {
		t.Errorf("ExpiresAt() = %v, want %v", lock.ExpiresAt(), expiresAt)
	}
}
