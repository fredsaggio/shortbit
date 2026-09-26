package services_test

import (
	"bytes"
	"context"
	"errors"
	"testing"
	"time"

	"github.com/fredsaggio/url-shortener/internal/models"
	"github.com/fredsaggio/url-shortener/internal/repositories"
	"github.com/fredsaggio/url-shortener/internal/services"
	"github.com/fredsaggio/url-shortener/internal/sessiontoken"
	"github.com/fredsaggio/url-shortener/internal/verificationcode"
)

type passwordResetRepositoryStub struct {
	createFunc        func(context.Context, string, models.PasswordResetAttempt, time.Time) (bool, error)
	findFunc          func(context.Context, []byte) (models.PasswordResetAttempt, error)
	recordFailureFunc func(context.Context, []byte, time.Time, time.Time, time.Time, int16) (int16, *time.Time, error)
	completeFunc      func(context.Context, []byte, []byte, string) (string, error)
}

func (s passwordResetRepositoryStub) CreatePasswordResetAttempt(ctx context.Context, email string, attempt models.PasswordResetAttempt, resendAllowedBefore time.Time) (bool, error) {
	return s.createFunc(ctx, email, attempt, resendAllowedBefore)
}

func (s passwordResetRepositoryStub) FindPasswordResetAttemptByTokenHash(ctx context.Context, tokenHash []byte) (models.PasswordResetAttempt, error) {
	return s.findFunc(ctx, tokenHash)
}

func (s passwordResetRepositoryStub) RecordPasswordResetFailure(ctx context.Context, tokenHash []byte, now, retryAt, lockUntil time.Time, maxAttempts int16) (int16, *time.Time, error) {
	return s.recordFailureFunc(ctx, tokenHash, now, retryAt, lockUntil, maxAttempts)
}

func (s passwordResetRepositoryStub) CompletePasswordReset(ctx context.Context, tokenHash, proofHash []byte, passwordHash string) (string, error) {
	return s.completeFunc(ctx, tokenHash, proofHash, passwordHash)
}

type passwordResetCodeSenderStub struct {
	sendFunc   func(context.Context, string, string) error
	noticeFunc func(context.Context, string) error
}

func (s passwordResetCodeSenderStub) SendPasswordResetCode(ctx context.Context, email, code string) error {
	return s.sendFunc(ctx, email, code)
}

func (s passwordResetCodeSenderStub) SendPasswordChangedNotice(ctx context.Context, email string) error {
	if s.noticeFunc != nil {
		return s.noticeFunc(ctx, email)
	}
	return nil
}

func newPasswordResetService(repo passwordResetRepositoryStub, sender passwordResetCodeSenderStub, generateToken services.TokenGenerator, generateCode services.VerificationCodeGenerator) *services.PasswordResetService {
	return services.NewPasswordResetService(repo, successfulPasswordHasher(), sender, generateToken, generateCode, services.PasswordResetConfig{
		CodeTTL: 10 * time.Minute, AttemptTTL: 30 * time.Minute,
	})
}

