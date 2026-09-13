package argon2

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"

	"golang.org/x/crypto/argon2"
)

const (
	memory      uint32 = 19 * 1024
	iterations  uint32 = 2
	parallelism uint8  = 1
	saltLength         = 16
	keyLength          = 32

	maxPasswordBytes = 1024
)

var (
	ErrPasswordTooLong = errors.New("password is too long")
	ErrInvalidHash     = errors.New("invalid password hash")
)

type Argon2id struct{}

func (Argon2id) Hash(password string) (string, error) {
	if len(password) > maxPasswordBytes {
		return "", ErrPasswordTooLong
	}

	salt := make([]byte, saltLength)

	if _, err := rand.Read(salt); err != nil {
		return "", fmt.Errorf("generate salt: %w", err)
	}

	hash := argon2.IDKey(
		[]byte(password),
		salt,
		iterations,
		memory,
		parallelism,
		keyLength,
	)

	encodedSalt := base64.RawStdEncoding.EncodeToString(salt)
	encodedHash := base64.RawStdEncoding.EncodeToString(hash)

	phc := fmt.Sprintf(
		"$argon2id$v=%d$m=%d,t=%d,p=%d$%s$%s",
		argon2.Version,
		memory,
		iterations,
		parallelism,
		encodedSalt,
		encodedHash,
	)

	return phc, nil
}

func (Argon2id) Compare(password, encodedHash string) (bool, error) {
	if len(password) > maxPasswordBytes {
		return false, ErrPasswordTooLong
	}

	parameters, salt, expectedHash, err := decodeHash(encodedHash)

	if err != nil {
		return false, err
	}

	actualHash := argon2.IDKey(
		[]byte(password),
		salt,
		parameters.iterations,
		parameters.memory,
		parameters.parallelism,
		uint32(len(expectedHash)),
	)

	match := subtle.ConstantTimeCompare(actualHash, expectedHash) == 1

	return match, nil
}

type hashParameters struct {
	memory      uint32
	iterations  uint32
	parallelism uint8
}

func decodeHash(encodedHash string) (hashParameters, []byte, []byte, error) {
	parts := strings.Split(encodedHash, "$")
	if len(parts) != 6 || parts[0] != "" || parts[1] != "argon2id" {
		return hashParameters{}, nil, nil, ErrInvalidHash
	}

	var version int

	n, err := fmt.Sscanf(parts[2], "v=%d", &version)
	if err != nil ||
		n != 1 ||
		parts[2] != fmt.Sprintf("v=%d", version) ||
		version != argon2.Version {
		return hashParameters{}, nil, nil, ErrInvalidHash
	}

	var parameters hashParameters

	n, err = fmt.Sscanf(
		parts[3],
		"m=%d,t=%d,p=%d",
		&parameters.memory,
		&parameters.iterations,
		&parameters.parallelism,
	)

	if err != nil ||
		n != 3 ||
		parts[3] != fmt.Sprintf(
			"m=%d,t=%d,p=%d",
			parameters.memory,
			parameters.iterations,
			parameters.parallelism,
		) {
		return hashParameters{}, nil, nil, ErrInvalidHash
	}

	if parameters.memory != memory ||
		parameters.iterations != iterations ||
		parameters.parallelism != parallelism {
		return hashParameters{}, nil, nil, ErrInvalidHash
	}

	salt, err := base64.RawStdEncoding.DecodeString(parts[4])
	if err != nil || len(salt) != saltLength {
		return hashParameters{}, nil, nil, ErrInvalidHash
	}

	hash, err := base64.RawStdEncoding.DecodeString(parts[5])
	if err != nil || len(hash) != keyLength {
		return hashParameters{}, nil, nil, ErrInvalidHash
	}

	return parameters, salt, hash, nil
}
