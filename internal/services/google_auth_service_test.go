package services_test

import (
	"context"
	"errors"
	"testing"
	"time"
	"uuid"

	"github.com/fredsaggio/url-shortener/internal/models"
	"github.com/fredsaggio/url-shortener/internal/repositories"
	"github.com/fredsaggio/url-shortener/internal/services"
)

type googleUserRepositoryStub struct {
	createWithIdentityFunc         func(ctx context.Context, email, provider, providerUserID string) (models.User, error)
	findUserByProviderIdentityFunc func(ctx context.Context, provider, providerUserID string) (models.User, error)
}

func (s googleUserRepositoryStub) CreateWithIdentity(ctx context.Context, email, provider, providerUserID string) (models.User, error) {
	return s.createWithIdentityFunc(ctx, email, provider, providerUserID)
}

func (s googleUserRepositoryStub) FindUserByProviderIdentity(ctx context.Context, provider, providerUserID string) (models.User, error) {
	return s.findUserByProviderIdentityFunc(ctx, provider, providerUserID)
}

type userSessionCreatorStub struct {
	createSessionFunc func(ctx context.Context, userID uuid.UUID) (services.LoginResult, error)
}

func (s userSessionCreatorStub) CreateSession(ctx context.Context, userID uuid.UUID) (services.LoginResult, error) {
	return s.createSessionFunc(ctx, userID)
}

type googleOIDCClientStub struct {
	authorizationURLFunc  func(state, nonce, codeVerifier string) string
	exchangeAndVerifyFunc func(ctx context.Context, code, expectedNonce, codeVerifier string) (services.GoogleIdentity, error)
}

func (s googleOIDCClientStub) AuthorizationURL(state, nonce, codeVerifier string) string {
	return s.authorizationURLFunc(state, nonce, codeVerifier)
}

func (s googleOIDCClientStub) ExchangeAndVerify(ctx context.Context, code, expectedNonce, codeVerifier string) (services.GoogleIdentity, error) {
	return s.exchangeAndVerifyFunc(ctx, code, expectedNonce, codeVerifier)
}

func TestGoogleAuthServiceAuthorizationURL(t *testing.T) {
	const (
		state        = "random-state"
		nonce        = "random-nonce"
		codeVerifier = "random-code-verifier"
		wantURL      = "https://accounts.google.com/o/oauth2/v2/auth"
	)

	oidcClient := googleOIDCClientStub{
		authorizationURLFunc: func(gotState, gotNonce, gotCodeVerifier string) string {
			if gotState != state {
				t.Errorf("AuthorizationURL() state = %q, want %q", gotState, state)
			}
			if gotNonce != nonce {
				t.Errorf("AuthorizationURL() nonce = %q, want %q", gotNonce, nonce)
			}
			if gotCodeVerifier != codeVerifier {
				t.Errorf("AuthorizationURL() code verifier = %q, want %q", gotCodeVerifier, codeVerifier)
			}

			return wantURL
		},
	}

	service := services.NewGoogleAuthService(nil, nil, oidcClient)
	if got := service.AuthorizationURL(state, nonce, codeVerifier); got != wantURL {
		t.Errorf("AuthorizationURL() = %q, want %q", got, wantURL)
	}
}

func TestGoogleAuthServiceCompleteLoginExistingIdentity(t *testing.T) {
	const (
		providerUserID = "google-subject-123"
		contextValue   = "request-context"
		code           = "authorization-code"
		expectedNonce  = "expected-nonce"
		codeVerifier   = "code-verifier"
	)

	type contextKey struct{}
	ctx := context.WithValue(context.Background(), contextKey{}, contextValue)
	wantUser := models.User{ID: uuid.MustParse("01991f29-7c22-7ab3-a395-4d402f09c317"), Email: "user@example.com"}
	wantLogin := services.LoginResult{Token: "raw-session-token", ExpiresAt: time.Date(2026, time.September, 15, 10, 0, 0, 0, time.UTC)}

	repository := googleUserRepositoryStub{
		findUserByProviderIdentityFunc: func(gotCtx context.Context, provider, gotProviderUserID string) (models.User, error) {
			if got := gotCtx.Value(contextKey{}); got != contextValue {
				t.Errorf("FindUserByProviderIdentity() context value = %v, want %q", got, contextValue)
			}
			if provider != "google" {
				t.Errorf("FindUserByProviderIdentity() provider = %q, want %q", provider, "google")
			}
			if gotProviderUserID != providerUserID {
				t.Errorf("FindUserByProviderIdentity() provider user ID = %q, want %q", gotProviderUserID, providerUserID)
			}

			return wantUser, nil
		},
		createWithIdentityFunc: func(context.Context, string, string, string) (models.User, error) {
			t.Fatal("CreateWithIdentity() should not be called for an existing identity")
			return models.User{}, nil
		},
	}

	sessionCreator := userSessionCreatorStub{
		createSessionFunc: func(gotCtx context.Context, userID uuid.UUID) (services.LoginResult, error) {
			if got := gotCtx.Value(contextKey{}); got != contextValue {
				t.Errorf("CreateSession() context value = %v, want %q", got, contextValue)
			}
			if userID != wantUser.ID {
				t.Errorf("CreateSession() user ID = %s, want %s", userID, wantUser.ID)
			}

			return wantLogin, nil
		},
	}

	oidcClient := googleOIDCClientStub{
		exchangeAndVerifyFunc: func(gotCtx context.Context, gotCode, gotExpectedNonce, gotCodeVerifier string) (services.GoogleIdentity, error) {
			if got := gotCtx.Value(contextKey{}); got != contextValue {
				t.Errorf("ExchangeAndVerify() context value = %v, want %q", got, contextValue)
			}
			if gotCode != code {
				t.Errorf("ExchangeAndVerify() code = %q, want %q", gotCode, code)
			}
			if gotExpectedNonce != expectedNonce {
				t.Errorf("ExchangeAndVerify() expected nonce = %q, want %q", gotExpectedNonce, expectedNonce)
			}
			if gotCodeVerifier != codeVerifier {
				t.Errorf("ExchangeAndVerify() code verifier = %q, want %q", gotCodeVerifier, codeVerifier)
			}

			return services.GoogleIdentity{Email: wantUser.Email, ProviderUserID: providerUserID}, nil
		},
	}

	service := services.NewGoogleAuthService(repository, sessionCreator, oidcClient)
	login, err := service.CompleteLogin(ctx, code, expectedNonce, codeVerifier)
	if err != nil {
		t.Fatalf("CompleteLogin() error = %v", err)
	}

	if login != wantLogin {
		t.Errorf("CompleteLogin() result = %+v, want %+v", login, wantLogin)
	}
}

