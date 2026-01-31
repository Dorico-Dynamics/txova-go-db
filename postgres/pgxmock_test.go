package postgres

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/Dorico-Dynamics/txova-go-core/logging"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/pashagolub/pgxmock/v4"
)

// mockPool wraps pgxmock to implement our Pool interface.
type mockPool struct {
	mock   pgxmock.PgxPoolIface
	logger *logging.Logger
}

func newMockPool(t *testing.T) (*mockPool, pgxmock.PgxPoolIface) {
	t.Helper()
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatalf("failed to create mock: %v", err)
	}
	return &mockPool{
		mock:   mock,
		logger: logging.Default(),
	}, mock
}

func (m *mockPool) Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
	tag, err := m.mock.Exec(ctx, sql, args...)
	if err != nil {
		return tag, FromPgError(err)
	}
	return tag, nil
}

func (m *mockPool) Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
	rows, err := m.mock.Query(ctx, sql, args...)
	if err != nil {
		return nil, FromPgError(err)
	}
	return rows, nil
}

func (m *mockPool) QueryRow(ctx context.Context, sql string, args ...any) pgx.Row {
	return m.mock.QueryRow(ctx, sql, args...)
}

func (m *mockPool) Acquire(ctx context.Context) (Conn, error) {
	return nil, errors.New("Acquire not implemented in mock")
}

func (m *mockPool) Begin(ctx context.Context) (Tx, error) {
	tx, err := m.mock.Begin(ctx)
	if err != nil {
		return nil, FromPgError(err)
	}
	return &mockTx{tx: tx, logger: m.logger}, nil
}

func (m *mockPool) BeginTx(ctx context.Context, opts pgx.TxOptions) (Tx, error) {
	tx, err := m.mock.BeginTx(ctx, opts)
	if err != nil {
		return nil, FromPgError(err)
	}
	return &mockTx{tx: tx, logger: m.logger}, nil
}

func (m *mockPool) Ping(ctx context.Context) error {
	return m.mock.Ping(ctx)
}

func (m *mockPool) Close() {
	m.mock.Close()
}

func (m *mockPool) Stat() PoolStats {
	return PoolStats{}
}

// mockTx wraps pgxmock transaction to implement our Tx interface.
type mockTx struct {
	tx     pgx.Tx
	logger *logging.Logger
}

func (t *mockTx) Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
	tag, err := t.tx.Exec(ctx, sql, args...)
	if err != nil {
		return tag, FromPgError(err)
	}
	return tag, nil
}

func (t *mockTx) Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
	rows, err := t.tx.Query(ctx, sql, args...)
	if err != nil {
		return nil, FromPgError(err)
	}
	return rows, nil
}

func (t *mockTx) QueryRow(ctx context.Context, sql string, args ...any) pgx.Row {
	return t.tx.QueryRow(ctx, sql, args...)
}

func (t *mockTx) Begin(ctx context.Context) (Tx, error) {
	nestedTx, err := t.tx.Begin(ctx)
	if err != nil {
		return nil, FromPgError(err)
	}
	return &mockTx{tx: nestedTx, logger: t.logger}, nil
}

func (t *mockTx) Commit(ctx context.Context) error {
	if err := t.tx.Commit(ctx); err != nil {
		return FromPgError(err)
	}
	return nil
}

func (t *mockTx) Rollback(ctx context.Context) error {
	if err := t.tx.Rollback(ctx); err != nil {
		return FromPgError(err)
	}
	return nil
}

func (t *mockTx) Conn() *pgx.Conn {
	return t.tx.Conn()
}

// =============================================================================
// Pool Mock Tests
// =============================================================================

func TestMockPool_Exec_Success(t *testing.T) {
	t.Parallel()

	pool, mock := newMockPool(t)
	defer mock.Close()

	ctx := context.Background()

	mock.ExpectExec("INSERT INTO users").
		WithArgs("test").
		WillReturnResult(pgxmock.NewResult("INSERT", 1))

	tag, err := pool.Exec(ctx, "INSERT INTO users (name) VALUES ($1)", "test")
	if err != nil {
		t.Fatalf("Exec() error = %v", err)
	}
	if tag.RowsAffected() != 1 {
		t.Errorf("RowsAffected() = %d, want 1", tag.RowsAffected())
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %v", err)
	}
}

