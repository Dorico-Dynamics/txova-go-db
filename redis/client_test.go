package redis

import (
	"log/slog"
	"testing"
	"time"

	"github.com/Dorico-Dynamics/txova-go-core/config"
)

func TestDefaultConfig(t *testing.T) {
	t.Parallel()

	cfg := DefaultConfig()

	if len(cfg.Addresses) != 1 || cfg.Addresses[0] != "localhost:6379" {
		t.Errorf("Addresses = %v, want [localhost:6379]", cfg.Addresses)
	}
	if cfg.DB != 0 {
		t.Errorf("DB = %d, want 0", cfg.DB)
	}
	if cfg.PoolSize != DefaultPoolSize {
		t.Errorf("PoolSize = %d, want %d", cfg.PoolSize, DefaultPoolSize)
	}
	if cfg.MinIdleConns != DefaultMinIdleConns {
		t.Errorf("MinIdleConns = %d, want %d", cfg.MinIdleConns, DefaultMinIdleConns)
	}
	if cfg.ConnMaxLifetime != DefaultConnMaxLifetime {
		t.Errorf("ConnMaxLifetime = %v, want %v", cfg.ConnMaxLifetime, DefaultConnMaxLifetime)
	}
	if cfg.ConnMaxIdleTime != DefaultConnMaxIdleTime {
		t.Errorf("ConnMaxIdleTime = %v, want %v", cfg.ConnMaxIdleTime, DefaultConnMaxIdleTime)
	}
	if cfg.DialTimeout != DefaultDialTimeout {
		t.Errorf("DialTimeout = %v, want %v", cfg.DialTimeout, DefaultDialTimeout)
	}
	if cfg.ReadTimeout != DefaultReadTimeout {
		t.Errorf("ReadTimeout = %v, want %v", cfg.ReadTimeout, DefaultReadTimeout)
	}
	if cfg.WriteTimeout != DefaultWriteTimeout {
		t.Errorf("WriteTimeout = %v, want %v", cfg.WriteTimeout, DefaultWriteTimeout)
	}
	if cfg.PoolTimeout != DefaultPoolTimeout {
		t.Errorf("PoolTimeout = %v, want %v", cfg.PoolTimeout, DefaultPoolTimeout)
	}
	if cfg.Mode != ModeStandalone {
		t.Errorf("Mode = %v, want %v", cfg.Mode, ModeStandalone)
	}
}