func TestGoogleAuthServiceCompleteLoginCreatesMissingIdentity(t *testing.T) {
	const (
		inputEmail      = "  USER@Example.COM  "
		normalizedEmail = "user@example.com"
		providerUserID  = "new-google-subject"
	)

	wantUser := models.User{ID: uuid.MustParse("01991f29-7c22-7ab3-a395-4d402f09c318"), Email: normalizedEmail}
	wantLogin := services.LoginResult{Token: "new-session-token", ExpiresAt: time.Date(2026, time.September, 15, 11, 0, 0, 0, time.UTC)}

	repository := googleUserRepositoryStub{
		findUserByProviderIdentityFunc: func(context.Context, string, string) (models.User, error) {
			return models.User{}, repositories.ErrAuthIdentityNotFound
		},
		createWithIdentityFunc: func(_ context.Context, email, provider, gotProviderUserID string) (models.User, error) {
			if email != normalizedEmail {
				t.Errorf("CreateWithIdentity() email = %q, want %q", email, normalizedEmail)
			}
			if provider != "google" {
				t.Errorf("CreateWithIdentity() provider = %q, want %q", provider, "google")
			}
			if gotProviderUserID != providerUserID {
				t.Errorf("CreateWithIdentity() provider user ID = %q, want %q", gotProviderUserID, providerUserID)
			}

			return wantUser, nil
		},
	}

	sessionCreator := userSessionCreatorStub{
		createSessionFunc: func(_ context.Context, userID uuid.UUID) (services.LoginResult, error) {
			if userID != wantUser.ID {
				t.Errorf("CreateSession() user ID = %s, want %s", userID, wantUser.ID)
			}

			return wantLogin, nil
		},
	}

	service := services.NewGoogleAuthService(repository, sessionCreator, verifiedGoogleOIDCClient(inputEmail, providerUserID))
	login, err := service.CompleteLogin(context.Background(), "authorization-code", "expected-nonce", "code-verifier")
	if err != nil {
		t.Fatalf("CompleteLogin() error = %v", err)
	}

	if login != wantLogin {
		t.Errorf("CompleteLogin() result = %+v, want %+v", login, wantLogin)
	}
}

func TestGoogleAuthServiceCompleteLoginRejectsInvalidEmailForMissingIdentity(t *testing.T) {
	repository := googleUserRepositoryStub{
		findUserByProviderIdentityFunc: func(context.Context, string, string) (models.User, error) {
			return models.User{}, repositories.ErrAuthIdentityNotFound
		},
		createWithIdentityFunc: func(context.Context, string, string, string) (models.User, error) {
			t.Fatal("CreateWithIdentity() should not be called for an invalid email")
			return models.User{}, nil
		},
	}

	service := services.NewGoogleAuthService(repository, unexpectedUserSessionCreator(t), verifiedGoogleOIDCClient("invalid-email", "new-google-subject"))
	_, err := service.CompleteLogin(context.Background(), "authorization-code", "expected-nonce", "code-verifier")

	if !errors.Is(err, services.ErrInvalidEmail) {
		t.Fatalf("CompleteLogin() error = %v, want %v", err, services.ErrInvalidEmail)
	}
}

