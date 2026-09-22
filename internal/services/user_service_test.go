package services_test

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"
	"time"
	"uuid"

	"github.com/fredsaggio/url-shortener/internal/models"
	"github.com/fredsaggio/url-shortener/internal/repositories"
	"github.com/fredsaggio/url-shortener/internal/services"
	"github.com/fredsaggio/url-shortener/internal/sessiontoken"
	"github.com/fredsaggio/url-shortener/internal/verificationcode"
)

const (
	testCodeTTL    = 10 * time.Minute
	testAttemptTTL = 30 * time.Minute
)

type userRepositoryStub struct {
	userExistsByEmailFunc                          func(context.Context, string) (bool, error)
	createPasswordRegistrationAttemptFunc          func(context.Context, models.PasswordRegistrationAttempt) error
	findPasswordRegistrationAttemptByTokenHashFunc func(context.Context, []byte) (models.PasswordRegistrationAttempt, error)
	recordPasswordRegistrationFailureFunc          func(context.Context, []byte, time.Time, time.Time, time.Time, int16) (int16, *time.Time, error)
	createWithPasswordFunc                         func(context.Context, string, string) (models.User, error)
	findByIDFunc                                   func(context.Context, uuid.UUID) (models.User, error)
}

func (s userRepositoryStub) UserExistsByEmail(ctx context.Context, email string) (bool, error) {
	return s.userExistsByEmailFunc(ctx, email)
}

func (s userRepositoryStub) CreatePasswordRegistrationAttempt(ctx context.Context, attempt models.PasswordRegistrationAttempt) error {
	return s.createPasswordRegistrationAttemptFunc(ctx, attempt)
}

func (s userRepositoryStub) FindPasswordRegistrationAttemptByTokenHash(ctx context.Context, tokenHash []byte) (models.PasswordRegistrationAttempt, error) {
	return s.findPasswordRegistrationAttemptByTokenHashFunc(ctx, tokenHash)
}

func (s userRepositoryStub) RecordPasswordRegistrationFailure(ctx context.Context, tokenHash []byte, now, retryAt, lockUntil time.Time, maxAttempts int16) (int16, *time.Time, error) {
	return s.recordPasswordRegistrationFailureFunc(ctx, tokenHash, now, retryAt, lockUntil, maxAttempts)
}

func (s userRepositoryStub) CreateWithPassword(ctx context.Context, email, passwordHash string) (models.User, error) {
	return s.createWithPasswordFunc(ctx, email, passwordHash)
}

func (s userRepositoryStub) FindByID(ctx context.Context, userID uuid.UUID) (models.User, error) {
	return s.findByIDFunc(ctx, userID)
}

type passwordHasherStub struct {
	hashFunc func(string) (string, error)
}

func (s passwordHasherStub) Hash(password string) (string, error) {
	return s.hashFunc(password)
}

type passwordRegistrationCodeSenderStub struct {
	sendFunc func(context.Context, string, string) error
}

func (s passwordRegistrationCodeSenderStub) SendPasswordRegistrationCode(ctx context.Context, email, code string) error {
	return s.sendFunc(ctx, email, code)
}

