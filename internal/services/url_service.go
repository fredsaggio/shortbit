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
)

const (
	minLinkPasswordCharacters = 8
	maxLinkPasswordBytes      = 1024
)

var (
	ErrInvalidOriginalURL     = errors.New("invalid original URL")
	ErrURLTooLong             = errors.New("URL too long")
	ErrURLPointsToShortDomain = errors.New("original URL points to short domain")
	ErrInvalidURLVisibility   = errors.New("invalid url visibility")
	ErrUnexpectedLinkPassword = errors.New("unexpected link password")
	ErrLinkPasswordTooShort   = errors.New("password too short")
	ErrLinkPasswordTooLong    = errors.New("password too long")
)

type URLRepository interface {
	ReserveID(ctx context.Context) (int64, error)
	Create(ctx context.Context, url models.URL) (models.URL, error)
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
