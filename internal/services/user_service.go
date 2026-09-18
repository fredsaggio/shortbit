package services

import (
	"context"
	"errors"
	"fmt"
	"net/mail"
	"strings"
	"time"
	"unicode/utf8"
	"uuid"

	"github.com/fredsaggio/url-shortener/internal/models"
	"github.com/fredsaggio/url-shortener/internal/verificationcode"
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
	UserExistsByEmail(ctx context.Context, email string) (bool, error)
	CreatePasswordRegistrationAttempt(ctx context.Context, attempt models.PasswordRegistrationAttempt) error
	FindByID(ctx context.Context, userID uuid.UUID) (models.User, error)
}

type PasswordHasher interface {
	Hash(password string) (string, error)
}

type VerificationCodeGenerator func() (string, error)

type PasswordRegistrationCodeSender interface {
	SendPasswordRegistrationCode(ctx context.Context, email, code string) error
}

type PasswordRegistrationConfig struct {
	CodeTTL    time.Duration
	AttemptTTL time.Duration
}

type PasswordRegistrationResult struct {
	Token     string
	ExpiresAt time.Time
}

type UserService struct {
	userRepo                 UserRepository
	hasher                   PasswordHasher
	generateToken            TokenGenerator
	generateVerificationCode VerificationCodeGenerator
	codeSender               PasswordRegistrationCodeSender
	registrationConfig       PasswordRegistrationConfig
}

func NewUserService(userRepo UserRepository, hasher PasswordHasher, generateToken TokenGenerator, generateVerificationCode VerificationCodeGenerator, codeSender PasswordRegistrationCodeSender, registrationConfig PasswordRegistrationConfig) *UserService {
	return &UserService{
		userRepo:                 userRepo,
		hasher:                   hasher,
		generateToken:            generateToken,
		generateVerificationCode: generateVerificationCode,
		codeSender:               codeSender,
		registrationConfig:       registrationConfig,
	}
}

func (s *UserService) StartPasswordRegistration(ctx context.Context, email, password string) (PasswordRegistrationResult, error) {
	normalizedEmail := normalizeEmail(email)

	if !isValidEmail(normalizedEmail) {
		return PasswordRegistrationResult{}, ErrInvalidEmail
	}

	if utf8.RuneCountInString(password) < minPasswordCharacters {
		return PasswordRegistrationResult{}, ErrPasswordTooShort
	}

	if len(password) > maxPasswordBytes {
		return PasswordRegistrationResult{}, ErrPasswordTooLong
	}

	exists, err := s.userRepo.UserExistsByEmail(ctx, normalizedEmail)

	if err != nil {
		return PasswordRegistrationResult{}, fmt.Errorf("check user existence: %w", err)
	}

	if exists {
		return PasswordRegistrationResult{}, ErrEmailAlreadyExists
	}

	passwordHash, err := s.hasher.Hash(password)

	if err != nil {
		return PasswordRegistrationResult{}, fmt.Errorf("hash password: %w", err)
	}

	token, tokenHash, err := s.generateToken()

	if err != nil {
		return PasswordRegistrationResult{}, fmt.Errorf("generate registration token: %w", err)
	}

	code, err := s.generateVerificationCode()

	if err != nil {
		return PasswordRegistrationResult{}, fmt.Errorf("generate verification code: %w", err)
	}

	now := time.Now().UTC()
	attemptExpiresAt := now.Add(s.registrationConfig.AttemptTTL)

	attempt := models.PasswordRegistrationAttempt{
		TokenHash:             tokenHash,
		Email:                 normalizedEmail,
		PasswordHash:          passwordHash,
		VerificationProofHash: verificationcode.Proof(token, code),
		LastCodeSentAt:        now,
		CodeExpiresAt:         now.Add(s.registrationConfig.CodeTTL),
		AttemptExpiresAt:      attemptExpiresAt,
	}

	if err := s.userRepo.CreatePasswordRegistrationAttempt(ctx, attempt); err != nil {
		return PasswordRegistrationResult{}, fmt.Errorf("create password registration attempt: %w", err)
	}

	if err := s.codeSender.SendPasswordRegistrationCode(ctx, normalizedEmail, code); err != nil {
		return PasswordRegistrationResult{}, fmt.Errorf("send password registration code: %w", err)
	}

	return PasswordRegistrationResult{
		Token:     token,
		ExpiresAt: attemptExpiresAt,
	}, nil
}

func (s *UserService) GetByID(ctx context.Context, userID uuid.UUID) (models.User, error) {
	user, err := s.userRepo.FindByID(ctx, userID)

	if err != nil {
		return models.User{}, fmt.Errorf("get user by ID: %w", err)
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
