// Package postgres provides PostgreSQL database utilities for the Txova platform.
package postgres

import (
	"context"
	"errors"
	"math/rand/v2"
	"time"

	"github.com/Dorico-Dynamics/txova-go-core/logging"
	"github.com/jackc/pgx/v5"
)

// txContextKey is the context key for storing the active transaction.
type txContextKey struct{}

// TxFromContext retrieves the active transaction from the context.
// Returns the transaction and true if found, nil and false otherwise.
func TxFromContext(ctx context.Context) (Tx, bool) {
	tx, ok := ctx.Value(txContextKey{}).(Tx)
	return tx, ok
}

// ContextWithTx returns a new context with the transaction stored in it.
func ContextWithTx(ctx context.Context, tx Tx) context.Context {
	return context.WithValue(ctx, txContextKey{}, tx)
}

// TxManagerConfig holds configuration for the TxManager.
type TxManagerConfig struct {
	// MaxRetries is the maximum number of retry attempts for retryable errors.
	// Default: 3.
	MaxRetries int

	// RetryBaseDelay is the base delay between retry attempts.
	// Actual delay uses exponential backoff with jitter.
	// Default: 50ms.
	RetryBaseDelay time.Duration

	// RetryMaxDelay is the maximum delay between retry attempts.
	// Default: 2s.
	RetryMaxDelay time.Duration

	// Logger for transaction events.
	Logger *logging.Logger
}

// DefaultTxManagerConfig returns a TxManagerConfig with sensible defaults.
func DefaultTxManagerConfig() TxManagerConfig {
	return TxManagerConfig{
		MaxRetries:     3,
		RetryBaseDelay: 50 * time.Millisecond,
		RetryMaxDelay:  2 * time.Second,
		Logger:         logging.Default(),
	}
}

// TxManagerOption is a functional option for configuring a TxManager.
type TxManagerOption func(*TxManagerConfig)

// WithMaxRetries sets the maximum number of retry attempts.
func WithMaxRetries(n int) TxManagerOption {
	return func(c *TxManagerConfig) {
		c.MaxRetries = n
	}
}

// WithRetryBaseDelay sets the base delay between retry attempts.
func WithRetryBaseDelay(d time.Duration) TxManagerOption {
	return func(c *TxManagerConfig) {
		c.RetryBaseDelay = d
	}
}

// WithRetryMaxDelay sets the maximum delay between retry attempts.
func WithRetryMaxDelay(d time.Duration) TxManagerOption {
	return func(c *TxManagerConfig) {
		c.RetryMaxDelay = d
	}
}

// WithTxLogger sets the logger for transaction events.
func WithTxLogger(logger *logging.Logger) TxManagerOption {
	return func(c *TxManagerConfig) {
		c.Logger = logger
	}
}

// txManager implements the TxManager interface.
type txManager struct {
	pool   Pool
	config TxManagerConfig
}

// NewTxManager creates a new TxManager with the given pool and options.
func NewTxManager(pool Pool, opts ...TxManagerOption) TxManager {
	cfg := DefaultTxManagerConfig()
	for _, opt := range opts {
		opt(&cfg)
	}
	return &txManager{pool: pool, config: cfg}
}

// WithTx executes fn within a transaction using default options.
// If a transaction already exists in the context, it uses the existing transaction.
// If fn returns nil, the transaction is committed.
// If fn returns an error or panics, the transaction is rolled back.
func (m *txManager) WithTx(ctx context.Context, fn func(tx Tx) error) error {
	return m.WithTxOptions(ctx, pgx.TxOptions{}, fn)
}

