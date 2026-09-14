package config

import (
	"errors"
	"fmt"
	"strconv"
	"time"
)

const (
	defaultHTTPAddr          = ":8080"
	defaultReadHeaderTimeout = 5 * time.Second
	defaultReadTimeout       = 10 * time.Second
	defaultWriteTimeout      = 15 * time.Second
	defaultIdleTimeout       = 60 * time.Second
	defaultShutdownTimeout   = 10 * time.Second
	defaultUserSessionTTL    = 72 * time.Hour
	defaultCookieSecure      = true
)

type Config struct {
	DatabaseURL string
	HTTP        HTTPConfig
	Session     SessionConfig
	Google      GoogleConfig
}

type SessionConfig struct {
	TTL          time.Duration
	CookieSecure bool
}

type GoogleConfig struct {
	ClientID     string
	ClientSecret string
	RedirectURL  string
}

type HTTPConfig struct {
	Addr              string
	ReadHeaderTimeout time.Duration
	ReadTimeout       time.Duration
	WriteTimeout      time.Duration
	IdleTimeout       time.Duration
	ShutdownTimeout   time.Duration
}

func Load(getEnv func(string) string) (Config, error) {
	sessionTTL, err := durationEnvOrDefault(getEnv, "USER_SESSION_TTL", defaultUserSessionTTL)
	if err != nil {
		return Config{}, err
	}

	if sessionTTL <= 0 {
		return Config{}, errors.New("USER_SESSION_TTL must be greater than zero")
	}

	cookieSecure, err := boolEnvOrDefault(getEnv, "COOKIE_SECURE", defaultCookieSecure)
	if err != nil {
		return Config{}, err
	}

	cfg := Config{
		DatabaseURL: getEnv("DATABASE_URL"),
		HTTP: HTTPConfig{
			Addr:              envOrDefault(getEnv, "HTTP_ADDR", defaultHTTPAddr),
			ReadHeaderTimeout: defaultReadHeaderTimeout,
			ReadTimeout:       defaultReadTimeout,
			WriteTimeout:      defaultWriteTimeout,
			IdleTimeout:       defaultIdleTimeout,
			ShutdownTimeout:   defaultShutdownTimeout,
		},
		Session: SessionConfig{
			TTL:          sessionTTL,
			CookieSecure: cookieSecure,
		},
		Google: GoogleConfig{
			ClientID:     getEnv("GOOGLE_CLIENT_ID"),
			ClientSecret: getEnv("GOOGLE_CLIENT_SECRET"),
			RedirectURL:  getEnv("GOOGLE_REDIRECT_URL"),
		},
	}

	if cfg.DatabaseURL == "" {
		return Config{}, errors.New("DATABASE_URL is required")
	}

	if cfg.Google.ClientID == "" {
		return Config{}, errors.New("GOOGLE_CLIENT_ID is required")
	}

	if cfg.Google.ClientSecret == "" {
		return Config{}, errors.New("GOOGLE_CLIENT_SECRET is required")
	}

	if cfg.Google.RedirectURL == "" {
		return Config{}, errors.New("GOOGLE_REDIRECT_URL is required")
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

func durationEnvOrDefault(getEnv func(string) string, key string, defaultValue time.Duration) (time.Duration, error) {
	value := getEnv(key)
	if value == "" {
		return defaultValue, nil
	}

	duration, err := time.ParseDuration(value)
	if err != nil {
		return 0, fmt.Errorf("%s must be a valid duration: %w", key, err)
	}

	return duration, nil
}

func boolEnvOrDefault(getEnv func(string) string, key string, defaultValue bool) (bool, error) {
	value := getEnv(key)
	if value == "" {
		return defaultValue, nil
	}

	boolean, err := strconv.ParseBool(value)
	if err != nil {
		return false, fmt.Errorf("%s must be a valid boolean: %w", key, err)
	}

	return boolean, nil
}
