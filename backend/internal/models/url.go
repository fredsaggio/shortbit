package models

import (
	"time"
	"uuid"
)

type URL struct {
	ID           int64
	ShortCode    string
	UserID       uuid.UUID
	OriginalURL  string
	Visibility   Visibility
	PasswordHash *string
	ClickCount   int64
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

type LinkAccessSession struct {
	TokenHash []byte
	LinkID    int64
	CreatedAt time.Time
	ExpiresAt time.Time
}

type Visibility string

const (
	VisibilityPublic  Visibility = "public"
	VisibilityPrivate Visibility = "private"
)

func (v Visibility) IsValid() bool {
	return v == VisibilityPublic || v == VisibilityPrivate
}
