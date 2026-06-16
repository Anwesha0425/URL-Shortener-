"""
ClickHouse client singleton for analytics queries
"""

import clickhouse_connect
from app.config import settings

clickhouse_client = clickhouse_connect.get_client(
    host=settings.CLICKHOUSE_HOST,
    port=settings.CLICKHOUSE_PORT,
    database=settings.CLICKHOUSE_DB,
)
