-- PostgreSQL initialization script
-- Runs when the Docker container starts for the first time

-- Enable UUID extension
CREATE EXTENSION IF NOT EXISTS "pgcrypto";

-- URLs table (primary store)
CREATE TABLE IF NOT EXISTS urls (
    id            BIGINT PRIMARY KEY,
    short_code    VARCHAR(20) UNIQUE NOT NULL,
    original_url  TEXT NOT NULL,
    user_id       BIGINT,
    custom_alias  VARCHAR(50),
    expires_at    TIMESTAMPTZ,
    is_active     BOOLEAN DEFAULT TRUE,
    click_count   BIGINT DEFAULT 0,
    created_at    TIMESTAMPTZ DEFAULT NOW(),
    updated_at    TIMESTAMPTZ DEFAULT NOW()
);

-- Outbox table (Transactional Outbox Pattern)
CREATE TABLE IF NOT EXISTS outbox_events (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    aggregate_id  BIGINT NOT NULL,
    event_type    VARCHAR(50) NOT NULL,
    payload       JSONB NOT NULL,
    processed_at  TIMESTAMPTZ,
    created_at    TIMESTAMPTZ DEFAULT NOW()
);

-- Users table
CREATE TABLE IF NOT EXISTS users (
    id         BIGSERIAL PRIMARY KEY,
    email      VARCHAR(255) UNIQUE NOT NULL,
    api_key    VARCHAR(64) UNIQUE NOT NULL,
    tier       VARCHAR(20) DEFAULT 'free',
    created_at TIMESTAMPTZ DEFAULT NOW()
);

-- Performance indexes
CREATE INDEX IF NOT EXISTS idx_urls_short_code ON urls(short_code);
CREATE INDEX IF NOT EXISTS idx_urls_user_id ON urls(user_id);
CREATE INDEX IF NOT EXISTS idx_urls_expires_at ON urls(expires_at) WHERE expires_at IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_urls_is_active ON urls(is_active) WHERE is_active = true;
CREATE INDEX IF NOT EXISTS idx_outbox_unprocessed ON outbox_events(created_at) WHERE processed_at IS NULL;

-- Seed demo data
INSERT INTO users (email, api_key, tier) VALUES
    ('demo@example.com', 'demo-api-key-12345', 'pro')
ON CONFLICT (email) DO NOTHING;

DO $$ BEGIN RAISE NOTICE 'Database initialized successfully'; END $$;
