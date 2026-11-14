package handlers

import (
	"bytes"
	_ "embed"
	"html/template"
	"log/slog"
	"net/http"
)

//go:embed templates/private_link_password.html
var privateLinkPasswordHTML string

var privateLinkPasswordTemplate = template.Must(template.New("private_link_password").Parse(privateLinkPasswordHTML))

type privateLinkPasswordPageData struct {
	ShortCode string
	Error     string
}

func renderPrivateLinkPasswordPage(w http.ResponseWriter, r *http.Request, shortCode, message string, status int) {
	var body bytes.Buffer
	if err := privateLinkPasswordTemplate.Execute(&body, privateLinkPasswordPageData{ShortCode: shortCode, Error: message}); err != nil {
		slog.ErrorContext(r.Context(), "render private link password page failed", "error", err)
		http.Error(w, "erro interno do servidor", http.StatusInternalServerError)
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