func TestPasswordResetServiceStartSavesAttemptAndSendsCode(t *testing.T) {
	const inputEmail, normalizedEmail = "  USER@Example.COM  ", "user@example.com"
	const rawToken, code = "password-reset-token", "00123456"
	tokenHash := bytes.Repeat([]byte{0x42}, 32)
	type contextKey struct{}
	ctx := context.WithValue(t.Context(), contextKey{}, "request-context")
	before := time.Now().UTC()
	saved := false

	repo := passwordResetRepositoryStub{createFunc: func(gotCtx context.Context, gotEmail string, attempt models.PasswordResetAttempt, resendAllowedBefore time.Time) (bool, error) {
		if gotCtx.Value(contextKey{}) != "request-context" {
			t.Error("repository did not receive request context")
		}
		if gotEmail != normalizedEmail {
			t.Errorf("repository email = %q, want %q", gotEmail, normalizedEmail)
		}
		if !bytes.Equal(attempt.TokenHash, tokenHash) {
			t.Errorf("token hash = %x, want %x", attempt.TokenHash, tokenHash)
		}
		if !verificationcode.Matches(rawToken, code, attempt.VerificationProofHash) {
			t.Error("stored proof does not match token and code")
		}
		if attempt.LastCodeSentAt.Before(before) || attempt.LastCodeSentAt.After(time.Now().UTC()) {
			t.Errorf("LastCodeSentAt = %v, outside request time", attempt.LastCodeSentAt)
		}
		if !attempt.CodeExpiresAt.Equal(attempt.LastCodeSentAt.Add(10*time.Minute)) || !attempt.AttemptExpiresAt.Equal(attempt.LastCodeSentAt.Add(30*time.Minute)) {
			t.Errorf("attempt expiration times = (%v, %v)", attempt.CodeExpiresAt, attempt.AttemptExpiresAt)
		}
		if !resendAllowedBefore.Equal(attempt.LastCodeSentAt.Add(-30 * time.Second)) {
			t.Errorf("resendAllowedBefore = %v, want 30 seconds before creation", resendAllowedBefore)
		}
		saved = true
		return true, nil
	}}
	sender := passwordResetCodeSenderStub{sendFunc: func(gotCtx context.Context, gotEmail, gotCode string) error {
		if !saved {
			t.Error("code sent before attempt was saved")
		}
		if gotCtx.Value(contextKey{}) != "request-context" {
			t.Error("sender did not receive request context")
		}
		if gotEmail != normalizedEmail || gotCode != code {
			t.Errorf("sent (%q, %q), want (%q, %q)", gotEmail, gotCode, normalizedEmail, code)
		}
		return nil
	}}
	service := newPasswordResetService(repo, sender,
		func() (string, []byte, error) { return rawToken, tokenHash, nil },
		func() (string, error) { return code, nil },
	)

	result, err := service.Start(ctx, inputEmail, "old-cookie-token")
	if err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	if result.Token != rawToken {
		t.Errorf("Start() token = %q, want %q", result.Token, rawToken)
	}
}

func TestPasswordResetServiceStartSuppressedAttemptDoesNotSendCode(t *testing.T) {
	for _, tt := range []struct {
		name, currentToken, wantToken string
	}{
		{name: "preserves existing cookie", currentToken: "old-cookie-token", wantToken: "old-cookie-token"},
		{name: "provides dummy cookie without existing cookie", wantToken: "new-random-token"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			repo := passwordResetRepositoryStub{createFunc: func(context.Context, string, models.PasswordResetAttempt, time.Time) (bool, error) {
				return false, nil
			}}
			sender := passwordResetCodeSenderStub{sendFunc: func(context.Context, string, string) error {
				t.Fatal("code must not be sent when attempt was not saved")
				return nil
			}}
			service := newPasswordResetService(repo, sender,
				func() (string, []byte, error) { return "new-random-token", bytes.Repeat([]byte{0x21}, 32), nil },
				func() (string, error) { return "12345678", nil },
			)

			result, err := service.Start(t.Context(), "user@example.com", tt.currentToken)
			if err != nil || result.Token != tt.wantToken {
				t.Fatalf("Start() = (%q, %v), want (%q, nil)", result.Token, err, tt.wantToken)
			}
		})
	}
}

func TestPasswordResetServiceStartRejectsInvalidEmail(t *testing.T) {
	service := newPasswordResetService(
		passwordResetRepositoryStub{createFunc: func(context.Context, string, models.PasswordResetAttempt, time.Time) (bool, error) {
			t.Fatal("repository called with invalid email")
			return false, nil
		}},
		passwordResetCodeSenderStub{sendFunc: func(context.Context, string, string) error {
			t.Fatal("sender called with invalid email")
			return nil
		}},
		func() (string, []byte, error) { t.Fatal("token generated for invalid email"); return "", nil, nil },
		func() (string, error) { t.Fatal("code generated for invalid email"); return "", nil },
	)

	_, err := service.Start(t.Context(), "not-an-email", "")
	if !errors.Is(err, services.ErrInvalidEmail) {
		t.Fatalf("Start() error = %v, want ErrInvalidEmail", err)
	}
}

