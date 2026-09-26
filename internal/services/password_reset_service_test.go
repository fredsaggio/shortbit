package services_test

import (
	"bytes"
	"context"
	"errors"
	"testing"
	"time"

	"github.com/fredsaggio/url-shortener/internal/models"
	"github.com/fredsaggio/url-shortener/internal/services"
	"github.com/fredsaggio/url-shortener/internal/verificationcode"
)

type passwordResetRepositoryStub struct {
	createFunc func(context.Context, string, models.PasswordResetAttempt, time.Time) (bool, error)
}

func (s passwordResetRepositoryStub) CreatePasswordResetAttempt(ctx context.Context, email string, attempt models.PasswordResetAttempt, resendAllowedBefore time.Time) (bool, error) {
	return s.createFunc(ctx, email, attempt, resendAllowedBefore)
}

type passwordResetCodeSenderStub struct {
	sendFunc func(context.Context, string, string) error
}

func (s passwordResetCodeSenderStub) SendPasswordResetCode(ctx context.Context, email, code string) error {
	return s.sendFunc(ctx, email, code)
}

func newPasswordResetService(repo passwordResetRepositoryStub, sender passwordResetCodeSenderStub, generateToken services.TokenGenerator, generateCode services.VerificationCodeGenerator) *services.PasswordResetService {
	return services.NewPasswordResetService(repo, sender, generateToken, generateCode, services.PasswordResetConfig{
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
