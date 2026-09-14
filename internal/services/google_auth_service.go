package services

import (
	"context"
	"errors"
	"fmt"

	"github.com/fredsaggio/url-shortener/internal/models"
	"github.com/fredsaggio/url-shortener/internal/repositories"
)

type GoogleUserRepository interface {
	CreateWithIdentity(ctx context.Context, email, provider, providerUserID string) (models.User, error)
	FindUserByProviderIdentity(ctx context.Context, provider, providerUserID string) (models.User, error)
}

type GoogleAuthService struct {
	googleRepo GoogleUserRepository
}

func NewGoogleAuthService(userRepo GoogleUserRepository) *GoogleAuthService {
	return &GoogleAuthService{
		googleRepo: userRepo,
	}
}

func (s *GoogleAuthService) RegisterWithProvider(ctx context.Context, email, provider, providerUserID string) (models.User, error) {
	normalizedEmail := normalizeEmail(email)

	if !isValidEmail(normalizedEmail) {
		return models.User{}, ErrInvalidEmail
	}

	user, err := s.googleRepo.CreateWithIdentity(ctx, normalizedEmail, provider, providerUserID)

	if err != nil {
		if errors.Is(err, repositories.ErrEmailAlreadyExists) {
			return models.User{}, ErrEmailAlreadyExists
		}

		return models.User{}, fmt.Errorf("create user with identity: %w", err)
	}

	return user, nil
}

func (s *GoogleAuthService) FindUserByProviderIdentity(ctx context.Context, provider, providerUserID string) (models.User, error) {
	user, err := s.googleRepo.FindUserByProviderIdentity(ctx, provider, providerUserID)

	if err != nil {
		return models.User{}, fmt.Errorf("get user by provider: %w", err)
	}

	return user, nil
}