// WithTxOptions executes fn within a transaction with the specified options.
// If a transaction already exists in the context, it uses the existing transaction
// (options are ignored for existing transactions).
// If fn returns nil, the transaction is committed.
// If fn returns an error or panics, the transaction is rolled back.
// Serialization failures and deadlocks are automatically retried.
func (m *txManager) WithTxOptions(ctx context.Context, opts pgx.TxOptions, fn func(tx Tx) error) error {
	// Check if there's an existing transaction in the context.
	if existingTx, ok := TxFromContext(ctx); ok {
		// Use the existing transaction (no commit/rollback, caller controls it).
		return fn(existingTx)
	}

	// Execute with retry for retryable errors.
	return m.executeWithRetry(ctx, opts, fn)
}

// executeWithRetry executes the transaction function with retry logic.
func (m *txManager) executeWithRetry(ctx context.Context, opts pgx.TxOptions, fn func(tx Tx) error) error {
	var lastErr error

	for attempt := 0; attempt <= m.config.MaxRetries; attempt++ {
		if attempt > 0 {
			// Wait before retry with exponential backoff and jitter.
			delay := m.calculateRetryDelay(attempt)
			m.config.Logger.InfoContext(ctx, "retrying transaction",
				"attempt", attempt+1,
				"max_retries", m.config.MaxRetries+1,
				"delay_ms", delay.Milliseconds(),
			)

			select {
			case <-ctx.Done():
				return Wrap(CodeTimeout, "context cancelled during retry", ctx.Err())
			case <-time.After(delay):
			}
		}

		err := m.executeTx(ctx, opts, fn)
		if err == nil {
			return nil
		}

		lastErr = err

		// Only retry on serialization failures or deadlocks.
		if !isRetryable(err) {
			return err
		}

		m.config.Logger.WarnContext(ctx, "retryable transaction error",
			"attempt", attempt+1,
			"error", err.Error(),
		)
	}

	return Wrap(CodeSerialization, "transaction failed after max retries", lastErr)
}

// executeTx executes a single transaction attempt.
func (m *txManager) executeTx(ctx context.Context, opts pgx.TxOptions, fn func(tx Tx) error) (err error) {
	tx, err := m.pool.BeginTx(ctx, opts)
	if err != nil {
		return err
	}

	// Store transaction in context for nested access.
	txCtx := ContextWithTx(ctx, tx)

	// Handle panics - rollback and re-panic.
	defer func() {
		if r := recover(); r != nil {
			// Attempt rollback, ignore error since we're panicking anyway.
			_ = tx.Rollback(ctx) //nolint:errcheck // Best-effort rollback before re-panic.
			m.config.Logger.ErrorContext(ctx, "transaction panic, rolled back",
				"panic", r,
			)
			panic(r)
		}
	}()

	// Execute the function.
	if err = fn(tx); err != nil {
		// Rollback on error.
		if rbErr := tx.Rollback(ctx); rbErr != nil {
			// Log rollback error but return original error.
			m.config.Logger.ErrorContext(ctx, "rollback failed",
				"original_error", err.Error(),
				"rollback_error", rbErr.Error(),
			)
		}
		return err
	}

	// Commit on success.
	if err = tx.Commit(txCtx); err != nil {
		return err
	}

	return nil
}

// calculateRetryDelay calculates the delay before the next retry attempt.
// Uses exponential backoff with jitter.
func (m *txManager) calculateRetryDelay(attempt int) time.Duration {
	// Exponential backoff: baseDelay * 2^(attempt-1)
	delay := m.config.RetryBaseDelay * time.Duration(1<<uint(attempt-1))

	// Cap at max delay.
	if delay > m.config.RetryMaxDelay {
		delay = m.config.RetryMaxDelay
	}

	// Add jitter (±25%).
	jitter := time.Duration(rand.Int64N(int64(delay) / 2))
	delay = delay - delay/4 + jitter

	return delay
}

// isRetryable checks if the error is retryable (serialization failure or deadlock).
func isRetryable(err error) bool {
	var dbErr *Error
	if errors.As(err, &dbErr) {
		code := dbErr.Code()
		return code == CodeSerialization || code == CodeDeadlock
	}
	return false
}
