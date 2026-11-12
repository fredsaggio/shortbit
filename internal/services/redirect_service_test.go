package services_test

import (
	"bytes"
	"context"
	"errors"
	"testing"
	"uuid"

	"github.com/fredsaggio/url-shortener/internal/messaging"
	"github.com/fredsaggio/url-shortener/internal/repositories"
	"github.com/fredsaggio/url-shortener/internal/services"
	"github.com/fredsaggio/url-shortener/internal/sessiontoken"
)

type publicLinkResolverStub func(context.Context, string) (int64, string, error)

func (resolve publicLinkResolverStub) ResolvePublic(ctx context.Context, code string) (int64, string, error) {
	return resolve(ctx, code)
}

func publicNotFound(context.Context, string) (int64, string, error) {
	return 0, "", repositories.ErrURLNotFound
}

type heatTrackerStub func(context.Context, string) (int64, error)

func (record heatTrackerStub) Record(ctx context.Context, code string) (int64, error) {
	return record(ctx, code)
}

func noOpHeatTracker(context.Context, string) (int64, error) { return 0, nil }

func unexpectedHeatTracker(t *testing.T) heatTrackerStub {
	t.Helper()
	return func(context.Context, string) (int64, error) {
		t.Error("Record() called without a successful public redirect")
		return 0, nil
	}
}

type clickPublisherStub struct {
	err    error
	events []messaging.ClickEvent
}

func (p *clickPublisherStub) Publish(_ context.Context, event messaging.ClickEvent) error {
	p.events = append(p.events, event)
	return p.err
}

type clickEventStoreStub struct {
	err   error
	calls []struct {
		eventID uuid.UUID
		urlID   int64
	}
}

func (s *clickEventStoreStub) CountClickOnce(_ context.Context, eventID uuid.UUID, urlID int64) (bool, error) {
	s.calls = append(s.calls, struct {
		eventID uuid.UUID
		urlID   int64
	}{eventID, urlID})
	return s.err == nil, s.err
}

type privateLinkResolverStub struct {
	findFunc    func(context.Context, string) (int64, string, error)
	resolveFunc func(context.Context, int64, []byte) (string, error)
}

func (s privateLinkResolverStub) FindPrivateByShortCode(ctx context.Context, code string) (int64, string, error) {
	return s.findFunc(ctx, code)
}

func (s privateLinkResolverStub) ResolvePrivate(ctx context.Context, linkID int64, tokenHash []byte) (string, error) {
	return s.resolveFunc(ctx, linkID, tokenHash)
}

func newRedirectService(public publicLinkResolverStub, private services.PrivateLinkResolver, heat heatTrackerStub) (*services.RedirectService, *clickPublisherStub, *clickEventStoreStub) {
	publisher := &clickPublisherStub{}
	clicks := &clickEventStoreStub{}
	return services.NewRedirectService(public, private, heat, publisher, clicks), publisher, clicks
}

func TestRedirectServiceResolvePublicReturnsOriginalURL(t *testing.T) {
	type contextKey struct{}
	ctx := context.WithValue(t.Context(), contextKey{}, "request-context")
	called := false
	heatCalled := false
	service, publisher, clicks := newRedirectService(func(gotCtx context.Context, gotCode string) (int64, string, error) {
		called = true
		if gotCtx.Value(contextKey{}) != "request-context" || gotCode != "Ab3dX9" {
			t.Errorf("ResolvePublic() arguments = (%v, %q), want request context and Ab3dX9", gotCtx, gotCode)
		}
		return 7, "https://example.com/article", nil
	}, nil, func(gotCtx context.Context, gotCode string) (int64, error) {
		heatCalled = true
		if !called || gotCtx.Value(contextKey{}) != "request-context" || gotCode != "Ab3dX9" {
			t.Errorf("Record() arguments = (%v, %q), repository called = %t; want request context, Ab3dX9, true", gotCtx, gotCode, called)
		}
		return 1, nil
	})

	got, err := service.ResolvePublic(ctx, "Ab3dX9")
	if err != nil || got != "https://example.com/article" || !called || !heatCalled {
		t.Errorf("ResolvePublic() = (%q, %v), repository called = %t, heat called = %t; want original URL, nil, true, true", got, err, called, heatCalled)
	}
	if len(publisher.events) != 1 || publisher.events[0].URLID != 7 {
		t.Errorf("published events = %+v, want one event for URL 7", publisher.events)
	}
	if len(clicks.calls) != 0 {
		t.Errorf("synchronous counts = %d, want 0 after a confirmed publish", len(clicks.calls))
	}
}

