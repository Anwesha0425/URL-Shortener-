"""
Kafka Consumer — Processes url.clicked events from Kafka
Persists to ClickHouse for OLAP analytics queries
"""

import asyncio
import json
import logging
from datetime import datetime

from aiokafka import AIOKafkaConsumer
from prometheus_client import Counter, Histogram

logger = logging.getLogger(__name__)

# Prometheus metrics
events_consumed = Counter(
    "analytics_events_consumed_total",
    "Total click events consumed from Kafka",
    ["status"],
)
processing_time = Histogram(
    "analytics_event_processing_seconds",
    "Time spent processing each click event",
)


class ClickEventConsumer:
    """
    Consumes url.clicked events from Kafka and stores in ClickHouse.

    Consumer Group: analytics-aggregator
    Topic: url.clicked

    Kafka guarantees: at-least-once delivery
    We handle idempotency by using ClickHouse's deduplication feature.
    """

    def __init__(self, brokers: str, topic: str, group_id: str, ch_client, ws_manager=None):
        self.brokers = brokers
        self.topic = topic
        self.group_id = group_id
        self.ch_client = ch_client
        self.ws_manager = ws_manager
        self.batch: list[dict] = []
        self.batch_size = 100  # flush every 100 events
        self.flush_interval = 5.0  # or every 5 seconds (whichever comes first)

    async def start(self):
        consumer = AIOKafkaConsumer(
            self.topic,
            bootstrap_servers=self.brokers,
            group_id=self.group_id,
            value_deserializer=lambda v: json.loads(v.decode("utf-8")),
            auto_offset_reset="earliest",
            enable_auto_commit=False,  # manual commit for reliability
        )

        await consumer.start()
        logger.info(f"Kafka consumer started — topic: {self.topic}")

        flush_task = asyncio.create_task(self._periodic_flush())

        try:
            async for message in consumer:
                await self._process_message(message)
                await consumer.commit()
        except asyncio.CancelledError:
            logger.info("Consumer shutting down")
        finally:
            flush_task.cancel()
            await self._flush_batch()  # flush remaining events
            await consumer.stop()

    async def _process_message(self, message):
        """Parse and batch click events"""
        with processing_time.time():
            try:
                event = message.value
                self.batch.append(
                    {
                        "short_code": event.get("short_code", ""),
                        "clicked_at": event.get(
                            "timestamp", datetime.utcnow().isoformat()
                        ),
                        "country": event.get("country", "Unknown"),
                        "referrer": event.get("referrer", ""),
                        "user_agent": event.get("user_agent", ""),
                        "ip_hash": event.get("ip_hash", ""),
                    }
                )

                events_consumed.labels(status="success").inc()

                # Push real-time update via WebSockets
                if self.ws_manager:
                    asyncio.create_task(
                        self.ws_manager.broadcast_to_room(
                            event.get("short_code", ""),
                            {"type": "click", "data": event}
                        )
                    )

                # Flush if batch is full
                if len(self.batch) >= self.batch_size:
                    await self._flush_batch()

            except Exception as e:
                logger.error(f"Failed to process message: {e}")
                events_consumed.labels(status="error").inc()

    async def _periodic_flush(self):
        """Flush batch to ClickHouse every N seconds"""
        while True:
            await asyncio.sleep(self.flush_interval)
            await self._flush_batch()

    async def _flush_batch(self):
        """Bulk insert events into ClickHouse"""
        if not self.batch:
            return

        batch_to_flush = self.batch[:]
        self.batch = []

        try:
            # clickhouse_connect is synchronous, so we use run_in_executor to avoid blocking the event loop
            loop = asyncio.get_event_loop()
            await loop.run_in_executor(
                None,
                lambda: self.ch_client.insert(
                    "url_clicks",
                    batch_to_flush,
                    column_names=[
                        "short_code",
                        "clicked_at",
                        "country",
                        "referrer",
                        "user_agent",
                        "ip_hash",
                    ],
                )
            )
            logger.info(f"Flushed {len(batch_to_flush)} events to ClickHouse")
        except Exception as e:
            logger.exception("Failed to flush to ClickHouse")
            # Re-add to batch for retry
            self.batch = batch_to_flush + self.batch
