package handlers

import (
	"fmt"
	"net/http/httptest"
	"testing"

	"github.com/fredsaggio/url-shortener/internal/problem/problemtest"
	"github.com/fredsaggio/url-shortener/internal/services"
)

func TestValidationErrorsCarryStableCodes(t *testing.T) {
	links := map[error]string{
		services.ErrInvalidOriginalURL:     "invalid_url",
		services.ErrURLTooLong:             "url_too_long",
		services.ErrURLPointsToShortDomain: "url_points_to_short_domain",
		services.ErrInvalidURLVisibility:   "invalid_visibility",
		services.ErrUnexpectedLinkPassword: "unexpected_link_password",
		services.ErrLinkPasswordTooShort:   "link_password_too_short",
		services.ErrLinkPasswordTooLong:    "link_password_too_long",
		services.ErrLinkPasswordRequired:   "link_password_required",
		services.ErrEmptyURLUpdate:         "empty_update",
	}
	for err, want := range links {
		if code, _, ok := linkInputError(fmt.Errorf("wrapped: %w", err)); !ok || code != want {
			t.Errorf("linkInputError(%v) = %q, %v; want %q", err, code, ok, want)
		}
	}
	if _, _, ok := linkInputError(fmt.Errorf("database down")); ok {
		t.Error("linkInputError accepted an unrelated error")
	}

	passwords := map[error]string{
		services.ErrPasswordTooShort:         "password_too_short",
		services.ErrPasswordTooLong:          "password_too_long",
		services.ErrPasswordMissingUppercase: "password_missing_uppercase",
		services.ErrPasswordMissingDigit:     "password_missing_digit",
		services.ErrPasswordMissingSpecial:   "password_missing_special",
	}
	for err, want := range passwords {
		if code, _, ok := passwordPolicyError(err); !ok || code != want {
			t.Errorf("passwordPolicyError(%v) = %q, %v; want %q", err, code, ok, want)
		}
	}

	names := map[error]string{
		services.ErrFullNameTooLong:  "full_name_too_long",
		services.ErrInvalidFullName:  "invalid_full_name",
		services.ErrFullNameRequired: "full_name_required",
	}
	for err, want := range names {
		recorder := httptest.NewRecorder()
		writeFullNameError(recorder, httptest.NewRequest("PATCH", "/me", nil), err)
		details, ok := problemtest.Decode(recorder.Body.String())
		if !ok || details.Code != want || details.Status != 400 {
			t.Errorf("writeFullNameError(%v) = %+v, want code %q", err, details, want)
		}
	}
}
