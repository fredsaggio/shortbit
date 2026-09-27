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
	"sync"
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

func (s *recordingPasswordRegistrationCodeSender) SendPasswordResetCode(context.Context, string, string) error {
	return nil
}

func (s *recordingPasswordRegistrationCodeSender) SendPasswordChangedNotice(context.Context, string) error {
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

func TestConcurrentPasswordRegistrationConfirmationsForSameEmail(t *testing.T) {
	const email = "registration-concurrency@example.com"

	attempts := []struct {
		password string
		code     string
		body     string
	}{
		{password: "senha-concorrente-um", code: "111111", body: `{"email":"registration-concurrency@example.com","password":"senha-concorrente-um"}`},
		{password: "senha-concorrente-dois", code: "222222", body: `{"email":"registration-concurrency@example.com","password":"senha-concorrente-dois"}`},
	}

	pool := dbtest.Open(t)
	codeSender := &recordingPasswordRegistrationCodeSender{}
	codeGenerator := verificationCodeSequence(t, attempts[0].code, attempts[1].code)
	router := passwordRegistrationTestRouter(pool, codeSender, codeGenerator)

	cookies := make([]*http.Cookie, len(attempts))
	for index, attempt := range attempts {
		response := performJSONRequest(t, router, http.MethodPost, "/registrations/password", attempt.body)
		if response.StatusCode != http.StatusAccepted {
			body := readResponseBody(t, response)
			response.Body.Close()
			t.Fatalf("start attempt %d status code = %d, want %d; body = %q", index, response.StatusCode, http.StatusAccepted, body)
		}

		cookies[index] = requireCookie(t, response.Cookies(), "password_registration")
		response.Body.Close()
	}

	if len(codeSender.sent) != 2 {
		t.Fatalf("sent registration codes = %d, want 2", len(codeSender.sent))
	}

	type confirmationResult struct {
		attemptIndex int
		statusCode   int
	}

	start := make(chan struct{})
	results := make(chan confirmationResult, len(attempts))
	var waitGroup sync.WaitGroup

	for index, attempt := range attempts {
		waitGroup.Add(1)

		go func() {
			defer waitGroup.Done()
			<-start

			request := httptest.NewRequest(
				http.MethodPost,
				"/registrations/password/confirm",
				strings.NewReader(`{"code":"`+attempt.code+`"}`),
			)
			request.Header.Set("Content-Type", "application/json")
			request.AddCookie(cookies[index])

			response := httptest.NewRecorder()
			router.ServeHTTP(response, request)

			results <- confirmationResult{attemptIndex: index, statusCode: response.Code}
		}()
	}

	close(start)
	waitGroup.Wait()
	close(results)

	winnerIndex := -1
	createdCount := 0
	conflictCount := 0

	for result := range results {
		switch result.statusCode {
		case http.StatusCreated:
			createdCount++
			winnerIndex = result.attemptIndex
		case http.StatusConflict:
			conflictCount++
		default:
			t.Errorf("attempt %d confirmation status code = %d, want %d or %d", result.attemptIndex, result.statusCode, http.StatusCreated, http.StatusConflict)
		}
	}

	if createdCount != 1 {
		t.Errorf("created confirmations = %d, want 1", createdCount)
	}
	if conflictCount != 1 {
		t.Errorf("conflicting confirmations = %d, want 1", conflictCount)
	}
	if winnerIndex == -1 {
		t.Fatal("no registration attempt won the concurrent confirmation")
	}

	var userCount, credentialCount int
	if err := pool.QueryRow(
		t.Context(),
		`
			SELECT COUNT(DISTINCT u.id), COUNT(pc.user_id)
			FROM users AS u
			LEFT JOIN password_credentials AS pc ON pc.user_id = u.id
			WHERE u.email = $1
		`,
		email,
	).Scan(&userCount, &credentialCount); err != nil {
		t.Fatalf("count account rows after concurrent confirmation: %v", err)
	}

	if userCount != 1 {
		t.Errorf("stored users = %d, want 1", userCount)
	}
	if credentialCount != 1 {
		t.Errorf("stored password credentials = %d, want 1", credentialCount)
	}

	var storedPasswordHash string
	if err := pool.QueryRow(
		t.Context(),
		`
			SELECT pc.password_hash
			FROM users AS u
			JOIN password_credentials AS pc ON pc.user_id = u.id
			WHERE u.email = $1
		`,
		email,
	).Scan(&storedPasswordHash); err != nil {
		t.Fatalf("query winning password credential: %v", err)
	}

	passwordHasher := argon2.Argon2id{}
	winnerMatches, err := passwordHasher.Compare(attempts[winnerIndex].password, storedPasswordHash)
	if err != nil {
		t.Fatalf("compare winning password: %v", err)
	}
	if !winnerMatches {
		t.Error("stored credential does not match the winning attempt password")
	}

	loserIndex := 1 - winnerIndex
	loserMatches, err := passwordHasher.Compare(attempts[loserIndex].password, storedPasswordHash)
	if err != nil {
		t.Fatalf("compare losing password: %v", err)
	}
	if loserMatches {
		t.Error("losing attempt overwrote the winning password credential")
	}
}

func TestRepeatedPasswordRegistrationConfirmationDoesNotCreateAnotherAccount(t *testing.T) {
	const (
		email    = "registration-repeated-confirmation@example.com"
		password = "senha-confirmacao-repetida"
		code     = "333333"
	)

	pool := dbtest.Open(t)
	codeSender := &recordingPasswordRegistrationCodeSender{}
	router := passwordRegistrationTestRouter(pool, codeSender, verificationCodeSequence(t, code))

	startResponse := performJSONRequest(
		t,
		router,
		http.MethodPost,
		"/registrations/password",
		`{"email":"registration-repeated-confirmation@example.com","password":"senha-confirmacao-repetida"}`,
	)
	if startResponse.StatusCode != http.StatusAccepted {
		body := readResponseBody(t, startResponse)
		startResponse.Body.Close()
		t.Fatalf("start status code = %d, want %d; body = %q", startResponse.StatusCode, http.StatusAccepted, body)
	}

	registrationCookie := requireCookie(t, startResponse.Cookies(), "password_registration")
	startResponse.Body.Close()

	firstConfirmation := performJSONRequestWithCookie(
		t,
		router,
		http.MethodPost,
		"/registrations/password/confirm",
		`{"code":"333333"}`,
		registrationCookie,
	)
	if firstConfirmation.StatusCode != http.StatusCreated {
		body := readResponseBody(t, firstConfirmation)
		firstConfirmation.Body.Close()
		t.Fatalf("first confirmation status code = %d, want %d; body = %q", firstConfirmation.StatusCode, http.StatusCreated, body)
	}
	firstConfirmation.Body.Close()

	secondConfirmation := performJSONRequestWithCookie(
		t,
		router,
		http.MethodPost,
		"/registrations/password/confirm",
		`{"code":"333333"}`,
		registrationCookie,
	)
	defer secondConfirmation.Body.Close()

	if secondConfirmation.StatusCode != http.StatusConflict {
		t.Fatalf("repeated confirmation status code = %d, want %d; body = %q", secondConfirmation.StatusCode, http.StatusConflict, readResponseBody(t, secondConfirmation))
	}

	clearedCookie := requireCookie(t, secondConfirmation.Cookies(), "password_registration")
	if clearedCookie.Value != "" || clearedCookie.MaxAge != -1 {
		t.Errorf("repeated confirmation did not clear registration cookie: value = %q, MaxAge = %d", clearedCookie.Value, clearedCookie.MaxAge)
	}

	var userCount, credentialCount int
	if err := pool.QueryRow(
		t.Context(),
		`
			SELECT COUNT(DISTINCT u.id), COUNT(pc.user_id)
			FROM users AS u
			LEFT JOIN password_credentials AS pc ON pc.user_id = u.id
			WHERE u.email = $1
		`,
		email,
	).Scan(&userCount, &credentialCount); err != nil {
		t.Fatalf("count account rows after repeated confirmation: %v", err)
	}

	if userCount != 1 {
		t.Errorf("stored users = %d, want 1", userCount)
	}
	if credentialCount != 1 {
		t.Errorf("stored password credentials = %d, want 1", credentialCount)
	}

	var storedPasswordHash string
	if err := pool.QueryRow(
		t.Context(),
		`
			SELECT pc.password_hash
			FROM users AS u
			JOIN password_credentials AS pc ON pc.user_id = u.id
			WHERE u.email = $1
		`,
		email,
	).Scan(&storedPasswordHash); err != nil {
		t.Fatalf("query password credential after repeated confirmation: %v", err)
	}

	passwordMatches, err := (argon2.Argon2id{}).Compare(password, storedPasswordHash)
	if err != nil {
		t.Fatalf("compare password after repeated confirmation: %v", err)
	}
	if !passwordMatches {
		t.Error("repeated confirmation changed the original password credential")
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

type registrationAndResetCodeSender interface {
	services.PasswordRegistrationCodeSender
	services.PasswordResetCodeSender
}

func passwordRegistrationTestRouter(pool *pgxpool.Pool, codeSender registrationAndResetCodeSender, generateCode services.VerificationCodeGenerator) http.Handler {
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
	resetService := services.NewPasswordResetService(userRepository, passwordHasher, codeSender, sessiontoken.Generate, verificationcode.GeneratePasswordReset,
		services.PasswordResetConfig{CodeTTL: 10 * time.Minute, AttemptTTL: 30 * time.Minute})
	resetHandler := handlers.NewPasswordResetHandler(resetService, registrationEmailLimiter, false)

	sessionRepository := repositories.NewUserSessionRepository(pool)
	authService := services.NewAuthService(userRepository, sessionRepository, passwordHasher, sessiontoken.Generate, 24*time.Hour, 720*time.Hour)
	loginLimiter := middleware.NewRateLimiter(1, 5, 100, time.Minute)
	sessionHandler := handlers.NewSessionHandler(authService, loginLimiter, false)
	meHandler := middleware.Authenticator(authService)(http.HandlerFunc(userHandler.GetUserInfo))

	applicationHandlers := &server.Handlers{
		UserHandler:          userHandler,
		SessionHandler:       sessionHandler,
		PasswordResetHandler: resetHandler,
		CreateURLHandler:     http.NotFoundHandler(),
		ListURLHandler:       http.NotFoundHandler(),
		MeHandler:            meHandler,
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
