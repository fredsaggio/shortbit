// Package problem writes API errors. Programs get RFC 9457 Problem Details:
// a JSON body served as application/problem+json, where detail is the
// Portuguese message and code a stable, machine-readable name clients can
// branch on without parsing text. type stays "about:blank", so title is the
// HTTP status phrase, as the RFC prescribes for it.
//
// People never see that JSON: a request from a browser navigation (one that
// accepts text/html, such as opening a deleted short link) gets an HTML page
// with the message written for people instead.
package problem

import (
	"bytes"
	_ "embed"
	"encoding/json"
	"html/template"
	"net/http"
	"strings"
	"unicode"
	"unicode/utf8"
)

const ContentType = "application/problem+json"

type Details struct {
	Type   string `json:"type"`
	Title  string `json:"title"`
	Status int    `json:"status"`
	Detail string `json:"detail"`
	Code   string `json:"code"`
}

//go:embed page.html
var pageHTML string

var pageTemplate = template.Must(template.New("problem").Parse(pageHTML))

type page struct {
	Title       string
	Text        string
	ActionHref  string
	ActionLabel string
}

const (
	genericServerText = "Algo deu errado do nosso lado. Tente de novo em instantes."
	homeHref          = "/"
	homeLabel         = "Ir para a página inicial"
)

// pages holds the wording people see for errors a browser can reach by
// opening an address. Other codes use their detail.
var pages = map[string]page{
	"link_not_found": {
		Title: "Link não encontrado",
		Text:  "Este link não existe ou foi apagado por quem o criou. Confira se ele foi copiado inteiro.",
	},
	"rate_limited": {
		Title: "Muitas tentativas seguidas",
		Text:  "Aguarde um pouco e tente de novo.",
	},
	"authentication_required": {
		Title:       "Entre na sua conta",
		Text:        "Você precisa estar conectado para ver esta página.",
		ActionHref:  "/entrar",
		ActionLabel: "Entrar",
	},
	"service_unavailable": {
		Title: "Não foi possível abrir agora",
		Text:  genericServerText,
	},
	"internal_error": {
		Title: "Algo deu errado",
		Text:  genericServerText,
	},
}

// Write sends the error with the given status, as a page for a browser and as
// a problem document for anything else. Like http.Error, it removes headers
// meant for a successful body and stops caching.
func Write(w http.ResponseWriter, r *http.Request, status int, code, detail string) {
	h := w.Header()
	h.Del("Content-Length")
	h.Del("Content-Encoding")
	h.Set("Cache-Control", "no-store")
	h.Set("X-Content-Type-Options", "nosniff")
	h.Add("Vary", "Accept")

	if wantsHTML(r) {
		if body, ok := renderPage(status, code, detail); ok {
			h.Set("Content-Type", "text/html; charset=utf-8")
			// The page has no scripts, images or forms of its own.
			h.Set("Content-Security-Policy", "default-src 'none'; style-src 'unsafe-inline'; base-uri 'none'; form-action 'none'; frame-ancestors 'none'")
			w.WriteHeader(status)
			_, _ = w.Write(body)
			return
		}
	}

	h.Set("Content-Type", ContentType)
	w.WriteHeader(status)
	// Encoding a struct of strings and an int cannot fail; a broken
	// connection has nowhere to be reported.
	_ = json.NewEncoder(w).Encode(Details{
		Type:   "about:blank",
		Title:  http.StatusText(status),
		Status: status,
		Detail: detail,
		Code:   code,
	})
}

// wantsHTML reports a browser navigation. The web app's own requests ask
// for application/json, and API clients rarely ask for text/html.
func wantsHTML(r *http.Request) bool {
	return r != nil && strings.Contains(r.Header.Get("Accept"), "text/html")
}

func renderPage(status int, code, detail string) ([]byte, bool) {
	content, known := pages[code]
	switch {
	case known:
	case status >= 500:
		content = pages["internal_error"]
	default:
		content = page{Title: "Não foi possível continuar", Text: sentence(detail)}
	}
	if content.ActionHref == "" {
		content.ActionHref, content.ActionLabel = homeHref, homeLabel
	}

	var body bytes.Buffer
	if err := pageTemplate.Execute(&body, content); err != nil {
		return nil, false
	}
	return body.Bytes(), true
}

// sentence capitalizes the message and ends it with a period.
func sentence(text string) string {
	text = strings.TrimSpace(text)
	if text == "" {
		return "Tente de novo."
	}
	first, size := utf8.DecodeRuneInString(text)
	text = string(unicode.ToUpper(first)) + text[size:]
	if !strings.HasSuffix(text, ".") {
		text += "."
	}
	return text
}
