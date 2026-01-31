// Package redis provides Redis client utilities for the Txova platform.
package redis

import (
	"context"
	"log/slog"
	"time"

	"github.com/Dorico-Dynamics/txova-go-core/config"
	"github.com/redis/go-redis/v9"
)

// Default configuration values.
const (
	// DefaultPoolSize is the default number of connections in the pool.
	DefaultPoolSize = 10
	// DefaultMinIdleConns is the default minimum number of idle connections.
	DefaultMinIdleConns = 2
	// DefaultConnMaxLifetime is the default maximum lifetime of a connection.
	DefaultConnMaxLifetime = 30 * time.Minute
	// DefaultConnMaxIdleTime is the default maximum idle time for a connection.
	DefaultConnMaxIdleTime = 10 * time.Minute
	// DefaultDialTimeout is the default timeout for establishing connections.
	DefaultDialTimeout = 5 * time.Second
	// DefaultReadTimeout is the default timeout for read operations.
	DefaultReadTimeout = 3 * time.Second
	// DefaultWriteTimeout is the default timeout for write operations.
	DefaultWriteTimeout = 3 * time.Second
	// DefaultPoolTimeout is the default timeout for getting a connection from the pool.
	DefaultPoolTimeout = 4 * time.Second
)

// Client wraps the go-redis client with Txova-specific functionality.
type Client struct {
	client redis.UniversalClient
	config *Config
	logger *slog.Logger
}

// Config holds the configuration for the Redis client.
type Config struct {
	// Addresses is the list of Redis addresses (host:port).
	// For standalone mode, use a single address.
	// For cluster mode, provide multiple addresses.
	// For sentinel mode, provide sentinel addresses.
	Addresses []string

	// Password is the Redis password (AUTH).
	Password string

	// DB is the database number to select (standalone mode only).
	DB int

	// PoolSize is the maximum number of connections in the pool.
	// Default: 10
	PoolSize int

	// MinIdleConns is the minimum number of idle connections.
	// Default: 2
	MinIdleConns int

	// ConnMaxLifetime is the maximum lifetime of a connection.
	// Default: 30 minutes
	ConnMaxLifetime time.Duration

	// ConnMaxIdleTime is the maximum idle time for a connection.
	// Default: 10 minutes
	ConnMaxIdleTime time.Duration

	// DialTimeout is the timeout for establishing new connections.
	// Default: 5 seconds
	DialTimeout time.Duration

	// ReadTimeout is the timeout for read operations.
	// Default: 3 seconds
	ReadTimeout time.Duration

	// WriteTimeout is the timeout for write operations.
	// Default: 3 seconds
	WriteTimeout time.Duration

	// PoolTimeout is the timeout for getting a connection from the pool.
	// Default: 4 seconds
	PoolTimeout time.Duration

	// Mode specifies the Redis deployment mode.
	// Default: Standalone
	Mode Mode

	// MasterName is the name of the master (sentinel mode only).
	MasterName string

	// TLSConfig enables TLS for connections.
	TLSEnabled bool
}

// Mode represents the Redis deployment mode.
type Mode int

const (
	// ModeStandalone is a single Redis server.
	ModeStandalone Mode = iota
	// ModeCluster is a Redis cluster.
	ModeCluster
	// ModeSentinel is Redis with Sentinel for high availability.
	ModeSentinel
)

// String returns the string representation of the mode.
func (m Mode) String() string {
	switch m {
	case ModeStandalone:
		return "standalone"
	case ModeCluster:
		return "cluster"
	case ModeSentinel:
		return "sentinel"
	default:
		return "unknown"
	}
}

// Metrics holds connection pool metrics.
type Metrics struct {
	Hits       uint32
	Misses     uint32
	Timeouts   uint32
	TotalConns uint32
	IdleConns  uint32
	StaleConns uint32
}

// DefaultConfig returns a Config with sensible defaults.
func DefaultConfig() *Config {
	return &Config{
		Addresses:       []string{"localhost:6379"},
		DB:              0,
		PoolSize:        DefaultPoolSize,
		MinIdleConns:    DefaultMinIdleConns,
		ConnMaxLifetime: DefaultConnMaxLifetime,
		ConnMaxIdleTime: DefaultConnMaxIdleTime,
		DialTimeout:     DefaultDialTimeout,
		ReadTimeout:     DefaultReadTimeout,
		WriteTimeout:    DefaultWriteTimeout,
		PoolTimeout:     DefaultPoolTimeout,
		Mode:            ModeStandalone,
	}
}

// ConfigOption is a functional option for configuring the Redis client.
type ConfigOption func(*Config)

