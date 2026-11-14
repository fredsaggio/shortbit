package services_test

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/fredsaggio/url-shortener/internal/repositories"
	"github.com/fredsaggio/url-shortener/internal/services"
)

type linkURLRepositoryStub struct {
	findFunc func(context.Context, string) (int64, string, error)
}

func (s linkURLRepositoryStub) FindPrivateByShortCode(ctx context.Context, shortCode string) (int64, string, error) {
	return s.findFunc(ctx, shortCode)
}

type linkAccessSessionRepositoryStub struct {
	createFunc func(context.Context, int64, []byte, time.Time) error
}

func (s linkAccessSessionRepositoryStub) CreateLinkAccessSession(ctx context.Context, linkID int64, tokenHash []byte, expiresAt time.Time) error {
	return s.createFunc(ctx, linkID, tokenHash, expiresAt)
}

type linkPasswordComparatorStub func(string, string) (bool, error)

func (compare linkPasswordComparatorStub) Compare(password, encodedHash string) (bool, error) {
	return compare(password, encodedHash)
}

func TestLinkAccessSessionServiceCreateSession(t *testing.T) {
	const (
		shortCode    = "Ab3dX9"
		password     = "senha-do-link"
		passwordHash = "$argon2id$private-link-test-hash"
		rawToken     = "raw-link-access-token"
		linkID       = int64(42)
		sessionTTL   = 15 * time.Minute
	)
	tokenHash := []byte("hashed-link-access-token")
	type contextKey struct{}
	ctx := context.WithValue(t.Context(), contextKey{}, "request-context")
	var calls []string
	var persistedExpiresAt time.Time

	urlRepo := linkURLRepositoryStub{findFunc: func(gotCtx context.Context, gotCode string) (int64, string, error) {
		calls = append(calls, "find")
		if gotCtx.Value(contextKey{}) != "request-context" || gotCode != shortCode {
			t.Errorf("FindPrivateByShortCode() arguments = (%v, %q), want request context and %q", gotCtx, gotCode, shortCode)
		}
		return linkID, passwordHash, nil
	}}
	comparator := linkPasswordComparatorStub(func(gotPassword, gotHash string) (bool, error) {
		calls = append(calls, "compare")
		if gotPassword != password || gotHash != passwordHash {
			t.Errorf("Compare() arguments = (%q, %q), want password and stored hash", gotPassword, gotHash)
		}
		return true, nil
	})
	generateToken := func() (string, []byte, error) {
		calls = append(calls, "generate")
		return rawToken, tokenHash, nil
	}
	sessionRepo := linkAccessSessionRepositoryStub{createFunc: func(gotCtx context.Context, gotLinkID int64, gotHash []byte, expiresAt time.Time) error {
		calls = append(calls, "create")
		if gotCtx.Value(contextKey{}) != "request-context" || gotLinkID != linkID || !bytes.Equal(gotHash, tokenHash) {
			t.Errorf("CreateLinkAccessSession() arguments = (%v, %d, %x), want request context, link ID and token hash", gotCtx, gotLinkID, gotHash)
		}
		persistedExpiresAt = expiresAt
		return nil
	}}
	service := services.NewLinkAccessSessionService(urlRepo, sessionRepo, sessionTTL, generateToken, comparator)

	before := time.Now().UTC()
	result, err := service.CreateSession(ctx, shortCode, password)
	after := time.Now().UTC()
	if err != nil {
		t.Fatalf("CreateSession() error = %v", err)
	}
	if result.Token != rawToken || !result.ExpiresAt.Equal(persistedExpiresAt) {
		t.Errorf("CreateSession() = %+v, want raw token and persisted expiry %v", result, persistedExpiresAt)
	}
	if result.ExpiresAt.Before(before.Add(sessionTTL)) || result.ExpiresAt.After(after.Add(sessionTTL)) {
		t.Errorf("expiresAt = %v, want between %v and %v", result.ExpiresAt, before.Add(sessionTTL), after.Add(sessionTTL))
	}
	if got := strings.Join(calls, ","); got != "find,compare,generate,create" {
		t.Errorf("calls = %q, want find,compare,generate,create", got)
	}
}

func TestLinkAccessSessionServiceCreateSessionErrors(t *testing.T) {
	findErr := errors.New("find failed")
	compareErr := errors.New("compare failed")
	tokenErr := errors.New("token generation failed")
	createErr := errors.New("create failed")
	tests := []struct {
		name       string
		findErr    error
		compareErr error
		matches    bool
		tokenErr   error
		createErr  error
		wantErr    error
		wantCalls  string
	}{
		{name: "private link not found", findErr: repositories.ErrURLNotFound, wantErr: services.ErrURLNotFound, wantCalls: "find"},
		{name: "wrapped private link not found", findErr: errors.Join(errors.New("context"), repositories.ErrURLNotFound), wantErr: services.ErrURLNotFound, wantCalls: "find"},
		{name: "repository failure", findErr: findErr, wantErr: findErr, wantCalls: "find"},
		{name: "password comparison failure", compareErr: compareErr, wantErr: compareErr, wantCalls: "find,compare"},
		{name: "incorrect link password", wantErr: services.ErrIncorrectLinkPassword, wantCalls: "find,compare"},
		{name: "token generation failure", matches: true, tokenErr: tokenErr, wantErr: tokenErr, wantCalls: "find,compare,generate"},
		{name: "session persistence failure", matches: true, createErr: createErr, wantErr: createErr, wantCalls: "find,compare,generate,create"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var calls []string
			urlRepo := linkURLRepositoryStub{findFunc: func(context.Context, string) (int64, string, error) {
				calls = append(calls, "find")
				return 42, "stored-password-hash", tt.findErr
			}}
			comparator := linkPasswordComparatorStub(func(string, string) (bool, error) {
				calls = append(calls, "compare")
				return tt.matches, tt.compareErr
			})
			generateToken := func() (string, []byte, error) {
				calls = append(calls, "generate")
				return "raw-token", []byte("token-hash"), tt.tokenErr
			}
			sessionRepo := linkAccessSessionRepositoryStub{createFunc: func(context.Context, int64, []byte, time.Time) error {
				calls = append(calls, "create")
				return tt.createErr
			}}
			service := services.NewLinkAccessSessionService(urlRepo, sessionRepo, 15*time.Minute, generateToken, comparator)

			result, err := service.CreateSession(t.Context(), "Ab3dX9", "senha-do-link")
			if !errors.Is(err, tt.wantErr) || result != (services.LinkAccessSessionResult{}) {
				t.Errorf("CreateSession() = (%+v, %v), want empty result and %v", result, err, tt.wantErr)
			}
			if got := strings.Join(calls, ","); got != tt.wantCalls {
				t.Errorf("calls = %q, want %q", got, tt.wantCalls)
			}
		})
	}
}
