package main

import (
	"testing"
	"time"

	"github.com/fredsaggio/url-shortener/internal/config"
)

func TestNewRedisOptions(t *testing.T) {
	t.Run("uses the plain address locally", func(t *testing.T) {
		options, err := newRedisOptions(config.RedisConfig{Addr: "127.0.0.1:6379", Timeout: time.Second})
		if err != nil {
			t.Fatalf("newRedisOptions() error = %v", err)
		}
		if options.Addr != "127.0.0.1:6379" || options.TLSConfig != nil || options.Password != "" {
			t.Errorf("options = addr %q, TLS %v, password set %v; want plain local address", options.Addr, options.TLSConfig != nil, options.Password != "")
		}
		if !options.ContextTimeoutEnabled || options.MaxRetries != -1 {
			t.Errorf("ContextTimeoutEnabled = %v, MaxRetries = %d; want true, -1", options.ContextTimeoutEnabled, options.MaxRetries)
		}
	})

	t.Run("uses TLS and the password from a rediss URL", func(t *testing.T) {
		options, err := newRedisOptions(config.RedisConfig{URL: "rediss://default:secret@redis.example:6379", Addr: "127.0.0.1:6379"})
		if err != nil {
			t.Fatalf("newRedisOptions() error = %v", err)
		}
		if options.Addr != "redis.example:6379" {
			t.Errorf("Addr = %q, want redis.example:6379", options.Addr)
		}
		if options.TLSConfig == nil {
			t.Error("TLSConfig = nil, want TLS for rediss://")
		}
		if options.Password != "secret" {
			t.Error("Password not taken from the URL")
		}
		if !options.ContextTimeoutEnabled || options.MaxRetries != -1 {
			t.Errorf("ContextTimeoutEnabled = %v, MaxRetries = %d; want true, -1", options.ContextTimeoutEnabled, options.MaxRetries)
		}
	})
}
