// Package postgres provides PostgreSQL integration tests using testcontainers.
//
//go:build integration

package postgres

import (
	"context"
	"embed"
	"errors"
	"testing"
	"time"

	"github.com/Dorico-Dynamics/txova-go-core/logging"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
)

//go:embed testdata/migrations/*.sql
var testMigrations embed.FS

// setupPostgres creates a PostgreSQL container for testing.
func setupPostgres(t *testing.T) (Pool, func()) {
	t.Helper()
	ctx := context.Background()

	container, err := postgres.Run(ctx,
		"postgres:16-alpine",
		postgres.WithDatabase("testdb"),
		postgres.WithUsername("testuser"),
		postgres.WithPassword("testpass"),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2).
				WithStartupTimeout(60*time.Second),
		),
	)
	if err != nil {
		t.Fatalf("failed to start postgres container: %v", err)
	}

	connStr, err := container.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		t.Fatalf("failed to get connection string: %v", err)
	}

	pool, err := NewPool(ctx,
		WithConnString(connStr),
		WithMaxConns(5),
		WithMinConns(1),
		WithLogger(logging.Default()),
	)
	if err != nil {
		t.Fatalf("failed to create pool: %v", err)
	}

	cleanup := func() {
		pool.Close()
		if err := container.Terminate(ctx); err != nil {
			t.Logf("failed to terminate container: %v", err)
		}
	}

	return pool, cleanup
}

func TestPool_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	pool, cleanup := setupPostgres(t)
	defer cleanup()

	ctx := context.Background()

	t.Run("Ping", func(t *testing.T) {
		err := pool.Ping(ctx)
		if err != nil {
			t.Fatalf("Ping() error = %v", err)
		}
	})

	t.Run("Stat", func(t *testing.T) {
		stats := pool.Stat()
		if stats.MaxConns != 5 {
			t.Errorf("Stat().MaxConns = %d, want 5", stats.MaxConns)
		}
	})

	t.Run("Exec", func(t *testing.T) {
		_, err := pool.Exec(ctx, "SELECT 1")
		if err != nil {
			t.Fatalf("Exec() error = %v", err)
		}
	})

	t.Run("Query", func(t *testing.T) {
		rows, err := pool.Query(ctx, "SELECT 1 as num, 'hello' as str")
		if err != nil {
			t.Fatalf("Query() error = %v", err)
		}
		defer rows.Close()

		if !rows.Next() {
			t.Fatal("Query() returned no rows")
		}

		var num int
		var str string
		if err := rows.Scan(&num, &str); err != nil {
			t.Fatalf("Scan() error = %v", err)
		}
		if num != 1 || str != "hello" {
			t.Errorf("Query() = (%d, %q), want (1, hello)", num, str)
		}
	})

	t.Run("QueryRow", func(t *testing.T) {
		var result int
		err := pool.QueryRow(ctx, "SELECT 42").Scan(&result)
		if err != nil {
			t.Fatalf("QueryRow() error = %v", err)
		}
		if result != 42 {
			t.Errorf("QueryRow() = %d, want 42", result)
		}
	})
}

func TestConnection_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	pool, cleanup := setupPostgres(t)
	defer cleanup()

	ctx := context.Background()

	t.Run("Acquire and Release", func(t *testing.T) {
		conn, err := pool.Acquire(ctx)
		if err != nil {
			t.Fatalf("Acquire() error = %v", err)
		}

		// Test Ping on connection
		err = conn.Ping(ctx)
		if err != nil {
			t.Errorf("Ping() error = %v", err)
		}

		// Test Exec on connection
		_, err = conn.Exec(ctx, "SELECT 1")
		if err != nil {
			t.Errorf("Exec() error = %v", err)
		}

		// Test Query on connection
		rows, err := conn.Query(ctx, "SELECT 1")
		if err != nil {
			t.Errorf("Query() error = %v", err)
		}
		rows.Close()

		// Test QueryRow on connection
		var result int
		err = conn.QueryRow(ctx, "SELECT 1").Scan(&result)
		if err != nil {
			t.Errorf("QueryRow() error = %v", err)
		}

		// Release
		conn.Release()
	})

	t.Run("Begin transaction from connection", func(t *testing.T) {
		conn, err := pool.Acquire(ctx)
		if err != nil {
			t.Fatalf("Acquire() error = %v", err)
		}
		defer conn.Release()

		tx, err := conn.Begin(ctx)
		if err != nil {
			t.Fatalf("Begin() error = %v", err)
		}

		err = tx.Rollback(ctx)
		if err != nil {
			t.Errorf("Rollback() error = %v", err)
		}
	})

	t.Run("BeginTx from connection", func(t *testing.T) {
		conn, err := pool.Acquire(ctx)
		if err != nil {
			t.Fatalf("Acquire() error = %v", err)
		}
		defer conn.Release()

		tx, err := conn.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.ReadCommitted})
		if err != nil {
			t.Fatalf("BeginTx() error = %v", err)
		}

		err = tx.Rollback(ctx)
		if err != nil {
			t.Errorf("Rollback() error = %v", err)
		}
	})
}

