package repository

import (
	"context"
	"fmt"

	"github.com/Anwesha0425/url-shortener-microservice/url-service/internal/domain"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"
)

// URLRepository handles all URL persistence operations
type URLRepository struct {
	db     *pgxpool.Pool
	logger *zap.Logger
}

func NewURLRepository(db *pgxpool.Pool, logger *zap.Logger) *URLRepository {
	return &URLRepository{db: db, logger: logger}
}

// Create inserts a new URL into the database
func (r *URLRepository) Create(ctx context.Context, url *domain.URL) error {
	query := `
		INSERT INTO urls (id, short_code, original_url, user_id, custom_alias, expires_at, is_active)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING created_at, updated_at
	`
	return r.db.QueryRow(ctx, query,
		url.ID,
		url.ShortCode,
		url.OriginalURL,
		url.UserID,
		url.CustomAlias,
		url.ExpiresAt,
		url.IsActive,
	).Scan(&url.CreatedAt, &url.UpdatedAt)
}

// GetByShortCode fetches a URL by its short code (used by redirect service)
func (r *URLRepository) GetByShortCode(ctx context.Context, shortCode string) (*domain.URL, error) {
	query := `
		SELECT id, short_code, original_url, user_id, custom_alias, expires_at, 
		       is_active, click_count, created_at, updated_at
		FROM urls
		WHERE short_code = $1 AND is_active = true
	`
	url := &domain.URL{}
	err := r.db.QueryRow(ctx, query, shortCode).Scan(
		&url.ID,
		&url.ShortCode,
		&url.OriginalURL,
		&url.UserID,
		&url.CustomAlias,
		&url.ExpiresAt,
		&url.IsActive,
		&url.ClickCount,
		&url.CreatedAt,
		&url.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("url not found: %w", err)
	}
	return url, nil
}

// GetByID fetches a URL by its numeric ID
func (r *URLRepository) GetByID(ctx context.Context, id int64) (*domain.URL, error) {
	query := `
		SELECT id, short_code, original_url, user_id, custom_alias, expires_at,
		       is_active, click_count, created_at, updated_at
		FROM urls
		WHERE id = $1
	`
	url := &domain.URL{}
	err := r.db.QueryRow(ctx, query, id).Scan(
		&url.ID,
		&url.ShortCode,
		&url.OriginalURL,
		&url.UserID,
		&url.CustomAlias,
		&url.ExpiresAt,
		&url.IsActive,
		&url.ClickCount,
		&url.CreatedAt,
		&url.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("url not found: %w", err)
	}
	return url, nil
}

// Update modifies an existing URL
func (r *URLRepository) Update(ctx context.Context, id int64, req *domain.UpdateURLRequest) (*domain.URL, error) {
	query := `
		UPDATE urls
		SET
			original_url = COALESCE($2, original_url),
			is_active    = COALESCE($3, is_active),
			expires_at   = COALESCE($4, expires_at),
			updated_at   = NOW()
		WHERE id = $1
		RETURNING id, short_code, original_url, user_id, custom_alias, expires_at,
		          is_active, click_count, created_at, updated_at
	`
	url := &domain.URL{}
	err := r.db.QueryRow(ctx, query, id, req.OriginalURL, req.IsActive, req.ExpiresAt).Scan(
		&url.ID,
		&url.ShortCode,
		&url.OriginalURL,
		&url.UserID,
		&url.CustomAlias,
		&url.ExpiresAt,
		&url.IsActive,
		&url.ClickCount,
		&url.CreatedAt,
		&url.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("url update failed: %w", err)
	}
	return url, nil
}

// Delete soft-deletes a URL by setting is_active = false
func (r *URLRepository) Delete(ctx context.Context, id int64) error {
	query := `UPDATE urls SET is_active = false, updated_at = NOW() WHERE id = $1`
	_, err := r.db.Exec(ctx, query, id)
	return err
}

// ListByUserID returns paginated URLs for a user
func (r *URLRepository) ListByUserID(ctx context.Context, userID int64, limit, offset int) ([]*domain.URL, int64, error) {
	countQuery := `SELECT COUNT(*) FROM urls WHERE user_id = $1 AND is_active = true`
	var total int64
	if err := r.db.QueryRow(ctx, countQuery, userID).Scan(&total); err != nil {
		return nil, 0, err
	}

	query := `
		SELECT id, short_code, original_url, user_id, custom_alias, expires_at,
		       is_active, click_count, created_at, updated_at
		FROM urls
		WHERE user_id = $1 AND is_active = true
		ORDER BY created_at DESC
		LIMIT $2 OFFSET $3
	`
	rows, err := r.db.Query(ctx, query, userID, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var urls []*domain.URL
	for rows.Next() {
		url := &domain.URL{}
		if err := rows.Scan(
			&url.ID, &url.ShortCode, &url.OriginalURL, &url.UserID,
			&url.CustomAlias, &url.ExpiresAt, &url.IsActive, &url.ClickCount,
			&url.CreatedAt, &url.UpdatedAt,
		); err != nil {
			return nil, 0, err
		}
		urls = append(urls, url)
	}
	return urls, total, nil
}

// ShortCodeExists checks if a custom alias is already taken
func (r *URLRepository) ShortCodeExists(ctx context.Context, shortCode string) (bool, error) {
	var exists bool
	err := r.db.QueryRow(ctx,
		`SELECT EXISTS(SELECT 1 FROM urls WHERE short_code = $1)`,
		shortCode,
	).Scan(&exists)
	return exists, err
}
