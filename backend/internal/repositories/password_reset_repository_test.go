//go:build integration

package repositories_test

import (
	"bytes"
	"crypto/sha256"
	"errors"
	"github.com/fredsaggio/url-shortener/internal/models"
	"github.com/fredsaggio/url-shortener/internal/repositories"
	"github.com/fredsaggio/url-shortener/internal/verificationcode"
	"github.com/jackc/pgx/v5/pgxpool"
	"testing"
	"time"
)

func runPasswordResetRepositoryTests(t *testing.T, pool *pgxpool.Pool, repository *repositories.UserRepository) {
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

	t.Run("completes password reset and revokes sessions atomically", func(t *testing.T) {
		const email = "complete-reset@example.com"
		user, err := repository.CreateWithPassword(t.Context(), email, "old-password-hash")
		if err != nil {
			t.Fatalf("CreateWithPassword() error = %v", err)
		}
		const token, code = "complete-reset-token", "00123456"
		tokenHash := sha256.Sum256([]byte(token))
		proofHash := verificationcode.Proof(token, code)
		now := time.Now().UTC().Truncate(time.Microsecond)
		attempt := models.PasswordResetAttempt{
			TokenHash: tokenHash[:], VerificationProofHash: proofHash,
			LastCodeSentAt: now, CodeExpiresAt: now.Add(10 * time.Minute), AttemptExpiresAt: now.Add(30 * time.Minute),
		}
		if created, err := repository.CreatePasswordResetAttempt(t.Context(), email, attempt, now.Add(-30*time.Second)); err != nil || !created {
			t.Fatalf("CreatePasswordResetAttempt() = (%v, %v)", created, err)
		}
		for _, value := range []string{"first-existing-session", "second-existing-session"} {
			sessionHash := sha256.Sum256([]byte(value))
			if _, err := pool.Exec(t.Context(), `INSERT INTO user_sessions(token_hash, user_id, expires_at) VALUES($1, $2, $3)`, sessionHash[:], user.ID, now.Add(time.Hour)); err != nil {
				t.Fatalf("insert existing session: %v", err)
			}
		}

		wrongProof := verificationcode.Proof(token, "87654321")
		if _, err := repository.CompletePasswordReset(t.Context(), tokenHash[:], wrongProof, "new-password-hash"); !errors.Is(err, repositories.ErrPasswordResetAttemptNotFound) {
			t.Fatalf("CompletePasswordReset(wrong proof) error = %v, want not found", err)
		}
		confirmedEmail, err := repository.CompletePasswordReset(t.Context(), tokenHash[:], proofHash, "new-password-hash")
		if err != nil {
			t.Fatalf("CompletePasswordReset() error = %v", err)
		}
		if confirmedEmail != email {
			t.Errorf("confirmed email = %q, want %q", confirmedEmail, email)
		}

		var passwordHash string
		if err := pool.QueryRow(t.Context(), `SELECT password_hash FROM password_credentials WHERE user_id = $1`, user.ID).Scan(&passwordHash); err != nil {
			t.Fatalf("read password credential: %v", err)
		}
		if passwordHash != "new-password-hash" {
			t.Errorf("password hash = %q, want new hash", passwordHash)
		}
		var sessionCount int
		if err := pool.QueryRow(t.Context(), `SELECT COUNT(*) FROM user_sessions WHERE user_id = $1`, user.ID).Scan(&sessionCount); err != nil {
			t.Fatalf("count sessions: %v", err)
		}
		if sessionCount != 0 {
			t.Errorf("session count = %d, want zero", sessionCount)
		}
		stored, err := repository.FindPasswordResetAttemptByTokenHash(t.Context(), tokenHash[:])
		if err != nil || stored.UsedAt == nil {
			t.Fatalf("used attempt = (%+v, %v), want UsedAt set", stored, err)
		}
		if _, err := repository.CompletePasswordReset(t.Context(), tokenHash[:], proofHash, "another-password-hash"); !errors.Is(err, repositories.ErrPasswordResetAttemptNotFound) {
			t.Errorf("reused code error = %v, want not found", err)
		}
	})

	t.Run("expired or locked password reset cannot change password", func(t *testing.T) {
		const email = "unavailable-reset@example.com"
		user, err := repository.CreateWithPassword(t.Context(), email, "old-password-hash")
		if err != nil {
			t.Fatalf("CreateWithPassword() error = %v", err)
		}
		const token, code = "unavailable-reset-token", "12345678"
		tokenHash := sha256.Sum256([]byte(token))
		proofHash := verificationcode.Proof(token, code)
		now := time.Now().UTC().Truncate(time.Microsecond)
		attempt := models.PasswordResetAttempt{
			TokenHash: tokenHash[:], VerificationProofHash: proofHash,
			LastCodeSentAt: now, CodeExpiresAt: now.Add(10 * time.Minute), AttemptExpiresAt: now.Add(30 * time.Minute),
		}
		if created, err := repository.CreatePasswordResetAttempt(t.Context(), email, attempt, now.Add(-30*time.Second)); err != nil || !created {
			t.Fatalf("CreatePasswordResetAttempt() = (%v, %v)", created, err)
		}
		if _, err := pool.Exec(t.Context(), `UPDATE password_reset_attempts SET locked_until = $1 WHERE token_hash = $2`, now.Add(5*time.Minute), tokenHash[:]); err != nil {
			t.Fatalf("lock attempt: %v", err)
		}
		if _, err := repository.CompletePasswordReset(t.Context(), tokenHash[:], proofHash, "new-password-hash"); !errors.Is(err, repositories.ErrPasswordResetAttemptNotFound) {
			t.Errorf("locked attempt error = %v, want not found", err)
		}
		if _, err := pool.Exec(t.Context(), `UPDATE password_reset_attempts SET locked_until = NULL, created_at = $1, code_expires_at = $2 WHERE token_hash = $3`, now.Add(-20*time.Minute), now.Add(-time.Second), tokenHash[:]); err != nil {
			t.Fatalf("expire code: %v", err)
		}
		if _, err := repository.CompletePasswordReset(t.Context(), tokenHash[:], proofHash, "new-password-hash"); !errors.Is(err, repositories.ErrPasswordResetAttemptNotFound) {
			t.Errorf("expired code error = %v, want not found", err)
		}
		var storedHash string
		if err := pool.QueryRow(t.Context(), `SELECT password_hash FROM password_credentials WHERE user_id = $1`, user.ID).Scan(&storedHash); err != nil {
			t.Fatalf("read credential: %v", err)
		}
		if storedHash != "old-password-hash" {
			t.Errorf("password was changed despite lock or expiry: %q", storedHash)
		}
	})
}
