package models

import (
	"time"
	"uuid"
)

type User struct {
	ID        uuid.UUID
	Email     string
	CreatedAt time.Time
	UpdatedAt time.Time
}

type PasswordCredential struct {
	UserID       uuid.UUID
	PasswordHash string
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

type AuthIdentity struct {
	UserID         uuid.UUID
	Provider       string
	ProviderUserID string
	CreatedAt      time.Time
}

type UserSession struct {
	TokenHash []byte
	UserID    uuid.UUID
	CreatedAt time.Time
	ExpiresAt time.Time
}
