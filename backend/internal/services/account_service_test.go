package services_test

import (
	"bytes"
	"context"
	"errors"
	"testing"
	"time"
	"uuid"

	"github.com/fredsaggio/url-shortener/internal/models"
	"github.com/fredsaggio/url-shortener/internal/repositories"
	"github.com/fredsaggio/url-shortener/internal/services"
	"github.com/fredsaggio/url-shortener/internal/sessiontoken"
)

const accountSessionTTL = 12 * time.Hour

var accountUserID = uuid.MustParse("01991f29-7c22-7ab3-a395-4d402f09c318")

type accountRepositoryStub struct {
	account      models.Account
	findErr      error
	passwordHash string
	hashErr      error
	replaceErr   error

	updatedName string
	replaced    *struct {
		hash      string
		tokenHash []byte
		expiresAt time.Time
	}
}

func (s *accountRepositoryStub) FindAccountByID(context.Context, uuid.UUID) (models.Account, error) {
	return s.account, s.findErr
}

func (s *accountRepositoryStub) UpdateFullName(_ context.Context, _ uuid.UUID, fullName string) error {
	s.updatedName = fullName
	s.account.FullName = &fullName
	return nil
}

func (s *accountRepositoryStub) FindPasswordHashByUserID(context.Context, uuid.UUID) (string, error) {
	return s.passwordHash, s.hashErr
}

func (s *accountRepositoryStub) ReplacePasswordAndSessions(_ context.Context, _ uuid.UUID, hash string, tokenHash []byte, expiresAt time.Time) error {
	s.replaced = &struct {
		hash      string
		tokenHash []byte
		expiresAt time.Time
	}{hash, tokenHash, expiresAt}
	return s.replaceErr
}

type sessionLookupStub struct {
	session models.UserSession
	err     error
}

func (s sessionLookupStub) FindSessionByTokenHash(_ context.Context, tokenHash []byte) (models.UserSession, error) {
	if !bytes.Equal(tokenHash, sessiontoken.Hash("current-session-token")) {
		return models.UserSession{}, repositories.ErrUserSessionNotFound
	}
	return s.session, s.err
}

type fakePasswordHasher struct{}

func (fakePasswordHasher) Hash(password string) (string, error) { return "hash:" + password, nil }

func (fakePasswordHasher) Compare(password, encodedHash string) (bool, error) {
	return encodedHash == "hash:"+password, nil
}

func newAccountService(repo *accountRepositoryStub, session models.UserSession) *services.AccountService {
	generateToken := func() (string, []byte, error) { return "new-session-token", []byte("new-session-hash"), nil }
	return services.NewAccountService(repo, sessionLookupStub{session: session}, fakePasswordHasher{}, generateToken, accountSessionTTL)
}

func currentSession(ttl time.Duration) models.UserSession {
	created := time.Now().UTC().Add(-time.Hour)
	return models.UserSession{UserID: accountUserID, CreatedAt: created, ExpiresAt: created.Add(ttl)}
}

func TestAccountServiceChangePassword(t *testing.T) {
	tests := []struct {
		name         string
		ttl          time.Duration
		wantRemember bool
	}{
		{name: "regular session", ttl: accountSessionTTL, wantRemember: false},
		{name: "remembered session", ttl: 30 * 24 * time.Hour, wantRemember: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := &accountRepositoryStub{passwordHash: "hash:senha-atual"}
			session := currentSession(tt.ttl)

			result, err := newAccountService(repo, session).ChangePassword(t.Context(), accountUserID, "current-session-token", "senha-atual", "senha-nova-123", "senha-nova-123")
			if err != nil {
				t.Fatalf("ChangePassword() error = %v", err)
			}

			if repo.replaced == nil || repo.replaced.hash != "hash:senha-nova-123" || string(repo.replaced.tokenHash) != "new-session-hash" {
				t.Fatalf("ReplacePasswordAndSessions() = %+v, want new hash and session", repo.replaced)
			}
			if !repo.replaced.expiresAt.Equal(session.ExpiresAt) || !result.Session.ExpiresAt.Equal(session.ExpiresAt) {
				t.Errorf("new session expires at %v, want the current session's %v", repo.replaced.expiresAt, session.ExpiresAt)
			}
			if result.Session.Token != "new-session-token" || result.RememberMe != tt.wantRemember {
				t.Errorf("result = %+v, want new token and remember %v", result, tt.wantRemember)
			}
		})
	}
}

