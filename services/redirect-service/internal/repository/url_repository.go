package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"
)

type URLRecord struct {
	ShortCode   string
	OriginalURL string
	IsActive    bool
	ExpiresAt   *time.Time
}

type URLRepository struct {
	db     *pgxpool.Pool
	logger *zap.Logger
}

func NewURLRepository(db *pgxpool.Pool, logger *zap.Logger) *URLRepository {
	return &URLRepository{db: db, logger: logger}
}

// GetByShortCode fetches the original URL for a short code.
// Returns an error if not found or inactive.
func (r *URLRepository) GetByShortCode(ctx context.Context, shortCode string) (*URLRecord, error) {
	var rec URLRecord
	err := r.db.QueryRow(ctx,
		`SELECT short_code, original_url, is_active, expires_at
		 FROM urls
		 WHERE short_code = $1 AND is_active = true`,
		shortCode,
	).Scan(&rec.ShortCode, &rec.OriginalURL, &rec.IsActive, &rec.ExpiresAt)
	if err != nil {
		return nil, fmt.Errorf("url not found: %w", err)
	}

	// Increment click count asynchronously
	go func() {
		_, err := r.db.Exec(context.Background(),
			`UPDATE urls SET click_count = click_count + 1 WHERE short_code = $1`,
			shortCode,
		)
		if err != nil {
			r.logger.Warn("failed to increment click count", zap.Error(err))
		}
	}()

	return &rec, nil
}
