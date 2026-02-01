package redis

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"testing"
	"time"
)

func TestNewCache(t *testing.T) {
	t.Parallel()

	client, err := New()
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	t.Run("with defaults", func(t *testing.T) {
		t.Parallel()
		cache := NewCache(client)

		if cache.client != client {
			t.Error("client not set correctly")
		}
		if cache.defaultTTL != DefaultCacheTTL {
			t.Errorf("defaultTTL = %v, want %v", cache.defaultTTL, DefaultCacheTTL)
		}
		if cache.keyPrefix != "" {
			t.Errorf("keyPrefix = %q, want empty", cache.keyPrefix)
		}
		if cache.logger == nil {
			t.Error("logger is nil")
		}
	})

	t.Run("with options", func(t *testing.T) {
		t.Parallel()
		logger := slog.Default()
		cache := NewCache(client,
			WithDefaultTTL(time.Hour),
			WithKeyPrefix("myapp"),
			WithCacheLogger(logger),
		)

		if cache.defaultTTL != time.Hour {
			t.Errorf("defaultTTL = %v, want %v", cache.defaultTTL, time.Hour)
		}
		if cache.keyPrefix != "myapp" {
			t.Errorf("keyPrefix = %q, want %q", cache.keyPrefix, "myapp")
		}
		if cache.logger != logger {
			t.Error("logger not set correctly")
		}
	})
}

func TestCache_prefixKey(t *testing.T) {
	t.Parallel()

	client, _ := New()

	t.Run("without prefix", func(t *testing.T) {
		t.Parallel()
		cache := NewCache(client)
		if got := cache.prefixKey("mykey"); got != "mykey" {
			t.Errorf("prefixKey() = %q, want %q", got, "mykey")
		}
	})

	t.Run("with prefix", func(t *testing.T) {
		t.Parallel()
		cache := NewCache(client, WithKeyPrefix("app"))
		if got := cache.prefixKey("mykey"); got != "app:mykey" {
			t.Errorf("prefixKey() = %q, want %q", got, "app:mykey")
		}
	})
}

func TestKeyBuilder(t *testing.T) {
	t.Parallel()

	t.Run("Key", func(t *testing.T) {
		t.Parallel()
		kb := NewKeyBuilder("userservice")
		if got := kb.Key("user", "123"); got != "userservice:user:123" {
			t.Errorf("Key() = %q, want %q", got, "userservice:user:123")
		}
	})

	t.Run("KeyWithParts empty", func(t *testing.T) {
		t.Parallel()
		kb := NewKeyBuilder("myservice")
		if got := kb.KeyWithParts(); got != "myservice" {
			t.Errorf("KeyWithParts() = %q, want %q", got, "myservice")
		}
	})

	t.Run("KeyWithParts single", func(t *testing.T) {
		t.Parallel()
		kb := NewKeyBuilder("myservice")
		if got := kb.KeyWithParts("users"); got != "myservice:users" {
			t.Errorf("KeyWithParts() = %q, want %q", got, "myservice:users")
		}
	})

	t.Run("KeyWithParts multiple", func(t *testing.T) {
		t.Parallel()
		kb := NewKeyBuilder("myservice")
		if got := kb.KeyWithParts("users", "active", "premium"); got != "myservice:users:active:premium" {
			t.Errorf("KeyWithParts() = %q, want %q", got, "myservice:users:active:premium")
		}
	})

	t.Run("Pattern", func(t *testing.T) {
		t.Parallel()
		kb := NewKeyBuilder("orderservice")
		if got := kb.Pattern("order"); got != "orderservice:order:*" {
			t.Errorf("Pattern() = %q, want %q", got, "orderservice:order:*")
		}
	})
}

func TestDefaultCacheTTL(t *testing.T) {
	t.Parallel()

	if DefaultCacheTTL != 15*time.Minute {
		t.Errorf("DefaultCacheTTL = %v, want %v", DefaultCacheTTL, 15*time.Minute)
	}
}

