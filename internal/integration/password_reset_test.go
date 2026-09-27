//go:build integration

package integration_test

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/fredsaggio/url-shortener/internal/argon2"
	"github.com/fredsaggio/url-shortener/internal/db/dbtest"
	"github.com/fredsaggio/url-shortener/internal/handlers"
	"github.com/fredsaggio/url-shortener/internal/middleware"
	"github.com/fredsaggio/url-shortener/internal/repositories"
	"github.com/fredsaggio/url-shortener/internal/server"
	"github.com/fredsaggio/url-shortener/internal/services"
	"github.com/fredsaggio/url-shortener/internal/sessiontoken"
)

type recordingPasswordResetSender struct {
	codes   []string
	notices []string
}

func (s *recordingPasswordResetSender) SendPasswordResetCode(_ context.Context, _ string, code string) error {
	s.codes = append(s.codes, code)
	return nil
}

func (s *recordingPasswordResetSender) SendPasswordChangedNotice(_ context.Context, email string) error {
	s.notices = append(s.notices, email)
	return nil
}

func TestPasswordResetIntegration(t *testing.T) {
	const email, oldPassword, newPassword = "reset-flow@example.com", "senha-antiga123", "senha-nova456"
	pool := dbtest.Open(t)
	createConfirmedPasswordUser(t, pool, email, oldPassword)
	hasher := argon2.Argon2id{}
	userRepo := repositories.NewUserRepository(pool)
	if _, err := userRepo.CreateWithIdentity(t.Context(), "google-only-reset@example.com", "google", "reset-google-subject"); err != nil {
		t.Fatalf("create Google-only account: %v", err)
	}
	sessionRepo := repositories.NewUserSessionRepository(pool)
	authService := services.NewAuthService(userRepo, sessionRepo, hasher, sessiontoken.Generate, 24*time.Hour, 720*time.Hour)
	sender := &recordingPasswordResetSender{}
	resetService := services.NewPasswordResetService(userRepo, hasher, sender, sessiontoken.Generate, func() (string, error) { return "12345678", nil },
		services.PasswordResetConfig{CodeTTL: 10 * time.Minute, AttemptTTL: 30 * time.Minute})
	allowAll := middleware.NewRateLimiter(1_000, 100, 100, time.Minute)
	router := server.NewServer(&server.Handlers{
		UserHandler:          &handlers.UserHandler{},
		SessionHandler:       handlers.NewSessionHandler(authService, allowAll, false),
		PasswordResetHandler: handlers.NewPasswordResetHandler(resetService, allowAll, false),
		CreateURLHandler:     http.NotFoundHandler(),
		ListURLHandler:       http.NotFoundHandler(),
		MeHandler:            middleware.Authenticator(authService)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) })),
	}, pool).NewRouterHTTP()

	login := performJSONRequest(t, router, http.MethodPost, "/sessions", `{"email":"reset-flow@example.com","password":"senha-antiga123"}`)
	defer login.Body.Close()
	if login.StatusCode != http.StatusNoContent {
		t.Fatalf("initial login status = %d", login.StatusCode)
	}
	sessionCookie := requireCookie(t, login.Cookies(), "user_session")

	unknown := performJSONRequest(t, router, http.MethodPost, "/password-resets", `{"email":"unknown-reset@example.com"}`)
	defer unknown.Body.Close()
	if unknown.StatusCode != http.StatusAccepted || len(sender.codes) != 0 {
		t.Fatalf("unknown account response = %d, sent codes = %d", unknown.StatusCode, len(sender.codes))
	}
	unknownCookie := requireCookie(t, unknown.Cookies(), "password_reset")
	if !unknownCookie.HttpOnly {
		t.Error("unknown account response omitted HttpOnly cookie")
	}
	googleOnly := performJSONRequest(t, router, http.MethodPost, "/password-resets", `{"email":"google-only-reset@example.com"}`)
	defer googleOnly.Body.Close()
	if googleOnly.StatusCode != http.StatusAccepted || len(sender.codes) != 0 {
		t.Fatalf("Google-only account response = %d, sent codes = %d", googleOnly.StatusCode, len(sender.codes))
	}
	requireCookie(t, googleOnly.Cookies(), "password_reset")

	start := performJSONRequest(t, router, http.MethodPost, "/password-resets", `{"email":"reset-flow@example.com"}`)
	defer start.Body.Close()
	if start.StatusCode != http.StatusAccepted || len(sender.codes) != 1 {
		t.Fatalf("start response = %d, sent codes = %d", start.StatusCode, len(sender.codes))
	}
	resetCookie := requireCookie(t, start.Cookies(), "password_reset")
	if len(sender.codes[0]) != 8 {
		t.Fatalf("sent code length = %d, want 8", len(sender.codes[0]))
	}

	wrong := performJSONRequestWithCookie(t, router, http.MethodPost, "/password-resets/confirm",
		`{"code":"00000000","password":"senha-nova456","password_confirmation":"senha-nova456"}`, resetCookie)
	defer wrong.Body.Close()
	if wrong.StatusCode != http.StatusBadRequest {
		t.Fatalf("wrong code status = %d, want 400", wrong.StatusCode)
	}
	locked := performJSONRequestWithCookie(t, router, http.MethodPost, "/password-resets/confirm",
		`{"code":"`+sender.codes[0]+`","password":"senha-nova456","password_confirmation":"senha-nova456"}`, resetCookie)
	defer locked.Body.Close()
	if locked.StatusCode != http.StatusTooManyRequests {
		t.Fatalf("confirm during cooldown status = %d, want 429", locked.StatusCode)
	}
	if _, err := pool.Exec(t.Context(), `UPDATE password_reset_attempts SET locked_until = NULL WHERE token_hash = $1`, sessiontoken.Hash(resetCookie.Value)); err != nil {
		t.Fatalf("end test cooldown: %v", err)
	}

	confirmBody := `{"code":"` + sender.codes[0] + `","password":"senha-nova456","password_confirmation":"senha-nova456"}`
	confirm := performJSONRequestWithCookie(t, router, http.MethodPost, "/password-resets/confirm", confirmBody, resetCookie)
	defer confirm.Body.Close()
	if confirm.StatusCode != http.StatusNoContent {
		t.Fatalf("confirm status = %d, want 204; body = %q", confirm.StatusCode, readResponseBody(t, confirm))
	}
	if len(sender.notices) != 1 || sender.notices[0] != email {
		t.Errorf("password changed notices = %v, want [%q]", sender.notices, email)
	}
	clearedCookie := requireCookie(t, confirm.Cookies(), "password_reset")
	if clearedCookie.MaxAge != -1 {
		t.Error("confirmation did not clear reset cookie")
	}
	if got := countUserSessions(t, pool, email); got != 0 {
		t.Errorf("sessions after reset = %d, want 0", got)
	}

	oldSession := performRequestWithCookie(t, router, http.MethodGet, "/me", sessionCookie)
	defer oldSession.Body.Close()
	if oldSession.StatusCode != http.StatusUnauthorized {
		t.Errorf("old session status = %d, want 401", oldSession.StatusCode)
	}
	oldLogin := performJSONRequest(t, router, http.MethodPost, "/sessions", `{"email":"reset-flow@example.com","password":"senha-antiga123"}`)
	defer oldLogin.Body.Close()
	if oldLogin.StatusCode != http.StatusUnauthorized {
		t.Errorf("old password login status = %d, want 401", oldLogin.StatusCode)
	}
	newLogin := performJSONRequest(t, router, http.MethodPost, "/sessions", `{"email":"reset-flow@example.com","password":"senha-nova456"}`)
	defer newLogin.Body.Close()
	if newLogin.StatusCode != http.StatusNoContent {
		t.Errorf("new password login status = %d, want 204", newLogin.StatusCode)
	}
	replay := performJSONRequestWithCookie(t, router, http.MethodPost, "/password-resets/confirm", confirmBody, resetCookie)
	defer replay.Body.Close()
	if replay.StatusCode != http.StatusGone {
		t.Errorf("replayed code status = %d, want 410", replay.StatusCode)
	}
}
