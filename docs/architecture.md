# URL Shortener Platform — System Design Deep Dive

> A production-grade URL shortener built as a portfolio of distributed systems patterns.
> This document explains the *why* behind every major technical decision.

---

## Architecture Overview

```
                        ┌─────────────────────────────────────────────────────────┐
                        │                   CLIENT LAYER                          │
                        │       Browser / Mobile App / Third-party Integrations   │
                        └────────────────────┬────────────────────────────────────┘
                                             │ HTTPS
                        ┌────────────────────▼────────────────────────────────────┐
                        │              CLOUDFLARE EDGE (300+ PoPs)                │
                        │    KV Cache → < 1ms redirect globally (cache HIT)       │
                        └────────────────────┬────────────────────────────────────┘
                                             │ Cache MISS only (~5% of traffic)
                        ┌────────────────────▼────────────────────────────────────┐
                        │                  KONG API GATEWAY                       │
                        │    Rate Limiting (Token Bucket) · JWT Auth · CORS       │
                        │    Prometheus Metrics · X-Request-ID Tracing            │
                        └────┬────────────────┬──────────────────┬───────────────┘
                             │                │                  │
               ┌─────────────▼──┐  ┌──────────▼────────┐  ┌────▼──────────────────┐
               │  URL Service   │  │ Redirect Service  │  │    Auth Service       │
               │  (Go / Gin)    │  │  (Go / Hot Path)  │  │  (Node.js / gRPC)     │
               │  Port: 8001    │  │  Port: 8002        │  │  Port: 8004 / 50051   │
               └───────┬────────┘  └──────────┬─────────┘  └───────────────────────┘
                       │                       │
         ┌─────────────▼──────────────────┐    │  Redirect Resolution Chain:
         │   PostgreSQL (Sharded)         │    │  1. Bloom Filter (0ms  — is it real?)
         │   16 shards via FNV-32 hash    │◄───┤  2. Redis Cache  (~2ms — cache HIT?)
         │   Primary + Read Replicas      │    │  3. Circuit Breaker → PostgreSQL
         └─────────────────────────────────┘    │  4. Publish click → Kafka (async)
                                                │
         ┌──────────────────────────────────────▼────────────────────────────────────┐
         │                           KAFKA (Event Bus)                               │
         │    url.created · url.clicked · url.expired · analytics.milestone          │
         └──────┬────────────────────────────────────────────────────┬───────────────┘
                │                                                    │
  ┌─────────────▼──────────────┐                     ┌──────────────▼──────────────┐
  │   Analytics Service        │                     │  Notification Service        │
  │   (Python / FastAPI)       │                     │  (Node.js / KafkaJS)        │
  │   ClickHouse OLAP          │                     │  Email · Webhook delivery   │
  │   WebSocket real-time push │                     └─────────────────────────────┘
  └────────────────────────────┘
```

---

## Distributed Systems Patterns Used

### 1. Snowflake ID Generation
**File**: `url-service/internal/service/url_service.go`

**Problem**: We need globally unique IDs for URLs without a centralized ID generator (which becomes a bottleneck).

**Solution**: Snowflake-style IDs composed of:
```
┌──────────────────────────┬───────────────┬──────────────┐
│  41-bit timestamp (ms)   │ 10-bit node ID│ 12-bit seq   │
└──────────────────────────┴───────────────┴──────────────┘
```
- 41-bit timestamp: ~69 years of IDs
- 10-bit node ID: 1,024 worker nodes
- 12-bit sequence: 4,096 IDs per millisecond per node

The ID is then Base62-encoded (`[0-9A-Za-z]`) to generate the short code.

**Why not UUID?** UUIDs are 128-bit and random — they cause random index writes in PostgreSQL (poor B-tree locality). Snowflake IDs are time-ordered, which means sequential inserts cluster together on disk.

---

### 2. Transactional Outbox Pattern
**File**: `url-service/internal/repository/outbox_repository.go`

**Problem**: After creating a URL, we need to publish a Kafka event. But what if the service crashes between the DB commit and the Kafka publish? The event is lost — silent data inconsistency.

**Solution**: Write the event to an `outbox_events` table **in the same transaction** as the URL insert. A background poller reads from `outbox_events` and publishes to Kafka, then deletes the row.

