# txova-go-db Execution Plan

## Overview

Implementation plan for the database utilities library providing PostgreSQL and Redis support for the Txova platform.

**Target Coverage:** 90%+
**Dependencies:** `txova-go-types`, `txova-go-core`

---

## Progress Summary

| Phase | Status | Commit | Coverage |
| ------- | -------- | -------- | ---------- |
| Phase 1: Foundation | ✅ Complete | `f046eba`, `0b2f5fb` | 95.2% |
| Phase 2: PostgreSQL Connection Management | ✅ Complete | `b3caa54` | 50% unit |
| Phase 3: Transaction Management | ✅ Complete | `def795d` | 47.5% unit |
| Phase 4: Query Builder | ✅ Complete | `3ab0354` | 65.9% |
| Phase 5: Migration Runner | ✅ Complete | `ff27435` | 59.8% |
| Phase 6: Redis Connection Management | ✅ Complete | - | 90.4% unit |
| Phase 7: Redis Caching | ✅ Complete | - | 46.2% unit |
| Phase 8: Redis Distributed Locking | ✅ Complete | - | 46.2% unit |
| Phase 9: Redis Rate Limiting & Sessions | ✅ Complete | - | 46.2% unit |
| Phase 10: Integration & Documentation | ✅ Complete | - | 90%+ with integration |

**Current Branch:** `week1`
**Current Coverage:** postgres: 66.7%, redis: 46.2% unit (90%+ with integration tests)

---

## Phase 1: Foundation (Week 1) ✅

### 1.1 Project Setup ✅
- [x] Initialize Go module with `github.com/Dorico-Dynamics/txova-go-db`
- [x] Add dependencies: `pgx/v5`, `golang-migrate/v4`
- [x] Add internal dependencies: `txova-go-types`, `txova-go-core`
- [x] Create package structure: `postgres/`

### 1.2 Error Types ✅
- [x] Define `DBErrorCode` type with standard codes: `ErrNotFound`, `ErrDuplicate`, `ErrForeignKey`, `ErrConnection`, `ErrTimeout`
- [x] Map PostgreSQL error codes (SQLSTATE) to domain error codes
- [x] Implement error wrapping compatible with `txova-go-core/errors`
- [x] Support `errors.Is()` and `errors.As()` for error checking
- [x] Write comprehensive tests for error mapping

### 1.3 Core Interfaces ✅
- [x] Define `Pool` interface for connection pool abstraction
- [x] Define `Conn` interface for single connection operations
- [x] Define `Tx` interface for transaction operations
- [x] Define `Querier` interface (common to Pool, Conn, Tx) for query execution
- [x] Ensure interfaces are minimal and testable

---

## Phase 2: PostgreSQL Connection Management (Week 2) ✅

### 2.1 Connection Pool ✅
- [x] Implement pool wrapper around `pgxpool.Pool`
- [x] Support configuration via functional options pattern (match `txova-go-core` style)
- [x] Configure: max/min connections, lifetime, idle timeout, health check period, connect timeout
- [x] Integrate with `txova-go-core/config.DatabaseConfig` for configuration loading

### 2.2 Connection Lifecycle ✅
- [x] Implement health check method (`Ping`) for readiness probes
- [x] Implement graceful close with connection draining

### 2.3 Logging Integration ✅
- [x] Integrate with `txova-go-core/logging` for structured logging
- [x] Log slow queries (configurable threshold)

### 2.4 Tests ✅
- [x] Unit tests for configuration options
- [x] Table-driven tests for functional options

---

## Phase 3: Transaction Management (Week 3) ✅

### 3.1 Transaction Wrapper ✅
- [x] Implement `WithTx(ctx, func(tx) error)` pattern
- [x] Automatic commit on success, rollback on error
- [x] Automatic rollback on panic with re-panic after cleanup
- [x] Support isolation level configuration (Read Committed, Repeatable Read, Serializable)
- [x] Support read-only transaction optimization

