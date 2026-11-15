package services

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
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
	ErrEmptyURLUpdate         = errors.New("nothing to update")
	ErrLinkPasswordRequired   = errors.New("link password required")
)

type URLRepository interface {
	ReserveID(ctx context.Context) (int64, error)
	Create(ctx context.Context, url models.URL) (models.URL, error)
	List(ctx context.Context, userID uuid.UUID, limit int, cursor *repositories.URLCursor) ([]models.URL, error)
	GetByShortcode(ctx context.Context, userID uuid.UUID, shortCode string) (models.URL, error)
	Update(ctx context.Context, userID uuid.UUID, shortCode string, edit repositories.URLEdit) (models.URL, error)
	Delete(ctx context.Context, userID uuid.UUID, shortCode string) error
}

// DestinationInvalidator drops cached destinations of edited or deleted links.
type DestinationInvalidator interface {
	Delete(ctx context.Context, shortCodes ...string) error
}

// URLUpdate holds the fields of an edit; nil leaves a field as it is. On a
// private link, an empty Password keeps the current one.
type URLUpdate struct {
	OriginalURL *string
	Visibility  *models.Visibility
	Password    *string
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
	destinations        DestinationInvalidator
	publicBaseURL       string
	publicHost          string
	maxOriginalURLBytes int
}

func NewURLService(repo URLRepository, generator ShortCodeGenerator, hasher PasswordHasher, destinations DestinationInvalidator, config URLServiceConfig) (*URLService, error) {
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
		destinations:        destinations,
		publicBaseURL:       strings.TrimRight(config.PublicBaseURL, "/"),
		publicHost:          strings.TrimSuffix(base.Hostname(), "."),
		maxOriginalURLBytes: config.MaxOriginalURLBytes,
	}, nil
}

func (s *URLService) Create(ctx context.Context, userID uuid.UUID, originalURL string, visibility models.Visibility, password string) (CreateURLResult, error) {
	if userID == uuid.Nil() {
		return CreateURLResult{}, ErrUnauthenticated
	}

	if err := s.validateOriginalURL(originalURL); err != nil {
		return CreateURLResult{}, err
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
		hash, err := s.hashLinkPassword(password)
		if err != nil {
			return CreateURLResult{}, err
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

// Update edits one of the owner's links. Switching to private needs a
// password; a new password ends every unlock session made with the old one.
// The cached destination is dropped before and after the write, so a redirect
// racing the edit cannot leave the old destination cached.
func (s *URLService) Update(ctx context.Context, userID uuid.UUID, shortCode string, update URLUpdate) (models.URL, error) {
	if userID == uuid.Nil() {
		return models.URL{}, ErrUnauthenticated
	}
	if update.OriginalURL == nil && update.Visibility == nil && update.Password == nil {
		return models.URL{}, ErrEmptyURLUpdate
	}

	current, err := s.repo.GetByShortcode(ctx, userID, shortCode)
	if err != nil {
		if errors.Is(err, repositories.ErrURLNotFound) {
			return models.URL{}, ErrURLNotFound
		}
		return models.URL{}, fmt.Errorf("get URL to update: %w", err)
	}

	edit := repositories.URLEdit{OriginalURL: current.OriginalURL, Visibility: current.Visibility, KeepPasswordHash: true}

	if update.OriginalURL != nil {
		if err := s.validateOriginalURL(*update.OriginalURL); err != nil {
			return models.URL{}, err
		}
		edit.OriginalURL = *update.OriginalURL
	}

	if update.Visibility != nil {
		if !update.Visibility.IsValid() {
			return models.URL{}, ErrInvalidURLVisibility
		}
		edit.Visibility = *update.Visibility
	}

	newPassword := ""
	if update.Password != nil {
		newPassword = *update.Password
	}

	switch edit.Visibility {
	case models.VisibilityPublic:
		if newPassword != "" {
			return models.URL{}, ErrUnexpectedLinkPassword
		}
		edit.KeepPasswordHash = false
		edit.PasswordHash = nil
	case models.VisibilityPrivate:
		if newPassword == "" {
			if current.Visibility == models.VisibilityPublic {
				return models.URL{}, ErrLinkPasswordRequired
			}
			break
		}
		hash, err := s.hashLinkPassword(newPassword)
		if err != nil {
			return models.URL{}, err
		}
		edit.PasswordHash = &hash
		edit.KeepPasswordHash = false
		edit.RevokeAccessSessions = true
	}

	s.invalidateDestinations(ctx, shortCode)
	updated, err := s.repo.Update(ctx, userID, shortCode, edit)
	if err != nil {
		if errors.Is(err, repositories.ErrURLNotFound) {
			return models.URL{}, ErrURLNotFound
		}
		return models.URL{}, fmt.Errorf("update URL: %w", err)
	}
	s.invalidateDestinations(ctx, shortCode)

	return updated, nil
}

// Delete removes one of the owner's links for good. Anyone opening it after
// that gets a not found; the short code is never handed out again.
func (s *URLService) Delete(ctx context.Context, userID uuid.UUID, shortCode string) error {
	if userID == uuid.Nil() {
		return ErrUnauthenticated
	}

	s.invalidateDestinations(ctx, shortCode)
	if err := s.repo.Delete(ctx, userID, shortCode); err != nil {
		if errors.Is(err, repositories.ErrURLNotFound) {
			return ErrURLNotFound
		}
		return fmt.Errorf("delete URL: %w", err)
	}
	s.invalidateDestinations(ctx, shortCode)

	return nil
}

func (s *URLService) validateOriginalURL(originalURL string) error {
	if originalURL == "" || originalURL != strings.TrimSpace(originalURL) {
		return ErrInvalidOriginalURL
	}

	if len(originalURL) > s.maxOriginalURLBytes {
		return ErrURLTooLong
	}

	parsed, err := url.Parse(originalURL)
	if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Hostname() == "" || parsed.User != nil || parsed.Opaque != "" {
		return ErrInvalidOriginalURL
	}

	if strings.EqualFold(strings.TrimSuffix(parsed.Hostname(), "."), s.publicHost) {
		return ErrURLPointsToShortDomain
	}

	return nil
}

func (s *URLService) hashLinkPassword(password string) (string, error) {
	if utf8.RuneCountInString(password) < minLinkPasswordCharacters {
		return "", ErrLinkPasswordTooShort
	}

	if len(password) > maxLinkPasswordBytes {
		return "", ErrLinkPasswordTooLong
	}

	hash, err := s.hasher.Hash(password)
	if err != nil {
		return "", fmt.Errorf("hash link password: %w", err)
	}

	return hash, nil
}

// A failed invalidation leaves the old destination cached until its entry
// expires (URL_CACHE_TTL); the database write itself already happened.
func (s *URLService) invalidateDestinations(ctx context.Context, shortCodes ...string) {
	if s.destinations == nil {
		return
	}
	if err := s.destinations.Delete(ctx, shortCodes...); err != nil {
		slog.WarnContext(ctx, "drop cached URL destinations failed", "error", err)
	}
}
