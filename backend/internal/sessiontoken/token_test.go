package sessiontoken_test

import (
	"bytes"
	"encoding/base64"
	"strings"
	"testing"

	"github.com/fredsaggio/url-shortener/internal/sessiontoken"
)

func TestGenerate(t *testing.T) {
	raw, hash, err := sessiontoken.Generate()
	if err != nil {
		t.Fatalf("Generate() error = %v", err)
	}

	if raw == "" {
		t.Fatal("Generate() Raw is empty")
	}

	if strings.Contains(raw, "=") {
		t.Fatalf("Generate() Raw = %q, want unpadded Base64URL", raw)
	}

	randomBytes, err := base64.RawURLEncoding.DecodeString(raw)
	if err != nil {
		t.Fatalf("Generate() Raw is not valid Base64URL: %v", err)
	}

	if len(randomBytes) != 32 {
		t.Fatalf("decoded token length = %d, want 32", len(randomBytes))
	}

	wantHash := sessiontoken.Hash(raw)
	if !bytes.Equal(hash, wantHash) {
		t.Fatal("Generate() Hash does not match Hash(Raw)")
	}
}

func TestGenerateReturnsDifferentTokens(t *testing.T) {
	firstRaw, firstHash, err := sessiontoken.Generate()
	if err != nil {
		t.Fatalf("first Generate() error = %v", err)
	}

	secondRaw, secondHash, err := sessiontoken.Generate()
	if err != nil {
		t.Fatalf("second Generate() error = %v", err)
	}

	if firstRaw == secondRaw {
		t.Fatal("Generate() returned identical raw tokens")
	}

	if bytes.Equal(firstHash, secondHash) {
		t.Fatal("Generate() returned identical token hashes")
	}
}

func TestHash(t *testing.T) {
	firstHash := sessiontoken.Hash("token")
	secondHash := sessiontoken.Hash("token")
	differentHash := sessiontoken.Hash("different-token")

	if !bytes.Equal(firstHash, secondHash) {
		t.Fatal("Hash() returned different hashes for the same token")
	}

	if bytes.Equal(firstHash, differentHash) {
		t.Fatal("Hash() returned identical hashes for different tokens")
	}
}
