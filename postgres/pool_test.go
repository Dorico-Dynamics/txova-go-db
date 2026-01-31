package postgres

import (
	"errors"
	"testing"
	"time"

	"github.com/Dorico-Dynamics/txova-go-core/config"
	"github.com/Dorico-Dynamics/txova-go-core/logging"
)

func TestDefaultPoolConfig(t *testing.T) {
	t.Parallel()

	cfg := DefaultPoolConfig()

	if cfg.MaxConns != 25 {
		t.Errorf("MaxConns = %d, want 25", cfg.MaxConns)
	}
	if cfg.MinConns != 5 {
		t.Errorf("MinConns = %d, want 5", cfg.MinConns)
	}
	if cfg.MaxConnLifetime != time.Hour {
		t.Errorf("MaxConnLifetime = %v, want %v", cfg.MaxConnLifetime, time.Hour)
	}
	if cfg.MaxConnIdleTime != 30*time.Minute {
		t.Errorf("MaxConnIdleTime = %v, want %v", cfg.MaxConnIdleTime, 30*time.Minute)
	}
	if cfg.HealthCheckPeriod != time.Minute {
		t.Errorf("HealthCheckPeriod = %v, want %v", cfg.HealthCheckPeriod, time.Minute)
	}
	if cfg.ConnectTimeout != 5*time.Second {
		t.Errorf("ConnectTimeout = %v, want %v", cfg.ConnectTimeout, 5*time.Second)
	}
	if cfg.SlowQueryThreshold != time.Second {
		t.Errorf("SlowQueryThreshold = %v, want %v", cfg.SlowQueryThreshold, time.Second)
	}
	if cfg.Logger == nil {
		t.Error("Logger should not be nil")
	}
}

func TestPoolConfigOptions(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		opt      Option
		validate func(t *testing.T, cfg PoolConfig)
	}{
		{
			name: "WithConnString",
			opt:  WithConnString("postgres://localhost/test"),
			validate: func(t *testing.T, cfg PoolConfig) {
				if cfg.ConnString != "postgres://localhost/test" {
					t.Errorf("ConnString = %q, want %q", cfg.ConnString, "postgres://localhost/test")
				}
			},
		},
		{
			name: "WithMaxConns",
			opt:  WithMaxConns(50),
			validate: func(t *testing.T, cfg PoolConfig) {
				if cfg.MaxConns != 50 {
					t.Errorf("MaxConns = %d, want 50", cfg.MaxConns)
				}
			},
		},
		{
			name: "WithMinConns",
			opt:  WithMinConns(10),
			validate: func(t *testing.T, cfg PoolConfig) {
				if cfg.MinConns != 10 {
					t.Errorf("MinConns = %d, want 10", cfg.MinConns)
				}
			},
		},
		{
			name: "WithMaxConnLifetime",
			opt:  WithMaxConnLifetime(2 * time.Hour),
			validate: func(t *testing.T, cfg PoolConfig) {
				if cfg.MaxConnLifetime != 2*time.Hour {
					t.Errorf("MaxConnLifetime = %v, want %v", cfg.MaxConnLifetime, 2*time.Hour)
				}
			},
		},
		{
			name: "WithMaxConnIdleTime",
			opt:  WithMaxConnIdleTime(15 * time.Minute),
			validate: func(t *testing.T, cfg PoolConfig) {
				if cfg.MaxConnIdleTime != 15*time.Minute {
					t.Errorf("MaxConnIdleTime = %v, want %v", cfg.MaxConnIdleTime, 15*time.Minute)
				}
			},
		},
		{
			name: "WithHealthCheckPeriod",
			opt:  WithHealthCheckPeriod(30 * time.Second),
			validate: func(t *testing.T, cfg PoolConfig) {
				if cfg.HealthCheckPeriod != 30*time.Second {
					t.Errorf("HealthCheckPeriod = %v, want %v", cfg.HealthCheckPeriod, 30*time.Second)
				}
			},
		},
		{
			name: "WithConnectTimeout",
			opt:  WithConnectTimeout(10 * time.Second),
			validate: func(t *testing.T, cfg PoolConfig) {
				if cfg.ConnectTimeout != 10*time.Second {
					t.Errorf("ConnectTimeout = %v, want %v", cfg.ConnectTimeout, 10*time.Second)
				}
			},
		},
		{
			name: "WithSlowQueryThreshold",
			opt:  WithSlowQueryThreshold(500 * time.Millisecond),
			validate: func(t *testing.T, cfg PoolConfig) {
				if cfg.SlowQueryThreshold != 500*time.Millisecond {
					t.Errorf("SlowQueryThreshold = %v, want %v", cfg.SlowQueryThreshold, 500*time.Millisecond)
				}
			},
		},
		{
			name: "WithLogger",
			opt:  WithLogger(logging.Default()),
			validate: func(t *testing.T, cfg PoolConfig) {
				if cfg.Logger == nil {
					t.Error("Logger should not be nil")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			cfg := DefaultPoolConfig()
			tt.opt(&cfg)
			tt.validate(t, cfg)
		})
	}
}

