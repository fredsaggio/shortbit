package verificationcode

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"fmt"
	"math/big"
)

const codeLimit = 1_000_000

func Generate() (string, error) {
	number, err := rand.Int(rand.Reader, big.NewInt(codeLimit))

	if err != nil {
		return "", fmt.Errorf("generate verification code: %w", err)
	}

	return fmt.Sprintf("%06d", number.Int64()), nil
}

func Proof(token, code string) []byte {
	mac := hmac.New(sha256.New, []byte(token))
	_, _ = mac.Write([]byte(code))
	return mac.Sum(nil)
}

func Matches(token, code string, expectedProof []byte) bool {
	actualProof := Proof(token, code)
	return hmac.Equal(actualProof, expectedProof)
}
