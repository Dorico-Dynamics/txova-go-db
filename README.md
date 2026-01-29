# txova-go-db

Database utilities for PostgreSQL and Redis, providing connection management, transactions, query building, migrations, caching, and distributed locking.

## Overview

`txova-go-db` provides comprehensive database utilities for Txova services, including PostgreSQL connection pooling with pgx, Redis caching patterns, distributed locking, and database migration management.

**Module:** `github.com/txova/txova-go-db`

## Features

- **PostgreSQL Connection Pooling** - pgxpool with health checks and graceful shutdown
- **Transaction Management** - Automatic rollback on error/panic
- **Query Builder** - Safe parameterized query construction
- **Migrations** - golang-migrate integration
- **Redis Caching** - Get/Set with TTL, get-or-compute pattern
- **Distributed Locking** - Redis-based locks with ownership
- **Rate Limiting** - Redis-backed rate limit tracking
- **Session Store** - Redis session management

## Packages

| Package | Description |
|---------|-------------|
| `postgres` | PostgreSQL connection and query utilities |
| `redis` | Redis client and caching utilities |

## Installation

```bash
go get github.com/txova/txova-go-db
```

## Usage

### PostgreSQL Connection

```go
import "github.com/txova/txova-go-db/postgres"

pool, err := postgres.NewPool(postgres.Config{
    URL:             "postgres://user:pass@localhost/txova",
    MaxConnections:  25,
    MinConnections:  5,
    ConnectTimeout:  5 * time.Second,
})
defer pool.Close()

// Health check
if err := pool.Ping(ctx); err != nil {
    log.Fatal("Database unavailable")
}
```

### Transactions

```go
import "github.com/txova/txova-go-db/postgres"

err := postgres.WithTx(ctx, pool, func(tx pgx.Tx) error {
    // All operations in transaction
    _, err := tx.Exec(ctx, "INSERT INTO users ...")
    if err != nil {
        return err // Auto rollback
    }
    
    _, err = tx.Exec(ctx, "INSERT INTO profiles ...")
    return err // Commit if nil, rollback otherwise
})
```

### Query Builder

```go
import "github.com/txova/txova-go-db/postgres"

query := postgres.Select("users").
    Columns("id", "name", "email").
    Where("status = ?", "active").
    Where("created_at > ?", cutoff).
    OrderBy("created_at DESC").
    Limit(20).
    Offset(40)

sql, args := query.Build()
// sql: SELECT id, name, email FROM users WHERE status = $1 AND created_at > $2 ORDER BY created_at DESC LIMIT 20 OFFSET 40
```

### Migrations

```go
import "github.com/txova/txova-go-db/postgres"

migrator := postgres.NewMigrator(postgres.MigratorConfig{
    DatabaseURL:    dbURL,
    MigrationsPath: "file://migrations",
})

// Apply all pending migrations
if err := migrator.Up(); err != nil {
    log.Fatal(err)
}

// Rollback last migration
if err := migrator.Down(1); err != nil {
    log.Fatal(err)
}
```

### Redis Caching

```go
import "github.com/txova/txova-go-db/redis"

cache := redis.NewCache(redisClient)

// Set with TTL
cache.Set(ctx, "user:profile:123", user, 15*time.Minute)

// Get
var user User
found, err := cache.Get(ctx, "user:profile:123", &user)

// Get or compute
user, err := cache.GetOrSet(ctx, "user:profile:123", 15*time.Minute, func() (*User, error) {
    return userRepo.FindByID(ctx, userID)
})
```

### Distributed Locking

```go
import "github.com/txova/txova-go-db/redis"

locker := redis.NewLocker(redisClient)

// Acquire lock with timeout
err := locker.WithLock(ctx, "ride:assign:123", 30*time.Second, func() error {
    // Exclusive access to ride assignment
    return assignDriver(ctx, rideID)
})
```

### Rate Limiting

```go
import "github.com/txova/txova-go-db/redis"

limiter := redis.NewRateLimiter(redisClient)

result, err := limiter.Allow(ctx, redis.RateLimitConfig{
    Key:    "ratelimit:api:user123",
    Limit:  100,
    Window: time.Minute,
})

if !result.Allowed {
    // Rate limited
    fmt.Printf("Retry after: %v\n", result.ResetAt)
}
```

### Session Store

```go
import "github.com/txova/txova-go-db/redis"

sessions := redis.NewSessionStore(redisClient)

// Create session
session := sessions.Create(ctx, redis.SessionData{
    UserID:     userID,
    DeviceID:   deviceID,
    DeviceInfo: "iPhone 15, iOS 18",
    IPAddress:  clientIP,
})

// Get session
session, err := sessions.Get(ctx, sessionID)

// List user sessions
sessions, err := sessions.ListByUser(ctx, userID)

// Delete session
sessions.Delete(ctx, sessionID)
```

## Cache Key Conventions

| Pattern | Example | Description |
|---------|---------|-------------|
| `{service}:{entity}:{id}` | `user:profile:uuid` | Single entity |
| `{service}:{entity}:list:{params}` | `ride:history:user:uuid:page:1` | List query |
| `{service}:config:{key}` | `pricing:config:maputo` | Configuration |

## Error Types

| Error | Description |
|-------|-------------|
| `ErrNotFound` | Record not found |
| `ErrDuplicate` | Unique constraint violation |
| `ErrForeignKey` | Foreign key violation |
| `ErrConnection` | Database connection error |
| `ErrTimeout` | Query timeout |

## Dependencies

**Internal:**
- `txova-go-types`
- `txova-go-core`

**External:**
- `github.com/jackc/pgx/v5` - PostgreSQL driver
- `github.com/redis/go-redis/v9` - Redis client
- `github.com/golang-migrate/migrate/v4` - Migrations

## Development

### Requirements

- Go 1.25+
- PostgreSQL 18+
- Redis 8+

### Testing

```bash
go test ./...
```

### Test Coverage Target

> 85%

## License

Proprietary - Dorico Dynamics