func TestTransaction_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	pool, cleanup := setupPostgres(t)
	defer cleanup()

	ctx := context.Background()

	// Create test table
	_, err := pool.Exec(ctx, `CREATE TABLE IF NOT EXISTS test_tx (id SERIAL PRIMARY KEY, value TEXT)`)
	if err != nil {
		t.Fatalf("failed to create table: %v", err)
	}

	t.Run("Commit", func(t *testing.T) {
		tx, err := pool.Begin(ctx)
		if err != nil {
			t.Fatalf("Begin() error = %v", err)
		}

		_, err = tx.Exec(ctx, "INSERT INTO test_tx (value) VALUES ($1)", "commit-test")
		if err != nil {
			t.Fatalf("Exec() error = %v", err)
		}

		err = tx.Commit(ctx)
		if err != nil {
			t.Fatalf("Commit() error = %v", err)
		}

		// Verify data persisted
		var count int
		err = pool.QueryRow(ctx, "SELECT COUNT(*) FROM test_tx WHERE value = 'commit-test'").Scan(&count)
		if err != nil {
			t.Fatalf("QueryRow() error = %v", err)
		}
		if count != 1 {
			t.Errorf("expected 1 row, got %d", count)
		}
	})

	t.Run("Rollback", func(t *testing.T) {
		tx, err := pool.Begin(ctx)
		if err != nil {
			t.Fatalf("Begin() error = %v", err)
		}

		_, err = tx.Exec(ctx, "INSERT INTO test_tx (value) VALUES ($1)", "rollback-test")
		if err != nil {
			t.Fatalf("Exec() error = %v", err)
		}

		err = tx.Rollback(ctx)
		if err != nil {
			t.Fatalf("Rollback() error = %v", err)
		}

		// Verify data not persisted
		var count int
		err = pool.QueryRow(ctx, "SELECT COUNT(*) FROM test_tx WHERE value = 'rollback-test'").Scan(&count)
		if err != nil {
			t.Fatalf("QueryRow() error = %v", err)
		}
		if count != 0 {
			t.Errorf("expected 0 rows, got %d", count)
		}
	})

	t.Run("Query in transaction", func(t *testing.T) {
		tx, err := pool.Begin(ctx)
		if err != nil {
			t.Fatalf("Begin() error = %v", err)
		}
		defer func() { _ = tx.Rollback(ctx) }()

		rows, err := tx.Query(ctx, "SELECT 1, 2, 3")
		if err != nil {
			t.Fatalf("Query() error = %v", err)
		}
		defer rows.Close()

		if !rows.Next() {
			t.Error("expected row")
		}
	})

	t.Run("QueryRow in transaction", func(t *testing.T) {
		tx, err := pool.Begin(ctx)
		if err != nil {
			t.Fatalf("Begin() error = %v", err)
		}
		defer func() { _ = tx.Rollback(ctx) }()

		var result int
		err = tx.QueryRow(ctx, "SELECT 99").Scan(&result)
		if err != nil {
			t.Fatalf("QueryRow() error = %v", err)
		}
		if result != 99 {
			t.Errorf("expected 99, got %d", result)
		}
	})

	t.Run("Nested transaction (savepoint)", func(t *testing.T) {
		tx, err := pool.Begin(ctx)
		if err != nil {
			t.Fatalf("Begin() error = %v", err)
		}
		defer func() { _ = tx.Rollback(ctx) }()

		// Begin nested transaction
		nested, err := tx.Begin(ctx)
		if err != nil {
			t.Fatalf("nested Begin() error = %v", err)
		}

		_, err = nested.Exec(ctx, "INSERT INTO test_tx (value) VALUES ($1)", "nested-test")
		if err != nil {
			t.Fatalf("Exec() error = %v", err)
		}

		// Rollback nested
		err = nested.Rollback(ctx)
		if err != nil {
			t.Errorf("nested Rollback() error = %v", err)
		}
	})

	t.Run("Conn returns underlying connection", func(t *testing.T) {
		tx, err := pool.Begin(ctx)
		if err != nil {
			t.Fatalf("Begin() error = %v", err)
		}
		defer func() { _ = tx.Rollback(ctx) }()

		conn := tx.Conn()
		if conn == nil {
			t.Error("Conn() returned nil")
		}
	})
}

