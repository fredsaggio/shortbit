package models

import (
	"time"
	"uuid"
)

type User struct {
	ID              uuid.UUID
	Email           string
	EmailVerifiedAt time.Time
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

type PasswordRegistrationAttempt struct {
	TokenHash             []byte
	Email                 string
	PasswordHash          string
	VerificationProofHash []byte
	FailedAttempts        int16
	LockedUntil           *time.Time // Ponteiro porque pode ser nullable.
	LastCodeSentAt        time.Time
	CodeExpiresAt         time.Time
	AttemptExpiresAt      time.Time
	CreatedAt             time.Time
	UpdatedAt             time.Time
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

type PasswordResetToken struct {
	TokenHash []byte
	UserID    uuid.UUID
	CreatedAt time.Time
	ExpiresAt time.Time
	UsedAt    *time.Time
}
