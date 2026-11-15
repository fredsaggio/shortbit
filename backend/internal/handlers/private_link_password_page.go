package handlers

import (
	"bytes"
	_ "embed"
	"github.com/fredsaggio/url-shortener/internal/problem"
	"html/template"
	"log/slog"
	"net/http"
)

//go:embed templates/private_link_password.html
var privateLinkPasswordHTML string

var privateLinkPasswordTemplate = template.Must(template.New("private_link_password").Parse(privateLinkPasswordHTML))

type privateLinkPasswordPageData struct {
	ShortCode string
	// FormAction keeps the QR marker, so a scan unlocked by password still
	// counts as a QR code click.
	FormAction string
	Error      string
}

func renderPrivateLinkPasswordPage(w http.ResponseWriter, r *http.Request, shortCode, message string, status int) {
	var body bytes.Buffer
	if err := privateLinkPasswordTemplate.Execute(&body, privateLinkPasswordPageData{ShortCode: shortCode, FormAction: keepClickSource(r, "/"+shortCode+"/access"), Error: message}); err != nil {
		slog.ErrorContext(r.Context(), "render private link password page failed", "error", err)
		problem.Write(w, r, http.StatusInternalServerError, "internal_error", "erro interno do servidor")
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.WriteHeader(status)
	if _, err := w.Write(body.Bytes()); err != nil {
		slog.ErrorContext(r.Context(), "write private link password page failed", "error", err)
	}
}