func TestRedirectServiceResolvePublicFallsBackToSynchronousCount(t *testing.T) {
	service, publisher, clicks := newRedirectService(func(context.Context, string) (int64, string, error) {
		return 7, "https://example.com/article", nil
	}, nil, noOpHeatTracker)
	publisher.err = messaging.ErrPublisherUnavailable

	got, err := service.ResolvePublic(t.Context(), "Ab3dX9")
	if err != nil || got != "https://example.com/article" {
		t.Fatalf("ResolvePublic() = (%q, %v), want original URL", got, err)
	}

	if len(publisher.events) != 1 || len(clicks.calls) != 1 {
		t.Fatalf("published %d events and counted %d synchronously, want 1 and 1", len(publisher.events), len(clicks.calls))
	}
	if clicks.calls[0].eventID != publisher.events[0].EventID || clicks.calls[0].urlID != 7 {
		t.Errorf("synchronous count = %+v, want the published event ID %v and URL 7", clicks.calls[0], publisher.events[0].EventID)
	}
}

func TestRedirectServiceResolvePublicFailsWhenClickCannotBeRecorded(t *testing.T) {
	storeErr := errors.New("database unavailable")
	service, publisher, clicks := newRedirectService(func(context.Context, string) (int64, string, error) {
		return 7, "https://example.com/article", nil
	}, nil, unexpectedHeatTracker(t))
	publisher.err = messaging.ErrPublisherUnavailable
	clicks.err = storeErr

	got, err := service.ResolvePublic(t.Context(), "Ab3dX9")
	if got != "" || !errors.Is(err, services.ErrClickNotRecorded) || !errors.Is(err, storeErr) {
		t.Errorf("ResolvePublic() = (%q, %v), want empty URL and ErrClickNotRecorded wrapping the store error", got, err)
	}
}

func TestRedirectServiceResolvePublicMapsNotFound(t *testing.T) {
	service, publisher, _ := newRedirectService(publicNotFound, nil, unexpectedHeatTracker(t))

	got, err := service.ResolvePublic(t.Context(), "missing")
	if got != "" || !errors.Is(err, services.ErrURLNotFound) {
		t.Errorf("ResolvePublic() = (%q, %v), want empty URL and ErrURLNotFound", got, err)
	}
	if len(publisher.events) != 0 {
		t.Errorf("published %d events for a missing link, want 0", len(publisher.events))
	}
}

func TestRedirectServiceResolvePublicWrapsRepositoryError(t *testing.T) {
	wantErr := errors.New("database unavailable")
	service, _, _ := newRedirectService(func(context.Context, string) (int64, string, error) {
		return 0, "", wantErr
	}, nil, unexpectedHeatTracker(t))

	got, err := service.ResolvePublic(t.Context(), "Ab3dX9")
	if got != "" || !errors.Is(err, wantErr) {
		t.Errorf("ResolvePublic() = (%q, %v), want empty URL and wrapped repository error", got, err)
	}
}

func TestRedirectServiceResolvePublicIgnoresHeatError(t *testing.T) {
	heatErr := errors.New("Redis unavailable")
	service, _, _ := newRedirectService(func(context.Context, string) (int64, string, error) {
		return 7, "https://example.com/article", nil
	}, nil, func(context.Context, string) (int64, error) {
		return 0, heatErr
	})

	got, err := service.ResolvePublic(t.Context(), "Ab3dX9")
	if err != nil || got != "https://example.com/article" {
		t.Errorf("ResolvePublic() = (%q, %v), want original URL despite Redis error", got, err)
	}
}