func TestMockPool_Exec_Error(t *testing.T) {
	t.Parallel()

	pool, mock := newMockPool(t)
	defer mock.Close()

	ctx := context.Background()

	mock.ExpectExec("INSERT").
		WithArgs("test").
		WillReturnError(errors.New("connection error"))

	_, err := pool.Exec(ctx, "INSERT INTO users (name) VALUES ($1)", "test")
	if err == nil {
		t.Error("expected error")
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %v", err)
	}
}

func TestMockPool_Query_Success(t *testing.T) {
	t.Parallel()

	pool, mock := newMockPool(t)
	defer mock.Close()

	ctx := context.Background()

	rows := pgxmock.NewRows([]string{"id", "name"}).
		AddRow(1, "Alice").
		AddRow(2, "Bob")

	mock.ExpectQuery("SELECT").WillReturnRows(rows)

	result, err := pool.Query(ctx, "SELECT id, name FROM users")
	if err != nil {
		t.Fatalf("Query() error = %v", err)
	}
	defer result.Close()

	count := 0
	for result.Next() {
		count++
	}
	if count != 2 {
		t.Errorf("expected 2 rows, got %d", count)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %v", err)
	}
}

func TestMockPool_Query_Error(t *testing.T) {
	t.Parallel()

	pool, mock := newMockPool(t)
	defer mock.Close()

	ctx := context.Background()

	mock.ExpectQuery("SELECT").WillReturnError(errors.New("query failed"))

	//nolint:sqlclosecheck // Testing error path, no rows returned
	_, err := pool.Query(ctx, "SELECT * FROM users")
	if err == nil {
		t.Error("expected error")
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %v", err)
	}
}

func TestMockPool_QueryRow_Success(t *testing.T) {
	t.Parallel()

	pool, mock := newMockPool(t)
	defer mock.Close()

	ctx := context.Background()

	rows := pgxmock.NewRows([]string{"count"}).AddRow(42)
	mock.ExpectQuery("SELECT COUNT").WillReturnRows(rows)

	var count int
	err := pool.QueryRow(ctx, "SELECT COUNT(*) FROM users").Scan(&count)
	if err != nil {
		t.Fatalf("QueryRow() error = %v", err)
	}
	if count != 42 {
		t.Errorf("count = %d, want 42", count)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %v", err)
	}
}

func TestMockPool_Begin_Commit(t *testing.T) {
	t.Parallel()

	pool, mock := newMockPool(t)
	defer mock.Close()

	ctx := context.Background()

	mock.ExpectBegin()
	mock.ExpectExec("INSERT").
		WithArgs("test").
		WillReturnResult(pgxmock.NewResult("INSERT", 1))
	mock.ExpectCommit()

	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatalf("Begin() error = %v", err)
	}

	_, err = tx.Exec(ctx, "INSERT INTO users (name) VALUES ($1)", "test")
	if err != nil {
		t.Fatalf("Exec() error = %v", err)
	}

	err = tx.Commit(ctx)
	if err != nil {
		t.Fatalf("Commit() error = %v", err)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %v", err)
	}
}

func TestMockPool_Begin_Rollback(t *testing.T) {
	t.Parallel()

	pool, mock := newMockPool(t)
	defer mock.Close()

	ctx := context.Background()

	mock.ExpectBegin()
	mock.ExpectRollback()

	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatalf("Begin() error = %v", err)
	}

	err = tx.Rollback(ctx)
	if err != nil {
		t.Fatalf("Rollback() error = %v", err)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %v", err)
	}
}

func TestMockPool_BeginTx(t *testing.T) {
	t.Parallel()

	pool, mock := newMockPool(t)
	defer mock.Close()

	ctx := context.Background()

	mock.ExpectBeginTx(pgx.TxOptions{IsoLevel: pgx.Serializable})
	mock.ExpectCommit()

	tx, err := pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.Serializable})
	if err != nil {
		t.Fatalf("BeginTx() error = %v", err)
	}

	err = tx.Commit(ctx)
	if err != nil {
		t.Fatalf("Commit() error = %v", err)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %v", err)
	}
}

func TestMockPool_Ping(t *testing.T) {
	t.Parallel()

	pool, mock := newMockPool(t)
	defer mock.Close()

	ctx := context.Background()

	mock.ExpectPing()

	err := pool.Ping(ctx)
	if err != nil {
		t.Fatalf("Ping() error = %v", err)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %v", err)
	}
}

