package services

import (
	"context"
	"errors"
	"fmt"
	"net/mail"
	"strings"
	"unicode/utf8"
	"uuid"

	"github.com/fredsaggio/url-shortener/internal/models"
	"github.com/fredsaggio/url-shortener/internal/repositories"
)

const (
	maxEmailBytes         = 254
	minPasswordCharacters = 8
	maxPasswordBytes      = 1024
)

var (
	ErrInvalidEmail       = errors.New("invalid email")
	ErrPasswordTooShort   = errors.New("password is too short")
	ErrPasswordTooLong    = errors.New("password is too long")
	ErrEmailAlreadyExists = errors.New("email already exists")
)

type UserRepository interface {
	CreateWithPassword(ctx context.Context, email, passwordHash string) (models.User, error)
	FindByID(ctx context.Context, userID uuid.UUID) (models.User, error)
}

type PasswordHasher interface {
	Hash(password string) (string, error)
}

type UserService struct {
	userRepo UserRepository
	hasher   PasswordHasher
}

func NewUserService(userRepo UserRepository, hasher PasswordHasher) *UserService {
	return &UserService{
		userRepo: userRepo,
		hasher:   hasher,
	}
}

func (s *UserService) RegisterWithPassword(ctx context.Context, email, password string) (models.User, error) {
	normalizedEmail := normalizeEmail(email)

	if !isValidEmail(normalizedEmail) {
		return models.User{}, ErrInvalidEmail
	}

	// O len() conta em bytes, em senhas que usam acentos e emojis, usar len() contaria errado (emojis tem 4 bytes e letras com acento tem 2 bytes)
	if utf8.RuneCountInString(password) < minPasswordCharacters {
		return models.User{}, ErrPasswordTooShort
	}

	// Limite técnico do argon2
	if len(password) > maxPasswordBytes {
		return models.User{}, ErrPasswordTooLong
	}

	passwordHash, err := s.hasher.Hash(password)

	if err != nil {
		return models.User{}, fmt.Errorf("hash password: %w", err)
	}

	user, err := s.userRepo.CreateWithPassword(ctx, normalizedEmail, passwordHash)

	if err != nil {
		if errors.Is(err, repositories.ErrEmailAlreadyExists) {
			return models.User{}, ErrEmailAlreadyExists
		}
		return models.User{}, fmt.Errorf("create user with password: %w", err)
	}

	return user, nil

}

func normalizeEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}

func isValidEmail(email string) bool {
	if email == "" || len(email) > maxEmailBytes {
		return false
	}

	address, err := mail.ParseAddress(email)

	if err != nil {
		return false
	}

	return address.Address == email
}

func (s *UserService) GetByID(ctx context.Context, userID uuid.UUID) (models.User, error) {
	user, err := s.userRepo.FindByID(ctx, userID)

	if err != nil {
		return models.User{}, fmt.Errorf("get user by ID: %w", err)
	}

	return user, nil
}
