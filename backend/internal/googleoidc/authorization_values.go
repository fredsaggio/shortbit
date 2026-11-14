package googleoidc

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
)

// 32 bytes = 256 bits
const authorizationValueSize = 32

type AuthorizationValues struct {
	State        string
	Nonce        string
	CodeVerifier string
}

func GenerateAuthorizationValues() (AuthorizationValues, error) {
	state, err := randomAuthorizationValue()
	if err != nil {
		return AuthorizationValues{}, fmt.Errorf("generate OAuth state: %w", err)
	}

	nonce, err := randomAuthorizationValue()

	if err != nil {
		return AuthorizationValues{}, fmt.Errorf("generate OIDC nonce: %w", err)
	}

	codeVerifier, err := randomAuthorizationValue()

	if err != nil {
		return AuthorizationValues{}, fmt.Errorf("generate PKCE code verifier: %w", err)
	}

	return AuthorizationValues{
		State:        state,
		Nonce:        nonce,
		CodeVerifier: codeVerifier,
	}, nil
}

func randomAuthorizationValue() (string, error) {
	randomBytes := make([]byte, authorizationValueSize)

	if _, err := rand.Read(randomBytes); err != nil {
		return "", fmt.Errorf("read cryptographically secure random bytes: %w", err)
	}

	return base64.RawURLEncoding.EncodeToString(randomBytes), nil
}