func TestMockPool_Ping_Error(t *testing.T) {
	t.Parallel()

	pool, mock := newMockPool(t)
	defer mock.Close()

	ctx := context.Background()

	mock.ExpectPing().WillReturnError(errors.New("connection refused"))

	err := pool.Ping(ctx)
	if err == nil {
		t.Error("expected error")
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %v", err)
	}
}

// =============================================================================
// Transaction Mock Tests
// =============================================================================

func TestMockTx_Query(t *testing.T) {
	t.Parallel()

	pool, mock := newMockPool(t)
	defer mock.Close()

	ctx := context.Background()

	mock.ExpectBegin()
	rows := pgxmock.NewRows([]string{"id"}).AddRow(1)
	mock.ExpectQuery("SELECT id FROM users").WillReturnRows(rows)
	mock.ExpectCommit()

	tx, _ := pool.Begin(ctx)

	result, err := tx.Query(ctx, "SELECT id FROM users")
	if err != nil {
		t.Fatalf("Query() error = %v", err)
	}
	result.Close()

	_ = tx.Commit(ctx)

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %v", err)
	}
}

func TestMockTx_QueryRow(t *testing.T) {
	t.Parallel()

	pool, mock := newMockPool(t)
	defer mock.Close()

	ctx := context.Background()

	mock.ExpectBegin()
	rows := pgxmock.NewRows([]string{"count"}).AddRow(5)
	mock.ExpectQuery("SELECT COUNT").WillReturnRows(rows)
	mock.ExpectCommit()

	tx, _ := pool.Begin(ctx)

	var count int
	err := tx.QueryRow(ctx, "SELECT COUNT(*) FROM users").Scan(&count)
	if err != nil {
		t.Fatalf("QueryRow() error = %v", err)
	}
	if count != 5 {
		t.Errorf("count = %d, want 5", count)
	}

	_ = tx.Commit(ctx)

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %v", err)
	}
}

func TestMockTx_Begin_Error(t *testing.T) {
	t.Parallel()

	pool, mock := newMockPool(t)
	defer mock.Close()

	ctx := context.Background()

	mock.ExpectBegin().WillReturnError(errors.New("cannot begin"))

	_, err := pool.Begin(ctx)
	if err == nil {
		t.Error("expected error")
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %v", err)
	}
}

func TestMockTx_Commit_Error(t *testing.T) {
	t.Parallel()

	pool, mock := newMockPool(t)
	defer mock.Close()

	ctx := context.Background()

	mock.ExpectBegin()
	mock.ExpectCommit().WillReturnError(errors.New("commit failed"))

	tx, _ := pool.Begin(ctx)
	err := tx.Commit(ctx)
	if err == nil {
		t.Error("expected error")
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %v", err)
	}
}

func TestMockTx_Rollback_Error(t *testing.T) {
	t.Parallel()

	pool, mock := newMockPool(t)
	defer mock.Close()

	ctx := context.Background()

	mock.ExpectBegin()
	mock.ExpectRollback().WillReturnError(errors.New("rollback failed"))

	tx, _ := pool.Begin(ctx)
	err := tx.Rollback(ctx)
	if err == nil {
		t.Error("expected error")
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %v", err)
	}
}

// =============================================================================
// TxManager Tests with pgxmock
// =============================================================================

func TestTxManager_WithTx_Success(t *testing.T) {
	t.Parallel()

	pool, mock := newMockPool(t)
	defer mock.Close()

	txMgr := NewTxManager(pool)
	ctx := context.Background()

	mock.ExpectBegin()
	mock.ExpectExec("INSERT INTO users").
		WithArgs("test").
		WillReturnResult(pgxmock.NewResult("INSERT", 1))
	mock.ExpectCommit()

	err := txMgr.WithTx(ctx, func(tx Tx) error {
		_, err := tx.Exec(ctx, "INSERT INTO users (name) VALUES ($1)", "test")
		return err
	})
	if err != nil {
		t.Fatalf("WithTx() error = %v", err)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %v", err)
	}
}

