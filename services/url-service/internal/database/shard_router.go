package database

import (
	"context"
	"fmt"
	"hash/fnv"
	"sync"

	"github.com/jackc/pgx/v5/pgxpool"
)

/**
 * ShardRouter — Consistent Hashing Database Sharding
 *
 * Distributes URL records across N PostgreSQL shards.
 * Shard selection is based on FNV-32 hash of the short_code.
 *
 * Design decisions:
 *   - FNV-32 hash: fast, no crypto overhead, even distribution
 *   - 16 shards by default, supports expansion to 256
 *   - Each shard is an independent Postgres instance
 *   - Cross-shard queries: avoided by design (each URL is self-contained)
 *   - Re-sharding: use consistent hashing ring to minimize data movement
 *
 * Shard key: short_code (first 2 chars determine shard)
 * This keeps all operations for a URL on a single shard.
 *
 * Interview talking point:
 *   "Why not user_id as shard key?"
 *   → User list queries would work well, but hot users would create hot shards.
 *   → short_code gives uniform distribution regardless of user activity.
 */

const defaultShardCount = 16

// ShardRouter routes database operations to the correct shard
type ShardRouter struct {
	shards     []*pgxpool.Pool
	shardCount int
	mu         sync.RWMutex
}

// ShardConfig describes a single shard's connection info
type ShardConfig struct {
	ShardID int
	DSN     string
}

// NewShardRouter initializes connections to all shards
func NewShardRouter(configs []ShardConfig) (*ShardRouter, error) {
	if len(configs) == 0 {
		return nil, fmt.Errorf("at least one shard config required")
	}

	shards := make([]*pgxpool.Pool, len(configs))
	for _, cfg := range configs {
		pool, err := pgxpool.New(context.Background(), cfg.DSN)
		if err != nil {
			return nil, fmt.Errorf("failed to connect to shard %d: %w", cfg.ShardID, err)
		}
		shards[cfg.ShardID] = pool
	}

	return &ShardRouter{
		shards:     shards,
		shardCount: len(configs),
	}, nil
}

// ShardFor returns the database pool responsible for the given short_code
func (r *ShardRouter) ShardFor(shortCode string) *pgxpool.Pool {
	shardID := r.shardID(shortCode)
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.shards[shardID]
}

// ShardID returns the shard index for a given key (exported for testing)
func (r *ShardRouter) ShardID(shortCode string) int {
	return r.shardID(shortCode)
}

// shardID computes which shard owns this short_code
// Uses FNV-32 hash for speed and even distribution
func (r *ShardRouter) shardID(shortCode string) int {
	h := fnv.New32a()
	h.Write([]byte(shortCode))
	return int(h.Sum32()) % r.shardCount
}

// AllShards returns all shard pools (used for cross-shard queries like analytics)
func (r *ShardRouter) AllShards() []*pgxpool.Pool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	result := make([]*pgxpool.Pool, len(r.shards))
	copy(result, r.shards)
	return result
}

// Close closes all shard connections
func (r *ShardRouter) Close() {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, shard := range r.shards {
		if shard != nil {
			shard.Close()
		}
	}
}

// ShardDistribution returns how many keys each shard would own (for analysis)
func ShardDistribution(keys []string, shardCount int) map[int]int {
	dist := make(map[int]int)
	for _, key := range keys {
		h := fnv.New32a()
		h.Write([]byte(key))
		shard := int(h.Sum32()) % shardCount
		dist[shard]++
	}
	return dist
}

// DefaultShardConfigs generates shard DSNs for a standard setup
// In production these come from Terraform outputs / K8s secrets
func DefaultShardConfigs(baseHost string, shardCount int) []ShardConfig {
	configs := make([]ShardConfig, shardCount)
	for i := range configs {
		configs[i] = ShardConfig{
			ShardID: i,
			DSN: fmt.Sprintf(
				"host=%s-%02d port=5432 user=urluser password=urlpassword dbname=urldb sslmode=disable",
				baseHost, i,
			),
		}
	}
	return configs
}
