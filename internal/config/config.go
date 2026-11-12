package config

import (
	"errors"
	"fmt"
	"net/url"
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

	defaultUserSessionTTL           = 12 * time.Hour
	defaultRememberedUserSessionTTL = 720 * time.Hour
	defaultLinkAccessSessionTTL     = 30 * time.Minute
	defaultCookieSecure             = true

	defaultPasswordRegistrationCodeTTL    = 10 * time.Minute
	defaultPasswordRegistrationAttemptTTL = 30 * time.Minute
	defaultPasswordResetCodeTTL           = 10 * time.Minute
	defaultPasswordResetAttemptTTL        = 30 * time.Minute

	defaultBaseURL             = "http://localhost:8080"
	defaultMaxOriginalURLBytes = 8192

	defaultRedisAddr             = "127.0.0.1:6379"
	defaultRedisOperationTimeout = 200 * time.Millisecond

	defaultRabbitMQPublishTimeout = 500 * time.Millisecond

	defaultRabbitMQPrefetch             = 10
	maxRabbitMQPrefetch                 = 65535
	defaultProcessedClickEventRetention = 7 * 24 * time.Hour
)

type Config struct {
	DatabaseURL          string
	HTTP                 HTTPConfig
	Session              SessionConfig
	LinkAccessSession    LinkAccessSessionConfig
	Google               GoogleConfig
	Email                EmailConfig
	Resend               ResendConfig
	PasswordRegistration PasswordRegistrationConfig
	PasswordReset        PasswordResetConfig
	URL                  URLConfig
	Redis                RedisConfig
	RabbitMQ             RabbitMQConfig
}

type SessionConfig struct {
	TTL           time.Duration
	RememberedTTL time.Duration
	CookieSecure  bool
}

type LinkAccessSessionConfig struct {
	TTL time.Duration
}

type EmailConfig struct {
	From string
}

type URLConfig struct {
	BaseURL             string
	MaxOriginalURLBytes int
}

type ResendConfig struct {
	APIKey string
}

type RedisConfig struct {
	Addr    string
	Timeout time.Duration
}

type RabbitMQConfig struct {
	URL            string
	PublishTimeout time.Duration
}

type WorkerConfig struct {
	DatabaseURL                  string
	RabbitMQURL                  string
	RabbitMQPrefetch             int
	ProcessedClickEventRetention time.Duration
}

type PasswordRegistrationConfig struct {
	CodeTTL    time.Duration
	AttemptTTL time.Duration
}