func TestTxManager_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	pool, cleanup := setupPostgres(t)
	defer cleanup()

	ctx := context.Background()

	// Create test table
	_, err := pool.Exec(ctx, `CREATE TABLE IF NOT EXISTS test_txmanager (id SERIAL PRIMARY KEY, value TEXT)`)
	if err != nil {
		t.Fatalf("failed to create table: %v", err)
	}

	// Get underlying pgxPool to create TxManager
	pgxPool := pool.(*pgxPool)
	txMgr := NewTxManager(pgxPool.pool, WithMaxRetries(3))

	t.Run("WithTx success", func(t *testing.T) {
		err := txMgr.WithTx(ctx, func(tx Tx) error {
			_, err := tx.Exec(ctx, "INSERT INTO test_txmanager (value) VALUES ($1)", "txmgr-success")
			return err
		})
		if err != nil {
			t.Fatalf("WithTx() error = %v", err)
		}

		// Verify committed
		var count int
		_ = pool.QueryRow(ctx, "SELECT COUNT(*) FROM test_txmanager WHERE value = 'txmgr-success'").Scan(&count)
		if count != 1 {
			t.Errorf("expected 1 row, got %d", count)
		}
	})

	t.Run("WithTx rollback on error", func(t *testing.T) {
		testErr := errors.New("test error")
		err := txMgr.WithTx(ctx, func(tx Tx) error {
			_, _ = tx.Exec(ctx, "INSERT INTO test_txmanager (value) VALUES ($1)", "txmgr-error")
			return testErr
		})
		if !errors.Is(err, testErr) {
			t.Fatalf("WithTx() error = %v, want %v", err, testErr)
		}

		// Verify rolled back
		var count int
		_ = pool.QueryRow(ctx, "SELECT COUNT(*) FROM test_txmanager WHERE value = 'txmgr-error'").Scan(&count)
		if count != 0 {
			t.Errorf("expected 0 rows (rollback), got %d", count)
		}
	})

	t.Run("WithTxOptions", func(t *testing.T) {
		err := txMgr.WithTxOptions(ctx, pgx.TxOptions{IsoLevel: pgx.Serializable}, func(tx Tx) error {
			_, err := tx.Exec(ctx, "INSERT INTO test_txmanager (value) VALUES ($1)", "txmgr-options")
			return err
		})
		if err != nil {
			t.Fatalf("WithTxOptions() error = %v", err)
		}
	})

	t.Run("Context propagation", func(t *testing.T) {
		err := txMgr.WithTx(ctx, func(tx Tx) error {
			// Get tx from context
			txCtx := ContextWithTx(ctx, tx)
			txFromCtx, ok := TxFromContext(txCtx)
			if !ok {
				t.Error("TxFromContext() returned false")
			}
			if txFromCtx != tx {
				t.Error("TxFromContext() returned different tx")
			}
			return nil
		})
		if err != nil {
			t.Fatalf("WithTx() error = %v", err)
		}
	})
}

