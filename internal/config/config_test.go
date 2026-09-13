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
				"DATABASE_URL":     "postgres://user:password@localhost:5432/app",
				"HTTP_ADDR":        ":9090",
				"USER_SESSION_TTL": "12h",
				"COOKIE_SECURE":    "false",
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
					TTL:          12 * time.Hour,
					CookieSecure: false,
				},
			},
		},
		{
			name: "uses default values",
			env: map[string]string{
				"DATABASE_URL": "postgres://localhost/app",
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
					TTL:          24 * time.Hour,
					CookieSecure: true,
				},
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
			name: "rejects invalid cookie secure value",
			env: map[string]string{
				"DATABASE_URL":  "postgres://localhost/app",
				"COOKIE_SECURE": "sometimes",
			},
			wantErrContains: "COOKIE_SECURE must be a valid boolean",
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
