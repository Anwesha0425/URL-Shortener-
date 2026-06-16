package domain

import "time"

// URL is the core domain entity
type URL struct {
	ID          int64      `json:"id"`
	ShortCode   string     `json:"short_code"`
	OriginalURL string     `json:"original_url"`
	UserID      *int64     `json:"user_id,omitempty"`
	CustomAlias *string    `json:"custom_alias,omitempty"`
	ExpiresAt   *time.Time `json:"expires_at,omitempty"`
	IsActive    bool       `json:"is_active"`
	ClickCount  int64      `json:"click_count"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
}

// CreateURLRequest is the input for creating a short URL
type CreateURLRequest struct {
	OriginalURL string     `json:"original_url" binding:"required,url"`
	CustomAlias *string    `json:"custom_alias,omitempty"`
	ExpiresAt   *time.Time `json:"expires_at,omitempty"`
	Tags        []string   `json:"tags,omitempty"`
}

// CreateURLResponse is the output after creating a short URL
type CreateURLResponse struct {
	ID          int64      `json:"id"`
	ShortCode   string     `json:"short_code"`
	ShortURL    string     `json:"short_url"`
	OriginalURL string     `json:"original_url"`
	ExpiresAt   *time.Time `json:"expires_at,omitempty"`
	CreatedAt   time.Time  `json:"created_at"`
}

// UpdateURLRequest is the input for updating a URL
type UpdateURLRequest struct {
	OriginalURL *string    `json:"original_url,omitempty" binding:"omitempty,url"`
	IsActive    *bool      `json:"is_active,omitempty"`
	ExpiresAt   *time.Time `json:"expires_at,omitempty"`
}

// OutboxEvent represents an event to be published to Kafka
type OutboxEvent struct {
	ID          string    `json:"id"`
	AggregateID int64     `json:"aggregate_id"`
	EventType   string    `json:"event_type"`
	Payload     []byte    `json:"payload"`
	ProcessedAt *time.Time `json:"processed_at,omitempty"`
	CreatedAt   time.Time  `json:"created_at"`
}

// Event types
const (
	EventURLCreated = "url.created"
	EventURLUpdated = "url.updated"
	EventURLDeleted = "url.deleted"
)

// URLCreatedEvent is published when a new URL is created
type URLCreatedEvent struct {
	ShortCode   string    `json:"short_code"`
	OriginalURL string    `json:"original_url"`
	UserID      *int64    `json:"user_id,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
}
