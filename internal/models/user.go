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

type UserWithPassword struct {
	ID uuid.UUID
	PasswordHash string
	CreatedAt time.Time
	UpdatedAt time.Time
}

type UserWithProvider struct {
	ID uuid.UUID
	Provider string
	ProviderUserID string
	CreatedAt time.Time
}

type UserSessions struct {
	
}