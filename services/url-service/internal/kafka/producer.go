package kafka

import (
	"context"
	"encoding/json"
	"strconv"
	"strings"
	"time"

	"github.com/Anwesha0425/url-shortener-microservice/url-service/internal/domain"
	"github.com/Anwesha0425/url-shortener-microservice/url-service/internal/repository"
	kafkago "github.com/segmentio/kafka-go"
	"go.uber.org/zap"
)

// Producer wraps the kafka-go writer
type Producer struct {
	writer *kafkago.Writer
	logger *zap.Logger
}

func NewProducer(brokers string, logger *zap.Logger) *Producer {
	writer := &kafkago.Writer{
		Addr:         kafkago.TCP(strings.Split(brokers, ",")...),
		Balancer:     &kafkago.LeastBytes{},
		RequiredAcks: kafkago.RequireOne,
		Async:        false, // sync for reliability
	}
	return &Producer{writer: writer, logger: logger}
}

// Publish sends a message to a Kafka topic
func (p *Producer) Publish(ctx context.Context, topic, key string, payload interface{}) error {
	data, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	return p.writer.WriteMessages(ctx, kafkago.Message{
		Topic: topic,
		Key:   []byte(key),
		Value: data,
		Time:  time.Now(),
	})
}

func (p *Producer) Close() error {
	return p.writer.Close()
}

// ─────────────────────────────────────────────────────────────────────────────
// OutboxPoller — polls outbox_events table and publishes unprocessed events
// This is the core of the Transactional Outbox Pattern
// ─────────────────────────────────────────────────────────────────────────────

type OutboxPoller struct {
	outboxRepo *repository.OutboxRepository
	producer   *Producer
	logger     *zap.Logger
	interval   time.Duration
}

func NewOutboxPoller(outboxRepo *repository.OutboxRepository, producer *Producer, logger *zap.Logger) *OutboxPoller {
	return &OutboxPoller{
		outboxRepo: outboxRepo,
		producer:   producer,
		logger:     logger,
		interval:   2 * time.Second,
	}
}

// Start runs the outbox poller in a loop
func (p *OutboxPoller) Start(ctx context.Context) {
	p.logger.Info("outbox poller started")
	ticker := time.NewTicker(p.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			p.logger.Info("outbox poller stopped")
			return
		case <-ticker.C:
			p.processOutbox(ctx)
		}
	}
}

func (p *OutboxPoller) processOutbox(ctx context.Context) {
	events, err := p.outboxRepo.GetUnprocessed(ctx, 100)
	if err != nil {
		p.logger.Error("failed to fetch outbox events", zap.Error(err))
		return
	}

	for _, event := range events {
		topic := eventToTopic(event.EventType)
		if err := p.producer.Publish(ctx, topic, strconv.FormatInt(event.AggregateID, 10), event.Payload); err != nil {
			p.logger.Error("failed to publish event",
				zap.String("id", event.ID),
				zap.String("type", event.EventType),
				zap.Error(err),
			)
			continue
		}

		if err := p.outboxRepo.MarkProcessed(ctx, event.ID); err != nil {
			p.logger.Error("failed to mark event processed", zap.Error(err))
		}

		p.logger.Info("event published",
			zap.String("type", event.EventType),
			zap.String("topic", topic),
		)
	}
}

func eventToTopic(eventType string) string {
	switch eventType {
	case domain.EventURLCreated:
		return "url.created"
	case domain.EventURLUpdated:
		return "url.updated"
	case domain.EventURLDeleted:
		return "url.deleted"
	default:
		return "url.events"
	}
}
