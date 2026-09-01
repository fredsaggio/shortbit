package config

import (
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"
)

const (
	defaultHTTPAddr          = ":8080"
	defaultReadHeaderTimeout = 5 * time.Second
	defaultReadTimeout       = 10 * time.Second
	defaultWriteTimeout      = 15 * time.Second
	defaultIdleTimeout       = 60 * time.Second
	defaultShutdownTimeout   = 10 * time.Second
)

type Config struct {
	DatabaseURL string
	HTTP    HTTPConfig
	Log         LogConfig
}


type HTTPConfig struct {
	Addr string
	ReadHeaderTimeout time.Duration
	ReadTimeout time.Duration
	WriteTimeout time.Duration
	IdleTimeout time.Duration
	ShutdownTimeout time.Duration
}

type LogConfig struct {
	Level  slog.Level
	Format string
}

func Load(getEnv func(string) string) (Config, error) {
	cfg := Config{
		DatabaseURL: getEnv("DATABASE_URL"),
		HTTP: HTTPConfig{
			Addr: envOrDefault(getEnv, "HTTP_ADDR", ":8080"),
			ReadHeaderTimeout: defaultReadHeaderTimeout,
  			ReadTimeout:       defaultReadTimeout,
  			WriteTimeout:      defaultWriteTimeout,
  			IdleTimeout:       defaultIdleTimeout,
  			ShutdownTimeout:   defaultShutdownTimeout,
		},
		Log: LogConfig{
			Format: strings.ToLower(envOrDefault(getEnv, "LOG_FORMAT", "json")),
		},
	}

	if cfg.DatabaseURL == "" {
		return Config{}, errors.New("DATABASE_URL is required")
	}

	if err := cfg.Log.Level.UnmarshalText(
		[]byte(envOrDefault(getEnv, "LOG_LEVEL", "info")),
	); err != nil {
		return Config{}, fmt.Errorf("invalid LOG_LEVEL: %w", err)
	}

	if cfg.Log.Format != "json" && cfg.Log.Format != "text" {
		return Config{}, fmt.Errorf("invalid LOG_FORMAT %q: expected json or text", cfg.Log.Format)
	}

	return cfg, nil
}

func envOrDefault(getEnv func(string) string, key string, defaultValue string) string {
	value := getEnv(key)

	if value == "" {
		return defaultValue
	}

	return value
}
