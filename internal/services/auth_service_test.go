package services_test

import (
	"bytes"
	"context"
	"errors"
	"testing"
	"time"
	"uuid"

	"github.com/fredsaggio/url-shortener/internal/models"
	"github.com/fredsaggio/url-shortener/internal/repositories"
	"github.com/fredsaggio/url-shortener/internal/services"
	"github.com/fredsaggio/url-shortener/internal/sessiontoken"
)

type passwordCredentialRepositoryStub struct {
	findPasswordCredentialsByEmailFunc func(ctx context.Context, email string) (models.User, models.PasswordCredential, error)
}

func (s passwordCredentialRepositoryStub) FindPasswordCredentialsByEmail(ctx context.Context, email string) (models.User, models.PasswordCredential, error) {
	return s.findPasswordCredentialsByEmailFunc(ctx, email)
}

type userSessionRepositoryStub struct {
	createFunc                 func(ctx context.Context, userID uuid.UUID, tokenHash []byte, expiresAt time.Time) error
	findSessionByTokenHashFunc func(ctx context.Context, tokenHash []byte) (models.UserSession, error)
}

func (s userSessionRepositoryStub) Create(ctx context.Context, userID uuid.UUID, tokenHash []byte, expiresAt time.Time) error {
	return s.createFunc(ctx, userID, tokenHash, expiresAt)
}

func (s userSessionRepositoryStub) FindSessionByTokenHash(ctx context.Context, tokenHash []byte) (models.UserSession, error) {
	return s.findSessionByTokenHashFunc(ctx, tokenHash)
}

type passwordComparatorStub struct {
	compareFunc func(password, encodedHash string) (bool, error)
}

func (s passwordComparatorStub) Compare(password, encodedHash string) (bool, error) {
	return s.compareFunc(password, encodedHash)
}

func TestAuthServiceLogin(t *testing.T) {
	const (
		inputEmail      = "  USER@Example.COM  "
		normalizedEmail = "user@example.com"
		inputPassword   = "senha-segura"
		passwordHash    = "$argon2id$test-hash"
		rawToken        = "raw-session-token"
		contextValue    = "request-context"
		sessionTTL      = 24 * time.Hour
	)

	tokenHash := []byte("hashed-session-token")
	userID := uuid.MustParse("01991f29-7c22-7ab3-a395-4d402f09c317")
	type contextKey struct{}
	ctx := context.WithValue(context.Background(), contextKey{}, contextValue)

	userRepository := passwordCredentialRepositoryStub{
		findPasswordCredentialsByEmailFunc: func(gotCtx context.Context, gotEmail string) (models.User, models.PasswordCredential, error) {
			if got := gotCtx.Value(contextKey{}); got != contextValue {
				t.Errorf("FindPasswordCredentialsByEmail() context value = %v, want %q", got, contextValue)
			}

			if gotEmail != normalizedEmail {
				t.Errorf("FindPasswordCredentialsByEmail() email = %q, want %q", gotEmail, normalizedEmail)
			}

			return models.User{ID: userID, Email: normalizedEmail}, models.PasswordCredential{UserID: userID, PasswordHash: passwordHash}, nil
		},
	}

	comparator := passwordComparatorStub{
		compareFunc: func(gotPassword, gotHash string) (bool, error) {
			if gotPassword != inputPassword {
				t.Errorf("Compare() password = %q, want %q", gotPassword, inputPassword)
			}

			if gotHash != passwordHash {
				t.Errorf("Compare() encoded hash = %q, want %q", gotHash, passwordHash)
			}

			return true, nil
		},
	}

	generateToken := func() (string, []byte, error) {
		return rawToken, tokenHash, nil
	}

	var persistedExpiresAt time.Time
	sessionRepository := userSessionRepositoryStub{
		createFunc: func(gotCtx context.Context, gotUserID uuid.UUID, gotTokenHash []byte, expiresAt time.Time) error {
			if got := gotCtx.Value(contextKey{}); got != contextValue {
				t.Errorf("Create() context value = %v, want %q", got, contextValue)
			}

			if gotUserID != userID {
				t.Errorf("Create() user ID = %v, want %v", gotUserID, userID)
			}

			if !bytes.Equal(gotTokenHash, tokenHash) {
				t.Errorf("Create() token hash = %q, want %q", gotTokenHash, tokenHash)
			}

			if bytes.Equal(gotTokenHash, []byte(rawToken)) {
				t.Error("Create() received the raw session token")
			}

			persistedExpiresAt = expiresAt
			return nil
		},
	}

	service := services.NewAuthService(userRepository, sessionRepository, comparator, generateToken, sessionTTL)
	beforeLogin := time.Now().UTC()
	result, err := service.Login(ctx, inputEmail, inputPassword)
	afterLogin := time.Now().UTC()

	if err != nil {
		t.Fatalf("Login() error = %v", err)
	}

	if result.Token != rawToken {
		t.Errorf("Login() token = %q, want %q", result.Token, rawToken)
	}

	if result.ExpiresAt != persistedExpiresAt {
		t.Errorf("Login() expiration = %v, persisted expiration = %v", result.ExpiresAt, persistedExpiresAt)
	}

	if result.ExpiresAt.Before(beforeLogin.Add(sessionTTL)) || result.ExpiresAt.After(afterLogin.Add(sessionTTL)) {
		t.Errorf("Login() expiration = %v, want between %v and %v", result.ExpiresAt, beforeLogin.Add(sessionTTL), afterLogin.Add(sessionTTL))
	}
}

