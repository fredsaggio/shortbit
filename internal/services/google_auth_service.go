package services

import (
	"context"
	"errors"
	"fmt"
	"uuid"

	"github.com/fredsaggio/url-shortener/internal/models"
	"github.com/fredsaggio/url-shortener/internal/repositories"
)

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

func (s *GoogleAuthService) LoginWithProvider(ctx context.Context, provider, providerUserID string) (LoginResult, error) {
	user, err := s.FindUserByProviderIdentity(ctx, provider, providerUserID)

	if err != nil {
		return LoginResult{}, fmt.Errorf("find user with provider id: %w", err)
	}

	login, err := s.sessionCreator.CreateSession(ctx, user.ID)

	if err != nil {
		return LoginResult{}, fmt.Errorf("create session: %w", err)
	}

	return login, nil
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
