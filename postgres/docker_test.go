// Package postgres provides PostgreSQL tests using docker-compose infrastructure.
//
//go:build docker

package postgres

import (
	"context"
	"errors"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/Dorico-Dynamics/txova-go-core/logging"
	"github.com/jackc/pgx/v5"
)

// getDockerPostgresConnStr returns the connection string for docker-compose postgres.
func getDockerPostgresConnStr() string {
	host := os.Getenv("POSTGRES_HOST")
	if host == "" {
		host = "localhost"
	}
	port := os.Getenv("POSTGRES_PORT")
	if port == "" {
		port = "5433"
	}
	user := os.Getenv("POSTGRES_USER")
	if user == "" {
		user = "txova"
	}
	password := os.Getenv("POSTGRES_PASSWORD")
	if password == "" {
		password = "txova_dev_password"
	}
	dbname := os.Getenv("POSTGRES_DB")
	if dbname == "" {
		dbname = "txova_test"
	}
	return fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=disable", user, password, host, port, dbname)
}

// setupDockerPool creates a pool connected to docker-compose postgres.
func setupDockerPool(t *testing.T) Pool {
	t.Helper()
	ctx := context.Background()

	connStr := getDockerPostgresConnStr()
	t.Logf("Connecting to: %s", connStr)

	pool, err := NewPool(ctx,
		WithConnString(connStr),
		WithMaxConns(5),
		WithMinConns(1),
		WithLogger(logging.Default()),
		WithSlowQueryThreshold(100*time.Millisecond),
	)
	if err != nil {
		t.Fatalf("failed to create pool: %v", err)
	}

	// Verify connection
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		t.Fatalf("failed to ping: %v", err)
	}

	t.Cleanup(func() {
		pool.Close()
	})

	return pool
}

func TestDockerPool_Ping(t *testing.T) {
	pool := setupDockerPool(t)
	ctx := context.Background()

	err := pool.Ping(ctx)
	if err != nil {
		t.Fatalf("Ping() error = %v", err)
	}
}

func TestDockerPool_Stat(t *testing.T) {
	pool := setupDockerPool(t)

	stats := pool.Stat()
	if stats.MaxConns != 5 {
		t.Errorf("Stat().MaxConns = %d, want 5", stats.MaxConns)
	}
	if stats.TotalConns < 1 {
		t.Errorf("Stat().TotalConns = %d, want >= 1", stats.TotalConns)
	}
}

func TestDockerPool_Exec(t *testing.T) {
	pool := setupDockerPool(t)
	ctx := context.Background()

	tag, err := pool.Exec(ctx, "SELECT 1")
	if err != nil {
		t.Fatalf("Exec() error = %v", err)
	}
	t.Logf("Exec result: %s", tag.String())
}

func TestDockerPool_Query(t *testing.T) {
	pool := setupDockerPool(t)
	ctx := context.Background()

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
}

func TestDockerPool_QueryRow(t *testing.T) {
	pool := setupDockerPool(t)
	ctx := context.Background()

	var result int
	err := pool.QueryRow(ctx, "SELECT 42").Scan(&result)
	if err != nil {
		t.Fatalf("QueryRow() error = %v", err)
	}
	if result != 42 {
		t.Errorf("QueryRow() = %d, want 42", result)
	}
}

func TestDockerPool_Begin(t *testing.T) {
	pool := setupDockerPool(t)
	ctx := context.Background()

	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatalf("Begin() error = %v", err)
	}

	err = tx.Rollback(ctx)
	if err != nil {
		t.Errorf("Rollback() error = %v", err)
	}
}

func TestDockerPool_BeginTx(t *testing.T) {
	pool := setupDockerPool(t)
	ctx := context.Background()

	tx, err := pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.Serializable})
	if err != nil {
		t.Fatalf("BeginTx() error = %v", err)
	}

	err = tx.Rollback(ctx)
	if err != nil {
		t.Errorf("Rollback() error = %v", err)
	}
}

