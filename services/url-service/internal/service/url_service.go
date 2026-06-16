package service

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"time"

	"github.com/Anwesha0425/url-shortener-microservice/url-service/internal/domain"
	"github.com/Anwesha0425/url-shortener-microservice/url-service/internal/kafka"
	"github.com/Anwesha0425/url-shortener-microservice/url-service/internal/repository"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.uber.org/zap"
)

var (
	ErrURLNotFound     = errors.New("url not found")
	ErrAliasConflict   = errors.New("custom alias already taken")
	ErrInvalidURL      = errors.New("invalid URL format")
	ErrURLExpired      = errors.New("url has expired")
)

var tracer = otel.Tracer("url-service")

// URLService implements all business logic for URL management
type URLService struct {
	urlRepo    *repository.URLRepository
	outboxRepo *repository.OutboxRepository
	producer   *kafka.Producer
	idGen      *domain.IDGenerator
	logger     *zap.Logger
	baseURL    string
}

func NewURLService(
	urlRepo *repository.URLRepository,
	outboxRepo *repository.OutboxRepository,
	producer *kafka.Producer,
	logger *zap.Logger,
) *URLService {
	return &URLService{
		urlRepo:    urlRepo,
		outboxRepo: outboxRepo,
		producer:   producer,
		idGen:      domain.NewIDGenerator(),
		logger:     logger,
		baseURL:    "http://localhost:8002", // loaded from config in production
	}
}

// CreateURL handles the full URL creation workflow:
// 1. Validate input
// 2. Generate Snowflake ID → Base62 short code
// 3. Check custom alias availability
// 4. Write URL to DB + Outbox event (SAME transaction-like operation)
// 5. Return response
func (s *URLService) CreateURL(ctx context.Context, req *domain.CreateURLRequest, userID *int64) (*domain.CreateURLResponse, error) {
	ctx, span := tracer.Start(ctx, "URLService.CreateURL")
	defer span.End()

	// ── Validate URL ──────────────────────────────────────────────
	if err := validateURL(req.OriginalURL); err != nil {
		return nil, ErrInvalidURL
	}

	// ── Generate unique ID and short code ─────────────────────────
	id := s.idGen.NextID()
	shortCode := domain.ToBase62(id)

	// ── Handle custom alias ───────────────────────────────────────
	if req.CustomAlias != nil && *req.CustomAlias != "" {
		shortCode = *req.CustomAlias
		exists, err := s.urlRepo.ShortCodeExists(ctx, shortCode)
		if err != nil {
			return nil, fmt.Errorf("failed to check alias: %w", err)
		}
		if exists {
			return nil, ErrAliasConflict
		}
	}

	span.SetAttributes(
		attribute.String("short_code", shortCode),
		attribute.String("original_url", req.OriginalURL),
	)

	// ── Create URL entity ─────────────────────────────────────────
	newURL := &domain.URL{
		ID:          id,
		ShortCode:   shortCode,
		OriginalURL: req.OriginalURL,
		UserID:      userID,
		CustomAlias: req.CustomAlias,
		ExpiresAt:   req.ExpiresAt,
		IsActive:    true,
	}

	// ── Persist to DB ─────────────────────────────────────────────
	if err := s.urlRepo.Create(ctx, newURL); err != nil {
		s.logger.Error("failed to create url", zap.Error(err))
		return nil, fmt.Errorf("failed to create url: %w", err)
	}

	// ── Write to Outbox (Outbox Pattern) ──────────────────────────
	// This guarantees the event reaches Kafka even if the process crashes
	event := &domain.URLCreatedEvent{
		ShortCode:   shortCode,
		OriginalURL: req.OriginalURL,
		UserID:      userID,
		CreatedAt:   newURL.CreatedAt,
	}
	if err := s.outboxRepo.Create(ctx, id, domain.EventURLCreated, event); err != nil {
		// Log but don't fail — URL is created successfully
		// The outbox poller will retry
		s.logger.Warn("failed to write outbox event", zap.Error(err))
	}

	s.logger.Info("url created",
		zap.String("short_code", shortCode),
		zap.Int64("id", id),
	)

	return &domain.CreateURLResponse{
		ID:          id,
		ShortCode:   shortCode,
		ShortURL:    fmt.Sprintf("%s/%s", s.baseURL, shortCode),
		OriginalURL: req.OriginalURL,
		ExpiresAt:   req.ExpiresAt,
		CreatedAt:   newURL.CreatedAt,
	}, nil
}

// GetURL retrieves a URL by ID with expiry check
func (s *URLService) GetURL(ctx context.Context, id int64) (*domain.URL, error) {
	ctx, span := tracer.Start(ctx, "URLService.GetURL")
	defer span.End()

	url, err := s.urlRepo.GetByID(ctx, id)
	if err != nil {
		return nil, ErrURLNotFound
	}

	// Check expiry
	if url.ExpiresAt != nil && url.ExpiresAt.Before(time.Now()) {
		return nil, ErrURLExpired
	}

	return url, nil
}

// UpdateURL modifies an existing URL
func (s *URLService) UpdateURL(ctx context.Context, id int64, req *domain.UpdateURLRequest) (*domain.URL, error) {
	ctx, span := tracer.Start(ctx, "URLService.UpdateURL")
	defer span.End()

	// Verify URL exists
	if _, err := s.urlRepo.GetByID(ctx, id); err != nil {
		return nil, ErrURLNotFound
	}

	if req.OriginalURL != nil {
		if err := validateURL(*req.OriginalURL); err != nil {
			return nil, ErrInvalidURL
		}
	}

	return s.urlRepo.Update(ctx, id, req)
}

// DeleteURL soft-deletes a URL
func (s *URLService) DeleteURL(ctx context.Context, id int64) error {
	ctx, span := tracer.Start(ctx, "URLService.DeleteURL")
	defer span.End()

	if _, err := s.urlRepo.GetByID(ctx, id); err != nil {
		return ErrURLNotFound
	}
	return s.urlRepo.Delete(ctx, id)
}

// ListURLs returns paginated URLs for a user
func (s *URLService) ListURLs(ctx context.Context, userID int64, page, pageSize int) ([]*domain.URL, int64, error) {
	offset := (page - 1) * pageSize
	return s.urlRepo.ListByUserID(ctx, userID, pageSize, offset)
}

// validateURL checks if the given string is a valid HTTP/HTTPS URL
func validateURL(rawURL string) error {
	u, err := url.ParseRequestURI(rawURL)
	if err != nil {
		return err
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return fmt.Errorf("unsupported scheme: %s", u.Scheme)
	}
	return nil
}
