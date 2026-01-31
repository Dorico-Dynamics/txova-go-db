package postgres

import (
	"context"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// Querier is the common interface for Pool, Conn, and Tx that can execute queries.
// This interface enables writing database code that works transparently with
// connection pools, individual connections, and transactions.
type Querier interface {
	// Exec executes a query that doesn't return rows.
	// Examples include INSERT, UPDATE, DELETE statements.
	Exec(ctx context.Context, sql string, arguments ...any) (pgconn.CommandTag, error)

	// Query executes a query that returns rows, typically a SELECT.
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)

	// QueryRow executes a query that is expected to return at most one row.
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

// Pool represents a PostgreSQL connection pool.
// It is the primary interface for database operations in the application.
type Pool interface {
	Querier

	// Acquire returns a connection from the pool.
	// The connection must be released back to the pool when done.
	Acquire(ctx context.Context) (Conn, error)

	// Begin starts a transaction.
	Begin(ctx context.Context) (Tx, error)

	// BeginTx starts a transaction with the specified options.
	BeginTx(ctx context.Context, txOptions pgx.TxOptions) (Tx, error)

	// Ping verifies the database connection is alive.
	// Use this for health checks.
	Ping(ctx context.Context) error

	// Close closes all connections in the pool.
	// It waits for all connections to be returned to the pool.
	Close()

	// Stat returns the current pool statistics.
	Stat() PoolStats
}

// Conn represents a single database connection acquired from the pool.
// It must be released back to the pool when done using Release().
type Conn interface {
	Querier

	// Begin starts a transaction on this connection.
	Begin(ctx context.Context) (Tx, error)

	// BeginTx starts a transaction with the specified options.
	BeginTx(ctx context.Context, txOptions pgx.TxOptions) (Tx, error)

	// Ping verifies the connection is alive.
	Ping(ctx context.Context) error

	// Release returns the connection to the pool.
	Release()
}

// Tx represents a database transaction.
// All operations on a transaction are atomic and will be committed or rolled back together.
type Tx interface {
	Querier

	// Begin starts a pseudo-nested transaction using a savepoint.
	Begin(ctx context.Context) (Tx, error)

	// Commit commits the transaction.
	// Returns ErrTxClosed if already closed.
	Commit(ctx context.Context) error

	// Rollback rolls back the transaction.
	// Safe to call multiple times or after commit (will return ErrTxClosed).
	Rollback(ctx context.Context) error

	// Conn returns the underlying connection.
	Conn() *pgx.Conn
}

// PoolStats contains statistics about the connection pool.
type PoolStats struct {
	// AcquireCount is the cumulative count of successful acquires from the pool.
	AcquireCount int64

	// AcquireDuration is the total duration of all successful acquires.
	AcquireDuration int64

	// AcquiredConns is the number of currently acquired connections.
	AcquiredConns int32

	// CanceledAcquireCount is the cumulative count of canceled acquires.
	CanceledAcquireCount int64

	// ConstructingConns is the number of connections currently being constructed.
	ConstructingConns int32

	// EmptyAcquireCount is the number of times the pool was empty when Acquire was called.
	EmptyAcquireCount int64

	// IdleConns is the number of currently idle connections.
	IdleConns int32

	// MaxConns is the maximum number of connections allowed.
	MaxConns int32

	// TotalConns is the total number of connections in the pool.
	TotalConns int32

	// NewConnsCount is the cumulative count of new connections created.
	NewConnsCount int64

	// MaxLifetimeDestroyCount is connections destroyed due to max lifetime.
	MaxLifetimeDestroyCount int64

	// MaxIdleDestroyCount is connections destroyed due to max idle time.
	MaxIdleDestroyCount int64
}

// TxManager manages database transactions with automatic commit/rollback.
type TxManager interface {
	// WithTx executes fn within a transaction.
	// If fn returns nil, the transaction is committed.
	// If fn returns an error or panics, the transaction is rolled back.
	WithTx(ctx context.Context, fn func(tx Tx) error) error

	// WithTxOptions executes fn within a transaction with the specified options.
	WithTxOptions(ctx context.Context, opts pgx.TxOptions, fn func(tx Tx) error) error
}

// Scanner is implemented by types that can scan database rows.
// This is compatible with pgx.Row and pgx.Rows.
type Scanner interface {
	Scan(dest ...any) error
}