func TestTxManager_WithTx_Rollback(t *testing.T) {
	t.Parallel()

	pool, mock := newMockPool(t)
	defer mock.Close()

	txMgr := NewTxManager(pool)
	ctx := context.Background()

	mock.ExpectBegin()
	mock.ExpectRollback()

	fnErr := errors.New("business logic error")
	err := txMgr.WithTx(ctx, func(tx Tx) error {
		return fnErr
	})
	if !errors.Is(err, fnErr) {
		t.Errorf("WithTx() error = %v, want %v", err, fnErr)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %v", err)
	}
}

func TestTxManager_WithTx_BeginError(t *testing.T) {
	t.Parallel()

	pool, mock := newMockPool(t)
	defer mock.Close()

	txMgr := NewTxManager(pool)
	ctx := context.Background()

	mock.ExpectBegin().WillReturnError(errors.New("cannot begin"))

	err := txMgr.WithTx(ctx, func(tx Tx) error {
		return nil
	})
	if err == nil {
		t.Error("expected error")
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %v", err)
	}
}

func TestTxManager_WithTxOptions_Serializable(t *testing.T) {
	t.Parallel()

	pool, mock := newMockPool(t)
	defer mock.Close()

	txMgr := NewTxManager(pool)
	ctx := context.Background()

	mock.ExpectBeginTx(pgx.TxOptions{IsoLevel: pgx.Serializable})
	mock.ExpectExec("SELECT 1").WillReturnResult(pgxmock.NewResult("SELECT", 1))
	mock.ExpectCommit()

	err := txMgr.WithTxOptions(ctx, pgx.TxOptions{IsoLevel: pgx.Serializable}, func(tx Tx) error {
		_, err := tx.Exec(ctx, "SELECT 1")
		return err
	})
	if err != nil {
		t.Fatalf("WithTxOptions() error = %v", err)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %v", err)
	}
}

func TestTxManager_WithTxOptions_ReadOnly(t *testing.T) {
	t.Parallel()

	pool, mock := newMockPool(t)
	defer mock.Close()

	txMgr := NewTxManager(pool)
	ctx := context.Background()

	mock.ExpectBeginTx(pgx.TxOptions{AccessMode: pgx.ReadOnly})
	rows := pgxmock.NewRows([]string{"count"}).AddRow(10)
	mock.ExpectQuery("SELECT COUNT").WillReturnRows(rows)
	mock.ExpectCommit()

	err := txMgr.WithTxOptions(ctx, pgx.TxOptions{AccessMode: pgx.ReadOnly}, func(tx Tx) error {
		var count int
		return tx.QueryRow(ctx, "SELECT COUNT(*) FROM users").Scan(&count)
	})
	if err != nil {
		t.Fatalf("WithTxOptions() error = %v", err)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %v", err)
	}
}

func TestTxManager_ExistingTxInContext(t *testing.T) {
	t.Parallel()

	pool, mock := newMockPool(t)
	defer mock.Close()

	txMgr := NewTxManager(pool)
	ctx := context.Background()

	mock.ExpectBegin()
	mock.ExpectExec("INSERT").
		WithArgs("test").
		WillReturnResult(pgxmock.NewResult("INSERT", 1))
	mock.ExpectCommit()

	// Begin outer transaction
	outerTx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatalf("Begin() error = %v", err)
	}

	// Store tx in context
	ctxWithTx := ContextWithTx(ctx, outerTx)

	// WithTx should use existing tx
	err = txMgr.WithTx(ctxWithTx, func(tx Tx) error {
		_, execErr := tx.Exec(ctx, "INSERT INTO users (name) VALUES ($1)", "test")
		return execErr
	})
	if err != nil {
		t.Fatalf("WithTx() error = %v", err)
	}

	// Commit outer transaction
	err = outerTx.Commit(ctx)
	if err != nil {
		t.Fatalf("Commit() error = %v", err)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %v", err)
	}
}

