package services

import (
	"context"
	"fmt"
	"time"

	"github.com/fredsaggio/url-shortener/internal/models"
	"github.com/fredsaggio/url-shortener/internal/verificationcode"
)

const passwordResetResendCooldown = 30 * time.Second

type PasswordResetRepository interface {
	CreatePasswordResetAttempt(ctx context.Context, email string, attempt models.PasswordResetAttempt, resendAllowedBefore time.Time) (bool, error)
}

type PasswordResetCodeSender interface {
	SendPasswordResetCode(ctx context.Context, email, code string) error
}

type PasswordResetConfig struct {
	CodeTTL    time.Duration
	AttemptTTL time.Duration
}

type PasswordResetStartResult struct {
	Token string
}

type PasswordResetService struct {
	repo          PasswordResetRepository
	sendCode      PasswordResetCodeSender
	generateToken TokenGenerator
	generateCode  VerificationCodeGenerator
	config        PasswordResetConfig
}

func NewPasswordResetService(repo PasswordResetRepository, sendCode PasswordResetCodeSender, generateToken TokenGenerator, generateCode VerificationCodeGenerator, config PasswordResetConfig) *PasswordResetService {
	return &PasswordResetService{
		repo:          repo,
		sendCode:      sendCode,
		generateToken: generateToken,
		generateCode:  generateCode,
		config:        config,
	}
}

func (s *PasswordResetService) Start(ctx context.Context, email, currentToken string) (PasswordResetStartResult, error) {
	normalizedEmail := normalizeEmail(email)

	if !isValidEmail(normalizedEmail) {
		return PasswordResetStartResult{}, ErrInvalidEmail
	}

	token, tokenHash, err := s.generateToken()

	if err != nil {
		return PasswordResetStartResult{}, fmt.Errorf("generate password reset token: %w", err)
	}

	code, err := s.generateCode()

	if err != nil {
		return PasswordResetStartResult{}, fmt.Errorf("generate password reset code: %w", err)
	}

	now := time.Now().UTC()
	attempt := models.PasswordResetAttempt{
		TokenHash:             tokenHash,
		VerificationProofHash: verificationcode.Proof(token, code),
		LastCodeSentAt:        now,
		CodeExpiresAt:         now.Add(s.config.CodeTTL),
		AttemptExpiresAt:      now.Add(s.config.AttemptTTL),
	}

	saved, err := s.repo.CreatePasswordResetAttempt(ctx, normalizedEmail, attempt, now.Add(-passwordResetResendCooldown))

	if err != nil {
		return PasswordResetStartResult{}, fmt.Errorf("create password reset attempt: %w", err)
	}

	if !saved {
		if currentToken != "" {
			return PasswordResetStartResult{Token: currentToken}, nil
		}

		return PasswordResetStartResult{Token: token}, nil
	}

	if err := s.sendCode.SendPasswordResetCode(ctx, normalizedEmail, code); err != nil {
		return PasswordResetStartResult{}, fmt.Errorf("send password reset code: %w", err)
	}

	return PasswordResetStartResult{Token: token}, nil
}
