package services

import (
	"context"
	"errors"
	"fmt"
	"time"
	"uuid"

	"github.com/fredsaggio/url-shortener/internal/models"
	"github.com/fredsaggio/url-shortener/internal/repositories"
	"github.com/fredsaggio/url-shortener/internal/sessiontoken"
)

var (
	ErrIncorrectEmailOrPassword = errors.New("email or password incorrect(s)")
	ErrUnauthenticated          = errors.New("unauthenticated")
)

type UserSessionRepository interface {
	Create(ctx context.Context, userID uuid.UUID, tokenHash []byte, expiresAt time.Time) error
	FindSessionByTokenHash(ctx context.Context, tokenHash []byte) (models.UserSession, error)
	DeleteSessionByTokenHash(ctx context.Context, tokenHash []byte) error
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

func (s *AuthService) LoginWithPassword(ctx context.Context, email, password string) (LoginResult, error) {
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

	// Não é preciso pegar o erro e tratar aqui porque as duas funções retornam a mesma coisa (Essa atual e essa que tá retornando), ou seja, no handler as variáveis já estão sendo passadas.
	return s.CreateSession(ctx, user.ID)
}

func (s *AuthService) CreateSession(ctx context.Context, userID uuid.UUID) (LoginResult, error) {
	token, tokenHash, err := s.generateToken()

	if err != nil {
		return LoginResult{}, fmt.Errorf("generate token: %w", err)
	}

	expirationTime := time.Now().UTC().Add(s.sessionTTL)

	if err = s.userSessionRepo.Create(ctx, userID, tokenHash, expirationTime); err != nil {
		return LoginResult{}, fmt.Errorf("create session: %w", err)
	}

	return LoginResult{
		Token:     token,
		ExpiresAt: expirationTime,
	}, nil
}

func (s *AuthService) Authenticate(ctx context.Context, token string) (uuid.UUID, error) {
	if token == "" {
		return uuid.Nil(), ErrUnauthenticated
	}
	tokenHash := sessiontoken.Hash(token)

	session, err := s.userSessionRepo.FindSessionByTokenHash(ctx, tokenHash)

	if err != nil {
		if errors.Is(err, repositories.ErrUserSessionNotFound) {
			return uuid.Nil(), ErrUnauthenticated
		}

		return uuid.Nil(), fmt.Errorf("find user session by token: %w", err)
	}

	return session.UserID, nil
}

func (s *AuthService) Logout(ctx context.Context, token string) error {
	if token == "" {
		return nil
	}
	tokenHash := sessiontoken.Hash(token)

	if err := s.userSessionRepo.DeleteSessionByTokenHash(ctx, tokenHash); err != nil {
		return fmt.Errorf("delete session by token hash: %w", err)
	}

	return nil
}
