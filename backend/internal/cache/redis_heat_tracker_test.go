//go:build integration

package cache_test

import (
	"net"
	"testing"
	"time"

	"github.com/fredsaggio/url-shortener/internal/cache"
	"github.com/redis/go-redis/v9"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

func TestRedisHeatTrackerRecordIntegration(t *testing.T) {
	ctx := t.Context()

	container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: testcontainers.ContainerRequest{
			Image:        "redis:8-alpine",
			ExposedPorts: []string{"6379/tcp"},
			WaitingFor:   wait.ForListeningPort("6379/tcp"),
		},
		Started: true,
	})
	testcontainers.CleanupContainer(t, container)
	if err != nil {
		t.Fatalf("start Redis container: %v", err)
	}

	host, err := container.Host(ctx)
	if err != nil {
		t.Fatalf("get Redis host: %v", err)
	}

	port, err := container.MappedPort(ctx, "6379/tcp")
	if err != nil {
		t.Fatalf("get Redis port: %v", err)
	}

	client := redis.NewClient(&redis.Options{
		Addr:       net.JoinHostPort(host, port.Port()),
		MaxRetries: -1,
	})
	t.Cleanup(func() { _ = client.Close() })

	if err := client.Ping(ctx).Err(); err != nil {
		t.Fatalf("ping Redis: %v", err)
	}

	const (
		code   = "TesteHeat123"
		window = time.Second
	)
	key := "url:heat:" + code
	tracker := cache.NewRedisHeatTracker(client, 2*time.Second, window)

	first, err := tracker.Record(ctx, code)
	if err != nil {
		t.Fatalf("first Record(): %v", err)
	}
	if first != 1 {
		t.Fatalf("first Record() = %d, want 1", first)
	}

	firstTTL, err := client.PTTL(ctx, key).Result()
	if err != nil {
		t.Fatalf("read first TTL: %v", err)
	}
	if firstTTL <= 0 || firstTTL > window {
		t.Fatalf("first TTL = %v, want between 0 and %v", firstTTL, window)
	}

	time.Sleep(150 * time.Millisecond)

	second, err := tracker.Record(ctx, code)
	if err != nil {
		t.Fatalf("second Record(): %v", err)
	}
	if second != 2 {
		t.Fatalf("second Record() = %d, want 2", second)
	}

	secondTTL, err := client.PTTL(ctx, key).Result()
	if err != nil {
		t.Fatalf("read second TTL: %v", err)
	}
	if secondTTL >= firstTTL-75*time.Millisecond {
		t.Errorf("second TTL = %v; first TTL = %v: second access appears to have renewed expiration", secondTTL, firstTTL)
	}

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		exists, err := client.Exists(ctx, key).Result()
		if err != nil {
			t.Fatalf("check counter expiration: %v", err)
		}
		if exists == 0 {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}

	third, err := tracker.Record(ctx, code)
	if err != nil {
		t.Fatalf("Record() after expiration: %v", err)
	}
	if third != 1 {
		t.Errorf("Record() after expiration = %d, want 1", third)
	}
}
