"""
Analytics API Router — Serves analytics queries from ClickHouse
CQRS Read Model: completely separate from the write path
"""

from datetime import datetime, timedelta
from typing import Literal

from fastapi import APIRouter, HTTPException, Query

from app.database import clickhouse_client

router = APIRouter()


@router.get("/{short_code}/summary")
async def get_summary(short_code: str):
    """
    Returns total click count and basic stats for a URL.
    Example: GET /api/v1/analytics/aB3xY/summary
    """
    result = await clickhouse_client.query(f"""
        SELECT
            count()                             AS total_clicks,
            countDistinct(ip_hash)              AS unique_visitors,
            min(clicked_at)                     AS first_click,
            max(clicked_at)                     AS last_click
        FROM url_clicks
        WHERE short_code = '{short_code}'
    """)

    if not result.result_rows:
        raise HTTPException(status_code=404, detail="No analytics found")

    row = result.result_rows[0]
    return {
        "short_code": short_code,
        "total_clicks": row[0],
        "unique_visitors": row[1],
        "first_click": row[2],
        "last_click": row[3],
    }


@router.get("/{short_code}/timeseries")
async def get_timeseries(
    short_code: str,
    granularity: Literal["minute", "hour", "day"] = Query(default="hour"),
    from_date: datetime = Query(default=datetime.utcnow() - timedelta(days=7)),
    to_date: datetime = Query(default=datetime.utcnow()),
):
    """
    Returns click counts over time.
    Example: GET /api/v1/analytics/aB3xY/timeseries?granularity=hour&from=...&to=...
    """
    trunc_fn = {
        "minute": "toStartOfMinute",
        "hour": "toStartOfHour",
        "day": "toStartOfDay",
    }[granularity]

    result = await clickhouse_client.query(f"""
        SELECT
            {trunc_fn}(clicked_at) AS period,
            count()                AS clicks
        FROM url_clicks
        WHERE short_code = '{short_code}'
          AND clicked_at BETWEEN '{from_date.isoformat()}' AND '{to_date.isoformat()}'
        GROUP BY period
        ORDER BY period ASC
    """)

    return {
        "short_code": short_code,
        "granularity": granularity,
        "timeseries": [
            {"period": str(row[0]), "clicks": row[1]} for row in result.result_rows
        ],
    }


@router.get("/{short_code}/geo")
async def get_geo_breakdown(short_code: str):
    """
    Returns click counts grouped by country.
    Example: GET /api/v1/analytics/aB3xY/geo
    """
    result = await clickhouse_client.query(f"""
        SELECT
            country,
            count()                AS clicks,
            countDistinct(ip_hash) AS unique_visitors
        FROM url_clicks
        WHERE short_code = '{short_code}'
        GROUP BY country
        ORDER BY clicks DESC
        LIMIT 50
    """)

    return {
        "short_code": short_code,
        "geo": [
            {"country": row[0], "clicks": row[1], "unique_visitors": row[2]}
            for row in result.result_rows
        ],
    }


@router.get("/{short_code}/referrers")
async def get_referrers(short_code: str):
    """
    Returns top referrer sources.
    Example: GET /api/v1/analytics/aB3xY/referrers
    """
    result = await clickhouse_client.query(f"""
        SELECT
            if(referrer = '', 'Direct', referrer) AS source,
            count()                               AS clicks
        FROM url_clicks
        WHERE short_code = '{short_code}'
        GROUP BY source
        ORDER BY clicks DESC
        LIMIT 20
    """)

    return {
        "short_code": short_code,
        "referrers": [
            {"source": row[0], "clicks": row[1]} for row in result.result_rows
        ],
    }