func TestPasswordResetServiceStartPropagatesFailures(t *testing.T) {
	wantErr := errors.New("dependency failed")
	tests := []struct {
		name  string
		stage string
	}{
		{name: "token generator", stage: "token"},
		{name: "code generator", stage: "code"},
		{name: "repository", stage: "repository"},
		{name: "email sender", stage: "sender"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := passwordResetRepositoryStub{createFunc: func(context.Context, string, models.PasswordResetAttempt, time.Time) (bool, error) {
				if tt.stage == "repository" {
					return false, wantErr
				}
				return true, nil
			}}
			sender := passwordResetCodeSenderStub{sendFunc: func(context.Context, string, string) error {
				if tt.stage == "sender" {
					return wantErr
				}
				return nil
			}}
			service := newPasswordResetService(repo, sender,
				func() (string, []byte, error) {
					if tt.stage == "token" {
						return "", nil, wantErr
					}
					return "raw-token", bytes.Repeat([]byte{0x33}, 32), nil
				},
				func() (string, error) {
					if tt.stage == "code" {
						return "", wantErr
					}
					return "12345678", nil
				},
			)

			result, err := service.Start(t.Context(), "user@example.com", "")
			if !errors.Is(err, wantErr) || result.Token != "" {
				t.Fatalf("Start() = (%q, %v), want (empty token, wrapped error)", result.Token, err)
			}
		})
	}
}

func validPasswordResetAttempt(token, code string) models.PasswordResetAttempt {
	now := time.Now().UTC()
	return models.PasswordResetAttempt{
		TokenHash: sessiontoken.Hash(token), VerificationProofHash: verificationcode.Proof(token, code),
		LastCodeSentAt: now, CodeExpiresAt: now.Add(10 * time.Minute), AttemptExpiresAt: now.Add(30 * time.Minute),
	}
}

func TestPasswordResetServiceConfirmChangesPasswordAfterMatchingCode(t *testing.T) {
	const token, code, password = "reset-token", "00123456", "nova-senha-segura"
	attempt := validPasswordResetAttempt(token, code)
	completed := false
	repo := passwordResetRepositoryStub{
		findFunc: func(_ context.Context, tokenHash []byte) (models.PasswordResetAttempt, error) {
			if !bytes.Equal(tokenHash, attempt.TokenHash) {
				t.Errorf("lookup hash = %x", tokenHash)
			}
			return attempt, nil
		},
		completeFunc: func(_ context.Context, tokenHash, proofHash []byte, passwordHash string) (string, error) {
			completed = true
			if !bytes.Equal(tokenHash, attempt.TokenHash) || !bytes.Equal(proofHash, attempt.VerificationProofHash) || passwordHash != "$argon2id$test-hash" {
				t.Errorf("CompletePasswordReset() = (%x, %x, %q)", tokenHash, proofHash, passwordHash)
			}
			return "user@example.com", nil
		},
	}
	noticeSent := false
	sender := passwordResetCodeSenderStub{noticeFunc: func(_ context.Context, email string) error {
		noticeSent = true
		if email != "user@example.com" {
			t.Errorf("notice email = %q", email)
		}
		return nil
	}}
	service := newPasswordResetService(repo, sender, nil, nil)
	if err := service.Confirm(t.Context(), token, code, password); err != nil {
		t.Fatalf("Confirm() error = %v", err)
	}
	if !completed {
		t.Fatal("CompletePasswordReset() was not called")
	}
	if !noticeSent {
		t.Fatal("password changed notice was not sent")
	}
}

func TestPasswordResetServiceConfirmIgnoresNoticeFailureAfterCommit(t *testing.T) {
	const token, code = "notice-failure-token", "12345678"
	attempt := validPasswordResetAttempt(token, code)
	repo := passwordResetRepositoryStub{
		findFunc:     func(context.Context, []byte) (models.PasswordResetAttempt, error) { return attempt, nil },
		completeFunc: func(context.Context, []byte, []byte, string) (string, error) { return "user@example.com", nil },
	}
	sender := passwordResetCodeSenderStub{noticeFunc: func(context.Context, string) error { return errors.New("provider unavailable") }}
	service := newPasswordResetService(repo, sender, nil, nil)
	if err := service.Confirm(t.Context(), token, code, "nova-senha-segura"); err != nil {
		t.Fatalf("Confirm() error = %v, want success after committed password change", err)
	}
}