func TestUserServiceStartPasswordRegistration(t *testing.T) {
	const (
		inputEmail      = "  USER@Example.COM  "
		normalizedEmail = "user@example.com"
		password        = "senha-segura"
		passwordHash    = "$argon2id$test-hash"
		rawToken        = "registration-token"
		code            = "123456"
		contextValue    = "request-context"
	)
	tokenHash := bytes.Repeat([]byte{0x42}, 32)

	type contextKey struct{}
	ctx := context.WithValue(context.Background(), contextKey{}, contextValue)

	var storedAttempt models.PasswordRegistrationAttempt
	repository := userRepositoryStub{
		userExistsByEmailFunc: func(gotCtx context.Context, gotEmail string) (bool, error) {
			if got := gotCtx.Value(contextKey{}); got != contextValue {
				t.Errorf("UserExistsByEmail() context value = %v, want %q", got, contextValue)
			}
			if gotEmail != normalizedEmail {
				t.Errorf("UserExistsByEmail() email = %q, want %q", gotEmail, normalizedEmail)
			}
			return false, nil
		},
		createPasswordRegistrationAttemptFunc: func(gotCtx context.Context, attempt models.PasswordRegistrationAttempt) error {
			if got := gotCtx.Value(contextKey{}); got != contextValue {
				t.Errorf("CreatePasswordRegistrationAttempt() context value = %v, want %q", got, contextValue)
			}
			storedAttempt = attempt
			return nil
		},
	}
	hasher := passwordHasherStub{hashFunc: func(gotPassword string) (string, error) {
		if gotPassword != password {
			t.Errorf("Hash() password = %q, want %q", gotPassword, password)
		}
		return passwordHash, nil
	}}
	generateToken := func() (string, []byte, error) { return rawToken, tokenHash, nil }
	generateCode := func() (string, error) { return code, nil }
	codeSender := passwordRegistrationCodeSenderStub{sendFunc: func(gotCtx context.Context, gotEmail, gotCode string) error {
		if got := gotCtx.Value(contextKey{}); got != contextValue {
			t.Errorf("SendPasswordRegistrationCode() context value = %v, want %q", got, contextValue)
		}
		if gotEmail != normalizedEmail {
			t.Errorf("SendPasswordRegistrationCode() email = %q, want %q", gotEmail, normalizedEmail)
		}
		if gotCode != code {
			t.Errorf("SendPasswordRegistrationCode() code = %q, want %q", gotCode, code)
		}
		return nil
	}}

	service := newUserService(repository, hasher, generateToken, generateCode, codeSender)
	beforeStart := time.Now().UTC()
	result, err := service.StartPasswordRegistration(ctx, inputEmail, password)
	afterStart := time.Now().UTC()
	if err != nil {
		t.Fatalf("StartPasswordRegistration() error = %v", err)
	}

	if result.Token != rawToken {
		t.Errorf("StartPasswordRegistration() token = %q, want %q", result.Token, rawToken)
	}
	if !result.ExpiresAt.Equal(storedAttempt.AttemptExpiresAt) {
		t.Errorf("StartPasswordRegistration() expiration = %v, want %v", result.ExpiresAt, storedAttempt.AttemptExpiresAt)
	}
	if !bytes.Equal(storedAttempt.TokenHash, tokenHash) {
		t.Errorf("stored token hash = %x, want %x", storedAttempt.TokenHash, tokenHash)
	}
	if storedAttempt.Email != normalizedEmail {
		t.Errorf("stored email = %q, want %q", storedAttempt.Email, normalizedEmail)
	}
	if storedAttempt.PasswordHash != passwordHash {
		t.Errorf("stored password hash = %q, want %q", storedAttempt.PasswordHash, passwordHash)
	}
	if storedAttempt.PasswordHash == password {
		t.Error("stored the plain-text password")
	}
	if !verificationcode.Matches(rawToken, code, storedAttempt.VerificationProofHash) {
		t.Error("stored verification proof does not match the generated token and code")
	}
	if storedAttempt.LastCodeSentAt.Before(beforeStart) || storedAttempt.LastCodeSentAt.After(afterStart) {
		t.Errorf("LastCodeSentAt = %v, want between %v and %v", storedAttempt.LastCodeSentAt, beforeStart, afterStart)
	}
	if !storedAttempt.CodeExpiresAt.Equal(storedAttempt.LastCodeSentAt.Add(testCodeTTL)) {
		t.Errorf("CodeExpiresAt = %v, want %v", storedAttempt.CodeExpiresAt, storedAttempt.LastCodeSentAt.Add(testCodeTTL))
	}
	if !storedAttempt.AttemptExpiresAt.Equal(storedAttempt.LastCodeSentAt.Add(testAttemptTTL)) {
		t.Errorf("AttemptExpiresAt = %v, want %v", storedAttempt.AttemptExpiresAt, storedAttempt.LastCodeSentAt.Add(testAttemptTTL))
	}
}

func TestUserServiceStartPasswordRegistrationRejectsInvalidInput(t *testing.T) {
	tests := []struct {
		name     string
		email    string
		password string
		wantErr  error
	}{
		{name: "invalid email", email: "invalid-email", password: "senha-segura", wantErr: services.ErrInvalidEmail},
		{name: "short password", email: "user@example.com", password: "1234567", wantErr: services.ErrPasswordTooShort},
		{name: "long password", email: "user@example.com", password: strings.Repeat("a", 1025), wantErr: services.ErrPasswordTooLong},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			service := newUserService(unexpectedUserRepository(t), unexpectedPasswordHasher(t), unexpectedRegistrationTokenGenerator(t), unexpectedVerificationCodeGenerator(t), unexpectedCodeSender(t))
			_, err := service.StartPasswordRegistration(context.Background(), tt.email, tt.password)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("StartPasswordRegistration() error = %v, want %v", err, tt.wantErr)
			}
		})
	}
}

