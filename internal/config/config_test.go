package config_test

import (
	"strings"
	"testing"
	"time"

	"github.com/fredsaggio/url-shortener/internal/config"
)

func TestLoad(t *testing.T) {
	tests := []struct {
		name            string
		env             map[string]string
		want            config.Config
		wantErrContains string
	}{
		{
			name: "loads provided configuration",
			env: map[string]string{
				"DATABASE_URL":                      "postgres://user:password@localhost:5432/app",
				"HTTP_ADDR":                         ":9090",
				"USER_SESSION_TTL":                  "12h",
				"REMEMBERED_USER_SESSION_TTL":       "336h",
				"LINK_ACCESS_SESSION_TTL":           "45m",
				"COOKIE_SECURE":                     "false",
				"GOOGLE_CLIENT_ID":                  "test-client-id",
				"GOOGLE_CLIENT_SECRET":              "test-client-secret",
				"GOOGLE_REDIRECT_URL":               "http://localhost:8080/auth/google/callback",
				"EMAIL_FROM":                        "URL Shortener <noreply@example.com>",
				"RESEND_API_KEY":                    "re_test_api_key",
				"PASSWORD_REGISTRATION_CODE_TTL":    "5m",
				"PASSWORD_REGISTRATION_ATTEMPT_TTL": "20m",
				"PASSWORD_RESET_CODE_TTL":           "8m",
				"PASSWORD_RESET_ATTEMPT_TTL":        "25m",
				"PUBLIC_BASE_URL":                   "https://shortbit.com.br",
				"REDIS_ADDR":                        "redis.example:6380",
				"REDIS_OPERATION_TIMEOUT":           "350ms",
				"RABBITMQ_URL":                      "amqps://user:password@rabbitmq.example:5671/shortbit",
				"RABBITMQ_PUBLISH_TIMEOUT":          "750ms",
			},
			want: config.Config{
				DatabaseURL: "postgres://user:password@localhost:5432/app",
				HTTP: config.HTTPConfig{
					Addr:              ":9090",
					ReadHeaderTimeout: 5 * time.Second,
					ReadTimeout:       10 * time.Second,
					WriteTimeout:      15 * time.Second,
					IdleTimeout:       60 * time.Second,
					ShutdownTimeout:   10 * time.Second,
				},
				Session: config.SessionConfig{
					TTL:           12 * time.Hour,
					RememberedTTL: 336 * time.Hour,
					CookieSecure:  false,
				},
				LinkAccessSession: config.LinkAccessSessionConfig{TTL: 45 * time.Minute},
				Google: config.GoogleConfig{
					ClientID:     "test-client-id",
					ClientSecret: "test-client-secret",
					RedirectURL:  "http://localhost:8080/auth/google/callback",
				},
				Email: config.EmailConfig{
					From: "URL Shortener <noreply@example.com>",
				},
				Resend: config.ResendConfig{
					APIKey: "re_test_api_key",
				},
				PasswordRegistration: config.PasswordRegistrationConfig{
					CodeTTL:    5 * time.Minute,
					AttemptTTL: 20 * time.Minute,
				},
				PasswordReset: config.PasswordResetConfig{CodeTTL: 8 * time.Minute, AttemptTTL: 25 * time.Minute},
				URL: config.URLConfig{
					BaseURL:             "https://shortbit.com.br",
					MaxOriginalURLBytes: 8192,
				},
				Redis:    config.RedisConfig{Addr: "redis.example:6380", Timeout: 350 * time.Millisecond},
				RabbitMQ: config.RabbitMQConfig{URL: "amqps://user:password@rabbitmq.example:5671/shortbit", PublishTimeout: 750 * time.Millisecond},
			},
		},
		{
			name: "uses default values",
			env: map[string]string{
				"DATABASE_URL":         "postgres://localhost/app",
				"GOOGLE_CLIENT_ID":     "test-client-id",
				"GOOGLE_CLIENT_SECRET": "test-client-secret",
				"GOOGLE_REDIRECT_URL":  "http://localhost:8080/auth/google/callback",
				"EMAIL_FROM":           "noreply@example.com",
				"RESEND_API_KEY":       "re_test_api_key",
				"RABBITMQ_URL":         "amqp://user:password@localhost:5672/",
			},
			want: config.Config{
				DatabaseURL: "postgres://localhost/app",
				HTTP: config.HTTPConfig{
					Addr:              ":8080",
					ReadHeaderTimeout: 5 * time.Second,
					ReadTimeout:       10 * time.Second,
					WriteTimeout:      15 * time.Second,
					IdleTimeout:       60 * time.Second,
					ShutdownTimeout:   10 * time.Second,
				},
				Session: config.SessionConfig{
					TTL:           12 * time.Hour,
					RememberedTTL: 720 * time.Hour,
					CookieSecure:  true,
				},
				LinkAccessSession: config.LinkAccessSessionConfig{TTL: 30 * time.Minute},
				Google: config.GoogleConfig{
					ClientID:     "test-client-id",
					ClientSecret: "test-client-secret",
					RedirectURL:  "http://localhost:8080/auth/google/callback",
				},
				Email: config.EmailConfig{
					From: "noreply@example.com",
				},
				Resend: config.ResendConfig{
					APIKey: "re_test_api_key",
				},
				PasswordRegistration: config.PasswordRegistrationConfig{
					CodeTTL:    10 * time.Minute,
					AttemptTTL: 30 * time.Minute,
				},
				PasswordReset: config.PasswordResetConfig{CodeTTL: 10 * time.Minute, AttemptTTL: 30 * time.Minute},
				URL: config.URLConfig{
					BaseURL:             "http://localhost:8080",
					MaxOriginalURLBytes: 8192,
				},
				Redis:    config.RedisConfig{Addr: "127.0.0.1:6379", Timeout: 200 * time.Millisecond},
				RabbitMQ: config.RabbitMQConfig{URL: "amqp://user:password@localhost:5672/", PublishTimeout: 500 * time.Millisecond},
			},
		},
		{
			name:            "rejects invalid Redis operation timeout",
			env:             map[string]string{"REDIS_OPERATION_TIMEOUT": "soon"},
			wantErrContains: "REDIS_OPERATION_TIMEOUT must be a valid duration",
		},
		{
			name:            "rejects zero Redis operation timeout",
			env:             map[string]string{"REDIS_OPERATION_TIMEOUT": "0s"},
			wantErrContains: "REDIS_OPERATION_TIMEOUT must be greater than zero",
		},
		{
			name:            "rejects negative Redis operation timeout",
			env:             map[string]string{"REDIS_OPERATION_TIMEOUT": "-1ms"},
			wantErrContains: "REDIS_OPERATION_TIMEOUT must be greater than zero",
		},
		{
			name: "rejects invalid session TTL",
			env: map[string]string{
				"DATABASE_URL":     "postgres://localhost/app",
				"USER_SESSION_TTL": "one day",
			},
			wantErrContains: "USER_SESSION_TTL must be a valid duration",
		},
		{
			name: "rejects zero session TTL",
			env: map[string]string{
				"DATABASE_URL":     "postgres://localhost/app",
				"USER_SESSION_TTL": "0s",
			},
			wantErrContains: "USER_SESSION_TTL must be greater than zero",
		},
		{
			name: "rejects negative session TTL",
			env: map[string]string{
				"DATABASE_URL":     "postgres://localhost/app",
				"USER_SESSION_TTL": "-1h",
			},
			wantErrContains: "USER_SESSION_TTL must be greater than zero",
		},
		{
			name:            "rejects invalid remembered session TTL",
			env:             map[string]string{"REMEMBERED_USER_SESSION_TTL": "thirty days"},
			wantErrContains: "REMEMBERED_USER_SESSION_TTL must be a valid duration",
		},
		{
			name:            "rejects zero remembered session TTL",
			env:             map[string]string{"REMEMBERED_USER_SESSION_TTL": "0s"},
			wantErrContains: "REMEMBERED_USER_SESSION_TTL must be greater than zero",
		},
		{
			name:            "rejects negative remembered session TTL",
			env:             map[string]string{"REMEMBERED_USER_SESSION_TTL": "-1h"},
			wantErrContains: "REMEMBERED_USER_SESSION_TTL must be greater than zero",
		},
		{
			name:            "rejects remembered TTL equal to ordinary TTL",
			env:             map[string]string{"USER_SESSION_TTL": "12h", "REMEMBERED_USER_SESSION_TTL": "12h"},
			wantErrContains: "REMEMBERED_USER_SESSION_TTL must be greater than USER_SESSION_TTL",
		},
		{
			name:            "rejects remembered TTL shorter than ordinary TTL",
			env:             map[string]string{"USER_SESSION_TTL": "24h", "REMEMBERED_USER_SESSION_TTL": "12h"},
			wantErrContains: "REMEMBERED_USER_SESSION_TTL must be greater than USER_SESSION_TTL",
		},
		{
			name:            "rejects invalid link access session TTL",
			env:             map[string]string{"LINK_ACCESS_SESSION_TTL": "thirty minutes"},
			wantErrContains: "LINK_ACCESS_SESSION_TTL must be a valid duration",
		},
		{
			name:            "rejects zero link access session TTL",
			env:             map[string]string{"LINK_ACCESS_SESSION_TTL": "0s"},
			wantErrContains: "LINK_ACCESS_SESSION_TTL must be greater than zero",
		},
		{
			name:            "rejects negative link access session TTL",
			env:             map[string]string{"LINK_ACCESS_SESSION_TTL": "-1m"},
			wantErrContains: "LINK_ACCESS_SESSION_TTL must be greater than zero",
		},
		{
			name: "rejects invalid cookie secure value",
			env: map[string]string{
				"DATABASE_URL":  "postgres://localhost/app",
				"COOKIE_SECURE": "sometimes",
			},
			wantErrContains: "COOKIE_SECURE must be a valid boolean",
		},
		{
			name: "rejects invalid password registration code TTL",
			env: map[string]string{
				"PASSWORD_REGISTRATION_CODE_TTL": "ten minutes",
			},
			wantErrContains: "PASSWORD_REGISTRATION_CODE_TTL must be a valid duration",
		},
		{
			name: "rejects non-positive password registration code TTL",
			env: map[string]string{
				"PASSWORD_REGISTRATION_CODE_TTL": "0s",
			},
			wantErrContains: "PASSWORD_REGISTRATION_CODE_TTL must be greater than zero",
		},
		{
			name: "rejects invalid password registration attempt TTL",
			env: map[string]string{
				"PASSWORD_REGISTRATION_ATTEMPT_TTL": "thirty minutes",
			},
			wantErrContains: "PASSWORD_REGISTRATION_ATTEMPT_TTL must be a valid duration",
		},
		{
			name: "rejects non-positive password registration attempt TTL",
			env: map[string]string{
				"PASSWORD_REGISTRATION_ATTEMPT_TTL": "-1m",
			},
			wantErrContains: "PASSWORD_REGISTRATION_ATTEMPT_TTL must be greater than zero",
		},
		{
			name: "rejects code TTL greater than attempt TTL",
			env: map[string]string{
				"PASSWORD_REGISTRATION_CODE_TTL":    "20m",
				"PASSWORD_REGISTRATION_ATTEMPT_TTL": "10m",
			},
			wantErrContains: "PASSWORD_REGISTRATION_CODE_TTL must not be greater than PASSWORD_REGISTRATION_ATTEMPT_TTL",
		},
		{name: "rejects invalid reset code TTL", env: map[string]string{"PASSWORD_RESET_CODE_TTL": "ten minutes"}, wantErrContains: "PASSWORD_RESET_CODE_TTL must be a valid duration"},
		{name: "rejects non-positive reset code TTL", env: map[string]string{"PASSWORD_RESET_CODE_TTL": "0s"}, wantErrContains: "PASSWORD_RESET_CODE_TTL must be greater than zero"},
		{name: "rejects invalid reset attempt TTL", env: map[string]string{"PASSWORD_RESET_ATTEMPT_TTL": "thirty minutes"}, wantErrContains: "PASSWORD_RESET_ATTEMPT_TTL must be a valid duration"},
		{name: "rejects non-positive reset attempt TTL", env: map[string]string{"PASSWORD_RESET_ATTEMPT_TTL": "-1m"}, wantErrContains: "PASSWORD_RESET_ATTEMPT_TTL must be greater than zero"},
		{name: "rejects reset code TTL greater than attempt TTL", env: map[string]string{"PASSWORD_RESET_CODE_TTL": "20m", "PASSWORD_RESET_ATTEMPT_TTL": "10m"}, wantErrContains: "PASSWORD_RESET_CODE_TTL must not be greater than PASSWORD_RESET_ATTEMPT_TTL"},
		{
			name: "rejects missing Google client ID",
			env: map[string]string{
				"DATABASE_URL":         "postgres://localhost/app",
				"GOOGLE_CLIENT_SECRET": "test-client-secret",
				"GOOGLE_REDIRECT_URL":  "http://localhost:8080/auth/google/callback",
			},
			wantErrContains: "GOOGLE_CLIENT_ID is required",
		},
		{
			name: "rejects missing Google client secret",
			env: map[string]string{
				"DATABASE_URL":        "postgres://localhost/app",
				"GOOGLE_CLIENT_ID":    "test-client-id",
				"GOOGLE_REDIRECT_URL": "http://localhost:8080/auth/google/callback",
			},
			wantErrContains: "GOOGLE_CLIENT_SECRET is required",
		},
		{
			name: "rejects missing Google redirect URL",
			env: map[string]string{
				"DATABASE_URL":         "postgres://localhost/app",
				"GOOGLE_CLIENT_ID":     "test-client-id",
				"GOOGLE_CLIENT_SECRET": "test-client-secret",
			},
			wantErrContains: "GOOGLE_REDIRECT_URL is required",
		},
		{
			name: "rejects missing email sender",
			env: map[string]string{
				"DATABASE_URL":         "postgres://localhost/app",
				"GOOGLE_CLIENT_ID":     "test-client-id",
				"GOOGLE_CLIENT_SECRET": "test-client-secret",
				"GOOGLE_REDIRECT_URL":  "http://localhost:8080/auth/google/callback",
				"RESEND_API_KEY":       "re_test_api_key",
			},
			wantErrContains: "EMAIL_FROM is required",
		},
		{
			name: "rejects missing Resend API key",
			env: map[string]string{
				"DATABASE_URL":         "postgres://localhost/app",
				"GOOGLE_CLIENT_ID":     "test-client-id",
				"GOOGLE_CLIENT_SECRET": "test-client-secret",
				"GOOGLE_REDIRECT_URL":  "http://localhost:8080/auth/google/callback",
				"EMAIL_FROM":           "noreply@example.com",
			},
			wantErrContains: "RESEND_API_KEY is required",
		},
		{
			name:            "rejects invalid RabbitMQ publish timeout",
			env:             map[string]string{"RABBITMQ_PUBLISH_TIMEOUT": "soon"},
			wantErrContains: "RABBITMQ_PUBLISH_TIMEOUT must be a valid duration",
		},
		{
			name:            "rejects zero RabbitMQ publish timeout",
			env:             map[string]string{"RABBITMQ_PUBLISH_TIMEOUT": "0s"},
			wantErrContains: "RABBITMQ_PUBLISH_TIMEOUT must be greater than zero",
		},
		{
			name:            "rejects missing RabbitMQ URL",
			env:             rabbitMQEnv(""),
			wantErrContains: "RABBITMQ_URL is required",
		},
		{
			name:            "rejects RabbitMQ URL with another scheme",
			env:             rabbitMQEnv("http://user:password@localhost:5672/"),
			wantErrContains: "RABBITMQ_URL must use the amqp or amqps scheme",
		},
		{
			name:            "rejects RabbitMQ URL without host",
			env:             rabbitMQEnv("amqp:///shortbit"),
			wantErrContains: "RABBITMQ_URL must include a host",
		},
		{
			name:            "rejects malformed RabbitMQ URL",
			env:             rabbitMQEnv("amqp://user:password@localhost:port/"),
			wantErrContains: "RABBITMQ_URL must be a valid URL",
		},
		{
			name:            "rejects missing database URL",
			env:             map[string]string{},
			wantErrContains: "DATABASE_URL is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			getEnv := func(key string) string {
				return tt.env[key]
			}

			got, err := config.Load(getEnv)

			if tt.wantErrContains != "" {
				if err == nil {
					t.Fatalf(
						"Load() error = nil, want error containing %q",
						tt.wantErrContains,
					)
				}

				if !strings.Contains(err.Error(), tt.wantErrContains) {
					t.Errorf(
						"Load() error = %q, want error containing %q",
						err,
						tt.wantErrContains,
					)
				}

				return
			}

			if err != nil {
				t.Fatalf("Load() unexpected error: %v", err)
			}

			if got != tt.want {
				t.Errorf("Load() = %#v, want %#v", got, tt.want)
			}
		})
	}
}