func TestRedirectServiceResolvePublic(t *testing.T) {
	private := privateLinkResolverStub{findFunc: func(context.Context, string) (int64, string, error) {
		t.Fatal("private lookup called for public link")
		return 0, "", nil
	}}
	service, _, _ := newRedirectService(func(_ context.Context, code string) (int64, string, error) {
		if code != "Ab3dX9" {
			t.Errorf("public shortcode = %q, want Ab3dX9", code)
		}
		return 7, "https://example.com/public", nil
	}, private, noOpHeatTracker)

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
	service, publisher, _ := newRedirectService(publicNotFound, private, unexpectedHeatTracker(t))

	result, err := service.Resolve(t.Context(), "Qz7Rt2", "")
	if err != nil || !result.PasswordRequired || result.OriginalURL != "" {
		t.Errorf("Resolve() = (%+v, %v), want password page without destination", result, err)
	}
	if len(publisher.events) != 0 {
		t.Errorf("published %d events for the password page, want 0", len(publisher.events))
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
	service, publisher, _ := newRedirectService(func(gotCtx context.Context, code string) (int64, string, error) {
		if gotCtx.Value(contextKey{}) != "request-context" || code != "Qz7Rt2" {
			t.Errorf("public resolution arguments = (%v, %q), want request context and Qz7Rt2", gotCtx, code)
		}
		return 0, "", repositories.ErrURLNotFound
	}, private, unexpectedHeatTracker(t))

	result, err := service.Resolve(ctx, "Qz7Rt2", "raw-link-token")
	if err != nil || result.OriginalURL != "https://example.com/private" || result.PasswordRequired {
		t.Errorf("Resolve() = (%+v, %v), want private destination without password page", result, err)
	}
	if len(publisher.events) != 1 || publisher.events[0].URLID != 42 {
		t.Errorf("published events = %+v, want one event for link 42", publisher.events)
	}
}

func TestRedirectServiceResolvePrivateFailsWhenClickCannotBeRecorded(t *testing.T) {
	service, publisher, clicks := newRedirectService(publicNotFound, privateLinkResolverStub{
		findFunc:    func(context.Context, string) (int64, string, error) { return 42, "stored-password-hash", nil },
		resolveFunc: func(context.Context, int64, []byte) (string, error) { return "https://example.com/private", nil },
	}, unexpectedHeatTracker(t))
	publisher.err = messaging.ErrPublisherUnavailable
	clicks.err = errors.New("database unavailable")

	result, err := service.Resolve(t.Context(), "Qz7Rt2", "raw-link-token")
	if !errors.Is(err, services.ErrClickNotRecorded) || result != (services.RedirectResult{}) {
		t.Errorf("Resolve() = (%+v, %v), want empty result and ErrClickNotRecorded", result, err)
	}
}

func TestRedirectServiceResolvePrivateWithInvalidOrExpiredCookie(t *testing.T) {
	service, publisher, _ := newRedirectService(publicNotFound, privateLinkResolverStub{
		findFunc: func(context.Context, string) (int64, string, error) { return 42, "stored-password-hash", nil },
		resolveFunc: func(context.Context, int64, []byte) (string, error) {
			return "", repositories.ErrURLNotFound
		},
	}, unexpectedHeatTracker(t))

	result, err := service.Resolve(t.Context(), "Qz7Rt2", "invalid-token")
	if err != nil || !result.PasswordRequired || result.OriginalURL != "" {
		t.Errorf("Resolve() = (%+v, %v), want password page without destination", result, err)
	}
	if len(publisher.events) != 0 {
		t.Errorf("published %d events for an invalid cookie, want 0", len(publisher.events))
	}
}

func TestRedirectServiceResolveMissingLink(t *testing.T) {
	service, _, _ := newRedirectService(publicNotFound, privateLinkResolverStub{findFunc: func(context.Context, string) (int64, string, error) {
		return 0, "", repositories.ErrURLNotFound
	}}, unexpectedHeatTracker(t))

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
			service, _, _ := newRedirectService(func(context.Context, string) (int64, string, error) {
				return 0, "", tt.publicErr
			}, private, unexpectedHeatTracker(t))

			result, err := service.Resolve(t.Context(), "Qz7Rt2", "raw-link-token")
			if !errors.Is(err, tt.wantErr) || result != (services.RedirectResult{}) {
				t.Errorf("Resolve() = (%+v, %v), want empty result and wrapped error %v", result, err, tt.wantErr)
			}
		})
	}
}