func TestUserServiceStartPasswordRegistrationRejectsExistingEmail(t *testing.T) {
	repository := userRepositoryStub{userExistsByEmailFunc: func(context.Context, string) (bool, error) { return true, nil }}
	service := newUserService(repository, unexpectedPasswordHasher(t), unexpectedRegistrationTokenGenerator(t), unexpectedVerificationCodeGenerator(t), unexpectedCodeSender(t))

	_, err := service.StartPasswordRegistration(context.Background(), "user@example.com", "senha-segura")
	if !errors.Is(err, services.ErrEmailAlreadyExists) {
		t.Fatalf("StartPasswordRegistration() error = %v, want %v", err, services.ErrEmailAlreadyExists)
	}
}

func TestUserServiceStartPasswordRegistrationPropagatesUserLookupError(t *testing.T) {
	wantErr := errors.New("database unavailable")
	repository := userRepositoryStub{userExistsByEmailFunc: func(context.Context, string) (bool, error) { return false, wantErr }}
	service := newUserService(repository, unexpectedPasswordHasher(t), unexpectedRegistrationTokenGenerator(t), unexpectedVerificationCodeGenerator(t), unexpectedCodeSender(t))

	_, err := service.StartPasswordRegistration(context.Background(), "user@example.com", "senha-segura")
	if !errors.Is(err, wantErr) {
		t.Fatalf("StartPasswordRegistration() error = %v, want wrapped %v", err, wantErr)
	}
}

func TestUserServiceStartPasswordRegistrationPropagatesHasherError(t *testing.T) {
	wantErr := errors.New("hasher unavailable")
	hasher := passwordHasherStub{hashFunc: func(string) (string, error) { return "", wantErr }}
	service := newUserService(availableEmailRepository(), hasher, unexpectedRegistrationTokenGenerator(t), unexpectedVerificationCodeGenerator(t), unexpectedCodeSender(t))

	_, err := service.StartPasswordRegistration(context.Background(), "user@example.com", "senha-segura")
	if !errors.Is(err, wantErr) {
		t.Fatalf("StartPasswordRegistration() error = %v, want wrapped %v", err, wantErr)
	}
}

func TestUserServiceStartPasswordRegistrationPropagatesTokenGeneratorError(t *testing.T) {
	wantErr := errors.New("random source unavailable")
	generateToken := func() (string, []byte, error) { return "", nil, wantErr }
	service := newUserService(availableEmailRepository(), successfulPasswordHasher(), generateToken, unexpectedVerificationCodeGenerator(t), unexpectedCodeSender(t))

	_, err := service.StartPasswordRegistration(context.Background(), "user@example.com", "senha-segura")
	if !errors.Is(err, wantErr) {
		t.Fatalf("StartPasswordRegistration() error = %v, want wrapped %v", err, wantErr)
	}
}

func TestUserServiceStartPasswordRegistrationPropagatesCodeGeneratorError(t *testing.T) {
	wantErr := errors.New("code generator unavailable")
	generateCode := func() (string, error) { return "", wantErr }
	service := newUserService(availableEmailRepository(), successfulPasswordHasher(), successfulTokenGenerator(), generateCode, unexpectedCodeSender(t))

	_, err := service.StartPasswordRegistration(context.Background(), "user@example.com", "senha-segura")
	if !errors.Is(err, wantErr) {
		t.Fatalf("StartPasswordRegistration() error = %v, want wrapped %v", err, wantErr)
	}
}