func TestAuthServiceLoginRejectsEmptyCredentials(t *testing.T) {
	tests := []struct {
		name     string
		email    string
		password string
	}{
		{name: "empty email", email: "", password: "senha-segura"},
		{name: "email with spaces", email: "   ", password: "senha-segura"},
		{name: "empty password", email: "user@example.com", password: ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			userRepository := passwordCredentialRepositoryStub{
				findPasswordCredentialsByEmailFunc: func(context.Context, string) (models.User, models.PasswordCredential, error) {
					t.Fatal("FindPasswordCredentialsByEmail() should not be called")
					return models.User{}, models.PasswordCredential{}, nil
				},
			}

			service := services.NewAuthService(userRepository, unexpectedSessionRepository(t), unexpectedComparator(t), unexpectedTokenGenerator(t), time.Hour)
			_, err := service.Login(context.Background(), tt.email, tt.password)

			if !errors.Is(err, services.ErrIncorrectEmailOrPassword) {
				t.Fatalf("Login() error = %v, want %v", err, services.ErrIncorrectEmailOrPassword)
			}
		})
	}
}

func TestAuthServiceLoginTranslatesMissingCredential(t *testing.T) {
	userRepository := passwordCredentialRepositoryStub{
		findPasswordCredentialsByEmailFunc: func(context.Context, string) (models.User, models.PasswordCredential, error) {
			return models.User{}, models.PasswordCredential{}, repositories.ErrPasswordCredentialNotFound
		},
	}

	service := services.NewAuthService(userRepository, unexpectedSessionRepository(t), unexpectedComparator(t), unexpectedTokenGenerator(t), time.Hour)
	_, err := service.Login(context.Background(), "user@example.com", "senha-segura")

	if !errors.Is(err, services.ErrIncorrectEmailOrPassword) {
		t.Fatalf("Login() error = %v, want %v", err, services.ErrIncorrectEmailOrPassword)
	}
}

func TestAuthServiceLoginPropagatesCredentialRepositoryError(t *testing.T) {
	wantErr := errors.New("database unavailable")
	userRepository := passwordCredentialRepositoryStub{
		findPasswordCredentialsByEmailFunc: func(context.Context, string) (models.User, models.PasswordCredential, error) {
			return models.User{}, models.PasswordCredential{}, wantErr
		},
	}

	service := services.NewAuthService(userRepository, unexpectedSessionRepository(t), unexpectedComparator(t), unexpectedTokenGenerator(t), time.Hour)
	_, err := service.Login(context.Background(), "user@example.com", "senha-segura")

	if !errors.Is(err, wantErr) {
		t.Fatalf("Login() error = %v, want wrapped %v", err, wantErr)
	}
}

func TestAuthServiceLoginPropagatesComparatorError(t *testing.T) {
	wantErr := errors.New("invalid encoded password hash")
	userRepository := validPasswordCredentialRepository()
	comparator := passwordComparatorStub{
		compareFunc: func(string, string) (bool, error) {
			return false, wantErr
		},
	}

	service := services.NewAuthService(userRepository, unexpectedSessionRepository(t), comparator, unexpectedTokenGenerator(t), time.Hour)
	_, err := service.Login(context.Background(), "user@example.com", "senha-segura")

	if !errors.Is(err, wantErr) {
		t.Fatalf("Login() error = %v, want wrapped %v", err, wantErr)
	}
}

