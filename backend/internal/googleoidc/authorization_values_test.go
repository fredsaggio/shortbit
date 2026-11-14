package googleoidc

import (
	"encoding/base64"
	"testing"
)

func TestGenerateAuthorizationValues(t *testing.T) {
	values, err := GenerateAuthorizationValues()
	if err != nil {
		t.Fatalf("GenerateAuthorizationValues() error = %v", err)
	}

	generated := map[string]string{
		"state":         values.State,
		"nonce":         values.Nonce,
		"code verifier": values.CodeVerifier,
	}

	for name, value := range generated {
		if value == "" {
			t.Fatalf("%s is empty", name)
		}

		randomBytes, err := base64.RawURLEncoding.DecodeString(value)
		if err != nil {
			t.Fatalf("%s = %q is not valid unpadded Base64URL: %v", name, value, err)
		}

		if len(randomBytes) != authorizationValueSize {
			t.Errorf("decoded %s length = %d bytes, want %d", name, len(randomBytes), authorizationValueSize)
		}
	}

	if values.State == values.Nonce {
		t.Error("state and nonce must be different")
	}
	if values.State == values.CodeVerifier {
		t.Error("state and code verifier must be different")
	}
	if values.Nonce == values.CodeVerifier {
		t.Error("nonce and code verifier must be different")
	}
}

func TestGenerateAuthorizationValuesReturnsFreshValues(t *testing.T) {
	first, err := GenerateAuthorizationValues()
	if err != nil {
		t.Fatalf("first GenerateAuthorizationValues() error = %v", err)
	}

	second, err := GenerateAuthorizationValues()
	if err != nil {
		t.Fatalf("second GenerateAuthorizationValues() error = %v", err)
	}

	if first.State == second.State {
		t.Error("GenerateAuthorizationValues() returned the same state twice")
	}
	if first.Nonce == second.Nonce {
		t.Error("GenerateAuthorizationValues() returned the same nonce twice")
	}
	if first.CodeVerifier == second.CodeVerifier {
		t.Error("GenerateAuthorizationValues() returned the same code verifier twice")
	}
}
