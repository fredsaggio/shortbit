package models

import (
	"time"
	"uuid"
)

type User struct {
	ID              uuid.UUID
	Email           string
	FullName        *string // Nil for accounts created before names existed.
	EmailVerifiedAt time.Time
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

// Account is the user plus how they can sign in, as shown on their settings page.
type Account struct {
	User
	HasPassword bool
	HasGoogle   bool
}

type PasswordRegistrationAttempt struct {
	TokenHash             []byte
	Email                 string
	FullName              *string
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

type PasswordResetAttempt struct {
	TokenHash             []byte
	UserID                uuid.UUID
	VerificationProofHash []byte
	FailedAttempts        int16
	LockedUntil           *time.Time
	LastCodeSentAt        time.Time
	CodeExpiresAt         time.Time
	AttemptExpiresAt      time.Time
	UsedAt                *time.Time
	CreatedAt             time.Time
	UpdatedAt             time.Time
}
