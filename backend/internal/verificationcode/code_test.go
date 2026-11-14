package verificationcode_test

import (
	"bytes"
	"crypto/sha256"
	"testing"

	"github.com/fredsaggio/url-shortener/internal/verificationcode"
)

func TestGenerateReturnsSixDecimalDigits(t *testing.T) {
	for range 100 {
		code, err := verificationcode.Generate()
		if err != nil {
			t.Fatalf("Generate() error = %v", err)
		}

		if len(code) != 6 {
			t.Fatalf("Generate() code length = %d, want 6; code = %q", len(code), code)
		}

		for _, character := range code {
			if character < '0' || character > '9' {
				t.Fatalf("Generate() code = %q, want only decimal digits", code)
			}
		}
	}
}

func TestGeneratePasswordResetReturnsEightDecimalDigits(t *testing.T) {
	for range 100 {
		code, err := verificationcode.GeneratePasswordReset()
		if err != nil {
			t.Fatalf("GeneratePasswordReset() error = %v", err)
		}
		if len(code) != 8 {
			t.Fatalf("GeneratePasswordReset() code length = %d, want 8; code = %q", len(code), code)
		}
		for _, character := range code {
			if character < '0' || character > '9' {
				t.Fatalf("GeneratePasswordReset() code = %q, want only decimal digits", code)
			}
		}
	}
}

func TestProofIsDeterministicAndBoundToTokenAndCode(t *testing.T) {
	const (
		token = "registration-token"
		code  = "123456"
	)

	proof := verificationcode.Proof(token, code)
	sameProof := verificationcode.Proof(token, code)
	proofWithDifferentToken := verificationcode.Proof("different-token", code)
	proofWithDifferentCode := verificationcode.Proof(token, "654321")

	if len(proof) != sha256.Size {
		t.Errorf("Proof() length = %d, want %d", len(proof), sha256.Size)
	}
	if !bytes.Equal(proof, sameProof) {
		t.Error("Proof() returned different values for the same token and code")
	}
	if bytes.Equal(proof, proofWithDifferentToken) {
		t.Error("Proof() returned the same value for different tokens")
	}
	if bytes.Equal(proof, proofWithDifferentCode) {
		t.Error("Proof() returned the same value for different codes")
	}
}

func TestMatches(t *testing.T) {
	const (
		token = "registration-token"
		code  = "123456"
	)

	proof := verificationcode.Proof(token, code)

	tests := []struct {
		name          string
		token         string
		code          string
		expectedProof []byte
		want          bool
	}{
		{name: "matching token and code", token: token, code: code, expectedProof: proof, want: true},
		{name: "different token", token: "different-token", code: code, expectedProof: proof, want: false},
		{name: "different code", token: token, code: "654321", expectedProof: proof, want: false},
		{name: "different proof", token: token, code: code, expectedProof: make([]byte, sha256.Size), want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := verificationcode.Matches(tt.token, tt.code, tt.expectedProof); got != tt.want {
				t.Errorf("Matches() = %t, want %t", got, tt.want)
			}
		})
	}
}
