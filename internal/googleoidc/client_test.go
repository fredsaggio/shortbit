package googleoidc

import (
	"crypto/sha256"
	"encoding/base64"
	"net/url"
	"testing"

	"github.com/coreos/go-oidc/v3/oidc"
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