// WithAddress sets a single Redis address.
func WithAddress(addr string) ConfigOption {
	return func(c *Config) {
		c.Addresses = []string{addr}
	}
}

// WithAddresses sets multiple Redis addresses.
func WithAddresses(addrs ...string) ConfigOption {
	return func(c *Config) {
		c.Addresses = addrs
	}
}

// WithPassword sets the Redis password.
func WithPassword(password string) ConfigOption {
	return func(c *Config) {
		c.Password = password
	}
}

// WithDB sets the Redis database number.
func WithDB(db int) ConfigOption {
	return func(c *Config) {
		c.DB = db
	}
}

// WithPoolSize sets the connection pool size.
func WithPoolSize(size int) ConfigOption {
	return func(c *Config) {
		c.PoolSize = size
	}
}

// WithMinIdleConns sets the minimum number of idle connections.
func WithMinIdleConns(n int) ConfigOption {
	return func(c *Config) {
		c.MinIdleConns = n
	}
}

// WithConnMaxLifetime sets the maximum lifetime of a connection.
func WithConnMaxLifetime(d time.Duration) ConfigOption {
	return func(c *Config) {
		c.ConnMaxLifetime = d
	}
}

// WithConnMaxIdleTime sets the maximum idle time for a connection.
func WithConnMaxIdleTime(d time.Duration) ConfigOption {
	return func(c *Config) {
		c.ConnMaxIdleTime = d
	}
}

// WithDialTimeout sets the connection dial timeout.
func WithDialTimeout(d time.Duration) ConfigOption {
	return func(c *Config) {
		c.DialTimeout = d
	}
}

// WithReadTimeout sets the read operation timeout.
func WithReadTimeout(d time.Duration) ConfigOption {
	return func(c *Config) {
		c.ReadTimeout = d
	}
}

// WithWriteTimeout sets the write operation timeout.
func WithWriteTimeout(d time.Duration) ConfigOption {
	return func(c *Config) {
		c.WriteTimeout = d
	}
}

// WithPoolTimeout sets the pool connection timeout.
func WithPoolTimeout(d time.Duration) ConfigOption {
	return func(c *Config) {
		c.PoolTimeout = d
	}
}

// WithMode sets the Redis deployment mode.
func WithMode(mode Mode) ConfigOption {
	return func(c *Config) {
		c.Mode = mode
	}
}

// WithMasterName sets the master name for sentinel mode.
func WithMasterName(name string) ConfigOption {
	return func(c *Config) {
		c.MasterName = name
	}
}

// WithTLS enables TLS for connections.
func WithTLS(enabled bool) ConfigOption {
	return func(c *Config) {
		c.TLSEnabled = enabled
	}
}

// WithLogger sets the logger for the client.
func WithLogger(logger *slog.Logger) ClientOption {
	return func(c *Client) {
		c.logger = logger
	}
}

// ClientOption is a functional option for configuring the Client instance.
type ClientOption func(*Client)

// New creates a new Redis client with the given options.
func New(opts ...ConfigOption) (*Client, error) {
	cfg := DefaultConfig()
	for _, opt := range opts {
		opt(cfg)
	}

	return NewWithConfig(cfg)
}

// NewWithConfig creates a new Redis client with the given configuration.
func NewWithConfig(cfg *Config, opts ...ClientOption) (*Client, error) {
	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	client := &Client{
		config: cfg,
		logger: slog.Default(),
	}

	for _, opt := range opts {
		opt(client)
	}

	// Create the appropriate Redis client based on mode
	switch cfg.Mode {
	case ModeCluster:
		client.client = redis.NewClusterClient(&redis.ClusterOptions{
			Addrs:           cfg.Addresses,
			Password:        cfg.Password,
			PoolSize:        cfg.PoolSize,
			MinIdleConns:    cfg.MinIdleConns,
			ConnMaxLifetime: cfg.ConnMaxLifetime,
			ConnMaxIdleTime: cfg.ConnMaxIdleTime,
			DialTimeout:     cfg.DialTimeout,
			ReadTimeout:     cfg.ReadTimeout,
			WriteTimeout:    cfg.WriteTimeout,
			PoolTimeout:     cfg.PoolTimeout,
		})
	case ModeSentinel:
		client.client = redis.NewFailoverClient(&redis.FailoverOptions{
			MasterName:       cfg.MasterName,
			SentinelAddrs:    cfg.Addresses,
			Password:         cfg.Password,
			DB:               cfg.DB,
			PoolSize:         cfg.PoolSize,
			MinIdleConns:     cfg.MinIdleConns,
			ConnMaxLifetime:  cfg.ConnMaxLifetime,
			ConnMaxIdleTime:  cfg.ConnMaxIdleTime,
			DialTimeout:      cfg.DialTimeout,
			ReadTimeout:      cfg.ReadTimeout,
			WriteTimeout:     cfg.WriteTimeout,
			PoolTimeout:      cfg.PoolTimeout,
			SentinelPassword: cfg.Password,
		})
	default: // ModeStandalone
		addr := "localhost:6379"
		if len(cfg.Addresses) > 0 {
			addr = cfg.Addresses[0]
		}
		client.client = redis.NewClient(&redis.Options{
			Addr:            addr,
			Password:        cfg.Password,
			DB:              cfg.DB,
			PoolSize:        cfg.PoolSize,
			MinIdleConns:    cfg.MinIdleConns,
			ConnMaxLifetime: cfg.ConnMaxLifetime,
			ConnMaxIdleTime: cfg.ConnMaxIdleTime,
			DialTimeout:     cfg.DialTimeout,
			ReadTimeout:     cfg.ReadTimeout,
			WriteTimeout:    cfg.WriteTimeout,
			PoolTimeout:     cfg.PoolTimeout,
		})
	}

	return client, nil
}

