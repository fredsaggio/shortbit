package services_test

import (
	"context"
	"errors"
	"slices"
	"testing"
	"time"

	"github.com/fredsaggio/url-shortener/internal/models"
	"github.com/fredsaggio/url-shortener/internal/services"
	"github.com/fredsaggio/url-shortener/internal/sessiontoken"
)

type codeRecorder struct{ dropped *[]string }

func (r codeRecorder) Delete(_ context.Context, shortCodes ...string) error {
	*r.dropped = append(*r.dropped, shortCodes...)
	return nil
}

func newDeletionService(repo *accountRepositoryStub, dropped *[]string) *services.AccountService {
	return services.NewAccountService(repo, sessionLookupStub{}, fakePasswordHasher{}, sessiontoken.Generate, time.Hour, codeRecorder{dropped: dropped})
}

func TestAccountServiceDeleteAccountWithPassword(t *testing.T) {
	repo := &accountRepositoryStub{
		account:      models.Account{User: models.User{ID: accountUserID, Email: "ana@example.com"}, HasPassword: true},
		passwordHash: "hash:Senha-atual1",
		deleteCodes:  []string{"Ab3dX9", "Qz7Rt2"},
	}
	dropped := []string{}
	service := newDeletionService(repo, &dropped)

	for _, wrong := range []string{"", "Senha-errada1"} {
		if err := service.DeleteAccount(t.Context(), accountUserID, wrong, "ana@example.com"); !errors.Is(err, services.ErrIncorrectCurrentPassword) {
			t.Fatalf("DeleteAccount(%q) error = %v, want ErrIncorrectCurrentPassword", wrong, err)
		}
	}
	if repo.deleted {
		t.Fatal("account deleted with a wrong password")
	}

	if err := service.DeleteAccount(t.Context(), accountUserID, "Senha-atual1", ""); err != nil {
		t.Fatalf("DeleteAccount() error = %v", err)
	}
	if !repo.deleted {
		t.Fatal("account not deleted")
	}
	if !slices.Equal(dropped, []string{"Ab3dX9", "Qz7Rt2"}) {
		t.Errorf("dropped cache entries = %v, want both links", dropped)
	}
}

func TestAccountServiceDeleteGoogleOnlyAccountConfirmsTheEmail(t *testing.T) {
	repo := &accountRepositoryStub{account: models.Account{User: models.User{ID: accountUserID, Email: "ana@example.com"}, HasGoogle: true}}
	dropped := []string{}
	service := newDeletionService(repo, &dropped)

	if err := service.DeleteAccount(t.Context(), accountUserID, "", "outra@example.com"); !errors.Is(err, services.ErrAccountEmailMismatch) {
		t.Fatalf("DeleteAccount() with another email error = %v, want ErrAccountEmailMismatch", err)
	}
	if repo.deleted {
		t.Fatal("account deleted with the wrong email")
	}

	if err := service.DeleteAccount(t.Context(), accountUserID, "", "  Ana@Example.com "); err != nil {
		t.Fatalf("DeleteAccount() error = %v", err)
	}
	if !repo.deleted {
		t.Fatal("account not deleted")
	}
}

func TestAccountServiceDeleteAccountReportsFailures(t *testing.T) {
	repo := &accountRepositoryStub{
		account:      models.Account{User: models.User{ID: accountUserID, Email: "ana@example.com"}, HasPassword: true},
		passwordHash: "hash:Senha-atual1",
		deleteErr:    errors.New("database unavailable"),
	}
	dropped := []string{}

	if err := newDeletionService(repo, &dropped).DeleteAccount(t.Context(), accountUserID, "Senha-atual1", ""); err == nil {
		t.Fatal("DeleteAccount() error = nil, want the repository error")
	}
	if len(dropped) != 0 {
		t.Errorf("dropped cache entries = %v, want none when nothing was deleted", dropped)
	}
}
