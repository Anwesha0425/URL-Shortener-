import { Pool } from 'pg';
import { logger } from '../utils/logger';

/**
 * runMigrations — ensures all required tables exist.
 * Called at startup before the server accepts traffic.
 *
 * In production, use a dedicated migration tool (e.g. Flyway, golang-migrate).
 * This lightweight version is sufficient for Docker Compose local development.
 */
export async function runMigrations(db: Pool): Promise<void> {
  await db.query(`
    -- Auth-specific tables (urls/outbox tables are in postgres/init.sql)

    -- Ensure users table has password_hash column
    ALTER TABLE users ADD COLUMN IF NOT EXISTS password_hash VARCHAR(255);

    -- Refresh tokens blacklist (backed by Redis, this is a fallback)
    CREATE TABLE IF NOT EXISTS refresh_tokens (
      id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
      user_id    BIGINT NOT NULL,
      token_hash VARCHAR(255) UNIQUE NOT NULL,
      expires_at TIMESTAMPTZ NOT NULL,
      revoked    BOOLEAN DEFAULT FALSE,
      created_at TIMESTAMPTZ DEFAULT NOW()
    );

    CREATE INDEX IF NOT EXISTS idx_refresh_tokens_user_id
      ON refresh_tokens(user_id);
    CREATE INDEX IF NOT EXISTS idx_refresh_tokens_hash
      ON refresh_tokens(token_hash) WHERE revoked = FALSE;
  `);

  logger.info('Auth service migrations complete');
}
