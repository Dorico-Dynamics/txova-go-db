// Package postgres provides PostgreSQL database utilities for the Txova platform.
package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/Dorico-Dynamics/txova-go-core/config"
	"github.com/Dorico-Dynamics/txova-go-core/logging"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// PoolConfig holds the configuration for a PostgreSQL connection pool.
// It integrates with txova-go-core/config.DatabaseConfig for configuration loading
// and txova-go-core/logging.Logger for structured logging.
type PoolConfig struct {
	// ConnString is the PostgreSQL connection string (DSN).
	// Use FromDatabaseConfig to create from txova-go-core/config.DatabaseConfig.
	ConnString string

	// MaxConns is the maximum number of connections in the pool.
	// Default: 25 (from config.DatabaseConfig.MaxConnections).
	MaxConns int32

	// MinConns is the minimum number of connections in the pool.
	// Default: 5.
	MinConns int32

	// MaxConnLifetime is the maximum lifetime of a connection.
	// Default: 1 hour.
	MaxConnLifetime time.Duration

	// MaxConnIdleTime is the maximum idle time before a connection is closed.
	// Default: 30 minutes.
	MaxConnIdleTime time.Duration

	// HealthCheckPeriod is the interval between health checks.
	// Default: 1 minute.
	HealthCheckPeriod time.Duration

	// ConnectTimeout is the timeout for establishing a new connection.
	// Default: 5 seconds.
	ConnectTimeout time.Duration

	// SlowQueryThreshold is the duration after which a query is considered slow.
	// Queries exceeding this threshold are logged as warnings.
	// Default: 1 second. Set to 0 to disable slow query logging.
	SlowQueryThreshold time.Duration

	// Logger is the logger for connection pool events.
	// If nil, a default logger is used.
	Logger *logging.Logger
}

// DefaultPoolConfig returns a PoolConfig with sensible defaults.
func DefaultPoolConfig() PoolConfig {
	return PoolConfig{
		MaxConns:           25,
		MinConns:           5,
		MaxConnLifetime:    time.Hour,
		MaxConnIdleTime:    30 * time.Minute,
		HealthCheckPeriod:  time.Minute,
		ConnectTimeout:     5 * time.Second,
		SlowQueryThreshold: time.Second,
		Logger:             logging.Default(),
	}
}

// Option is a functional option for configuring a PoolConfig.
type Option func(*PoolConfig)

// WithConnString sets the connection string.
func WithConnString(connString string) Option {
	return func(c *PoolConfig) {
		c.ConnString = connString
	}
}

// WithMaxConns sets the maximum number of connections.
func WithMaxConns(n int32) Option {
	return func(c *PoolConfig) {
		c.MaxConns = n
	}
}

// WithMinConns sets the minimum number of connections.
func WithMinConns(n int32) Option {
	return func(c *PoolConfig) {
		c.MinConns = n
	}
}

// WithMaxConnLifetime sets the maximum connection lifetime.
func WithMaxConnLifetime(d time.Duration) Option {
	return func(c *PoolConfig) {
		c.MaxConnLifetime = d
	}
}

// WithMaxConnIdleTime sets the maximum connection idle time.
func WithMaxConnIdleTime(d time.Duration) Option {
	return func(c *PoolConfig) {
		c.MaxConnIdleTime = d
	}
}

// WithHealthCheckPeriod sets the health check interval.
func WithHealthCheckPeriod(d time.Duration) Option {
	return func(c *PoolConfig) {
		c.HealthCheckPeriod = d
	}
}

// WithConnectTimeout sets the connection timeout.
func WithConnectTimeout(d time.Duration) Option {
	return func(c *PoolConfig) {
		c.ConnectTimeout = d
	}
}

// WithSlowQueryThreshold sets the slow query threshold.
// Queries exceeding this threshold are logged as warnings.
func WithSlowQueryThreshold(d time.Duration) Option {
	return func(c *PoolConfig) {
		c.SlowQueryThreshold = d
	}
}

// WithLogger sets the logger for pool events.
func WithLogger(logger *logging.Logger) Option {
	return func(c *PoolConfig) {
		c.Logger = logger
	}
}