// NewFromCoreConfig creates a new Redis client from txova-go-core config.
func NewFromCoreConfig(cfg config.RedisConfig, opts ...ClientOption) (*Client, error) {
	poolSize := cfg.PoolSize
	if poolSize <= 0 {
		poolSize = DefaultPoolSize
	}

	redisConfig := &Config{
		Addresses: []string{cfg.Address()},
		Password:  cfg.Password,
		DB:        cfg.DB,
		PoolSize:  poolSize,
		// Use defaults for other settings
		MinIdleConns:    DefaultMinIdleConns,
		ConnMaxLifetime: DefaultConnMaxLifetime,
		ConnMaxIdleTime: DefaultConnMaxIdleTime,
		DialTimeout:     DefaultDialTimeout,
		ReadTimeout:     DefaultReadTimeout,
		WriteTimeout:    DefaultWriteTimeout,
		PoolTimeout:     DefaultPoolTimeout,
		Mode:            ModeStandalone,
	}

	return NewWithConfig(redisConfig, opts...)
}

// Validate validates the configuration.
func (c *Config) Validate() error {
	if len(c.Addresses) == 0 {
		return NewError(CodeInternal, "at least one Redis address is required")
	}

	if c.Mode == ModeSentinel && c.MasterName == "" {
		return NewError(CodeInternal, "master name is required for sentinel mode")
	}

	if c.PoolSize <= 0 {
		return NewError(CodeInternal, "pool size must be positive")
	}

	return nil
}

// Ping checks if the Redis connection is healthy.
func (c *Client) Ping(ctx context.Context) error {
	if err := c.client.Ping(ctx).Err(); err != nil {
		return FromRedisError(err)
	}
	return nil
}

// Close closes the Redis client connection.
func (c *Client) Close() error {
	if err := c.client.Close(); err != nil {
		return FromRedisError(err)
	}
	return nil
}

// Client returns the underlying go-redis client.
// Use this for advanced operations not covered by the wrapper.
func (c *Client) Client() redis.UniversalClient {
	return c.client
}

// Stats returns the connection pool statistics.
func (c *Client) Stats() *Metrics {
	stats := c.client.PoolStats()
	return &Metrics{
		Hits:       stats.Hits,
		Misses:     stats.Misses,
		Timeouts:   stats.Timeouts,
		TotalConns: stats.TotalConns,
		IdleConns:  stats.IdleConns,
		StaleConns: stats.StaleConns,
	}
}

// Name returns the component name for app lifecycle integration.
func (c *Client) Name() string {
	return "redis"
}

// Init initializes the Redis connection (implements app.Initializer).
func (c *Client) Init(ctx context.Context) error {
	c.logger.Info("connecting to Redis",
		"addresses", c.config.Addresses,
		"mode", c.config.Mode.String(),
	)

	if err := c.Ping(ctx); err != nil {
		c.logger.Error("Redis connection failed",
			"error", err.Error(),
			"addresses", c.config.Addresses,
		)
		return err
	}

	c.logger.Info("Redis connected successfully",
		"addresses", c.config.Addresses,
		"mode", c.config.Mode.String(),
		"pool_size", c.config.PoolSize,
	)

	return nil
}

// Check performs a health check (implements app.HealthChecker).
func (c *Client) Check(ctx context.Context) error {
	return c.Ping(ctx)
}
