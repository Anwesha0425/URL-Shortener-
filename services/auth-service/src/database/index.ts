import { Pool } from 'pg';
import { config } from '../config';

export const db = new Pool({
  host:     config.DB_HOST,
  port:     config.DB_PORT,
  user:     config.DB_USER,
  password: config.DB_PASSWORD,
  database: config.DB_NAME,
  max:      20,
  idleTimeoutMillis:    30000,
  connectionTimeoutMillis: 5000,
});

export async function runMigrations(pool: Pool) {
  await pool.query(`
    CREATE TABLE IF NOT EXISTS users (
      id            BIGSERIAL PRIMARY KEY,
      email         VARCHAR(255) UNIQUE NOT NULL,
      password_hash VARCHAR(255) NOT NULL,
      api_key       VARCHAR(64) UNIQUE,
      tier          VARCHAR(20) DEFAULT 'free',
      created_at    TIMESTAMPTZ DEFAULT NOW()
    );

    CREATE TABLE IF NOT EXISTS api_keys (
      id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
      user_id      BIGINT REFERENCES users(id) ON DELETE CASCADE,
      key_hash     VARCHAR(255) NOT NULL,
      name         VARCHAR(100) NOT NULL,
      is_active    BOOLEAN DEFAULT TRUE,
      last_used_at TIMESTAMPTZ,
      created_at   TIMESTAMPTZ DEFAULT NOW()
    );

    CREATE INDEX IF NOT EXISTS idx_users_email   ON users(email);
    CREATE INDEX IF NOT EXISTS idx_api_keys_user ON api_keys(user_id);
  `);
}