func TestAccountServiceChangePasswordRejects(t *testing.T) {
	tests := []struct {
		name         string
		current      string
		newPassword  string
		confirmation string
		hashErr      error
		sessionUser  uuid.UUID
		token        string
		wantErr      error
	}{
		{name: "confirmation mismatch", current: "senha-atual", newPassword: "senha-nova-123", confirmation: "outra-senha-1", wantErr: services.ErrPasswordConfirmationMismatch},
		{name: "short new password", current: "senha-atual", newPassword: "curta", confirmation: "curta", wantErr: services.ErrPasswordTooShort},
		{name: "wrong current password", current: "errada-123", newPassword: "senha-nova-123", confirmation: "senha-nova-123", wantErr: services.ErrIncorrectCurrentPassword},
		{name: "empty current password", current: "", newPassword: "senha-nova-123", confirmation: "senha-nova-123", wantErr: services.ErrIncorrectCurrentPassword},
		{name: "same password", current: "senha-atual", newPassword: "senha-atual", confirmation: "senha-atual", wantErr: services.ErrSamePassword},
		{name: "Google-only account", current: "qualquer-1", newPassword: "senha-nova-123", confirmation: "senha-nova-123", hashErr: repositories.ErrPasswordCredentialNotFound, wantErr: services.ErrAccountHasNoPassword},
		{name: "unknown session", current: "senha-atual", newPassword: "senha-nova-123", confirmation: "senha-nova-123", token: "other-token", wantErr: services.ErrUnauthenticated},
		{name: "session of another user", current: "senha-atual", newPassword: "senha-nova-123", confirmation: "senha-nova-123", sessionUser: uuid.MustParse("01991f29-7c22-7ab3-a395-4d402f09c319"), wantErr: services.ErrUnauthenticated},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := &accountRepositoryStub{passwordHash: "hash:senha-atual", hashErr: tt.hashErr}
			session := currentSession(accountSessionTTL)
			if tt.sessionUser != (uuid.UUID{}) {
				session.UserID = tt.sessionUser
			}
			token := "current-session-token"
			if tt.token != "" {
				token = tt.token
			}

			_, err := newAccountService(repo, session).ChangePassword(t.Context(), accountUserID, token, tt.current, tt.newPassword, tt.confirmation)

			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("ChangePassword() error = %v, want %v", err, tt.wantErr)
			}
			if repo.replaced != nil {
				t.Error("password replaced despite the error")
			}
		})
	}
}

func TestAccountServiceUpdateFullName(t *testing.T) {
	repo := &accountRepositoryStub{account: models.Account{User: models.User{ID: accountUserID, Email: "ana@example.com"}}}

	account, err := newAccountService(repo, models.UserSession{}).UpdateFullName(t.Context(), accountUserID, "  Ana   Maria  Souza ")
	if err != nil {
		t.Fatalf("UpdateFullName() error = %v", err)
	}
	if repo.updatedName != "Ana Maria Souza" || account.FullName == nil || *account.FullName != "Ana Maria Souza" {
		t.Errorf("updated name = %q, account = %+v; want normalized name", repo.updatedName, account)
	}

	if _, err := newAccountService(repo, models.UserSession{}).UpdateFullName(t.Context(), accountUserID, "   "); !errors.Is(err, services.ErrFullNameRequired) {
		t.Errorf("UpdateFullName(blank) error = %v, want %v", err, services.ErrFullNameRequired)
	}
}
