package models

import (
	"time"
	"uuid"
)

type URL struct {
	ID           uuid.UUID
	ShortCode    string
	UserID       uuid.UUID
	OriginalURL  string
	Visibility   Visibility
	PasswordHash string
	ClickCount   int
	ExpiresAt    time.Time
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

type Visibility string

const (
	Public Visibility = "public"
	Private Visibility = "private"
)



