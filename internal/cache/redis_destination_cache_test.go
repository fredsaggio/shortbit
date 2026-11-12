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

func TestRedisDestinationCacheIntegration(t *testing.T) {
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

	const ttl = time.Second
	destinations := cache.NewRedisDestinationCache(client, 2*time.Second, ttl)

	t.Run("misses an unknown shortcode", func(t *testing.T) {
		_, _, found, err := destinations.Get(t.Context(), "NotCached1")
		if err != nil || found {
			t.Errorf("Get() found = %v, error = %v; want a miss without error", found, err)
		}
	})

	t.Run("returns a stored destination until the TTL expires", func(t *testing.T) {
		const code = "HotLink123"

		if err := destinations.Set(t.Context(), code, 42, "https://example.com/hot"); err != nil {
			t.Fatalf("Set() error = %v", err)
		}

		urlID, originalURL, found, err := destinations.Get(t.Context(), code)
		if err != nil || !found || urlID != 42 || originalURL != "https://example.com/hot" {
			t.Fatalf("Get() = (%d, %q, %v, %v), want (42, destination, true, nil)", urlID, originalURL, found, err)
		}

		gotTTL, err := client.PTTL(t.Context(), "url:dest:"+code).Result()
		if err != nil {
			t.Fatalf("read TTL: %v", err)
		}
		if gotTTL <= 0 || gotTTL > ttl {
			t.Errorf("TTL = %v, want between 0 and %v", gotTTL, ttl)
		}

		deadline := time.Now().Add(3 * time.Second)
		for {
			_, _, found, err := destinations.Get(t.Context(), code)
			if err != nil {
				t.Fatalf("Get() after TTL error = %v", err)
			}
			if !found {
				break
			}
			if time.Now().After(deadline) {
				t.Fatal("destination did not expire")
			}
			time.Sleep(50 * time.Millisecond)
		}
	})

	t.Run("reports a corrupted entry as an error", func(t *testing.T) {
		const code = "Corrupt123"
		if err := client.Set(t.Context(), "url:dest:"+code, "not json", time.Minute).Err(); err != nil {
			t.Fatalf("store corrupted entry: %v", err)
		}

		if _, _, found, err := destinations.Get(t.Context(), code); err == nil || found {
			t.Errorf("Get() found = %v, error = %v; want an error", found, err)
		}
	})
}
