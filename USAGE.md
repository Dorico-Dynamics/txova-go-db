# txova-go-db Usage Guide

Complete developer guide for PostgreSQL and Redis utilities.

## Table of Contents

- [PostgreSQL](#postgresql)
  - [Connection Pooling](#connection-pooling)
  - [Querying](#querying)
  - [Transaction Management](#transaction-management)
  - [Query Builders](#query-builders)
  - [Migrations](#migrations)
  - [Error Handling](#error-handling)
- [Redis](#redis)
  - [Client Setup](#client-setup)
  - [Caching](#caching)
  - [Sessions](#sessions)
  - [Distributed Locking](#distributed-locking)
  - [Rate Limiting](#rate-limiting)
  - [Error Handling](#redis-error-handling)
- [Design Patterns](#design-patterns)
- [Configuration Reference](#configuration-reference)

---

## PostgreSQL

### Connection Pooling

#### Creating a Pool

```go
import "github.com/Dorico-Dynamics/txova-go-db/postgres"

// With options
pool, err := postgres.NewPool(
    postgres.WithConnString("postgres://user:pass@localhost:5432/mydb"),
    postgres.WithMaxConns(25),
    postgres.WithMinConns(5),
    postgres.WithMaxConnLifetime(time.Hour),
    postgres.WithMaxConnIdleTime(30*time.Minute),
    postgres.WithConnectTimeout(5*time.Second),
    postgres.WithSlowQueryThreshold(time.Second),
)
if err != nil {
    log.Fatal(err)
}
defer pool.Close()

// From txova-go-core config
cfg := &config.DatabaseConfig{
    Host:           "localhost",
    Port:           5432,
    Database:       "mydb",
    User:           "user",
    Password:       "pass",
    MaxConnections: 25,
}
pool, err := postgres.NewPoolFromConfig(ctx, postgres.FromDatabaseConfig(cfg))
```

#### Health Checks

```go
// Ping the database
if err := pool.Ping(ctx); err != nil {
    log.Fatal("Database unavailable:", err)
}

// Get pool statistics
stats := pool.Stat()
fmt.Printf("Total: %d, Idle: %d, In-use: %d\n", 
    stats.TotalConns(), stats.IdleConns(), stats.AcquiredConns())
```

#### Default Configuration

| Setting | Default | Description |
|---------|---------|-------------|
| MaxConns | 25 | Maximum pool connections |
| MinConns | 5 | Minimum idle connections |
| MaxConnLifetime | 1 hour | Maximum connection age |
| MaxConnIdleTime | 30 min | Maximum idle time |
| HealthCheckPeriod | 1 min | Background health check interval |
| ConnectTimeout | 5 sec | Connection timeout |
| SlowQueryThreshold | 1 sec | Log warning for slower queries |

---

### Querying

Pool, Conn, and Tx all implement the `Querier` interface:

```go
// Execute (INSERT, UPDATE, DELETE without returning)
tag, err := pool.Exec(ctx, "UPDATE users SET name = $1 WHERE id = $2", name, id)
fmt.Printf("Rows affected: %d\n", tag.RowsAffected())

// Query multiple rows
rows, err := pool.Query(ctx, "SELECT id, name FROM users WHERE status = $1", "active")
if err != nil {
    return err
}
defer rows.Close()

for rows.Next() {
    var id int
    var name string
    if err := rows.Scan(&id, &name); err != nil {
        return err
    }
    fmt.Printf("User: %d - %s\n", id, name)
}
if err := rows.Err(); err != nil {
    return err
}

// Query single row
var name string
err := pool.QueryRow(ctx, "SELECT name FROM users WHERE id = $1", id).Scan(&name)
if err != nil {
    if postgres.IsNotFound(err) {
        return fmt.Errorf("user not found")
    }
    return err
}
```

---

### Transaction Management

#### TxManager

```go
txManager := postgres.NewTxManager(pool,
    postgres.WithMaxRetries(3),
    postgres.WithRetryBaseDelay(50*time.Millisecond),
    postgres.WithRetryMaxDelay(2*time.Second),
)

// Simple transaction - auto commit/rollback
err := txManager.WithTx(ctx, func(tx postgres.Tx) error {
    _, err := tx.Exec(ctx, "INSERT INTO users (name) VALUES ($1)", "John")
    if err != nil {
        return err // Rollback
    }
    
    _, err = tx.Exec(ctx, "INSERT INTO profiles (user_id) VALUES ($1)", userID)
    return err // nil = Commit, error = Rollback
})

// With custom isolation level
err := txManager.WithTxOptions(ctx, pgx.TxOptions{
    IsoLevel: pgx.Serializable,
}, func(tx postgres.Tx) error {
    // Serializable transaction
    return nil
})
```

#### Automatic Retry

TxManager automatically retries on serialization failures and deadlocks:

```go
// Retries up to 3 times with exponential backoff + jitter
err := txManager.WithTx(ctx, func(tx postgres.Tx) error {
    // If this fails due to serialization or deadlock,
    // it will be retried automatically
    return updateBalance(ctx, tx, accountID, amount)
})
```

#### Context-Based Transaction

```go
// Store transaction in context
txCtx := postgres.ContextWithTx(ctx, tx)

// Retrieve from context (useful for repository patterns)
if tx, ok := postgres.TxFromContext(ctx); ok {
    // Use existing transaction
    return tx.Exec(ctx, sql, args...)
}
// No transaction, use pool directly
return pool.Exec(ctx, sql, args...)
```

#### Nested Transactions (Savepoints)

```go
err := txManager.WithTx(ctx, func(tx postgres.Tx) error {
    // Primary operation
    _, err := tx.Exec(ctx, "INSERT INTO orders ...", args...)
    if err != nil {
        return err
    }
    
    // Nested operation via savepoint
    nestedTx, err := tx.Begin(ctx)
    if err != nil {
        return err
    }
    
    err = nestedTx.Exec(ctx, "UPDATE inventory ...", args...)
    if err != nil {
        nestedTx.Rollback(ctx) // Only rolls back to savepoint
        // Outer transaction continues
    } else {
        nestedTx.Commit(ctx)
    }
    
    return nil
})
```

---

### Query Builders

All builders use parameterized queries and support method chaining.

#### SELECT

```go
query := postgres.Select("users").
    Columns("id", "name", "email").
    Distinct().
    Where("status = ?", "active").
    OrWhere("role = ?", "admin").
    WhereIn("department", "sales", "marketing").
    WhereNotIn("status", "banned", "suspended").
    WhereLike("name", "John%").
    WhereILike("email", "%@example.com").  // Case-insensitive
    WhereNull("deleted_at").
    WhereNotNull("email").
    WhereBetween("created_at", startDate, endDate).
    Join("profiles", "users.id = profiles.user_id").
    LeftJoin("posts", "users.id = posts.author_id").
    RightJoin("comments", "posts.id = comments.post_id").
    GroupBy("status", "role").
    Having("COUNT(*) > ?", 10).
    OrderByAsc("created_at").
    OrderByDesc("name").
    Limit(20).
    Offset(40).
    ForUpdate()  // Row locking

sql, args, err := query.Build()
if err != nil {
    return err
}
rows, err := pool.Query(ctx, sql, args...)
```

#### SELECT with Pagination

```go
import "github.com/Dorico-Dynamics/txova-go-core/pagination"

pageReq := pagination.PageRequest{
    Page:     1,
    PageSize: 20,
    SortBy:   "created_at",
    SortDir:  pagination.SortDesc,
}

query := postgres.Select("users").
    Columns("id", "name").
    Where("status = ?", "active").
    Page(pageReq)  // Applies limit, offset, and sorting

sql, args, err := query.Build()
```

#### INSERT

```go
query := postgres.Insert("users").
    Columns("id", "name", "email").
    Values(uuid.New(), "John", "john@example.com").
    Values(uuid.New(), "Jane", "jane@example.com").
    Returning("id", "created_at")

sql, args, err := query.Build()
rows, err := pool.Query(ctx, sql, args...)
```

#### INSERT with Conflict Handling

```go
// ON CONFLICT DO NOTHING (by columns)
query := postgres.Insert("users").
    Columns("email", "name").
    Values("john@example.com", "John").
    OnConflictDoNothing("email")

// ON CONFLICT ON CONSTRAINT DO NOTHING
query := postgres.Insert("users").
    Columns("email", "name").
    Values("john@example.com", "John").
    OnConflictConstraintDoNothing("users_email_key")
```

#### UPDATE

```go
query := postgres.Update("users").
    Set("name", "John Doe").
    Set("updated_at", time.Now()).
    SetMap(map[string]any{
        "status": "active",
        "role":   "admin",
    }).
    Where("id = ?", userID).
    Returning("id", "updated_at")

sql, args, err := query.Build()
```

#### DELETE

```go
query := postgres.Delete("users").
    Where("status = ?", "inactive").
    Where("deleted_at < ?", time.Now().AddDate(0, -6, 0)).
    Returning("id")

sql, args, err := query.Build()

// DELETE without WHERE requires explicit opt-in (safety feature)
query := postgres.Delete("temp_data").
    AllowUnrestrictedDelete()
```

#### Column Allowlist (Security)

```go
// Validates columns against allowlist
query := postgres.SelectWithAllowlist("users", "id", "name", "email").
    Columns("id", "name")  // OK

query := postgres.SelectWithAllowlist("users", "id", "name", "email").
    Columns("password")  // Error: column not in allowlist
```

#### Debug Helpers

```go
// Get SQL only (empty string on error)
sql := query.SQL()

// Get args only (nil on error)
args := query.Args()

// Panic on error (useful in tests)
sql, args := query.MustBuild()
```

---

### Migrations

```go
import "embed"

//go:embed migrations/*.sql
var migrationsFS embed.FS

migrator, err := postgres.NewMigrator(pool, migrationsFS,
    postgres.WithMigrationsTable("schema_migrations"),
    postgres.WithMigrationsPath("migrations"),
    postgres.WithLockTimeout(15*time.Second),
    postgres.WithMigratorLogger(logger),
)
if err != nil {
    log.Fatal(err)
}
defer migrator.Close()

// Apply all pending migrations
if err := migrator.Up(ctx); err != nil {
    log.Fatal(err)
}

// Rollback all migrations
if err := migrator.Down(ctx); err != nil {
    log.Fatal(err)
}

// Apply/rollback specific steps
migrator.Steps(ctx, 2)   // Apply 2 migrations
migrator.Steps(ctx, -1)  // Rollback 1 migration

// Check current version
version, dirty, err := migrator.Version()

// Force version (dangerous - use only for recovery)
migrator.Force(5)
```

#### Migration File Naming

```
migrations/
  0001_create_users_table.up.sql
  0001_create_users_table.down.sql
  0002_add_email_index.up.sql
  0002_add_email_index.down.sql
```

---

### Error Handling

#### Error Codes

| Code | Constant | Description |
|------|----------|-------------|
| 404 | `CodeNotFound` | Record not found |
| 409 | `CodeDuplicate` | Unique constraint violation |
| 409 | `CodeForeignKey` | Foreign key violation |
| 400 | `CodeCheckViolation` | Check constraint violation |
| 503 | `CodeConnection` | Connection error |
| 503 | `CodeTimeout` | Query timeout |
| 409 | `CodeSerialization` | Transaction serialization failure |
| 409 | `CodeDeadlock` | Deadlock detected |
| 400 | `CodeInvalidInput` | Invalid input |
| 500 | `CodeInternal` | Internal error |

#### Checking Errors

```go
err := pool.QueryRow(ctx, sql, args...).Scan(&result)
if err != nil {
    // Convert to database error
    dbErr := postgres.AsError(err)
    if dbErr != nil {
        switch dbErr.Code() {
        case postgres.CodeNotFound:
            return nil, ErrUserNotFound
        case postgres.CodeDuplicate:
            return nil, ErrEmailExists
        }
        
        // Access PostgreSQL details
        fmt.Println("Detail:", dbErr.Detail())
        fmt.Println("Hint:", dbErr.Hint())
        fmt.Println("Table:", dbErr.TableName())
        fmt.Println("Column:", dbErr.Column())
        fmt.Println("Constraint:", dbErr.Constraint())
        fmt.Println("SQLState:", dbErr.SQLState())
    }
    return nil, err
}

// Helper functions
if postgres.IsNotFound(err) { /* ... */ }
if postgres.IsDuplicate(err) { /* ... */ }
if postgres.IsForeignKey(err) { /* ... */ }
if postgres.IsConnection(err) { /* ... */ }
if postgres.IsTimeout(err) { /* ... */ }
if postgres.IsSerialization(err) { /* ... */ }
if postgres.IsDeadlock(err) { /* ... */ }
```

#### Integration with txova-go-core

```go
import coreerrors "github.com/Dorico-Dynamics/txova-go-core/errors"

// Database errors work with core error checking
if coreerrors.IsNotFound(err) { /* ... */ }
if coreerrors.IsConflict(err) { /* ... */ }
if coreerrors.IsServiceUnavailable(err) { /* ... */ }
```

---

## Redis

### Client Setup

#### Standalone Mode

```go
import "github.com/Dorico-Dynamics/txova-go-db/redis"

client, err := redis.New(
    redis.WithAddress("localhost:6379"),
    redis.WithPassword("secret"),
    redis.WithDB(0),
    redis.WithPoolSize(10),
    redis.WithMinIdleConns(2),
    redis.WithDialTimeout(5*time.Second),
    redis.WithReadTimeout(3*time.Second),
    redis.WithWriteTimeout(3*time.Second),
)
if err != nil {
    log.Fatal(err)
}
defer client.Close()

// Health check
if err := client.Ping(ctx); err != nil {
    log.Fatal("Redis unavailable:", err)
}
```

#### Cluster Mode

```go
client, err := redis.New(
    redis.WithAddresses("node1:6379", "node2:6379", "node3:6379"),
    redis.WithMode(redis.ModeCluster),
    redis.WithPassword("secret"),
)
```

#### Sentinel Mode (High Availability)

```go
client, err := redis.New(
    redis.WithAddresses("sentinel1:26379", "sentinel2:26379"),
    redis.WithMode(redis.ModeSentinel),
    redis.WithMasterName("mymaster"),
    redis.WithPassword("secret"),
)
```

#### From txova-go-core Config

```go
coreConfig := config.RedisConfig{
    Address:  "localhost:6379",
    Password: "secret",
    DB:       0,
}
client, err := redis.NewFromCoreConfig(coreConfig)
```

---

### Caching

#### Basic Operations

```go
cache := redis.NewCache(client,
    redis.WithDefaultTTL(15*time.Minute),
    redis.WithKeyPrefix("myapp"),
    redis.WithCacheLogger(logger),
)

// Set
err := cache.Set(ctx, "key", []byte("value"))
err := cache.SetWithTTL(ctx, "key", []byte("value"), time.Hour)

// Get (returns nil data on miss, not an error)
data, err := cache.Get(ctx, "key")
if data == nil {
    // Cache miss
}

// Delete
err := cache.Delete(ctx, "key1", "key2")

// Delete by pattern (SCAN-based, safe for large datasets)
err := cache.DeleteByPattern(ctx, "myapp:users:*")

// Check existence
exists, err := cache.Exists(ctx, "key")

// TTL operations
ttl, err := cache.TTL(ctx, "key")  // -2 = not exists, -1 = no expiry
ok, err := cache.Expire(ctx, "key", time.Hour)
```

#### JSON Operations

```go
// Set JSON
user := User{ID: "123", Name: "John"}
err := cache.SetJSON(ctx, "user:123", user)
err := cache.SetJSONWithTTL(ctx, "user:123", user, time.Hour)

// Get JSON
var user User
found, err := cache.GetJSON(ctx, "user:123", &user)
if !found {
    // Cache miss
}
```

#### Bulk Operations

```go
// Get multiple keys
values, err := cache.MGet(ctx, "key1", "key2", "key3")
// values["key1"] is nil if not found

// Set multiple keys
err := cache.MSet(ctx, map[string][]byte{
    "key1": []byte("value1"),
    "key2": []byte("value2"),
})
err := cache.MSetWithTTL(ctx, valuesMap, time.Hour)
```

#### Computed Values (Cache-Aside Pattern)

```go
// Get or compute (bytes)
data, err := cache.GetOrSet(ctx, "expensive:result", func(ctx context.Context) ([]byte, error) {
    // Only called on cache miss
    result, err := expensiveComputation(ctx)
    if err != nil {
        return nil, err
    }
    return json.Marshal(result)
})

// Get or compute (JSON)
var user User
err := cache.GetOrSetJSON(ctx, "user:123", &user, func(ctx context.Context) (any, error) {
    return userRepo.FindByID(ctx, "123")
})

// With custom TTL
data, err := cache.GetOrSetWithTTL(ctx, "key", time.Hour, computeFunc)
err := cache.GetOrSetJSONWithTTL(ctx, "key", time.Hour, &dest, computeFunc)
```

#### Key Builder Utility

```go
kb := redis.NewKeyBuilder("myservice")

key := kb.Key("users", "uuid123")           // "myservice:users:uuid123"
key := kb.KeyWithParts("cache", "v2", "data")  // "myservice:cache:v2:data"
pattern := kb.Pattern("users")              // "myservice:users:*"
```

---

### Sessions

```go
store := redis.NewSessionStore(client,
    redis.WithSessionKeyPrefix("session"),
    redis.WithSessionDefaultTTL(30*24*time.Hour),  // 30 days
    redis.WithSessionLogger(logger),
)

// Create session
session, err := store.Create(ctx, userID,
    redis.WithDeviceID("device-uuid"),
    redis.WithDeviceInfo("iPhone 15, iOS 18"),
    redis.WithIPAddress("192.168.1.1"),
    redis.WithSessionData(customData),
)
// session.ID, session.UserID, session.CreatedAt, session.ExpiresAt, etc.

// Create with custom TTL
session, err := store.CreateWithTTL(ctx, userID, 24*time.Hour, opts...)

// Get session (auto-updates LastActive)
session, err := store.Get(ctx, sessionID)

// Get without updating LastActive
session, err := store.GetWithTouch(ctx, sessionID, false)

// Update session data
session.Data = newData
err := store.Update(ctx, session)

// List user's sessions
sessions, err := store.ListByUserID(ctx, userID)

// Count active sessions
count, err := store.Count(ctx, userID)

// Check existence
exists, err := store.Exists(ctx, sessionID)

// Extend TTL
err := store.Extend(ctx, sessionID, 24*time.Hour)

// Delete single session
err := store.Delete(ctx, sessionID)

// Delete all user sessions (logout everywhere)
deleted, err := store.DeleteByUserID(ctx, userID)
```

---

### Distributed Locking

```go
locker := redis.NewLocker(client,
    redis.WithLockKeyPrefix("lock"),
    redis.WithDefaultLockTTL(30*time.Second),
    redis.WithLockRetryDelay(50*time.Millisecond),
    redis.WithLockRetryCount(100),
    redis.WithLockerLogger(logger),
)

// Acquire lock (non-blocking, returns error if held)
lock, err := locker.Acquire(ctx, "resource:123")
if err != nil {
    return err  // Lock already held
}
defer lock.Release(ctx)

// Try acquire (non-blocking, returns nil/nil if held)
lock, err := locker.TryAcquire(ctx, "resource:123")
if err != nil {
    return err  // Unexpected error
}
if lock == nil {
    return nil  // Lock held by someone else, skip
}
defer lock.Release(ctx)

// Acquire with retry (blocking, retries with backoff)
lock, err := locker.AcquireWithRetry(ctx, "resource:123")

// With custom TTL
lock, err := locker.AcquireWithTTL(ctx, "resource:123", time.Minute)
```

#### Convenience Methods

```go
// Execute function with lock
err := locker.WithLock(ctx, "resource:123", func(ctx context.Context) error {
    // Exclusive access guaranteed
    return processResource(ctx)
})

// With custom TTL
err := locker.WithLockAndTTL(ctx, "resource:123", time.Minute, func(ctx context.Context) error {
    return processResource(ctx)
})
```

#### Lock Operations

```go
// Extend TTL
err := lock.Extend(ctx, time.Minute)

// Check TTL
ttl, err := lock.TTL(ctx)

// Verify ownership on server
held, err := lock.Verify(ctx)

// Lock properties
key := lock.Key()
owner := lock.Owner()
isHeld := lock.IsHeld()
expiresAt := lock.ExpiresAt()
isExpired := lock.IsExpired()  // Local approximation

// Release (atomic ownership check via Lua script)
err := lock.Release(ctx)
```

---

### Rate Limiting

```go
limiter := redis.NewRateLimiter(client,
    redis.WithRateLimitKeyPrefix("ratelimit"),
    redis.WithRateLimitWindow(time.Minute),
    redis.WithRateLimitMax(100),
    redis.WithRateLimitBurst(10),  // Optional burst allowance
    redis.WithRateLimiterLogger(logger),
)

// Convenience constructors
userLimiter := redis.UserRateLimiter(client, 100, time.Minute)
ipLimiter := redis.IPRateLimiter(client, 1000, time.Hour)
```

#### Fixed Window

```go
result, err := limiter.Allow(ctx, "user:123")
result, err := limiter.AllowN(ctx, "user:123", 5)  // Check 5 requests
```

#### Sliding Window (Smoother)

```go
result, err := limiter.SlidingWindowAllow(ctx, "user:123")
result, err := limiter.SlidingWindowAllowN(ctx, "user:123", 5)
```

#### Result

```go
type RateLimitResult struct {
    Allowed   bool          // Request allowed?
    Remaining int64         // Requests left in window
    ResetAt   time.Time     // When window resets
    Total     int64         // Max requests per window
}

if !result.Allowed {
    return fmt.Errorf("rate limited, retry after %v", result.ResetAt)
}
fmt.Printf("Remaining: %d/%d\n", result.Remaining, result.Total)
```

#### Status Check (Without Increment)

```go
status, err := limiter.GetStatus(ctx, "user:123")
// Doesn't increment counter
```

#### Reset

```go
err := limiter.Reset(ctx, "user:123")
```

---

### Redis Error Handling

#### Error Codes

| Code | Constant | Description |
|------|----------|-------------|
| 404 | `CodeNotFound` | Key not found |
| 503 | `CodeConnection` | Connection error |
| 503 | `CodeTimeout` | Operation timeout |
| 409 | `CodeLockFailed` | Lock acquisition failed |
| 409 | `CodeLockNotHeld` | Lock not held by owner |
| 429 | `CodeRateLimited` | Rate limit exceeded |
| 400 | `CodeSerialization` | Serialization error |
| 500 | `CodeInternal` | Internal error |

#### Checking Errors

```go
err := cache.Get(ctx, "key")
if err != nil {
    redisErr := redis.AsError(err)
    if redisErr != nil {
        switch redisErr.Code() {
        case redis.CodeConnection:
            // Handle connection error
        case redis.CodeTimeout:
            // Handle timeout
        }
    }
}

// Helper functions
if redis.IsNotFound(err) { /* ... */ }
if redis.IsConnection(err) { /* ... */ }
if redis.IsTimeout(err) { /* ... */ }
if redis.IsLockFailed(err) { /* ... */ }
if redis.IsSerialization(err) { /* ... */ }
```

---

## Design Patterns

### Cache-Aside Pattern

```go
func (r *UserRepository) GetByID(ctx context.Context, id string) (*User, error) {
    var user User
    err := r.cache.GetOrSetJSON(ctx, r.kb.Key("users", id), &user, 
        func(ctx context.Context) (any, error) {
            return r.db.QueryUser(ctx, id)
        },
    )
    return &user, err
}
```

### Lock-Protected Operation

```go
func (s *RideService) AssignDriver(ctx context.Context, rideID, driverID string) error {
    return s.locker.WithLock(ctx, fmt.Sprintf("ride:assign:%s", rideID), 
        func(ctx context.Context) error {
            // Only one caller can assign at a time
            return s.repo.AssignDriver(ctx, rideID, driverID)
        },
    )
}
```

### Rate-Limited Endpoint

```go
func (h *Handler) HandleRequest(ctx context.Context, userID string) error {
    result, err := h.limiter.Allow(ctx, userID)
    if err != nil {
        return err
    }
    if !result.Allowed {
        return &RateLimitError{ResetAt: result.ResetAt}
    }
    return h.processRequest(ctx)
}
```

### Repository with Transaction Support

```go
func (r *OrderRepository) Create(ctx context.Context, order *Order) error {
    // Check for existing transaction
    var querier postgres.Querier = r.pool
    if tx, ok := postgres.TxFromContext(ctx); ok {
        querier = tx
    }
    
    sql, args, err := postgres.Insert("orders").
        Columns("id", "user_id", "total").
        Values(order.ID, order.UserID, order.Total).
        Returning("created_at").
        Build()
    if err != nil {
        return err
    }
    
    return querier.QueryRow(ctx, sql, args...).Scan(&order.CreatedAt)
}
```

---

## Configuration Reference

### PostgreSQL Pool

| Option | Default | Description |
|--------|---------|-------------|
| `WithMaxConns` | 25 | Maximum connections |
| `WithMinConns` | 5 | Minimum idle connections |
| `WithMaxConnLifetime` | 1 hour | Maximum connection age |
| `WithMaxConnIdleTime` | 30 min | Maximum idle time |
| `WithHealthCheckPeriod` | 1 min | Background health check |
| `WithConnectTimeout` | 5 sec | Connection timeout |
| `WithSlowQueryThreshold` | 1 sec | Slow query log threshold |

### Transaction Manager

| Option | Default | Description |
|--------|---------|-------------|
| `WithMaxRetries` | 3 | Max retry attempts |
| `WithRetryBaseDelay` | 50ms | Initial retry delay |
| `WithRetryMaxDelay` | 2 sec | Maximum retry delay |

### Migrator

| Option | Default | Description |
|--------|---------|-------------|
| `WithMigrationsTable` | schema_migrations | Migration tracking table |
| `WithMigrationsPath` | . | Path in embedded FS |
| `WithLockTimeout` | 15 sec | Migration lock timeout |

### Redis Client

| Option | Default | Description |
|--------|---------|-------------|
| `WithPoolSize` | 10 | Connection pool size |
| `WithMinIdleConns` | 2 | Minimum idle connections |
| `WithConnMaxLifetime` | 30 min | Maximum connection age |
| `WithConnMaxIdleTime` | 10 min | Maximum idle time |
| `WithDialTimeout` | 5 sec | Connection timeout |
| `WithReadTimeout` | 3 sec | Read timeout |
| `WithWriteTimeout` | 3 sec | Write timeout |
| `WithPoolTimeout` | 4 sec | Pool checkout timeout |

### Cache

| Option | Default | Description |
|--------|---------|-------------|
| `WithDefaultTTL` | 15 min | Default cache TTL |
| `WithKeyPrefix` | "" | Key prefix |

### Session Store

| Option | Default | Description |
|--------|---------|-------------|
| `WithSessionDefaultTTL` | 30 days | Default session TTL |
| `WithSessionKeyPrefix` | session | Key prefix |

### Locker

| Option | Default | Description |
|--------|---------|-------------|
| `WithDefaultLockTTL` | 30 sec | Default lock TTL |
| `WithLockRetryDelay` | 50ms | Retry delay |
| `WithLockRetryCount` | 100 | Max retry attempts |
| `WithLockKeyPrefix` | lock | Key prefix |

### Rate Limiter

| Option | Default | Description |
|--------|---------|-------------|
| `WithRateLimitWindow` | 1 min | Time window |
| `WithRateLimitMax` | 100 | Max requests per window |
| `WithRateLimitBurst` | 0 | Burst allowance |
| `WithRateLimitKeyPrefix` | ratelimit | Key prefix |
