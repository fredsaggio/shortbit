package services

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"unicode/utf8"
	"uuid"

	"github.com/fredsaggio/url-shortener/internal/models"
	"github.com/fredsaggio/url-shortener/internal/repositories"
)

const (
	minLinkPasswordCharacters = 8
	maxLinkPasswordBytes      = 1024
	maxURLListLimit           = 20
)

var (
	ErrInvalidOriginalURL     = errors.New("invalid original URL")
	ErrURLTooLong             = errors.New("URL too long")
	ErrURLPointsToShortDomain = errors.New("original URL points to short domain")
	ErrInvalidURLVisibility   = errors.New("invalid url visibility")
	ErrUnexpectedLinkPassword = errors.New("unexpected link password")
	ErrLinkPasswordTooShort   = errors.New("password too short")
	ErrLinkPasswordTooLong    = errors.New("password too long")
	ErrInvalidURLListLimit    = errors.New("invalid URL list limit")
	ErrURLNotFound            = errors.New("url not found")
)

type URLRepository interface {
	ReserveID(ctx context.Context) (int64, error)
	Create(ctx context.Context, url models.URL) (models.URL, error)
	List(ctx context.Context, userID uuid.UUID, limit int, cursor *repositories.URLCursor) ([]models.URL, error)
	GetByShortcode(ctx context.Context, userID uuid.UUID, shortCode string) (models.URL, error)
}

type ShortCodeGenerator interface {
	Generate(id int64) (string, error)
}

type URLServiceConfig struct {
	PublicBaseURL       string
	MaxOriginalURLBytes int
}

type CreateURLResult struct {
	ShortCode string
	ShortURL  string
}

type ListURLResult struct {
	URLs       []models.URL
	NextCursor *repositories.URLCursor
}

type URLService struct {
	repo                URLRepository
	generator           ShortCodeGenerator
	hasher              PasswordHasher
	publicBaseURL       string
	publicHost          string
	maxOriginalURLBytes int
}

func NewURLService(repo URLRepository, generator ShortCodeGenerator, hasher PasswordHasher, config URLServiceConfig) (*URLService, error) {
	// Package URL pra parsear a string e pegar as informações separadas (scheme, rawquery, etc)
	base, err := url.Parse(config.PublicBaseURL)
	if err != nil || (base.Scheme != "http" && base.Scheme != "https") ||
		base.Hostname() == "" || base.User != nil || strings.ContainsAny(config.PublicBaseURL, "?#") ||
		(base.Path != "" && base.Path != "/") {
		return nil, errors.New("invalid public base URL configuration")
	}
	if config.MaxOriginalURLBytes <= 0 {
		return nil, errors.New("invalid maximum original URL size")
	}

	return &URLService{
		repo:                repo,
		generator:           generator,
		hasher:              hasher,
		publicBaseURL:       strings.TrimRight(config.PublicBaseURL, "/"),
		publicHost:          strings.TrimSuffix(base.Hostname(), "."),
		maxOriginalURLBytes: config.MaxOriginalURLBytes,
	}, nil
}

func (s *URLService) Create(ctx context.Context, userID uuid.UUID, originalURL string, visibility models.Visibility, password string) (CreateURLResult, error) {
	if userID == uuid.Nil() {
		return CreateURLResult{}, ErrUnauthenticated
	}

	if originalURL == "" || originalURL != strings.TrimSpace(originalURL) {
		return CreateURLResult{}, ErrInvalidOriginalURL
	}

	if len(originalURL) > s.maxOriginalURLBytes {
		return CreateURLResult{}, ErrURLTooLong
	}

	parsed, err := url.Parse(originalURL)
	if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Hostname() == "" || parsed.User != nil || parsed.Opaque != "" {
		return CreateURLResult{}, ErrInvalidOriginalURL
	}

	if strings.EqualFold(strings.TrimSuffix(parsed.Hostname(), "."), s.publicHost) {
		return CreateURLResult{}, ErrURLPointsToShortDomain
	}

	if !visibility.IsValid() {
		return CreateURLResult{}, ErrInvalidURLVisibility
	}

	var passwordHash *string

	switch visibility {
	case models.VisibilityPublic:
		if password != "" {
			return CreateURLResult{}, ErrUnexpectedLinkPassword
		}
	case models.VisibilityPrivate:
		if utf8.RuneCountInString(password) < minLinkPasswordCharacters {
			return CreateURLResult{}, ErrLinkPasswordTooShort
		}

		if len(password) > maxLinkPasswordBytes {
			return CreateURLResult{}, ErrLinkPasswordTooLong
		}

		hash, err := s.hasher.Hash(password)

		if err != nil {
			return CreateURLResult{}, fmt.Errorf("hash link password: %w", err)
		}

		passwordHash = &hash
	}

	id, err := s.repo.ReserveID(ctx)

	if err != nil {
		return CreateURLResult{}, fmt.Errorf("reserve URL ID: %w", err)
	}

	shortcode, err := s.generator.Generate(id)

	if err != nil {
		return CreateURLResult{}, fmt.Errorf("generate shortcode: %w", err)
	}

	created, err := s.repo.Create(ctx, models.URL{
		ID:           id,
		ShortCode:    shortcode,
		UserID:       userID,
		OriginalURL:  originalURL,
		Visibility:   visibility,
		PasswordHash: passwordHash,
	})

	if err != nil {
		return CreateURLResult{}, fmt.Errorf("save URL: %w", err)
	}

	return CreateURLResult{
		ShortCode: created.ShortCode,
		ShortURL:  s.publicBaseURL + "/" + created.ShortCode,
	}, nil
}

func (s *URLService) List(ctx context.Context, userID uuid.UUID, limit int, cursor *repositories.URLCursor) (ListURLResult, error) {
	if userID == uuid.Nil() {
		return ListURLResult{}, ErrUnauthenticated
	}
	if limit <= 0 || limit > maxURLListLimit {
		return ListURLResult{}, ErrInvalidURLListLimit
	}

	urls, err := s.repo.List(ctx, userID, limit+1, cursor)

	if err != nil {
		return ListURLResult{}, fmt.Errorf("list URLs: %w", err)
	}

	result := ListURLResult{URLs: urls}

	if len(urls) > limit {
		result.URLs = urls[:limit]
		last := result.URLs[len(result.URLs)-1]
		result.NextCursor = &repositories.URLCursor{CreatedAt: last.CreatedAt, ID: last.ID}
	}

	return result, nil
}

func (s *URLService) GetByShortCode(ctx context.Context, userID uuid.UUID, shortCode string) (models.URL, error) {
	if userID == uuid.Nil() {
		return models.URL{}, ErrUnauthenticated
	}

	link, err := s.repo.GetByShortcode(ctx, userID, shortCode)

	if err != nil {
		if errors.Is(err, repositories.ErrURLNotFound) {
			return models.URL{}, ErrURLNotFound
		}

		return models.URL{}, fmt.Errorf("get url by shortcode: %w", err)
	}

	return link, nil
}