func TestTxManager_Retry_SerializationFailure(t *testing.T) {
	t.Parallel()

	pool, mock := newMockPool(t)
	defer mock.Close()

	txMgr := NewTxManager(pool,
		WithMaxRetries(2),
		WithRetryBaseDelay(time.Millisecond),
		WithRetryMaxDelay(5*time.Millisecond),
	)
	ctx := context.Background()

	// First attempt - serialization failure
	mock.ExpectBegin()
	mock.ExpectExec("UPDATE").
		WillReturnError(&pgconn.PgError{Code: "40001", Message: "could not serialize access"})
	mock.ExpectRollback()

	// Second attempt - success
	mock.ExpectBegin()
	mock.ExpectExec("UPDATE").
		WillReturnResult(pgxmock.NewResult("UPDATE", 1))
	mock.ExpectCommit()

	attempts := 0
	err := txMgr.WithTx(ctx, func(tx Tx) error {
		attempts++
		_, err := tx.Exec(ctx, "UPDATE accounts SET balance = balance - 100 WHERE id = 1")
		return err
	})
	if err != nil {
		t.Fatalf("WithTx() error = %v", err)
	}
	if attempts != 2 {
		t.Errorf("attempts = %d, want 2", attempts)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %v", err)
	}
}

func TestTxManager_Retry_Deadlock(t *testing.T) {
	t.Parallel()

	pool, mock := newMockPool(t)
	defer mock.Close()

	txMgr := NewTxManager(pool,
		WithMaxRetries(2),
		WithRetryBaseDelay(time.Millisecond),
		WithRetryMaxDelay(5*time.Millisecond),
	)
	ctx := context.Background()

	// First attempt - deadlock
	mock.ExpectBegin()
	mock.ExpectExec("UPDATE").
		WillReturnError(&pgconn.PgError{Code: "40P01", Message: "deadlock detected"})
	mock.ExpectRollback()

	// Second attempt - success
	mock.ExpectBegin()
	mock.ExpectExec("UPDATE").
		WillReturnResult(pgxmock.NewResult("UPDATE", 1))
	mock.ExpectCommit()

	err := txMgr.WithTx(ctx, func(tx Tx) error {
		_, err := tx.Exec(ctx, "UPDATE users SET name = 'test'")
		return err
	})
	if err != nil {
		t.Fatalf("WithTx() error = %v", err)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %v", err)
	}
}

func TestTxManager_MaxRetriesExceeded(t *testing.T) {
	t.Parallel()

	pool, mock := newMockPool(t)
	defer mock.Close()

	txMgr := NewTxManager(pool,
		WithMaxRetries(2),
		WithRetryBaseDelay(time.Millisecond),
		WithRetryMaxDelay(5*time.Millisecond),
	)
	ctx := context.Background()

	// All attempts fail with serialization error
	for i := 0; i < 3; i++ {
		mock.ExpectBegin()
		mock.ExpectExec("UPDATE").
			WillReturnError(&pgconn.PgError{Code: "40001", Message: "could not serialize access"})
		mock.ExpectRollback()
	}

	err := txMgr.WithTx(ctx, func(tx Tx) error {
		_, err := tx.Exec(ctx, "UPDATE users SET name = 'test'")
		return err
	})
	if err == nil {
		t.Error("expected error after max retries")
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %v", err)
	}
}

// =============================================================================
// Error Mapping Tests with pgxmock
// =============================================================================

func TestErrorMapping_DuplicateKey(t *testing.T) {
	t.Parallel()

	pool, mock := newMockPool(t)
	defer mock.Close()

	ctx := context.Background()

	mock.ExpectExec("INSERT").
		WithArgs("test@example.com").
		WillReturnError(&pgconn.PgError{
			Code:           "23505",
			Message:        "duplicate key value violates unique constraint",
			ConstraintName: "users_email_key",
		})

	_, err := pool.Exec(ctx, "INSERT INTO users (email) VALUES ($1)", "test@example.com")
	if err == nil {
		t.Fatal("expected error")
	}
	if !IsDuplicate(err) {
		t.Errorf("IsDuplicate() = false, want true for error: %v", err)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %v", err)
	}
}

func TestErrorMapping_ForeignKey(t *testing.T) {
	t.Parallel()

	pool, mock := newMockPool(t)
	defer mock.Close()

	ctx := context.Background()

	mock.ExpectExec("INSERT").
		WithArgs(999).
		WillReturnError(&pgconn.PgError{
			Code:           "23503",
			Message:        "insert or update on table violates foreign key constraint",
			ConstraintName: "orders_user_id_fkey",
		})

	_, err := pool.Exec(ctx, "INSERT INTO orders (user_id) VALUES ($1)", 999)
	if err == nil {
		t.Fatal("expected error")
	}
	if !IsForeignKey(err) {
		t.Errorf("IsForeignKey() = false, want true for error: %v", err)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %v", err)
	}
}

