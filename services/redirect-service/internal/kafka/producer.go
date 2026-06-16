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
			Topic:        "url.clicked",
			Balancer:     &kafkago.LeastBytes{},
			Async:        true, // Non-blocking — fire and forget
			BatchTimeout: 10 * time.Millisecond,
		},
		logger: logger,
	}
}

// PublishClick sends a click event to Kafka asynchronously.
// Errors are logged but never block the redirect response.
func (p *AsyncProducer) PublishClick(shortCode string) {
	payload, _ := json.Marshal(ClickEvent{
		ShortCode: shortCode,
		ClickedAt: time.Now().UTC(),
	})

	err := p.writer.WriteMessages(context.Background(), kafkago.Message{
		Key:   []byte(shortCode),
		Value: payload,
	})
	if err != nil {
		p.logger.Warn("failed to publish click event", zap.Error(err), zap.String("short_code", shortCode))
	}
}

func (p *AsyncProducer) Close() {
	p.writer.Close()
}