func TestGoogleAuthServiceCompleteLoginRejectsExistingEmail(t *testing.T) {
	repository := googleUserRepositoryStub{
		findUserByProviderIdentityFunc: func(context.Context, string, string) (models.User, error) {
			return models.User{}, repositories.ErrAuthIdentityNotFound
		},
		createWithIdentityFunc: func(context.Context, string, string, string) (models.User, error) {
			return models.User{}, repositories.ErrEmailAlreadyExists
		},
	}

	service := services.NewGoogleAuthService(repository, unexpectedUserSessionCreator(t), verifiedGoogleOIDCClient("user@example.com", "new-google-subject"))
	_, err := service.CompleteLogin(context.Background(), "authorization-code", "expected-nonce", "code-verifier")

	if !errors.Is(err, services.ErrEmailAlreadyExists) {
		t.Fatalf("CompleteLogin() error = %v, want %v", err, services.ErrEmailAlreadyExists)
	}
}

func TestGoogleAuthServiceCompleteLoginPropagatesIdentityLookupError(t *testing.T) {
	wantErr := errors.New("database unavailable")
	repository := googleUserRepositoryStub{
		findUserByProviderIdentityFunc: func(context.Context, string, string) (models.User, error) {
			return models.User{}, wantErr
		},
		createWithIdentityFunc: func(context.Context, string, string, string) (models.User, error) {
			t.Fatal("CreateWithIdentity() should not be called after a lookup error")
			return models.User{}, nil
		},
	}

	service := services.NewGoogleAuthService(repository, unexpectedUserSessionCreator(t), verifiedGoogleOIDCClient("user@example.com", "google-subject"))
	_, err := service.CompleteLogin(context.Background(), "authorization-code", "expected-nonce", "code-verifier")

	if !errors.Is(err, wantErr) {
		t.Fatalf("CompleteLogin() error = %v, want wrapped %v", err, wantErr)
	}
}

func TestGoogleAuthServiceCompleteLoginPropagatesIdentityCreationError(t *testing.T) {
	wantErr := errors.New("database unavailable")
	repository := googleUserRepositoryStub{
		findUserByProviderIdentityFunc: func(context.Context, string, string) (models.User, error) {
			return models.User{}, repositories.ErrAuthIdentityNotFound
		},
		createWithIdentityFunc: func(context.Context, string, string, string) (models.User, error) {
			return models.User{}, wantErr
		},
	}

	service := services.NewGoogleAuthService(repository, unexpectedUserSessionCreator(t), verifiedGoogleOIDCClient("user@example.com", "google-subject"))
	_, err := service.CompleteLogin(context.Background(), "authorization-code", "expected-nonce", "code-verifier")

	if !errors.Is(err, wantErr) {
		t.Fatalf("CompleteLogin() error = %v, want wrapped %v", err, wantErr)
	}
}

func TestGoogleAuthServiceCompleteLoginPropagatesSessionCreationError(t *testing.T) {
	wantErr := errors.New("session storage unavailable")
	wantUser := models.User{ID: uuid.MustParse("01991f29-7c22-7ab3-a395-4d402f09c319"), Email: "user@example.com"}
	repository := googleUserRepositoryStub{
		findUserByProviderIdentityFunc: func(context.Context, string, string) (models.User, error) {
			return wantUser, nil
		},
		createWithIdentityFunc: func(context.Context, string, string, string) (models.User, error) {
			t.Fatal("CreateWithIdentity() should not be called for an existing identity")
			return models.User{}, nil
		},
	}

	sessionCreator := userSessionCreatorStub{
		createSessionFunc: func(context.Context, uuid.UUID) (services.LoginResult, error) {
			return services.LoginResult{}, wantErr
		},
	}

	service := services.NewGoogleAuthService(repository, sessionCreator, verifiedGoogleOIDCClient("user@example.com", "google-subject"))
	_, err := service.CompleteLogin(context.Background(), "authorization-code", "expected-nonce", "code-verifier")

	if !errors.Is(err, wantErr) {
		t.Fatalf("CompleteLogin() error = %v, want %v", err, wantErr)
	}
}

func TestGoogleAuthServiceCompleteLoginPropagatesOIDCError(t *testing.T) {
	wantErr := errors.New("invalid ID token")
	oidcClient := googleOIDCClientStub{
		exchangeAndVerifyFunc: func(context.Context, string, string, string) (services.GoogleIdentity, error) {
			return services.GoogleIdentity{}, wantErr
		},
	}

	service := services.NewGoogleAuthService(nil, nil, oidcClient)
	_, err := service.CompleteLogin(context.Background(), "authorization-code", "expected-nonce", "code-verifier")

	if !errors.Is(err, wantErr) {
		t.Fatalf("CompleteLogin() error = %v, want wrapped %v", err, wantErr)
	}
}

func verifiedGoogleOIDCClient(email, providerUserID string) googleOIDCClientStub {
	return googleOIDCClientStub{
		exchangeAndVerifyFunc: func(context.Context, string, string, string) (services.GoogleIdentity, error) {
			return services.GoogleIdentity{Email: email, ProviderUserID: providerUserID}, nil
		},
	}
}

func unexpectedUserSessionCreator(t *testing.T) userSessionCreatorStub {
	t.Helper()

	return userSessionCreatorStub{
		createSessionFunc: func(context.Context, uuid.UUID) (services.LoginResult, error) {
			t.Fatal("CreateSession() should not be called")
			return services.LoginResult{}, nil
		},
	}
}