type PasswordResetConfig struct {
	CodeTTL    time.Duration
	AttemptTTL time.Duration
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

	rememberedSessionTTL, err := durationEnvOrDefault(getEnv, "REMEMBERED_USER_SESSION_TTL", defaultRememberedUserSessionTTL)
	if err != nil {
		return Config{}, err
	}
	if rememberedSessionTTL <= 0 {
		return Config{}, errors.New("REMEMBERED_USER_SESSION_TTL must be greater than zero")
	}
	if rememberedSessionTTL <= sessionTTL {
		return Config{}, errors.New("REMEMBERED_USER_SESSION_TTL must be greater than USER_SESSION_TTL")
	}

	linkAccessSessionTTL, err := durationEnvOrDefault(getEnv, "LINK_ACCESS_SESSION_TTL", defaultLinkAccessSessionTTL)
	if err != nil {
		return Config{}, err
	}
	if linkAccessSessionTTL <= 0 {
		return Config{}, errors.New("LINK_ACCESS_SESSION_TTL must be greater than zero")
	}

	cookieSecure, err := boolEnvOrDefault(getEnv, "COOKIE_SECURE", defaultCookieSecure)
	if err != nil {
		return Config{}, err
	}

	codeTTL, err := durationEnvOrDefault(getEnv, "PASSWORD_REGISTRATION_CODE_TTL", defaultPasswordRegistrationCodeTTL)
	if err != nil {
		return Config{}, err
	}

	if codeTTL <= 0 {
		return Config{}, errors.New("PASSWORD_REGISTRATION_CODE_TTL must be greater than zero")
	}

	redisOperationTimeout, err := durationEnvOrDefault(getEnv, "REDIS_OPERATION_TIMEOUT", defaultRedisOperationTimeout)
	if err != nil {
		return Config{}, err
	}
	if redisOperationTimeout <= 0 {
		return Config{}, errors.New("REDIS_OPERATION_TIMEOUT must be greater than zero")
	}

	rabbitMQPublishTimeout, err := durationEnvOrDefault(getEnv, "RABBITMQ_PUBLISH_TIMEOUT", defaultRabbitMQPublishTimeout)
	if err != nil {
		return Config{}, err
	}
	if rabbitMQPublishTimeout <= 0 {
		return Config{}, errors.New("RABBITMQ_PUBLISH_TIMEOUT must be greater than zero")
	}

	attemptTTL, err := durationEnvOrDefault(getEnv, "PASSWORD_REGISTRATION_ATTEMPT_TTL", defaultPasswordRegistrationAttemptTTL)
	if err != nil {
		return Config{}, err
	}

	if attemptTTL <= 0 {
		return Config{}, errors.New("PASSWORD_REGISTRATION_ATTEMPT_TTL must be greater than zero")
	}

	if codeTTL > attemptTTL {
		return Config{}, errors.New("PASSWORD_REGISTRATION_CODE_TTL must not be greater than PASSWORD_REGISTRATION_ATTEMPT_TTL")
	}

	resetCodeTTL, err := durationEnvOrDefault(getEnv, "PASSWORD_RESET_CODE_TTL", defaultPasswordResetCodeTTL)
	if err != nil {
		return Config{}, err
	}
	if resetCodeTTL <= 0 {
		return Config{}, errors.New("PASSWORD_RESET_CODE_TTL must be greater than zero")
	}

	resetAttemptTTL, err := durationEnvOrDefault(getEnv, "PASSWORD_RESET_ATTEMPT_TTL", defaultPasswordResetAttemptTTL)
	if err != nil {
		return Config{}, err
	}
	if resetAttemptTTL <= 0 {
		return Config{}, errors.New("PASSWORD_RESET_ATTEMPT_TTL must be greater than zero")
	}
	if resetCodeTTL > resetAttemptTTL {
		return Config{}, errors.New("PASSWORD_RESET_CODE_TTL must not be greater than PASSWORD_RESET_ATTEMPT_TTL")
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
			TTL:           sessionTTL,
			RememberedTTL: rememberedSessionTTL,
			CookieSecure:  cookieSecure,
		},
		LinkAccessSession: LinkAccessSessionConfig{TTL: linkAccessSessionTTL},
		Google: GoogleConfig{
			ClientID:     getEnv("GOOGLE_CLIENT_ID"),
			ClientSecret: getEnv("GOOGLE_CLIENT_SECRET"),
			RedirectURL:  getEnv("GOOGLE_REDIRECT_URL"),
		},
		Email: EmailConfig{
			From: getEnv("EMAIL_FROM"),
		},
		Resend: ResendConfig{
			APIKey: getEnv("RESEND_API_KEY"),
		},
		PasswordRegistration: PasswordRegistrationConfig{
			CodeTTL:    codeTTL,
			AttemptTTL: attemptTTL,
		},
		PasswordReset: PasswordResetConfig{CodeTTL: resetCodeTTL,
			AttemptTTL: resetAttemptTTL,
		},
		URL: URLConfig{
			BaseURL:             envOrDefault(getEnv, "PUBLIC_BASE_URL", defaultBaseURL),
			MaxOriginalURLBytes: defaultMaxOriginalURLBytes,
		},
		Redis: RedisConfig{
			Addr:    envOrDefault(getEnv, "REDIS_ADDR", defaultRedisAddr),
			Timeout: redisOperationTimeout,
		},
		RabbitMQ: RabbitMQConfig{
			URL:            getEnv("RABBITMQ_URL"),
			PublishTimeout: rabbitMQPublishTimeout,
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

	if cfg.Email.From == "" {
		return Config{}, errors.New("EMAIL_FROM is required")
	}

	if cfg.Resend.APIKey == "" {
		return Config{}, errors.New("RESEND_API_KEY is required")
	}

	if cfg.RabbitMQ.URL == "" {
		return Config{}, errors.New("RABBITMQ_URL is required")
	}

	if err := validateAMQPURL(cfg.RabbitMQ.URL); err != nil {
		return Config{}, err
	}

	return cfg, nil
}

func LoadWorker(getEnv func(string) string) (WorkerConfig, error) {
	prefetch, err := intEnvOrDefault(getEnv, "RABBITMQ_PREFETCH", defaultRabbitMQPrefetch)
	if err != nil {
		return WorkerConfig{}, err
	}
	if prefetch <= 0 || prefetch > maxRabbitMQPrefetch {
		return WorkerConfig{}, fmt.Errorf("RABBITMQ_PREFETCH must be between 1 and %d", maxRabbitMQPrefetch)
	}

	retention, err := durationEnvOrDefault(getEnv, "PROCESSED_CLICK_EVENT_RETENTION", defaultProcessedClickEventRetention)
	if err != nil {
		return WorkerConfig{}, err
	}
	if retention <= 0 {
		return WorkerConfig{}, errors.New("PROCESSED_CLICK_EVENT_RETENTION must be greater than zero")
	}

	cfg := WorkerConfig{
		DatabaseURL:                  getEnv("DATABASE_URL"),
		RabbitMQURL:                  getEnv("RABBITMQ_URL"),
		RabbitMQPrefetch:             prefetch,
		ProcessedClickEventRetention: retention,
	}

	if cfg.DatabaseURL == "" {
		return WorkerConfig{}, errors.New("DATABASE_URL is required")
	}

	if cfg.RabbitMQURL == "" {
		return WorkerConfig{}, errors.New("RABBITMQ_URL is required")
	}

	if err := validateAMQPURL(cfg.RabbitMQURL); err != nil {
		return WorkerConfig{}, err
	}

	return cfg, nil
}

func validateAMQPURL(value string) error {
	parsed, err := url.Parse(value)
	if err != nil {
		return errors.New("RABBITMQ_URL must be a valid URL")
	}

	if parsed.Scheme != "amqp" && parsed.Scheme != "amqps" {
		return errors.New("RABBITMQ_URL must use the amqp or amqps scheme")
	}

	if parsed.Host == "" {
		return errors.New("RABBITMQ_URL must include a host")
	}

	return nil
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

func intEnvOrDefault(getEnv func(string) string, key string, defaultValue int) (int, error) {
	value := getEnv(key)
	if value == "" {
		return defaultValue, nil
	}

	number, err := strconv.Atoi(value)
	if err != nil {
		return 0, fmt.Errorf("%s must be a valid integer: %w", key, err)
	}

	return number, nil
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
