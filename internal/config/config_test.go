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
			},
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
