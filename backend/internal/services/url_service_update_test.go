package services_test

import (
	"context"
	"errors"
	"slices"
	"testing"
	"uuid"

	"github.com/fredsaggio/url-shortener/internal/models"
	"github.com/fredsaggio/url-shortener/internal/repositories"
	"github.com/fredsaggio/url-shortener/internal/services"
)

var editOwner = uuid.MustParse("01991f29-7c22-7ab3-a395-4d402f09c311")

// recordingInvalidator logs every cache drop next to the repository writes,
// so tests can check the order.
type recordingInvalidator struct{ log *[]string }

func (r recordingInvalidator) Delete(_ context.Context, shortCodes ...string) error {
	for _, code := range shortCodes {
		*r.log = append(*r.log, "drop "+code)
	}
	return nil
}

func ptr[T any](value T) *T { return &value }

type editFixture struct {
	service *services.URLService
	edits   *[]repositories.URLEdit
	log     *[]string
}

func newEditFixture(t *testing.T, current models.URL) editFixture {
	t.Helper()
	edits := &[]repositories.URLEdit{}
	log := &[]string{}

	repo := urlRepositoryStub{
		getByShortcodeFunc: func(_ context.Context, userID uuid.UUID, shortCode string) (models.URL, error) {
			if userID != editOwner || shortCode != current.ShortCode {
				return models.URL{}, repositories.ErrURLNotFound
			}
			return current, nil
		},
		updateFunc: func(_ context.Context, _ uuid.UUID, shortCode string, edit repositories.URLEdit) (models.URL, error) {
			*edits = append(*edits, edit)
			*log = append(*log, "update "+shortCode)
			updated := current
			updated.OriginalURL = edit.OriginalURL
			updated.Visibility = edit.Visibility
			return updated, nil
		},
		deleteFunc: func(_ context.Context, userID uuid.UUID, shortCode string) error {
			if userID != editOwner || shortCode != current.ShortCode {
				return repositories.ErrURLNotFound
			}
			*log = append(*log, "delete "+shortCode)
			return nil
		},
	}
	hasher := passwordHasherStub{hashFunc: func(password string) (string, error) { return "hash:" + password, nil }}

	service, err := services.NewURLService(repo, nil, hasher, recordingInvalidator{log: log}, services.URLServiceConfig{
		PublicBaseURL:       "https://shortbit.com.br",
		MaxOriginalURLBytes: 256,
	})
	if err != nil {
		t.Fatalf("NewURLService() error = %v", err)
	}
	return editFixture{service: service, edits: edits, log: log}
}

var publicLink = models.URL{ID: 7, ShortCode: "Ab3dX9", UserID: editOwner, OriginalURL: "https://example.com/old", Visibility: models.VisibilityPublic}
var privateLink = models.URL{ID: 8, ShortCode: "Qz7Rt2", UserID: editOwner, OriginalURL: "https://example.com/secret", Visibility: models.VisibilityPrivate}

func TestURLServiceUpdateChangesDestinationAndDropsCacheAroundTheWrite(t *testing.T) {
	fixture := newEditFixture(t, publicLink)

	updated, err := fixture.service.Update(t.Context(), editOwner, "Ab3dX9", services.URLUpdate{OriginalURL: ptr("https://example.com/new")})
	if err != nil {
		t.Fatalf("Update() error = %v", err)
	}
	if updated.OriginalURL != "https://example.com/new" {
		t.Errorf("OriginalURL = %q", updated.OriginalURL)
	}

	edit := (*fixture.edits)[0]
	if edit.Visibility != models.VisibilityPublic || edit.KeepPasswordHash || edit.PasswordHash != nil || edit.RevokeAccessSessions {
		t.Errorf("edit = %+v, want public with no password and no revocation", edit)
	}
	if want := []string{"drop Ab3dX9", "update Ab3dX9", "drop Ab3dX9"}; !slices.Equal(*fixture.log, want) {
		t.Errorf("order = %v, want %v", *fixture.log, want)
	}
}

func TestURLServiceUpdateToPrivateNeedsAPassword(t *testing.T) {
	fixture := newEditFixture(t, publicLink)

	_, err := fixture.service.Update(t.Context(), editOwner, "Ab3dX9", services.URLUpdate{Visibility: ptr(models.VisibilityPrivate)})
	if !errors.Is(err, services.ErrLinkPasswordRequired) {
		t.Fatalf("Update() error = %v, want ErrLinkPasswordRequired", err)
	}

	_, err = fixture.service.Update(t.Context(), editOwner, "Ab3dX9", services.URLUpdate{Visibility: ptr(models.VisibilityPrivate), Password: ptr("senha-do-link")})
	if err != nil {
		t.Fatalf("Update() with password error = %v", err)
	}
	edit := (*fixture.edits)[0]
	if edit.Visibility != models.VisibilityPrivate || edit.KeepPasswordHash || edit.PasswordHash == nil || *edit.PasswordHash != "hash:senha-do-link" {
		t.Errorf("edit = %+v, want private with the new hash", edit)
	}
}