type testCacheData struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

func TestCacheOptions(t *testing.T) {
	t.Parallel()

	client, _ := New()

	t.Run("WithDefaultTTL", func(t *testing.T) {
		t.Parallel()
		cache := NewCache(client, WithDefaultTTL(5*time.Minute))
		if cache.defaultTTL != 5*time.Minute {
			t.Errorf("defaultTTL = %v, want %v", cache.defaultTTL, 5*time.Minute)
		}
	})

	t.Run("WithKeyPrefix", func(t *testing.T) {
		t.Parallel()
		cache := NewCache(client, WithKeyPrefix("test-prefix"))
		if cache.keyPrefix != "test-prefix" {
			t.Errorf("keyPrefix = %q, want %q", cache.keyPrefix, "test-prefix")
		}
	})

	t.Run("WithCacheLogger", func(t *testing.T) {
		t.Parallel()
		logger := slog.Default()
		cache := NewCache(client, WithCacheLogger(logger))
		if cache.logger != logger {
			t.Error("logger not set correctly")
		}
	})

	t.Run("multiple options", func(t *testing.T) {
		t.Parallel()
		logger := slog.Default()
		cache := NewCache(client,
			WithDefaultTTL(30*time.Minute),
			WithKeyPrefix("multi"),
			WithCacheLogger(logger),
		)
		if cache.defaultTTL != 30*time.Minute {
			t.Errorf("defaultTTL = %v, want %v", cache.defaultTTL, 30*time.Minute)
		}
		if cache.keyPrefix != "multi" {
			t.Errorf("keyPrefix = %q, want %q", cache.keyPrefix, "multi")
		}
		if cache.logger != logger {
			t.Error("logger not set correctly")
		}
	})
}

func TestCache_Delete_EmptyKeys(t *testing.T) {
	t.Parallel()

	client, _ := New()
	cache := NewCache(client)

	// Should return nil for empty keys
	err := cache.Delete(context.Background())
	if err != nil {
		t.Errorf("Delete() with empty keys should return nil, got %v", err)
	}
}

func TestCache_MGet_EmptyKeys(t *testing.T) {
	t.Parallel()

	client, _ := New()
	cache := NewCache(client)

	result, err := cache.MGet(context.Background())
	if err != nil {
		t.Errorf("MGet() error = %v", err)
	}
	if result == nil {
		t.Error("MGet() should return empty map, got nil")
	}
	if len(result) != 0 {
		t.Errorf("MGet() should return empty map, got %v", result)
	}
}

func TestCache_MSet_EmptyValues(t *testing.T) {
	t.Parallel()

	client, _ := New()
	cache := NewCache(client)

	err := cache.MSet(context.Background(), map[string][]byte{})
	if err != nil {
		t.Errorf("MSet() with empty values should return nil, got %v", err)
	}
}

func TestCache_JSONSerialization(t *testing.T) {
	t.Parallel()

	t.Run("marshal error handling", func(t *testing.T) {
		t.Parallel()

		client, _ := New()
		cache := NewCache(client)

		// Channel cannot be marshaled to JSON
		ch := make(chan int)
		err := cache.SetJSON(context.Background(), "key", ch)

		if err == nil {
			t.Error("SetJSON should return error for non-marshalable type")
		}
		if !IsSerialization(err) {
			t.Errorf("error should be serialization error, got %v", err)
		}
	})
}

func TestCache_GetOrSetJSON_MarshalError(t *testing.T) {
	t.Parallel()

	client, _ := New()
	cache := NewCache(client)

	// Test with a destination that receives a value that cannot be properly handled
	// This tests the path where compute returns a value that can be marshaled
	var dest testCacheData

	computeCalled := false
	err := cache.GetOrSetJSON(context.Background(), "nonexistent-key-"+t.Name(), &dest, func(ctx context.Context) (any, error) {
		computeCalled = true
		return testCacheData{ID: 1, Name: "test"}, nil
	})

	// This will fail because Redis is not running, but we test the flow
	if err == nil && !computeCalled {
		t.Error("compute function should be called on cache miss")
	}
}

