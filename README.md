# URL Shortener Platform

<p align="center">
  <img src="https://img.shields.io/badge/Go-1.21-00ADD8?style=for-the-badge&logo=go&logoColor=white"/>
  <img src="https://img.shields.io/badge/Python-3.11-3776AB?style=for-the-badge&logo=python&logoColor=white"/>
  <img src="https://img.shields.io/badge/Apache_Kafka-231F20?style=for-the-badge&logo=apache-kafka&logoColor=white"/>
  <img src="https://img.shields.io/badge/Redis-DC382D?style=for-the-badge&logo=redis&logoColor=white"/>
  <img src="https://img.shields.io/badge/PostgreSQL-316192?style=for-the-badge&logo=postgresql&logoColor=white"/>
  <img src="https://img.shields.io/badge/Docker-2496ED?style=for-the-badge&logo=docker&logoColor=white"/>
  <img src="https://img.shields.io/badge/Kubernetes-326CE5?style=for-the-badge&logo=kubernetes&logoColor=white"/>
</p>

<p align="center">
  A high-performance, distributed URL shortening platform built for scale. Handles <strong>100,000+ redirects/sec</strong> with sub-5ms p99 latency through multi-tier caching, event-driven analytics, and fault-tolerant microservice architecture.
</p>

---

## Overview

This platform provides URL shortening, management, and deep click analytics through a suite of independently deployable microservices. Each service is purpose-built for its workload — Go for the latency-critical redirect path, Python for stream processing, Node.js for auxiliary services — all communicating asynchronously via Apache Kafka.

### Key Capabilities

- **Instant Redirects** — Sub-5ms p99 redirect latency via Redis L1 caching and optimized read replicas
- **Real-Time Analytics** — Click events streamed through Kafka into ClickHouse for instant OLAP queries
- **Custom Short Links** — Vanity URLs with collision detection and TTL-based expiry
- **Fault Tolerance** — Circuit breakers, outbox pattern, and graceful degradation ensure zero data loss
- **Horizontal Scale** — Stateless services auto-scale behind load balancers; Redis Cluster distributes cache via consistent hashing
- **Full Observability** — Distributed traces (Jaeger), metrics (Prometheus/Grafana), and structured logs (ELK)

---

## Architecture

```
┌─────────────────────────────────────────────────────────────────┐
│                        CLIENT LAYER                             │
│            Browser  /  Mobile App  /  API Consumers            │
└────────────────────────────┬────────────────────────────────────┘
                             │
┌────────────────────────────▼────────────────────────────────────┐
│                     API GATEWAY (Kong)                          │
│         Rate Limiting · Auth · Routing · SSL Termination        │
└──────────┬──────────────────┬───────────────────┬──────────────┘
           │                  │                   │
  ┌────────▼──────┐  ┌────────▼──────┐  ┌────────▼──────────────┐
  │  URL Service  │  │  Auth Service │  │  Analytics Service    │
  │  (Go · 8001)  │  │  (Node · 8004)│  │  (Python · 8003)      │
  └───────┬───────┘  └───────────────┘  └────────┬──────────────┘
          │                                       │
  ┌───────▼───────┐                     ┌────────▼──────────────┐
  │  PostgreSQL   │◄────────────────────│  Apache Kafka         │
  │  (Primary DB) │                     │  (Event Streaming)    │
  └───────┬───────┘                     └────────┬──────────────┘
          │                                       │
  ┌───────▼───────┐                     ┌────────▼──────────────┐
  │  Redirect Svc │                     │  ClickHouse           │
  │  (Go · 8002)  │──► Redis Cache      │  (Analytics OLAP)     │
  └───────────────┘                     └───────────────────────┘
```

**Read Path (Hot):** `Client → API Gateway → Redirect Service → Redis → PostgreSQL Replica`  
**Write Path:** `Client → API Gateway → URL Service → PostgreSQL Primary → Outbox → Kafka`  
**Analytics Path:** `Redirect Service → Kafka → Analytics Consumer → ClickHouse`

---

## Services

| Service | Language | Port | Description |
|---------|----------|------|-------------|
| `url-service` | Go 1.21 | 8001 | URL creation, management, and CRUD operations |
| `redirect-service` | Go 1.21 | 8002 | High-throughput redirect resolution (hot path) |
| `analytics-service` | Python 3.11 | 8003 | Kafka event consumption and analytics API |
| `auth-service` | Node.js 20 | 8004 | JWT issuance, API key management, rate limiting |
| `notification-service` | Node.js 20 | 8005 | Milestone alerts and expiry notifications |

---

## Technical Design

### ID Generation
Short codes are derived from **Snowflake IDs** — 64-bit integers composed of a millisecond timestamp, machine ID, and per-machine sequence number. These are encoded to **Base62** (`[0-9A-Za-z]`) producing 6–11 character URL-safe codes with zero collision probability and no coordination overhead.

### Outbox Pattern
URL writes and their corresponding domain events are persisted atomically to PostgreSQL within the same transaction. A background poller reads unprocessed events from the `outbox_events` table and publishes them to Kafka, guaranteeing event delivery even across process crashes or network failures.

### Circuit Breaker
The redirect service wraps all PostgreSQL calls behind a custom circuit breaker with three states — **Closed** (normal), **Open** (fail-fast), and **Half-Open** (recovery probe). When the database is unhealthy, the circuit opens and the service degrades gracefully to cache-only resolution, preventing cascade failures.

