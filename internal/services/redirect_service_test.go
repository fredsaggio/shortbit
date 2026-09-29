package services_test

import (
	"bytes"
	"context"
	"errors"
	"testing"

	"github.com/fredsaggio/url-shortener/internal/repositories"
	"github.com/fredsaggio/url-shortener/internal/services"
	"github.com/fredsaggio/url-shortener/internal/sessiontoken"
)

type publicLinkResolverStub func(context.Context, string) (string, error)

func (resolve publicLinkResolverStub) ResolvePublicAndCountClick(ctx context.Context, code string) (string, error) {
	return resolve(ctx, code)
}

func TestRedirectServiceResolvePublicReturnsOriginalURL(t *testing.T) {
	type contextKey struct{}
	ctx := context.WithValue(t.Context(), contextKey{}, "request-context")
	called := false
	service := services.NewRedirectService(publicLinkResolverStub(func(gotCtx context.Context, gotCode string) (string, error) {
		called = true
		if gotCtx.Value(contextKey{}) != "request-context" || gotCode != "Ab3dX9" {
			t.Errorf("ResolvePublicAndCountClick() arguments = (%v, %q), want request context and Ab3dX9", gotCtx, gotCode)
		}
		return "https://example.com/article", nil
	}), nil)

	got, err := service.ResolvePublic(ctx, "Ab3dX9")
	if err != nil || got != "https://example.com/article" || !called {
		t.Errorf("ResolvePublic() = (%q, %v), repository called = %t; want original URL, nil, true", got, err, called)
	}
}

func TestRedirectServiceResolvePublicMapsNotFound(t *testing.T) {
	service := services.NewRedirectService(publicLinkResolverStub(func(context.Context, string) (string, error) {
		return "", repositories.ErrURLNotFound
	}), nil)

	got, err := service.ResolvePublic(t.Context(), "missing")
	if got != "" || !errors.Is(err, services.ErrURLNotFound) {
		t.Errorf("ResolvePublic() = (%q, %v), want empty URL and ErrURLNotFound", got, err)
	}
}

func TestRedirectServiceResolvePublicWrapsRepositoryError(t *testing.T) {
	wantErr := errors.New("database unavailable")
	service := services.NewRedirectService(publicLinkResolverStub(func(context.Context, string) (string, error) {
		return "", wantErr
	}), nil)

	got, err := service.ResolvePublic(t.Context(), "Ab3dX9")
	if got != "" || !errors.Is(err, wantErr) {
		t.Errorf("ResolvePublic() = (%q, %v), want empty URL and wrapped repository error", got, err)
	}
}

type privateLinkResolverStub struct {
	findFunc    func(context.Context, string) (int64, string, error)
	resolveFunc func(context.Context, int64, []byte) (string, error)
}

func (s privateLinkResolverStub) FindPrivateByShortCode(ctx context.Context, code string) (int64, string, error) {
	return s.findFunc(ctx, code)
}

func (s privateLinkResolverStub) ResolvePrivateAndCountClick(ctx context.Context, linkID int64, tokenHash []byte) (string, error) {
	return s.resolveFunc(ctx, linkID, tokenHash)
}

func TestRedirectServiceResolvePublic(t *testing.T) {
	private := privateLinkResolverStub{findFunc: func(context.Context, string) (int64, string, error) {
		t.Fatal("private lookup called for public link")
		return 0, "", nil
	}}
	service := services.NewRedirectService(publicLinkResolverStub(func(_ context.Context, code string) (string, error) {
		if code != "Ab3dX9" {
			t.Errorf("public shortcode = %q, want Ab3dX9", code)
		}
		return "https://example.com/public", nil
	}), private)

	result, err := service.Resolve(t.Context(), "Ab3dX9", "ignored-cookie-token")
	if err != nil || result.OriginalURL != "https://example.com/public" || result.PasswordRequired {
		t.Errorf("Resolve() = (%+v, %v), want public destination without password", result, err)
	}
}

func TestRedirectServiceResolvePrivateWithoutCookie(t *testing.T) {
	private := privateLinkResolverStub{
		findFunc: func(_ context.Context, code string) (int64, string, error) {
			if code != "Qz7Rt2" {
				t.Errorf("private shortcode = %q, want Qz7Rt2", code)
			}
			return 42, "stored-password-hash", nil
		},
		resolveFunc: func(context.Context, int64, []byte) (string, error) {
			t.Fatal("private redirect attempted without a cookie")
			return "", nil
		},
	}
	service := services.NewRedirectService(publicLinkResolverStub(func(context.Context, string) (string, error) {
		return "", repositories.ErrURLNotFound
	}), private)

	result, err := service.Resolve(t.Context(), "Qz7Rt2", "")
	if err != nil || !result.PasswordRequired || result.OriginalURL != "" {
		t.Errorf("Resolve() = (%+v, %v), want password page without destination", result, err)
	}
}

