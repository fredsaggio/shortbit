package sessiontoken

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
)

const tokenSize = 32

type Pair struct {
	Raw  string
	Hash [32]byte
}

func Generate() (Pair, error) {
	randomBytes := make([]byte, tokenSize)

	if _, err := rand.Read(randomBytes); err != nil {
		return Pair{}, fmt.Errorf("generate random token: %w", err)
	}

	raw := base64.RawURLEncoding.EncodeToString(randomBytes)

	return Pair{
		Raw:  raw,
		Hash: Hash(raw),
	}, nil
}

func Hash(raw string) [32]byte {
	return sha256.Sum256([]byte(raw))
}