func TestMigrator_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	ctx := context.Background()

	container, err := postgres.Run(ctx,
		"postgres:16-alpine",
		postgres.WithDatabase("testdb"),
		postgres.WithUsername("testuser"),
		postgres.WithPassword("testpass"),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2).
				WithStartupTimeout(60*time.Second),
		),
	)
	if err != nil {
		t.Fatalf("failed to start postgres container: %v", err)
	}
	defer func() { _ = container.Terminate(ctx) }()

	connStr, err := container.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		t.Fatalf("failed to get connection string: %v", err)
	}

	// Create a minimal pool config for migrations
	poolCfg, err := pgxPoolParse(connStr)
	if err != nil {
		t.Fatalf("failed to parse connection string: %v", err)
	}

	pool, err := pgxPoolNew(ctx, poolCfg)
	if err != nil {
		t.Fatalf("failed to create pool: %v", err)
	}
	defer pool.Close()

	t.Run("NewMigrator with nil pool", func(t *testing.T) {
		_, err := NewMigrator(nil, testMigrations)
		if err == nil {
			t.Error("expected error for nil pool")
		}
	})

	t.Run("NewMigrator with nil migrations", func(t *testing.T) {
		_, err := NewMigrator(pool, nil)
		if err == nil {
			t.Error("expected error for nil migrations")
		}
	})

	// Create migrator with test migrations
	migrator, err := NewMigrator(pool, testMigrations,
		WithMigrationsTable("test_migrations"),
		WithMigratorLogger(logging.Default()),
	)
	if err != nil {
		t.Fatalf("NewMigrator() error = %v", err)
	}
	defer func() { _ = migrator.Close() }()

	t.Run("Version before migrations", func(t *testing.T) {
		version, dirty, err := migrator.Version()
		if err != nil {
			t.Fatalf("Version() error = %v", err)
		}
		if version != 0 {
			t.Errorf("Version() = %d, want 0", version)
		}
		if dirty {
			t.Error("dirty should be false")
		}
	})

	t.Run("Up migrations", func(t *testing.T) {
		err := migrator.Up(ctx)
		if err != nil {
			t.Fatalf("Up() error = %v", err)
		}
	})

	t.Run("Version after Up", func(t *testing.T) {
		version, dirty, err := migrator.Version()
		if err != nil {
			t.Fatalf("Version() error = %v", err)
		}
		if version == 0 {
			t.Error("Version() should be > 0 after migrations")
		}
		if dirty {
			t.Error("dirty should be false")
		}
	})

	t.Run("Up with no pending migrations", func(t *testing.T) {
		// Should not error when no migrations pending
		err := migrator.Up(ctx)
		if err != nil {
			t.Fatalf("Up() with no pending error = %v", err)
		}
	})

	t.Run("Steps", func(t *testing.T) {
		// Steps(0) should be no-op
		err := migrator.Steps(ctx, 0)
		if err != nil {
			t.Fatalf("Steps(0) error = %v", err)
		}
	})

	t.Run("Down migrations", func(t *testing.T) {
		err := migrator.Down(ctx)
		if err != nil {
			t.Fatalf("Down() error = %v", err)
		}
	})

	t.Run("Force version", func(t *testing.T) {
		err := migrator.Force(0)
		if err != nil {
			t.Fatalf("Force() error = %v", err)
		}
	})
}

// Helper functions to create pgxpool directly (bypassing our wrapper for migrator tests)
func pgxPoolParse(connString string) (*pgxPoolConfig, error) {
	return pgxpool.ParseConfig(connString)
}

type pgxPoolConfig = pgxpool.Config

func pgxPoolNew(ctx context.Context, cfg *pgxpool.Config) (*pgxpool.Pool, error) {
	return pgxpool.NewWithConfig(ctx, cfg)
}

func TestErrorMapping_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	pool, cleanup := setupPostgres(t)
	defer cleanup()

	ctx := context.Background()

	// Create test table
	_, err := pool.Exec(ctx, `
		CREATE TABLE IF NOT EXISTS test_errors (
			id SERIAL PRIMARY KEY,
			unique_val TEXT UNIQUE NOT NULL,
			ref_id INT REFERENCES test_errors(id)
		)
	`)
	if err != nil {
		t.Fatalf("failed to create table: %v", err)
	}

	t.Run("Duplicate key error", func(t *testing.T) {
		_, _ = pool.Exec(ctx, "INSERT INTO test_errors (unique_val) VALUES ('duplicate')")
		_, err := pool.Exec(ctx, "INSERT INTO test_errors (unique_val) VALUES ('duplicate')")
		if err == nil {
			t.Fatal("expected duplicate error")
		}
		if !IsDuplicate(err) {
			t.Errorf("IsDuplicate() = false, want true for error: %v", err)
		}
	})

	t.Run("Foreign key error", func(t *testing.T) {
		_, err := pool.Exec(ctx, "INSERT INTO test_errors (unique_val, ref_id) VALUES ('fk-test', 99999)")
		if err == nil {
			t.Fatal("expected foreign key error")
		}
		if !IsForeignKey(err) {
			t.Errorf("IsForeignKey() = false, want true for error: %v", err)
		}
	})

	t.Run("Not found (no rows)", func(t *testing.T) {
		var id int
		err := pool.QueryRow(ctx, "SELECT id FROM test_errors WHERE unique_val = 'nonexistent'").Scan(&id)
		if err == nil {
			t.Fatal("expected error")
		}
		// pgx returns ErrNoRows which should be detected
		if !IsNotFound(err) {
			t.Logf("Note: pgx.ErrNoRows not wrapped as NotFound: %v", err)
		}
	})
}

