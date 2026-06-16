"""
Analytics Service — Config
"""
from pydantic_settings import BaseSettings


class Settings(BaseSettings):
    KAFKA_BROKERS:     str = "localhost:9092"
    CLICKHOUSE_HOST:   str = "localhost"
    CLICKHOUSE_PORT:   int = 8123
    CLICKHOUSE_DB:     str = "default"
    PORT:              int = 8003
    JAEGER_ENDPOINT:   str = "http://localhost:4318/v1/traces"
    REDIS_HOST:        str = "localhost"
    REDIS_PORT:        int = 6379

    class Config:
        env_file = ".env"


settings = Settings()