// FromDatabaseConfig creates a PoolConfig from txova-go-core/config.DatabaseConfig.
// This enables seamless integration with the core configuration system.
func FromDatabaseConfig(dbCfg *config.DatabaseConfig, opts ...Option) PoolConfig {
	cfg := DefaultPoolConfig()
	cfg.ConnString = dbCfg.DSN()
	cfg.MaxConns = int32(dbCfg.MaxConnections)

	for _, opt := range opts {
		opt(&cfg)
	}

	return cfg
}

// Validate validates the pool configuration.
func (c *PoolConfig) Validate() error {
	if c.ConnString == "" {
		return fmt.Errorf("connection string is required")
	}
	if c.MaxConns < 1 {
		return fmt.Errorf("max connections must be at least 1")
	}
	if c.MinConns < 0 {
		return fmt.Errorf("min connections cannot be negative")
	}
	if c.MinConns > c.MaxConns {
		return fmt.Errorf("min connections (%d) cannot exceed max connections (%d)", c.MinConns, c.MaxConns)
	}
	return nil
}

// pgxPool wraps pgxpool.Pool to implement the Pool interface.
type pgxPool struct {
	pool   *pgxpool.Pool
	config PoolConfig
	logger *logging.Logger
}

// NewPool creates a new PostgreSQL connection pool.
// It validates the configuration and establishes the connection.
func NewPool(ctx context.Context, opts ...Option) (Pool, error) {
	cfg := DefaultPoolConfig()
	for _, opt := range opts {
		opt(&cfg)
	}

	if err := cfg.Validate(); err != nil {
		return nil, Wrap(CodeConnection, "invalid pool configuration", err)
	}

	return newPoolFromConfig(ctx, cfg)
}

// NewPoolFromConfig creates a new PostgreSQL connection pool from a PoolConfig.
func NewPoolFromConfig(ctx context.Context, cfg PoolConfig) (Pool, error) {
	if err := cfg.Validate(); err != nil {
		return nil, Wrap(CodeConnection, "invalid pool configuration", err)
	}

	return newPoolFromConfig(ctx, cfg)
}

func newPoolFromConfig(ctx context.Context, cfg PoolConfig) (Pool, error) {
	logger := cfg.Logger
	if logger == nil {
		logger = logging.Default()
	}

	// Parse the connection string into pgxpool.Config.
	poolCfg, err := pgxpool.ParseConfig(cfg.ConnString)
	if err != nil {
		return nil, Wrap(CodeConnection, "failed to parse connection string", err)
	}

	// Apply pool configuration.
	poolCfg.MaxConns = cfg.MaxConns
	poolCfg.MinConns = cfg.MinConns
	poolCfg.MaxConnLifetime = cfg.MaxConnLifetime
	poolCfg.MaxConnIdleTime = cfg.MaxConnIdleTime
	poolCfg.HealthCheckPeriod = cfg.HealthCheckPeriod

	// Set connect timeout on the underlying connection config.
	if cfg.ConnectTimeout > 0 {
		poolCfg.ConnConfig.ConnectTimeout = cfg.ConnectTimeout
	}

	logger.Info("creating PostgreSQL connection pool",
		"max_conns", cfg.MaxConns,
		"min_conns", cfg.MinConns,
		"max_conn_lifetime", cfg.MaxConnLifetime.String(),
		"max_conn_idle_time", cfg.MaxConnIdleTime.String(),
		"health_check_period", cfg.HealthCheckPeriod.String(),
	)

	// Create the pool.
	pool, err := pgxpool.NewWithConfig(ctx, poolCfg)
	if err != nil {
		return nil, Wrap(CodeConnection, "failed to create connection pool", err)
	}

	logger.Info("PostgreSQL connection pool created successfully")

	return &pgxPool{
		pool:   pool,
		config: cfg,
		logger: logger,
	}, nil
}

// Exec executes a query that doesn't return rows.
func (p *pgxPool) Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
	start := time.Now()
	tag, err := p.pool.Exec(ctx, sql, args...)
	duration := time.Since(start)

	p.logSlowQuery(ctx, sql, duration)

	if err != nil {
		p.logger.ErrorContext(ctx, "query execution failed",
			"sql", truncateSQL(sql),
			"duration_ms", duration.Milliseconds(),
			"error", err.Error(),
		)
		return tag, FromPgError(err)
	}
	return tag, nil
}

