package googleoidc

import (
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/url"
	"testing"
	"time"

	"github.com/coreos/go-oidc/v3/oidc"
	jose "github.com/go-jose/go-jose/v4"
	"golang.org/x/oauth2"
)

func TestClientAuthorizationURL(t *testing.T) {
	const (
		clientID     = "application-client-id"
		redirectURL  = "http://localhost:8080/auth/google/callback"
		state        = "random-state"
		nonce        = "random-nonce"
		codeVerifier = "random-code-verifier-with-enough-characters-123456789"
		authURL      = "https://accounts.google.com/o/oauth2/v2/auth"
	)

	client := &Client{
		oauth2Config: oauth2.Config{
			ClientID:    clientID,
			RedirectURL: redirectURL,
			Endpoint: oauth2.Endpoint{
				AuthURL: authURL,
			},
			Scopes: []string{oidc.ScopeOpenID, oidc.ScopeEmail},
		},
	}

	got, err := url.Parse(client.AuthorizationURL(state, nonce, codeVerifier))
	if err != nil {
		t.Fatalf("parse authorization URL: %v", err)
	}

	codeVerifierHash := sha256.Sum256([]byte(codeVerifier))
	wantCodeChallenge := base64.RawURLEncoding.EncodeToString(codeVerifierHash[:])

	wantParameters := map[string]string{
		"client_id":             clientID,
		"redirect_uri":          redirectURL,
		"response_type":         "code",
		"scope":                 "openid email",
		"state":                 state,
		"nonce":                 nonce,
		"code_challenge":        wantCodeChallenge,
		"code_challenge_method": "S256",
		"access_type":           "online",
	}

	for name, want := range wantParameters {
		if value := got.Query().Get(name); value != want {
			t.Errorf("query parameter %q = %q, want %q", name, value, want)
		}
	}

	if got.Query().Has("code_verifier") {
		t.Error("authorization URL must not expose the code verifier")
	}
}

