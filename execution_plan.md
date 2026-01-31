# txova-go-db Execution Plan

## Overview

Implementation plan for the database utilities library providing PostgreSQL and Redis support for the Txova platform.

**Target Coverage:** 90%+  
**Dependencies:** `txova-go-types`, `txova-go-core`

---

## Phase 1: Foundation (Week 1)

### 1.1 Project Setup
- Initialize Go module with `github.com/Dorico-Dynamics/txova-go-db`
- Add dependencies: `pgx/v5`, `go-redis/v9`, `golang-migrate/v4`
- Add internal dependencies: `txova-go-types`, `txova-go-core`
- Create package structure: `postgres/`, `redis/`

### 1.2 Error Types
- Define `DBErrorCode` type with standard codes: `ErrNotFound`, `ErrDuplicate`, `ErrForeignKey`, `ErrConnection`, `ErrTimeout`
- Map PostgreSQL error codes (SQLSTATE) to domain error codes
- Implement error wrapping compatible with `txova-go-core/errors`
- Support `errors.Is()` and `errors.As()` for error checking
- Write comprehensive tests for error mapping

### 1.3 Core Interfaces
- Define `Pool` interface for connection pool abstraction
- Define `Conn` interface for single connection operations
- Define `Tx` interface for transaction operations
- Define `Querier` interface (common to Pool, Conn, Tx) for query execution
- Ensure interfaces are minimal and testable

---

## Phase 2: PostgreSQL Connection Management (Week 2)

### 2.1 Connection Pool
- Implement pool wrapper around `pgxpool.Pool`
- Support configuration via functional options pattern (match `txova-go-core` style)
- Configure: max/min connections, lifetime, idle timeout, health check period, connect timeout
- Implement SSL mode configuration
- Integrate with `txova-go-core/config.DatabaseConfig` for configuration loading

### 2.2 Connection Lifecycle
- Implement connection retry with exponential backoff on startup
- Implement health check method (`Ping`) for readiness probes
- Implement graceful close with connection draining
- Add connection pool statistics for observability

### 2.3 Logging Integration
- Integrate with `txova-go-core/logging` for structured logging
- Log connection events: connect, disconnect, errors
- Log slow queries (configurable threshold)
- Ensure PII is not logged in query parameters

### 2.4 Tests
- Unit tests with mocked pool interface
- Table-driven tests for configuration options
- Tests for retry logic and error handling

---

## Phase 3: Transaction Management (Week 3)

### 3.1 Transaction Wrapper
- Implement `WithTx(ctx, func(tx) error)` pattern
- Automatic commit on success, rollback on error
- Automatic rollback on panic with re-panic after cleanup
- Support isolation level configuration (Read Committed, Repeatable Read, Serializable)
- Support read-only transaction optimization

### 3.2 Context Propagation
- Store transaction in context for nested access
- Implement `TxFromContext(ctx)` to retrieve active transaction
- Propagate existing transaction automatically (avoid nested BEGIN)
- Implement savepoint support for nested transaction semantics

### 3.3 Timeout Handling
- Respect context deadlines and cancellation
- Implement query-level timeout configuration
- Ensure proper cleanup on context cancellation

### 3.4 Tests
- Tests for commit/rollback scenarios
- Tests for panic recovery
- Tests for nested transaction handling
- Tests for timeout and cancellation

---

## Phase 4: Query Builder (Week 4)

### 4.1 SELECT Builder
- Build SELECT queries with column selection
- Support WHERE conditions: AND, OR, IN, LIKE, IS NULL, IS NOT NULL
- Support comparison operators: =, !=, <, >, <=, >=
- Support ORDER BY with direction (ASC, DESC)
- Support LIMIT and OFFSET for pagination
- Support cursor-based pagination

### 4.2 INSERT/UPDATE/DELETE Builders
- Build INSERT with RETURNING clause support
- Build UPDATE with WHERE conditions
- Build DELETE with WHERE conditions
- Support batch INSERT operations

### 4.3 JOIN Support
- Support INNER JOIN, LEFT JOIN, RIGHT JOIN
- Support multiple joins in single query
- Support join conditions

### 4.4 Safety Features
- Use parameterized queries exclusively (prevent SQL injection)
- Validate column names against configurable allowlist
- Support typed ID parameters from `txova-go-types/ids`
- Provide method to retrieve generated SQL and args for debugging

### 4.5 Tests
- Tests for each query type
- Tests for SQL injection prevention
- Tests for parameter binding with typed IDs
- Tests for complex WHERE conditions

---

## Phase 5: Migration Runner (Week 5)

