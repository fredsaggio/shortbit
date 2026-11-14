package verificationcode

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"fmt"
	"math/big"
)

func Generate() (string, error) {
	return generateDigits(6)
}

func GeneratePasswordReset() (string, error) {
	return generateDigits(8)
}

func generateDigits(digits int) (string, error) {
	limit := int64(1)
	for range digits {
		limit *= 10
	}

	number, err := rand.Int(rand.Reader, big.NewInt(limit))

	if err != nil {
		return "", fmt.Errorf("generate verification code: %w", err)
	}

	return fmt.Sprintf("%0*d", digits, number.Int64()), nil
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
