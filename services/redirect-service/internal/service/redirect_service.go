package service

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/Anwesha0425/url-shortener-microservice/redirect-service/internal/cache"
	"github.com/Anwesha0425/url-shortener-microservice/redirect-service/internal/circuitbreaker"
	"github.com/Anwesha0425/url-shortener-microservice/redirect-service/internal/kafka"
	"github.com/Anwesha0425/url-shortener-microservice/redirect-service/internal/repository"
	"go.uber.org/zap"
)

var (
	ErrNotFound = errors.New("url not found")
	ErrExpired  = errors.New("url expired")
	ErrInactive = errors.New("url is inactive")
)

// ClickEvent is published to Kafka for analytics processing
type ClickEvent struct {
	ShortCode   string    `json:"short_code"`
	OriginalURL string    `json:"original_url"`
	IPHash      string    `json:"ip_hash"`    // SHA256(ip + salt) — privacy preserving
	UserAgent   string    `json:"user_agent"`
	Referrer    string    `json:"referrer"`
	Country     string    `json:"country"`    // resolved asynchronously
	Timestamp   time.Time `json:"timestamp"`
}

// RedirectService handles the critical redirect path
// Performance contract: < 5ms p99 latency
//
// Resolution order:
//  1. Redis L1 Cache (< 1ms)
//  2. Circuit Breaker check
//  3. PostgreSQL Read Replica (< 10ms)
//  4. Async: publish ClickEvent to Kafka (non-blocking)
type RedirectService struct {
	urlRepo  *repository.URLRepository
	cache    *cache.RedisCache
	cb       *circuitbreaker.CircuitBreaker
	producer *kafka.AsyncProducer
	logger   *zap.Logger
}

func NewRedirectService(
	urlRepo *repository.URLRepository,
	cache *cache.RedisCache,
	cb *circuitbreaker.CircuitBreaker,
	producer *kafka.AsyncProducer,
	logger *zap.Logger,
) *RedirectService {
	return &RedirectService{
		urlRepo:  urlRepo,
		cache:    cache,
		cb:       cb,
		producer: producer,
		logger:   logger,
	}
}

// Resolve returns the original URL for a given short code
func (s *RedirectService) Resolve(ctx context.Context, shortCode string, req RedirectRequest) (string, error) {
	// ── Step 1: Check Redis Cache (L1) ────────────────────────────
	cached, err := s.cache.Get(ctx, shortCode)
	if err == nil && cached != "" {
		// Cache HIT — publish click event async and return
		s.publishClickEventAsync(shortCode, cached, req)
		s.logger.Debug("cache hit", zap.String("code", shortCode))
		return cached, nil
	}

	// ── Step 2: Circuit Breaker Check ─────────────────────────────
	// If DB is unhealthy, circuit opens → return error immediately
	// rather than waiting for timeout (fail fast)
	if !s.cb.Allow() {
		s.logger.Warn("circuit open — DB unavailable", zap.String("code", shortCode))
		return "", fmt.Errorf("service temporarily unavailable")
	}

	// ── Step 3: Database Lookup ───────────────────────────────────
	url, err := s.urlRepo.GetByShortCode(ctx, shortCode)
	if err != nil {
		s.cb.RecordFailure()
		return "", ErrNotFound
	}
	s.cb.RecordSuccess()

	// Check expiry
	if url.ExpiresAt != nil && url.ExpiresAt.Before(time.Now()) {
		return "", ErrExpired
	}
	if !url.IsActive {
		return "", ErrInactive
	}

	// ── Step 4: Populate Cache ────────────────────────────────────
	ttl := 24 * time.Hour
	if url.ExpiresAt != nil {
		ttl = time.Until(*url.ExpiresAt)
	}
	if err := s.cache.Set(ctx, shortCode, url.OriginalURL, ttl); err != nil {
		s.logger.Warn("failed to cache url", zap.Error(err))
	}

	// ── Step 5: Async Click Event (non-blocking) ──────────────────
	s.publishClickEventAsync(shortCode, url.OriginalURL, req)

	return url.OriginalURL, nil
}

// publishClickEventAsync fires the analytics event without blocking the response
func (s *RedirectService) publishClickEventAsync(shortCode, originalURL string, req RedirectRequest) {
	event := ClickEvent{
		ShortCode:   shortCode,
		OriginalURL: originalURL,
		IPHash:      hashIP(req.IP),
		UserAgent:   req.UserAgent,
		Referrer:    req.Referrer,
		Timestamp:   time.Now(),
	}
	s.producer.PublishAsync("url.clicked", shortCode, event)
}

func hashIP(ip string) string {
	// SHA256(ip + salt) for privacy — never store raw IPs
	// Simplified here; use crypto/sha256 in production
	return fmt.Sprintf("hashed_%s", ip)
}

// RedirectRequest carries HTTP request metadata for analytics
type RedirectRequest struct {
	IP        string
	UserAgent string
	Referrer  string
}
