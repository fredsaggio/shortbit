package services

import (
	"context"
	"errors"
	"fmt"
	"time"
	"uuid"

	"github.com/fredsaggio/url-shortener/internal/models"
	"github.com/fredsaggio/url-shortener/internal/repositories"
)

var ErrIncorrectEmailOrPassword = errors.New("email or password incorrect(s)")

type UserSessionRepository interface {
	Create(ctx context.Context, userID uuid.UUID, tokenHash []byte, expiresAt time.Time) error
}

type PasswordCredentialRepository interface {
	FindPasswordCredentialsByEmail(ctx context.Context, email string) (models.User, models.PasswordCredential, error)
}

type PasswordComparator interface {
	Compare(password, encodedHash string) (bool, error)
}

type TokenGenerator func() (string, []byte, error)

type AuthService struct {
	userRepo        PasswordCredentialRepository
	userSessionRepo UserSessionRepository
	hasher          PasswordComparator
	generateToken   TokenGenerator
	sessionTTL      time.Duration
}

type LoginResult struct {
	Token     string
	ExpiresAt time.Time
}

func NewAuthService(userRepo PasswordCredentialRepository, userSessionRepo UserSessionRepository, hasher PasswordComparator, token TokenGenerator, sessionTTL time.Duration) *AuthService {
	return &AuthService{
		userRepo:        userRepo,
		userSessionRepo: userSessionRepo,
		hasher:          hasher,
		generateToken:   token,
		sessionTTL:      sessionTTL,
	}
}

func (s *AuthService) Login(ctx context.Context, email, password string) (LoginResult, error) {
	normalizedEmail := normalizeEmail(email)

	if normalizedEmail == "" || password == "" {
		return LoginResult{}, ErrIncorrectEmailOrPassword
	}

	user, credential, err := s.userRepo.FindPasswordCredentialsByEmail(ctx, normalizedEmail)

	if err != nil {
		if errors.Is(err, repositories.ErrPasswordCredentialNotFound) {
			return LoginResult{}, ErrIncorrectEmailOrPassword
		}

		return LoginResult{}, fmt.Errorf("find password credentials: %w", err)
	}

	match, err := s.hasher.Compare(password, credential.PasswordHash)

	if err != nil {
		return LoginResult{}, fmt.Errorf("compare with database user: %w", err)
	}

	if !match {
		return LoginResult{}, ErrIncorrectEmailOrPassword
	}

	token, tokenHash, err := s.generateToken()

	if err != nil {
		return LoginResult{}, fmt.Errorf("generate token: %w", err)
	}

	expirationTime := time.Now().UTC().Add(s.sessionTTL)

	if err = s.userSessionRepo.Create(ctx, user.ID, tokenHash, expirationTime); err != nil {
		return LoginResult{}, fmt.Errorf("create session: %w", err)
	}

	return LoginResult{
		Token:     token,
		ExpiresAt: expirationTime,
	}, nil

}
