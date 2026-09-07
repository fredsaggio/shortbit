package sessiontoken_test

import (
	"encoding/base64"
	"strings"
	"testing"

	"github.com/fredsaggio/url-shortener/internal/sessiontoken"
)

func TestGenerate(t *testing.T) {
	token, err := sessiontoken.Generate()
	if err != nil {
		t.Fatalf("Generate() error = %v", err)
	}

	if token.Raw == "" {
		t.Fatal("Generate() Raw is empty")
	}

	if strings.Contains(token.Raw, "=") {
		t.Fatalf("Generate() Raw = %q, want unpadded Base64URL", token.Raw)
	}

	randomBytes, err := base64.RawURLEncoding.DecodeString(token.Raw)
	if err != nil {
		t.Fatalf("Generate() Raw is not valid Base64URL: %v", err)
	}

	if len(randomBytes) != 32 {
		t.Fatalf("decoded token length = %d, want 32", len(randomBytes))
	}

	wantHash := sessiontoken.Hash(token.Raw)
	if token.Hash != wantHash {
		t.Fatal("Generate() Hash does not match Hash(Raw)")
	}
}

func TestGenerateReturnsDifferentTokens(t *testing.T) {
	firstToken, err := sessiontoken.Generate()
	if err != nil {
		t.Fatalf("first Generate() error = %v", err)
	}

	secondToken, err := sessiontoken.Generate()
	if err != nil {
		t.Fatalf("second Generate() error = %v", err)
	}

	if firstToken.Raw == secondToken.Raw {
		t.Fatal("Generate() returned identical raw tokens")
	}

	if firstToken.Hash == secondToken.Hash {
		t.Fatal("Generate() returned identical token hashes")
	}
}

func TestHash(t *testing.T) {
	firstHash := sessiontoken.Hash("token")
	secondHash := sessiontoken.Hash("token")
	differentHash := sessiontoken.Hash("different-token")

	if firstHash != secondHash {
		t.Fatal("Hash() returned different hashes for the same token")
	}

	if firstHash == differentHash {
		t.Fatal("Hash() returned identical hashes for different tokens")
	}
}
