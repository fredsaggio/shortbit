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

func TestGoogleAuthServiceLoginWithProviderExistingIdentity(t *testing.T) {
	const (
		providerUserID = "google-subject-123"
		contextValue   = "request-context"
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

	service := services.NewGoogleAuthService(repository, sessionCreator)
	login, err := service.LoginWithProvider(ctx, "user@example.com", providerUserID)
	if err != nil {
		t.Fatalf("LoginWithProvider() error = %v", err)
	}

	if login != wantLogin {
		t.Errorf("LoginWithProvider() result = %+v, want %+v", login, wantLogin)
	}
}

func TestGoogleAuthServiceLoginWithProviderCreatesMissingIdentity(t *testing.T) {
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

	service := services.NewGoogleAuthService(repository, sessionCreator)
	login, err := service.LoginWithProvider(context.Background(), inputEmail, providerUserID)
	if err != nil {
		t.Fatalf("LoginWithProvider() error = %v", err)
	}

	if login != wantLogin {
		t.Errorf("LoginWithProvider() result = %+v, want %+v", login, wantLogin)
	}
}

func TestGoogleAuthServiceLoginWithProviderRejectsInvalidEmailForMissingIdentity(t *testing.T) {
	repository := googleUserRepositoryStub{
		findUserByProviderIdentityFunc: func(context.Context, string, string) (models.User, error) {
			return models.User{}, repositories.ErrAuthIdentityNotFound
		},
		createWithIdentityFunc: func(context.Context, string, string, string) (models.User, error) {
			t.Fatal("CreateWithIdentity() should not be called for an invalid email")
			return models.User{}, nil
		},
	}

	service := services.NewGoogleAuthService(repository, unexpectedUserSessionCreator(t))
	_, err := service.LoginWithProvider(context.Background(), "invalid-email", "new-google-subject")

	if !errors.Is(err, services.ErrInvalidEmail) {
		t.Fatalf("LoginWithProvider() error = %v, want %v", err, services.ErrInvalidEmail)
	}
}

func TestGoogleAuthServiceLoginWithProviderRejectsExistingEmail(t *testing.T) {
	repository := googleUserRepositoryStub{
		findUserByProviderIdentityFunc: func(context.Context, string, string) (models.User, error) {
			return models.User{}, repositories.ErrAuthIdentityNotFound
		},
		createWithIdentityFunc: func(context.Context, string, string, string) (models.User, error) {
			return models.User{}, repositories.ErrEmailAlreadyExists
		},
	}

	service := services.NewGoogleAuthService(repository, unexpectedUserSessionCreator(t))
	_, err := service.LoginWithProvider(context.Background(), "user@example.com", "new-google-subject")

	if !errors.Is(err, services.ErrEmailAlreadyExists) {
		t.Fatalf("LoginWithProvider() error = %v, want %v", err, services.ErrEmailAlreadyExists)
	}
}

func TestGoogleAuthServiceLoginWithProviderPropagatesIdentityLookupError(t *testing.T) {
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

	service := services.NewGoogleAuthService(repository, unexpectedUserSessionCreator(t))
	_, err := service.LoginWithProvider(context.Background(), "user@example.com", "google-subject")

	if !errors.Is(err, wantErr) {
		t.Fatalf("LoginWithProvider() error = %v, want wrapped %v", err, wantErr)
	}
}

func TestGoogleAuthServiceLoginWithProviderPropagatesIdentityCreationError(t *testing.T) {
	wantErr := errors.New("database unavailable")
	repository := googleUserRepositoryStub{
		findUserByProviderIdentityFunc: func(context.Context, string, string) (models.User, error) {
			return models.User{}, repositories.ErrAuthIdentityNotFound
		},
		createWithIdentityFunc: func(context.Context, string, string, string) (models.User, error) {
			return models.User{}, wantErr
		},
	}

	service := services.NewGoogleAuthService(repository, unexpectedUserSessionCreator(t))
	_, err := service.LoginWithProvider(context.Background(), "user@example.com", "google-subject")

	if !errors.Is(err, wantErr) {
		t.Fatalf("LoginWithProvider() error = %v, want wrapped %v", err, wantErr)
	}
}

func TestGoogleAuthServiceLoginWithProviderPropagatesSessionCreationError(t *testing.T) {
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

	service := services.NewGoogleAuthService(repository, sessionCreator)
	_, err := service.LoginWithProvider(context.Background(), "user@example.com", "google-subject")

	if !errors.Is(err, wantErr) {
		t.Fatalf("LoginWithProvider() error = %v, want %v", err, wantErr)
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