func TestDockerConnection_AcquireRelease(t *testing.T) {
	pool := setupDockerPool(t)
	ctx := context.Background()

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

	// Test Begin on connection
	tx, err := conn.Begin(ctx)
	if err != nil {
		t.Errorf("Begin() error = %v", err)
	}
	_ = tx.Rollback(ctx)

	// Test BeginTx on connection
	tx, err = conn.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		t.Errorf("BeginTx() error = %v", err)
	}
	_ = tx.Rollback(ctx)

	// Release
	conn.Release()
}

func TestDockerTransaction_CommitRollback(t *testing.T) {
	pool := setupDockerPool(t)
	ctx := context.Background()

	// Create test table
	_, err := pool.Exec(ctx, `CREATE TABLE IF NOT EXISTS docker_test_tx (id SERIAL PRIMARY KEY, value TEXT)`)
	if err != nil {
		t.Fatalf("failed to create table: %v", err)
	}
	defer func() {
		_, _ = pool.Exec(ctx, `DROP TABLE IF EXISTS docker_test_tx`)
	}()

	t.Run("Commit", func(t *testing.T) {
		tx, err := pool.Begin(ctx)
		if err != nil {
			t.Fatalf("Begin() error = %v", err)
		}

		_, err = tx.Exec(ctx, "INSERT INTO docker_test_tx (value) VALUES ($1)", "commit-test")
		if err != nil {
			t.Fatalf("Exec() error = %v", err)
		}

		err = tx.Commit(ctx)
		if err != nil {
			t.Fatalf("Commit() error = %v", err)
		}

		// Verify data persisted
		var count int
		err = pool.QueryRow(ctx, "SELECT COUNT(*) FROM docker_test_tx WHERE value = 'commit-test'").Scan(&count)
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

		_, err = tx.Exec(ctx, "INSERT INTO docker_test_tx (value) VALUES ($1)", "rollback-test")
		if err != nil {
			t.Fatalf("Exec() error = %v", err)
		}

		err = tx.Rollback(ctx)
		if err != nil {
			t.Fatalf("Rollback() error = %v", err)
		}

		// Verify data not persisted
		var count int
		err = pool.QueryRow(ctx, "SELECT COUNT(*) FROM docker_test_tx WHERE value = 'rollback-test'").Scan(&count)
		if err != nil {
			t.Fatalf("QueryRow() error = %v", err)
		}
		if count != 0 {
			t.Errorf("expected 0 rows, got %d", count)
		}
	})
}

