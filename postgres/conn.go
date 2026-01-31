// Package postgres provides PostgreSQL database utilities for the Txova platform.
package postgres

import (
	"context"
	"time"

	"github.com/Dorico-Dynamics/txova-go-core/logging"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// pgxConn wraps pgxpool.Conn to implement the Conn interface.
type pgxConn struct {
	conn               *pgxpool.Conn
	logger             *logging.Logger
	slowQueryThreshold time.Duration
}

// Exec executes a query that doesn't return rows.
func (c *pgxConn) Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
	start := time.Now()
	tag, err := c.conn.Exec(ctx, sql, args...)
	duration := time.Since(start)

	c.logSlowQuery(ctx, sql, duration)

	if err != nil {
		c.logger.ErrorContext(ctx, "query execution failed",
			"sql", truncateSQL(sql),
			"duration_ms", duration.Milliseconds(),
			"error", err.Error(),
		)
		return tag, FromPgError(err)
	}
	return tag, nil
}

// Query executes a query that returns rows.
func (c *pgxConn) Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
	start := time.Now()
	rows, err := c.conn.Query(ctx, sql, args...)
	duration := time.Since(start)

	c.logSlowQuery(ctx, sql, duration)

	if err != nil {
		c.logger.ErrorContext(ctx, "query execution failed",
			"sql", truncateSQL(sql),
			"duration_ms", duration.Milliseconds(),
			"error", err.Error(),
		)
		return nil, FromPgError(err)
	}
	return rows, nil
}

// QueryRow executes a query that is expected to return at most one row.
func (c *pgxConn) QueryRow(ctx context.Context, sql string, args ...any) pgx.Row {
	start := time.Now()
	row := c.conn.QueryRow(ctx, sql, args...)
	duration := time.Since(start)

	c.logSlowQuery(ctx, sql, duration)

	return row
}

// logSlowQuery logs a warning if the query duration exceeds the threshold.
func (c *pgxConn) logSlowQuery(ctx context.Context, sql string, duration time.Duration) {
	if c.slowQueryThreshold > 0 && duration >= c.slowQueryThreshold {
		c.logger.WarnContext(ctx, "slow query detected",
			"sql", truncateSQL(sql),
			"duration_ms", duration.Milliseconds(),
			"threshold_ms", c.slowQueryThreshold.Milliseconds(),
		)
	}
}

// Begin starts a transaction on this connection.
func (c *pgxConn) Begin(ctx context.Context) (Tx, error) {
	tx, err := c.conn.Begin(ctx)
	if err != nil {
		return nil, Wrap(CodeConnection, "failed to begin transaction", err)
	}
	return &pgxTx{tx: tx, logger: c.logger, slowQueryThreshold: c.slowQueryThreshold}, nil
}

// BeginTx starts a transaction with the specified options.
func (c *pgxConn) BeginTx(ctx context.Context, txOptions pgx.TxOptions) (Tx, error) {
	tx, err := c.conn.BeginTx(ctx, txOptions)
	if err != nil {
		return nil, Wrap(CodeConnection, "failed to begin transaction", err)
	}
	return &pgxTx{tx: tx, logger: c.logger, slowQueryThreshold: c.slowQueryThreshold}, nil
}

// Ping verifies the connection is alive.
func (c *pgxConn) Ping(ctx context.Context) error {
	if err := c.conn.Ping(ctx); err != nil {
		return Wrap(CodeConnection, "ping failed", err)
	}
	return nil
}

// Release returns the connection to the pool.
func (c *pgxConn) Release() {
	c.conn.Release()
}
