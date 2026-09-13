package services_test

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
	"uuid"

	"github.com/fredsaggio/url-shortener/internal/models"
	"github.com/fredsaggio/url-shortener/internal/repositories"
	"github.com/fredsaggio/url-shortener/internal/services"
)

type userRepositoryStub struct {
	createWithPasswordFunc func(
		ctx context.Context,
		email string,
		passwordHash string,
	) (models.User, error)
	findByIDFunc func(ctx context.Context, userID uuid.UUID) (models.User, error)
}

func (s userRepositoryStub) CreateWithPassword(
	ctx context.Context,
	email string,
	passwordHash string,
) (models.User, error) {
	return s.createWithPasswordFunc(ctx, email, passwordHash)
}

func (s userRepositoryStub) FindByID(ctx context.Context, userID uuid.UUID) (models.User, error) {
	return s.findByIDFunc(ctx, userID)
}

type passwordHasherStub struct {
	hashFunc func(password string) (string, error)
}

func (s passwordHasherStub) Hash(password string) (string, error) {
	return s.hashFunc(password)
}

func TestUserServiceRegisterWithPassword(t *testing.T) {
	const (
		inputEmail      = "  USER@Example.COM  "
		normalizedEmail = "user@example.com"
		password        = "senha-segura"
		encodedPassword = "$argon2id$test-hash"
		contextValue    = "request-context"
	)

	type contextKey struct{}
	ctx := context.WithValue(context.Background(), contextKey{}, contextValue)

	wantUser := models.User{
		ID:        uuid.MustParse("01991f29-7c22-7ab3-a395-4d402f09c317"),
		Email:     normalizedEmail,
		CreatedAt: time.Date(2026, time.September, 12, 10, 0, 0, 0, time.UTC),
		UpdatedAt: time.Date(2026, time.September, 12, 10, 0, 0, 0, time.UTC),
	}

	hasher := passwordHasherStub{
		hashFunc: func(gotPassword string) (string, error) {
			if gotPassword != password {
				t.Fatalf("Hash() password = %q, want %q", gotPassword, password)
			}

			return encodedPassword, nil
		},
	}

	repository := userRepositoryStub{
		createWithPasswordFunc: func(
			gotCtx context.Context,
			gotEmail string,
			gotPasswordHash string,
		) (models.User, error) {
			if got := gotCtx.Value(contextKey{}); got != contextValue {
				t.Fatalf("CreateWithPassword() context value = %v, want %q", got, contextValue)
			}

			if gotEmail != normalizedEmail {
				t.Errorf("CreateWithPassword() email = %q, want %q", gotEmail, normalizedEmail)
			}

			if gotPasswordHash != encodedPassword {
				t.Errorf(
					"CreateWithPassword() password hash = %q, want %q",
					gotPasswordHash,
					encodedPassword,
				)
			}

			if gotPasswordHash == password {
				t.Error("CreateWithPassword() received the plain-text password")
			}

			return wantUser, nil
		},
	}

	service := services.NewUserService(repository, hasher)

	gotUser, err := service.RegisterWithPassword(ctx, inputEmail, password)
	if err != nil {
		t.Fatalf("RegisterWithPassword() error = %v", err)
	}

	if gotUser != wantUser {
		t.Errorf("RegisterWithPassword() user = %+v, want %+v", gotUser, wantUser)
	}
}

func TestUserServiceRegisterWithPasswordRejectsInvalidEmail(t *testing.T) {
	repository := userRepositoryStub{
		createWithPasswordFunc: func(context.Context, string, string) (models.User, error) {
			t.Fatal("CreateWithPassword() should not be called")
			return models.User{}, nil
		},
	}

	hasher := passwordHasherStub{
		hashFunc: func(string) (string, error) {
			t.Fatal("Hash() should not be called")
			return "", nil
		},
	}

	service := services.NewUserService(repository, hasher)

	_, err := service.RegisterWithPassword(
		context.Background(),
		"invalid-email",
		"senha-segura",
	)
	if !errors.Is(err, services.ErrInvalidEmail) {
		t.Fatalf("RegisterWithPassword() error = %v, want %v", err, services.ErrInvalidEmail)
	}
}

func TestUserServiceRegisterWithPasswordRejectsInvalidPassword(t *testing.T) {
	tests := []struct {
		name     string
		password string
		wantErr  error
	}{
		{
			name:     "too short",
			password: "1234567",
			wantErr:  services.ErrPasswordTooShort,
		},
		{
			name:     "too long",
			password: strings.Repeat("a", 1025),
			wantErr:  services.ErrPasswordTooLong,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repository := userRepositoryStub{
				createWithPasswordFunc: func(context.Context, string, string) (models.User, error) {
					t.Fatal("CreateWithPassword() should not be called")
					return models.User{}, nil
				},
			}

			hasher := passwordHasherStub{
				hashFunc: func(string) (string, error) {
					t.Fatal("Hash() should not be called")
					return "", nil
				},
			}

			service := services.NewUserService(repository, hasher)

			_, err := service.RegisterWithPassword(
				context.Background(),
				"user@example.com",
				tt.password,
			)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("RegisterWithPassword() error = %v, want %v", err, tt.wantErr)
			}
		})
	}
}