func TestClientExtractIDToken(t *testing.T) {
	tests := []struct {
		name    string
		extra   map[string]any
		want    string
		wantErr error
	}{
		{
			name:  "ID token present",
			extra: map[string]any{"id_token": "raw-id-token"},
			want:  "raw-id-token",
		},
		{
			name:    "ID token absent",
			extra:   map[string]any{},
			wantErr: ErrMissingIDToken,
		},
		{
			name:    "ID token empty",
			extra:   map[string]any{"id_token": ""},
			wantErr: ErrMissingIDToken,
		},
		{
			name:    "ID token has invalid type",
			extra:   map[string]any{"id_token": 123},
			wantErr: ErrMissingIDToken,
		},
	}

	client := &Client{}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			token := (&oauth2.Token{}).WithExtra(tt.extra)

			got, err := client.extractIDToken(token)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("extractIDToken() error = %v, want %v", err, tt.wantErr)
			}

			if got != tt.want {
				t.Errorf("extractIDToken() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestClientVerifyIDToken(t *testing.T) {
	const (
		issuer        = "https://accounts.google.com"
		clientID      = "application-client-id"
		subject       = "google-subject-123"
		expectedNonce = "expected-nonce"
	)

	trustedPrivateKey := generateRSAKey(t)
	client := &Client{
		tokenVerifier: oidc.NewVerifier(
			issuer,
			&oidc.StaticKeySet{PublicKeys: []crypto.PublicKey{trustedPrivateKey.Public()}},
			&oidc.Config{ClientID: clientID},
		),
	}

	validClaims := testIDTokenClaims{
		Issuer:        issuer,
		Audience:      clientID,
		Subject:       subject,
		Nonce:         expectedNonce,
		Expiration:    time.Now().Add(time.Hour).Unix(),
		IssuedAt:      time.Now().Unix(),
		Email:         "user@example.com",
		EmailVerified: true,
	}
	validRawIDToken := signIDToken(t, trustedPrivateKey, validClaims)

	t.Run("valid signed token and nonce", func(t *testing.T) {
		idToken, err := client.verifyIDToken(t.Context(), validRawIDToken, expectedNonce)
		if err != nil {
			t.Fatalf("verifyIDToken() error = %v", err)
		}

		if idToken.Subject != subject {
			t.Errorf("verifyIDToken() subject = %q, want %q", idToken.Subject, subject)
		}
		if idToken.Nonce != expectedNonce {
			t.Errorf("verifyIDToken() nonce = %q, want %q", idToken.Nonce, expectedNonce)
		}
	})

	t.Run("different nonce", func(t *testing.T) {
		_, err := client.verifyIDToken(t.Context(), validRawIDToken, "another-nonce")
		if !errors.Is(err, ErrNonceMismatch) {
			t.Fatalf("verifyIDToken() error = %v, want %v", err, ErrNonceMismatch)
		}
	})

	t.Run("empty expected nonce", func(t *testing.T) {
		_, err := client.verifyIDToken(t.Context(), validRawIDToken, "")
		if !errors.Is(err, ErrNonceMismatch) {
			t.Fatalf("verifyIDToken() error = %v, want %v", err, ErrNonceMismatch)
		}
	})

	t.Run("token signed by an untrusted key", func(t *testing.T) {
		untrustedPrivateKey := generateRSAKey(t)
		untrustedRawIDToken := signIDToken(t, untrustedPrivateKey, validClaims)

		if _, err := client.verifyIDToken(t.Context(), untrustedRawIDToken, expectedNonce); err == nil {
			t.Fatal("verifyIDToken() error = nil, want signature verification error")
		}
	})

	t.Run("token issued by another provider", func(t *testing.T) {
		claims := validClaims
		claims.Issuer = "https://attacker.example.com"
		rawIDToken := signIDToken(t, trustedPrivateKey, claims)

		if _, err := client.verifyIDToken(t.Context(), rawIDToken, expectedNonce); err == nil {
			t.Fatal("verifyIDToken() error = nil, want issuer verification error")
		}
	})

	t.Run("token issued for another application", func(t *testing.T) {
		claims := validClaims
		claims.Audience = "another-application-client-id"
		rawIDToken := signIDToken(t, trustedPrivateKey, claims)

		if _, err := client.verifyIDToken(t.Context(), rawIDToken, expectedNonce); err == nil {
			t.Fatal("verifyIDToken() error = nil, want audience verification error")
		}
	})

	t.Run("expired token", func(t *testing.T) {
		claims := validClaims
		claims.Expiration = time.Now().Add(-time.Hour).Unix()
		rawIDToken := signIDToken(t, trustedPrivateKey, claims)

		if _, err := client.verifyIDToken(t.Context(), rawIDToken, expectedNonce); err == nil {
			t.Fatal("verifyIDToken() error = nil, want expiration verification error")
		}
	})
}

type testIDTokenClaims struct {
	Issuer        string `json:"iss"`
	Audience      string `json:"aud"`
	Subject       string `json:"sub"`
	Nonce         string `json:"nonce"`
	Expiration    int64  `json:"exp"`
	IssuedAt      int64  `json:"iat"`
	Email         string `json:"email"`
	EmailVerified bool   `json:"email_verified"`
}

func generateRSAKey(t *testing.T) *rsa.PrivateKey {
	t.Helper()

	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generate RSA key: %v", err)
	}

	return privateKey
}

func signIDToken(t *testing.T, privateKey *rsa.PrivateKey, claims testIDTokenClaims) string {
	t.Helper()

	payload, err := json.Marshal(claims)
	if err != nil {
		t.Fatalf("marshal ID token claims: %v", err)
	}

	signer, err := jose.NewSigner(jose.SigningKey{Algorithm: jose.RS256, Key: privateKey}, nil)
	if err != nil {
		t.Fatalf("create ID token signer: %v", err)
	}

	signedToken, err := signer.Sign(payload)
	if err != nil {
		t.Fatalf("sign ID token: %v", err)
	}

	rawIDToken, err := signedToken.CompactSerialize()
	if err != nil {
		t.Fatalf("serialize ID token: %v", err)
	}

	return rawIDToken
}
