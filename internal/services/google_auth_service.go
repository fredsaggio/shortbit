package services

import (
	"context"
	"errors"
	"fmt"
	"uuid"

	"github.com/fredsaggio/url-shortener/internal/models"
	"github.com/fredsaggio/url-shortener/internal/repositories"
)

const googleProvider = "google"

type GoogleUserRepository interface {
	CreateWithIdentity(ctx context.Context, email, provider, providerUserID string) (models.User, error)
	FindUserByProviderIdentity(ctx context.Context, provider, providerUserID string) (models.User, error)
}

type UserSessionCreator interface {
	CreateSession(ctx context.Context, userID uuid.UUID) (LoginResult, error)
}

type GoogleAuthService struct {
	googleRepo     GoogleUserRepository
	sessionCreator UserSessionCreator
}

func NewGoogleAuthService(userRepo GoogleUserRepository, sessionCreator UserSessionCreator) *GoogleAuthService {
	return &GoogleAuthService{
		googleRepo:     userRepo,
		sessionCreator: sessionCreator,
	}
}

func (s *GoogleAuthService) LoginWithProvider(ctx context.Context, email, providerUserID string) (LoginResult, error) {
	user, err := s.googleRepo.FindUserByProviderIdentity(ctx, googleProvider, providerUserID)

	switch {
	case err == nil:
	case errors.Is(err, repositories.ErrAuthIdentityNotFound):
		normalizedEmail := normalizeEmail(email)

		if !isValidEmail(normalizedEmail) {
			return LoginResult{}, ErrInvalidEmail
		}

		user, err = s.googleRepo.CreateWithIdentity(ctx, normalizedEmail, googleProvider, providerUserID)
		if errors.Is(err, repositories.ErrEmailAlreadyExists) {
			return LoginResult{}, ErrEmailAlreadyExists
		}
		if err != nil {
			return LoginResult{}, fmt.Errorf("create user with Google identity: %w", err)
		}

	default:
		return LoginResult{}, fmt.Errorf("find user by Google identity: %w", err)
	}

	return s.sessionCreator.CreateSession(ctx, user.ID)
}
