package googleoidc

import (
	"context"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/coreos/go-oidc/v3/oidc"
	"github.com/fredsaggio/url-shortener/internal/config"
	"github.com/fredsaggio/url-shortener/internal/services"
	jose "github.com/go-jose/go-jose/v4"
	"golang.org/x/oauth2"
)

func TestNew(t *testing.T) {
	const (
		clientID     = "application-client-id"
		clientSecret = "application-client-secret"
		redirectURL  = "http://localhost:8080/auth/google/callback"
		authURL      = "https://accounts.google.com/o/oauth2/v2/auth"
		tokenURL     = "https://oauth2.googleapis.com/token"
	)

	httpClient := &http.Client{
		Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
			wantURL := googleIssuer + "/.well-known/openid-configuration"
			if request.URL.String() != wantURL {
				t.Errorf("discovery URL = %q, want %q", request.URL.String(), wantURL)
			}

			discovery := `{
				"issuer":"https://accounts.google.com",
				"authorization_endpoint":"https://accounts.google.com/o/oauth2/v2/auth",
				"token_endpoint":"https://oauth2.googleapis.com/token",
				"jwks_uri":"https://www.googleapis.com/oauth2/v3/certs",
				"id_token_signing_alg_values_supported":["RS256"]
			}`

			return jsonResponse(request, http.StatusOK, discovery), nil
		}),
	}
	ctx := oidc.ClientContext(t.Context(), httpClient)

	client, err := New(ctx, config.GoogleConfig{ClientID: clientID, ClientSecret: clientSecret, RedirectURL: redirectURL})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	if client.oauth2Config.ClientID != clientID {
		t.Errorf("oauth2 client ID = %q, want %q", client.oauth2Config.ClientID, clientID)
	}
	if client.oauth2Config.ClientSecret != clientSecret {
		t.Errorf("oauth2 client secret = %q, want %q", client.oauth2Config.ClientSecret, clientSecret)
	}
	if client.oauth2Config.RedirectURL != redirectURL {
		t.Errorf("oauth2 redirect URL = %q, want %q", client.oauth2Config.RedirectURL, redirectURL)
	}
	if client.oauth2Config.Endpoint.AuthURL != authURL {
		t.Errorf("authorization endpoint = %q, want %q", client.oauth2Config.Endpoint.AuthURL, authURL)
	}
	if client.oauth2Config.Endpoint.TokenURL != tokenURL {
		t.Errorf("token endpoint = %q, want %q", client.oauth2Config.Endpoint.TokenURL, tokenURL)
	}
	if client.tokenVerifier == nil {
		t.Fatal("token verifier = nil")
	}
}

func TestNewPropagatesDiscoveryError(t *testing.T) {
	httpClient := &http.Client{
		Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
			return jsonResponse(request, http.StatusServiceUnavailable, `{"error":"unavailable"}`), nil
		}),
	}
	ctx := oidc.ClientContext(t.Context(), httpClient)

	_, err := New(ctx, config.GoogleConfig{ClientID: "client-id", ClientSecret: "client-secret", RedirectURL: "http://localhost/callback"})
	if err == nil {
		t.Fatal("New() error = nil, want discovery error")
	}
}

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