func TestPoolConfig_Validate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		cfg     PoolConfig
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid config",
			cfg: PoolConfig{
				ConnString: "postgres://localhost/test",
				MaxConns:   25,
				MinConns:   5,
			},
			wantErr: false,
		},
		{
			name: "missing connection string",
			cfg: PoolConfig{
				ConnString: "",
				MaxConns:   25,
				MinConns:   5,
			},
			wantErr: true,
			errMsg:  "connection string is required",
		},
		{
			name: "zero max connections",
			cfg: PoolConfig{
				ConnString: "postgres://localhost/test",
				MaxConns:   0,
				MinConns:   5,
			},
			wantErr: true,
			errMsg:  "max connections must be at least 1",
		},
		{
			name: "negative min connections",
			cfg: PoolConfig{
				ConnString: "postgres://localhost/test",
				MaxConns:   25,
				MinConns:   -1,
			},
			wantErr: true,
			errMsg:  "min connections cannot be negative",
		},
		{
			name: "min exceeds max",
			cfg: PoolConfig{
				ConnString: "postgres://localhost/test",
				MaxConns:   5,
				MinConns:   10,
			},
			wantErr: true,
			errMsg:  "min connections (10) cannot exceed max connections (5)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := tt.cfg.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.wantErr && err.Error() != tt.errMsg {
				t.Errorf("Validate() error = %q, want %q", err.Error(), tt.errMsg)
			}
		})
	}
}

func TestFromDatabaseConfig(t *testing.T) {
	t.Parallel()

	dbCfg := &config.DatabaseConfig{
		Host:           "localhost",
		Port:           5432,
		Name:           "testdb",
		User:           "testuser",
		Password:       "testpass",
		MaxConnections: 50,
		SSLMode:        "disable",
	}

	cfg := FromDatabaseConfig(dbCfg)

	expectedDSN := "host=localhost port=5432 user=testuser password=testpass dbname=testdb sslmode=disable"
	if cfg.ConnString != expectedDSN {
		t.Errorf("ConnString = %q, want %q", cfg.ConnString, expectedDSN)
	}
	if cfg.MaxConns != 50 {
		t.Errorf("MaxConns = %d, want 50", cfg.MaxConns)
	}
	// Other defaults should still be set
	if cfg.MinConns != 5 {
		t.Errorf("MinConns = %d, want 5 (default)", cfg.MinConns)
	}
}