func TestErrorMapping_CheckViolation(t *testing.T) {
	t.Parallel()

	pool, mock := newMockPool(t)
	defer mock.Close()

	ctx := context.Background()

	mock.ExpectExec("INSERT").
		WithArgs(-10).
		WillReturnError(&pgconn.PgError{
			Code:    "23514",
			Message: "new row violates check constraint",
		})

	_, err := pool.Exec(ctx, "INSERT INTO products (price) VALUES ($1)", -10)
	if err == nil {
		t.Fatal("expected error")
	}
	if !IsCode(err, CodeCheckViolation) {
		t.Errorf("expected CodeCheckViolation, got error: %v", err)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %v", err)
	}
}

// =============================================================================
// Query Builder Integration with Mock
// =============================================================================

func TestQueryBuilder_Select_WithMock(t *testing.T) {
	t.Parallel()

	pool, mock := newMockPool(t)
	defer mock.Close()

	ctx := context.Background()

	sql, args, err := Select("users").
		Columns("id", "name", "email").
		Where("active", true).
		OrderByAsc("name").
		Limit(10).
		Build()
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}

	rows := pgxmock.NewRows([]string{"id", "name", "email"}).
		AddRow(1, "Alice", "alice@example.com").
		AddRow(2, "Bob", "bob@example.com")

	mock.ExpectQuery("SELECT id, name, email FROM users").
		WithArgs(true).
		WillReturnRows(rows)

	result, err := pool.Query(ctx, sql, args...)
	if err != nil {
		t.Fatalf("Query() error = %v", err)
	}
	defer result.Close()

	count := 0
	for result.Next() {
		count++
	}
	if count != 2 {
		t.Errorf("expected 2 rows, got %d", count)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %v", err)
	}
}

func TestQueryBuilder_Insert_WithMock(t *testing.T) {
	t.Parallel()

	pool, mock := newMockPool(t)
	defer mock.Close()

	ctx := context.Background()

	sql, args, err := Insert("users").
		Columns("name", "email").
		Values("Charlie", "charlie@example.com").
		Build()
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}

	mock.ExpectExec("INSERT INTO users").
		WithArgs("Charlie", "charlie@example.com").
		WillReturnResult(pgxmock.NewResult("INSERT", 1))

	tag, err := pool.Exec(ctx, sql, args...)
	if err != nil {
		t.Fatalf("Exec() error = %v", err)
	}
	if tag.RowsAffected() != 1 {
		t.Errorf("RowsAffected() = %d, want 1", tag.RowsAffected())
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %v", err)
	}
}

func TestQueryBuilder_Update_WithMock(t *testing.T) {
	t.Parallel()

	pool, mock := newMockPool(t)
	defer mock.Close()

	ctx := context.Background()

	sql, args, err := Update("users").
		Set("active", false).
		Where("id", 1).
		Build()
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}

	mock.ExpectExec("UPDATE users SET").
		WithArgs(false, 1).
		WillReturnResult(pgxmock.NewResult("UPDATE", 1))

	tag, err := pool.Exec(ctx, sql, args...)
	if err != nil {
		t.Fatalf("Exec() error = %v", err)
	}
	if tag.RowsAffected() != 1 {
		t.Errorf("RowsAffected() = %d, want 1", tag.RowsAffected())
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %v", err)
	}
}

func TestQueryBuilder_Delete_WithMock(t *testing.T) {
	t.Parallel()

	pool, mock := newMockPool(t)
	defer mock.Close()

	ctx := context.Background()

	sql, args, err := Delete("users").
		Where("id", 1).
		Build()
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}

	mock.ExpectExec("DELETE FROM users").
		WithArgs(1).
		WillReturnResult(pgxmock.NewResult("DELETE", 1))

	tag, err := pool.Exec(ctx, sql, args...)
	if err != nil {
		t.Fatalf("Exec() error = %v", err)
	}
	if tag.RowsAffected() != 1 {
		t.Errorf("RowsAffected() = %d, want 1", tag.RowsAffected())
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %v", err)
	}
}

// =============================================================================
// Additional TxManager Tests
// =============================================================================

