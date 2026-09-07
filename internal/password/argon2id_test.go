package password_test

import (
	"encoding/base64"
	"errors"
	"strings"
	"testing"

	"github.com/fredsaggio/url-shortener/internal/password"
)

func TestArgon2idHashAndCompare(t *testing.T) {
	hasher := password.Argon2id{}

	encodedHash, err := hasher.Hash("senha-segura")
	if err != nil {
		t.Fatalf("Hash() error = %v", err)
	}

	if !strings.HasPrefix(
		encodedHash,
		"$argon2id$v=19$m=19456,t=2,p=1$",
	) {
		t.Fatalf("Hash() returned unexpected PHC: %q", encodedHash)
	}

	tests := []struct {
		name     string
		password string
		want     bool
	}{
		{
			name:     "correct password",
			password: "senha-segura",
			want:     true,
		},
		{
			name:     "incorrect password",
			password: "senha-incorreta",
			want:     false,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			match, err := hasher.Compare(test.password, encodedHash)
			if err != nil {
				t.Fatalf("Compare() error = %v", err)
			}

			if match != test.want {
				t.Errorf(
					"Compare() match = %v, want %v",
					match,
					test.want,
				)
			}
		})
	}
}

func TestArgon2idHashUsesRandomSalt(t *testing.T) {
	hasher := password.Argon2id{}

	firstHash, err := hasher.Hash("mesma-senha")
	if err != nil {
		t.Fatalf("first Hash() error = %v", err)
	}

	secondHash, err := hasher.Hash("mesma-senha")
	if err != nil {
		t.Fatalf("second Hash() error = %v", err)
	}

	if firstHash == secondHash {
		t.Fatal("Hash() returned identical hashes for the same password")
	}

	firstMatch, err := hasher.Compare("mesma-senha", firstHash)
	if err != nil {
		t.Fatalf("Compare() first hash error = %v", err)
	}

	secondMatch, err := hasher.Compare("mesma-senha", secondHash)
	if err != nil {
		t.Fatalf("Compare() second hash error = %v", err)
	}

	if !firstMatch || !secondMatch {
		t.Fatal("different salted hashes must accept the original password")
	}
}

func TestArgon2idRejectsPasswordTooLong(t *testing.T) {
	hasher := password.Argon2id{}
	longPassword := strings.Repeat("a", 1025)

	_, err := hasher.Hash(longPassword)
	if !errors.Is(err, password.ErrPasswordTooLong) {
		t.Fatalf(
			"Hash() error = %v, want ErrPasswordTooLong",
			err,
		)
	}

	_, err = hasher.Compare(longPassword, "irrelevant")
	if !errors.Is(err, password.ErrPasswordTooLong) {
		t.Fatalf(
			"Compare() error = %v, want ErrPasswordTooLong",
			err,
		)
	}
}

func TestArgon2idCompareRejectsInvalidHash(t *testing.T) {
	hasher := password.Argon2id{}

	validHash, err := hasher.Hash("senha-segura")
	if err != nil {
		t.Fatalf("Hash() error = %v", err)
	}

	oneByte := base64.RawStdEncoding.EncodeToString([]byte{1})

	tests := []struct {
		name        string
		encodedHash string
	}{
		{
			name:        "empty hash",
			encodedHash: "",
		},
		{
			name: "wrong algorithm",
			encodedHash: replacePHCPart(
				t,
				validHash,
				1,
				"argon2i",
			),
		},
		{
			name: "unsupported version",
			encodedHash: replacePHCPart(
				t,
				validHash,
				2,
				"v=18",
			),
		},
		{
			name: "unsupported parameters",
			encodedHash: replacePHCPart(
				t,
				validHash,
				3,
				"m=19457,t=2,p=1",
			),
		},
		{
			name: "invalid salt encoding",
			encodedHash: replacePHCPart(
				t,
				validHash,
				4,
				"***",
			),
		},
		{
			name: "wrong salt length",
			encodedHash: replacePHCPart(
				t,
				validHash,
				4,
				oneByte,
			),
		},
		{
			name: "invalid hash encoding",
			encodedHash: replacePHCPart(
				t,
				validHash,
				5,
				"***",
			),
		},
		{
			name: "wrong hash length",
			encodedHash: replacePHCPart(
				t,
				validHash,
				5,
				oneByte,
			),
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			match, err := hasher.Compare(
				"senha-segura",
				test.encodedHash,
			)

			if !errors.Is(err, password.ErrInvalidHash) {
				t.Fatalf(
					"Compare() error = %v, want ErrInvalidHash",
					err,
				)
			}

			if match {
				t.Fatal("Compare() match = true, want false")
			}
		})
	}
}

func replacePHCPart(
	t *testing.T,
	encodedHash string,
	index int,
	value string,
) string {
	t.Helper()

	parts := strings.Split(encodedHash, "$")

	if len(parts) != 6 {
		t.Fatalf(
			"invalid test fixture: PHC contains %d parts",
			len(parts),
		)
	}

	parts[index] = value

	return strings.Join(parts, "$")
}

func BenchmarkArgon2idHash(b *testing.B) {
	hasher := password.Argon2id{}

	b.ReportAllocs()

	for b.Loop() {
		_, err := hasher.Hash("senha-segura")
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkArgon2idCompare(b *testing.B) {
	hasher := password.Argon2id{}

	encodedHash, err := hasher.Hash("senha-segura")
	if err != nil {
		b.Fatal(err)
	}

	b.ReportAllocs()

	for b.Loop() {
		match, err := hasher.Compare("senha-segura", encodedHash)
		if err != nil {
			b.Fatal(err)
		}

		if !match {
			b.Fatal("password should match")
		}
	}
}
