package services

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"
	"uuid"

	"github.com/fredsaggio/url-shortener/internal/models"
	"github.com/fredsaggio/url-shortener/internal/repositories"
	"github.com/fredsaggio/url-shortener/internal/sessiontoken"
)

var (
	ErrAccountHasNoPassword         = errors.New("account has no password")
	ErrIncorrectCurrentPassword     = errors.New("current password is incorrect")
	ErrPasswordConfirmationMismatch = errors.New("password confirmation does not match")
	ErrSamePassword                 = errors.New("new password equals the current one")
	ErrAccountEmailMismatch         = errors.New("typed email does not match the account")
)

type AccountRepository interface {
	FindAccountByID(ctx context.Context, userID uuid.UUID) (models.Account, error)
	UpdateFullName(ctx context.Context, userID uuid.UUID, fullName string) error
	FindPasswordHashByUserID(ctx context.Context, userID uuid.UUID) (string, error)
	ReplacePasswordAndSessions(ctx context.Context, userID uuid.UUID, passwordHash string, sessionTokenHash []byte, sessionExpiresAt time.Time) error
	DeleteAccount(ctx context.Context, userID uuid.UUID) ([]string, error)
}

type SessionLookup interface {
	FindSessionByTokenHash(ctx context.Context, tokenHash []byte) (models.UserSession, error)
}

type PasswordHashComparator interface {
	Hash(password string) (string, error)
	Compare(password, encodedHash string) (bool, error)
}

type PasswordChangeResult struct {
	Session    LoginResult
	RememberMe bool
}

type AccountService struct {
	accounts      AccountRepository
	sessions      SessionLookup
	hasher        PasswordHashComparator
	generateToken TokenGenerator
	sessionTTL    time.Duration
	destinations  DestinationInvalidator
}

func NewAccountService(accounts AccountRepository, sessions SessionLookup, hasher PasswordHashComparator, generateToken TokenGenerator, sessionTTL time.Duration, destinations DestinationInvalidator) *AccountService {
	return &AccountService{
		accounts:      accounts,
		sessions:      sessions,
		hasher:        hasher,
		generateToken: generateToken,
		sessionTTL:    sessionTTL,
		destinations:  destinations,
	}
}

func (s *AccountService) GetAccount(ctx context.Context, userID uuid.UUID) (models.Account, error) {
	account, err := s.accounts.FindAccountByID(ctx, userID)
	if err != nil {
		return models.Account{}, fmt.Errorf("find account: %w", err)
	}
	return account, nil
}

func (s *AccountService) UpdateFullName(ctx context.Context, userID uuid.UUID, fullName string) (models.Account, error) {
	normalized, err := normalizeFullName(fullName)
	if err != nil {
		return models.Account{}, err
	}

	if err := s.accounts.UpdateFullName(ctx, userID, normalized); err != nil {
		return models.Account{}, fmt.Errorf("update full name: %w", err)
	}

	return s.GetAccount(ctx, userID)
}

// ChangePassword follows the OWASP guidance: the current password is required,
// and every session is revoked while the caller gets a new session ID. The new
// session keeps the old one's expiry, so a remembered login stays remembered.
func (s *AccountService) ChangePassword(ctx context.Context, userID uuid.UUID, sessionToken, currentPassword, newPassword, confirmation string) (PasswordChangeResult, error) {
	if newPassword != confirmation {
		return PasswordChangeResult{}, ErrPasswordConfirmationMismatch
	}
	if err := validateAccountPassword(newPassword); err != nil {
		return PasswordChangeResult{}, err
	}
	if currentPassword == "" || len(currentPassword) > maxPasswordBytes {
		return PasswordChangeResult{}, ErrIncorrectCurrentPassword
	}

	session, err := s.sessions.FindSessionByTokenHash(ctx, sessiontoken.Hash(sessionToken))
	if err != nil {
		if errors.Is(err, repositories.ErrUserSessionNotFound) {
			return PasswordChangeResult{}, ErrUnauthenticated
		}
		return PasswordChangeResult{}, fmt.Errorf("find current session: %w", err)
	}
	if session.UserID != userID {
		return PasswordChangeResult{}, ErrUnauthenticated
	}

	currentHash, err := s.accounts.FindPasswordHashByUserID(ctx, userID)
	if err != nil {
		if errors.Is(err, repositories.ErrPasswordCredentialNotFound) {
			return PasswordChangeResult{}, ErrAccountHasNoPassword
		}
		return PasswordChangeResult{}, fmt.Errorf("find password hash: %w", err)
	}

	match, err := s.hasher.Compare(currentPassword, currentHash)
	if err != nil {
		return PasswordChangeResult{}, fmt.Errorf("compare current password: %w", err)
	}
	if !match {
		return PasswordChangeResult{}, ErrIncorrectCurrentPassword
	}
	if newPassword == currentPassword {
		return PasswordChangeResult{}, ErrSamePassword
	}

	newHash, err := s.hasher.Hash(newPassword)
	if err != nil {
		return PasswordChangeResult{}, fmt.Errorf("hash new password: %w", err)
	}

	token, tokenHash, err := s.generateToken()
	if err != nil {
		return PasswordChangeResult{}, fmt.Errorf("generate session token: %w", err)
	}

	if err := s.accounts.ReplacePasswordAndSessions(ctx, userID, newHash, tokenHash, session.ExpiresAt); err != nil {
		if errors.Is(err, repositories.ErrPasswordCredentialNotFound) {
			return PasswordChangeResult{}, ErrAccountHasNoPassword
		}
		return PasswordChangeResult{}, fmt.Errorf("replace password and sessions: %w", err)
	}

	return PasswordChangeResult{
		Session:    LoginResult{Token: token, ExpiresAt: session.ExpiresAt},
		RememberMe: session.ExpiresAt.Sub(session.CreatedAt) > s.sessionTTL,
	}, nil
}

// DeleteAccount removes the account, its links and its sessions for good.
// An open session alone is not enough (OWASP Authentication Cheat Sheet):
// an account with a password confirms it, and a Google-only account, which
// has none, types its email. The cached destinations of the deleted links
// are dropped afterwards.
func (s *AccountService) DeleteAccount(ctx context.Context, userID uuid.UUID, password, email string) error {
	account, err := s.accounts.FindAccountByID(ctx, userID)
	if err != nil {
		if errors.Is(err, repositories.ErrUserNotFound) {
			return ErrUnauthenticated
		}
		return fmt.Errorf("find account to delete: %w", err)
	}

	if account.HasPassword {
		if password == "" || len(password) > maxPasswordBytes {
			return ErrIncorrectCurrentPassword
		}
		hash, err := s.accounts.FindPasswordHashByUserID(ctx, userID)
		if err != nil {
			return fmt.Errorf("find password hash: %w", err)
		}
		match, err := s.hasher.Compare(password, hash)
		if err != nil {
			return fmt.Errorf("compare password: %w", err)
		}
		if !match {
			return ErrIncorrectCurrentPassword
		}
	} else if normalizeEmail(email) != account.Email {
		return ErrAccountEmailMismatch
	}

	shortCodes, err := s.accounts.DeleteAccount(ctx, userID)
	if err != nil {
		if errors.Is(err, repositories.ErrUserNotFound) {
			return ErrUnauthenticated
		}
		return fmt.Errorf("delete account: %w", err)
	}

	if s.destinations != nil && len(shortCodes) > 0 {
		if err := s.destinations.Delete(ctx, shortCodes...); err != nil {
			slog.WarnContext(ctx, "drop cached destinations of a deleted account failed", "links", len(shortCodes), "error", err)
		}
	}

	return nil
}
