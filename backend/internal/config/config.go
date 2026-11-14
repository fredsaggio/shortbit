package config

import (
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
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

	defaultUserSessionTTL           = 12 * time.Hour
	defaultRememberedUserSessionTTL = 720 * time.Hour
	defaultLinkAccessSessionTTL     = 30 * time.Minute
	defaultCookieSecure             = true

	defaultPasswordRegistrationCodeTTL    = 10 * time.Minute
	defaultPasswordRegistrationAttemptTTL = 30 * time.Minute
	defaultPasswordResetCodeTTL           = 10 * time.Minute
	defaultPasswordResetAttemptTTL        = 30 * time.Minute

	defaultBaseURL             = "http://localhost:8080"
	defaultFrontendURL         = "http://localhost:5173"
	defaultMaxOriginalURLBytes = 8192

	defaultRedisAddr             = "127.0.0.1:6379"
	defaultRedisOperationTimeout = 200 * time.Millisecond

	defaultHotURLThreshold = 20
	defaultHotURLWindow    = 60 * time.Second
	defaultURLCacheTTL     = 5 * time.Minute

	defaultRabbitMQPublishTimeout = 500 * time.Millisecond

	defaultRabbitMQPrefetch             = 10
	maxRabbitMQPrefetch                 = 65535
	defaultProcessedClickEventRetention = 7 * 24 * time.Hour
)

type Config struct {
	DatabaseURL          string
	RunMigrations        bool
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
	HotURL               HotURLConfig
	RabbitMQ             RabbitMQConfig
	ClickWorker          ClickWorkerConfig
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
	FrontendURL         string
	MaxOriginalURLBytes int
}

type ResendConfig struct {
	APIKey string
}

// RedisConfig takes either URL (redis:// or rediss://, with password and TLS,
// as hosted Redis services hand out) or a plain Addr for a local server.
type RedisConfig struct {
	URL     string
	Addr    string
	Timeout time.Duration
}

