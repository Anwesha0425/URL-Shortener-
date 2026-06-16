package bloomfilter

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"
)

/**
 * BloomLoader — populates the Bloom Filter from PostgreSQL at startup.
 *
 * Strategy:
 *   1. Count total active URLs → size the filter optimally
 *   2. Stream all short_codes via cursor (avoid loading 100M rows into RAM)
 *   3. After startup, new URLs are added to the filter in real-time
 *      (url-service publishes to Kafka → redirect-service consumer adds to filter)
 *
 * Streaming cursor: we use PostgreSQL DECLARE CURSOR to avoid loading all
 * 100M short_codes into memory at once. Fetched in batches of 10,000.
 */

const (
	defaultFPR      = 0.01     // 1% false positive rate
	cursorBatchSize = 10_000
)

// LoadFromDB populates a BloomFilter with all active short_codes from PostgreSQL.
// Returns the populated filter and the count of URLs loaded.
func LoadFromDB(ctx context.Context, db *pgxpool.Pool, logger *zap.Logger) (*BloomFilter, int64, error) {
	// Step 1: Count rows to size the filter optimally
	var count int64
	err := db.QueryRow(ctx, "SELECT COUNT(*) FROM urls WHERE is_active = true").Scan(&count)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to count URLs: %w", err)
	}

	if count == 0 {
		logger.Info("Bloom filter: no active URLs — starting empty")
		return NewBloomFilter(1_000_000, defaultFPR), 0, nil
	}

	// Add 20% headroom for new URLs written between startup and first Kafka event
	filter := NewBloomFilter(uint64(float64(count)*1.2), defaultFPR)
	logger.Info("Bloom filter: sizing filter", zap.Int64("url_count", count))

	// Step 2: Stream short_codes via cursor
	tx, err := db.Begin(ctx)
	if err != nil {
		return nil, 0, err
	}
	defer tx.Rollback(ctx)

	_, err = tx.Exec(ctx, "DECLARE url_cursor CURSOR FOR SELECT short_code FROM urls WHERE is_active = true")
	if err != nil {
		return nil, 0, fmt.Errorf("failed to declare cursor: %w", err)
	}

	loaded := int64(0)
	for {
		rows, err := tx.Query(ctx, fmt.Sprintf("FETCH %d FROM url_cursor", cursorBatchSize))
		if err != nil {
			return nil, 0, fmt.Errorf("cursor fetch failed: %w", err)
		}

		batch := 0
		for rows.Next() {
			var shortCode string
			if err := rows.Scan(&shortCode); err != nil {
				rows.Close()
				return nil, 0, err
			}
			filter.Add(shortCode)
			batch++
		}
		rows.Close()

		loaded += int64(batch)
		if batch < cursorBatchSize {
			break // Last batch — we're done
		}

		if loaded%100_000 == 0 {
			logger.Info("Bloom filter loading...", zap.Int64("loaded", loaded), zap.Int64("total", count))
		}
	}

	logger.Info("Bloom filter populated",
		zap.Int64("loaded", loaded),
		zap.Float64("fpr", filter.FalsePositiveRate()),
	)

	return filter, loaded, nil
}
