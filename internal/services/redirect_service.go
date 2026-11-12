package services

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/fredsaggio/url-shortener/internal/messaging"
	"github.com/fredsaggio/url-shortener/internal/repositories"
	"github.com/fredsaggio/url-shortener/internal/sessiontoken"
)

var ErrClickNotRecorded = errors.New("click could not be recorded")

type PublicLinkResolver interface {
	ResolvePublic(ctx context.Context, shortCode string) (int64, string, error)
}

type HeatTracker interface {
	Record(ctx context.Context, shortCode string) (int64, error)
}

type PrivateLinkResolver interface {
	FindPrivateByShortCode(ctx context.Context, shortCode string) (int64, string, error)
	// ResolvePrivate must validate the token, link ID, and expiry in the same database operation.
	ResolvePrivate(ctx context.Context, linkID int64, tokenHash []byte) (string, error)
}

type DestinationCache interface {
	Get(ctx context.Context, shortCode string) (int64, string, bool, error)
	Set(ctx context.Context, shortCode string, urlID int64, originalURL string) error
}

type ClickPublisher interface {
	Publish(ctx context.Context, event messaging.ClickEvent) error
}

type RedirectResult struct {
	OriginalURL      string
	PasswordRequired bool
}

type RedirectService struct {
	public       PublicLinkResolver
	private      PrivateLinkResolver
	heat         HeatTracker
	cache        DestinationCache
	hotThreshold int64
	publisher    ClickPublisher
	clicks       ClickEventStore
	now          func() time.Time
}

func NewRedirectService(public PublicLinkResolver, private PrivateLinkResolver, heat HeatTracker, cache DestinationCache, hotThreshold int64, publisher ClickPublisher, clicks ClickEventStore) *RedirectService {
	return &RedirectService{
		public:       public,
		private:      private,
		heat:         heat,
		cache:        cache,
		hotThreshold: hotThreshold,
		publisher:    publisher,
		clicks:       clicks,
		now:          time.Now,
	}
}

func (s *RedirectService) Resolve(ctx context.Context, shortCode, token string) (RedirectResult, error) {
	originalURL, err := s.ResolvePublic(ctx, shortCode)
	if err == nil {
		return RedirectResult{OriginalURL: originalURL}, nil
	}
	if !errors.Is(err, ErrURLNotFound) {
		return RedirectResult{}, fmt.Errorf("resolve public URL: %w", err)
	}

	linkID, _, err := s.private.FindPrivateByShortCode(ctx, shortCode)
	if err != nil {
		if errors.Is(err, repositories.ErrURLNotFound) {
			return RedirectResult{}, ErrURLNotFound
		}
		return RedirectResult{}, fmt.Errorf("find private URL: %w", err)
	}
	if token == "" {
		return RedirectResult{PasswordRequired: true}, nil
	}

	originalURL, err = s.private.ResolvePrivate(ctx, linkID, sessiontoken.Hash(token))
	if err != nil {
		if errors.Is(err, repositories.ErrURLNotFound) {
			return RedirectResult{PasswordRequired: true}, nil
		}
		return RedirectResult{}, fmt.Errorf("resolve private URL: %w", err)
	}

	if err := s.countClick(ctx, linkID); err != nil {
		return RedirectResult{}, err
	}

	return RedirectResult{OriginalURL: originalURL}, nil
}

func (s *RedirectService) ResolvePublic(ctx context.Context, shortCode string) (string, error) {
	urlID, originalURL, cached, err := s.cache.Get(ctx, shortCode)
	if err != nil {
		slog.Warn("read cached URL destination failed", "short_code", shortCode, "error", err)
		cached = false
	}

	if !cached {
		urlID, originalURL, err = s.public.ResolvePublic(ctx, shortCode)
		if err != nil {
			if errors.Is(err, repositories.ErrURLNotFound) {
				return "", ErrURLNotFound
			}
			return "", fmt.Errorf("resolve public URL: %w", err)
		}
	}

	if err := s.countClick(ctx, urlID); err != nil {
		return "", err
	}

	heat, err := s.heat.Record(ctx, shortCode)
	if err != nil {
		slog.Warn("record public URL heat failed", "short_code", shortCode, "error", err)
		return originalURL, nil
	}

	// Admitting only on a miss keeps the TTL as the maximum staleness: a hot
	// link is read from PostgreSQL again after each expiry.
	if !cached && heat >= s.hotThreshold {
		if err := s.cache.Set(ctx, shortCode, urlID, originalURL); err != nil {
			slog.Warn("cache hot URL destination failed", "short_code", shortCode, "error", err)
		}
	}

	return originalURL, nil
}

// The fallback reuses the event ID: if a publish that timed out still reaches
// the queue, the worker finds the event already processed.
func (s *RedirectService) countClick(ctx context.Context, urlID int64) error {
	event := messaging.NewClickEvent(urlID, s.now())

	publishErr := s.publisher.Publish(ctx, event)
	if publishErr == nil {
		return nil
	}

	slog.WarnContext(ctx, "publish click event failed; counting synchronously", "event_id", event.EventID, "url_id", urlID, "error", publishErr)

	if _, err := s.clicks.CountClickOnce(ctx, event.EventID, urlID); err != nil {
		if errors.Is(err, repositories.ErrURLNotFound) {
			return ErrURLNotFound
		}
		return fmt.Errorf("%w: publish: %w; count synchronously: %w", ErrClickNotRecorded, publishErr, err)
	}

	return nil
}