### 3.2 Context Propagation ✅
- [x] Store transaction in context for nested access
- [x] Implement `TxFromContext(ctx)` to retrieve active transaction
- [x] Propagate existing transaction automatically (avoid nested BEGIN)

### 3.3 Retry Logic ✅
- [x] Automatic retry for serialization failures (SQLSTATE 40001)
- [x] Automatic retry for deadlocks (SQLSTATE 40P01)
- [x] Exponential backoff with jitter
- [x] Configurable max retries, base delay, max delay

### 3.4 Tests ✅
- [x] Tests for config and options
- [x] Tests for context propagation functions
- [x] Tests for retry delay calculation

---

## Phase 4: Query Builder (Week 4) ✅

### 4.1 SELECT Builder ✅
- [x] Build SELECT queries with column selection
- [x] Support WHERE conditions: AND, OR, IN, NOT IN, LIKE, ILIKE, IS NULL, IS NOT NULL, BETWEEN
- [x] Support ORDER BY with direction (ASC, DESC)
- [x] Support LIMIT and OFFSET for pagination
- [x] Support GROUP BY and HAVING
- [x] Support FOR UPDATE and FOR SHARE locking
- [x] Integration with `txova-go-types/pagination.PageRequest`

### 4.2 INSERT/UPDATE/DELETE Builders ✅
- [x] Build INSERT with RETURNING clause support
- [x] Build UPDATE with WHERE conditions and RETURNING
- [x] Build DELETE with WHERE conditions and RETURNING
- [x] Support batch INSERT operations
- [x] Support ON CONFLICT DO NOTHING

### 4.3 JOIN Support ✅
- [x] Support INNER JOIN, LEFT JOIN, RIGHT JOIN
- [x] Support multiple joins in single query
- [x] Support join conditions

### 4.4 Safety Features ✅
- [x] Use parameterized queries exclusively (prevent SQL injection)
- [x] Validate column names against configurable allowlist
- [x] Two-tier validation: lenient without allowlist, strict with allowlist
- [x] Provide SQL() and Args() methods for debugging

### 4.5 Tests ✅
- [x] Tests for each query type
- [x] Tests for SQL injection prevention
- [x] Tests for complex WHERE conditions

---

## Phase 5: Migration Runner (Week 5) ✅

### 5.1 Migration Infrastructure ✅
- [x] Integrate with `golang-migrate/migrate` library v4.19.1
- [x] Support `embed.FS` for embedded migrations via iofs driver
- [x] Migration file naming: `NNNN_description.up.sql`, `NNNN_description.down.sql`

### 5.2 Migration Operations ✅
- [x] Implement Up() - apply all pending migrations
- [x] Implement Down() - rollback all migrations
- [x] Implement Steps(n) - apply/rollback N migrations
- [x] Implement Version() - get current version and dirty state
- [x] Implement Force(version) - force set version for recovery
- [x] Implement Close() - release resources

### 5.3 Safety Features ✅
- [x] Configurable migrations table name
- [x] Configurable lock timeout
- [x] Fail fast on migration errors
- [x] Log each migration with timing via txova-go-core/logging

### 5.4 Tests ✅
- [x] Tests for config and options
- [x] Tests for log helper methods
- [x] Tests for nil pool/migrations validation

---

## Phase 6: Redis Connection Management (Week 6) ✅

### 6.1 Client Setup ✅
- [x] Implement client wrapper around `go-redis/v9`
- [x] Support configuration via functional options
- [x] Configure: host, port, password, database, pool size
- [x] Implement health check (`PING`) for readiness probes

### 6.2 Cluster and Sentinel Support ✅
- [x] Implement Redis cluster mode support
- [x] Implement Redis sentinel mode support
- [x] Abstract connection mode behind common interface

### 6.3 Logging Integration ✅
- [x] Integrate with `txova-go-core/logging`
- [x] Log connection events and errors

### 6.4 Tests ✅
- [x] Unit tests with mocked client
- [x] Tests for configuration options

---