func TestURLServiceUpdatePrivateKeepsOrReplacesThePassword(t *testing.T) {
	fixture := newEditFixture(t, privateLink)

	if _, err := fixture.service.Update(t.Context(), editOwner, "Qz7Rt2", services.URLUpdate{OriginalURL: ptr("https://example.com/moved"), Password: ptr("")}); err != nil {
		t.Fatalf("Update() keeping password error = %v", err)
	}
	if edit := (*fixture.edits)[0]; !edit.KeepPasswordHash || edit.RevokeAccessSessions {
		t.Errorf("edit = %+v, want the password kept and sessions untouched", edit)
	}

	if _, err := fixture.service.Update(t.Context(), editOwner, "Qz7Rt2", services.URLUpdate{Password: ptr("senha-nova-do-link")}); err != nil {
		t.Fatalf("Update() new password error = %v", err)
	}
	if edit := (*fixture.edits)[1]; edit.KeepPasswordHash || *edit.PasswordHash != "hash:senha-nova-do-link" || !edit.RevokeAccessSessions {
		t.Errorf("edit = %+v, want the new hash and every unlock session revoked", edit)
	}
}

func TestURLServiceUpdateToPublicClearsThePassword(t *testing.T) {
	fixture := newEditFixture(t, privateLink)

	if _, err := fixture.service.Update(t.Context(), editOwner, "Qz7Rt2", services.URLUpdate{Visibility: ptr(models.VisibilityPublic)}); err != nil {
		t.Fatalf("Update() error = %v", err)
	}
	if edit := (*fixture.edits)[0]; edit.KeepPasswordHash || edit.PasswordHash != nil {
		t.Errorf("edit = %+v, want the password cleared", edit)
	}
}

func TestURLServiceUpdateRejectsInvalidEdits(t *testing.T) {
	tests := []struct {
		name    string
		current models.URL
		code    string
		update  services.URLUpdate
		want    error
	}{
		{name: "nothing to change", current: publicLink, code: "Ab3dX9", update: services.URLUpdate{}, want: services.ErrEmptyURLUpdate},
		{name: "invalid destination", current: publicLink, code: "Ab3dX9", update: services.URLUpdate{OriginalURL: ptr("javascript:alert(1)")}, want: services.ErrInvalidOriginalURL},
		{name: "destination on the short domain", current: publicLink, code: "Ab3dX9", update: services.URLUpdate{OriginalURL: ptr("https://shortbit.com.br/x")}, want: services.ErrURLPointsToShortDomain},
		{name: "invalid visibility", current: publicLink, code: "Ab3dX9", update: services.URLUpdate{Visibility: ptr(models.Visibility("secret"))}, want: services.ErrInvalidURLVisibility},
		{name: "password on a public link", current: publicLink, code: "Ab3dX9", update: services.URLUpdate{Password: ptr("senha-do-link")}, want: services.ErrUnexpectedLinkPassword},
		{name: "short new password", current: privateLink, code: "Qz7Rt2", update: services.URLUpdate{Password: ptr("curta")}, want: services.ErrLinkPasswordTooShort},
		{name: "someone else's link", current: publicLink, code: "Zz9Zz9", update: services.URLUpdate{OriginalURL: ptr("https://example.com/x")}, want: services.ErrURLNotFound},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fixture := newEditFixture(t, tt.current)

			_, err := fixture.service.Update(t.Context(), editOwner, tt.code, tt.update)
			if !errors.Is(err, tt.want) {
				t.Fatalf("Update() error = %v, want %v", err, tt.want)
			}
			if len(*fixture.edits) != 0 {
				t.Errorf("repository updated %d times, want none", len(*fixture.edits))
			}
		})
	}
}

func TestURLServiceDelete(t *testing.T) {
	fixture := newEditFixture(t, publicLink)

	if err := fixture.service.Delete(t.Context(), editOwner, "Ab3dX9"); err != nil {
		t.Fatalf("Delete() error = %v", err)
	}
	if want := []string{"drop Ab3dX9", "delete Ab3dX9", "drop Ab3dX9"}; !slices.Equal(*fixture.log, want) {
		t.Errorf("order = %v, want %v", *fixture.log, want)
	}

	if err := fixture.service.Delete(t.Context(), editOwner, "Zz9Zz9"); !errors.Is(err, services.ErrURLNotFound) {
		t.Errorf("Delete() of a missing link error = %v, want ErrURLNotFound", err)
	}
	if err := fixture.service.Delete(t.Context(), uuid.Nil(), "Ab3dX9"); !errors.Is(err, services.ErrUnauthenticated) {
		t.Errorf("Delete() without user error = %v, want ErrUnauthenticated", err)
	}
}
