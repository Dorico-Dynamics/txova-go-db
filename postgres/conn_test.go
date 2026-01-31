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
			hasWarning := len(logOutput) > 0 && contains(logOutput, "slow query detected")

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
			hasWarning := len(logOutput) > 0 && contains(logOutput, "slow query detected")

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

// contains checks if substr is in s
func contains(s, substr string) bool {
	return bytes.Contains([]byte(s), []byte(substr))
}