func TestUserServiceStartPasswordRegistrationPropagatesCreateAttemptError(t *testing.T) {
	wantErr := errors.New("database unavailable")
	repository := userRepositoryStub{
		userExistsByEmailFunc:                 func(context.Context, string) (bool, error) { return false, nil },
		createPasswordRegistrationAttemptFunc: func(context.Context, models.PasswordRegistrationAttempt) error { return wantErr },
	}
	service := newUserService(repository, successfulPasswordHasher(), successfulTokenGenerator(), successfulVerificationCodeGenerator(), unexpectedCodeSender(t))

	_, err := service.StartPasswordRegistration(context.Background(), "user@example.com", "senha-segura")
	if !errors.Is(err, wantErr) {
		t.Fatalf("StartPasswordRegistration() error = %v, want wrapped %v", err, wantErr)
	}
}

func TestUserServiceStartPasswordRegistrationPropagatesCodeSenderError(t *testing.T) {
	wantErr := errors.New("email provider unavailable")
	attemptCreated := false
	repository := userRepositoryStub{
		userExistsByEmailFunc: func(context.Context, string) (bool, error) { return false, nil },
		createPasswordRegistrationAttemptFunc: func(context.Context, models.PasswordRegistrationAttempt) error {
			attemptCreated = true
			return nil
		},
	}
	codeSender := passwordRegistrationCodeSenderStub{sendFunc: func(context.Context, string, string) error { return wantErr }}
	service := newUserService(repository, successfulPasswordHasher(), successfulTokenGenerator(), successfulVerificationCodeGenerator(), codeSender)

	_, err := service.StartPasswordRegistration(context.Background(), "user@example.com", "senha-segura")
	if !errors.Is(err, wantErr) {
		t.Fatalf("StartPasswordRegistration() error = %v, want wrapped %v", err, wantErr)
	}
	if !attemptCreated {
		t.Error("registration attempt was not created before sending the code")
	}
}

func TestUserServiceConfirmPasswordRegistration(t *testing.T) {
	const (
		token        = "registration-token"
		code         = "123456"
		email        = "user@example.com"
		passwordHash = "$argon2id$confirmed-password-hash"
	)

	wantUser := models.User{
		ID:              uuid.MustParse("01991f29-7c22-7ab3-a395-4d402f09c318"),
		Email:           email,
		EmailVerifiedAt: time.Now().UTC(),
		CreatedAt:       time.Now().UTC(),
		UpdatedAt:       time.Now().UTC(),
	}
	attempt := validConfirmationAttempt(token, code)
	attempt.Email = email
	attempt.PasswordHash = passwordHash

	type contextKey struct{}
	ctx := context.WithValue(context.Background(), contextKey{}, "request-context")
	repository := unexpectedUserRepository(t)
	repository.findPasswordRegistrationAttemptByTokenHashFunc = func(gotCtx context.Context, gotTokenHash []byte) (models.PasswordRegistrationAttempt, error) {
		if got := gotCtx.Value(contextKey{}); got != "request-context" {
			t.Errorf("FindPasswordRegistrationAttemptByTokenHash() context value = %v, want %q", got, "request-context")
		}
		if wantTokenHash := sessiontoken.Hash(token); !bytes.Equal(gotTokenHash, wantTokenHash) {
			t.Errorf("token hash = %x, want %x", gotTokenHash, wantTokenHash)
		}
		return attempt, nil
	}
	repository.createWithPasswordFunc = func(gotCtx context.Context, gotEmail, gotPasswordHash string) (models.User, error) {
		if got := gotCtx.Value(contextKey{}); got != "request-context" {
			t.Errorf("CreateWithPassword() context value = %v, want %q", got, "request-context")
		}
		if gotEmail != email {
			t.Errorf("CreateWithPassword() email = %q, want %q", gotEmail, email)
		}
		if gotPasswordHash != passwordHash {
			t.Errorf("CreateWithPassword() password hash = %q, want %q", gotPasswordHash, passwordHash)
		}
		return wantUser, nil
	}

	service := newUserService(repository, unexpectedPasswordHasher(t), nil, nil, nil)
	user, err := service.ConfirmPasswordRegistration(ctx, token, code)
	if err != nil {
		t.Fatalf("ConfirmPasswordRegistration() error = %v", err)
	}
	if user != wantUser {
		t.Errorf("ConfirmPasswordRegistration() user = %+v, want %+v", user, wantUser)
	}
}

