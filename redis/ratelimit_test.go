package redis

import (
	"log/slog"
	"testing"
	"time"
)

func TestNewRateLimiter(t *testing.T) {
	t.Parallel()

	client, err := New()
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	t.Run("with defaults", func(t *testing.T) {
		t.Parallel()
		rl := NewRateLimiter(client)

		if rl.client != client {
			t.Error("client not set correctly")
		}
		if rl.keyPrefix != "ratelimit" {
			t.Errorf("keyPrefix = %q, want %q", rl.keyPrefix, "ratelimit")
		}
		if rl.window != DefaultRateLimitWindow {
			t.Errorf("window = %v, want %v", rl.window, DefaultRateLimitWindow)
		}
		if rl.maxReqs != DefaultRateLimitMax {
			t.Errorf("maxReqs = %d, want %d", rl.maxReqs, DefaultRateLimitMax)
		}
		if rl.burst != 0 {
			t.Errorf("burst = %d, want 0", rl.burst)
		}
		if rl.logger == nil {
			t.Error("logger is nil")
		}
	})

	t.Run("with options", func(t *testing.T) {
		t.Parallel()
		logger := slog.Default()
		rl := NewRateLimiter(client,
			WithRateLimitKeyPrefix("myrl"),
			WithRateLimitWindow(time.Hour),
			WithRateLimitMax(1000),
			WithRateLimitBurst(50),
			WithRateLimiterLogger(logger),
		)

		if rl.keyPrefix != "myrl" {
			t.Errorf("keyPrefix = %q, want %q", rl.keyPrefix, "myrl")
		}
		if rl.window != time.Hour {
			t.Errorf("window = %v, want %v", rl.window, time.Hour)
		}
		if rl.maxReqs != 1000 {
			t.Errorf("maxReqs = %d, want 1000", rl.maxReqs)
		}
		if rl.burst != 50 {
			t.Errorf("burst = %d, want 50", rl.burst)
		}
		if rl.logger != logger {
			t.Error("logger not set correctly")
		}
	})
}

func TestRateLimiter_rateLimitKey(t *testing.T) {
	t.Parallel()

	client, _ := New()

	t.Run("default prefix", func(t *testing.T) {
		t.Parallel()
		rl := NewRateLimiter(client)
		if got := rl.rateLimitKey("user123"); got != "ratelimit:user123" {
			t.Errorf("rateLimitKey() = %q, want %q", got, "ratelimit:user123")
		}
	})

	t.Run("custom prefix", func(t *testing.T) {
		t.Parallel()
		rl := NewRateLimiter(client, WithRateLimitKeyPrefix("custom"))
		if got := rl.rateLimitKey("user123"); got != "custom:user123" {
			t.Errorf("rateLimitKey() = %q, want %q", got, "custom:user123")
		}
	})
}

func TestDefaultRateLimitConstants(t *testing.T) {
	t.Parallel()

	if DefaultRateLimitWindow != time.Minute {
		t.Errorf("DefaultRateLimitWindow = %v, want %v", DefaultRateLimitWindow, time.Minute)
	}
	if DefaultRateLimitMax != 100 {
		t.Errorf("DefaultRateLimitMax = %d, want 100", DefaultRateLimitMax)
	}
}

func TestRateLimiterOptions(t *testing.T) {
	t.Parallel()

	client, _ := New()

	t.Run("WithRateLimitKeyPrefix", func(t *testing.T) {
		t.Parallel()
		rl := NewRateLimiter(client, WithRateLimitKeyPrefix("test-prefix"))
		if rl.keyPrefix != "test-prefix" {
			t.Errorf("keyPrefix = %q, want %q", rl.keyPrefix, "test-prefix")
		}
	})

	t.Run("WithRateLimitWindow", func(t *testing.T) {
		t.Parallel()
		rl := NewRateLimiter(client, WithRateLimitWindow(5*time.Minute))
		if rl.window != 5*time.Minute {
			t.Errorf("window = %v, want %v", rl.window, 5*time.Minute)
		}
	})

	t.Run("WithRateLimitMax", func(t *testing.T) {
		t.Parallel()
		rl := NewRateLimiter(client, WithRateLimitMax(500))
		if rl.maxReqs != 500 {
			t.Errorf("maxReqs = %d, want 500", rl.maxReqs)
		}
	})

	t.Run("WithRateLimitBurst", func(t *testing.T) {
		t.Parallel()
		rl := NewRateLimiter(client, WithRateLimitBurst(25))
		if rl.burst != 25 {
			t.Errorf("burst = %d, want 25", rl.burst)
		}
	})

	t.Run("WithRateLimiterLogger", func(t *testing.T) {
		t.Parallel()
		logger := slog.Default()
		rl := NewRateLimiter(client, WithRateLimiterLogger(logger))
		if rl.logger != logger {
			t.Error("logger not set correctly")
		}
	})
}

func TestRateLimitResult(t *testing.T) {
	t.Parallel()

	resetAt := time.Now().Add(time.Minute)
	result := &RateLimitResult{
		Allowed:   true,
		Remaining: 99,
		ResetAt:   resetAt,
		Total:     100,
	}

	if !result.Allowed {
		t.Error("Allowed should be true")
	}
	if result.Remaining != 99 {
		t.Errorf("Remaining = %d, want 99", result.Remaining)
	}
	if !result.ResetAt.Equal(resetAt) {
		t.Errorf("ResetAt = %v, want %v", result.ResetAt, resetAt)
	}
	if result.Total != 100 {
		t.Errorf("Total = %d, want 100", result.Total)
	}
}

func TestUserRateLimiter(t *testing.T) {
	t.Parallel()

	client, _ := New()
	rl := UserRateLimiter(client, 50, time.Hour)

	if rl.keyPrefix != "ratelimit:user" {
		t.Errorf("keyPrefix = %q, want %q", rl.keyPrefix, "ratelimit:user")
	}
	if rl.maxReqs != 50 {
		t.Errorf("maxReqs = %d, want 50", rl.maxReqs)
	}
	if rl.window != time.Hour {
		t.Errorf("window = %v, want %v", rl.window, time.Hour)
	}
}

func TestIPRateLimiter(t *testing.T) {
	t.Parallel()

	client, _ := New()
	rl := IPRateLimiter(client, 100, 5*time.Minute)

	if rl.keyPrefix != "ratelimit:ip" {
		t.Errorf("keyPrefix = %q, want %q", rl.keyPrefix, "ratelimit:ip")
	}
	if rl.maxReqs != 100 {
		t.Errorf("maxReqs = %d, want 100", rl.maxReqs)
	}
	if rl.window != 5*time.Minute {
		t.Errorf("window = %v, want %v", rl.window, 5*time.Minute)
	}
}

func TestUserRateLimiter_WithOptions(t *testing.T) {
	t.Parallel()

	client, _ := New()
	rl := UserRateLimiter(client, 50, time.Hour, WithRateLimitBurst(10))

	if rl.burst != 10 {
		t.Errorf("burst = %d, want 10", rl.burst)
	}
}

func TestIPRateLimiter_WithOptions(t *testing.T) {
	t.Parallel()

	client, _ := New()
	rl := IPRateLimiter(client, 100, time.Minute, WithRateLimitBurst(20))

	if rl.burst != 20 {
		t.Errorf("burst = %d, want 20", rl.burst)
	}
}