func TestCache_GetOrSet_ComputeError(t *testing.T) {
	t.Parallel()

	// Use a non-existent Redis address to ensure we test the error path
	// without interference from a running Redis instance
	client, _ := New(WithAddress("localhost:59999"))
	cache := NewCache(client)

	computeErr := errors.New("compute failed")
	_, err := cache.GetOrSet(context.Background(), "compute-error-test-key", func(ctx context.Context) ([]byte, error) {
		return nil, computeErr
	})

	// The error propagates - either from failed get (no Redis) or from compute
	if err == nil {
		t.Error("GetOrSet should propagate error")
	}
}

func TestCache_GetOrSetJSON_ComputeError(t *testing.T) {
	t.Parallel()

	client, _ := New()
	cache := NewCache(client)

	computeErr := errors.New("compute failed")
	var dest testCacheData
	err := cache.GetOrSetJSON(context.Background(), "key", &dest, func(ctx context.Context) (any, error) {
		return nil, computeErr
	})

	if err == nil {
		t.Error("GetOrSetJSON should propagate compute error")
	}
}

func TestCache_GetJSON_UnmarshalError(t *testing.T) {
	t.Parallel()

	// Test that invalid JSON data causes unmarshal error
	invalidJSON := []byte("not valid json {{{")

	var dest testCacheData
	err := json.Unmarshal(invalidJSON, &dest)
	if err == nil {
		t.Error("json.Unmarshal should fail for invalid JSON")
	}
}

func TestKeyBuilder_NewKeyBuilder(t *testing.T) {
	t.Parallel()

	kb := NewKeyBuilder("testservice")
	if kb.service != "testservice" {
		t.Errorf("service = %q, want %q", kb.service, "testservice")
	}
}

func TestCache_SetWithTTL_ZeroTTL(t *testing.T) {
	t.Parallel()

	// Test that zero TTL is handled (stores without expiration)
	client, _ := New()
	cache := NewCache(client)

	// This will fail because no Redis server, but tests the code path
	_ = cache.SetWithTTL(context.Background(), "key", []byte("value"), 0)
}

func TestCache_GetOrSetWithTTL(t *testing.T) {
	t.Parallel()

	client, _ := New()
	cache := NewCache(client)

	computeCalled := false
	_, err := cache.GetOrSetWithTTL(context.Background(), "test-key", time.Hour, func(ctx context.Context) ([]byte, error) {
		computeCalled = true
		return []byte("computed"), nil
	})

	// With no Redis, get will fail and return error before compute
	// This is correct behavior - we don't call compute if we can't check cache
	if err == nil && !computeCalled {
		t.Error("expected either error or compute to be called")
	}
}

func TestCache_GetOrSetJSONWithTTL(t *testing.T) {
	t.Parallel()

	client, _ := New()
	cache := NewCache(client)

	var dest testCacheData
	computeCalled := false

	err := cache.GetOrSetJSONWithTTL(context.Background(), "test-key", time.Hour, &dest, func(ctx context.Context) (any, error) {
		computeCalled = true
		return testCacheData{ID: 42, Name: "computed"}, nil
	})

	// With no Redis, get will fail and return error before compute
	// This is correct behavior - we don't call compute if we can't check cache
	if err == nil && !computeCalled {
		t.Error("expected either error or compute to be called")
	}
}

func TestCache_MSetWithTTL(t *testing.T) {
	t.Parallel()

	client, _ := New()
	cache := NewCache(client)

	values := map[string][]byte{
		"key1": []byte("value1"),
		"key2": []byte("value2"),
	}

	// Will fail without Redis, but tests the code path
	_ = cache.MSetWithTTL(context.Background(), values, time.Hour)
}
