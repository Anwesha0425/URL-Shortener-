package database

import (
	"context"
	"fmt"

	"github.com/Anwesha0425/url-shortener-microservice/url-service/internal/config"
	"github.com/jackc/pgx/v5/pgxpool"
)

// NewPostgres creates a new PostgreSQL connection pool
func NewPostgres(cfg *config.Config) (*pgxpool.Pool, error) {
	dsn := fmt.Sprintf(
		"host=%s port=%s user=%s password=%s dbname=%s sslmode=disable",
		cfg.DBHost, cfg.DBPort, cfg.DBUser, cfg.DBPassword, cfg.DBName,
	)

	pool, err := pgxpool.New(context.Background(), dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to create db pool: %w", err)
	}

	// Verify connection
	if err := pool.Ping(context.Background()); err != nil {
		return nil, fmt.Errorf("failed to ping db: %w", err)
	}

	return pool, nil
}

// RunMigrations runs database migrations (simplified inline version)
func RunMigrations(db *pgxpool.Pool) error {
	migrations := []string{
		// URLs table
		`CREATE TABLE IF NOT EXISTS urls (
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
		)`,

		// Outbox table — heart of the Outbox Pattern
		`CREATE TABLE IF NOT EXISTS outbox_events (
			id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			aggregate_id  BIGINT NOT NULL,
			event_type    VARCHAR(50) NOT NULL,
			payload       JSONB NOT NULL,
			processed_at  TIMESTAMPTZ,
			created_at    TIMESTAMPTZ DEFAULT NOW()
		)`,

		// Users table (simplified)
		`CREATE TABLE IF NOT EXISTS users (
			id         BIGSERIAL PRIMARY KEY,
			email      VARCHAR(255) UNIQUE NOT NULL,
			api_key    VARCHAR(64) UNIQUE NOT NULL,
			tier       VARCHAR(20) DEFAULT 'free',
			created_at TIMESTAMPTZ DEFAULT NOW()
		)`,

		// Indexes for performance
		`CREATE INDEX IF NOT EXISTS idx_urls_short_code ON urls(short_code)`,
		`CREATE INDEX IF NOT EXISTS idx_urls_user_id ON urls(user_id)`,
		`CREATE INDEX IF NOT EXISTS idx_urls_expires_at ON urls(expires_at) WHERE expires_at IS NOT NULL`,
		`CREATE INDEX IF NOT EXISTS idx_outbox_unprocessed ON outbox_events(created_at) WHERE processed_at IS NULL`,
	}

	for _, migration := range migrations {
		if _, err := db.Exec(context.Background(), migration); err != nil {
			return fmt.Errorf("migration failed: %w\nSQL: %s", err, migration)
		}
	}
	return nil
}