func rabbitMQEnv(rabbitMQURL string) map[string]string {
	return map[string]string{
		"DATABASE_URL":         "postgres://localhost/app",
		"GOOGLE_CLIENT_ID":     "test-client-id",
		"GOOGLE_CLIENT_SECRET": "test-client-secret",
		"GOOGLE_REDIRECT_URL":  "http://localhost:8080/auth/google/callback",
		"EMAIL_FROM":           "noreply@example.com",
		"RESEND_API_KEY":       "re_test_api_key",
		"RABBITMQ_URL":         rabbitMQURL,
	}
}

func TestLoadWorker(t *testing.T) {
	validEnv := func(overrides map[string]string) map[string]string {
		env := map[string]string{
			"DATABASE_URL": "postgres://localhost/app",
			"RABBITMQ_URL": "amqp://user:password@localhost:5672/",
		}
		for key, value := range overrides {
			env[key] = value
		}
		return env
	}

	defaults := config.WorkerConfig{
		DatabaseURL:                  "postgres://localhost/app",
		RabbitMQURL:                  "amqp://user:password@localhost:5672/",
		RabbitMQPrefetch:             10,
		ProcessedClickEventRetention: 7 * 24 * time.Hour,
	}

	tests := []struct {
		name            string
		env             map[string]string
		want            config.WorkerConfig
		wantErrContains string
	}{
		{name: "uses default values without API settings", env: validEnv(nil), want: defaults},
		{
			name: "loads provided configuration",
			env:  validEnv(map[string]string{"RABBITMQ_PREFETCH": "25", "PROCESSED_CLICK_EVENT_RETENTION": "72h"}),
			want: config.WorkerConfig{
				DatabaseURL:                  "postgres://localhost/app",
				RabbitMQURL:                  "amqp://user:password@localhost:5672/",
				RabbitMQPrefetch:             25,
				ProcessedClickEventRetention: 72 * time.Hour,
			},
		},
		{name: "rejects invalid prefetch", env: validEnv(map[string]string{"RABBITMQ_PREFETCH": "many"}), wantErrContains: "RABBITMQ_PREFETCH must be a valid integer"},
		{name: "rejects zero prefetch", env: validEnv(map[string]string{"RABBITMQ_PREFETCH": "0"}), wantErrContains: "RABBITMQ_PREFETCH must be between 1 and 65535"},
		{name: "rejects prefetch above the AMQP limit", env: validEnv(map[string]string{"RABBITMQ_PREFETCH": "65536"}), wantErrContains: "RABBITMQ_PREFETCH must be between 1 and 65535"},
		{name: "rejects invalid retention", env: validEnv(map[string]string{"PROCESSED_CLICK_EVENT_RETENTION": "forever"}), wantErrContains: "PROCESSED_CLICK_EVENT_RETENTION must be a valid duration"},
		{name: "rejects zero retention", env: validEnv(map[string]string{"PROCESSED_CLICK_EVENT_RETENTION": "0s"}), wantErrContains: "PROCESSED_CLICK_EVENT_RETENTION must be greater than zero"},
		{name: "rejects missing database URL", env: validEnv(map[string]string{"DATABASE_URL": ""}), wantErrContains: "DATABASE_URL is required"},
		{name: "rejects missing RabbitMQ URL", env: validEnv(map[string]string{"RABBITMQ_URL": ""}), wantErrContains: "RABBITMQ_URL is required"},
		{name: "rejects RabbitMQ URL with another scheme", env: validEnv(map[string]string{"RABBITMQ_URL": "http://localhost:5672/"}), wantErrContains: "RABBITMQ_URL must use the amqp or amqps scheme"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := config.LoadWorker(func(key string) string { return tt.env[key] })

			if tt.wantErrContains != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErrContains) {
					t.Fatalf("LoadWorker() error = %v, want error containing %q", err, tt.wantErrContains)
				}
				return
			}

			if err != nil {
				t.Fatalf("LoadWorker() unexpected error: %v", err)
			}
			if got != tt.want {
				t.Errorf("LoadWorker() = %#v, want %#v", got, tt.want)
			}
		})
	}
}