### 5.1 Migration Infrastructure
- Integrate with `golang-migrate/migrate` library
- Support filesystem-based migration files
- Support `embed.FS` for embedded migrations
- Migration file naming: `NNNN_description.up.sql`, `NNNN_description.down.sql`

### 5.2 Migration Operations
- Implement up migrations (apply pending)
- Implement down migrations (rollback)
- Implement version tracking in database
- Implement dry-run mode (preview without applying)

### 5.3 Safety Features
- Lock migration table during execution (prevent concurrent runs)
- Fail fast on migration errors
- Log each migration applied with timing
- Integrate logging with `txova-go-core/logging`

### 5.4 Tests
- Tests with embedded test migrations
- Tests for up/down operations
- Tests for version tracking
- Tests for concurrent execution prevention

---

## Phase 6: Redis Connection Management (Week 6)

### 6.1 Client Setup
- Implement client wrapper around `go-redis/v9`
- Support configuration via functional options
- Configure: host, port, password, database, pool size
- Implement health check (`PING`) for readiness probes

### 6.2 Cluster and Sentinel Support
- Implement Redis cluster mode support
- Implement Redis sentinel mode support
- Abstract connection mode behind common interface

### 6.3 Logging Integration
- Integrate with `txova-go-core/logging`
- Log connection events and errors

### 6.4 Tests
- Unit tests with mocked client
- Tests for configuration options

---

## Phase 7: Redis Caching (Week 7)

### 7.1 Basic Operations
- Implement Get/Set with TTL support
- Implement Delete operation
- Implement GetOrSet (atomic get-or-compute pattern)
- Support JSON serialization for complex types

### 7.2 Batch Operations
- Implement MGET for bulk retrieval
- Implement MSET for bulk storage
- Implement delete by pattern for cache invalidation

### 7.3 Cache Key Management
- Enforce key conventions: `{service}:{entity}:{id}`
- Provide key builder utilities
- Support configurable default TTL (15 minutes)
- Distinguish between nil value and cache miss

### 7.4 Observability
- Log cache hits/misses for metrics
- Integrate with `txova-go-core/logging`

### 7.5 Tests
- Tests for all cache operations
- Tests for serialization/deserialization
- Tests for TTL behavior
- Tests for cache miss vs nil value

---

## Phase 8: Redis Distributed Locking (Week 8)

### 8.1 Lock Operations
- Implement acquire lock with `SET NX` and TTL
- Implement release lock with ownership verification
- Implement lock extension (TTL refresh while holding)
- Implement blocking wait for lock acquisition

### 8.2 Lock Safety
- Lock key format: `lock:{resource}:{id}`
- Include owner identifier to prevent wrong release
- Default TTL: 30 seconds
- Provide `WithLock(ctx, key, func() error)` wrapper pattern

### 8.3 Tests
- Tests for acquire/release
- Tests for ownership verification
- Tests for TTL extension
- Tests for concurrent lock contention

---

## Phase 9: Redis Rate Limiting & Sessions (Week 9)

### 9.1 Rate Limiting
- Implement fixed window rate limiting
- Implement sliding window rate limiting
- Support per-user limits (keyed by user ID)
- Support per-IP limits (keyed by IP address)
- Return remaining count and reset time
- Support burst allowance configuration
- Support configurable windows (1s, 1m, 1h)

### 9.2 Session Store
- Implement session creation with TTL
- Implement session retrieval by ID
- Implement session update
- Implement session deletion (invalidation)
- Implement list all sessions for user
- Store session data: user_id, device_id, device_info, ip_address, created_at, last_active
- Session key format: `session:{session_id}`
- User sessions index: `user:sessions:{user_id}` (SET)
- Default TTL: 30 days
- Update last_active on each access

### 9.3 Tests
- Tests for rate limit scenarios
- Tests for window expiration
- Tests for session CRUD operations
- Tests for session listing

---

## Phase 10: Integration & Documentation (Week 10)

### 10.1 Integration Testing
- Set up testcontainers for PostgreSQL
- Set up testcontainers for Redis
- Write end-to-end integration tests
- Verify all components work together

### 10.2 Final Validation
- Run full test suite with coverage report
- Verify 90%+ coverage target
- Run `golangci-lint` with comprehensive ruleset
- Fix any linting issues

### 10.3 Documentation
- Write README.md with quick start guide
- Write USAGE.md with detailed examples
- Ensure all exported types and functions have godoc comments

---

## Success Criteria

| Metric | Target |
|--------|--------|
| Test coverage | > 90% |
| Connection pool efficiency | > 90% |
| All P0 features implemented | 100% |
| All P1 features implemented | 100% |
| Zero critical linting issues | ✓ |
| Integration tests passing | ✓ |