// Query executes a query that returns rows.
func (p *pgxPool) Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
	start := time.Now()
	rows, err := p.pool.Query(ctx, sql, args...)
	duration := time.Since(start)

	p.logSlowQuery(ctx, sql, duration)

	if err != nil {
		p.logger.ErrorContext(ctx, "query execution failed",
			"sql", truncateSQL(sql),
			"duration_ms", duration.Milliseconds(),
			"error", err.Error(),
		)
		return nil, FromPgError(err)
	}
	return rows, nil
}

// QueryRow executes a query that is expected to return at most one row.
func (p *pgxPool) QueryRow(ctx context.Context, sql string, args ...any) pgx.Row {
	start := time.Now()
	row := p.pool.QueryRow(ctx, sql, args...)
	duration := time.Since(start)

	p.logSlowQuery(ctx, sql, duration)

	return row
}

// logSlowQuery logs a warning if the query duration exceeds the threshold.
func (p *pgxPool) logSlowQuery(ctx context.Context, sql string, duration time.Duration) {
	if p.config.SlowQueryThreshold > 0 && duration >= p.config.SlowQueryThreshold {
		p.logger.WarnContext(ctx, "slow query detected",
			"sql", truncateSQL(sql),
			"duration_ms", duration.Milliseconds(),
			"threshold_ms", p.config.SlowQueryThreshold.Milliseconds(),
		)
	}
}

// truncateSQL truncates SQL for logging to prevent overly long log messages.
func truncateSQL(sql string) string {
	const maxLen = 200
	if len(sql) <= maxLen {
		return sql
	}
	return sql[:maxLen] + "..."
}

// Acquire returns a connection from the pool.
func (p *pgxPool) Acquire(ctx context.Context) (Conn, error) {
	conn, err := p.pool.Acquire(ctx)
	if err != nil {
		return nil, Wrap(CodeConnection, "failed to acquire connection", err)
	}
	return &pgxConn{conn: conn, logger: p.logger, slowQueryThreshold: p.config.SlowQueryThreshold}, nil
}

// Begin starts a transaction.
func (p *pgxPool) Begin(ctx context.Context) (Tx, error) {
	tx, err := p.pool.Begin(ctx)
	if err != nil {
		return nil, Wrap(CodeConnection, "failed to begin transaction", err)
	}
	return &pgxTx{tx: tx, logger: p.logger, slowQueryThreshold: p.config.SlowQueryThreshold}, nil
}

// BeginTx starts a transaction with the specified options.
func (p *pgxPool) BeginTx(ctx context.Context, txOptions pgx.TxOptions) (Tx, error) {
	tx, err := p.pool.BeginTx(ctx, txOptions)
	if err != nil {
		return nil, Wrap(CodeConnection, "failed to begin transaction", err)
	}
	return &pgxTx{tx: tx, logger: p.logger, slowQueryThreshold: p.config.SlowQueryThreshold}, nil
}

// Ping verifies the database connection is alive.
func (p *pgxPool) Ping(ctx context.Context) error {
	if err := p.pool.Ping(ctx); err != nil {
		return Wrap(CodeConnection, "ping failed", err)
	}
	return nil
}

// Close closes all connections in the pool.
func (p *pgxPool) Close() {
	p.logger.Info("closing PostgreSQL connection pool")
	p.pool.Close()
}

// Stat returns the current pool statistics.
func (p *pgxPool) Stat() PoolStats {
	stat := p.pool.Stat()
	return PoolStats{
		AcquireCount:            stat.AcquireCount(),
		AcquireDuration:         int64(stat.AcquireDuration()),
		AcquiredConns:           stat.AcquiredConns(),
		CanceledAcquireCount:    stat.CanceledAcquireCount(),
		ConstructingConns:       stat.ConstructingConns(),
		EmptyAcquireCount:       stat.EmptyAcquireCount(),
		IdleConns:               stat.IdleConns(),
		MaxConns:                stat.MaxConns(),
		TotalConns:              stat.TotalConns(),
		NewConnsCount:           stat.NewConnsCount(),
		MaxLifetimeDestroyCount: stat.MaxLifetimeDestroyCount(),
		MaxIdleDestroyCount:     stat.MaxIdleDestroyCount(),
	}
}