func TestMode_String(t *testing.T) {
	t.Parallel()

	tests := []struct {
		mode Mode
		want string
	}{
		{ModeStandalone, "standalone"},
		{ModeCluster, "cluster"},
		{ModeSentinel, "sentinel"},
		{Mode(99), "unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			t.Parallel()
			if got := tt.mode.String(); got != tt.want {
				t.Errorf("Mode.String() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestConfigOptions(t *testing.T) {
	t.Parallel()

	t.Run("WithAddress", func(t *testing.T) {
		t.Parallel()
		cfg := DefaultConfig()
		WithAddress("redis.example.com:6380")(cfg)
		if len(cfg.Addresses) != 1 || cfg.Addresses[0] != "redis.example.com:6380" {
			t.Errorf("Addresses = %v, want [redis.example.com:6380]", cfg.Addresses)
		}
	})

	t.Run("WithAddresses", func(t *testing.T) {
		t.Parallel()
		cfg := DefaultConfig()
		WithAddresses("node1:6379", "node2:6379", "node3:6379")(cfg)
		if len(cfg.Addresses) != 3 {
			t.Errorf("len(Addresses) = %d, want 3", len(cfg.Addresses))
		}
	})

	t.Run("WithPassword", func(t *testing.T) {
		t.Parallel()
		cfg := DefaultConfig()
		WithPassword("secret123")(cfg)
		if cfg.Password != "secret123" {
			t.Errorf("Password = %q, want %q", cfg.Password, "secret123")
		}
	})

	t.Run("WithDB", func(t *testing.T) {
		t.Parallel()
		cfg := DefaultConfig()
		WithDB(5)(cfg)
		if cfg.DB != 5 {
			t.Errorf("DB = %d, want 5", cfg.DB)
		}
	})

	t.Run("WithPoolSize", func(t *testing.T) {
		t.Parallel()
		cfg := DefaultConfig()
		WithPoolSize(50)(cfg)
		if cfg.PoolSize != 50 {
			t.Errorf("PoolSize = %d, want 50", cfg.PoolSize)
		}
	})

	t.Run("WithMinIdleConns", func(t *testing.T) {
		t.Parallel()
		cfg := DefaultConfig()
		WithMinIdleConns(5)(cfg)
		if cfg.MinIdleConns != 5 {
			t.Errorf("MinIdleConns = %d, want 5", cfg.MinIdleConns)
		}
	})

	t.Run("WithConnMaxLifetime", func(t *testing.T) {
		t.Parallel()
		cfg := DefaultConfig()
		WithConnMaxLifetime(time.Hour)(cfg)
		if cfg.ConnMaxLifetime != time.Hour {
			t.Errorf("ConnMaxLifetime = %v, want %v", cfg.ConnMaxLifetime, time.Hour)
		}
	})

	t.Run("WithConnMaxIdleTime", func(t *testing.T) {
		t.Parallel()
		cfg := DefaultConfig()
		WithConnMaxIdleTime(time.Minute * 5)(cfg)
		if cfg.ConnMaxIdleTime != time.Minute*5 {
			t.Errorf("ConnMaxIdleTime = %v, want %v", cfg.ConnMaxIdleTime, time.Minute*5)
		}
	})

	t.Run("WithDialTimeout", func(t *testing.T) {
		t.Parallel()
		cfg := DefaultConfig()
		WithDialTimeout(time.Second * 10)(cfg)
		if cfg.DialTimeout != time.Second*10 {
			t.Errorf("DialTimeout = %v, want %v", cfg.DialTimeout, time.Second*10)
		}
	})

	t.Run("WithReadTimeout", func(t *testing.T) {
		t.Parallel()
		cfg := DefaultConfig()
		WithReadTimeout(time.Second * 5)(cfg)
		if cfg.ReadTimeout != time.Second*5 {
			t.Errorf("ReadTimeout = %v, want %v", cfg.ReadTimeout, time.Second*5)
		}
	})

	t.Run("WithWriteTimeout", func(t *testing.T) {
		t.Parallel()
		cfg := DefaultConfig()
		WithWriteTimeout(time.Second * 5)(cfg)
		if cfg.WriteTimeout != time.Second*5 {
			t.Errorf("WriteTimeout = %v, want %v", cfg.WriteTimeout, time.Second*5)
		}
	})

	t.Run("WithPoolTimeout", func(t *testing.T) {
		t.Parallel()
		cfg := DefaultConfig()
		WithPoolTimeout(time.Second * 6)(cfg)
		if cfg.PoolTimeout != time.Second*6 {
			t.Errorf("PoolTimeout = %v, want %v", cfg.PoolTimeout, time.Second*6)
		}
	})

	t.Run("WithMode", func(t *testing.T) {
		t.Parallel()
		cfg := DefaultConfig()
		WithMode(ModeCluster)(cfg)
		if cfg.Mode != ModeCluster {
			t.Errorf("Mode = %v, want %v", cfg.Mode, ModeCluster)
		}
	})

	t.Run("WithMasterName", func(t *testing.T) {
		t.Parallel()
		cfg := DefaultConfig()
		WithMasterName("mymaster")(cfg)
		if cfg.MasterName != "mymaster" {
			t.Errorf("MasterName = %q, want %q", cfg.MasterName, "mymaster")
		}
	})

	t.Run("WithTLS", func(t *testing.T) {
		t.Parallel()
		cfg := DefaultConfig()
		WithTLS(true)(cfg)
		if !cfg.TLSEnabled {
			t.Error("TLSEnabled should be true")
		}
	})
}

func TestConfig_Validate(t *testing.T) {
	t.Parallel()

	t.Run("valid config", func(t *testing.T) {
		t.Parallel()
		cfg := DefaultConfig()
		if err := cfg.Validate(); err != nil {
			t.Errorf("Validate() error = %v, want nil", err)
		}
	})

	t.Run("no addresses", func(t *testing.T) {
		t.Parallel()
		cfg := DefaultConfig()
		cfg.Addresses = nil
		if err := cfg.Validate(); err == nil {
			t.Error("Validate() should fail with no addresses")
		}
	})

	t.Run("empty addresses", func(t *testing.T) {
		t.Parallel()
		cfg := DefaultConfig()
		cfg.Addresses = []string{}
		if err := cfg.Validate(); err == nil {
			t.Error("Validate() should fail with empty addresses")
		}
	})

	t.Run("sentinel without master name", func(t *testing.T) {
		t.Parallel()
		cfg := DefaultConfig()
		cfg.Mode = ModeSentinel
		cfg.MasterName = ""
		if err := cfg.Validate(); err == nil {
			t.Error("Validate() should fail for sentinel without master name")
		}
	})

	t.Run("sentinel with master name", func(t *testing.T) {
		t.Parallel()
		cfg := DefaultConfig()
		cfg.Mode = ModeSentinel
		cfg.MasterName = "mymaster"
		if err := cfg.Validate(); err != nil {
			t.Errorf("Validate() error = %v, want nil", err)
		}
	})

	t.Run("zero pool size", func(t *testing.T) {
		t.Parallel()
		cfg := DefaultConfig()
		cfg.PoolSize = 0
		if err := cfg.Validate(); err == nil {
			t.Error("Validate() should fail with zero pool size")
		}
	})

	t.Run("negative pool size", func(t *testing.T) {
		t.Parallel()
		cfg := DefaultConfig()
		cfg.PoolSize = -1
		if err := cfg.Validate(); err == nil {
			t.Error("Validate() should fail with negative pool size")
		}
	})
}

func TestNew(t *testing.T) {
	t.Parallel()

	t.Run("with defaults", func(t *testing.T) {
		t.Parallel()
		client, err := New()
		if err != nil {
			t.Fatalf("New() error = %v", err)
		}
		if client == nil {
			t.Fatal("New() returned nil client")
		}
		if client.config == nil {
			t.Error("client.config is nil")
		}
		if client.logger == nil {
			t.Error("client.logger is nil")
		}
	})

	t.Run("with options", func(t *testing.T) {
		t.Parallel()
		client, err := New(
			WithAddress("localhost:6380"),
			WithPassword("secret"),
			WithDB(1),
			WithPoolSize(20),
		)
		if err != nil {
			t.Fatalf("New() error = %v", err)
		}
		if client.config.Addresses[0] != "localhost:6380" {
			t.Errorf("Address = %q, want %q", client.config.Addresses[0], "localhost:6380")
		}
		if client.config.Password != "secret" {
			t.Errorf("Password = %q, want %q", client.config.Password, "secret")
		}
		if client.config.DB != 1 {
			t.Errorf("DB = %d, want 1", client.config.DB)
		}
		if client.config.PoolSize != 20 {
			t.Errorf("PoolSize = %d, want 20", client.config.PoolSize)
		}
	})

	t.Run("validation failure", func(t *testing.T) {
		t.Parallel()
		_, err := New(WithPoolSize(-1))
		if err == nil {
			t.Error("New() should fail with invalid config")
		}
	})
}

func TestNewWithConfig(t *testing.T) {
	t.Parallel()

	t.Run("standalone mode", func(t *testing.T) {
		t.Parallel()
		cfg := DefaultConfig()
		client, err := NewWithConfig(cfg)
		if err != nil {
			t.Fatalf("NewWithConfig() error = %v", err)
		}
		if client.client == nil {
			t.Error("client.client is nil")
		}
	})

	t.Run("cluster mode", func(t *testing.T) {
		t.Parallel()
		cfg := DefaultConfig()
		cfg.Mode = ModeCluster
		cfg.Addresses = []string{"node1:6379", "node2:6379"}
		client, err := NewWithConfig(cfg)
		if err != nil {
			t.Fatalf("NewWithConfig() error = %v", err)
		}
		if client.client == nil {
			t.Error("client.client is nil")
		}
	})

	t.Run("sentinel mode", func(t *testing.T) {
		t.Parallel()
		cfg := DefaultConfig()
		cfg.Mode = ModeSentinel
		cfg.MasterName = "mymaster"
		cfg.Addresses = []string{"sentinel1:26379", "sentinel2:26379"}
		client, err := NewWithConfig(cfg)
		if err != nil {
			t.Fatalf("NewWithConfig() error = %v", err)
		}
		if client.client == nil {
			t.Error("client.client is nil")
		}
	})

	t.Run("with logger option", func(t *testing.T) {
		t.Parallel()
		logger := slog.Default()
		cfg := DefaultConfig()
		client, err := NewWithConfig(cfg, WithLogger(logger))
		if err != nil {
			t.Fatalf("NewWithConfig() error = %v", err)
		}
		if client.logger != logger {
			t.Error("logger was not set correctly")
		}
	})
}

func TestNewFromCoreConfig(t *testing.T) {
	t.Parallel()

	t.Run("creates client from core config", func(t *testing.T) {
		t.Parallel()
		coreConfig := config.RedisConfig{
			Host:     "redis.example.com",
			Port:     6380,
			Password: "secret123",
			DB:       2,
			PoolSize: 25,
		}

		client, err := NewFromCoreConfig(coreConfig)
		if err != nil {
			t.Fatalf("NewFromCoreConfig() error = %v", err)
		}

		if len(client.config.Addresses) != 1 {
			t.Fatalf("len(Addresses) = %d, want 1", len(client.config.Addresses))
		}
		if client.config.Addresses[0] != "redis.example.com:6380" {
			t.Errorf("Address = %q, want %q", client.config.Addresses[0], "redis.example.com:6380")
		}
		if client.config.Password != "secret123" {
			t.Errorf("Password = %q, want %q", client.config.Password, "secret123")
		}
		if client.config.DB != 2 {
			t.Errorf("DB = %d, want 2", client.config.DB)
		}
		if client.config.PoolSize != 25 {
			t.Errorf("PoolSize = %d, want 25", client.config.PoolSize)
		}
	})

	t.Run("uses default values from core config", func(t *testing.T) {
		t.Parallel()
		coreConfig := config.RedisConfig{
			Host: "localhost",
			Port: 6379,
		}

		client, err := NewFromCoreConfig(coreConfig)
		if err != nil {
			t.Fatalf("NewFromCoreConfig() error = %v", err)
		}

		if client.config.MinIdleConns != DefaultMinIdleConns {
			t.Errorf("MinIdleConns = %d, want %d", client.config.MinIdleConns, DefaultMinIdleConns)
		}
		if client.config.ConnMaxLifetime != DefaultConnMaxLifetime {
			t.Errorf("ConnMaxLifetime = %v, want %v", client.config.ConnMaxLifetime, DefaultConnMaxLifetime)
		}
		if client.config.Mode != ModeStandalone {
			t.Errorf("Mode = %v, want %v", client.config.Mode, ModeStandalone)
		}
	})

	t.Run("with client options", func(t *testing.T) {
		t.Parallel()
		logger := slog.Default()
		coreConfig := config.RedisConfig{
			Host: "localhost",
			Port: 6379,
		}

		client, err := NewFromCoreConfig(coreConfig, WithLogger(logger))
		if err != nil {
			t.Fatalf("NewFromCoreConfig() error = %v", err)
		}

		if client.logger != logger {
			t.Error("logger was not set correctly")
		}
	})
}

func TestClient_Name(t *testing.T) {
	t.Parallel()

	client, err := New()
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	if got := client.Name(); got != "redis" {
		t.Errorf("Name() = %q, want %q", got, "redis")
	}
}

func TestClient_Client(t *testing.T) {
	t.Parallel()

	client, err := New()
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	underlying := client.Client()
	if underlying == nil {
		t.Error("Client() returned nil")
	}
}

func TestClient_Stats(t *testing.T) {
	t.Parallel()

	client, err := New()
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	stats := client.Stats()
	if stats == nil {
		t.Error("Stats() returned nil")
	}
}

func TestMetrics(t *testing.T) {
	t.Parallel()

	metrics := &Metrics{
		Hits:       100,
		Misses:     20,
		Timeouts:   5,
		TotalConns: 10,
		IdleConns:  3,
		StaleConns: 1,
	}

	if metrics.Hits != 100 {
		t.Errorf("Hits = %d, want 100", metrics.Hits)
	}
	if metrics.Misses != 20 {
		t.Errorf("Misses = %d, want 20", metrics.Misses)
	}
	if metrics.Timeouts != 5 {
		t.Errorf("Timeouts = %d, want 5", metrics.Timeouts)
	}
	if metrics.TotalConns != 10 {
		t.Errorf("TotalConns = %d, want 10", metrics.TotalConns)
	}
	if metrics.IdleConns != 3 {
		t.Errorf("IdleConns = %d, want 3", metrics.IdleConns)
	}
	if metrics.StaleConns != 1 {
		t.Errorf("StaleConns = %d, want 1", metrics.StaleConns)
	}
}

func TestDefaultConstants(t *testing.T) {
	t.Parallel()

	if DefaultPoolSize != 10 {
		t.Errorf("DefaultPoolSize = %d, want 10", DefaultPoolSize)
	}
	if DefaultMinIdleConns != 2 {
		t.Errorf("DefaultMinIdleConns = %d, want 2", DefaultMinIdleConns)
	}
	if DefaultConnMaxLifetime != 30*time.Minute {
		t.Errorf("DefaultConnMaxLifetime = %v, want %v", DefaultConnMaxLifetime, 30*time.Minute)
	}
	if DefaultConnMaxIdleTime != 10*time.Minute {
		t.Errorf("DefaultConnMaxIdleTime = %v, want %v", DefaultConnMaxIdleTime, 10*time.Minute)
	}
	if DefaultDialTimeout != 5*time.Second {
		t.Errorf("DefaultDialTimeout = %v, want %v", DefaultDialTimeout, 5*time.Second)
	}
	if DefaultReadTimeout != 3*time.Second {
		t.Errorf("DefaultReadTimeout = %v, want %v", DefaultReadTimeout, 3*time.Second)
	}
	if DefaultWriteTimeout != 3*time.Second {
		t.Errorf("DefaultWriteTimeout = %v, want %v", DefaultWriteTimeout, 3*time.Second)
	}
	if DefaultPoolTimeout != 4*time.Second {
		t.Errorf("DefaultPoolTimeout = %v, want %v", DefaultPoolTimeout, 4*time.Second)
	}
}
