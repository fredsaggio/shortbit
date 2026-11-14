package services

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/fredsaggio/url-shortener/internal/repositories"
)

var ErrIncorrectLinkPassword = errors.New("incorrect link access password")

type LinkURLRepository interface {
	FindPrivateByShortCode(ctx context.Context, shortCode string) (int64, string, error)
}

type LinkAccessSessionRepository interface {
	CreateLinkAccessSession(ctx context.Context, linkID int64, tokenHash []byte, expiresAt time.Time) error
}

type LinkAccessSessionResult struct {
	Token     string
	ExpiresAt time.Time
}

type LinkAccessSessionService struct {
	urlRepo       LinkURLRepository
	linkRepo      LinkAccessSessionRepository
	sessionTTL    time.Duration
	generateToken TokenGenerator
	hasher        PasswordComparator
}

func NewLinkAccessSessionService(urlRepo LinkURLRepository, linkRepo LinkAccessSessionRepository, sessionTTL time.Duration, generateToken TokenGenerator, hasher PasswordComparator) *LinkAccessSessionService {
	return &LinkAccessSessionService{
		urlRepo:       urlRepo,
		linkRepo:      linkRepo,
		sessionTTL:    sessionTTL,
		generateToken: generateToken,
		hasher:        hasher,
	}
}

func (s *LinkAccessSessionService) CreateSession(ctx context.Context, shortCode, password string) (LinkAccessSessionResult, error) {
	id, passwordHash, err := s.urlRepo.FindPrivateByShortCode(ctx, shortCode)

	if err != nil {
		if errors.Is(err, repositories.ErrURLNotFound) {
			return LinkAccessSessionResult{}, ErrURLNotFound
		}
		return LinkAccessSessionResult{}, fmt.Errorf("find private url by shortCode: %w", err)
	}

	matches, err := s.hasher.Compare(password, passwordHash)

	if err != nil {
		return LinkAccessSessionResult{}, fmt.Errorf("compare password: %w", err)
	}

	if !matches {
		return LinkAccessSessionResult{}, ErrIncorrectLinkPassword
	}

	token, tokenHash, err := s.generateToken()

	if err != nil {
		return LinkAccessSessionResult{}, fmt.Errorf("generate token: %w", err)
	}

	expirationTime := time.Now().UTC().Add(s.sessionTTL)

	err = s.linkRepo.CreateLinkAccessSession(ctx, id, tokenHash, expirationTime)

	if err != nil {
		return LinkAccessSessionResult{}, fmt.Errorf("create link access session: %w", err)
	}

	return LinkAccessSessionResult{
		Token:     token,
		ExpiresAt: expirationTime,
	}, nil
}
