// Package frontend serves the built React app (frontend/dist) from the API.
// In production the app, the API and the short links share one origin, so the
// session cookies work without CORS and short links stay at /{code}.
package frontend

import (
	"bytes"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

// AppRoutes are the React routes. They are more specific than the short link
// route GET /{code}, so the mux sends them to the app; shortcodes never
// generates a code equal to one of them.
var AppRoutes = []string{
	"GET /{$}",
	"GET /entrar",
	"GET /cadastro",
	"GET /recuperar-senha",
	"GET /painel",
	"GET /painel/",
	"GET /conta",
}

// The app loads only its own bundle, styles and fonts and calls only its own
// origin. Radix and sonner set inline styles, hence 'unsafe-inline' for styles
// only. See the OWASP Content Security Policy and HTTP Headers cheat sheets.
const contentSecurityPolicy = "default-src 'self'; " +
	"script-src 'self'; " +
	"style-src 'self' 'unsafe-inline'; " +
	"img-src 'self' data:; " +
	"font-src 'self' data:; " +
	"connect-src 'self'; " +
	"object-src 'none'; " +
	"base-uri 'self'; " +
	"form-action 'self'; " +
	"frame-ancestors 'none'"

var rootFileName = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]*$`)

type Handler struct {
	dir       string
	index     []byte
	indexTime time.Time
	rootFiles []string
}

// NewHandler checks that dir holds a build (index.html) and lists the files
// at its root, such as favicon.svg, to serve them by name.
func NewHandler(dir string) (*Handler, error) {
	indexPath := filepath.Join(dir, "index.html")
	index, err := os.ReadFile(indexPath)
	if err != nil {
		return nil, fmt.Errorf("read frontend index %s: %w", indexPath, err)
	}

	info, err := os.Stat(indexPath)
	if err != nil {
		return nil, fmt.Errorf("stat frontend index: %w", err)
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("list frontend directory: %w", err)
	}

	var rootFiles []string
	for _, entry := range entries {
		name := entry.Name()
		if !entry.Type().IsRegular() || name == "index.html" || !rootFileName.MatchString(name) {
			continue
		}
		rootFiles = append(rootFiles, name)
	}

	if len(index) == 0 {
		return nil, errors.New("frontend index.html is empty")
	}

	return &Handler{dir: dir, index: index, indexTime: info.ModTime(), rootFiles: rootFiles}, nil
}

func (h *Handler) Register(mux *http.ServeMux) {
	for _, route := range AppRoutes {
		mux.HandleFunc(route, h.serveIndex)
	}

	mux.Handle("GET /assets/", h.serveAssets())

	for _, name := range h.rootFiles {
		mux.HandleFunc("GET /"+name, h.serveRootFile(name))
	}
}

func (h *Handler) serveIndex(w http.ResponseWriter, r *http.Request) {
	header := w.Header()
	// Always revalidate: a new deploy changes the asset names in index.html.
	header.Set("Cache-Control", "no-cache")
	header.Set("Content-Type", "text/html; charset=utf-8")
	header.Set("Content-Security-Policy", contentSecurityPolicy)
	header.Set("X-Content-Type-Options", "nosniff")
	header.Set("X-Frame-Options", "DENY")
	header.Set("Referrer-Policy", "strict-origin-when-cross-origin")

	http.ServeContent(w, r, "index.html", h.indexTime, bytes.NewReader(h.index))
}

func (h *Handler) serveAssets() http.Handler {
	files := http.FileServerFS(os.DirFS(h.dir))

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// No directory listings.
		if strings.HasSuffix(r.URL.Path, "/") {
			http.NotFound(w, r)
			return
		}

		// Vite puts a content hash in every asset name, so a name never
		// changes content and can be cached for good.
		w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		files.ServeHTTP(w, r)
	})
}

func (h *Handler) serveRootFile(name string) http.HandlerFunc {
	path := filepath.Join(h.dir, name)

	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "public, max-age=3600")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		http.ServeFile(w, r, path)
	}
}
