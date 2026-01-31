package postgres

import (
	"testing"
	"testing/fstest"
	"time"

	"github.com/Dorico-Dynamics/txova-go-core/logging"
)

func TestDefaultMigratorConfig(t *testing.T) {
	t.Parallel()

	config := DefaultMigratorConfig()

	if config.TableName != "schema_migrations" {
		t.Errorf("expected TableName = %q, got %q", "schema_migrations", config.TableName)
	}

	if config.LockTimeout != 15*time.Second {
		t.Errorf("expected LockTimeout = %v, got %v", 15*time.Second, config.LockTimeout)
	}

	if config.Logger != nil {
		t.Errorf("expected Logger = nil, got %v", config.Logger)
	}
}

func TestWithMigrationsTable(t *testing.T) {
	t.Parallel()

	config := DefaultMigratorConfig()
	opt := WithMigrationsTable("custom_migrations")
	opt(&config)

	if config.TableName != "custom_migrations" {
		t.Errorf("expected TableName = %q, got %q", "custom_migrations", config.TableName)
	}
}

func TestWithLockTimeout(t *testing.T) {
	t.Parallel()

	config := DefaultMigratorConfig()
	opt := WithLockTimeout(30 * time.Second)
	opt(&config)

	if config.LockTimeout != 30*time.Second {
		t.Errorf("expected LockTimeout = %v, got %v", 30*time.Second, config.LockTimeout)
	}
}

func TestWithMigratorLogger(t *testing.T) {
	t.Parallel()

	logger := logging.New(logging.DefaultConfig())
	config := DefaultMigratorConfig()
	opt := WithMigratorLogger(logger)
	opt(&config)

	if config.Logger != logger {
		t.Errorf("expected Logger to be set")
	}
}

func TestNewMigrator_NilPool(t *testing.T) {
	t.Parallel()

	migrations := fstest.MapFS{
		"0001_initial.up.sql":   &fstest.MapFile{Data: []byte("CREATE TABLE test (id INT);")},
		"0001_initial.down.sql": &fstest.MapFile{Data: []byte("DROP TABLE test;")},
	}

	_, err := NewMigrator(nil, migrations)
	if err == nil {
		t.Fatal("expected error for nil pool")
	}

	expectedMsg := "pool cannot be nil"
	if err.Error() != expectedMsg {
		t.Errorf("expected error %q, got %q", expectedMsg, err.Error())
	}
}

func TestNewMigrator_NilPoolPrecedence(t *testing.T) {
	t.Parallel()

	// When both pool and migrations are nil, pool is validated first.
	// This test verifies the validation order: pool is checked before migrations.
	_, err := NewMigrator(nil, nil)
	if err == nil {
		t.Fatal("expected error for nil pool")
	}

	// Pool validation takes precedence over migrations validation
	expectedMsg := "pool cannot be nil"
	if err.Error() != expectedMsg {
		t.Errorf("expected error %q, got %q", expectedMsg, err.Error())
	}
}

func TestMigratorConfig_MultipleOptions(t *testing.T) {
	t.Parallel()

	logger := logging.New(logging.DefaultConfig())
	config := DefaultMigratorConfig()

	opts := []MigratorOption{
		WithMigrationsTable("my_migrations"),
		WithLockTimeout(60 * time.Second),
		WithMigratorLogger(logger),
	}

	for _, opt := range opts {
		opt(&config)
	}

	if config.TableName != "my_migrations" {
		t.Errorf("expected TableName = %q, got %q", "my_migrations", config.TableName)
	}

	if config.LockTimeout != 60*time.Second {
		t.Errorf("expected LockTimeout = %v, got %v", 60*time.Second, config.LockTimeout)
	}

	if config.Logger != logger {
		t.Errorf("expected Logger to be set")
	}
}

func TestMigrator_LogHelpers(t *testing.T) {
	t.Parallel()

	// Test that log helpers don't panic with nil logger
	m := &Migrator{
		config: MigratorConfig{
			Logger: nil,
		},
	}

	// These should not panic
	m.log("test message", "key", "value")
	m.logError("test error", testError("test"))
}

type testError string

func (e testError) Error() string { return string(e) }
