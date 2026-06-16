package kafka

import (
	"context"
	"encoding/json"
	"time"

	kafkago "github.com/segmentio/kafka-go"
	"go.uber.org/zap"
)

type ClickEvent struct {
	ShortCode   string    `json:"short_code"`
	ClickedAt   time.Time `json:"clicked_at"`
}

type AsyncProducer struct {
	writer *kafkago.Writer
	logger *zap.Logger
}

func NewAsyncProducer(brokers []string, logger *zap.Logger) *AsyncProducer {
	return &AsyncProducer{
		writer: &kafkago.Writer{
			Addr:         kafkago.TCP(brokers...),
			Balancer:     &kafkago.LeastBytes{},
			Async:        true, // Non-blocking — fire and forget
			BatchTimeout: 10 * time.Millisecond,
		},
		logger: logger,
	}
}

// PublishAsync sends any JSON-serializable event to the given Kafka topic.
// The key is used for partition affinity (same shortCode → same partition).
// Never blocks the caller — errors are only logged.
func (p *AsyncProducer) PublishAsync(topic, key string, payload any) {
	data, err := json.Marshal(payload)
	if err != nil {
		p.logger.Warn("failed to marshal event", zap.Error(err))
		return
	}

	err = p.writer.WriteMessages(context.Background(), kafkago.Message{
		Topic: topic,
		Key:   []byte(key),
		Value: data,
	})
	if err != nil {
		p.logger.Warn("failed to publish event",
			zap.Error(err),
			zap.String("topic", topic),
			zap.String("key", key),
		)
	}
}

// PublishClick is a convenience wrapper for url.clicked events
func (p *AsyncProducer) PublishClick(shortCode string) {
	p.PublishAsync("url.clicked", shortCode, ClickEvent{
		ShortCode: shortCode,
		ClickedAt: time.Now().UTC(),
	})
}

func (p *AsyncProducer) Close() {
	p.writer.Close()
}