func TestUserServiceConfirmPasswordRegistrationRejectsUnavailableAttempt(t *testing.T) {
	const token = "registration-token"

	t.Run("empty token", func(t *testing.T) {
		service := newUserService(unexpectedUserRepository(t), unexpectedPasswordHasher(t), nil, nil, nil)
		_, err := service.ConfirmPasswordRegistration(context.Background(), "", "123456")
		if !errors.Is(err, services.ErrRegistrationAttemptUnavailable) {
			t.Fatalf("ConfirmPasswordRegistration() error = %v, want %v", err, services.ErrRegistrationAttemptUnavailable)
		}
	})

	t.Run("unknown token", func(t *testing.T) {
		repository := unexpectedUserRepository(t)
		repository.findPasswordRegistrationAttemptByTokenHashFunc = func(context.Context, []byte) (models.PasswordRegistrationAttempt, error) {
			return models.PasswordRegistrationAttempt{}, repositories.ErrRegistrationAttemptNotFound
		}
		service := newUserService(repository, unexpectedPasswordHasher(t), nil, nil, nil)

		_, err := service.ConfirmPasswordRegistration(context.Background(), token, "123456")
		if !errors.Is(err, services.ErrRegistrationAttemptUnavailable) {
			t.Fatalf("ConfirmPasswordRegistration() error = %v, want %v", err, services.ErrRegistrationAttemptUnavailable)
		}
	})

	t.Run("expired attempt", func(t *testing.T) {
		attempt := validConfirmationAttempt(token, "123456")
		attempt.AttemptExpiresAt = time.Now().UTC().Add(-time.Minute)
		repository := unexpectedUserRepository(t)
		repository.findPasswordRegistrationAttemptByTokenHashFunc = func(context.Context, []byte) (models.PasswordRegistrationAttempt, error) {
			return attempt, nil
		}
		service := newUserService(repository, unexpectedPasswordHasher(t), nil, nil, nil)

		_, err := service.ConfirmPasswordRegistration(context.Background(), token, "123456")
		if !errors.Is(err, services.ErrRegistrationAttemptUnavailable) {
			t.Fatalf("ConfirmPasswordRegistration() error = %v, want %v", err, services.ErrRegistrationAttemptUnavailable)
		}
	})
}

func TestUserServiceConfirmPasswordRegistrationRejectsLockedOrExpiredCode(t *testing.T) {
	const (
		token = "registration-token"
		code  = "123456"
	)

	tests := []struct {
		name    string
		prepare func(*models.PasswordRegistrationAttempt)
		wantErr error
	}{
		{
			name: "active cooldown or lock",
			prepare: func(attempt *models.PasswordRegistrationAttempt) {
				lockedUntil := time.Now().UTC().Add(time.Minute)
				attempt.LockedUntil = &lockedUntil
			},
			wantErr: services.ErrRegistrationAttemptLocked,
		},
		{
			name: "expired verification code",
			prepare: func(attempt *models.PasswordRegistrationAttempt) {
				attempt.CodeExpiresAt = time.Now().UTC().Add(-time.Minute)
			},
			wantErr: services.ErrVerificationCodeExpired,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			attempt := validConfirmationAttempt(token, code)
			tt.prepare(&attempt)
			repository := unexpectedUserRepository(t)
			repository.findPasswordRegistrationAttemptByTokenHashFunc = func(context.Context, []byte) (models.PasswordRegistrationAttempt, error) {
				return attempt, nil
			}
			service := newUserService(repository, unexpectedPasswordHasher(t), nil, nil, nil)

			_, err := service.ConfirmPasswordRegistration(context.Background(), token, code)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("ConfirmPasswordRegistration() error = %v, want %v", err, tt.wantErr)
			}
		})
	}
}