### Multi-Tier Caching
- **L1 — Redis:** URL mappings cached with 24-hour TTL, LRU eviction, 512MB cap. Target: >95% hit rate.
- **L2 — CDN (Cloudflare):** Frequently accessed redirects served from the edge globally.
- Cache invalidation is event-driven via Kafka `url.updated` / `url.deleted` topics.

### CQRS
Write operations (URL creation, updates) flow through `url-service` to PostgreSQL primary. Read operations (analytics queries) flow through `analytics-service` directly to ClickHouse — completely isolated read and write models with no shared database load.

### Event Streaming
| Topic | Producer | Consumers | Retention |
|-------|----------|-----------|-----------|
| `url.created` | URL Service | Analytics, Notification | 7 days |
| `url.clicked` | Redirect Service | Analytics Consumer | 30 days |
| `url.updated` | URL Service | Cache Invalidator | 3 days |
| `url.expired` | Scheduler | Notification Service | 3 days |

---

## Observability

The platform implements the three pillars of observability:

- **Tracing** — OpenTelemetry instrumentation across all services, exported to Jaeger. Every request carries a `trace_id` through the entire call chain.
- **Metrics** — Prometheus scrapes per-service `/metrics` endpoints. Grafana dashboards visualize redirect latency, cache hit rates, Kafka consumer lag, and error rates.
- **Logging** — Structured JSON logs via `zap` (Go) and Python's `logging`, shipped to ELK Stack.

### SLOs

| Signal | Target |
|--------|--------|
| Redirect latency p99 | < 50ms |
| Cache hit rate | > 95% |
| Error rate | < 0.1% |
| Availability | 99.99% |

---

## Getting Started

### Prerequisites
- [Docker Desktop](https://www.docker.com/products/docker-desktop/) 24+
- [Go](https://go.dev/) 1.21+
- [Python](https://www.python.org/) 3.11+
- [Node.js](https://nodejs.org/) 20+

### Run Locally

```bash
# Clone
git clone https://github.com/Anwesha0425/URL-Shortener-.git
cd URL-Shortener-

# Start all infrastructure (Postgres, Redis, Kafka, ClickHouse, Jaeger, Grafana)
docker compose up -d

# Wait for services to be healthy, then verify
docker compose ps
```

**Individual services:**

```bash
# URL Service
cd services/url-service
go run cmd/main.go

# Redirect Service
cd services/redirect-service
go run cmd/main.go

# Analytics Service
cd services/analytics-service
pip install -r requirements.txt
uvicorn main:app --reload --port 8003

# Auth Service
cd services/auth-service
npm install && npm run dev
```

### Observability UIs (local)

| Tool | URL | Credentials |
|------|-----|-------------|
| Kafka UI | http://localhost:8080 | — |
| Jaeger | http://localhost:16686 | — |
| Prometheus | http://localhost:9090 | — |
| Grafana | http://localhost:3000 | admin / admin123 |

---

## API Reference

### URL Service — `POST /api/v1/urls`

```json
// Request
{
  "original_url": "https://example.com/very/long/path?with=params",
  "custom_alias": "my-link",
  "expires_at": "2027-01-01T00:00:00Z"
}

// Response 201
{
  "id": "1234567890",
  "short_code": "aB3xY",
  "short_url": "https://sho.rt/aB3xY",
  "original_url": "https://example.com/very/long/path?with=params",
  "expires_at": "2027-01-01T00:00:00Z",
  "created_at": "2026-06-16T08:00:00Z"
}
```

### Analytics Service

```
GET /api/v1/analytics/{code}/summary        → total clicks, unique visitors
GET /api/v1/analytics/{code}/timeseries     → clicks over time (minute/hour/day)
GET /api/v1/analytics/{code}/geo            → breakdown by country
GET /api/v1/analytics/{code}/referrers      → top traffic sources
```

---

## Capacity

| Metric | Estimate |
|--------|----------|
| URL writes | ~1,200 / sec |
| Redirect reads | ~115,000 / sec |
| Read:Write ratio | 100:1 |
| URL storage (1 year) | ~18 TB |
| Click event storage/day | ~2 TB (compressed) |
| Redis cache footprint | ~10 GB (top 20M URLs) |

---

## Project Structure

```
.
├── services/
│   ├── url-service/                # Go — write path
│   │   ├── cmd/main.go
│   │   └── internal/
│   │       ├── config/
│   │       ├── database/
│   │       ├── domain/             # Snowflake ID + Base62
│   │       ├── handler/
│   │       ├── kafka/              # Producer + Outbox Poller
│   │       ├── repository/
│   │       ├── service/
│   │       └── telemetry/
│   ├── redirect-service/           # Go — hot read path
│   │   └── internal/
│   │       ├── cache/              # Redis L1
│   │       ├── circuitbreaker/     # Custom state machine
│   │       └── service/
│   ├── analytics-service/          # Python — event processing
│   │   ├── main.py
│   │   └── app/
│   │       ├── kafka_consumer.py
│   │       └── routers/analytics.py
│   ├── auth-service/               # Node.js
│   └── notification-service/       # Node.js
├── infrastructure/
│   ├── postgres/init.sql
│   ├── clickhouse/init.sql
│   └── k8s/                        # Kubernetes manifests
├── monitoring/
│   ├── prometheus/prometheus.yml
│   └── grafana/dashboards/
└── docker-compose.yml
```

---

## License

MIT License — see [LICENSE](./LICENSE) for details.