func TestFromDatabaseConfig_WithOptions(t *testing.T) {
	t.Parallel()

	dbCfg := &config.DatabaseConfig{
		Host:           "localhost",
		Port:           5432,
		Name:           "testdb",
		User:           "testuser",
		Password:       "testpass",
		MaxConnections: 50,
		SSLMode:        "require",
	}

	customLogger := logging.Default()
	cfg := FromDatabaseConfig(dbCfg,
		WithMinConns(10),
		WithSlowQueryThreshold(2*time.Second),
		WithLogger(customLogger),
	)

	if cfg.MinConns != 10 {
		t.Errorf("MinConns = %d, want 10", cfg.MinConns)
	}
	if cfg.SlowQueryThreshold != 2*time.Second {
		t.Errorf("SlowQueryThreshold = %v, want %v", cfg.SlowQueryThreshold, 2*time.Second)
	}
	if cfg.Logger != customLogger {
		t.Error("Logger should be the custom logger")
	}
}

func TestTruncateSQL(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "short SQL",
			input: "SELECT * FROM users",
			want:  "SELECT * FROM users",
		},
		{
			name:  "exactly 200 chars",
			input: string(make([]byte, 200)),
			want:  string(make([]byte, 200)),
		},
		{
			name:  "exceeds 200 chars",
			input: string(make([]byte, 250)),
			want:  string(make([]byte, 200)) + "...",
		},
		{
			name:  "empty string",
			input: "",
			want:  "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := truncateSQL(tt.input)
			if got != tt.want {
				t.Errorf("truncateSQL() len = %d, want len %d", len(got), len(tt.want))
			}
		})
	}
}

func TestNewPool_ValidationError(t *testing.T) {
	t.Parallel()

	ctx := t.Context()

	// Test with invalid configuration (missing connection string)
	_, err := NewPool(ctx, WithMaxConns(10))

	if err == nil {
		t.Fatal("expected error for missing connection string")
	}

	// Verify it's a database error with the correct code
	var dbErr *Error
	if !errors.As(err, &dbErr) {
		t.Fatalf("expected *Error, got %T", err)
	}
	if dbErr.Code() != CodeConnection {
		t.Errorf("Code() = %v, want %v", dbErr.Code(), CodeConnection)
	}
}

func TestNewPoolFromConfig_ValidationError(t *testing.T) {
	t.Parallel()

	ctx := t.Context()

	// Test with invalid configuration (min > max)
	cfg := PoolConfig{
		ConnString: "postgres://localhost/test",
		MaxConns:   5,
		MinConns:   10,
	}

	_, err := NewPoolFromConfig(ctx, cfg)

	if err == nil {
		t.Fatal("expected error for min > max")
	}

	// Verify it's a database error with the correct code
	var dbErr *Error
	if !errors.As(err, &dbErr) {
		t.Fatalf("expected *Error, got %T", err)
	}
	if dbErr.Code() != CodeConnection {
		t.Errorf("Code() = %v, want %v", dbErr.Code(), CodeConnection)
	}
}

func TestPoolConfig_MultipleOptions(t *testing.T) {
	t.Parallel()

	cfg := DefaultPoolConfig()

	// Apply multiple options
	opts := []Option{
		WithConnString("postgres://localhost/test"),
		WithMaxConns(100),
		WithMinConns(20),
		WithMaxConnLifetime(2 * time.Hour),
		WithSlowQueryThreshold(500 * time.Millisecond),
	}

	for _, opt := range opts {
		opt(&cfg)
	}

	if cfg.ConnString != "postgres://localhost/test" {
		t.Errorf("ConnString = %q, want %q", cfg.ConnString, "postgres://localhost/test")
	}
	if cfg.MaxConns != 100 {
		t.Errorf("MaxConns = %d, want 100", cfg.MaxConns)
	}
	if cfg.MinConns != 20 {
		t.Errorf("MinConns = %d, want 20", cfg.MinConns)
	}
	if cfg.MaxConnLifetime != 2*time.Hour {
		t.Errorf("MaxConnLifetime = %v, want %v", cfg.MaxConnLifetime, 2*time.Hour)
	}
	if cfg.SlowQueryThreshold != 500*time.Millisecond {
		t.Errorf("SlowQueryThreshold = %v, want %v", cfg.SlowQueryThreshold, 500*time.Millisecond)
	}
}