func TestClientExchangeCode(t *testing.T) {
	const (
		clientID        = "application-client-id"
		clientSecret    = "application-client-secret"
		redirectURL     = "http://localhost:8080/auth/google/callback"
		code            = "authorization-code"
		codeVerifier    = "original-code-verifier"
		wantIDToken     = "raw-id-token"
		wantAccessToken = "google-access-token"
	)

	tokenEndpoint := "https://oauth.example.test/token"
	ctx := context.WithValue(t.Context(), oauth2.HTTPClient, &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		if request.Method != http.MethodPost {
			t.Errorf("token endpoint method = %s, want POST", request.Method)
		}
		if err := request.ParseForm(); err != nil {
			t.Fatalf("parse token request: %v", err)
		}

		wantForm := map[string]string{
			"grant_type":    "authorization_code",
			"code":          code,
			"code_verifier": codeVerifier,
			"client_id":     clientID,
			"client_secret": clientSecret,
			"redirect_uri":  redirectURL,
		}
		for name, want := range wantForm {
			if got := request.Form.Get(name); got != want {
				t.Errorf("token request field %q = %q, want %q", name, got, want)
			}
		}

		return jsonResponse(request, http.StatusOK, `{"access_token":"google-access-token","token_type":"Bearer","expires_in":3600,"id_token":"raw-id-token"}`), nil
	})})

	client := &Client{
		oauth2Config: oauth2.Config{
			ClientID:     clientID,
			ClientSecret: clientSecret,
			RedirectURL:  redirectURL,
			Endpoint: oauth2.Endpoint{
				TokenURL:  tokenEndpoint,
				AuthStyle: oauth2.AuthStyleInParams,
			},
		},
	}

	token, err := client.exchangeCode(ctx, code, codeVerifier)
	if err != nil {
		t.Fatalf("exchangeCode() error = %v", err)
	}

	if token.AccessToken != wantAccessToken {
		t.Errorf("access token = %q, want %q", token.AccessToken, wantAccessToken)
	}
	if got, _ := token.Extra("id_token").(string); got != wantIDToken {
		t.Errorf("ID token = %q, want %q", got, wantIDToken)
	}
}

