package cache

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

// Only resolution data is cached: never the password hash, tokens, or the
// click count.
type cachedDestination struct {
	URLID       int64  `json:"url_id"`
	OriginalURL string `json:"original_url"`
}

type RedisDestinationCache struct {
	client  *redis.Client
	timeout time.Duration
	ttl     time.Duration
}

func NewRedisDestinationCache(client *redis.Client, timeout time.Duration, ttl time.Duration) *RedisDestinationCache {
	return &RedisDestinationCache{client: client, timeout: timeout, ttl: ttl}
}

func destinationKey(shortCode string) string {
	return "url:dest:" + shortCode
}

func (c *RedisDestinationCache) Get(ctx context.Context, shortCode string) (int64, string, bool, error) {
	opCtx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	value, err := c.client.Get(opCtx, destinationKey(shortCode)).Bytes()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return 0, "", false, nil
		}
		return 0, "", false, fmt.Errorf("get cached URL destination: %w", err)
	}

	var destination cachedDestination
	if err := json.Unmarshal(value, &destination); err != nil {
		return 0, "", false, fmt.Errorf("decode cached URL destination: %w", err)
	}

	return destination.URLID, destination.OriginalURL, true, nil
}

func (c *RedisDestinationCache) Set(ctx context.Context, shortCode string, urlID int64, originalURL string) error {
	opCtx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	value, err := json.Marshal(cachedDestination{URLID: urlID, OriginalURL: originalURL})
	if err != nil {
		return fmt.Errorf("encode cached URL destination: %w", err)
	}

	if err := c.client.Set(opCtx, destinationKey(shortCode), value, c.ttl).Err(); err != nil {
		return fmt.Errorf("set cached URL destination: %w", err)
	}

	return nil
}