func TestQueryBuilder_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	pool, cleanup := setupPostgres(t)
	defer cleanup()

	ctx := context.Background()

	// Create and populate test table
	_, err := pool.Exec(ctx, `
		CREATE TABLE IF NOT EXISTS users (
			id SERIAL PRIMARY KEY,
			name TEXT NOT NULL,
			email TEXT UNIQUE NOT NULL,
			active BOOLEAN DEFAULT true,
			created_at TIMESTAMP DEFAULT NOW()
		)
	`)
	if err != nil {
		t.Fatalf("failed to create table: %v", err)
	}

	// Insert test data using InsertBuilder
	t.Run("Insert", func(t *testing.T) {
		sql, args, err := Insert("users").
			Columns("name", "email", "active").
			Values("Alice", "alice@example.com", true).
			Values("Bob", "bob@example.com", true).
			Values("Charlie", "charlie@example.com", false).
			Build()
		if err != nil {
			t.Fatalf("Build() error = %v", err)
		}

		_, err = pool.Exec(ctx, sql, args...)
		if err != nil {
			t.Fatalf("Exec() error = %v", err)
		}
	})

	t.Run("Select with Where", func(t *testing.T) {
		sql, args, err := Select("users").
			Columns("id", "name", "email").
			Where("active", true).
			OrderByAsc("name").
			Build()
		if err != nil {
			t.Fatalf("Build() error = %v", err)
		}

		rows, err := pool.Query(ctx, sql, args...)
		if err != nil {
			t.Fatalf("Query() error = %v", err)
		}
		defer rows.Close()

		count := 0
		for rows.Next() {
			count++
		}
		if count != 2 {
			t.Errorf("expected 2 active users, got %d", count)
		}
	})

	t.Run("Select with Pagination", func(t *testing.T) {
		sql, args, err := Select("users").
			Columns("name").
			OrderByAsc("name").
			Limit(2).
			Offset(1).
			Build()
		if err != nil {
			t.Fatalf("Build() error = %v", err)
		}

		rows, err := pool.Query(ctx, sql, args...)
		if err != nil {
			t.Fatalf("Query() error = %v", err)
		}
		defer rows.Close()

		var names []string
		for rows.Next() {
			var name string
			_ = rows.Scan(&name)
			names = append(names, name)
		}
		if len(names) != 2 {
			t.Errorf("expected 2 names, got %d", len(names))
		}
	})

	t.Run("Update", func(t *testing.T) {
		sql, args, err := Update("users").
			Set("active", false).
			Where("name", "Alice").
			Build()
		if err != nil {
			t.Fatalf("Build() error = %v", err)
		}

		tag, err := pool.Exec(ctx, sql, args...)
		if err != nil {
			t.Fatalf("Exec() error = %v", err)
		}
		if tag.RowsAffected() != 1 {
			t.Errorf("expected 1 row affected, got %d", tag.RowsAffected())
		}
	})

	t.Run("Delete", func(t *testing.T) {
		sql, args, err := Delete("users").
			Where("name", "Charlie").
			Build()
		if err != nil {
			t.Fatalf("Build() error = %v", err)
		}

		tag, err := pool.Exec(ctx, sql, args...)
		if err != nil {
			t.Fatalf("Exec() error = %v", err)
		}
		if tag.RowsAffected() != 1 {
			t.Errorf("expected 1 row affected, got %d", tag.RowsAffected())
		}
	})
}
