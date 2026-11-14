package frontend_test

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/fredsaggio/url-shortener/internal/frontend"
)

const indexHTML = `<!doctype html><html><body><div id="root"></div></body></html>`

func newBuild(t *testing.T) string {
	t.Helper()

	dir := t.TempDir()
	write := func(name, content string) {
		path := filepath.Join(dir, name)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	write("index.html", indexHTML)
	write("favicon.svg", "<svg></svg>")
	write("assets/index-abc123.js", "console.log('app')")

	return dir
}

func newMux(t *testing.T, dir string) *http.ServeMux {
	t.Helper()

	handler, err := frontend.NewHandler(dir)
	if err != nil {
		t.Fatalf("NewHandler() error = %v", err)
	}

	mux := http.NewServeMux()
	handler.Register(mux)
	mux.HandleFunc("GET /{code}", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Short-Code", r.PathValue("code"))
		w.WriteHeader(http.StatusFound)
	})

	return mux
}

func get(mux *http.ServeMux, path string) *httptest.ResponseRecorder {
	recorder := httptest.NewRecorder()
	mux.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, path, nil))
	return recorder
}

func TestServesTheAppOnItsRoutes(t *testing.T) {
	mux := newMux(t, newBuild(t))

	for _, path := range []string{"/", "/entrar", "/cadastro", "/recuperar-senha", "/painel", "/painel/links/Ab3dX9", "/conta"} {
		t.Run(path, func(t *testing.T) {
			response := get(mux, path)

			if response.Code != http.StatusOK {
				t.Fatalf("status = %d, want %d", response.Code, http.StatusOK)
			}
			if response.Body.String() != indexHTML {
				t.Errorf("body = %q, want index.html", response.Body.String())
			}
			if got := response.Header().Get("Cache-Control"); got != "no-cache" {
				t.Errorf("Cache-Control = %q, want no-cache", got)
			}
			csp := response.Header().Get("Content-Security-Policy")
			for _, directive := range []string{"default-src 'self'", "script-src 'self'", "frame-ancestors 'none'", "object-src 'none'"} {
				if !strings.Contains(csp, directive) {
					t.Errorf("Content-Security-Policy = %q, missing %q", csp, directive)
				}
			}
			if got := response.Header().Get("X-Content-Type-Options"); got != "nosniff" {
				t.Errorf("X-Content-Type-Options = %q, want nosniff", got)
			}
		})
	}
}

func TestLeavesShortLinksToTheAPI(t *testing.T) {
	mux := newMux(t, newBuild(t))

	response := get(mux, "/Ab3dX9")

	if response.Code != http.StatusFound || response.Header().Get("X-Short-Code") != "Ab3dX9" {
		t.Fatalf("GET /Ab3dX9 status = %d, code = %q; want the short link handler", response.Code, response.Header().Get("X-Short-Code"))
	}
}

func TestServesHashedAssetsWithLongCache(t *testing.T) {
	mux := newMux(t, newBuild(t))

	response := get(mux, "/assets/index-abc123.js")

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusOK)
	}
	if got := response.Header().Get("Cache-Control"); got != "public, max-age=31536000, immutable" {
		t.Errorf("Cache-Control = %q", got)
	}
	if got := response.Header().Get("Content-Type"); !strings.HasPrefix(got, "text/javascript") {
		t.Errorf("Content-Type = %q, want text/javascript", got)
	}
}

func TestDoesNotListOrEscapeTheBuild(t *testing.T) {
	mux := newMux(t, newBuild(t))

	for _, path := range []string{"/assets/", "/assets/missing.js", "/assets/../../etc/passwd"} {
		t.Run(path, func(t *testing.T) {
			response := get(mux, path)
			if response.Code == http.StatusOK {
				t.Errorf("GET %s status = %d, want an error", path, response.Code)
			}
			if strings.Contains(response.Body.String(), "index-abc123.js") {
				t.Errorf("GET %s listed the directory", path)
			}
		})
	}
}

func TestServesRootFiles(t *testing.T) {
	mux := newMux(t, newBuild(t))

	response := get(mux, "/favicon.svg")

	if response.Code != http.StatusOK || response.Body.String() != "<svg></svg>" {
		t.Fatalf("GET /favicon.svg status = %d body = %q", response.Code, response.Body.String())
	}
}

func TestRejectsADirectoryWithoutABuild(t *testing.T) {
	if _, err := frontend.NewHandler(t.TempDir()); err == nil {
		t.Fatal("NewHandler() error = nil, want an error for a directory without index.html")
	}
}
