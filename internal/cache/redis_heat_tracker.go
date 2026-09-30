package cache

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

var incrementHeatScript = redis.NewScript(
	`local count = redis.call("INCR", KEYS[1])
	if count == 1 then
		redis.call("PEXPIRE", KEYS[1], ARGV[1])
	end
	return count
	`)

type RedisHeatTracker struct {
	client  *redis.Client
	timeout time.Duration
	window  time.Duration
}

func NewRedisHeatTracker(client *redis.Client, timeout time.Duration, window time.Duration) *RedisHeatTracker {
	return &RedisHeatTracker{client: client, timeout: timeout, window: window}
}

func (t *RedisHeatTracker) Record(ctx context.Context, shortCode string) (int64, error) {
	opCtx, cancel := context.WithTimeout(ctx, t.timeout)
	defer cancel()

	key := "url:heat:" + shortCode

	count, err := incrementHeatScript.Run(opCtx, t.client, []string{key}, time.Minute.Milliseconds()).Int64()

	if err != nil {
		return 0, fmt.Errorf("record URL heat: %w", err)
	}

	return count, nil
}