type HotURLConfig struct {
	Threshold int64
	Window    time.Duration
	CacheTTL  time.Duration
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

// ClickWorkerConfig lets the API run the click worker in its own process, for
// hosts where a separate worker service is not available. The standalone
// worker (cmd/worker) reads the same prefetch and retention settings.
type ClickWorkerConfig struct {
	RunInAPI                     bool
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
	ClientIPHeader    string
	FrontendDir       string
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

	hotURLThreshold, err := intEnvOrDefault(getEnv, "HOT_URL_THRESHOLD", defaultHotURLThreshold)
	if err != nil {
		return Config{}, err
	}
	if hotURLThreshold <= 0 {
		return Config{}, errors.New("HOT_URL_THRESHOLD must be greater than zero")
	}

	hotURLWindow, err := durationEnvOrDefault(getEnv, "HOT_URL_WINDOW", defaultHotURLWindow)
	if err != nil {
		return Config{}, err
	}
	if hotURLWindow <= 0 {
		return Config{}, errors.New("HOT_URL_WINDOW must be greater than zero")
	}

	urlCacheTTL, err := durationEnvOrDefault(getEnv, "URL_CACHE_TTL", defaultURLCacheTTL)
	if err != nil {
		return Config{}, err
	}
	if urlCacheTTL <= 0 {
		return Config{}, errors.New("URL_CACHE_TTL must be greater than zero")
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

	runMigrations, err := boolEnvOrDefault(getEnv, "RUN_MIGRATIONS", false)
	if err != nil {
		return Config{}, err
	}

	runClickWorker, err := boolEnvOrDefault(getEnv, "RUN_CLICK_WORKER", false)
	if err != nil {
		return Config{}, err
	}

	prefetch, retention, err := loadClickWorkerSettings(getEnv)
	if err != nil {
		return Config{}, err
	}

	clientIPHeader, err := clientIPHeaderEnv(getEnv)
	if err != nil {
		return Config{}, err
	}

	cfg := Config{
		DatabaseURL:   getEnv("DATABASE_URL"),
		RunMigrations: runMigrations,
		HTTP: HTTPConfig{
			Addr:              envOrDefault(getEnv, "HTTP_ADDR", defaultHTTPAddr),
			ClientIPHeader:    clientIPHeader,
			FrontendDir:       getEnv("FRONTEND_DIST_DIR"),
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
			FrontendURL:         strings.TrimSuffix(envOrDefault(getEnv, "FRONTEND_URL", defaultFrontendURL), "/"),
			MaxOriginalURLBytes: defaultMaxOriginalURLBytes,
		},
		Redis: RedisConfig{
			URL:     getEnv("REDIS_URL"),
			Addr:    envOrDefault(getEnv, "REDIS_ADDR", defaultRedisAddr),
			Timeout: redisOperationTimeout,
		},
		HotURL: HotURLConfig{
			Threshold: int64(hotURLThreshold),
			Window:    hotURLWindow,
			CacheTTL:  urlCacheTTL,
		},
		RabbitMQ: RabbitMQConfig{
			URL:            getEnv("RABBITMQ_URL"),
			PublishTimeout: rabbitMQPublishTimeout,
		},
		ClickWorker: ClickWorkerConfig{
			RunInAPI:                     runClickWorker,
			RabbitMQPrefetch:             prefetch,
			ProcessedClickEventRetention: retention,
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

	if err := validateFrontendURL(cfg.URL.FrontendURL); err != nil {
		return Config{}, err
	}

	if cfg.RabbitMQ.URL == "" {
		return Config{}, errors.New("RABBITMQ_URL is required")
	}

	if err := validateAMQPURL(cfg.RabbitMQ.URL); err != nil {
		return Config{}, err
	}

	if cfg.Redis.URL != "" {
		if err := validateRedisURL(cfg.Redis.URL); err != nil {
			return Config{}, err
		}
	}

	return cfg, nil
}

func LoadWorker(getEnv func(string) string) (WorkerConfig, error) {
	prefetch, retention, err := loadClickWorkerSettings(getEnv)
	if err != nil {
		return WorkerConfig{}, err
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

func loadClickWorkerSettings(getEnv func(string) string) (int, time.Duration, error) {
	prefetch, err := intEnvOrDefault(getEnv, "RABBITMQ_PREFETCH", defaultRabbitMQPrefetch)
	if err != nil {
		return 0, 0, err
	}
	if prefetch <= 0 || prefetch > maxRabbitMQPrefetch {
		return 0, 0, fmt.Errorf("RABBITMQ_PREFETCH must be between 1 and %d", maxRabbitMQPrefetch)
	}

	retention, err := durationEnvOrDefault(getEnv, "PROCESSED_CLICK_EVENT_RETENTION", defaultProcessedClickEventRetention)
	if err != nil {
		return 0, 0, err
	}
	if retention <= 0 {
		return 0, 0, errors.New("PROCESSED_CLICK_EVENT_RETENTION must be greater than zero")
	}

	return prefetch, retention, nil
}

var headerNamePattern = regexp.MustCompile(`^[A-Za-z0-9-]+$`)

// CLIENT_IP_HEADER names a header that the proxy in front of the API always
// overwrites with the client address. X-Forwarded-For is refused: it is a list
// that clients can prefill, so its first entry is whatever they want.
func clientIPHeaderEnv(getEnv func(string) string) (string, error) {
	value := strings.TrimSpace(getEnv("CLIENT_IP_HEADER"))
	if value == "" {
		return "", nil
	}

	if !headerNamePattern.MatchString(value) {
		return "", errors.New("CLIENT_IP_HEADER must be an HTTP header name such as True-Client-IP")
	}

	header := http.CanonicalHeaderKey(value)
	if header == "X-Forwarded-For" {
		return "", errors.New("CLIENT_IP_HEADER must not be X-Forwarded-For, which clients can forge; use a header the proxy overwrites, such as True-Client-IP")
	}

	return header, nil
}

func validateRedisURL(value string) error {
	parsed, err := url.Parse(value)
	if err != nil {
		return errors.New("REDIS_URL must be a valid URL")
	}

	if parsed.Scheme != "redis" && parsed.Scheme != "rediss" {
		return errors.New("REDIS_URL must use the redis or rediss scheme")
	}

	if parsed.Host == "" {
		return errors.New("REDIS_URL must include a host")
	}

	return nil
}

// FRONTEND_URL is the only destination of the Google callback redirect, so it
// must be a bare origin: no path, query, fragment, or credentials.
func validateFrontendURL(value string) error {
	parsed, err := url.Parse(value)
	if err != nil {
		return errors.New("FRONTEND_URL must be a valid URL")
	}

	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return errors.New("FRONTEND_URL must use the http or https scheme")
	}

	if parsed.Host == "" || parsed.User != nil || parsed.Path != "" || parsed.RawQuery != "" || parsed.Fragment != "" {
		return errors.New("FRONTEND_URL must be an origin such as https://shortbit.example, without path or query")
	}

	return nil
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
