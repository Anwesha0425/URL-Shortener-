package cache

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

// RedisCache implements multi-tier caching for URL resolution
// Key design: LRU eviction, 24h TTL, 512MB max memory
type RedisCache struct {
	client *redis.Client
	logger *zap.Logger
}

func NewRedisCache(host, port string, logger *zap.Logger) *RedisCache {
	client := redis.NewClient(&redis.Options{
		Addr:         fmt.Sprintf("%s:%s", host, port),
		PoolSize:     100,            // connection pool size
		MinIdleConns: 10,
		DialTimeout:  2 * time.Second,
		ReadTimeout:  500 * time.Millisecond, // tight timeout for hot path
		WriteTimeout: 500 * time.Millisecond,
	})

	return &RedisCache{client: client, logger: logger}
}

// Get retrieves a URL from cache
// Returns ("", error) on miss or error
func (c *RedisCache) Get(ctx context.Context, shortCode string) (string, error) {
	val, err := c.client.Get(ctx, c.key(shortCode)).Result()
	if err == redis.Nil {
		return "", fmt.Errorf("cache miss")
	}
	return val, err
}

// Set stores a URL in cache with TTL
func (c *RedisCache) Set(ctx context.Context, shortCode, originalURL string, ttl time.Duration) error {
	return c.client.Set(ctx, c.key(shortCode), originalURL, ttl).Err()
}

// Delete removes a URL from cache (called on URL update/delete)
func (c *RedisCache) Delete(ctx context.Context, shortCode string) error {
	return c.client.Del(ctx, c.key(shortCode)).Err()
}

// Ping checks if Redis is alive (used by health checks)
func (c *RedisCache) Ping(ctx context.Context) error {
	return c.client.Ping(ctx).Err()
}

// key returns the Redis key for a short code
// Namespacing prevents key collisions with other apps
func (c *RedisCache) key(shortCode string) string {
	return fmt.Sprintf("url:redirect:%s", shortCode)
}
