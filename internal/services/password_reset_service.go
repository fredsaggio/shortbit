package services

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"
	"unicode/utf8"

	"github.com/fredsaggio/url-shortener/internal/models"
	"github.com/fredsaggio/url-shortener/internal/repositories"
	"github.com/fredsaggio/url-shortener/internal/sessiontoken"
	"github.com/fredsaggio/url-shortener/internal/verificationcode"
)

const (
	passwordResetResendCooldown            = 30 * time.Second
	passwordResetFailureCooldown           = 30 * time.Second
	passwordResetFailureLockDuration       = 5 * time.Minute
	passwordChangedNoticeTimeout           = 5 * time.Second
	maxPasswordResetFailedAttempts   int16 = 5
)

var (
	ErrPasswordResetAttemptUnavailable = errors.New("password reset attempt unavailable")
	ErrPasswordResetAttemptLocked      = errors.New("password reset attempt locked")
	ErrPasswordResetCodeExpired        = errors.New("password reset code expired")
	ErrPasswordResetCodeInvalid        = errors.New("password reset code invalid")
)

type PasswordResetRepository interface {
	CreatePasswordResetAttempt(ctx context.Context, email string, attempt models.PasswordResetAttempt, resendAllowedBefore time.Time) (bool, error)
	FindPasswordResetAttemptByTokenHash(ctx context.Context, tokenHash []byte) (models.PasswordResetAttempt, error)
	RecordPasswordResetFailure(ctx context.Context, tokenHash []byte, now, retryAt, lockUntil time.Time, maxAttempts int16) (int16, *time.Time, error)
	CompletePasswordReset(ctx context.Context, tokenHash, proofHash []byte, passwordHash string) (string, error)
}

type PasswordResetCodeSender interface {
	SendPasswordResetCode(ctx context.Context, email, code string) error
	SendPasswordChangedNotice(ctx context.Context, email string) error
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
	hasher        PasswordHasher
	sendCode      PasswordResetCodeSender
	generateToken TokenGenerator
	generateCode  VerificationCodeGenerator
	config        PasswordResetConfig
}

func NewPasswordResetService(repo PasswordResetRepository, hasher PasswordHasher, sendCode PasswordResetCodeSender, generateToken TokenGenerator, generateCode VerificationCodeGenerator, config PasswordResetConfig) *PasswordResetService {
	return &PasswordResetService{
		repo:          repo,
		hasher:        hasher,
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

func (s *PasswordResetService) Confirm(ctx context.Context, token, code, password string) error {
	if token == "" {
		return ErrPasswordResetAttemptUnavailable
	}
	if utf8.RuneCountInString(password) < minPasswordCharacters {
		return ErrPasswordTooShort
	}
	if len(password) > maxPasswordBytes {
		return ErrPasswordTooLong
	}

	tokenHash := sessiontoken.Hash(token)
	attempt, err := s.repo.FindPasswordResetAttemptByTokenHash(ctx, tokenHash)
	if err != nil {
		if errors.Is(err, repositories.ErrPasswordResetAttemptNotFound) {
			return ErrPasswordResetAttemptUnavailable
		}
		return fmt.Errorf("find password reset attempt: %w", err)
	}

	now := time.Now().UTC()
	if attempt.UsedAt != nil || !attempt.AttemptExpiresAt.After(now) {
		return ErrPasswordResetAttemptUnavailable
	}
	if attempt.LockedUntil != nil && attempt.LockedUntil.After(now) {
		return ErrPasswordResetAttemptLocked
	}
	if !attempt.CodeExpiresAt.After(now) {
		return ErrPasswordResetCodeExpired
	}
	if !verificationcode.Matches(token, code, attempt.VerificationProofHash) {
		failedAttempts, _, err := s.repo.RecordPasswordResetFailure(ctx, tokenHash, now,
			now.Add(passwordResetFailureCooldown), now.Add(passwordResetFailureLockDuration), maxPasswordResetFailedAttempts)
		if err != nil {
			if errors.Is(err, repositories.ErrPasswordResetAttemptNotFound) {
				return ErrPasswordResetAttemptUnavailable
			}
			return fmt.Errorf("record password reset failure: %w", err)
		}
		if failedAttempts == 0 {
			return ErrPasswordResetAttemptLocked
		}
		return ErrPasswordResetCodeInvalid
	}

	passwordHash, err := s.hasher.Hash(password)
	if err != nil {
		return fmt.Errorf("hash new password: %w", err)
	}
	email, err := s.repo.CompletePasswordReset(ctx, tokenHash, verificationcode.Proof(token, code), passwordHash)
	if err != nil {
		if errors.Is(err, repositories.ErrPasswordResetAttemptNotFound) {
			return ErrPasswordResetAttemptUnavailable
		}
		return fmt.Errorf("complete password reset: %w", err)
	}
	noticeCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), passwordChangedNoticeTimeout)
	defer cancel()
	if err := s.sendCode.SendPasswordChangedNotice(noticeCtx, email); err != nil {
		slog.ErrorContext(noticeCtx, "send password changed notice failed", "error", err)
	}
	return nil
}
