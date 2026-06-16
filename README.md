# 🚀 URL Shortener Microservice Platform

> A production-grade, MAANG-interview-ready microservice system built with real-world system design patterns.

## 🏗️ Architecture Overview

```
Client → API Gateway (Kong) → Services → Databases
                                       → Kafka → Analytics
```

## 📦 Services

| Service | Language | Port | Responsibility |
|---------|----------|------|----------------|
| `url-service` | Go | 8001 | Create/manage short URLs |
| `redirect-service` | Go | 8002 | Resolve & redirect (HOT PATH) |
| `analytics-service` | Python | 8003 | Kafka consumer + analytics API |
| `auth-service` | Node.js | 8004 | JWT + API Key + Rate Limiting |
| `notification-service` | Node.js | 8005 | Email/Webhook alerts |

## 🎨 System Design Topics Covered

- ✅ API Gateway & Rate Limiting
- ✅ CQRS Pattern
- ✅ Outbox Pattern (Guaranteed Event Delivery)
- ✅ Circuit Breaker
- ✅ Consistent Hashing
- ✅ Database Sharding
- ✅ Multi-tier Caching (Redis)
- ✅ Event-Driven Architecture (Kafka)
- ✅ Distributed Tracing (OpenTelemetry + Jaeger)
- ✅ Snowflake ID Generation
- ✅ Kubernetes + HPA Auto-scaling
- ✅ ClickHouse OLAP Analytics
- ✅ WebSocket Real-time Dashboard
- ✅ CDN Integration

## 🚀 Quick Start (Local Development)

### Prerequisites
- Docker & Docker Compose
- Go 1.21+
- Python 3.11+
- Node.js 20+

### Run Everything Locally

```bash
# Clone the repo
git clone https://github.com/Anwesha0425/url-shortener-microservice.git
cd url-shortener-microservice

# Start infrastructure (Postgres, Redis, Kafka, ClickHouse)
docker compose up -d

# Start URL Service
cd services/url-service && go run cmd/main.go

# Start Redirect Service
cd services/redirect-service && go run cmd/main.go

# Start Analytics Service
cd services/analytics-service && uvicorn main:app --reload

# Start Auth Service
cd services/auth-service && npm run dev
```

## 📁 Project Structure

```
url-shortener-microservice/
├── services/
│   ├── url-service/           # Go - Write path
│   ├── redirect-service/      # Go - Hot read path
│   ├── analytics-service/     # Python - Event processing
│   ├── auth-service/          # Node.js - Auth + Rate limiting
│   └── notification-service/  # Node.js - Email/Webhook
├── infrastructure/
│   ├── k8s/                   # Kubernetes manifests
│   ├── terraform/             # Infrastructure as Code
│   └── kafka/                 # Kafka topic configs
├── api-gateway/               # Kong configuration
├── monitoring/                # Prometheus + Grafana
├── docker-compose.yml         # Local development
└── docs/                      # Architecture diagrams
```

## 📊 API Reference

### URL Service

```http
POST   /api/v1/urls              # Create short URL
GET    /api/v1/urls/:id          # Get URL details
PUT    /api/v1/urls/:id          # Update URL
DELETE /api/v1/urls/:id          # Delete URL
GET    /api/v1/urls              # List user's URLs
```

### Redirect Service

```http
GET /:short_code                 # Redirect to original URL
```

### Analytics Service

```http
GET /api/v1/analytics/:code/summary     # Click summary
GET /api/v1/analytics/:code/timeseries  # Time series data
GET /api/v1/analytics/:code/geo         # Geographic breakdown
GET /api/v1/analytics/:code/referrers   # Referrer breakdown
```

## 🧪 Load Testing

```bash
# Run k6 load test
k6 run scripts/load-test.js
```

## 📈 Performance Targets

| Metric | Target |
|--------|--------|
| Redirect latency (p99) | < 50ms |
| Cache hit rate | > 95% |
| Error rate | < 0.1% |
| Throughput | 115,000 req/sec |

## 🏢 Interview System Design Topics

See [SYSTEM_DESIGN.md](./docs/SYSTEM_DESIGN.md) for detailed explanations of every design decision.

---

Built with ❤️ for MAANG interviews and real-world production systems.