func TestRedirectServiceResolvePrivateWithValidCookie(t *testing.T) {
	type contextKey struct{}
	ctx := context.WithValue(t.Context(), contextKey{}, "request-context")
	private := privateLinkResolverStub{
		findFunc: func(gotCtx context.Context, code string) (int64, string, error) {
			if gotCtx.Value(contextKey{}) != "request-context" || code != "Qz7Rt2" {
				t.Errorf("private lookup arguments = (%v, %q), want request context and Qz7Rt2", gotCtx, code)
			}
			return 42, "stored-password-hash", nil
		},
		resolveFunc: func(gotCtx context.Context, linkID int64, tokenHash []byte) (string, error) {
			if gotCtx.Value(contextKey{}) != "request-context" || linkID != 42 || !bytes.Equal(tokenHash, sessiontoken.Hash("raw-link-token")) {
				t.Errorf("private resolution arguments = (%v, %d, %x), want request context, link 42 and hashed token", gotCtx, linkID, tokenHash)
			}
			return "https://example.com/private", nil
		},
	}
	service := services.NewRedirectService(publicLinkResolverStub(func(gotCtx context.Context, code string) (string, error) {
		if gotCtx.Value(contextKey{}) != "request-context" || code != "Qz7Rt2" {
			t.Errorf("public resolution arguments = (%v, %q), want request context and Qz7Rt2", gotCtx, code)
		}
		return "", repositories.ErrURLNotFound
	}), private)

	result, err := service.Resolve(ctx, "Qz7Rt2", "raw-link-token")
	if err != nil || result.OriginalURL != "https://example.com/private" || result.PasswordRequired {
		t.Errorf("Resolve() = (%+v, %v), want private destination without password page", result, err)
	}
}

func TestRedirectServiceResolvePrivateWithInvalidOrExpiredCookie(t *testing.T) {
	service := services.NewRedirectService(publicLinkResolverStub(func(context.Context, string) (string, error) {
		return "", repositories.ErrURLNotFound
	}), privateLinkResolverStub{
		findFunc: func(context.Context, string) (int64, string, error) { return 42, "stored-password-hash", nil },
		resolveFunc: func(context.Context, int64, []byte) (string, error) {
			return "", repositories.ErrURLNotFound
		},
	})

	result, err := service.Resolve(t.Context(), "Qz7Rt2", "invalid-token")
	if err != nil || !result.PasswordRequired || result.OriginalURL != "" {
		t.Errorf("Resolve() = (%+v, %v), want password page without destination", result, err)
	}
}

func TestRedirectServiceResolveMissingLink(t *testing.T) {
	service := services.NewRedirectService(publicLinkResolverStub(func(context.Context, string) (string, error) {
		return "", repositories.ErrURLNotFound
	}), privateLinkResolverStub{findFunc: func(context.Context, string) (int64, string, error) {
		return 0, "", repositories.ErrURLNotFound
	}})

	result, err := service.Resolve(t.Context(), "MissingCode", "")
	if !errors.Is(err, services.ErrURLNotFound) || result != (services.RedirectResult{}) {
		t.Errorf("Resolve() = (%+v, %v), want empty result and ErrURLNotFound", result, err)
	}
}

func TestRedirectServiceResolvePropagatesInfrastructureErrors(t *testing.T) {
	publicErr := errors.New("public database failure")
	privateFindErr := errors.New("private lookup failure")
	privateResolveErr := errors.New("private database failure")
	tests := []struct {
		name       string
		publicErr  error
		findErr    error
		resolveErr error
		wantErr    error
	}{
		{name: "public lookup", publicErr: publicErr, wantErr: publicErr},
		{name: "private lookup", publicErr: repositories.ErrURLNotFound, findErr: privateFindErr, wantErr: privateFindErr},
		{name: "private resolution", publicErr: repositories.ErrURLNotFound, resolveErr: privateResolveErr, wantErr: privateResolveErr},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			private := privateLinkResolverStub{
				findFunc: func(context.Context, string) (int64, string, error) {
					return 42, "stored-password-hash", tt.findErr
				},
				resolveFunc: func(context.Context, int64, []byte) (string, error) {
					return "", tt.resolveErr
				},
			}
			service := services.NewRedirectService(publicLinkResolverStub(func(context.Context, string) (string, error) {
				return "", tt.publicErr
			}), private)

			result, err := service.Resolve(t.Context(), "Qz7Rt2", "raw-link-token")
			if !errors.Is(err, tt.wantErr) || result != (services.RedirectResult{}) {
				t.Errorf("Resolve() = (%+v, %v), want empty result and wrapped error %v", result, err, tt.wantErr)
			}
		})
	}
}
