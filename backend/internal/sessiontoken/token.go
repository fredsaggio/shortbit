package sessiontoken

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
)

const tokenSize = 32

func Generate() (string, []byte, error) {
	randomBytes := make([]byte, tokenSize)

	if _, err := rand.Read(randomBytes); err != nil {
		return "", nil, fmt.Errorf("generate random token: %w", err)
	}

	token := base64.RawURLEncoding.EncodeToString(randomBytes)

	return token, Hash(token), nil
}

func Hash(token string) []byte {
	tokenHash := sha256.Sum256([]byte(token))
	return tokenHash[:]
}
