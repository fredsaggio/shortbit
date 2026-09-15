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

type GoogleIdentity struct {
	Email          string
	ProviderUserID string
}

type GoogleOIDCClient interface {
	AuthorizationURL(state, nonce, codeVerifier string) string
	ExchangeAndVerify(ctx context.Context, code, expectedNonce, codeVerifier string) (GoogleIdentity, error)
}

type GoogleUserRepository interface {
	CreateWithIdentity(ctx context.Context, email, provider, providerUserID string) (models.User, error)
	FindUserByProviderIdentity(ctx context.Context, provider, providerUserID string) (models.User, error)
}

type UserSessionCreator interface {
	CreateSession(ctx context.Context, userID uuid.UUID) (LoginResult, error)
}

type GoogleAuthService struct {
	oidcClient     GoogleOIDCClient
	googleRepo     GoogleUserRepository
	sessionCreator UserSessionCreator
}

func NewGoogleAuthService(userRepo GoogleUserRepository, sessionCreator UserSessionCreator, oidcClient GoogleOIDCClient) *GoogleAuthService {
	return &GoogleAuthService{
		googleRepo:     userRepo,
		sessionCreator: sessionCreator,
		oidcClient:     oidcClient,
	}
}

func (s *GoogleAuthService) AuthorizationURL(state, nonce, codeVerifier string) string {
	return s.oidcClient.AuthorizationURL(state, nonce, codeVerifier)
}

func (s *GoogleAuthService) CompleteLogin(ctx context.Context, code, expectedNonce, codeVerifier string) (LoginResult, error) {
	identity, err := s.oidcClient.ExchangeAndVerify(ctx, code, expectedNonce, codeVerifier)

	if err != nil {
		return LoginResult{}, fmt.Errorf("verify Google identity: %w", err)
	}

	return s.loginWithProvider(ctx, identity.Email, identity.ProviderUserID)
}

func (s *GoogleAuthService) loginWithProvider(ctx context.Context, email, providerUserID string) (LoginResult, error) {
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
