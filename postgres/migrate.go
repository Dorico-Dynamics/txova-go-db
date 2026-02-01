// Package postgres provides PostgreSQL database utilities including migrations.
package postgres

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"time"

	"github.com/Dorico-Dynamics/txova-go-core/logging"
	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/database/pgx/v5"
	"github.com/golang-migrate/migrate/v4/source/iofs"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"
)

// MigratorConfig holds configuration for the migration runner.
type MigratorConfig struct {
	// TableName is the name of the schema migrations table.
	// Default: "schema_migrations"
	TableName string

	// LockTimeout is the timeout for acquiring the migration lock.
	// Default: 15 seconds
	LockTimeout time.Duration

	// MigrationsPath is the path within the fs.FS where migration files are located.
	// Default: "." (root of the filesystem)
	MigrationsPath string

	// Logger is the logger for migration events.
	Logger *logging.Logger
}

// DefaultMigratorConfig returns a default configuration.
func DefaultMigratorConfig() MigratorConfig {
	return MigratorConfig{
		TableName:      "schema_migrations",
		LockTimeout:    15 * time.Second,
		MigrationsPath: ".",
		Logger:         nil,
	}
}

// MigratorOption is a functional option for configuring the Migrator.
type MigratorOption func(*MigratorConfig)

// WithMigrationsTable sets the migrations table name.
func WithMigrationsTable(name string) MigratorOption {
	return func(c *MigratorConfig) {
		c.TableName = name
	}
}

// WithLockTimeout sets the lock timeout for migrations.
func WithLockTimeout(timeout time.Duration) MigratorOption {
	return func(c *MigratorConfig) {
		c.LockTimeout = timeout
	}
}

// WithMigratorLogger sets the logger for migration events.
func WithMigratorLogger(logger *logging.Logger) MigratorOption {
	return func(c *MigratorConfig) {
		c.Logger = logger
	}
}

// WithMigrationsPath sets the path within the fs.FS where migration files are located.
func WithMigrationsPath(path string) MigratorOption {
	return func(c *MigratorConfig) {
		c.MigrationsPath = path
	}
}

// Migrator handles database migrations.
type Migrator struct {
	config   MigratorConfig
	pool     *pgxpool.Pool
	migrate  *migrate.Migrate
	sourceFS fs.FS
}

// NewMigrator creates a new Migrator with the given pool and migration source.
// The migrations parameter should be an embed.FS or any fs.FS containing migration files.
// Migration files should follow the naming convention: NNNN_description.up.sql and NNNN_description.down.sql.
func NewMigrator(pool *pgxpool.Pool, migrations fs.FS, opts ...MigratorOption) (*Migrator, error) {
	if pool == nil {
		return nil, fmt.Errorf("pool cannot be nil")
	}
	if migrations == nil {
		return nil, fmt.Errorf("migrations filesystem cannot be nil")
	}

	config := DefaultMigratorConfig()
	for _, opt := range opts {
		opt(&config)
	}

	// Create source driver from fs.FS
	sourceDriver, err := iofs.New(migrations, config.MigrationsPath)
	if err != nil {
		return nil, fmt.Errorf("creating migration source: %w", err)
	}

	// Create database driver using stdlib adapter
	// pgx/v5 driver requires a *sql.DB, so we use stdlib.OpenDBFromPool
	db := stdlib.OpenDBFromPool(pool)

	dbDriver, err := pgx.WithInstance(db, &pgx.Config{
		MigrationsTable: config.TableName,
	})
	if err != nil {
		return nil, fmt.Errorf("creating database driver: %w", err)
	}

	// Create migrate instance
	m, err := migrate.NewWithInstance("iofs", sourceDriver, "pgx5", dbDriver)
	if err != nil {
		return nil, fmt.Errorf("creating migrate instance: %w", err)
	}

	return &Migrator{
		config:   config,
		pool:     pool,
		migrate:  m,
		sourceFS: migrations,
	}, nil
}

// Up applies all pending migrations.
func (m *Migrator) Up(ctx context.Context) error {
	start := time.Now()
	m.log("starting up migrations")

	err := m.migrate.Up()
	if err != nil && !errors.Is(err, migrate.ErrNoChange) {
		m.logError("up migration failed", err)
		return fmt.Errorf("running up migrations: %w", err)
	}

	if errors.Is(err, migrate.ErrNoChange) {
		m.log("no pending migrations")
		return nil
	}

	m.log("up migrations completed", "duration_ms", time.Since(start).Milliseconds())
	return nil
}

// Down rolls back all migrations.
func (m *Migrator) Down(ctx context.Context) error {
	start := time.Now()
	m.log("starting down migrations")

	err := m.migrate.Down()
	if err != nil && !errors.Is(err, migrate.ErrNoChange) {
		m.logError("down migration failed", err)
		return fmt.Errorf("running down migrations: %w", err)
	}

	if errors.Is(err, migrate.ErrNoChange) {
		m.log("no migrations to rollback")
		return nil
	}

	m.log("down migrations completed", "duration_ms", time.Since(start).Milliseconds())
	return nil
}

// Steps applies n migrations. If n > 0, applies n up migrations.
// If n < 0, applies n down migrations (rollback).
func (m *Migrator) Steps(ctx context.Context, n int) error {
	if n == 0 {
		return nil
	}

	start := time.Now()
	direction := "up"
	if n < 0 {
		direction = "down"
	}
	m.log("applying migration steps", "n", n, "direction", direction)

	err := m.migrate.Steps(n)
	if err != nil && !errors.Is(err, migrate.ErrNoChange) {
		m.logError("migration steps failed", err)
		return fmt.Errorf("applying %d migration steps: %w", n, err)
	}

	if errors.Is(err, migrate.ErrNoChange) {
		m.log("no migrations to apply")
		return nil
	}

	m.log("migration steps completed", "n", n, "duration_ms", time.Since(start).Milliseconds())
	return nil
}

// Version returns the current migration version.
// Returns the version number, whether the database is dirty (failed migration), and any error.
func (m *Migrator) Version() (uint, bool, error) {
	version, dirty, err := m.migrate.Version()
	if err != nil && !errors.Is(err, migrate.ErrNilVersion) {
		return 0, false, fmt.Errorf("getting migration version: %w", err)
	}

	if errors.Is(err, migrate.ErrNilVersion) {
		return 0, false, nil
	}

	return version, dirty, nil
}

// Force sets the migration version without running migrations.
// This is useful for recovering from a failed migration.
// Use with caution - this does not run any migration files.
func (m *Migrator) Force(version int) error {
	m.log("forcing migration version", "version", version)

	if err := m.migrate.Force(version); err != nil {
		m.logError("force version failed", err)
		return fmt.Errorf("forcing migration version %d: %w", version, err)
	}

	m.log("migration version forced", "version", version)
	return nil
}

// Close releases resources held by the migrator.
func (m *Migrator) Close() error {
	if m.migrate == nil {
		return nil
	}

	sourceErr, dbErr := m.migrate.Close()
	if sourceErr != nil {
		return fmt.Errorf("closing source driver: %w", sourceErr)
	}
	if dbErr != nil {
		return fmt.Errorf("closing database driver: %w", dbErr)
	}

	return nil
}

// log logs a message if a logger is configured.
func (m *Migrator) log(msg string, args ...any) {
	if m.config.Logger != nil {
		m.config.Logger.Info(msg, args...)
	}
}

// logError logs an error message if a logger is configured.
func (m *Migrator) logError(msg string, err error) {
	if m.config.Logger != nil {
		m.config.Logger.Error(msg, "error", err.Error())
	}
}