func TestUserServiceConfirmPasswordRegistrationRecordsInvalidCode(t *testing.T) {
	const (
		token       = "registration-token"
		correctCode = "123456"
		wrongCode   = "654321"
	)

	attempt := validConfirmationAttempt(token, correctCode)
	repository := unexpectedUserRepository(t)
	repository.findPasswordRegistrationAttemptByTokenHashFunc = func(context.Context, []byte) (models.PasswordRegistrationAttempt, error) {
		return attempt, nil
	}
	repository.recordPasswordRegistrationFailureFunc = func(_ context.Context, gotTokenHash []byte, now, retryAt, lockUntil time.Time, maxAttempts int16) (int16, *time.Time, error) {
		if wantTokenHash := sessiontoken.Hash(token); !bytes.Equal(gotTokenHash, wantTokenHash) {
			t.Errorf("token hash = %x, want %x", gotTokenHash, wantTokenHash)
		}
		if !retryAt.Equal(now.Add(30 * time.Second)) {
			t.Errorf("retry at = %v, want %v", retryAt, now.Add(30*time.Second))
		}
		if !lockUntil.Equal(now.Add(5 * time.Minute)) {
			t.Errorf("lock until = %v, want %v", lockUntil, now.Add(5*time.Minute))
		}
		if maxAttempts != 5 {
			t.Errorf("max attempts = %d, want 5", maxAttempts)
		}
		return 1, &retryAt, nil
	}

	service := newUserService(repository, unexpectedPasswordHasher(t), nil, nil, nil)
	_, err := service.ConfirmPasswordRegistration(context.Background(), token, wrongCode)
	if !errors.Is(err, services.ErrVerificationCodeInvalid) {
		t.Fatalf("ConfirmPasswordRegistration() error = %v, want %v", err, services.ErrVerificationCodeInvalid)
	}
}

func TestUserServiceConfirmPasswordRegistrationLocksOnFifthInvalidCode(t *testing.T) {
	const token = "registration-token"

	repository := unexpectedUserRepository(t)
	repository.findPasswordRegistrationAttemptByTokenHashFunc = func(context.Context, []byte) (models.PasswordRegistrationAttempt, error) {
		return validConfirmationAttempt(token, "123456"), nil
	}
	repository.recordPasswordRegistrationFailureFunc = func(_ context.Context, _ []byte, _ time.Time, _ time.Time, lockUntil time.Time, _ int16) (int16, *time.Time, error) {
		return 0, &lockUntil, nil
	}

	service := newUserService(repository, unexpectedPasswordHasher(t), nil, nil, nil)
	_, err := service.ConfirmPasswordRegistration(context.Background(), token, "654321")
	if !errors.Is(err, services.ErrRegistrationAttemptLocked) {
		t.Fatalf("ConfirmPasswordRegistration() error = %v, want %v", err, services.ErrRegistrationAttemptLocked)
	}
}

