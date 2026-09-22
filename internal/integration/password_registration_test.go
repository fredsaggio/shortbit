//go:build integration

package integration_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
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
	"github.com/fredsaggio/url-shortener/internal/verificationcode"
	"github.com/jackc/pgx/v5/pgxpool"
)

type sentPasswordRegistrationCode struct {
	email string
	code  string
}

type recordingPasswordRegistrationCodeSender struct {
	sent []sentPasswordRegistrationCode
}

func (s *recordingPasswordRegistrationCodeSender) SendPasswordRegistrationCode(_ context.Context, email, code string) error {
	s.sent = append(s.sent, sentPasswordRegistrationCode{email: email, code: code})
	return nil
}

func TestPasswordRegistrationResendIntegration(t *testing.T) {
	const (
		email      = "registration-resend-integration@example.com"
		password   = "senha12345"
		firstCode  = "123456"
		secondCode = "654321"
	)

	pool := dbtest.Open(t)
	codeSender := &recordingPasswordRegistrationCodeSender{}
	codeGenerator := verificationCodeSequence(t, firstCode, secondCode)
	router := passwordRegistrationTestRouter(pool, codeSender, codeGenerator)

	startResponse := performJSONRequest(
		t,
		router,
		http.MethodPost,
		"/registrations/password",
		`{"email":"registration-resend-integration@example.com","password":"senha12345"}`,
	)
	defer startResponse.Body.Close()

	if startResponse.StatusCode != http.StatusAccepted {
		t.Fatalf("start status code = %d, want %d; body = %q", startResponse.StatusCode, http.StatusAccepted, readResponseBody(t, startResponse))
	}

	registrationCookie := requireCookie(t, startResponse.Cookies(), "password_registration")
	if registrationCookie.Value == "" {
		t.Fatal("password registration cookie has an empty token")
	}
	if len(codeSender.sent) != 1 {
		t.Fatalf("sent codes after starting registration = %d, want 1", len(codeSender.sent))
	}
	if codeSender.sent[0] != (sentPasswordRegistrationCode{email: email, code: firstCode}) {
		t.Errorf("first sent code = %+v, want email %q and code %q", codeSender.sent[0], email, firstCode)
	}

	immediateResendResponse := performRequestWithCookie(t, router, http.MethodPost, "/registrations/password/resend", registrationCookie)
	defer immediateResendResponse.Body.Close()

	if immediateResendResponse.StatusCode != http.StatusTooManyRequests {
		t.Fatalf("immediate resend status code = %d, want %d; body = %q", immediateResendResponse.StatusCode, http.StatusTooManyRequests, readResponseBody(t, immediateResendResponse))
	}
	if len(codeSender.sent) != 1 {
		t.Errorf("sent codes after rejected resend = %d, want 1", len(codeSender.sent))
	}

	tokenHash := sessiontoken.Hash(registrationCookie.Value)
	if _, err := pool.Exec(
		t.Context(),
		`UPDATE password_registration_attempts SET last_code_sent_at = NOW() - INTERVAL '31 seconds' WHERE token_hash = $1`,
		tokenHash,
	); err != nil {
		t.Fatalf("move registration resend cooldown into the past: %v", err)
	}

	resendResponse := performRequestWithCookie(t, router, http.MethodPost, "/registrations/password/resend", registrationCookie)
	defer resendResponse.Body.Close()

	if resendResponse.StatusCode != http.StatusAccepted {
		t.Fatalf("resend status code = %d, want %d; body = %q", resendResponse.StatusCode, http.StatusAccepted, readResponseBody(t, resendResponse))
	}
	if len(codeSender.sent) != 2 {
		t.Fatalf("sent codes after accepted resend = %d, want 2", len(codeSender.sent))
	}
	if codeSender.sent[1] != (sentPasswordRegistrationCode{email: email, code: secondCode}) {
		t.Errorf("second sent code = %+v, want email %q and code %q", codeSender.sent[1], email, secondCode)
	}

	var storedProofHash []byte
	if err := pool.QueryRow(
		t.Context(),
		"SELECT verification_proof_hash FROM password_registration_attempts WHERE token_hash = $1",
		tokenHash,
	).Scan(&storedProofHash); err != nil {
		t.Fatalf("query registration proof after resend: %v", err)
	}
	if !bytes.Equal(storedProofHash, verificationcode.Proof(registrationCookie.Value, secondCode)) {
		t.Error("stored proof does not match the resent code")
	}
	if bytes.Equal(storedProofHash, verificationcode.Proof(registrationCookie.Value, firstCode)) {
		t.Error("stored proof still matches the original code")
	}

	confirmResponse := performJSONRequestWithCookie(
		t,
		router,
		http.MethodPost,
		"/registrations/password/confirm",
		`{"code":"654321"}`,
		registrationCookie,
	)
	defer confirmResponse.Body.Close()

	if confirmResponse.StatusCode != http.StatusCreated {
		t.Fatalf("confirm status code = %d, want %d; body = %q", confirmResponse.StatusCode, http.StatusCreated, readResponseBody(t, confirmResponse))
	}

	var createdUser struct {
		ID    string `json:"id"`
		Email string `json:"email"`
	}
	if err := json.NewDecoder(confirmResponse.Body).Decode(&createdUser); err != nil {
		t.Fatalf("decode confirmed user: %v", err)
	}
	if createdUser.ID == "" {
		t.Error("confirmed user has an empty ID")
	}
	if createdUser.Email != email {
		t.Errorf("confirmed user email = %q, want %q", createdUser.Email, email)
	}

	var accountExists bool
	if err := pool.QueryRow(
		t.Context(),
		`
			SELECT EXISTS (
				SELECT 1
				FROM users AS u
				JOIN password_credentials AS pc ON pc.user_id = u.id
				WHERE u.email = $1 AND u.email_verified_at IS NOT NULL
			)
		`,
		email,
	).Scan(&accountExists); err != nil {
		t.Fatalf("check confirmed password account: %v", err)
	}
	if !accountExists {
		t.Error("confirmed user and password credential were not stored")
	}

	clearedCookie := requireCookie(t, confirmResponse.Cookies(), "password_registration")
	if clearedCookie.Value != "" || clearedCookie.MaxAge != -1 {
		t.Errorf("confirmation did not clear registration cookie: value = %q, MaxAge = %d", clearedCookie.Value, clearedCookie.MaxAge)
	}
}