func TestTxManager_ContextCancellation(t *testing.T) {
	t.Parallel()

	pool, mock := newMockPool(t)
	defer mock.Close()

	txMgr := NewTxManager(pool,
		WithMaxRetries(2),
		WithRetryBaseDelay(100*time.Millisecond),
	)

	// Create a context that's already cancelled
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	// First attempt fails with retryable error
	mock.ExpectBegin()
	mock.ExpectExec("UPDATE").
		WillReturnError(&pgconn.PgError{Code: "40001", Message: "serialization failure"})
	mock.ExpectRollback()

	err := txMgr.WithTx(ctx, func(tx Tx) error {
		_, err := tx.Exec(ctx, "UPDATE users SET name = 'test'")
		return err
	})

	// Should fail due to context cancellation during retry wait
	if err == nil {
		t.Error("expected error due to context cancellation")
	}
}

func TestTxManager_NonRetryableError(t *testing.T) {
	t.Parallel()

	pool, mock := newMockPool(t)
	defer mock.Close()

	txMgr := NewTxManager(pool, WithMaxRetries(3))
	ctx := context.Background()

	// Non-retryable error (foreign key violation)
	mock.ExpectBegin()
	mock.ExpectExec("INSERT").
		WithArgs(999).
		WillReturnError(&pgconn.PgError{Code: "23503", Message: "foreign key violation"})
	mock.ExpectRollback()

	err := txMgr.WithTx(ctx, func(tx Tx) error {
		_, err := tx.Exec(ctx, "INSERT INTO orders (user_id) VALUES ($1)", 999)
		return err
	})

	if err == nil {
		t.Error("expected error")
	}
	// Should not retry - only one attempt
	if mockErr := mock.ExpectationsWereMet(); mockErr != nil {
		t.Errorf("unfulfilled expectations (means retry happened): %v", mockErr)
	}
}

func TestTxManager_RollbackError(t *testing.T) {
	t.Parallel()

	pool, mock := newMockPool(t)
	defer mock.Close()

	txMgr := NewTxManager(pool)
	ctx := context.Background()

	mock.ExpectBegin()
	mock.ExpectExec("INSERT").
		WithArgs("test").
		WillReturnError(errors.New("insert failed"))
	mock.ExpectRollback().WillReturnError(errors.New("rollback also failed"))

	fnErr := errors.New("insert failed")
	err := txMgr.WithTx(ctx, func(tx Tx) error {
		_, err := tx.Exec(ctx, "INSERT INTO users (name) VALUES ($1)", "test")
		if err != nil {
			return fnErr
		}
		return nil
	})

	// Should return the original error even if rollback fails
	if !errors.Is(err, fnErr) {
		t.Errorf("expected original error, got: %v", err)
	}

	if mockErr := mock.ExpectationsWereMet(); mockErr != nil {
		t.Errorf("unfulfilled expectations: %v", mockErr)
	}
}

func TestMockPool_Stat(t *testing.T) {
	t.Parallel()

	pool, mock := newMockPool(t)
	defer mock.Close()

	stats := pool.Stat()
	// Mock pool returns empty stats
	if stats.TotalConns != 0 {
		t.Errorf("TotalConns = %d, want 0", stats.TotalConns)
	}
}

func TestMockPool_Close(t *testing.T) {
	t.Parallel()

	pool, mock := newMockPool(t)
	mock.ExpectClose()

	pool.Close()

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %v", err)
	}
}

func TestMockPool_Acquire_NotImplemented(t *testing.T) {
	t.Parallel()

	pool, mock := newMockPool(t)
	defer mock.Close()

	ctx := context.Background()
	_, err := pool.Acquire(ctx)
	if err == nil {
		t.Error("expected error for unimplemented Acquire")
	}
}