func TestUserServiceRegisterWithPasswordPropagatesHasherError(t *testing.T) {
	wantErr := errors.New("hasher unavailable")

	repository := userRepositoryStub{
		createWithPasswordFunc: func(context.Context, string, string) (models.User, error) {
			t.Fatal("CreateWithPassword() should not be called")
			return models.User{}, nil
		},
	}

	hasher := passwordHasherStub{
		hashFunc: func(string) (string, error) {
			return "", wantErr
		},
	}

	service := services.NewUserService(repository, hasher)

	_, err := service.RegisterWithPassword(
		context.Background(),
		"user@example.com",
		"senha-segura",
	)
	if !errors.Is(err, wantErr) {
		t.Fatalf("RegisterWithPassword() error = %v, want wrapped %v", err, wantErr)
	}
}

func TestUserServiceRegisterWithPasswordTranslatesDuplicateEmail(t *testing.T) {
	repository := userRepositoryStub{
		createWithPasswordFunc: func(context.Context, string, string) (models.User, error) {
			return models.User{}, repositories.ErrEmailAlreadyExists
		},
	}

	hasher := passwordHasherStub{
		hashFunc: func(string) (string, error) {
			return "$argon2id$test-hash", nil
		},
	}

	service := services.NewUserService(repository, hasher)

	_, err := service.RegisterWithPassword(
		context.Background(),
		"user@example.com",
		"senha-segura",
	)
	if !errors.Is(err, services.ErrEmailAlreadyExists) {
		t.Fatalf(
			"RegisterWithPassword() error = %v, want %v",
			err,
			services.ErrEmailAlreadyExists,
		)
	}
}

func TestUserServiceRegisterWithPasswordPropagatesRepositoryError(t *testing.T) {
	wantErr := errors.New("database unavailable")

	repository := userRepositoryStub{
		createWithPasswordFunc: func(context.Context, string, string) (models.User, error) {
			return models.User{}, wantErr
		},
	}

	hasher := passwordHasherStub{
		hashFunc: func(string) (string, error) {
			return "$argon2id$test-hash", nil
		},
	}

	service := services.NewUserService(repository, hasher)

	_, err := service.RegisterWithPassword(
		context.Background(),
		"user@example.com",
		"senha-segura",
	)
	if !errors.Is(err, wantErr) {
		t.Fatalf("RegisterWithPassword() error = %v, want wrapped %v", err, wantErr)
	}
}

func TestUserServiceGetByID(t *testing.T) {
	wantUserID := uuid.MustParse("01991f29-7c22-7ab3-a395-4d402f09c317")
	wantUser := models.User{
		ID:        wantUserID,
		Email:     "user@example.com",
		CreatedAt: time.Date(2026, time.September, 13, 10, 0, 0, 0, time.UTC),
		UpdatedAt: time.Date(2026, time.September, 13, 11, 0, 0, 0, time.UTC),
	}

	type contextKey struct{}
	ctx := context.WithValue(context.Background(), contextKey{}, "request-context")
	repository := userRepositoryStub{
		findByIDFunc: func(gotCtx context.Context, gotUserID uuid.UUID) (models.User, error) {
			if got := gotCtx.Value(contextKey{}); got != "request-context" {
				t.Errorf("FindByID() context value = %v, want %q", got, "request-context")
			}
			if gotUserID != wantUserID {
				t.Errorf("FindByID() user ID = %s, want %s", gotUserID, wantUserID)
			}
			return wantUser, nil
		},
	}

	service := services.NewUserService(repository, unexpectedPasswordHasher(t))
	user, err := service.GetByID(ctx, wantUserID)
	if err != nil {
		t.Fatalf("GetByID() error = %v", err)
	}

	if user != wantUser {
		t.Errorf("GetByID() user = %+v, want %+v", user, wantUser)
	}
}

func TestUserServiceGetByIDPropagatesRepositoryError(t *testing.T) {
	wantErr := errors.New("database unavailable")
	repository := userRepositoryStub{
		findByIDFunc: func(context.Context, uuid.UUID) (models.User, error) {
			return models.User{}, wantErr
		},
	}

	service := services.NewUserService(repository, unexpectedPasswordHasher(t))
	_, err := service.GetByID(context.Background(), uuid.MustParse("01991f29-7c22-7ab3-a395-4d402f09c317"))
	if !errors.Is(err, wantErr) {
		t.Fatalf("GetByID() error = %v, want wrapped %v", err, wantErr)
	}
}

func unexpectedPasswordHasher(t *testing.T) passwordHasherStub {
	t.Helper()

	return passwordHasherStub{
		hashFunc: func(string) (string, error) {
			t.Fatal("Hash() should not be called")
			return "", nil
		},
	}
}