func TestAuthServiceLoginRejectsIncorrectPassword(t *testing.T) {
	comparator := passwordComparatorStub{
		compareFunc: func(string, string) (bool, error) {
			return false, nil
		},
	}

	service := services.NewAuthService(validPasswordCredentialRepository(), unexpectedSessionRepository(t), comparator, unexpectedTokenGenerator(t), time.Hour)
	_, err := service.Login(context.Background(), "user@example.com", "senha-incorreta")

	if !errors.Is(err, services.ErrIncorrectEmailOrPassword) {
		t.Fatalf("Login() error = %v, want %v", err, services.ErrIncorrectEmailOrPassword)
	}
}

func TestAuthServiceLoginPropagatesTokenGeneratorError(t *testing.T) {
	wantErr := errors.New("random source unavailable")
	generateToken := func() (string, []byte, error) {
		return "", nil, wantErr
	}

	service := services.NewAuthService(validPasswordCredentialRepository(), unexpectedSessionRepository(t), matchingPasswordComparator(), generateToken, time.Hour)
	_, err := service.Login(context.Background(), "user@example.com", "senha-segura")

	if !errors.Is(err, wantErr) {
		t.Fatalf("Login() error = %v, want wrapped %v", err, wantErr)
	}
}

func TestAuthServiceLoginPropagatesSessionRepositoryError(t *testing.T) {
	wantErr := errors.New("database unavailable")
	sessionRepository := userSessionRepositoryStub{
		createFunc: func(context.Context, uuid.UUID, []byte, time.Time) error {
			return wantErr
		},
	}

	generateToken := func() (string, []byte, error) {
		return "raw-session-token", []byte("hashed-session-token"), nil
	}

	service := services.NewAuthService(validPasswordCredentialRepository(), sessionRepository, matchingPasswordComparator(), generateToken, time.Hour)
	_, err := service.Login(context.Background(), "user@example.com", "senha-segura")

	if !errors.Is(err, wantErr) {
		t.Fatalf("Login() error = %v, want wrapped %v", err, wantErr)
	}
}

func TestAuthServiceAuthenticate(t *testing.T) {
	const (
		rawToken     = "raw-session-token"
		contextValue = "request-context"
	)

	wantUserID := uuid.MustParse("01991f29-7c22-7ab3-a395-4d402f09c317")
	wantTokenHash := sessiontoken.Hash(rawToken)
	type contextKey struct{}
	ctx := context.WithValue(context.Background(), contextKey{}, contextValue)

	sessionRepository := userSessionRepositoryStub{
		findSessionByTokenHashFunc: func(gotCtx context.Context, gotTokenHash []byte) (models.UserSession, error) {
			if got := gotCtx.Value(contextKey{}); got != contextValue {
				t.Errorf("FindSessionByTokenHash() context value = %v, want %q", got, contextValue)
			}

			if !bytes.Equal(gotTokenHash, wantTokenHash) {
				t.Errorf("FindSessionByTokenHash() token hash = %x, want %x", gotTokenHash, wantTokenHash)
			}

			if bytes.Equal(gotTokenHash, []byte(rawToken)) {
				t.Error("FindSessionByTokenHash() received the raw token")
			}

			return models.UserSession{UserID: wantUserID}, nil
		},
	}

	service := services.NewAuthService(unexpectedPasswordCredentialRepository(t), sessionRepository, unexpectedComparator(t), unexpectedTokenGenerator(t), time.Hour)
	userID, err := service.Authenticate(ctx, rawToken)
	if err != nil {
		t.Fatalf("Authenticate() error = %v", err)
	}

	if userID != wantUserID {
		t.Errorf("Authenticate() user ID = %s, want %s", userID, wantUserID)
	}
}

func TestAuthServiceAuthenticateRejectsEmptyToken(t *testing.T) {
	repositoryCalled := false
	sessionRepository := userSessionRepositoryStub{
		findSessionByTokenHashFunc: func(context.Context, []byte) (models.UserSession, error) {
			repositoryCalled = true
			return models.UserSession{}, repositories.ErrUserSessionNotFound
		},
	}

	service := services.NewAuthService(unexpectedPasswordCredentialRepository(t), sessionRepository, unexpectedComparator(t), unexpectedTokenGenerator(t), time.Hour)
	userID, err := service.Authenticate(context.Background(), "")

	if !errors.Is(err, services.ErrUnauthenticated) {
		t.Errorf("Authenticate() error = %v, want %v", err, services.ErrUnauthenticated)
	}

	if userID != uuid.Nil() {
		t.Errorf("Authenticate() user ID = %s, want nil UUID", userID)
	}

	if repositoryCalled {
		t.Error("FindSessionByTokenHash() was called for an empty token")
	}
}

