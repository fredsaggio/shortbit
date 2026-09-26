//go:build integration

package repositories_test

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"errors"
	"testing"
	"time"
	"uuid"

	"github.com/fredsaggio/url-shortener/internal/db/dbtest"
	"github.com/fredsaggio/url-shortener/internal/models"
	"github.com/fredsaggio/url-shortener/internal/repositories"
	"github.com/fredsaggio/url-shortener/internal/verificationcode"
)

func TestUserRepositoryIntegration(t *testing.T) {
	pool := dbtest.Open(t)
	repository := repositories.NewUserRepository(pool)

	t.Run("creates password registration attempt", func(t *testing.T) {
		const (
			rawToken     = "password-registration-token"
			code         = "123456"
			email        = "pending-registration@example.com"
			passwordHash = "$argon2id$pending-registration-hash"
		)

		tokenHashArray := sha256.Sum256([]byte(rawToken))
		verificationProof := hmac.New(sha256.New, []byte(rawToken))
		_, _ = verificationProof.Write([]byte(code))

		now := time.Now().UTC().Truncate(time.Microsecond)
		attempt := models.PasswordRegistrationAttempt{
			TokenHash:             tokenHashArray[:],
			Email:                 email,
			PasswordHash:          passwordHash,
			VerificationProofHash: verificationProof.Sum(nil),
			LastCodeSentAt:        now,
			CodeExpiresAt:         now.Add(10 * time.Minute),
			AttemptExpiresAt:      now.Add(30 * time.Minute),
		}

		if err := repository.CreatePasswordRegistrationAttempt(t.Context(), attempt); err != nil {
			t.Fatalf("CreatePasswordRegistrationAttempt() error = %v", err)
		}

		var stored models.PasswordRegistrationAttempt
		err := pool.QueryRow(
			t.Context(),
			`
				SELECT
					token_hash,
					email,
					password_hash,
					verification_proof_hash,
					failed_attempts,
					locked_until,
					last_code_sent_at,
					code_expires_at,
					attempt_expires_at,
					created_at,
					updated_at
				FROM password_registration_attempts
				WHERE token_hash = $1
			`,
			attempt.TokenHash,
		).Scan(
			&stored.TokenHash,
			&stored.Email,
			&stored.PasswordHash,
			&stored.VerificationProofHash,
			&stored.FailedAttempts,
			&stored.LockedUntil,
			&stored.LastCodeSentAt,
			&stored.CodeExpiresAt,
			&stored.AttemptExpiresAt,
			&stored.CreatedAt,
			&stored.UpdatedAt,
		)
		if err != nil {
			t.Fatalf("query created password registration attempt: %v", err)
		}

		if !bytes.Equal(stored.TokenHash, attempt.TokenHash) {
			t.Errorf("stored token hash = %x, want %x", stored.TokenHash, attempt.TokenHash)
		}
		if stored.Email != attempt.Email {
			t.Errorf("stored email = %q, want %q", stored.Email, attempt.Email)
		}
		if stored.PasswordHash != attempt.PasswordHash {
			t.Errorf("stored password hash = %q, want %q", stored.PasswordHash, attempt.PasswordHash)
		}
		if !bytes.Equal(stored.VerificationProofHash, attempt.VerificationProofHash) {
			t.Errorf("stored verification proof hash = %x, want %x", stored.VerificationProofHash, attempt.VerificationProofHash)
		}
		if stored.FailedAttempts != 0 {
			t.Errorf("stored failed attempts = %d, want 0", stored.FailedAttempts)
		}
		if stored.LockedUntil != nil {
			t.Errorf("stored locked until = %v, want nil", stored.LockedUntil)
		}
		if !stored.LastCodeSentAt.Equal(attempt.LastCodeSentAt) {
			t.Errorf("stored last code sent at = %v, want %v", stored.LastCodeSentAt, attempt.LastCodeSentAt)
		}
		if !stored.CodeExpiresAt.Equal(attempt.CodeExpiresAt) {
			t.Errorf("stored code expires at = %v, want %v", stored.CodeExpiresAt, attempt.CodeExpiresAt)
		}
		if !stored.AttemptExpiresAt.Equal(attempt.AttemptExpiresAt) {
			t.Errorf("stored attempt expires at = %v, want %v", stored.AttemptExpiresAt, attempt.AttemptExpiresAt)
		}
		if stored.CreatedAt.IsZero() || stored.UpdatedAt.IsZero() {
			t.Error("stored registration attempt has zero database timestamps")
		}
		if bytes.Equal(stored.TokenHash, []byte(rawToken)) {
			t.Error("stored the raw registration token")
		}
		if bytes.Equal(stored.VerificationProofHash, []byte(code)) {
			t.Error("stored the raw verification code")
		}

		var userExists bool
		if err := pool.QueryRow(t.Context(), "SELECT EXISTS(SELECT 1 FROM users WHERE email = $1)", email).Scan(&userExists); err != nil {
			t.Fatalf("check user after creating registration attempt: %v", err)
		}
		if userExists {
			t.Error("CreatePasswordRegistrationAttempt() created a user before email confirmation")
		}
	})

	t.Run("finds password registration attempt by token hash", func(t *testing.T) {
		tokenHash := sha256.Sum256([]byte("registration-token-to-find"))
		proofHash := sha256.Sum256([]byte("registration-proof-to-find"))
		now := time.Now().UTC().Truncate(time.Microsecond)

		want := models.PasswordRegistrationAttempt{
			TokenHash:             tokenHash[:],
			Email:                 "find-registration-attempt@example.com",
			PasswordHash:          "$argon2id$find-registration-attempt-hash",
			VerificationProofHash: proofHash[:],
			LastCodeSentAt:        now,
			CodeExpiresAt:         now.Add(10 * time.Minute),
			AttemptExpiresAt:      now.Add(30 * time.Minute),
		}

		if err := repository.CreatePasswordRegistrationAttempt(t.Context(), want); err != nil {
			t.Fatalf("CreatePasswordRegistrationAttempt() error = %v", err)
		}

		got, err := repository.FindPasswordRegistrationAttemptByTokenHash(t.Context(), tokenHash[:])
		if err != nil {
			t.Fatalf("FindPasswordRegistrationAttemptByTokenHash() error = %v", err)
		}

		if !bytes.Equal(got.TokenHash, want.TokenHash) {
			t.Errorf("token hash = %x, want %x", got.TokenHash, want.TokenHash)
		}
		if got.Email != want.Email {
			t.Errorf("email = %q, want %q", got.Email, want.Email)
		}
		if got.PasswordHash != want.PasswordHash {
			t.Errorf("password hash = %q, want %q", got.PasswordHash, want.PasswordHash)
		}
		if !bytes.Equal(got.VerificationProofHash, want.VerificationProofHash) {
			t.Errorf("verification proof hash = %x, want %x", got.VerificationProofHash, want.VerificationProofHash)
		}
		if got.FailedAttempts != 0 {
			t.Errorf("failed attempts = %d, want 0", got.FailedAttempts)
		}
		if got.LockedUntil != nil {
			t.Errorf("locked until = %v, want nil", got.LockedUntil)
		}
		if !got.LastCodeSentAt.Equal(want.LastCodeSentAt) {
			t.Errorf("last code sent at = %v, want %v", got.LastCodeSentAt, want.LastCodeSentAt)
		}
		if !got.CodeExpiresAt.Equal(want.CodeExpiresAt) {
			t.Errorf("code expires at = %v, want %v", got.CodeExpiresAt, want.CodeExpiresAt)
		}
		if !got.AttemptExpiresAt.Equal(want.AttemptExpiresAt) {
			t.Errorf("attempt expires at = %v, want %v", got.AttemptExpiresAt, want.AttemptExpiresAt)
		}
		if got.CreatedAt.IsZero() || got.UpdatedAt.IsZero() {
			t.Error("found registration attempt has zero database timestamps")
		}
	})

	t.Run("returns not found for unknown registration token hash", func(t *testing.T) {
		unknownTokenHash := sha256.Sum256([]byte("unknown-registration-token"))

		_, err := repository.FindPasswordRegistrationAttemptByTokenHash(t.Context(), unknownTokenHash[:])
		if !errors.Is(err, repositories.ErrRegistrationAttemptNotFound) {
			t.Fatalf(
				"FindPasswordRegistrationAttemptByTokenHash() error = %v, want %v",
				err,
				repositories.ErrRegistrationAttemptNotFound,
			)
		}
	})

	t.Run("records password registration failures and applies cooldowns", func(t *testing.T) {
		const maxAttempts int16 = 5

		tokenHash := sha256.Sum256([]byte("registration-token-with-failures"))
		proofHash := sha256.Sum256([]byte("registration-proof-with-failures"))
		baseTime := time.Now().UTC().Truncate(time.Microsecond)

		attempt := models.PasswordRegistrationAttempt{
			TokenHash:             tokenHash[:],
			Email:                 "registration-failures@example.com",
			PasswordHash:          "$argon2id$registration-failures-hash",
			VerificationProofHash: proofHash[:],
			LastCodeSentAt:        baseTime,
			CodeExpiresAt:         baseTime.Add(10 * time.Minute),
			AttemptExpiresAt:      baseTime.Add(time.Hour),
		}

		if err := repository.CreatePasswordRegistrationAttempt(t.Context(), attempt); err != nil {
			t.Fatalf("CreatePasswordRegistrationAttempt() error = %v", err)
		}

		recordFailure := func(now time.Time) (int16, *time.Time) {
			t.Helper()

			failedAttempts, lockedUntil, err := repository.RecordPasswordRegistrationFailure(
				t.Context(),
				tokenHash[:],
				now,
				now.Add(30*time.Second),
				now.Add(5*time.Minute),
				maxAttempts,
			)
			if err != nil {
				t.Fatalf("RecordPasswordRegistrationFailure() error = %v", err)
			}

			return failedAttempts, lockedUntil
		}

		assertState := func(wantAttempts int16, wantLockedUntil time.Time, gotAttempts int16, gotLockedUntil *time.Time) {
			t.Helper()

			if gotAttempts != wantAttempts {
				t.Errorf("failed attempts = %d, want %d", gotAttempts, wantAttempts)
			}
			if gotLockedUntil == nil {
				t.Fatal("locked until = nil, want a timestamp")
			}
			if !gotLockedUntil.Equal(wantLockedUntil) {
				t.Errorf("locked until = %v, want %v", gotLockedUntil, wantLockedUntil)
			}
		}

		firstAttemptTime := baseTime
		failedAttempts, lockedUntil := recordFailure(firstAttemptTime)
		firstCooldownUntil := firstAttemptTime.Add(30 * time.Second)
		assertState(1, firstCooldownUntil, failedAttempts, lockedUntil)

		failedAttempts, lockedUntil = recordFailure(firstAttemptTime.Add(10 * time.Second))
		assertState(1, firstCooldownUntil, failedAttempts, lockedUntil)

		for attemptNumber := int16(2); attemptNumber <= 4; attemptNumber++ {
			attemptTime := baseTime.Add(time.Duration(attemptNumber-1) * 31 * time.Second)
			failedAttempts, lockedUntil = recordFailure(attemptTime)
			assertState(attemptNumber, attemptTime.Add(30*time.Second), failedAttempts, lockedUntil)
		}

		fifthAttemptTime := baseTime.Add(4 * 31 * time.Second)
		failedAttempts, lockedUntil = recordFailure(fifthAttemptTime)
		longBlockUntil := fifthAttemptTime.Add(5 * time.Minute)
		assertState(0, longBlockUntil, failedAttempts, lockedUntil)

		failedAttempts, lockedUntil = recordFailure(longBlockUntil.Add(time.Second))
		assertState(1, longBlockUntil.Add(31*time.Second), failedAttempts, lockedUntil)
	})

	t.Run("does not record failure for expired registration attempt", func(t *testing.T) {
		tokenHash := sha256.Sum256([]byte("expired-registration-token"))
		proofHash := sha256.Sum256([]byte("expired-registration-proof"))
		now := time.Now().UTC().Truncate(time.Microsecond)

		attempt := models.PasswordRegistrationAttempt{
			TokenHash:             tokenHash[:],
			Email:                 "expired-registration@example.com",
			PasswordHash:          "$argon2id$expired-registration-hash",
			VerificationProofHash: proofHash[:],
			LastCodeSentAt:        now,
			CodeExpiresAt:         now.Add(30 * time.Second),
			AttemptExpiresAt:      now.Add(time.Minute),
		}

		if err := repository.CreatePasswordRegistrationAttempt(t.Context(), attempt); err != nil {
			t.Fatalf("CreatePasswordRegistrationAttempt() error = %v", err)
		}

		afterExpiration := attempt.AttemptExpiresAt.Add(time.Second)
		_, _, err := repository.RecordPasswordRegistrationFailure(
			t.Context(),
			tokenHash[:],
			afterExpiration,
			afterExpiration.Add(30*time.Second),
			afterExpiration.Add(5*time.Minute),
			5,
		)
		if !errors.Is(err, repositories.ErrRegistrationAttemptNotFound) {
			t.Fatalf("RecordPasswordRegistrationFailure() error = %v, want %v", err, repositories.ErrRegistrationAttemptNotFound)
		}
	})

	t.Run("returns not found when recording failure for unknown token hash", func(t *testing.T) {
		unknownTokenHash := sha256.Sum256([]byte("unknown-registration-failure-token"))
		now := time.Now().UTC()

		_, _, err := repository.RecordPasswordRegistrationFailure(
			t.Context(),
			unknownTokenHash[:],
			now,
			now.Add(30*time.Second),
			now.Add(5*time.Minute),
			5,
		)
		if !errors.Is(err, repositories.ErrRegistrationAttemptNotFound) {
			t.Fatalf("RecordPasswordRegistrationFailure() error = %v, want %v", err, repositories.ErrRegistrationAttemptNotFound)
		}
	})

	t.Run("updates password registration code after resend cooldown", func(t *testing.T) {
		tokenHash := sha256.Sum256([]byte("registration-token-to-resend"))
		originalProofHash := sha256.Sum256([]byte("original-registration-proof"))
		newProofHash := sha256.Sum256([]byte("new-registration-proof"))
		now := time.Now().UTC().Truncate(time.Microsecond)

		attempt := models.PasswordRegistrationAttempt{
			TokenHash:             tokenHash[:],
			Email:                 "resend-registration-code@example.com",
			PasswordHash:          "$argon2id$resend-registration-code-hash",
			VerificationProofHash: originalProofHash[:],
			LastCodeSentAt:        now.Add(-time.Minute),
			CodeExpiresAt:         now.Add(5 * time.Minute),
			AttemptExpiresAt:      now.Add(20 * time.Minute),
		}

		if err := repository.CreatePasswordRegistrationAttempt(t.Context(), attempt); err != nil {
			t.Fatalf("CreatePasswordRegistrationAttempt() error = %v", err)
		}

		updated, err := repository.UpdatePasswordRegistrationCode(
			t.Context(),
			tokenHash[:],
			newProofHash[:],
			now,
			now.Add(30*time.Minute),
			now.Add(-30*time.Second),
		)
		if err != nil {
			t.Fatalf("UpdatePasswordRegistrationCode() error = %v", err)
		}
		if !updated {
			t.Fatal("UpdatePasswordRegistrationCode() updated = false, want true")
		}

		stored, err := repository.FindPasswordRegistrationAttemptByTokenHash(t.Context(), tokenHash[:])
		if err != nil {
			t.Fatalf("FindPasswordRegistrationAttemptByTokenHash() error = %v", err)
		}

		if !bytes.Equal(stored.VerificationProofHash, newProofHash[:]) {
			t.Errorf("verification proof hash = %x, want %x", stored.VerificationProofHash, newProofHash)
		}
		if !stored.LastCodeSentAt.Equal(now) {
			t.Errorf("last code sent at = %v, want %v", stored.LastCodeSentAt, now)
		}
		if !stored.CodeExpiresAt.Equal(attempt.AttemptExpiresAt) {
			t.Errorf("code expires at = %v, want capped at %v", stored.CodeExpiresAt, attempt.AttemptExpiresAt)
		}
	})

	t.Run("does not update password registration code when resend is unavailable", func(t *testing.T) {
		tests := []struct {
			name         string
			tokenSeed    string
			email        string
			prepare      func(models.PasswordRegistrationAttempt)
			operationNow func(models.PasswordRegistrationAttempt) time.Time
		}{
			{
				name:      "cooldown has not elapsed",
				tokenSeed: "registration-resend-cooldown",
				email:     "registration-resend-cooldown@example.com",
				operationNow: func(attempt models.PasswordRegistrationAttempt) time.Time {
					return attempt.LastCodeSentAt.Add(10 * time.Second)
				},
			},
			{
				name:      "attempt is locked",
				tokenSeed: "registration-resend-locked",
				email:     "registration-resend-locked@example.com",
				prepare: func(attempt models.PasswordRegistrationAttempt) {
					lockedUntil := attempt.LastCodeSentAt.Add(10 * time.Minute)
					if _, err := pool.Exec(
						t.Context(),
						"UPDATE password_registration_attempts SET locked_until = $1 WHERE token_hash = $2",
						lockedUntil,
						attempt.TokenHash,
					); err != nil {
						t.Fatalf("lock password registration attempt: %v", err)
					}
				},
				operationNow: func(attempt models.PasswordRegistrationAttempt) time.Time {
					return attempt.LastCodeSentAt.Add(time.Minute)
				},
			},
			{
				name:      "attempt has expired",
				tokenSeed: "registration-resend-expired",
				email:     "registration-resend-expired@example.com",
				operationNow: func(attempt models.PasswordRegistrationAttempt) time.Time {
					return attempt.AttemptExpiresAt.Add(time.Second)
				},
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				tokenHash := sha256.Sum256([]byte(tt.tokenSeed))
				originalProofHash := sha256.Sum256([]byte(tt.tokenSeed + "-original-proof"))
				newProofHash := sha256.Sum256([]byte(tt.tokenSeed + "-new-proof"))
				baseTime := time.Now().UTC().Truncate(time.Microsecond)

				attempt := models.PasswordRegistrationAttempt{
					TokenHash:             tokenHash[:],
					Email:                 tt.email,
					PasswordHash:          "$argon2id$registration-resend-unavailable-hash",
					VerificationProofHash: originalProofHash[:],
					LastCodeSentAt:        baseTime,
					CodeExpiresAt:         baseTime.Add(10 * time.Minute),
					AttemptExpiresAt:      baseTime.Add(30 * time.Minute),
				}

				if err := repository.CreatePasswordRegistrationAttempt(t.Context(), attempt); err != nil {
					t.Fatalf("CreatePasswordRegistrationAttempt() error = %v", err)
				}

				if tt.prepare != nil {
					tt.prepare(attempt)
				}

				now := tt.operationNow(attempt)
				updated, err := repository.UpdatePasswordRegistrationCode(
					t.Context(),
					tokenHash[:],
					newProofHash[:],
					now,
					now.Add(10*time.Minute),
					now.Add(-30*time.Second),
				)
				if err != nil {
					t.Fatalf("UpdatePasswordRegistrationCode() error = %v", err)
				}
				if updated {
					t.Fatal("UpdatePasswordRegistrationCode() updated = true, want false")
				}

				stored, err := repository.FindPasswordRegistrationAttemptByTokenHash(t.Context(), tokenHash[:])
				if err != nil {
					t.Fatalf("FindPasswordRegistrationAttemptByTokenHash() error = %v", err)
				}
				if !bytes.Equal(stored.VerificationProofHash, originalProofHash[:]) {
					t.Errorf("verification proof hash changed to %x, want %x", stored.VerificationProofHash, originalProofHash)
				}
			})
		}
	})

	t.Run("only updates password registration code once during the same cooldown window", func(t *testing.T) {
		tokenHash := sha256.Sum256([]byte("registration-resend-repeated"))
		originalProofHash := sha256.Sum256([]byte("registration-resend-repeated-original-proof"))
		firstProofHash := sha256.Sum256([]byte("registration-resend-repeated-first-proof"))
		secondProofHash := sha256.Sum256([]byte("registration-resend-repeated-second-proof"))
		now := time.Now().UTC().Truncate(time.Microsecond)

		attempt := models.PasswordRegistrationAttempt{
			TokenHash:             tokenHash[:],
			Email:                 "registration-resend-repeated@example.com",
			PasswordHash:          "$argon2id$registration-resend-repeated-hash",
			VerificationProofHash: originalProofHash[:],
			LastCodeSentAt:        now.Add(-time.Minute),
			CodeExpiresAt:         now.Add(10 * time.Minute),
			AttemptExpiresAt:      now.Add(30 * time.Minute),
		}

		if err := repository.CreatePasswordRegistrationAttempt(t.Context(), attempt); err != nil {
			t.Fatalf("CreatePasswordRegistrationAttempt() error = %v", err)
		}

		resendAllowedBefore := now.Add(-30 * time.Second)
		firstUpdated, err := repository.UpdatePasswordRegistrationCode(t.Context(), tokenHash[:], firstProofHash[:], now, now.Add(10*time.Minute), resendAllowedBefore)
		if err != nil {
			t.Fatalf("first UpdatePasswordRegistrationCode() error = %v", err)
		}
		if !firstUpdated {
			t.Fatal("first UpdatePasswordRegistrationCode() updated = false, want true")
		}

		secondUpdated, err := repository.UpdatePasswordRegistrationCode(t.Context(), tokenHash[:], secondProofHash[:], now, now.Add(10*time.Minute), resendAllowedBefore)
		if err != nil {
			t.Fatalf("second UpdatePasswordRegistrationCode() error = %v", err)
		}
		if secondUpdated {
			t.Fatal("second UpdatePasswordRegistrationCode() updated = true, want false")
		}

		stored, err := repository.FindPasswordRegistrationAttemptByTokenHash(t.Context(), tokenHash[:])
		if err != nil {
			t.Fatalf("FindPasswordRegistrationAttemptByTokenHash() error = %v", err)
		}
		if !bytes.Equal(stored.VerificationProofHash, firstProofHash[:]) {
			t.Errorf("verification proof hash = %x, want first proof %x", stored.VerificationProofHash, firstProofHash)
		}
	})

	t.Run("deletes obsolete password registration attempts idempotently", func(t *testing.T) {
		baseTime := time.Now().UTC().Truncate(time.Microsecond)

		createAttempt := func(tokenSeed, email string, codeExpiresAt, attemptExpiresAt time.Time) []byte {
			t.Helper()

			tokenHash := sha256.Sum256([]byte(tokenSeed))
			proofHash := sha256.Sum256([]byte(tokenSeed + "-proof"))
			attempt := models.PasswordRegistrationAttempt{
				TokenHash:             tokenHash[:],
				Email:                 email,
				PasswordHash:          "$argon2id$cleanup-registration-attempt-hash",
				VerificationProofHash: proofHash[:],
				LastCodeSentAt:        baseTime,
				CodeExpiresAt:         codeExpiresAt,
				AttemptExpiresAt:      attemptExpiresAt,
			}

			if err := repository.CreatePasswordRegistrationAttempt(t.Context(), attempt); err != nil {
				t.Fatalf("CreatePasswordRegistrationAttempt() error = %v", err)
			}

			return tokenHash[:]
		}

		expiredTokenHash := createAttempt(
			"cleanup-expired-registration-token",
			"cleanup-expired-registration@example.com",
			baseTime.Add(5*time.Second),
			baseTime.Add(10*time.Second),
		)
		registeredEmailTokenHash := createAttempt(
			"cleanup-registered-email-token",
			"cleanup-registered-email@example.com",
			baseTime.Add(10*time.Minute),
			baseTime.Add(30*time.Minute),
		)
		activeTokenHash := createAttempt(
			"cleanup-active-registration-token",
			"cleanup-active-registration@example.com",
			baseTime.Add(10*time.Minute),
			baseTime.Add(30*time.Minute),
		)

		if _, err := pool.Exec(
			t.Context(),
			"INSERT INTO users(email, email_verified_at) VALUES ($1, $2)",
			"cleanup-registered-email@example.com",
			baseTime,
		); err != nil {
			t.Fatalf("insert user for registration cleanup: %v", err)
		}

		deleted, err := repository.DeleteObsoletePasswordRegistrationAttempts(t.Context(), baseTime.Add(20*time.Second))
		if err != nil {
			t.Fatalf("DeleteObsoletePasswordRegistrationAttempts() error = %v", err)
		}
		if deleted != 2 {
			t.Errorf("DeleteObsoletePasswordRegistrationAttempts() deleted = %d, want 2", deleted)
		}

		for _, tokenHash := range [][]byte{expiredTokenHash, registeredEmailTokenHash} {
			_, err := repository.FindPasswordRegistrationAttemptByTokenHash(t.Context(), tokenHash)
			if !errors.Is(err, repositories.ErrRegistrationAttemptNotFound) {
				t.Errorf("FindPasswordRegistrationAttemptByTokenHash() error = %v, want %v", err, repositories.ErrRegistrationAttemptNotFound)
			}
		}

		if _, err := repository.FindPasswordRegistrationAttemptByTokenHash(t.Context(), activeTokenHash); err != nil {
			t.Errorf("active registration attempt was deleted: %v", err)
		}

		deleted, err = repository.DeleteObsoletePasswordRegistrationAttempts(t.Context(), baseTime.Add(20*time.Second))
		if err != nil {
			t.Fatalf("second DeleteObsoletePasswordRegistrationAttempts() error = %v", err)
		}
		if deleted != 0 {
			t.Errorf("second DeleteObsoletePasswordRegistrationAttempts() deleted = %d, want 0", deleted)
		}
	})

	t.Run("returns false when user email does not exist", func(t *testing.T) {
		exists, err := repository.UserExistsByEmail(t.Context(), "available@example.com")
		if err != nil {
			t.Fatalf("UserExistsByEmail() error = %v", err)
		}

		if exists {
			t.Error("UserExistsByEmail() = true, want false")
		}
	})

	t.Run("returns true when user email exists", func(t *testing.T) {
		const email = "registered@example.com"

		if _, err := pool.Exec(
			t.Context(),
			"INSERT INTO users(email, email_verified_at) VALUES ($1, $2)",
			email,
			time.Now().UTC(),
		); err != nil {
			t.Fatalf("insert existing user: %v", err)
		}

		exists, err := repository.UserExistsByEmail(t.Context(), email)
		if err != nil {
			t.Fatalf("UserExistsByEmail() error = %v", err)
		}

		if !exists {
			t.Error("UserExistsByEmail() = false, want true")
		}
	})

	t.Run("creates user and password credential", func(t *testing.T) {
		const (
			email        = "user@example.com"
			passwordHash = "$argon2id$integration-test-hash"
		)

		user, err := repository.CreateWithPassword(
			t.Context(),
			email,
			passwordHash,
		)
		if err != nil {
			t.Fatalf("CreateWithPassword() error = %v", err)
		}

		if user.ID == uuid.Nil() {
			t.Error("CreateWithPassword() returned a nil user ID")
		}

		if version := user.ID[6] >> 4; version != 7 {
			t.Errorf("CreateWithPassword() UUID version = %d, want 7", version)
		}

		if user.Email != email {
			t.Errorf("CreateWithPassword() email = %q, want %q", user.Email, email)
		}

		if user.CreatedAt.IsZero() {
			t.Error("CreateWithPassword() returned a zero CreatedAt")
		}

		if user.UpdatedAt.IsZero() {
			t.Error("CreateWithPassword() returned a zero UpdatedAt")
		}

		var (
			storedEmail        string
			credentialUserID   uuid.UUID
			storedPasswordHash string
		)

		err = pool.QueryRow(
			t.Context(),
			`
				SELECT u.email, pc.user_id, pc.password_hash
				FROM users AS u
				JOIN password_credentials AS pc ON pc.user_id = u.id
				WHERE u.id = $1
			`,
			user.ID,
		).Scan(
			&storedEmail,
			&credentialUserID,
			&storedPasswordHash,
		)
		if err != nil {
			t.Fatalf("query created user and credential: %v", err)
		}

		if storedEmail != email {
			t.Errorf("stored email = %q, want %q", storedEmail, email)
		}

		if credentialUserID != user.ID {
			t.Errorf(
				"credential user ID = %s, want %s",
				credentialUserID,
				user.ID,
			)
		}

		if storedPasswordHash != passwordHash {
			t.Errorf(
				"stored password hash = %q, want %q",
				storedPasswordHash,
				passwordHash,
			)
		}
	})

	t.Run("finds password credentials by email", func(t *testing.T) {
		const (
			email        = "login@example.com"
			passwordHash = "$argon2id$login-test-hash"
		)

		createdUser, err := repository.CreateWithPassword(
			t.Context(),
			email,
			passwordHash,
		)
		if err != nil {
			t.Fatalf("CreateWithPassword() error = %v", err)
		}

		user, credential, err := repository.FindPasswordCredentialsByEmail(
			t.Context(),
			email,
		)
		if err != nil {
			t.Fatalf("FindPasswordCredentialsByEmail() error = %v", err)
		}

		if user != createdUser {
			t.Errorf(
				"FindPasswordCredentialsByEmail() user = %+v, want %+v",
				user,
				createdUser,
			)
		}

		if credential.UserID != createdUser.ID {
			t.Errorf(
				"credential user ID = %s, want %s",
				credential.UserID,
				createdUser.ID,
			)
		}

		if credential.PasswordHash != passwordHash {
			t.Errorf(
				"credential password hash = %q, want %q",
				credential.PasswordHash,
				passwordHash,
			)
		}

		if credential.CreatedAt.IsZero() {
			t.Error("credential has a zero CreatedAt")
		}

		if credential.UpdatedAt.IsZero() {
			t.Error("credential has a zero UpdatedAt")
		}
	})

	t.Run("creates password reset attempt for password account", func(t *testing.T) {
		const email = "password-reset@example.com"
		user, err := repository.CreateWithPassword(t.Context(), email, "$argon2id$password-reset-hash")
		if err != nil {
			t.Fatalf("CreateWithPassword() error = %v", err)
		}

		const rawToken = "password-reset-raw-token"
		const code = "00123456"
		tokenHash := sha256.Sum256([]byte(rawToken))
		now := time.Now().UTC().Truncate(time.Microsecond)
		attempt := models.PasswordResetAttempt{
			TokenHash:             tokenHash[:],
			VerificationProofHash: verificationcode.Proof(rawToken, code),
			LastCodeSentAt:        now,
			CodeExpiresAt:         now.Add(10 * time.Minute),
			AttemptExpiresAt:      now.Add(30 * time.Minute),
		}

		created, err := repository.CreatePasswordResetAttempt(t.Context(), email, attempt, now.Add(-30*time.Second))
		if err != nil || !created {
			t.Fatalf("CreatePasswordResetAttempt() = (%v, %v), want (true, nil)", created, err)
		}

		stored, err := repository.FindPasswordResetAttemptByTokenHash(t.Context(), tokenHash[:])
		if err != nil {
			t.Fatalf("FindPasswordResetAttemptByTokenHash() error = %v", err)
		}

		if !bytes.Equal(stored.TokenHash, tokenHash[:]) {
			t.Errorf("stored token hash = %x, want %x", stored.TokenHash, tokenHash)
		}
		if bytes.Equal(stored.TokenHash, []byte(rawToken)) {
			t.Error("stored the raw attempt token")
		}
		if !bytes.Equal(stored.VerificationProofHash, attempt.VerificationProofHash) || bytes.Equal(stored.VerificationProofHash, []byte(code)) {
			t.Error("stored proof is incorrect or contains the raw code")
		}
		if stored.UserID != user.ID {
			t.Errorf("stored user ID = %s, want %s", stored.UserID, user.ID)
		}
		if stored.CreatedAt.IsZero() || stored.UpdatedAt.IsZero() {
			t.Error("stored attempt has zero timestamps")
		}
		if !stored.CodeExpiresAt.Equal(attempt.CodeExpiresAt) || !stored.AttemptExpiresAt.Equal(attempt.AttemptExpiresAt) {
			t.Errorf("stored expirations = (%v, %v), want (%v, %v)", stored.CodeExpiresAt, stored.AttemptExpiresAt, attempt.CodeExpiresAt, attempt.AttemptExpiresAt)
		}
		if stored.FailedAttempts != 0 || stored.LockedUntil != nil || stored.UsedAt != nil {
			t.Errorf("new attempt state = (%d, %v, %v), want (0, nil, nil)", stored.FailedAttempts, stored.LockedUntil, stored.UsedAt)
		}
	})

	t.Run("resend replaces code and token without extending active attempt", func(t *testing.T) {
		const email = "replace-password-reset@example.com"
		user, err := repository.CreateWithPassword(t.Context(), email, "$argon2id$replace-password-reset-hash")
		if err != nil {
			t.Fatalf("CreateWithPassword() error = %v", err)
		}

		firstToken, secondToken := "first-password-reset-token", "second-password-reset-token"
		firstHash := sha256.Sum256([]byte(firstToken))
		secondHash := sha256.Sum256([]byte(secondToken))
		now := time.Now().UTC().Add(-time.Minute).Truncate(time.Microsecond)
		firstAttempt := models.PasswordResetAttempt{
			TokenHash:             firstHash[:],
			VerificationProofHash: verificationcode.Proof(firstToken, "12345678"),
			LastCodeSentAt:        now,
			CodeExpiresAt:         now.Add(30 * time.Second),
			AttemptExpiresAt:      now.Add(45 * time.Second),
		}
		if created, err := repository.CreatePasswordResetAttempt(t.Context(), email, firstAttempt, now.Add(-30*time.Second)); err != nil || !created {
			t.Fatalf("first CreatePasswordResetAttempt() = (%v, %v), want (true, nil)", created, err)
		}

		failureAt := now.Add(time.Second)
		failedAttempts, _, err := repository.RecordPasswordResetFailure(t.Context(), firstHash[:], failureAt, failureAt.Add(30*time.Second), failureAt.Add(5*time.Minute), 5)
		if err != nil || failedAttempts != 1 {
			t.Fatalf("RecordPasswordResetFailure() = (%d, %v), want (1, nil)", failedAttempts, err)
		}

		resendAt := now.Add(31 * time.Second)
		secondAttempt := models.PasswordResetAttempt{
			TokenHash:             secondHash[:],
			VerificationProofHash: verificationcode.Proof(secondToken, "87654321"),
			LastCodeSentAt:        resendAt,
			CodeExpiresAt:         resendAt.Add(10 * time.Minute),
			AttemptExpiresAt:      resendAt.Add(30 * time.Minute),
		}
		created, err := repository.CreatePasswordResetAttempt(t.Context(), email, secondAttempt, resendAt.Add(-30*time.Second))
		if err != nil || !created {
			t.Fatalf("second CreatePasswordResetAttempt() = (%v, %v), want (true, nil)", created, err)
		}

		_, err = repository.FindPasswordResetAttemptByTokenHash(t.Context(), firstHash[:])
		if !errors.Is(err, repositories.ErrPasswordResetAttemptNotFound) {
			t.Errorf("old token lookup error = %v, want not found", err)
		}
		stored, err := repository.FindPasswordResetAttemptByTokenHash(t.Context(), secondHash[:])
		if err != nil {
			t.Fatalf("find resent attempt: %v", err)
		}
		if stored.UserID != user.ID || !bytes.Equal(stored.VerificationProofHash, secondAttempt.VerificationProofHash) {
			t.Error("resent attempt has incorrect user or proof")
		}
		if stored.FailedAttempts != 1 {
			t.Errorf("failed attempts after resend = %d, want 1", stored.FailedAttempts)
		}
		if !stored.AttemptExpiresAt.Equal(firstAttempt.AttemptExpiresAt) || !stored.CodeExpiresAt.Equal(firstAttempt.AttemptExpiresAt) {
			t.Errorf("resend expirations = (%v, %v), want attempt deadline %v", stored.CodeExpiresAt, stored.AttemptExpiresAt, firstAttempt.AttemptExpiresAt)
		}
		var count int
		if err := pool.QueryRow(t.Context(), "SELECT COUNT(*) FROM password_reset_attempts WHERE user_id = $1", user.ID).Scan(&count); err != nil {
			t.Fatalf("count reset attempts: %v", err)
		}
		if count != 1 {
			t.Errorf("reset attempt count = %d, want 1", count)
		}
	})

	t.Run("does not create attempt for unknown email or Google-only account", func(t *testing.T) {
		const googleEmail = "google-only-password-reset@example.com"
		if _, err := repository.CreateWithIdentity(t.Context(), googleEmail, "google", "google-only-password-reset-subject"); err != nil {
			t.Fatalf("CreateWithIdentity() error = %v", err)
		}

		for _, email := range []string{"unknown-password-reset@example.com", googleEmail} {
			tokenHash := sha256.Sum256([]byte(email))
			now := time.Now().UTC()
			attempt := models.PasswordResetAttempt{
				TokenHash: tokenHash[:], VerificationProofHash: verificationcode.Proof(email, "12345678"),
				LastCodeSentAt: now, CodeExpiresAt: now.Add(10 * time.Minute), AttemptExpiresAt: now.Add(30 * time.Minute),
			}
			created, err := repository.CreatePasswordResetAttempt(t.Context(), email, attempt, now.Add(-30*time.Second))
			if err != nil || created {
				t.Errorf("CreatePasswordResetAttempt(%q) = (%v, %v), want (false, nil)", email, created, err)
			}
		}

		var googleOnlyAttemptExists bool
		if err := pool.QueryRow(t.Context(), `SELECT EXISTS (
			SELECT 1 FROM password_reset_attempts AS a
			JOIN users AS u ON u.id = a.user_id
			WHERE u.email = $1
		)`, googleEmail).Scan(&googleOnlyAttemptExists); err != nil {
			t.Fatalf("check Google-only reset attempt: %v", err)
		}
		if googleOnlyAttemptExists {
			t.Error("reset attempt was created for Google-only account")
		}
	})

	t.Run("suppresses resend during cooldown without replacing code", func(t *testing.T) {
		const email = "cooldown-password-reset@example.com"
		if _, err := repository.CreateWithPassword(t.Context(), email, "$argon2id$cooldown-hash"); err != nil {
			t.Fatalf("CreateWithPassword() error = %v", err)
		}

		now := time.Now().UTC().Truncate(time.Microsecond)
		firstHash := sha256.Sum256([]byte("cooldown-first-token"))
		secondHash := sha256.Sum256([]byte("cooldown-second-token"))
		first := models.PasswordResetAttempt{
			TokenHash: firstHash[:], VerificationProofHash: verificationcode.Proof("cooldown-first-token", "12345678"),
			LastCodeSentAt: now, CodeExpiresAt: now.Add(10 * time.Minute), AttemptExpiresAt: now.Add(30 * time.Minute),
		}
		if created, err := repository.CreatePasswordResetAttempt(t.Context(), email, first, now.Add(-30*time.Second)); err != nil || !created {
			t.Fatalf("first CreatePasswordResetAttempt() = (%v, %v), want (true, nil)", created, err)
		}

		resendAt := now.Add(10 * time.Second)
		second := models.PasswordResetAttempt{
			TokenHash: secondHash[:], VerificationProofHash: verificationcode.Proof("cooldown-second-token", "87654321"),
			LastCodeSentAt: resendAt, CodeExpiresAt: resendAt.Add(10 * time.Minute), AttemptExpiresAt: resendAt.Add(30 * time.Minute),
		}
		if created, err := repository.CreatePasswordResetAttempt(t.Context(), email, second, resendAt.Add(-30*time.Second)); err != nil || created {
			t.Fatalf("early resend = (%v, %v), want (false, nil)", created, err)
		}
		stored, err := repository.FindPasswordResetAttemptByTokenHash(t.Context(), firstHash[:])
		if err != nil || !bytes.Equal(stored.VerificationProofHash, first.VerificationProofHash) {
			t.Fatalf("first code changed during cooldown: attempt=%+v, error=%v", stored, err)
		}
		if _, err := repository.FindPasswordResetAttemptByTokenHash(t.Context(), secondHash[:]); !errors.Is(err, repositories.ErrPasswordResetAttemptNotFound) {
			t.Fatalf("second token lookup error = %v, want not found", err)
		}
	})

	t.Run("fifth wrong code locks attempt and blocks resend", func(t *testing.T) {
		const email = "locked-password-reset@example.com"
		if _, err := repository.CreateWithPassword(t.Context(), email, "$argon2id$locked-hash"); err != nil {
			t.Fatalf("CreateWithPassword() error = %v", err)
		}

		now := time.Now().UTC().Truncate(time.Microsecond)
		tokenHash := sha256.Sum256([]byte("locked-reset-token"))
		attempt := models.PasswordResetAttempt{
			TokenHash: tokenHash[:], VerificationProofHash: verificationcode.Proof("locked-reset-token", "12345678"),
			LastCodeSentAt: now, CodeExpiresAt: now.Add(10 * time.Minute), AttemptExpiresAt: now.Add(30 * time.Minute),
		}
		if created, err := repository.CreatePasswordResetAttempt(t.Context(), email, attempt, now.Add(-30*time.Second)); err != nil || !created {
			t.Fatalf("CreatePasswordResetAttempt() = (%v, %v), want (true, nil)", created, err)
		}

		for i := 1; i <= 5; i++ {
			failureAt := now.Add(time.Duration(i) * 31 * time.Second)
			failed, lockedUntil, err := repository.RecordPasswordResetFailure(t.Context(), tokenHash[:], failureAt, failureAt.Add(30*time.Second), failureAt.Add(5*time.Minute), 5)
			if err != nil || lockedUntil == nil {
				t.Fatalf("failure %d = (%d, %v, %v)", i, failed, lockedUntil, err)
			}
			wantFailed := int16(i)
			wantLock := failureAt.Add(30 * time.Second)
			if i == 5 {
				wantFailed = 0
				wantLock = failureAt.Add(5 * time.Minute)
			}
			if failed != wantFailed || !lockedUntil.Equal(wantLock) {
				t.Errorf("failure %d = (%d, %v), want (%d, %v)", i, failed, *lockedUntil, wantFailed, wantLock)
			}
		}

		resendAt := now.Add(6 * time.Minute)
		newHash := sha256.Sum256([]byte("locked-reset-new-token"))
		resend := models.PasswordResetAttempt{
			TokenHash: newHash[:], VerificationProofHash: verificationcode.Proof("locked-reset-new-token", "87654321"),
			LastCodeSentAt: resendAt, CodeExpiresAt: resendAt.Add(10 * time.Minute), AttemptExpiresAt: resendAt.Add(30 * time.Minute),
		}
		if created, err := repository.CreatePasswordResetAttempt(t.Context(), email, resend, resendAt.Add(-30*time.Second)); err != nil || created {
			t.Fatalf("resend during lock = (%v, %v), want (false, nil)", created, err)
		}
	})

	t.Run("rejects unknown reset token", func(t *testing.T) {
		tokenHash := sha256.Sum256([]byte("missing-reset-attempt"))
		_, err := repository.FindPasswordResetAttemptByTokenHash(t.Context(), tokenHash[:])
		if !errors.Is(err, repositories.ErrPasswordResetAttemptNotFound) {
			t.Fatalf("FindPasswordResetAttemptByTokenHash() error = %v, want not found", err)
		}
	})

	t.Run("finds user by ID", func(t *testing.T) {
		createdUser, err := repository.CreateWithPassword(t.Context(), "find-by-id@example.com", "$argon2id$find-by-id-test-hash")
		if err != nil {
			t.Fatalf("CreateWithPassword() error = %v", err)
		}

		user, err := repository.FindByID(t.Context(), createdUser.ID)
		if err != nil {
			t.Fatalf("FindByID() error = %v", err)
		}

		if user != createdUser {
			t.Errorf("FindByID() user = %+v, want %+v", user, createdUser)
		}
	})

	t.Run("returns not found for unknown user ID", func(t *testing.T) {
		unknownUserID := uuid.MustParse("01991f29-7c22-7ab3-a395-4d402f09c399")

		_, err := repository.FindByID(t.Context(), unknownUserID)
		if !errors.Is(err, repositories.ErrUserNotFound) {
			t.Fatalf("FindByID() error = %v, want %v", err, repositories.ErrUserNotFound)
		}
	})

	t.Run("returns not found for unknown email", func(t *testing.T) {
		_, _, err := repository.FindPasswordCredentialsByEmail(
			t.Context(),
			"missing@example.com",
		)
		if !errors.Is(err, repositories.ErrPasswordCredentialNotFound) {
			t.Fatalf(
				"FindPasswordCredentialsByEmail() error = %v, want %v",
				err,
				repositories.ErrPasswordCredentialNotFound,
			)
		}
	})

	t.Run("returns not found for user without password", func(t *testing.T) {
		const email = "google-only@example.com"

		if _, err := pool.Exec(
			t.Context(),
			"INSERT INTO users(email, email_verified_at) VALUES ($1, NOW())",
			email,
		); err != nil {
			t.Fatalf("insert user without password: %v", err)
		}

		_, _, err := repository.FindPasswordCredentialsByEmail(
			t.Context(),
			email,
		)
		if !errors.Is(err, repositories.ErrPasswordCredentialNotFound) {
			t.Fatalf(
				"FindPasswordCredentialsByEmail() error = %v, want %v",
				err,
				repositories.ErrPasswordCredentialNotFound,
			)
		}
	})

	t.Run("recognizes duplicate email", func(t *testing.T) {
		const email = "duplicate@example.com"

		if _, err := repository.CreateWithPassword(
			t.Context(),
			email,
			"$argon2id$first-hash",
		); err != nil {
			t.Fatalf("first CreateWithPassword() error = %v", err)
		}

		_, err := repository.CreateWithPassword(
			t.Context(),
			email,
			"$argon2id$second-hash",
		)
		if !errors.Is(err, repositories.ErrEmailAlreadyExists) {
			t.Fatalf(
				"second CreateWithPassword() error = %v, want %v",
				err,
				repositories.ErrEmailAlreadyExists,
			)
		}

		var usersWithEmail int
		if err := pool.QueryRow(
			t.Context(),
			"SELECT COUNT(*) FROM users WHERE email = $1",
			email,
		).Scan(&usersWithEmail); err != nil {
			t.Fatalf("count users with duplicate email: %v", err)
		}

		if usersWithEmail != 1 {
			t.Errorf("users with duplicate email = %d, want 1", usersWithEmail)
		}
	})

	t.Run("rolls back user when credential insert fails", func(t *testing.T) {
		const email = "rollback@example.com"

		_, err := repository.CreateWithPassword(
			t.Context(),
			email,
			" ",
		)
		if err == nil {
			t.Fatal("CreateWithPassword() error = nil, want credential constraint error")
		}

		var usersWithEmail int
		if err := pool.QueryRow(
			t.Context(),
			"SELECT COUNT(*) FROM users WHERE email = $1",
			email,
		).Scan(&usersWithEmail); err != nil {
			t.Fatalf("count users after credential failure: %v", err)
		}

		if usersWithEmail != 0 {
			t.Errorf("users left after credential failure = %d, want 0", usersWithEmail)
		}
	})

	t.Run("creates user and auth identity", func(t *testing.T) {
		const (
			email          = "identity@example.com"
			provider       = "google"
			providerUserID = "google-subject-123"
		)

		user, err := repository.CreateWithIdentity(t.Context(), email, provider, providerUserID)
		if err != nil {
			t.Fatalf("CreateWithIdentity() error = %v", err)
		}

		if user.ID == uuid.Nil() {
			t.Error("CreateWithIdentity() returned a nil user ID")
		}

		if version := user.ID[6] >> 4; version != 7 {
			t.Errorf("CreateWithIdentity() UUID version = %d, want 7", version)
		}

		if user.Email != email {
			t.Errorf("CreateWithIdentity() email = %q, want %q", user.Email, email)
		}

		if user.CreatedAt.IsZero() || user.UpdatedAt.IsZero() {
			t.Error("CreateWithIdentity() returned zero timestamps")
		}

		var (
			storedUserID         uuid.UUID
			storedProvider       string
			storedProviderUserID string
			identityCreatedAt    time.Time
		)

		err = pool.QueryRow(
			t.Context(),
			`
				SELECT user_id, provider, provider_user_id, created_at
				FROM auth_identities
				WHERE provider = $1 AND provider_user_id = $2
			`,
			provider,
			providerUserID,
		).Scan(&storedUserID, &storedProvider, &storedProviderUserID, &identityCreatedAt)
		if err != nil {
			t.Fatalf("query created auth identity: %v", err)
		}

		if storedUserID != user.ID {
			t.Errorf("identity user ID = %s, want %s", storedUserID, user.ID)
		}

		if storedProvider != provider {
			t.Errorf("identity provider = %q, want %q", storedProvider, provider)
		}

		if storedProviderUserID != providerUserID {
			t.Errorf("identity provider user ID = %q, want %q", storedProviderUserID, providerUserID)
		}

		if identityCreatedAt.IsZero() {
			t.Error("identity has a zero CreatedAt")
		}

		var hasPasswordCredential bool
		if err := pool.QueryRow(t.Context(), "SELECT EXISTS(SELECT 1 FROM password_credentials WHERE user_id = $1)", user.ID).Scan(&hasPasswordCredential); err != nil {
			t.Fatalf("check password credential: %v", err)
		}

		if hasPasswordCredential {
			t.Error("CreateWithIdentity() created an unexpected password credential")
		}
	})

	t.Run("links identity to existing password user", func(t *testing.T) {
		const (
			email          = "link-identity-password-user@example.com"
			provider       = "google"
			providerUserID = "link-identity-google-subject"
			passwordHash   = "$argon2id$link-identity-password-hash"
		)

		passwordUser, err := repository.CreateWithPassword(t.Context(), email, passwordHash)
		if err != nil {
			t.Fatalf("CreateWithPassword() error = %v", err)
		}

		linkedUser, err := repository.LinkIdentityToPasswordUserByEmail(t.Context(), email, provider, providerUserID)
		if err != nil {
			t.Fatalf("LinkIdentityToPasswordUserByEmail() error = %v", err)
		}

		if linkedUser != passwordUser {
			t.Errorf("LinkIdentityToPasswordUserByEmail() user = %+v, want %+v", linkedUser, passwordUser)
		}

		var (
			usersWithEmail       int
			storedIdentityUserID uuid.UUID
			storedPasswordHash   string
		)

		if err := pool.QueryRow(t.Context(), "SELECT COUNT(*) FROM users WHERE email = $1", email).Scan(&usersWithEmail); err != nil {
			t.Fatalf("count users after identity linking: %v", err)
		}
		if usersWithEmail != 1 {
			t.Errorf("users with linked email = %d, want 1", usersWithEmail)
		}

		if err := pool.QueryRow(
			t.Context(),
			"SELECT user_id FROM auth_identities WHERE provider = $1 AND provider_user_id = $2",
			provider,
			providerUserID,
		).Scan(&storedIdentityUserID); err != nil {
			t.Fatalf("query linked identity: %v", err)
		}
		if storedIdentityUserID != passwordUser.ID {
			t.Errorf("linked identity user ID = %s, want %s", storedIdentityUserID, passwordUser.ID)
		}

		if err := pool.QueryRow(
			t.Context(),
			"SELECT password_hash FROM password_credentials WHERE user_id = $1",
			passwordUser.ID,
		).Scan(&storedPasswordHash); err != nil {
			t.Fatalf("query password credential after identity linking: %v", err)
		}
		if storedPasswordHash != passwordHash {
			t.Errorf("password hash after identity linking = %q, want %q", storedPasswordHash, passwordHash)
		}
	})

	t.Run("returns password account not found for unknown email", func(t *testing.T) {
		const (
			email          = "unknown-password-account@example.com"
			provider       = "google"
			providerUserID = "unknown-password-account-google-subject"
		)

		_, err := repository.LinkIdentityToPasswordUserByEmail(t.Context(), email, provider, providerUserID)
		if !errors.Is(err, repositories.ErrPasswordAccountNotFound) {
			t.Fatalf("LinkIdentityToPasswordUserByEmail() error = %v, want %v", err, repositories.ErrPasswordAccountNotFound)
		}

		var identityExists bool
		if err := pool.QueryRow(
			t.Context(),
			"SELECT EXISTS(SELECT 1 FROM auth_identities WHERE provider = $1 AND provider_user_id = $2)",
			provider,
			providerUserID,
		).Scan(&identityExists); err != nil {
			t.Fatalf("check identity for unknown password account: %v", err)
		}
		if identityExists {
			t.Error("identity was created for an unknown password account")
		}
	})

	t.Run("does not link identity to Google-only user", func(t *testing.T) {
		const (
			email                  = "google-only-linking@example.com"
			provider               = "google"
			existingProviderUserID = "existing-google-only-subject"
			newProviderUserID      = "new-google-only-subject"
		)

		googleUser, err := repository.CreateWithIdentity(t.Context(), email, provider, existingProviderUserID)
		if err != nil {
			t.Fatalf("CreateWithIdentity() error = %v", err)
		}

		_, err = repository.LinkIdentityToPasswordUserByEmail(t.Context(), email, provider, newProviderUserID)
		if !errors.Is(err, repositories.ErrPasswordAccountNotFound) {
			t.Fatalf("LinkIdentityToPasswordUserByEmail() error = %v, want %v", err, repositories.ErrPasswordAccountNotFound)
		}

		var identities int
		if err := pool.QueryRow(t.Context(), "SELECT COUNT(*) FROM auth_identities WHERE user_id = $1", googleUser.ID).Scan(&identities); err != nil {
			t.Fatalf("count identities for Google-only user: %v", err)
		}
		if identities != 1 {
			t.Errorf("Google-only user identities = %d, want 1", identities)
		}
	})

	t.Run("recognizes identity that is already linked", func(t *testing.T) {
		const (
			email          = "identity-already-linked@example.com"
			provider       = "google"
			providerUserID = "already-linked-google-subject"
		)

		user, err := repository.CreateWithPassword(t.Context(), email, "$argon2id$already-linked-password-hash")
		if err != nil {
			t.Fatalf("CreateWithPassword() error = %v", err)
		}

		if _, err := repository.LinkIdentityToPasswordUserByEmail(t.Context(), email, provider, providerUserID); err != nil {
			t.Fatalf("first LinkIdentityToPasswordUserByEmail() error = %v", err)
		}

		_, err = repository.LinkIdentityToPasswordUserByEmail(t.Context(), email, provider, providerUserID)
		if !errors.Is(err, repositories.ErrAuthIdentityAlreadyExists) {
			t.Fatalf("second LinkIdentityToPasswordUserByEmail() error = %v, want %v", err, repositories.ErrAuthIdentityAlreadyExists)
		}

		var identities int
		if err := pool.QueryRow(t.Context(), "SELECT COUNT(*) FROM auth_identities WHERE user_id = $1", user.ID).Scan(&identities); err != nil {
			t.Fatalf("count identities after repeated linking: %v", err)
		}
		if identities != 1 {
			t.Errorf("identities after repeated linking = %d, want 1", identities)
		}
	})

	t.Run("does not replace provider identity already linked to user", func(t *testing.T) {
		const (
			email                  = "provider-already-linked@example.com"
			provider               = "google"
			originalProviderUserID = "original-linked-google-subject"
			newProviderUserID      = "replacement-google-subject"
		)

		user, err := repository.CreateWithPassword(t.Context(), email, "$argon2id$provider-linked-password-hash")
		if err != nil {
			t.Fatalf("CreateWithPassword() error = %v", err)
		}

		if _, err := repository.LinkIdentityToPasswordUserByEmail(t.Context(), email, provider, originalProviderUserID); err != nil {
			t.Fatalf("first LinkIdentityToPasswordUserByEmail() error = %v", err)
		}

		_, err = repository.LinkIdentityToPasswordUserByEmail(t.Context(), email, provider, newProviderUserID)
		if !errors.Is(err, repositories.ErrProviderAlreadyLinked) {
			t.Fatalf("LinkIdentityToPasswordUserByEmail() error = %v, want %v", err, repositories.ErrProviderAlreadyLinked)
		}

		var storedProviderUserID string
		if err := pool.QueryRow(
			t.Context(),
			"SELECT provider_user_id FROM auth_identities WHERE user_id = $1 AND provider = $2",
			user.ID,
			provider,
		).Scan(&storedProviderUserID); err != nil {
			t.Fatalf("query identity after rejected replacement: %v", err)
		}
		if storedProviderUserID != originalProviderUserID {
			t.Errorf("stored provider user ID = %q, want %q", storedProviderUserID, originalProviderUserID)
		}
	})

	t.Run("finds user by provider identity", func(t *testing.T) {
		const (
			email          = "find-identity@example.com"
			provider       = "google"
			providerUserID = "google-subject-to-find"
		)

		createdUser, err := repository.CreateWithIdentity(t.Context(), email, provider, providerUserID)
		if err != nil {
			t.Fatalf("CreateWithIdentity() error = %v", err)
		}

		user, err := repository.FindUserByProviderIdentity(t.Context(), provider, providerUserID)
		if err != nil {
			t.Fatalf("FindUserByProviderIdentity() error = %v", err)
		}

		if user != createdUser {
			t.Errorf("FindUserByProviderIdentity() user = %+v, want %+v", user, createdUser)
		}
	})

	t.Run("returns not found for unknown provider identity", func(t *testing.T) {
		_, err := repository.FindUserByProviderIdentity(t.Context(), "google", "unknown-google-subject")
		if !errors.Is(err, repositories.ErrAuthIdentityNotFound) {
			t.Fatalf("FindUserByProviderIdentity() error = %v, want %v", err, repositories.ErrAuthIdentityNotFound)
		}
	})

	t.Run("recognizes duplicate email when creating identity", func(t *testing.T) {
		const (
			email          = "identity-duplicate@example.com"
			provider       = "google"
			providerUserID = "duplicate-email-google-subject"
		)

		if _, err := repository.CreateWithPassword(t.Context(), email, "$argon2id$existing-account-hash"); err != nil {
			t.Fatalf("CreateWithPassword() error = %v", err)
		}

		_, err := repository.CreateWithIdentity(t.Context(), email, provider, providerUserID)
		if !errors.Is(err, repositories.ErrEmailAlreadyExists) {
			t.Fatalf("CreateWithIdentity() error = %v, want %v", err, repositories.ErrEmailAlreadyExists)
		}

		var identityExists bool
		if err := pool.QueryRow(
			t.Context(),
			"SELECT EXISTS(SELECT 1 FROM auth_identities WHERE provider = $1 AND provider_user_id = $2)",
			provider,
			providerUserID,
		).Scan(&identityExists); err != nil {
			t.Fatalf("check auth identity after duplicate email: %v", err)
		}

		if identityExists {
			t.Error("auth identity was created for an existing email")
		}
	})

	t.Run("rolls back user when identity insert fails", func(t *testing.T) {
		const email = "identity-rollback@example.com"

		_, err := repository.CreateWithIdentity(t.Context(), email, "", "google-subject-with-invalid-provider")
		if err == nil {
			t.Fatal("CreateWithIdentity() error = nil, want identity constraint error")
		}

		var usersWithEmail int
		if err := pool.QueryRow(t.Context(), "SELECT COUNT(*) FROM users WHERE email = $1", email).Scan(&usersWithEmail); err != nil {
			t.Fatalf("count users after identity failure: %v", err)
		}

		if usersWithEmail != 0 {
			t.Errorf("users left after identity failure = %d, want 0", usersWithEmail)
		}
	})
}
