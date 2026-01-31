package postgres

import (
	"bytes"
	"context"
	"log/slog"
	"testing"
	"time"

	"github.com/Dorico-Dynamics/txova-go-core/logging"
)

func TestPgxConn_logSlowQuery(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		threshold     time.Duration
		duration      time.Duration
		expectWarning bool
	}{
		{
			name:          "slow query logged",
			threshold:     100 * time.Millisecond,
			duration:      200 * time.Millisecond,
			expectWarning: true,
		},
		{
			name:          "fast query not logged",
			threshold:     100 * time.Millisecond,
			duration:      50 * time.Millisecond,
			expectWarning: false,
		},
		{
			name:          "threshold zero disables logging",
			threshold:     0,
			duration:      200 * time.Millisecond,
			expectWarning: false,
		},
		{
			name:          "exactly at threshold logged",
			threshold:     100 * time.Millisecond,
			duration:      100 * time.Millisecond,
			expectWarning: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var buf bytes.Buffer
			logger := logging.New(logging.Config{
				Level:       slog.LevelDebug,
				Format:      "json",
				ServiceName: "test",
				Output:      &buf,
			})

			conn := &pgxConn{
				conn:               nil, // We're not using the actual connection
				logger:             logger,
				slowQueryThreshold: tt.threshold,
			}

			ctx := context.Background()
			conn.logSlowQuery(ctx, "SELECT * FROM users", tt.duration)

			logOutput := buf.String()
			hasWarning := logOutput != "" && contains(logOutput, "slow query detected")

			if hasWarning != tt.expectWarning {
				t.Errorf("logSlowQuery() warning = %v, want %v, output: %s", hasWarning, tt.expectWarning, logOutput)
			}
		})
	}
}

func TestPgxTx_logSlowQuery(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		threshold     time.Duration
		duration      time.Duration
		expectWarning bool
	}{
		{
			name:          "slow query logged",
			threshold:     100 * time.Millisecond,
			duration:      200 * time.Millisecond,
			expectWarning: true,
		},
		{
			name:          "fast query not logged",
			threshold:     100 * time.Millisecond,
			duration:      50 * time.Millisecond,
			expectWarning: false,
		},
		{
			name:          "threshold zero disables logging",
			threshold:     0,
			duration:      200 * time.Millisecond,
			expectWarning: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var buf bytes.Buffer
			logger := logging.New(logging.Config{
				Level:       slog.LevelDebug,
				Format:      "json",
				ServiceName: "test",
				Output:      &buf,
			})

			tx := &pgxTx{
				tx:                 nil, // We're not using the actual transaction
				logger:             logger,
				slowQueryThreshold: tt.threshold,
			}

			ctx := context.Background()
			tx.logSlowQuery(ctx, "SELECT * FROM users", tt.duration)

			logOutput := buf.String()
			hasWarning := logOutput != "" && contains(logOutput, "slow query detected")

			if hasWarning != tt.expectWarning {
				t.Errorf("logSlowQuery() warning = %v, want %v, output: %s", hasWarning, tt.expectWarning, logOutput)
			}
		})
	}
}

func TestPgxPool_logSlowQuery(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	logger := logging.New(logging.Config{
		Level:       slog.LevelDebug,
		Format:      "json",
		ServiceName: "test",
		Output:      &buf,
	})

	pool := &pgxPool{
		pool: nil, // We're not using the actual pool
		config: PoolConfig{
			SlowQueryThreshold: 100 * time.Millisecond,
		},
		logger: logger,
	}

	ctx := context.Background()
	pool.logSlowQuery(ctx, "SELECT * FROM users", 200*time.Millisecond)

	logOutput := buf.String()
	if !contains(logOutput, "slow query detected") {
		t.Errorf("expected slow query warning, got: %s", logOutput)
	}
}

// contains checks if substr is in s.
func contains(s, substr string) bool {
	return bytes.Contains([]byte(s), []byte(substr))
}

func TestPoolStats_Fields(t *testing.T) {
	t.Parallel()

	stats := PoolStats{
		AcquireCount:            100,
		AcquireDuration:         5000,
		AcquiredConns:           10,
		CanceledAcquireCount:    2,
		ConstructingConns:       1,
		EmptyAcquireCount:       5,
		IdleConns:               15,
		MaxConns:                25,
		TotalConns:              20,
		NewConnsCount:           50,
		MaxLifetimeDestroyCount: 3,
		MaxIdleDestroyCount:     7,
	}

	if stats.AcquireCount != 100 {
		t.Errorf("AcquireCount = %d, want 100", stats.AcquireCount)
	}
	if stats.AcquireDuration != 5000 {
		t.Errorf("AcquireDuration = %d, want 5000", stats.AcquireDuration)
	}
	if stats.AcquiredConns != 10 {
		t.Errorf("AcquiredConns = %d, want 10", stats.AcquiredConns)
	}
	if stats.CanceledAcquireCount != 2 {
		t.Errorf("CanceledAcquireCount = %d, want 2", stats.CanceledAcquireCount)
	}
	if stats.ConstructingConns != 1 {
		t.Errorf("ConstructingConns = %d, want 1", stats.ConstructingConns)
	}
	if stats.EmptyAcquireCount != 5 {
		t.Errorf("EmptyAcquireCount = %d, want 5", stats.EmptyAcquireCount)
	}
	if stats.IdleConns != 15 {
		t.Errorf("IdleConns = %d, want 15", stats.IdleConns)
	}
	if stats.MaxConns != 25 {
		t.Errorf("MaxConns = %d, want 25", stats.MaxConns)
	}
	if stats.TotalConns != 20 {
		t.Errorf("TotalConns = %d, want 20", stats.TotalConns)
	}
	if stats.NewConnsCount != 50 {
		t.Errorf("NewConnsCount = %d, want 50", stats.NewConnsCount)
	}
	if stats.MaxLifetimeDestroyCount != 3 {
		t.Errorf("MaxLifetimeDestroyCount = %d, want 3", stats.MaxLifetimeDestroyCount)
	}
	if stats.MaxIdleDestroyCount != 7 {
		t.Errorf("MaxIdleDestroyCount = %d, want 7", stats.MaxIdleDestroyCount)
	}
}