func TestAuthServiceAuthenticateTranslatesMissingSession(t *testing.T) {
	sessionRepository := userSessionRepositoryStub{
		findSessionByTokenHashFunc: func(context.Context, []byte) (models.UserSession, error) {
			return models.UserSession{}, repositories.ErrUserSessionNotFound
		},
	}

	service := services.NewAuthService(unexpectedPasswordCredentialRepository(t), sessionRepository, unexpectedComparator(t), unexpectedTokenGenerator(t), time.Hour)
	userID, err := service.Authenticate(context.Background(), "unknown-session-token")

	if !errors.Is(err, services.ErrUnauthenticated) {
		t.Errorf("Authenticate() error = %v, want %v", err, services.ErrUnauthenticated)
	}

	if userID != uuid.Nil() {
		t.Errorf("Authenticate() user ID = %s, want nil UUID", userID)
	}
}

func TestAuthServiceAuthenticatePropagatesRepositoryError(t *testing.T) {
	wantErr := errors.New("database unavailable")
	sessionRepository := userSessionRepositoryStub{
		findSessionByTokenHashFunc: func(context.Context, []byte) (models.UserSession, error) {
			return models.UserSession{}, wantErr
		},
	}

	service := services.NewAuthService(unexpectedPasswordCredentialRepository(t), sessionRepository, unexpectedComparator(t), unexpectedTokenGenerator(t), time.Hour)
	userID, err := service.Authenticate(context.Background(), "validly-shaped-token")

	if !errors.Is(err, wantErr) {
		t.Errorf("Authenticate() error = %v, want wrapped %v", err, wantErr)
	}

	if errors.Is(err, services.ErrUnauthenticated) {
		t.Errorf("Authenticate() error = %v, do not want %v", err, services.ErrUnauthenticated)
	}

	if userID != uuid.Nil() {
		t.Errorf("Authenticate() user ID = %s, want nil UUID", userID)
	}
}

func validPasswordCredentialRepository() passwordCredentialRepositoryStub {
	userID := uuid.MustParse("01991f29-7c22-7ab3-a395-4d402f09c317")

	return passwordCredentialRepositoryStub{
		findPasswordCredentialsByEmailFunc: func(context.Context, string) (models.User, models.PasswordCredential, error) {
			return models.User{ID: userID, Email: "user@example.com"}, models.PasswordCredential{UserID: userID, PasswordHash: "$argon2id$test-hash"}, nil
		},
	}
}

func unexpectedPasswordCredentialRepository(t *testing.T) passwordCredentialRepositoryStub {
	t.Helper()

	return passwordCredentialRepositoryStub{
		findPasswordCredentialsByEmailFunc: func(context.Context, string) (models.User, models.PasswordCredential, error) {
			t.Fatal("FindPasswordCredentialsByEmail() should not be called")
			return models.User{}, models.PasswordCredential{}, nil
		},
	}
}

func matchingPasswordComparator() passwordComparatorStub {
	return passwordComparatorStub{
		compareFunc: func(string, string) (bool, error) {
			return true, nil
		},
	}
}

func unexpectedSessionRepository(t *testing.T) userSessionRepositoryStub {
	t.Helper()

	return userSessionRepositoryStub{
		createFunc: func(context.Context, uuid.UUID, []byte, time.Time) error {
			t.Fatal("Create() should not be called")
			return nil
		},
		findSessionByTokenHashFunc: func(context.Context, []byte) (models.UserSession, error) {
			t.Fatal("FindSessionByTokenHash() should not be called")
			return models.UserSession{}, nil
		},
	}
}

func unexpectedComparator(t *testing.T) passwordComparatorStub {
	t.Helper()

	return passwordComparatorStub{
		compareFunc: func(string, string) (bool, error) {
			t.Fatal("Compare() should not be called")
			return false, nil
		},
	}
}

func unexpectedTokenGenerator(t *testing.T) services.TokenGenerator {
	t.Helper()

	return func() (string, []byte, error) {
		t.Fatal("token generator should not be called")
		return "", nil, nil
	}
}
