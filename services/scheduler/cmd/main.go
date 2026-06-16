package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	kafkago "github.com/segmentio/kafka-go"
	"go.uber.org/zap"
)

/**
 * URL Expiry Scheduler
 *
 * Runs as a separate lightweight pod in Kubernetes (CronJob).
 * Responsibilities:
 *   1. Scan for URLs past their expires_at timestamp
 *   2. Soft-delete them (is_active = false)
 *   3. Publish url.expired events to Kafka (triggers notifications)
 *   4. Invalidate Redis cache for expired URLs
 *
 * Schedule: Every 5 minutes (configured in K8s CronJob)
 *
 * Design: Process in batches of 500 to avoid long-running transactions.
 * Uses cursor-based pagination (by ID) for consistent batch boundaries.
 */

type ExpiredURL struct {
	ID        int64  `json:"id"`
	ShortCode string `json:"short_code"`
	UserID    *int64 `json:"user_id"`
}

func main() {
	logger, _ := zap.NewProduction()
	defer logger.Sync()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	go func() { <-quit; cancel() }()

	// DB connection
	db, err := pgxpool.New(ctx, dbDSN())
	if err != nil {
		log.Fatalf("DB connect failed: %v", err)
	}
	defer db.Close()

	// Kafka writer
	writer := &kafkago.Writer{
		Addr:     kafkago.TCP(getEnv("KAFKA_BROKERS", "localhost:9092")),
		Balancer: &kafkago.LeastBytes{},
	}
	defer writer.Close()

	logger.Info("URL Expiry Scheduler starting")

	if err := runExpiryCycle(ctx, db, writer, logger); err != nil {
		logger.Error("Expiry cycle failed", zap.Error(err))
		os.Exit(1)
	}

	logger.Info("Expiry cycle complete — exiting")
}

func runExpiryCycle(ctx context.Context, db *pgxpool.Pool, writer *kafkago.Writer, logger *zap.Logger) error {
	batchSize := 500
	processed := 0
	var lastID int64 = 0

	for {
		// ── Fetch batch of expired URLs ────────────────────────────
		rows, err := db.Query(ctx, `
			SELECT id, short_code, user_id
			FROM urls
			WHERE is_active   = true
			  AND expires_at  < NOW()
			  AND id          > $1
			ORDER BY id ASC
			LIMIT $2
		`, lastID, batchSize)
		if err != nil {
			return fmt.Errorf("query failed: %w", err)
		}

		var batch []ExpiredURL
		for rows.Next() {
			var u ExpiredURL
			if err := rows.Scan(&u.ID, &u.ShortCode, &u.UserID); err != nil {
				return err
			}
			batch = append(batch, u)
			if u.ID > lastID {
				lastID = u.ID
			}
		}
		rows.Close()

		if len(batch) == 0 {
			break // No more expired URLs
		}

		// ── Soft-delete batch ──────────────────────────────────────
		ids := make([]int64, len(batch))
		for i, u := range batch {
			ids[i] = u.ID
		}

		_, err = db.Exec(ctx, `
			UPDATE urls
			SET is_active  = false,
			    updated_at = NOW()
			WHERE id = ANY($1)
		`, ids)
		if err != nil {
			return fmt.Errorf("update failed: %w", err)
		}

		// ── Publish url.expired events to Kafka ───────────────────
		var messages []kafkago.Message
		for _, u := range batch {
			payload, _ := json.Marshal(map[string]interface{}{
				"short_code": u.ShortCode,
				"user_id":    u.UserID,
				"expired_at": time.Now().UTC().Format(time.RFC3339),
			})
			messages = append(messages, kafkago.Message{
				Topic: "url.expired",
				Key:   []byte(u.ShortCode),
				Value: payload,
			})
		}

		if err := writer.WriteMessages(ctx, messages...); err != nil {
			// Log but don't fail — URLs are already marked inactive
			logger.Warn("Failed to publish expiry events", zap.Error(err))
		}

		processed += len(batch)
		logger.Info("Batch processed",
			zap.Int("batch_size", len(batch)),
			zap.Int("total_processed", processed),
		)

		if len(batch) < batchSize {
			break // Last batch
		}
	}

	logger.Info("Expiry cycle complete", zap.Int("total_expired", processed))
	return nil
}

func dbDSN() string {
	return fmt.Sprintf(
		"host=%s port=%s user=%s password=%s dbname=%s sslmode=disable",
		getEnv("DB_HOST", "localhost"),
		getEnv("DB_PORT", "5432"),
		getEnv("DB_USER", "urluser"),
		getEnv("DB_PASSWORD", "urlpassword"),
		getEnv("DB_NAME", "urldb"),
	)
}

func getEnv(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