func TestUserServiceConfirmPasswordRegistrationPropagatesRepositoryErrors(t *testing.T) {
	const (
		token = "registration-token"
		code  = "123456"
	)

	t.Run("find attempt", func(t *testing.T) {
		wantErr := errors.New("database unavailable")
		repository := unexpectedUserRepository(t)
		repository.findPasswordRegistrationAttemptByTokenHashFunc = func(context.Context, []byte) (models.PasswordRegistrationAttempt, error) {
			return models.PasswordRegistrationAttempt{}, wantErr
		}
		service := newUserService(repository, unexpectedPasswordHasher(t), nil, nil, nil)

		_, err := service.ConfirmPasswordRegistration(context.Background(), token, code)
		if !errors.Is(err, wantErr) {
			t.Fatalf("ConfirmPasswordRegistration() error = %v, want wrapped %v", err, wantErr)
		}
	})

	t.Run("record invalid code", func(t *testing.T) {
		wantErr := errors.New("database unavailable")
		repository := unexpectedUserRepository(t)
		repository.findPasswordRegistrationAttemptByTokenHashFunc = func(context.Context, []byte) (models.PasswordRegistrationAttempt, error) {
			return validConfirmationAttempt(token, code), nil
		}
		repository.recordPasswordRegistrationFailureFunc = func(context.Context, []byte, time.Time, time.Time, time.Time, int16) (int16, *time.Time, error) {
			return 0, nil, wantErr
		}
		service := newUserService(repository, unexpectedPasswordHasher(t), nil, nil, nil)

		_, err := service.ConfirmPasswordRegistration(context.Background(), token, "654321")
		if !errors.Is(err, wantErr) {
			t.Fatalf("ConfirmPasswordRegistration() error = %v, want wrapped %v", err, wantErr)
		}
	})

	t.Run("attempt expires while recording invalid code", func(t *testing.T) {
		repository := unexpectedUserRepository(t)
		repository.findPasswordRegistrationAttemptByTokenHashFunc = func(context.Context, []byte) (models.PasswordRegistrationAttempt, error) {
			return validConfirmationAttempt(token, code), nil
		}
		repository.recordPasswordRegistrationFailureFunc = func(context.Context, []byte, time.Time, time.Time, time.Time, int16) (int16, *time.Time, error) {
			return 0, nil, repositories.ErrRegistrationAttemptNotFound
		}
		service := newUserService(repository, unexpectedPasswordHasher(t), nil, nil, nil)

		_, err := service.ConfirmPasswordRegistration(context.Background(), token, "654321")
		if !errors.Is(err, services.ErrRegistrationAttemptUnavailable) {
			t.Fatalf("ConfirmPasswordRegistration() error = %v, want %v", err, services.ErrRegistrationAttemptUnavailable)
		}
	})

	t.Run("create user", func(t *testing.T) {
		wantErr := errors.New("database unavailable")
		repository := unexpectedUserRepository(t)
		repository.findPasswordRegistrationAttemptByTokenHashFunc = func(context.Context, []byte) (models.PasswordRegistrationAttempt, error) {
			return validConfirmationAttempt(token, code), nil
		}
		repository.createWithPasswordFunc = func(context.Context, string, string) (models.User, error) {
			return models.User{}, wantErr
		}
		service := newUserService(repository, unexpectedPasswordHasher(t), nil, nil, nil)

		_, err := service.ConfirmPasswordRegistration(context.Background(), token, code)
		if !errors.Is(err, wantErr) {
			t.Fatalf("ConfirmPasswordRegistration() error = %v, want wrapped %v", err, wantErr)
		}
	})

	t.Run("email already created by concurrent attempt", func(t *testing.T) {
		repository := unexpectedUserRepository(t)
		repository.findPasswordRegistrationAttemptByTokenHashFunc = func(context.Context, []byte) (models.PasswordRegistrationAttempt, error) {
			return validConfirmationAttempt(token, code), nil
		}
		repository.createWithPasswordFunc = func(context.Context, string, string) (models.User, error) {
			return models.User{}, repositories.ErrEmailAlreadyExists
		}
		service := newUserService(repository, unexpectedPasswordHasher(t), nil, nil, nil)

		_, err := service.ConfirmPasswordRegistration(context.Background(), token, code)
		if !errors.Is(err, services.ErrEmailAlreadyExists) {
			t.Fatalf("ConfirmPasswordRegistration() error = %v, want %v", err, services.ErrEmailAlreadyExists)
		}
	})
}

func TestUserServiceGetByID(t *testing.T) {
	wantUserID := uuid.MustParse("01991f29-7c22-7ab3-a395-4d402f09c317")
	wantUser := models.User{
		ID:        wantUserID,
		Email:     "user@example.com",
		CreatedAt: time.Date(2026, time.September, 13, 10, 0, 0, 0, time.UTC),
		UpdatedAt: time.Date(2026, time.September, 13, 11, 0, 0, 0, time.UTC),
	}

	type contextKey struct{}
	ctx := context.WithValue(context.Background(), contextKey{}, "request-context")
	repository := userRepositoryStub{findByIDFunc: func(gotCtx context.Context, gotUserID uuid.UUID) (models.User, error) {
		if got := gotCtx.Value(contextKey{}); got != "request-context" {
			t.Errorf("FindByID() context value = %v, want %q", got, "request-context")
		}
		if gotUserID != wantUserID {
			t.Errorf("FindByID() user ID = %s, want %s", gotUserID, wantUserID)
		}
		return wantUser, nil
	}}

	service := newUserService(repository, unexpectedPasswordHasher(t), nil, nil, nil)
	user, err := service.GetByID(ctx, wantUserID)
	if err != nil {
		t.Fatalf("GetByID() error = %v", err)
	}
	if user != wantUser {
		t.Errorf("GetByID() user = %+v, want %+v", user, wantUser)
	}
}

func TestUserServiceGetByIDPropagatesRepositoryError(t *testing.T) {
	wantErr := errors.New("database unavailable")
	repository := userRepositoryStub{findByIDFunc: func(context.Context, uuid.UUID) (models.User, error) { return models.User{}, wantErr }}
	service := newUserService(repository, unexpectedPasswordHasher(t), nil, nil, nil)

	_, err := service.GetByID(context.Background(), uuid.MustParse("01991f29-7c22-7ab3-a395-4d402f09c317"))
	if !errors.Is(err, wantErr) {
		t.Fatalf("GetByID() error = %v, want wrapped %v", err, wantErr)
	}
}

