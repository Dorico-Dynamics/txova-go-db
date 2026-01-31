// Package postgres provides PostgreSQL database utilities for the Txova platform.
package postgres

import (
	"context"
	"time"

	"github.com/Dorico-Dynamics/txova-go-core/logging"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// pgxTx wraps pgx.Tx to implement the Tx interface.
type pgxTx struct {
	tx                 pgx.Tx
	logger             *logging.Logger
	slowQueryThreshold time.Duration
}

// Exec executes a query that doesn't return rows.
func (t *pgxTx) Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
	start := time.Now()
	tag, err := t.tx.Exec(ctx, sql, args...)
	duration := time.Since(start)

	t.logSlowQuery(ctx, sql, duration)

	if err != nil {
		t.logger.ErrorContext(ctx, "query execution failed",
			"sql", truncateSQL(sql),
			"duration_ms", duration.Milliseconds(),
			"error", err.Error(),
		)
		return tag, FromPgError(err)
	}
	return tag, nil
}

// Query executes a query that returns rows.
func (t *pgxTx) Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
	start := time.Now()
	rows, err := t.tx.Query(ctx, sql, args...)
	duration := time.Since(start)

	t.logSlowQuery(ctx, sql, duration)

	if err != nil {
		t.logger.ErrorContext(ctx, "query execution failed",
			"sql", truncateSQL(sql),
			"duration_ms", duration.Milliseconds(),
			"error", err.Error(),
		)
		return nil, FromPgError(err)
	}
	return rows, nil
}

// QueryRow executes a query that is expected to return at most one row.
func (t *pgxTx) QueryRow(ctx context.Context, sql string, args ...any) pgx.Row {
	start := time.Now()
	row := t.tx.QueryRow(ctx, sql, args...)
	duration := time.Since(start)

	t.logSlowQuery(ctx, sql, duration)

	return row
}

// logSlowQuery logs a warning if the query duration exceeds the threshold.
func (t *pgxTx) logSlowQuery(ctx context.Context, sql string, duration time.Duration) {
	if t.slowQueryThreshold > 0 && duration >= t.slowQueryThreshold {
		t.logger.WarnContext(ctx, "slow query detected",
			"sql", truncateSQL(sql),
			"duration_ms", duration.Milliseconds(),
			"threshold_ms", t.slowQueryThreshold.Milliseconds(),
		)
	}
}

// Begin starts a pseudo-nested transaction using a savepoint.
func (t *pgxTx) Begin(ctx context.Context) (Tx, error) {
	nestedTx, err := t.tx.Begin(ctx)
	if err != nil {
		return nil, Wrap(CodeConnection, "failed to begin nested transaction", err)
	}
	return &pgxTx{tx: nestedTx, logger: t.logger, slowQueryThreshold: t.slowQueryThreshold}, nil
}

// Commit commits the transaction.
func (t *pgxTx) Commit(ctx context.Context) error {
	if err := t.tx.Commit(ctx); err != nil {
		return Wrap(CodeConnection, "failed to commit transaction", err)
	}
	return nil
}

// Rollback rolls back the transaction.
func (t *pgxTx) Rollback(ctx context.Context) error {
	if err := t.tx.Rollback(ctx); err != nil {
		return Wrap(CodeConnection, "failed to rollback transaction", err)
	}
	return nil
}

// Conn returns the underlying connection.
func (t *pgxTx) Conn() *pgx.Conn {
	return t.tx.Conn()
}
