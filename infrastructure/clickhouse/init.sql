-- ClickHouse initialization
-- Analytics OLAP store for URL click events

CREATE TABLE IF NOT EXISTS url_clicks (
    short_code   String,
    clicked_at   DateTime,
    country      LowCardinality(String),
    referrer     String,
    user_agent   String,
    ip_hash      String
)
ENGINE = MergeTree()
PARTITION BY toYYYYMM(clicked_at)
ORDER BY (short_code, clicked_at)
TTL clicked_at + INTERVAL 2 YEAR;

-- Materialized view for real-time hourly aggregates
CREATE MATERIALIZED VIEW IF NOT EXISTS url_clicks_hourly_mv
ENGINE = SummingMergeTree()
ORDER BY (short_code, hour)
AS SELECT
    short_code,
    toStartOfHour(clicked_at) AS hour,
    count()                   AS clicks
FROM url_clicks
GROUP BY short_code, hour;