func newUserService(repository userRepositoryStub, hasher passwordHasherStub, generateToken services.TokenGenerator, generateCode services.VerificationCodeGenerator, codeSender services.PasswordRegistrationCodeSender) *services.UserService {
	return services.NewUserService(repository, hasher, generateToken, generateCode, codeSender, services.PasswordRegistrationConfig{
		CodeTTL: testCodeTTL, AttemptTTL: testAttemptTTL,
	})
}

func availableEmailRepository() userRepositoryStub {
	return userRepositoryStub{
		userExistsByEmailFunc:                 func(context.Context, string) (bool, error) { return false, nil },
		createPasswordRegistrationAttemptFunc: func(context.Context, models.PasswordRegistrationAttempt) error { return nil },
	}
}

func successfulPasswordHasher() passwordHasherStub {
	return passwordHasherStub{hashFunc: func(string) (string, error) { return "$argon2id$test-hash", nil }}
}

func successfulTokenGenerator() services.TokenGenerator {
	return func() (string, []byte, error) { return "registration-token", bytes.Repeat([]byte{0x42}, 32), nil }
}

func successfulVerificationCodeGenerator() services.VerificationCodeGenerator {
	return func() (string, error) { return "123456", nil }
}

func validConfirmationAttempt(token, code string) models.PasswordRegistrationAttempt {
	now := time.Now().UTC()

	return models.PasswordRegistrationAttempt{
		TokenHash:             sessiontoken.Hash(token),
		Email:                 "user@example.com",
		PasswordHash:          "$argon2id$confirmation-test-hash",
		VerificationProofHash: verificationcode.Proof(token, code),
		LastCodeSentAt:        now,
		CodeExpiresAt:         now.Add(10 * time.Minute),
		AttemptExpiresAt:      now.Add(30 * time.Minute),
	}
}

func unexpectedUserRepository(t *testing.T) userRepositoryStub {
	t.Helper()
	return userRepositoryStub{
		userExistsByEmailFunc: func(context.Context, string) (bool, error) {
			t.Fatal("UserExistsByEmail() should not be called")
			return false, nil
		},
		createPasswordRegistrationAttemptFunc: func(context.Context, models.PasswordRegistrationAttempt) error {
			t.Fatal("CreatePasswordRegistrationAttempt() should not be called")
			return nil
		},
		findPasswordRegistrationAttemptByTokenHashFunc: func(context.Context, []byte) (models.PasswordRegistrationAttempt, error) {
			t.Fatal("FindPasswordRegistrationAttemptByTokenHash() should not be called")
			return models.PasswordRegistrationAttempt{}, nil
		},
		recordPasswordRegistrationFailureFunc: func(context.Context, []byte, time.Time, time.Time, time.Time, int16) (int16, *time.Time, error) {
			t.Fatal("RecordPasswordRegistrationFailure() should not be called")
			return 0, nil, nil
		},
		createWithPasswordFunc: func(context.Context, string, string) (models.User, error) {
			t.Fatal("CreateWithPassword() should not be called")
			return models.User{}, nil
		},
		findByIDFunc: func(context.Context, uuid.UUID) (models.User, error) {
			t.Fatal("FindByID() should not be called")
			return models.User{}, nil
		},
	}
}

func unexpectedPasswordHasher(t *testing.T) passwordHasherStub {
	t.Helper()
	return passwordHasherStub{hashFunc: func(string) (string, error) {
		t.Fatal("Hash() should not be called")
		return "", nil
	}}
}

func unexpectedRegistrationTokenGenerator(t *testing.T) services.TokenGenerator {
	t.Helper()
	return func() (string, []byte, error) {
		t.Fatal("token generator should not be called")
		return "", nil, nil
	}
}

func unexpectedVerificationCodeGenerator(t *testing.T) services.VerificationCodeGenerator {
	t.Helper()
	return func() (string, error) {
		t.Fatal("verification code generator should not be called")
		return "", nil
	}
}

func unexpectedCodeSender(t *testing.T) passwordRegistrationCodeSenderStub {
	t.Helper()
	return passwordRegistrationCodeSenderStub{sendFunc: func(context.Context, string, string) error {
		t.Fatal("SendPasswordRegistrationCode() should not be called")
		return nil
	}}
}
