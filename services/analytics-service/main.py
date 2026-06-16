"""
Analytics Service — Main Application
Consumes Kafka click events and provides analytics APIs
"""
import asyncio
import logging
from contextlib import asynccontextmanager

from fastapi import FastAPI
from fastapi.middleware.cors import CORSMiddleware
from prometheus_client import make_asgi_app

from app.config import settings
from app.kafka_consumer import ClickEventConsumer
from app.routers import analytics
from app.database import clickhouse_client

logging.basicConfig(level=logging.INFO)
logger = logging.getLogger(__name__)


@asynccontextmanager
async def lifespan(app: FastAPI):
    """Startup and shutdown lifecycle manager"""
    # ── Startup ────────────────────────────────────────────────────
    logger.info("Analytics Service starting...")

    # Start Kafka consumer in background
    consumer = ClickEventConsumer(
        brokers=settings.KAFKA_BROKERS,
        topic="url.clicked",
        group_id="analytics-aggregator",
        ch_client=clickhouse_client,
    )
    consumer_task = asyncio.create_task(consumer.start())
    logger.info("Kafka consumer started")

    yield  # App is running

    # ── Shutdown ───────────────────────────────────────────────────
    consumer_task.cancel()
    await clickhouse_client.close()
    logger.info("Analytics Service stopped")


app = FastAPI(
    title="URL Analytics Service",
    description="Real-time click analytics for the URL Shortener platform",
    version="1.0.0",
    lifespan=lifespan,
)

# CORS
app.add_middleware(
    CORSMiddleware,
    allow_origins=["*"],
    allow_methods=["*"],
    allow_headers=["*"],
)

# Prometheus metrics endpoint
metrics_app = make_asgi_app()
app.mount("/metrics", metrics_app)

# Routers
app.include_router(analytics.router, prefix="/api/v1/analytics", tags=["analytics"])


@app.get("/health")
async def health():
    return {"status": "ok", "service": "analytics-service"}
