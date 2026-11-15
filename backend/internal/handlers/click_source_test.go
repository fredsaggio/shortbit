package handlers_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/fredsaggio/url-shortener/internal/handlers"
	"github.com/fredsaggio/url-shortener/internal/models"
	"github.com/fredsaggio/url-shortener/internal/services"
)

func TestRedirectReadsTheQRMarker(t *testing.T) {
	tests := []struct {
		path string
		want models.ClickSource
	}{
		{path: "/Ab3dX9", want: models.ClickSourceLink},
		{path: "/Ab3dX9?qr=1", want: models.ClickSourceQR},
		{path: "/Ab3dX9?qr=0", want: models.ClickSourceLink},
		{path: "/Ab3dX9?qr=sim", want: models.ClickSourceLink},
	}
	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			var got models.ClickSource
			handler := newURLHandler(urlServiceStub{resolveFunc: func(_ context.Context, _, _ string, source models.ClickSource) (services.RedirectResult, error) {
				got = source
				return services.RedirectResult{OriginalURL: "https://example.com/article"}, nil
			}})
			request := httptest.NewRequest(http.MethodGet, tt.path, nil)
			request.SetPathValue("code", "Ab3dX9")
			response := httptest.NewRecorder()

			handler.Redirect(response, request)

			if got != tt.want {
				t.Errorf("source = %q, want %q", got, tt.want)
			}
			// The marker labels the click only: the destination is untouched.
			if location := response.Header().Get("Location"); location != "https://example.com/article" {
				t.Errorf("Location = %q, want the destination unchanged", location)
			}
		})
	}
}

func TestPrivateLinkKeepsTheQRMarkerThroughThePasswordForm(t *testing.T) {
	handler := newURLHandler(urlServiceStub{resolveFunc: func(context.Context, string, string, models.ClickSource) (services.RedirectResult, error) {
		return services.RedirectResult{PasswordRequired: true}, nil
	}})
	for path, wantAction := range map[string]string{
		"/Qz7Rt2?qr=1": `action="/Qz7Rt2/access?qr=1"`,
		"/Qz7Rt2":      `action="/Qz7Rt2/access"`,
	} {
		request := httptest.NewRequest(http.MethodGet, path, nil)
		request.SetPathValue("code", "Qz7Rt2")
		response := httptest.NewRecorder()

		handler.Redirect(response, request)

		if !strings.Contains(response.Body.String(), wantAction) {
			t.Errorf("GET %s: form without %s", path, wantAction)
		}
	}

	service := linkAccessSessionServiceStub{createFunc: func(context.Context, string, string) (services.LinkAccessSessionResult, error) {
		return services.LinkAccessSessionResult{Token: "raw-link-token", ExpiresAt: time.Now().Add(time.Hour)}, nil
	}}
	for target, wantLocation := range map[string]string{
		"/Qz7Rt2/access?qr=1": "/Qz7Rt2?qr=1",
		"/Qz7Rt2/access":      "/Qz7Rt2",
	} {
		request := httptest.NewRequest(http.MethodPost, target, strings.NewReader(url.Values{"password": {"senha do link"}}.Encode()))
		request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		request.SetPathValue("code", "Qz7Rt2")
		response := httptest.NewRecorder()

		handlers.NewLinkAccessSessionHandler(service, false).RedirectPrivateLink(response, request)

		if got := response.Header().Get("Location"); got != wantLocation {
			t.Errorf("POST %s: Location = %q, want %q", target, got, wantLocation)
		}
	}

	wrong := linkAccessSessionServiceStub{createFunc: func(context.Context, string, string) (services.LinkAccessSessionResult, error) {
		return services.LinkAccessSessionResult{}, services.ErrIncorrectLinkPassword
	}}
	request := httptest.NewRequest(http.MethodPost, "/Qz7Rt2/access?qr=1", strings.NewReader("password=errada"))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	request.SetPathValue("code", "Qz7Rt2")
	response := httptest.NewRecorder()
	handlers.NewLinkAccessSessionHandler(wrong, false).RedirectPrivateLink(response, request)
	if !strings.Contains(response.Body.String(), `action="/Qz7Rt2/access?qr=1"`) {
		t.Error("wrong password page lost the QR marker")
	}
}