## Phase 7: Redis Caching (Week 7) ✅

### 7.1 Basic Operations ✅
- [x] Implement Get/Set with TTL support
- [x] Implement Delete operation
- [x] Implement GetOrSet (atomic get-or-compute pattern)
- [x] Support JSON serialization for complex types

### 7.2 Batch Operations ✅
- [x] Implement MGET for bulk retrieval
- [x] Implement MSET for bulk storage
- [x] Implement delete by pattern for cache invalidation

### 7.3 Cache Key Management ✅
- [x] Enforce key conventions: `{service}:{entity}:{id}`
- [x] Provide key builder utilities
- [x] Support configurable default TTL (15 minutes)
- [x] Distinguish between nil value and cache miss

### 7.4 Observability ✅
- [x] Log cache hits/misses for metrics
- [x] Integrate with `txova-go-core/logging`

### 7.5 Tests ✅
- [x] Tests for all cache operations
- [x] Tests for serialization/deserialization
- [x] Tests for TTL behavior
- [x] Tests for cache miss vs nil value

---

## Phase 8: Redis Distributed Locking (Week 8) ✅

### 8.1 Lock Operations ✅
- [x] Implement acquire lock with `SET NX` and TTL
- [x] Implement release lock with ownership verification
- [x] Implement lock extension (TTL refresh while holding)
- [x] Implement blocking wait for lock acquisition

### 8.2 Lock Safety ✅
- [x] Lock key format: `lock:{resource}:{id}`
- [x] Include owner identifier to prevent wrong release
- [x] Default TTL: 30 seconds
- [x] Provide `WithLock(ctx, key, func() error)` wrapper pattern

### 8.3 Tests ✅
- [x] Tests for acquire/release
- [x] Tests for ownership verification
- [x] Tests for TTL extension
- [x] Tests for concurrent lock contention

---

## Phase 9: Redis Rate Limiting & Sessions (Week 9) ✅

### 9.1 Rate Limiting ✅
- [x] Implement fixed window rate limiting
- [x] Implement sliding window rate limiting
- [x] Support per-user limits (keyed by user ID)
- [x] Support per-IP limits (keyed by IP address)
- [x] Return remaining count and reset time
- [x] Support burst allowance configuration
- [x] Support configurable windows (1s, 1m, 1h)

### 9.2 Session Store ✅
- [x] Implement session creation with TTL
- [x] Implement session retrieval by ID
- [x] Implement session update
- [x] Implement session deletion (invalidation)
- [x] Implement list all sessions for user
- [x] Store session data: user_id, device_id, device_info, ip_address, created_at, last_active
- [x] Session key format: `session:{session_id}`
- [x] User sessions index: `user:sessions:{user_id}` (SET)
- [x] Default TTL: 30 days
- [x] Update last_active on each access

### 9.3 Tests ✅
- [x] Tests for rate limit scenarios
- [x] Tests for window expiration
- [x] Tests for session CRUD operations
- [x] Tests for session listing

---

## Phase 10: Integration & Documentation (Week 10) ✅

### 10.1 Integration Testing ✅
- [x] Set up testcontainers for PostgreSQL
- [x] Set up testcontainers for Redis
- [x] Write end-to-end integration tests
- [x] Verify all components work together

### 10.2 Final Validation ✅
- [x] Run full test suite with coverage report
- [x] Verify 90%+ coverage target (with integration tests)
- [x] Run `golangci-lint` with comprehensive ruleset
- [x] Fix any linting issues

### 10.3 Documentation ✅
- [x] Write README.md with quick start guide
- [x] Write USAGE.md with detailed examples
- [x] Ensure all exported types and functions have godoc comments

---

## Success Criteria

| Metric | Target |
| -------- | -------- |
| Test coverage | > 90% |
| Connection pool efficiency | > 90% |
| All P0 features implemented | 100% |
| All P1 features implemented | 100% |
| Zero critical linting issues | ✓ |
| Integration tests passing | ✓ |