```sql
BEGIN;
  INSERT INTO urls (short_code, ...) VALUES (...);
  INSERT INTO outbox_events (topic, payload) VALUES ('url.created', '...');
COMMIT;
-- Separate poller:
SELECT * FROM outbox_events WHERE published = false;
-- → publish to Kafka
-- → DELETE FROM outbox_events WHERE id = ...
```

**Guarantee**: At-least-once delivery. Kafka consumers must be idempotent.

---

### 3. CQRS (Command Query Responsibility Segregation)
**Files**: `url-service` (write) vs `analytics-service` (read)

**Problem**: Write workloads (creating URLs, updating clicks) and read workloads (analytics dashboards, aggregations) have completely different query patterns and scaling needs.

**Solution**: Separate the write path (PostgreSQL) from the read path (ClickHouse):
- **Write path**: PostgreSQL (ACID guarantees, row-level locking)
- **Read path**: ClickHouse (columnar storage, billion-row aggregations in milliseconds)
- **Sync**: Kafka event stream bridges the two

---

### 4. Token Bucket Rate Limiting
**File**: `auth-service/src/cache/rateLimiter.ts`

**Why Token Bucket over Fixed Window?**

Fixed Window: All 100 requests allowed at second 0-1, then 100 more at second 1-2.  
A user can burst 200 requests at the boundary (seconds 0.9 → 1.1) — the "thundering herd" problem.

Token Bucket: Tokens refill at a steady rate. Short bursts are allowed (up to `burst` tokens), but sustained rate is capped.

Implementation: Single Lua script executes atomically in Redis — no race conditions.

```lua
local elapsed = now - last_refill
local new_tokens = min(capacity, tokens + floor(elapsed * rps / 1000))
if new_tokens < 1 then return {0, 0, capacity} end  -- Rate limited
```

**Tier limits:**
| Tier       | RPS | Burst |
|------------|-----|-------|
| Free       | 10  | 50    |
| Pro        | 100 | 500   |
| Enterprise | 1K  | 5K    |

---

### 5. Circuit Breaker
**File**: `redirect-service/internal/circuitbreaker/circuit_breaker.go`

**Problem**: If PostgreSQL is slow or down, the redirect service will queue up thousands of goroutines waiting for DB responses. The service eventually OOMs.

**Solution**: Circuit Breaker with 3 states:
```
CLOSED ──(5 failures in 10s)──► OPEN ──(30s timeout)──► HALF-OPEN
  ▲                                                            │
  └──────────────────(1 success)──────────────────────────────┘
```
- **CLOSED**: Normal operation, requests pass through
- **OPEN**: DB is failing — immediately return 503 (no DB hit)
- **HALF-OPEN**: Test one request — if it succeeds, close the breaker

---

### 6. Bloom Filter (Cache Penetration Defense)
**File**: `redirect-service/internal/bloomfilter/bloom.go`

**Problem**: Cache Penetration Attack — attacker spams non-existent short codes, bypassing Redis and hammering the database.

**Solution**: Space-efficient probabilistic data structure loaded at startup.

```
Request → Bloom Filter.Test(code)
            │
            ├── "DEFINITELY NOT IN SET" → 404 (no DB hit, ~0ms)
            └── "POSSIBLY IN SET"       → Redis → DB (normal flow)
```

**Math**: For 10M URLs at 1% FPR:
- Bit array size: `m = -n*ln(p)/ln(2)² ≈ 95.8M bits = ~11.5MB`
- Hash functions: `k = (m/n)*ln(2) ≈ 7`
- Lookup cost: 7 bitwise checks < 1 microsecond

**Zero false negatives**: A real short code will *always* pass the filter. A non-existent one fails 99% of the time.

---

### 7. Consistent Hashing (Database Sharding)
**File**: `url-service/internal/database/shard_router.go`

**Problem**: As URL count grows to billions, a single PostgreSQL instance can't handle the write throughput or storage.

**Solution**: FNV-32 hash of `short_code` % `shard_count` determines which Postgres instance stores the URL.

```
short_code "aB3xY"
    → FNV32("aB3xY") = 2847562910
    → 2847562910 % 16 = 14
    → Shard #14
```

**Why FNV-32 over MD5/SHA?**: 10-50x faster with equally good distribution for short strings. No crypto overhead.