func TestMockTx_NestedTransaction(t *testing.T) {
	t.Parallel()

	pool, mock := newMockPool(t)
	defer mock.Close()

	ctx := context.Background()

	mock.ExpectBegin()
	mock.ExpectBegin() // Nested/savepoint
	mock.ExpectExec("INSERT").
		WithArgs("nested").
		WillReturnResult(pgxmock.NewResult("INSERT", 1))
	mock.ExpectCommit() // Inner commit
	mock.ExpectCommit() // Outer commit

	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatalf("Begin() error = %v", err)
	}

	// Begin nested transaction
	nestedTx, err := tx.Begin(ctx)
	if err != nil {
		t.Fatalf("nested Begin() error = %v", err)
	}

	_, err = nestedTx.Exec(ctx, "INSERT INTO users (name) VALUES ($1)", "nested")
	if err != nil {
		t.Fatalf("Exec() error = %v", err)
	}

	err = nestedTx.Commit(ctx)
	if err != nil {
		t.Fatalf("nested Commit() error = %v", err)
	}

	err = tx.Commit(ctx)
	if err != nil {
		t.Fatalf("Commit() error = %v", err)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %v", err)
	}
}

func TestMockTx_Query_Error(t *testing.T) {
	t.Parallel()

	pool, mock := newMockPool(t)
	defer mock.Close()

	ctx := context.Background()

	mock.ExpectBegin()
	mock.ExpectQuery("SELECT").WillReturnError(errors.New("query failed"))
	mock.ExpectRollback()

	tx, _ := pool.Begin(ctx)

	//nolint:sqlclosecheck // Testing error path, no rows returned
	_, err := tx.Query(ctx, "SELECT * FROM users")
	if err == nil {
		t.Error("expected error")
	}

	_ = tx.Rollback(ctx)

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %v", err)
	}
}

func TestMockTx_Begin_NestedError(t *testing.T) {
	t.Parallel()

	pool, mock := newMockPool(t)
	defer mock.Close()

	ctx := context.Background()

	mock.ExpectBegin()
	mock.ExpectBegin().WillReturnError(errors.New("cannot create savepoint"))
	mock.ExpectRollback()

	tx, _ := pool.Begin(ctx)

	_, err := tx.Begin(ctx)
	if err == nil {
		t.Error("expected error for nested begin")
	}

	_ = tx.Rollback(ctx)

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %v", err)
	}
}

func TestTxManager_PanicRecovery(t *testing.T) {
	t.Parallel()

	pool, mock := newMockPool(t)
	defer mock.Close()

	txMgr := NewTxManager(pool)
	ctx := context.Background()

	mock.ExpectBegin()
	mock.ExpectRollback()

	defer func() {
		if r := recover(); r == nil {
			t.Error("expected panic to propagate")
		} else if r != "test panic" {
			t.Errorf("expected 'test panic', got %v", r)
		}
		// Verify mock expectations after panic recovery
		if err := mock.ExpectationsWereMet(); err != nil {
			t.Errorf("unfulfilled expectations: %v", err)
		}
	}()

	_ = txMgr.WithTx(ctx, func(tx Tx) error {
		panic("test panic")
	})
}

func TestTxManager_CommitError(t *testing.T) {
	t.Parallel()

	pool, mock := newMockPool(t)
	defer mock.Close()

	txMgr := NewTxManager(pool)
	ctx := context.Background()

	mock.ExpectBegin()
	mock.ExpectExec("INSERT").
		WithArgs("test").
		WillReturnResult(pgxmock.NewResult("INSERT", 1))
	mock.ExpectCommit().WillReturnError(errors.New("commit failed"))

	err := txMgr.WithTx(ctx, func(tx Tx) error {
		_, err := tx.Exec(ctx, "INSERT INTO users (name) VALUES ($1)", "test")
		return err
	})

	if err == nil {
		t.Error("expected commit error")
	}

	if mockErr := mock.ExpectationsWereMet(); mockErr != nil {
		t.Errorf("unfulfilled expectations: %v", mockErr)
	}
}

func TestTxManager_BeginTxError(t *testing.T) {
	t.Parallel()

	pool, mock := newMockPool(t)
	defer mock.Close()

	txMgr := NewTxManager(pool)
	ctx := context.Background()

	mock.ExpectBeginTx(pgx.TxOptions{IsoLevel: pgx.Serializable}).
		WillReturnError(errors.New("cannot begin tx"))

	err := txMgr.WithTxOptions(ctx, pgx.TxOptions{IsoLevel: pgx.Serializable}, func(tx Tx) error {
		return nil
	})

	if err == nil {
		t.Error("expected error")
	}

	if mockErr := mock.ExpectationsWereMet(); mockErr != nil {
		t.Errorf("unfulfilled expectations: %v", mockErr)
	}
}
