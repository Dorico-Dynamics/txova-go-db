package postgres

import (
	"context"
	"testing"
	"time"

	"github.com/Dorico-Dynamics/txova-go-core/logging"
)

func TestDefaultTxManagerConfig(t *testing.T) {
	t.Parallel()

	cfg := DefaultTxManagerConfig()

	if cfg.MaxRetries != 3 {
		t.Errorf("MaxRetries = %d, want 3", cfg.MaxRetries)
	}
	if cfg.RetryBaseDelay != 50*time.Millisecond {
		t.Errorf("RetryBaseDelay = %v, want 50ms", cfg.RetryBaseDelay)
	}
	if cfg.RetryMaxDelay != 2*time.Second {
		t.Errorf("RetryMaxDelay = %v, want 2s", cfg.RetryMaxDelay)
	}
	if cfg.Logger == nil {
		t.Error("Logger should not be nil")
	}
}

func TestTxManagerConfigOptions(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		opt      TxManagerOption
		validate func(*testing.T, TxManagerConfig)
	}{
		{
			name: "WithMaxRetries",
			opt:  WithMaxRetries(5),
			validate: func(t *testing.T, cfg TxManagerConfig) {
				t.Helper()
				if cfg.MaxRetries != 5 {
					t.Errorf("MaxRetries = %d, want 5", cfg.MaxRetries)
				}
			},
		},
		{
			name: "WithRetryBaseDelay",
			opt:  WithRetryBaseDelay(100 * time.Millisecond),
			validate: func(t *testing.T, cfg TxManagerConfig) {
				t.Helper()
				if cfg.RetryBaseDelay != 100*time.Millisecond {
					t.Errorf("RetryBaseDelay = %v, want 100ms", cfg.RetryBaseDelay)
				}
			},
		},
		{
			name: "WithRetryMaxDelay",
			opt:  WithRetryMaxDelay(5 * time.Second),
			validate: func(t *testing.T, cfg TxManagerConfig) {
				t.Helper()
				if cfg.RetryMaxDelay != 5*time.Second {
					t.Errorf("RetryMaxDelay = %v, want 5s", cfg.RetryMaxDelay)
				}
			},
		},
		{
			name: "WithTxLogger",
			opt:  WithTxLogger(logging.Default()),
			validate: func(t *testing.T, cfg TxManagerConfig) {
				t.Helper()
				if cfg.Logger == nil {
					t.Error("Logger should not be nil")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			cfg := DefaultTxManagerConfig()
			tt.opt(&cfg)
			tt.validate(t, cfg)
		})
	}
}

func TestTxFromContext(t *testing.T) {
	t.Parallel()

	t.Run("empty context returns false", func(t *testing.T) {
		t.Parallel()

		ctx := context.Background()
		tx, ok := TxFromContext(ctx)

		if ok {
			t.Error("expected ok to be false for empty context")
		}
		if tx != nil {
			t.Error("expected tx to be nil for empty context")
		}
	})

	t.Run("context with tx returns tx", func(t *testing.T) {
		t.Parallel()

		// Create a mock tx (we can't create a real one without a database).
		// We use the context key directly for this test.
		ctx := context.Background()
		ctx = context.WithValue(ctx, txContextKey{}, (*pgxTx)(nil))

		tx, ok := TxFromContext(ctx)

		if !ok {
			t.Error("expected ok to be true for context with tx")
		}
		// tx will be nil since we stored a nil *pgxTx, but the ok should be true.
		_ = tx
	})
}

func TestContextWithTx(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	// Storing nil should still create a new context.
	newCtx := ContextWithTx(ctx, nil)
	if newCtx == ctx {
		t.Error("expected new context to be different from original")
	}

	// When nil is stored, TxFromContext should return nil, false
	// because the type assertion to Tx interface fails for nil.
	tx, ok := TxFromContext(newCtx)
	if ok {
		t.Error("expected ok to be false when nil is stored")
	}
	if tx != nil {
		t.Error("expected tx to be nil when nil is stored")
	}
}

func TestCalculateRetryDelay(t *testing.T) {
	t.Parallel()

	mgr := &txManager{
		config: TxManagerConfig{
			RetryBaseDelay: 100 * time.Millisecond,
			RetryMaxDelay:  1 * time.Second,
		},
	}

	// Test that delays are within expected ranges.
	// With base 100ms:
	// Attempt 1: base * 2^0 = 100ms, with jitter ±25% → 75-125ms
	// Attempt 2: base * 2^1 = 200ms, with jitter ±25% → 150-250ms
	// Attempt 3: base * 2^2 = 400ms, with jitter ±25% → 300-500ms

	delay1 := mgr.calculateRetryDelay(1)
	if delay1 < 75*time.Millisecond || delay1 > 125*time.Millisecond {
		t.Errorf("delay1 = %v, expected in range [75ms, 125ms]", delay1)
	}

	delay2 := mgr.calculateRetryDelay(2)
	if delay2 < 150*time.Millisecond || delay2 > 250*time.Millisecond {
		t.Errorf("delay2 = %v, expected in range [150ms, 250ms]", delay2)
	}

	delay3 := mgr.calculateRetryDelay(3)
	if delay3 < 300*time.Millisecond || delay3 > 500*time.Millisecond {
		t.Errorf("delay3 = %v, expected in range [300ms, 500ms]", delay3)
	}
}

func TestCalculateRetryDelayMaxDelay(t *testing.T) {
	t.Parallel()

	mgr := &txManager{
		config: TxManagerConfig{
			RetryBaseDelay: 100 * time.Millisecond,
			RetryMaxDelay:  150 * time.Millisecond,
		},
	}

	// With a low max delay, high attempts should be capped.
	// Attempt 5: base * 2^4 = 1600ms, capped to 150ms, with jitter → ~112-187ms
	delay := mgr.calculateRetryDelay(5)
	// Since we cap first then add jitter, the result should be around 112-187ms.
	if delay > 200*time.Millisecond {
		t.Errorf("delay = %v, expected to be capped near max delay", delay)
	}
}

func TestIsRetryable(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		err      error
		expected bool
	}{
		{
			name:     "serialization error is retryable",
			err:      New(CodeSerialization, "serialization failure"),
			expected: true,
		},
		{
			name:     "deadlock error is retryable",
			err:      New(CodeDeadlock, "deadlock detected"),
			expected: true,
		},
		{
			name:     "not found error is not retryable",
			err:      New(CodeNotFound, "record not found"),
			expected: false,
		},
		{
			name:     "connection error is not retryable",
			err:      New(CodeConnection, "connection failed"),
			expected: false,
		},
		{
			name:     "duplicate error is not retryable",
			err:      New(CodeDuplicate, "duplicate key"),
			expected: false,
		},
		{
			name:     "timeout error is not retryable",
			err:      New(CodeTimeout, "query timeout"),
			expected: false,
		},
		{
			name:     "wrapped serialization error is retryable",
			err:      Wrap(CodeSerialization, "outer message", New(CodeInternal, "inner")),
			expected: true,
		},
		{
			name:     "nil error is not retryable",
			err:      nil,
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			result := isRetryable(tt.err)
			if result != tt.expected {
				t.Errorf("isRetryable(%v) = %v, want %v", tt.err, result, tt.expected)
			}
		})
	}
}
