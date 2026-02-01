# txova-go-db

Database utilities for PostgreSQL and Redis, providing connection management, transactions, query building, migrations, caching, sessions, distributed locking, and rate limiting.

## Overview

`txova-go-db` provides comprehensive database utilities for Txova services, including PostgreSQL connection pooling with pgx, type-safe query builders, transaction management with automatic retry, Redis caching patterns, distributed locking, session management, and rate limiting.

**Module:** `github.com/Dorico-Dynamics/txova-go-db`

## Features

### PostgreSQL
- **Connection Pooling** - pgxpool with health checks, graceful shutdown, and slow query logging
- **Transaction Management** - Auto rollback/commit, serialization retry, nested transactions via savepoints
- **Query Builders** - Type-safe INSERT, SELECT, UPDATE, DELETE with parameterized queries
- **Column Allowlists** - Security validation to prevent SQL injection via dynamic columns
- **Migrations** - Embedded filesystem support with golang-migrate
- **Error Handling** - Structured errors with PostgreSQL-specific details

### Redis
- **Multiple Modes** - Standalone, Cluster, and Sentinel support
- **Caching** - Get/Set with TTL, JSON support, bulk operations, cache-aside pattern
- **Sessions** - User session management with device tracking
- **Distributed Locking** - Redis-based locks with ownership verification
- **Rate Limiting** - Fixed window and sliding window algorithms

## Packages

| Package | Description |
|---------|-------------|
| `postgres` | PostgreSQL connection, transactions, query builders, migrations, errors |
| `redis` | Redis client, caching, sessions, locking, rate limiting |

## Installation

```bash
go get github.com/Dorico-Dynamics/txova-go-db
```

## Quick Start

### PostgreSQL

```go
import "github.com/Dorico-Dynamics/txova-go-db/postgres"

// Create pool
pool, err := postgres.NewPool(
    postgres.WithConnString("postgres://user:pass@localhost/db"),
    postgres.WithMaxConns(25),
)
defer pool.Close()

// Query builder
query := postgres.Select("users").
    Columns("id", "name", "email").
    Where("status = ?", "active").
    OrderByDesc("created_at").
    Limit(20)

sql, args, err := query.Build()
rows, err := pool.Query(ctx, sql, args...)

// Transactions with auto-retry
txMgr := postgres.NewTxManager(pool)
err := txMgr.WithTx(ctx, func(tx postgres.Tx) error {
    _, err := tx.Exec(ctx, "INSERT INTO users ...", args...)
    return err // nil = commit, error = rollback
})
```

### Redis

```go
import "github.com/Dorico-Dynamics/txova-go-db/redis"

// Create client
client, err := redis.New(
    redis.WithAddress("localhost:6379"),
    redis.WithPoolSize(10),
)
defer client.Close()

// Caching with cache-aside pattern
cache := redis.NewCache(client, redis.WithDefaultTTL(15*time.Minute))
var user User
err := cache.GetOrSetJSON(ctx, "user:123", &user, func(ctx context.Context) (any, error) {
    return userRepo.FindByID(ctx, "123")
})

// Distributed locking
locker := redis.NewLocker(client)
err := locker.WithLock(ctx, "resource:123", func(ctx context.Context) error {
    return processExclusively(ctx)
})

// Rate limiting
limiter := redis.NewRateLimiter(client,
    redis.WithRateLimitMax(100),
    redis.WithRateLimitWindow(time.Minute),
)
result, err := limiter.Allow(ctx, "user:123")
if !result.Allowed {
    return fmt.Errorf("rate limited, reset at %v", result.ResetAt)
}
```

## Documentation

See [USAGE.md](USAGE.md) for complete API documentation and examples.

## Error Handling

Both packages provide structured error types that integrate with `txova-go-core`:

```go
// PostgreSQL
if postgres.IsNotFound(err) { /* ... */ }
if postgres.IsDuplicate(err) { /* ... */ }

// Redis  
if redis.IsConnection(err) { /* ... */ }
if redis.IsTimeout(err) { /* ... */ }

// Works with txova-go-core
if coreerrors.IsNotFound(err) { /* ... */ }
if coreerrors.IsConflict(err) { /* ... */ }
```

## Dependencies

**Internal:**
- `github.com/Dorico-Dynamics/txova-go-core`

**External:**
- `github.com/jackc/pgx/v5` - PostgreSQL driver
- `github.com/redis/go-redis/v9` - Redis client
- `github.com/golang-migrate/migrate/v4` - Database migrations

## Development

### Requirements

- Go 1.24+
- PostgreSQL 16+
- Redis 7+

### Testing

```bash
# Unit tests
go test ./...

# With race detection
go test -race ./...

# Integration tests (requires databases)
go test -tags=integration ./...
```

### Linting

```bash
golangci-lint run ./...
gosec ./...
go vet ./...
```

### Test Coverage Target

> 85%

## License

Proprietary - Dorico Dynamics
