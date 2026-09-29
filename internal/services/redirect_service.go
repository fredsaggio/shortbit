package services

import (
	"context"
	"errors"
	"fmt"

	"github.com/fredsaggio/url-shortener/internal/repositories"
	"github.com/fredsaggio/url-shortener/internal/sessiontoken"
)

type PublicLinkResolver interface {
	ResolvePublicAndCountClick(ctx context.Context, shortCode string) (string, error)
}

type PrivateLinkResolver interface {
	FindPrivateByShortCode(ctx context.Context, shortCode string) (int64, string, error)
	// ResolvePrivateAndCountClick must validate the token, link ID, and expiry before incrementing the click in the same database operation.
	ResolvePrivateAndCountClick(ctx context.Context, linkID int64, tokenHash []byte) (string, error)
}

type RedirectResult struct {
	OriginalURL      string
	PasswordRequired bool
}

type RedirectService struct {
	public  PublicLinkResolver
	private PrivateLinkResolver
}

func NewRedirectService(public PublicLinkResolver, private PrivateLinkResolver) *RedirectService {
	return &RedirectService{public: public, private: private}
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

	originalURL, err = s.private.ResolvePrivateAndCountClick(ctx, linkID, sessiontoken.Hash(token))
	if err != nil {
		if errors.Is(err, repositories.ErrURLNotFound) {
			return RedirectResult{PasswordRequired: true}, nil
		}
		return RedirectResult{}, fmt.Errorf("resolve private URL: %w", err)
	}

	return RedirectResult{OriginalURL: originalURL}, nil
}

func (s *RedirectService) ResolvePublic(ctx context.Context, shortCode string) (string, error) {
	originalURL, err := s.public.ResolvePublicAndCountClick(ctx, shortCode)
	if err != nil {
		if errors.Is(err, repositories.ErrURLNotFound) {
			return "", ErrURLNotFound
		}
		return "", fmt.Errorf("resolve public URL and count click: %w", err)
	}
	return originalURL, nil
}
