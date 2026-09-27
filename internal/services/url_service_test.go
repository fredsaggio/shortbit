package services_test

import (
	"context"
	"errors"
	"strings"
	"testing"
	"uuid"

	"github.com/fredsaggio/url-shortener/internal/models"
	"github.com/fredsaggio/url-shortener/internal/services"
)

type urlRepositoryStub struct {
	reserveIDFunc func(context.Context) (int64, error)
	createFunc    func(context.Context, models.URL) (models.URL, error)
}

func (s urlRepositoryStub) ReserveID(ctx context.Context) (int64, error) {
	return s.reserveIDFunc(ctx)
}

func (s urlRepositoryStub) Create(ctx context.Context, url models.URL) (models.URL, error) {
	return s.createFunc(ctx, url)
}

type shortCodeGeneratorStub func(int64) (string, error)

func (s shortCodeGeneratorStub) Generate(id int64) (string, error) {
	return s(id)
}

func TestNewURLServiceValidatesConfig(t *testing.T) {
	tests := []struct {
		name   string
		config services.URLServiceConfig
		valid  bool
	}{
		{name: "valid base URL", config: services.URLServiceConfig{PublicBaseURL: "https://sho.rt/", MaxOriginalURLBytes: 2048}, valid: true},
		{name: "empty query marker", config: services.URLServiceConfig{PublicBaseURL: "https://sho.rt/?", MaxOriginalURLBytes: 2048}},
		{name: "empty fragment marker", config: services.URLServiceConfig{PublicBaseURL: "https://sho.rt/#", MaxOriginalURLBytes: 2048}},
		{name: "query", config: services.URLServiceConfig{PublicBaseURL: "https://sho.rt/?source=api", MaxOriginalURLBytes: 2048}},
		{name: "fragment", config: services.URLServiceConfig{PublicBaseURL: "https://sho.rt/#section", MaxOriginalURLBytes: 2048}},
		{name: "path", config: services.URLServiceConfig{PublicBaseURL: "https://sho.rt/links", MaxOriginalURLBytes: 2048}},
		{name: "non HTTP scheme", config: services.URLServiceConfig{PublicBaseURL: "ftp://sho.rt", MaxOriginalURLBytes: 2048}},
		{name: "missing host", config: services.URLServiceConfig{PublicBaseURL: "https:///short", MaxOriginalURLBytes: 2048}},
		{name: "missing maximum", config: services.URLServiceConfig{PublicBaseURL: "https://sho.rt"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			service, err := services.NewURLService(nil, nil, nil, tt.config)
			if tt.valid {
				if err != nil || service == nil {
					t.Fatalf("NewURLService() = (%v, %v), want non-nil service and no error", service, err)
				}
				return
			}
			if err == nil {
				t.Fatal("NewURLService() error = nil, want invalid config error")
			}
		})
	}
}

func TestURLServiceCreatePublic(t *testing.T) {
	userID := uuid.MustParse("01991f29-7c22-7ab3-a395-4d402f09c401")
	type contextKey struct{}
	ctx := context.WithValue(t.Context(), contextKey{}, "request-context")
	var calls []string

	repo := urlRepositoryStub{
		reserveIDFunc: func(gotCtx context.Context) (int64, error) {
			if gotCtx.Value(contextKey{}) != "request-context" {
				t.Error("ReserveID() did not receive request context")
			}
			calls = append(calls, "reserve")
			return 42, nil
		},
		createFunc: func(gotCtx context.Context, got models.URL) (models.URL, error) {
			if gotCtx.Value(contextKey{}) != "request-context" {
				t.Error("Create() did not receive request context")
			}
			calls = append(calls, "create")
			if got.ID != 42 || got.ShortCode != "Ab3dX9" || got.UserID != userID ||
				got.OriginalURL != "https://example.com/article" || got.Visibility != models.VisibilityPublic ||
				got.PasswordHash != nil {
				t.Errorf("Create() URL = %+v, want public URL without password hash", got)
			}
			return got, nil
		},
	}
	generator := shortCodeGeneratorStub(func(id int64) (string, error) {
		calls = append(calls, "generate")
		if id != 42 {
			t.Errorf("Generate() ID = %d, want 42", id)
		}
		return "Ab3dX9", nil
	})
	hasher := passwordHasherStub{hashFunc: func(string) (string, error) {
		t.Fatal("Hash() must not be called for a public URL")
		return "", nil
	}}
	service := newURLServiceForTest(t, repo, generator, hasher, "https://sho.rt/")

	result, err := service.Create(ctx, userID, "https://example.com/article", models.VisibilityPublic, "")
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if result.ShortCode != "Ab3dX9" || result.ShortURL != "https://sho.rt/Ab3dX9" {
		t.Errorf("Create() result = %+v, want expected code and short URL", result)
	}
	if strings.Join(calls, ",") != "reserve,generate,create" {
		t.Errorf("calls = %v, want reserve, generate, create", calls)
	}
}

