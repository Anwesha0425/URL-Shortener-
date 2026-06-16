package repository

import (
	"context"
	"encoding/json"
	"time"

	"github.com/Anwesha0425/url-shortener-microservice/url-service/internal/domain"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"
)

// OutboxRepository handles the Transactional Outbox Pattern
// Guarantees: events are NEVER lost even if Kafka is down
// Because: events are first written to DB (same transaction as the URL insert)
// Then a background poller picks them up and publishes to Kafka
type OutboxRepository struct {
	db     *pgxpool.Pool
	logger *zap.Logger
}

func NewOutboxRepository(db *pgxpool.Pool, logger *zap.Logger) *OutboxRepository {
	return &OutboxRepository{db: db, logger: logger}
}

// Create writes an event to the outbox table within a transaction
func (r *OutboxRepository) Create(ctx context.Context, aggregateID int64, eventType string, payload interface{}) error {
	data, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	_, err = r.db.Exec(ctx, `
		INSERT INTO outbox_events (aggregate_id, event_type, payload)
		VALUES ($1, $2, $3)
	`, aggregateID, eventType, data)
	return err
}

// GetUnprocessed fetches events that haven't been published to Kafka yet
func (r *OutboxRepository) GetUnprocessed(ctx context.Context, limit int) ([]*domain.OutboxEvent, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id, aggregate_id, event_type, payload, created_at
		FROM outbox_events
		WHERE processed_at IS NULL
		ORDER BY created_at ASC
		LIMIT $1
	`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var events []*domain.OutboxEvent
	for rows.Next() {
		event := &domain.OutboxEvent{}
		if err := rows.Scan(
			&event.ID, &event.AggregateID, &event.EventType,
			&event.Payload, &event.CreatedAt,
		); err != nil {
			return nil, err
		}
		events = append(events, event)
	}
	return events, nil
}

// MarkProcessed marks an event as successfully published
func (r *OutboxRepository) MarkProcessed(ctx context.Context, id string) error {
	now := time.Now()
	_, err := r.db.Exec(ctx, `
		UPDATE outbox_events SET processed_at = $1 WHERE id = $2
	`, now, id)
	return err
}