func TestClientExchangeCodePropagatesTokenEndpointError(t *testing.T) {
	const tokenEndpoint = "https://oauth.example.test/token"
	ctx := context.WithValue(t.Context(), oauth2.HTTPClient, &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		return jsonResponse(request, http.StatusBadRequest, `{"error":"invalid_grant"}`), nil
	})})

	client := &Client{
		oauth2Config: oauth2.Config{
			ClientID: "client-id",
			Endpoint: oauth2.Endpoint{TokenURL: tokenEndpoint, AuthStyle: oauth2.AuthStyleInParams},
		},
	}

	if _, err := client.exchangeCode(ctx, "invalid-code", "code-verifier"); err == nil {
		t.Fatal("exchangeCode() error = nil, want token endpoint error")
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

func TestIdentityFromIDToken(t *testing.T) {
	const (
		issuer   = "https://accounts.google.com"
		clientID = "application-client-id"
		nonce    = "expected-nonce"
	)

	privateKey := generateRSAKey(t)
	verifier := oidc.NewVerifier(
		issuer,
		&oidc.StaticKeySet{PublicKeys: []crypto.PublicKey{privateKey.Public()}},
		&oidc.Config{ClientID: clientID},
	)
	validClaims := testIDTokenClaims{
		Issuer:        issuer,
		Audience:      clientID,
		Subject:       "google-subject-123",
		Nonce:         nonce,
		Expiration:    time.Now().Add(time.Hour).Unix(),
		IssuedAt:      time.Now().Unix(),
		Email:         "user@example.com",
		EmailVerified: true,
	}

	t.Run("verified email and subject", func(t *testing.T) {
		idToken := verifyTestIDToken(t, verifier, signIDToken(t, privateKey, validClaims))

		identity, err := identityFromIDToken(idToken)
		if err != nil {
			t.Fatalf("identityFromIDToken() error = %v", err)
		}

		want := services.GoogleIdentity{Email: validClaims.Email, Subject: validClaims.Subject}
		if identity != want {
			t.Errorf("identityFromIDToken() = %+v, want %+v", identity, want)
		}
	})

	t.Run("email not verified", func(t *testing.T) {
		claims := validClaims
		claims.EmailVerified = false
		idToken := verifyTestIDToken(t, verifier, signIDToken(t, privateKey, claims))

		_, err := identityFromIDToken(idToken)
		if !errors.Is(err, ErrEmailNotVerified) {
			t.Fatalf("identityFromIDToken() error = %v, want %v", err, ErrEmailNotVerified)
		}
	})

	t.Run("missing subject", func(t *testing.T) {
		claims := validClaims
		claims.Subject = ""
		idToken := verifyTestIDToken(t, verifier, signIDToken(t, privateKey, claims))

		_, err := identityFromIDToken(idToken)
		if !errors.Is(err, ErrMissingIdentity) {
			t.Fatalf("identityFromIDToken() error = %v, want %v", err, ErrMissingIdentity)
		}
	})

	t.Run("missing email", func(t *testing.T) {
		claims := validClaims
		claims.Email = ""
		idToken := verifyTestIDToken(t, verifier, signIDToken(t, privateKey, claims))

		_, err := identityFromIDToken(idToken)
		if !errors.Is(err, ErrMissingIdentity) {
			t.Fatalf("identityFromIDToken() error = %v, want %v", err, ErrMissingIdentity)
		}
	})

	t.Run("email claim has invalid type", func(t *testing.T) {
		claims := map[string]any{
			"iss":            issuer,
			"aud":            clientID,
			"sub":            validClaims.Subject,
			"nonce":          nonce,
			"exp":            time.Now().Add(time.Hour).Unix(),
			"iat":            time.Now().Unix(),
			"email":          123,
			"email_verified": true,
		}
		idToken := verifyTestIDToken(t, verifier, signIDToken(t, privateKey, claims))

		if _, err := identityFromIDToken(idToken); err == nil {
			t.Fatal("identityFromIDToken() error = nil, want claims decoding error")
		}
	})
}

func TestClientExchangeAndVerify(t *testing.T) {
	const (
		issuer        = "https://accounts.google.com"
		clientID      = "application-client-id"
		clientSecret  = "application-client-secret"
		redirectURL   = "http://localhost:8080/auth/google/callback"
		code          = "authorization-code"
		codeVerifier  = "original-code-verifier"
		expectedNonce = "expected-nonce"
		email         = "user@example.com"
		subject       = "google-subject-123"
	)

	privateKey := generateRSAKey(t)
	rawIDToken := signIDToken(t, privateKey, testIDTokenClaims{
		Issuer:        issuer,
		Audience:      clientID,
		Subject:       subject,
		Nonce:         expectedNonce,
		Expiration:    time.Now().Add(time.Hour).Unix(),
		IssuedAt:      time.Now().Unix(),
		Email:         email,
		EmailVerified: true,
	})

	tokenEndpoint := "https://oauth.example.test/token"
	ctx := context.WithValue(t.Context(), oauth2.HTTPClient, &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		if err := request.ParseForm(); err != nil {
			t.Fatalf("parse token request: %v", err)
		}
		if got := request.Form.Get("code"); got != code {
			t.Errorf("authorization code = %q, want %q", got, code)
		}
		if got := request.Form.Get("code_verifier"); got != codeVerifier {
			t.Errorf("code verifier = %q, want %q", got, codeVerifier)
		}

		body, err := json.Marshal(map[string]any{
			"access_token": "google-access-token",
			"token_type":   "Bearer",
			"expires_in":   3600,
			"id_token":     rawIDToken,
		})
		if err != nil {
			t.Fatalf("encode token response: %v", err)
		}

		return jsonResponse(request, http.StatusOK, string(body)), nil
	})})

	client := &Client{
		oauth2Config: oauth2.Config{
			ClientID:     clientID,
			ClientSecret: clientSecret,
			RedirectURL:  redirectURL,
			Endpoint: oauth2.Endpoint{
				TokenURL:  tokenEndpoint,
				AuthStyle: oauth2.AuthStyleInParams,
			},
		},
		tokenVerifier: oidc.NewVerifier(
			issuer,
			&oidc.StaticKeySet{PublicKeys: []crypto.PublicKey{privateKey.Public()}},
			&oidc.Config{ClientID: clientID},
		),
	}

	identity, err := client.ExchangeAndVerify(ctx, code, expectedNonce, codeVerifier)
	if err != nil {
		t.Fatalf("ExchangeAndVerify() error = %v", err)
	}

	want := services.GoogleIdentity{Email: email, Subject: subject}
	if identity != want {
		t.Errorf("ExchangeAndVerify() = %+v, want %+v", identity, want)
	}
}