**Why `short_code` not `user_id`?**: Avoids hot shards for power users. Short codes distribute uniformly regardless of user behavior.

---

### 8. Distributed Tracing (OpenTelemetry + Jaeger)
**Files**: `*/internal/telemetry/tracer.go`

Every request gets a unique `Trace ID` embedded in the `traceparent` HTTP header (W3C standard). When the redirect service calls Redis and PostgreSQL, child spans are created under the same trace.

**Sampling strategy**:
- Redirect service: 5% (hot path — high volume, low overhead budget)
- URL service: 10% (write path — less volume, more interesting traces)
- Error spans: 100% always sampled

---

### 9. gRPC for Internal Service Communication
**Files**: `shared/protos/auth.proto`, `auth-service/src/grpc/server.ts`

**Why REST for external, gRPC for internal?**

| | REST + JSON | gRPC + Protobuf |
|--|--|--|
| Protocol | HTTP/1.1 | HTTP/2 |
| Payload | ~200 bytes JSON | ~20 bytes binary |
| Serialization | 10-50µs | 1-5µs |
| Multiplexing | No (connection pool) | Yes (streams on 1 conn) |
| Type safety | Runtime (schema optional) | Compile-time enforced |

For `VerifyToken` (called on every write), this matters: 50µs × 100K req/min = **5 seconds of savings per minute** vs REST.

---

### 10. Event-Driven Architecture (Kafka)
**Topics**:
| Topic | Producer | Consumers |
|--|--|--|
| `url.created` | url-service (Outbox) | analytics-service, notification-service |
| `url.clicked` | redirect-service | analytics-service |
| `url.expired` | scheduler | notification-service |
| `analytics.milestone` | analytics-service | notification-service |

**Why Kafka over RabbitMQ?**
- Kafka retains events by time (replay capability for analytics)
- Kafka consumer groups allow multiple independent consumers of the same event
- Kafka handles 1M+ messages/sec; RabbitMQ peaks at ~50K

---

## SLO Targets

| SLO | Target | Measurement |
|-----|--------|-------------|
| Redirect p99 latency | < 50ms | Prometheus histogram |
| URL creation p99 | < 200ms | Prometheus histogram |
| Cache hit rate | > 95% | `cache_hits / (cache_hits + misses)` |
| Availability | 99.9% | `(total - errors) / total * 100` |
| Error rate | < 0.1% | `5xx / total requests` |

---

## Infrastructure Topology

```
AWS EKS Cluster (ap-south-1)
├── Node Group: t3.medium × 3-20 nodes (auto-scaling)
│
├── url-shortener namespace
│   ├── url-service         Deployment  (3-20 pods)  HPA: CPU 70%
│   ├── redirect-service    Deployment  (10-100 pods) HPA: CPU 60%
│   ├── analytics-service   Deployment  (2-10 pods)  HPA: Memory 75%
│   ├── auth-service        Deployment  (2-5 pods)
│   ├── notification-service Deployment (1-3 pods)
│   └── url-expiry-scheduler CronJob    (every 5 min)
│
├── Data Layer
│   ├── RDS PostgreSQL r6g.large (Multi-AZ, 100GB auto-scaling → 1TB)
│   ├── ElastiCache Redis r6g.large (1 primary + 2 read replicas)
│   └── ClickHouse (self-managed on EKS, dedicated node group)
│
└── Edge Layer
    └── Cloudflare Workers (300+ PoPs worldwide, KV edge cache)
```

---

## Why Every Design Decision Was Made

| Decision | Alternative Considered | Why We Chose This |
|----------|----------------------|-------------------|
| Go for URL/Redirect | Java, Node.js | Go goroutines handle 100K concurrent connections with minimal memory |
| Python for Analytics | Go, Java | Pandas/ClickHouse ecosystem, rapid ETL development |
| ClickHouse for analytics | PostgreSQL + TimescaleDB | ClickHouse ingests 1M rows/sec; columnar reads are 100x faster for aggregations |
| Kafka over Redis Pub/Sub | Redis Streams | Kafka retains messages; multiple independent consumers; horizontal partition scaling |
| Bloom Filter | Null object cache | Bloom filter uses 11MB for 10M URLs; null cache uses ~500MB |
| Base62 encoding | UUID, nanoid | Base62 is URL-safe, human-readable, and produces predictably short strings |