func TestURLServiceCreatePrivate(t *testing.T) {
	userID := uuid.MustParse("01991f29-7c22-7ab3-a395-4d402f09c402")
	var calls []string
	hasher := passwordHasherStub{hashFunc: func(password string) (string, error) {
		calls = append(calls, "hash")
		if password != "senha-segura" {
			t.Errorf("Hash() password = %q, want senha-segura", password)
		}
		return "$argon2id$test-link-hash", nil
	}}
	repo := urlRepositoryStub{
		reserveIDFunc: func(context.Context) (int64, error) {
			calls = append(calls, "reserve")
			return 43, nil
		},
		createFunc: func(_ context.Context, got models.URL) (models.URL, error) {
			calls = append(calls, "create")
			if got.ID != 43 || got.ShortCode != "Ab3dYz" || got.UserID != userID ||
				got.Visibility != models.VisibilityPrivate || got.PasswordHash == nil ||
				*got.PasswordHash != "$argon2id$test-link-hash" {
				t.Errorf("Create() URL = %+v, want private URL with password hash", got)
			}
			return got, nil
		},
	}
	generator := shortCodeGeneratorStub(func(id int64) (string, error) {
		calls = append(calls, "generate")
		return "Ab3dYz", nil
	})
	service := newURLServiceForTest(t, repo, generator, hasher, "https://sho.rt")

	result, err := service.Create(t.Context(), userID, "https://example.com/private", models.VisibilityPrivate, "senha-segura")
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if result.ShortURL != "https://sho.rt/Ab3dYz" {
		t.Errorf("Create() short URL = %q, want https://sho.rt/Ab3dYz", result.ShortURL)
	}
	if strings.Join(calls, ",") != "hash,reserve,generate,create" {
		t.Errorf("calls = %v, want hash, reserve, generate, create", calls)
	}
}