func TestClientExchangeAndVerifyPropagatesStepErrors(t *testing.T) {
	const (
		issuer        = "https://accounts.google.com"
		clientID      = "application-client-id"
		expectedNonce = "expected-nonce"
	)

	privateKey := generateRSAKey(t)
	verifier := oidc.NewVerifier(
		issuer,
		&oidc.StaticKeySet{PublicKeys: []crypto.PublicKey{privateKey.Public()}},
		&oidc.Config{ClientID: clientID},
	)
	newClient := func() *Client {
		return &Client{
			oauth2Config: oauth2.Config{
				ClientID: clientID,
				Endpoint: oauth2.Endpoint{
					TokenURL:  "https://oauth.example.test/token",
					AuthStyle: oauth2.AuthStyleInParams,
				},
			},
			tokenVerifier: verifier,
		}
	}

	t.Run("authorization code exchange", func(t *testing.T) {
		ctx := tokenResponseContext(t, http.StatusBadRequest, `{"error":"invalid_grant"}`)

		if _, err := newClient().ExchangeAndVerify(ctx, "invalid-code", expectedNonce, "code-verifier"); err == nil {
			t.Fatal("ExchangeAndVerify() error = nil, want code exchange error")
		}
	})

	t.Run("missing ID token", func(t *testing.T) {
		ctx := tokenResponseContext(t, http.StatusOK, `{"access_token":"access-token","token_type":"Bearer"}`)

		_, err := newClient().ExchangeAndVerify(ctx, "authorization-code", expectedNonce, "code-verifier")
		if !errors.Is(err, ErrMissingIDToken) {
			t.Fatalf("ExchangeAndVerify() error = %v, want %v", err, ErrMissingIDToken)
		}
	})

	t.Run("invalid ID token", func(t *testing.T) {
		ctx := tokenResponseContext(t, http.StatusOK, `{"access_token":"access-token","token_type":"Bearer","id_token":"not-a-jwt"}`)

		if _, err := newClient().ExchangeAndVerify(ctx, "authorization-code", expectedNonce, "code-verifier"); err == nil {
			t.Fatal("ExchangeAndVerify() error = nil, want ID token verification error")
		}
	})

	t.Run("invalid identity claims", func(t *testing.T) {
		rawIDToken := signIDToken(t, privateKey, testIDTokenClaims{
			Issuer:        issuer,
			Audience:      clientID,
			Subject:       "google-subject-123",
			Nonce:         expectedNonce,
			Expiration:    time.Now().Add(time.Hour).Unix(),
			IssuedAt:      time.Now().Unix(),
			Email:         "user@example.com",
			EmailVerified: false,
		})
		body, err := json.Marshal(map[string]any{
			"access_token": "access-token",
			"token_type":   "Bearer",
			"id_token":     rawIDToken,
		})
		if err != nil {
			t.Fatalf("encode token response: %v", err)
		}
		ctx := tokenResponseContext(t, http.StatusOK, string(body))

		_, err = newClient().ExchangeAndVerify(ctx, "authorization-code", expectedNonce, "code-verifier")
		if !errors.Is(err, ErrEmailNotVerified) {
			t.Fatalf("ExchangeAndVerify() error = %v, want %v", err, ErrEmailNotVerified)
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

func signIDToken(t *testing.T, privateKey *rsa.PrivateKey, claims any) string {
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

func verifyTestIDToken(t *testing.T, verifier *oidc.IDTokenVerifier, rawIDToken string) *oidc.IDToken {
	t.Helper()

	idToken, err := verifier.Verify(t.Context(), rawIDToken)
	if err != nil {
		t.Fatalf("verify test ID token: %v", err)
	}

	return idToken
}

type roundTripFunc func(request *http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return f(request)
}

func jsonResponse(request *http.Request, statusCode int, body string) *http.Response {
	return &http.Response{
		StatusCode: statusCode,
		Status:     http.StatusText(statusCode),
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(strings.NewReader(body)),
		Request:    request,
	}
}

func tokenResponseContext(t *testing.T, statusCode int, body string) context.Context {
	t.Helper()

	httpClient := &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		return jsonResponse(request, statusCode, body), nil
	})}

	return context.WithValue(t.Context(), oauth2.HTTPClient, httpClient)
}