func TestPasswordResetServiceConfirmRecordsWrongCode(t *testing.T) {
	const token = "reset-token-wrong-code"
	attempt := validPasswordResetAttempt(token, "12345678")
	for _, tt := range []struct {
		name     string
		failures int16
		want     error
	}{
		{name: "ordinary failure", failures: 1, want: services.ErrPasswordResetCodeInvalid},
		{name: "fifth failure", failures: 0, want: services.ErrPasswordResetAttemptLocked},
	} {
		t.Run(tt.name, func(t *testing.T) {
			repo := passwordResetRepositoryStub{
				findFunc: func(context.Context, []byte) (models.PasswordResetAttempt, error) { return attempt, nil },
				recordFailureFunc: func(_ context.Context, tokenHash []byte, now, retryAt, lockUntil time.Time, maxAttempts int16) (int16, *time.Time, error) {
					if !bytes.Equal(tokenHash, attempt.TokenHash) || !retryAt.Equal(now.Add(30*time.Second)) || !lockUntil.Equal(now.Add(5*time.Minute)) || maxAttempts != 5 {
						t.Error("failure policy or token hash is incorrect")
					}
					return tt.failures, &retryAt, nil
				},
				completeFunc: func(context.Context, []byte, []byte, string) (string, error) {
					t.Fatal("wrong code must not change password")
					return "", nil
				},
			}
			service := newPasswordResetService(repo, passwordResetCodeSenderStub{}, nil, nil)
			if err := service.Confirm(t.Context(), token, "87654321", "nova-senha-segura"); !errors.Is(err, tt.want) {
				t.Fatalf("Confirm() error = %v, want %v", err, tt.want)
			}
		})
	}
}

func TestPasswordResetServiceConfirmRejectsUnavailableAndExpiredAttempts(t *testing.T) {
	const token, code = "reset-token-state", "12345678"
	now := time.Now().UTC()
	usedAt := now.Add(-time.Minute)
	lockedUntil := now.Add(time.Minute)
	tests := []struct {
		name   string
		change func(*models.PasswordResetAttempt)
		want   error
	}{
		{name: "used", change: func(a *models.PasswordResetAttempt) { a.UsedAt = &usedAt }, want: services.ErrPasswordResetAttemptUnavailable},
		{name: "attempt expired", change: func(a *models.PasswordResetAttempt) { a.AttemptExpiresAt = now.Add(-time.Second) }, want: services.ErrPasswordResetAttemptUnavailable},
		{name: "locked", change: func(a *models.PasswordResetAttempt) { a.LockedUntil = &lockedUntil }, want: services.ErrPasswordResetAttemptLocked},
		{name: "code expired", change: func(a *models.PasswordResetAttempt) { a.CodeExpiresAt = now.Add(-time.Second) }, want: services.ErrPasswordResetCodeExpired},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			attempt := validPasswordResetAttempt(token, code)
			tt.change(&attempt)
			repo := passwordResetRepositoryStub{findFunc: func(context.Context, []byte) (models.PasswordResetAttempt, error) { return attempt, nil }}
			service := newPasswordResetService(repo, passwordResetCodeSenderStub{}, nil, nil)
			if err := service.Confirm(t.Context(), token, code, "nova-senha-segura"); !errors.Is(err, tt.want) {
				t.Fatalf("Confirm() error = %v, want %v", err, tt.want)
			}
		})
	}
}

func TestPasswordResetServiceConfirmRejectsMissingTokenAndInvalidPassword(t *testing.T) {
	service := newPasswordResetService(passwordResetRepositoryStub{}, passwordResetCodeSenderStub{}, nil, nil)
	for _, tt := range []struct {
		token, password string
		want            error
	}{
		{token: "", password: "nova-senha-segura", want: services.ErrPasswordResetAttemptUnavailable},
		{token: "reset-token", password: "curta", want: services.ErrPasswordTooShort},
		{token: "reset-token", password: string(bytes.Repeat([]byte{'x'}, 1025)), want: services.ErrPasswordTooLong},
	} {
		if err := service.Confirm(t.Context(), tt.token, "12345678", tt.password); !errors.Is(err, tt.want) {
			t.Errorf("Confirm(%q, password length %d) error = %v, want %v", tt.token, len(tt.password), err, tt.want)
		}
	}
}

func TestPasswordResetServiceConfirmMapsMissingAttempt(t *testing.T) {
	repo := passwordResetRepositoryStub{findFunc: func(context.Context, []byte) (models.PasswordResetAttempt, error) {
		return models.PasswordResetAttempt{}, repositories.ErrPasswordResetAttemptNotFound
	}}
	service := newPasswordResetService(repo, passwordResetCodeSenderStub{}, nil, nil)
	if err := service.Confirm(t.Context(), "missing-token", "12345678", "nova-senha-segura"); !errors.Is(err, services.ErrPasswordResetAttemptUnavailable) {
		t.Fatalf("Confirm() error = %v, want unavailable", err)
	}
}