func verificationCodeSequence(t *testing.T, codes ...string) services.VerificationCodeGenerator {
	t.Helper()

	next := 0
	return func() (string, error) {
		if next >= len(codes) {
			return "", errors.New("verification code sequence exhausted")
		}

		code := codes[next]
		next++
		return code, nil
	}
}

func passwordRegistrationTestRouter(pool *pgxpool.Pool, codeSender services.PasswordRegistrationCodeSender, generateCode services.VerificationCodeGenerator) http.Handler {
	passwordHasher := argon2.Argon2id{}
	userRepository := repositories.NewUserRepository(pool)
	userService := services.NewUserService(
		userRepository,
		passwordHasher,
		sessiontoken.Generate,
		generateCode,
		codeSender,
		services.PasswordRegistrationConfig{CodeTTL: 10 * time.Minute, AttemptTTL: 30 * time.Minute},
	)
	registrationEmailLimiter := middleware.NewRateLimiter(1_000, 100, 100, time.Minute)
	userHandler := handlers.NewUserHandler(userService, registrationEmailLimiter, false)

	sessionRepository := repositories.NewUserSessionRepository(pool)
	authService := services.NewAuthService(userRepository, sessionRepository, passwordHasher, sessiontoken.Generate, 24*time.Hour)
	loginLimiter := middleware.NewRateLimiter(1, 5, 100, time.Minute)
	sessionHandler := handlers.NewSessionHandler(authService, loginLimiter, false)
	meHandler := middleware.Authenticator(authService)(http.HandlerFunc(userHandler.GetUserInfo))

	applicationHandlers := &server.Handlers{
		UserHandler:    userHandler,
		SessionHandler: sessionHandler,
		MeHandler:      meHandler,
	}

	return server.NewServer(applicationHandlers, pool).NewRouterHTTP()
}

func performJSONRequestWithCookie(t *testing.T, handler http.Handler, method, target, body string, cookie *http.Cookie) *http.Response {
	t.Helper()

	request := httptest.NewRequest(method, target, strings.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	request.AddCookie(cookie)

	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	return response.Result()
}
