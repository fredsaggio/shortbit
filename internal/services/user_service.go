package services

import (
	"context"
	"errors"
	"net/mail"
	"strings"

	"github.com/fredsaggio/url-shortener/internal/models"
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

func (s *UserService) Register(ctx context.Context, email, password string) (models.User, error) {
	normalizedEmail := normalizeEmail(email)

	if !isValidEmail(normalizedEmail) {
		return models.User{}, ErrInvalidEmail
	}

	// temp 
	return models.User{}, nil
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
