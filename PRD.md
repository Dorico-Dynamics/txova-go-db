# txova-go-db

## Overview
Database utilities for PostgreSQL and Redis, providing connection management, transaction handling, query building, migrations, and caching patterns.

**Module:** `github.com/txova/txova-go-db`

---

## Packages

### `postgres` - PostgreSQL Utilities

#### Connection Management
| Feature | Priority | Description |
|---------|----------|-------------|
| Connection pooling | P0 | pgxpool with configurable pool size |
| Health check | P0 | Ping with timeout for readiness probes |
| Graceful close | P0 | Drain connections on shutdown |
| Connection retry | P0 | Retry with backoff on startup |
| SSL support | P0 | Configurable SSL mode |

**Pool Configuration:**
| Setting | Default | Description |
|---------|---------|-------------|
| max_connections | 25 | Maximum pool size |
| min_connections | 5 | Minimum idle connections |
| max_conn_lifetime | 1h | Connection max age |
| max_conn_idle_time | 30m | Idle connection timeout |
| health_check_period | 1m | Background health check |
| connect_timeout | 5s | Connection timeout |

---

#### Transaction Management
| Feature | Priority | Description |
|---------|----------|-------------|
| Transaction wrapper | P0 | Execute function within transaction |
| Auto rollback | P0 | Rollback on error or panic |
| Nested transactions | P1 | Savepoint support |
| Read-only transactions | P1 | Optimization for read operations |
| Transaction timeout | P0 | Context-based timeout |

**Requirements:**
- `WithTx(ctx, func(tx) error)` pattern for transactions
- Automatic rollback if function returns error
- Automatic rollback if panic occurs
- Support isolation level configuration
- Propagate existing transaction from context

---

#### Query Builder
| Feature | Priority | Description |
|---------|----------|-------------|
| SELECT builder | P0 | Build SELECT queries with conditions |
| INSERT builder | P0 | Build INSERT with returning |
| UPDATE builder | P0 | Build UPDATE with conditions |
| DELETE builder | P0 | Build DELETE with conditions |
| WHERE conditions | P0 | AND, OR, IN, LIKE, NULL checks |
| Pagination | P0 | LIMIT, OFFSET, cursor-based |
| Ordering | P0 | ORDER BY with direction |
| Joins | P1 | INNER, LEFT, RIGHT joins |

**Requirements:**
- Use parameterized queries (prevent SQL injection)
- Support typed ID parameters from txova-go-types
- Return generated SQL and args for debugging
- Validate column names against allowlist

---

#### Migration Runner
| Feature | Priority | Description |
|---------|----------|-------------|
| Up migrations | P0 | Apply pending migrations |
| Down migrations | P0 | Rollback migrations |
| Version tracking | P0 | Track applied migrations in DB |
| Migration files | P0 | Load from filesystem |
| Embedded migrations | P1 | Support embed.FS |
| Dry run | P1 | Preview without applying |

**Requirements:**
- Use golang-migrate/migrate library
- Migration files: `NNNN_description.up.sql`, `NNNN_description.down.sql`
- Lock table during migrations (prevent concurrent runs)
- Log each migration applied
- Fail fast on migration errors

---

### `redis` - Redis Utilities

#### Connection Management
| Feature | Priority | Description |
|---------|----------|-------------|
| Connection pooling | P0 | go-redis with pool |
| Health check | P0 | PING for readiness |
| Cluster support | P1 | Redis cluster mode |
| Sentinel support | P1 | Redis sentinel mode |

---

#### Caching
| Feature | Priority | Description |
|---------|----------|-------------|
| Get/Set | P0 | Basic cache operations |
| TTL support | P0 | Expiration on set |
| Delete | P0 | Remove cached items |
| Get or Set | P0 | Atomic get-or-compute pattern |
| Batch operations | P1 | MGET, MSET for bulk ops |
| Cache invalidation | P0 | Delete by pattern |

**Cache Key Conventions:**
| Pattern | Example | Description |
|---------|---------|-------------|
| `{service}:{entity}:{id}` | `user:profile:uuid` | Single entity |
| `{service}:{entity}:list:{params}` | `ride:history:user:uuid:page:1` | List query |
| `{service}:config:{key}` | `pricing:config:maputo` | Configuration |

**Requirements:**
- JSON serialization for complex types
- Configurable default TTL (15 minutes)
- Support nil/not-found distinction
- Log cache hits/misses for metrics

---

#### Distributed Locking
| Feature | Priority | Description |
|---------|----------|-------------|
| Acquire lock | P0 | SET NX with TTL |
| Release lock | P0 | Delete with ownership check |
| Lock extension | P1 | Extend TTL while holding |
| Wait for lock | P1 | Block until available |

**Requirements:**
- Lock key format: `lock:{resource}:{id}`
- Include owner identifier to prevent wrong release
- Default TTL: 30 seconds
- Provide `WithLock(ctx, key, func() error)` wrapper

---

#### Rate Limiting
| Feature | Priority | Description |
|---------|----------|-------------|
| Fixed window | P0 | Simple count per window |
| Sliding window | P1 | More accurate limiting |
| Per-user limits | P0 | Keyed by user ID |
| Per-IP limits | P0 | Keyed by IP address |

**Rate Limit Keys:**
| Pattern | Description |
|---------|-------------|
| `ratelimit:api:{user_id}` | Per-user API limit |
| `ratelimit:otp:{phone}` | OTP request limit |
| `ratelimit:login:{ip}` | Login attempt limit |

**Requirements:**
- Return remaining count and reset time
- Support burst allowance
- Configurable windows (1s, 1m, 1h)

---

#### Session Store
| Feature | Priority | Description |
|---------|----------|-------------|
| Create session | P0 | Store session data with TTL |
| Get session | P0 | Retrieve session by ID |
| Update session | P0 | Modify session data |
| Delete session | P0 | Invalidate session |
| List user sessions | P1 | Get all sessions for user |

**Session Data:**
| Field | Description |
|-------|-------------|
| user_id | Associated user |
| device_id | Device identifier |
| device_info | Device name, OS, app version |
| ip_address | Last known IP |
| created_at | Session creation time |
| last_active | Last activity timestamp |

**Requirements:**
- Session key: `session:{session_id}`
- User sessions index: `user:sessions:{user_id}` (SET)
- Default TTL: 30 days
- Update last_active on each access

---

## Common Patterns

### Repository Pattern
| Requirement | Description |
|-------------|-------------|
| Interface-based | Define repository interfaces in service |
| Implementation | Implement using postgres package |
| Testability | Easy to mock for unit tests |
| Context propagation | All methods take context |

### Error Handling
| Error Type | Description |
|------------|-------------|
| ErrNotFound | Record not found |
| ErrDuplicate | Unique constraint violation |
| ErrForeignKey | Foreign key violation |
| ErrConnection | Database connection error |
| ErrTimeout | Query timeout |

**Requirements:**
- Map PostgreSQL error codes to domain errors
- Wrap original error for debugging
- Use `errors.Is()` for checking

---

## Dependencies

**Internal:**
- `txova-go-types`
- `txova-go-core`

**External:**
- `github.com/jackc/pgx/v5` — PostgreSQL driver
- `github.com/redis/go-redis/v9` — Redis client
- `github.com/golang-migrate/migrate/v4` — Migrations

---

## Success Metrics
| Metric | Target |
|--------|--------|
| Test coverage | > 85% |
| Connection pool efficiency | > 90% |
| Cache hit rate | > 70% |
| Query latency P99 | < 100ms |
| Lock contention rate | < 5% |