func TestDockerTransaction_QueryMethods(t *testing.T) {
	pool := setupDockerPool(t)
	ctx := context.Background()

	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatalf("Begin() error = %v", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	// Query
	rows, err := tx.Query(ctx, "SELECT 1, 2, 3")
	if err != nil {
		t.Fatalf("Query() error = %v", err)
	}
	rows.Close()

	// QueryRow
	var result int
	err = tx.QueryRow(ctx, "SELECT 99").Scan(&result)
	if err != nil {
		t.Fatalf("QueryRow() error = %v", err)
	}
	if result != 99 {
		t.Errorf("expected 99, got %d", result)
	}

	// Nested transaction (savepoint)
	nested, err := tx.Begin(ctx)
	if err != nil {
		t.Fatalf("nested Begin() error = %v", err)
	}
	_ = nested.Rollback(ctx)

	// Conn
	conn := tx.Conn()
	if conn == nil {
		t.Error("Conn() returned nil")
	}
}

func TestDockerTxManager(t *testing.T) {
	pool := setupDockerPool(t)
	ctx := context.Background()

	// Create test table
	_, err := pool.Exec(ctx, `CREATE TABLE IF NOT EXISTS docker_test_txmgr (id SERIAL PRIMARY KEY, value TEXT)`)
	if err != nil {
		t.Fatalf("failed to create table: %v", err)
	}
	defer func() {
		_, _ = pool.Exec(ctx, `DROP TABLE IF EXISTS docker_test_txmgr`)
	}()

	txMgr := NewTxManager(pool)

	t.Run("WithTx success", func(t *testing.T) {
		err := txMgr.WithTx(ctx, func(tx Tx) error {
			_, err := tx.Exec(ctx, "INSERT INTO docker_test_txmgr (value) VALUES ($1)", "success")
			return err
		})
		if err != nil {
			t.Fatalf("WithTx() error = %v", err)
		}

		var count int
		_ = pool.QueryRow(ctx, "SELECT COUNT(*) FROM docker_test_txmgr WHERE value = 'success'").Scan(&count)
		if count != 1 {
			t.Errorf("expected 1 row, got %d", count)
		}
	})

	t.Run("WithTx rollback on error", func(t *testing.T) {
		testErr := errors.New("test error")
		err := txMgr.WithTx(ctx, func(tx Tx) error {
			_, _ = tx.Exec(ctx, "INSERT INTO docker_test_txmgr (value) VALUES ($1)", "error")
			return testErr
		})
		if !errors.Is(err, testErr) {
			t.Fatalf("WithTx() error = %v, want %v", err, testErr)
		}

		var count int
		_ = pool.QueryRow(ctx, "SELECT COUNT(*) FROM docker_test_txmgr WHERE value = 'error'").Scan(&count)
		if count != 0 {
			t.Errorf("expected 0 rows, got %d", count)
		}
	})

	t.Run("WithTxOptions", func(t *testing.T) {
		err := txMgr.WithTxOptions(ctx, pgx.TxOptions{IsoLevel: pgx.Serializable}, func(tx Tx) error {
			_, err := tx.Exec(ctx, "INSERT INTO docker_test_txmgr (value) VALUES ($1)", "serializable")
			return err
		})
		if err != nil {
			t.Fatalf("WithTxOptions() error = %v", err)
		}
	})
}

func TestDockerErrorMapping(t *testing.T) {
	pool := setupDockerPool(t)
	ctx := context.Background()

	// Create test table
	_, err := pool.Exec(ctx, `
		CREATE TABLE IF NOT EXISTS docker_test_errors (
			id SERIAL PRIMARY KEY,
			unique_val TEXT UNIQUE NOT NULL,
			ref_id INT REFERENCES docker_test_errors(id)
		)
	`)
	if err != nil {
		t.Fatalf("failed to create table: %v", err)
	}
	defer func() {
		_, _ = pool.Exec(ctx, `DROP TABLE IF EXISTS docker_test_errors`)
	}()

	t.Run("Duplicate key error", func(t *testing.T) {
		_, _ = pool.Exec(ctx, "INSERT INTO docker_test_errors (unique_val) VALUES ('duplicate')")
		_, err := pool.Exec(ctx, "INSERT INTO docker_test_errors (unique_val) VALUES ('duplicate')")
		if err == nil {
			t.Fatal("expected duplicate error")
		}
		if !IsDuplicate(err) {
			t.Errorf("IsDuplicate() = false for error: %v", err)
		}
	})

	t.Run("Foreign key error", func(t *testing.T) {
		_, err := pool.Exec(ctx, "INSERT INTO docker_test_errors (unique_val, ref_id) VALUES ('fk-test', 99999)")
		if err == nil {
			t.Fatal("expected foreign key error")
		}
		if !IsForeignKey(err) {
			t.Errorf("IsForeignKey() = false for error: %v", err)
		}
	})
}

func TestDockerSlowQueryLogging(t *testing.T) {
	pool := setupDockerPool(t)
	ctx := context.Background()

	// This should trigger slow query logging (threshold is 100ms)
	_, err := pool.Exec(ctx, "SELECT pg_sleep(0.15)")
	if err != nil {
		t.Logf("pg_sleep error (expected on some systems): %v", err)
	}
}