func TestURLServiceCreateRejectsInvalidInputBeforeReservingID(t *testing.T) {
	userID := uuid.MustParse("01991f29-7c22-7ab3-a395-4d402f09c403")
	tests := []struct {
		name       string
		baseURL    string
		userID     uuid.UUID
		original   string
		visibility models.Visibility
		password   string
		wantErr    error
	}{
		{name: "missing user", userID: uuid.Nil(), original: "https://example.com", visibility: models.VisibilityPublic, wantErr: services.ErrUnauthenticated},
		{name: "empty URL", userID: userID, visibility: models.VisibilityPublic, wantErr: services.ErrInvalidOriginalURL},
		{name: "surrounding whitespace", userID: userID, original: " https://example.com ", visibility: models.VisibilityPublic, wantErr: services.ErrInvalidOriginalURL},
		{name: "URL too long", userID: userID, original: "https://example.com/" + strings.Repeat("x", 300), visibility: models.VisibilityPublic, wantErr: services.ErrURLTooLong},
		{name: "relative URL", userID: userID, original: "/article", visibility: models.VisibilityPublic, wantErr: services.ErrInvalidOriginalURL},
		{name: "non HTTP scheme", userID: userID, original: "javascript:alert(1)", visibility: models.VisibilityPublic, wantErr: services.ErrInvalidOriginalURL},
		{name: "missing host", userID: userID, original: "https:///article", visibility: models.VisibilityPublic, wantErr: services.ErrInvalidOriginalURL},
		{name: "URL credentials", userID: userID, original: "https://user:secret@example.com", visibility: models.VisibilityPublic, wantErr: services.ErrInvalidOriginalURL},
		{name: "own domain", userID: userID, original: "https://sho.rt/another", visibility: models.VisibilityPublic, wantErr: services.ErrURLPointsToShortDomain},
		{name: "own domain with trailing dot", userID: userID, original: "https://SHO.RT./another", visibility: models.VisibilityPublic, wantErr: services.ErrURLPointsToShortDomain},
		{name: "configured domain with trailing dot", baseURL: "https://sho.rt.", userID: userID, original: "https://sho.rt/another", visibility: models.VisibilityPublic, wantErr: services.ErrURLPointsToShortDomain},
		{name: "invalid visibility", userID: userID, original: "https://example.com", visibility: models.Visibility("hidden"), wantErr: services.ErrInvalidURLVisibility},
		{name: "public URL with password", userID: userID, original: "https://example.com", visibility: models.VisibilityPublic, password: "senha-segura", wantErr: services.ErrUnexpectedLinkPassword},
		{name: "private URL without password", userID: userID, original: "https://example.com", visibility: models.VisibilityPrivate, wantErr: services.ErrLinkPasswordTooShort},
		{name: "private password too long", userID: userID, original: "https://example.com", visibility: models.VisibilityPrivate, password: strings.Repeat("a", 1025), wantErr: services.ErrLinkPasswordTooLong},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := urlRepositoryStub{
				reserveIDFunc: func(context.Context) (int64, error) {
					t.Fatal("ReserveID() called for invalid input")
					return 0, nil
				},
				createFunc: func(context.Context, models.URL) (models.URL, error) {
					t.Fatal("Create() called for invalid input")
					return models.URL{}, nil
				},
			}
			generator := shortCodeGeneratorStub(func(int64) (string, error) {
				t.Fatal("Generate() called for invalid input")
				return "", nil
			})
			hasher := passwordHasherStub{hashFunc: func(string) (string, error) {
				t.Fatal("Hash() called for invalid input")
				return "", nil
			}}
			baseURL := tt.baseURL
			if baseURL == "" {
				baseURL = "https://sho.rt"
			}
			service := newURLServiceForTest(t, repo, generator, hasher, baseURL)

			_, err := service.Create(t.Context(), tt.userID, tt.original, tt.visibility, tt.password)
			if !errors.Is(err, tt.wantErr) {
				t.Errorf("Create() error = %v, want %v", err, tt.wantErr)
			}
		})
	}
}

func TestURLServiceCreatePropagatesDependencyErrors(t *testing.T) {
	userID := uuid.MustParse("01991f29-7c22-7ab3-a395-4d402f09c404")
	wantErr := errors.New("dependency failed")
	tests := []struct {
		name       string
		failAt     string
		visibility models.Visibility
		password   string
	}{
		{name: "hash", failAt: "hash", visibility: models.VisibilityPrivate, password: "senha-segura"},
		{name: "reserve", failAt: "reserve", visibility: models.VisibilityPublic},
		{name: "generate", failAt: "generate", visibility: models.VisibilityPublic},
		{name: "create", failAt: "create", visibility: models.VisibilityPublic},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := urlRepositoryStub{
				reserveIDFunc: func(context.Context) (int64, error) {
					if tt.failAt == "reserve" {
						return 0, wantErr
					}
					return 42, nil
				},
				createFunc: func(_ context.Context, got models.URL) (models.URL, error) {
					if tt.failAt == "create" {
						return models.URL{}, wantErr
					}
					return got, nil
				},
			}
			generator := shortCodeGeneratorStub(func(int64) (string, error) {
				if tt.failAt == "generate" {
					return "", wantErr
				}
				return "Ab3dX9", nil
			})
			hasher := passwordHasherStub{hashFunc: func(string) (string, error) {
				if tt.failAt == "hash" {
					return "", wantErr
				}
				return "$argon2id$test", nil
			}}
			service := newURLServiceForTest(t, repo, generator, hasher, "https://sho.rt")

			_, err := service.Create(t.Context(), userID, "https://example.com", tt.visibility, tt.password)
			if !errors.Is(err, wantErr) {
				t.Errorf("Create() error = %v, want wrapped %v", err, wantErr)
			}
		})
	}
}

func newURLServiceForTest(t *testing.T, repo services.URLRepository, generator services.ShortCodeGenerator, hasher services.PasswordHasher, baseURL string) *services.URLService {
	t.Helper()
	service, err := services.NewURLService(repo, generator, hasher, services.URLServiceConfig{
		PublicBaseURL:       baseURL,
		MaxOriginalURLBytes: 256,
	})
	if err != nil {
		t.Fatalf("NewURLService() error = %v", err)
	}
	return service
}
