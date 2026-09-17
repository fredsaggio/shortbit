package googleoidc

import (
	"context"
	"errors"
	"fmt"

	"github.com/coreos/go-oidc/v3/oidc"
	"github.com/fredsaggio/url-shortener/internal/config"
	"github.com/fredsaggio/url-shortener/internal/services"
	"golang.org/x/oauth2"
)

// googleIssuer é a raiz de confiança do cliente OIDC. A biblioteca busca a
// configuração pública do Google a partir deste endereço e só aceita tokens
// cujo claim "iss" identifica esse emissor.
const googleIssuer = "https://accounts.google.com"

var (
	ErrMissingIDToken   = errors.New("Google response does not contain an ID token")
	ErrNonceMismatch    = errors.New("Google ID token nonce does not match")
	ErrEmailNotVerified = errors.New("Google email is not verified")
	ErrMissingIdentity  = errors.New("Google identity is missing required fields")
)

type Client struct {
	// oauth2Config conhece o ClientID, ClientSecret, RedirectURL e endpoints.
	// Ele monta a URL de autorização e faz a troca do authorization code.
	oauth2Config  oauth2.Config
	tokenVerifier *oidc.IDTokenVerifier
}

var _ services.GoogleOIDCClient = (*Client)(nil)

type identityClaims struct {
	Email         string `json:"email"`
	EmailVerified bool   `json:"email_verified"`
}

// New prepara um cliente que será criado uma vez na inicialização da API e
// reutilizado em todos os logins. NewProvider faz uma requisição ao documento
// de discovery do Google para obter endpoints, algoritmos e o endereço das
// chaves públicas usadas na verificação das assinaturas.
func New(ctx context.Context, cfg config.GoogleConfig) (*Client, error) {
	provider, err := oidc.NewProvider(ctx, googleIssuer)

	if err != nil {
		return nil, fmt.Errorf("discover Google OIDC provider: %w", err)
	}

	oauth2Config := oauth2.Config{
		// ClientID identifica publicamente nossa aplicação perante o Google.
		ClientID: cfg.ClientID,
		// ClientSecret autentica nosso backend durante a troca do code. Ele não
		// deve ser enviado ao navegador.
		ClientSecret: cfg.ClientSecret,
		// RedirectURL é o callback para o qual o navegador será devolvido.
		RedirectURL: cfg.RedirectURL,
		// Endpoint contém as URLs de autorização e de troca de tokens descobertas
		// no documento OIDC do Google.
		Endpoint: provider.Endpoint(),
		// openid solicita um ID Token; email solicita os claims de e-mail. Não
		// pedimos profile porque atualmente não precisamos de nome nem foto.
		Scopes: []string{oidc.ScopeOpenID, oidc.ScopeEmail},
	}

	tokenVerifier := provider.Verifier(&oidc.Config{
		ClientID: cfg.ClientID,
	})

	return &Client{
		oauth2Config:  oauth2Config,
		tokenVerifier: tokenVerifier,
	}, nil
}

// AuthorizationURL monta a URL para a qual o handler redirecionará o navegador.
//
// state é um valor aleatório devolvido no callback e protege a ligação entre o
// navegador que iniciou o login e o callback recebido.
//
// nonce é outro valor aleatório, mas ele volta dentro do ID Token assinado e
// liga esse token à tentativa de login atual.
//
// codeVerifier é o segredo temporário do PKCE. O valor original fica conosco;
// nesta URL a biblioteca envia apenas SHA-256(codeVerifier), chamado challenge.
func (c *Client) AuthorizationURL(state, nonce, codeVerifier string) string {
	// AccessTypeOnline evita pedir um refresh token, pois usamos o Google apenas
	// para autenticar e depois mantemos nossa própria sessão.
	return c.oauth2Config.AuthCodeURL(state, oidc.Nonce(nonce), oauth2.S256ChallengeOption(codeVerifier), oauth2.AccessTypeOnline)
}

// ExchangeAndVerify conclui a parte OIDC do callback. Cada parâmetro representa
// uma credencial ou verificação diferente:
//   - code: authorization code descartável enviado pelo Google no callback;
//   - expectedNonce: nonce guardado por nós quando o login foi iniciado;
//   - codeVerifier: segredo PKCE original que prova quem iniciou o fluxo.
//
// O resultado só é devolvido depois que o ID Token e o nonce forem validados.
func (c *Client) ExchangeAndVerify(ctx context.Context, code, expectedNonce, codeVerifier string) (services.GoogleIdentity, error) {
	// 1. Resgata o authorization code no token endpoint do Google. O retorno é
	// uma estrutura OAuth que pode conter vários tokens e metadados.
	token, err := c.exchangeCode(ctx, code, codeVerifier)

	if err != nil {
		return services.GoogleIdentity{}, err
	}

	// 2. O ID Token é uma extensão do OpenID Connect e fica nos campos extras da
	// resposta OAuth, sob a chave "id_token".
	rawIDToken, err := c.extractIDToken(token)

	if err != nil {
		return services.GoogleIdentity{}, err
	}

	// 3. rawIDToken ainda é uma string não confiável. A verificação da assinatura
	// e dos claims fundamentais acontece antes de lermos a identidade.
	idToken, err := c.verifyIDToken(ctx, rawIDToken, expectedNonce)

	if err != nil {
		return services.GoogleIdentity{}, err
	}

	// 4. Converte os claims já confiáveis para o pequeno tipo usado pelo service.
	return identityFromIDToken(idToken)
}

// exchangeCode faz a segunda requisição ao Google: uma chamada direta do nosso
// backend ao token endpoint. O code veio pelo callback; o codeVerifier ficou
// protegido conosco desde o início. O Google só aceita a troca se o verifier
// corresponder ao challenge enviado anteriormente.
func (c *Client) exchangeCode(ctx context.Context, code, codeVerifier string) (*oauth2.Token, error) {
	// O nome token não significa apenas access token: oauth2.Token é uma estrutura
	// que guarda access token, metadados e campos extras, incluindo o ID Token.
	token, err := c.oauth2Config.Exchange(ctx, code, oauth2.VerifierOption(codeVerifier))
	if err != nil {
		return nil, fmt.Errorf("exchange Google authorization code: %w", err)
	}

	return token, nil
}

// extractIDToken retira a string JWT chamada "id_token" dos campos extras da
// resposta OAuth. A extração não prova autenticidade; rawIDToken ainda precisa
// ser entregue ao tokenVerifier.
func (c *Client) extractIDToken(token *oauth2.Token) (string, error) {
	rawIDToken, ok := token.Extra("id_token").(string)

	if !ok || rawIDToken == "" {
		return "", ErrMissingIDToken
	}

	return rawIDToken, nil
}

// verifyIDToken transforma o JWT ainda não confiável em um IDToken validado.
// tokenVerifier confere assinatura, algoritmo, issuer, audience e expiração.
// Depois comparamos manualmente o nonce porque somente nossa aplicação conhece
// o valor esperado para esta tentativa específica de login.
func (c *Client) verifyIDToken(ctx context.Context, rawIDToken, expectedNonce string) (*oidc.IDToken, error) {
	idToken, err := c.tokenVerifier.Verify(ctx, rawIDToken)

	if err != nil {
		return nil, fmt.Errorf("verify Google ID token: %w", err)
	}

	if expectedNonce == "" || idToken.Nonce != expectedNonce {
		return nil, ErrNonceMismatch
	}

	return idToken, nil
}

// identityFromIDToken extrai os dados que nossa aplicação realmente usa. Esta
// função só deve receber um IDToken retornado por verifyIDToken.
func identityFromIDToken(idToken *oidc.IDToken) (services.GoogleIdentity, error) {
	var claims identityClaims

	if err := idToken.Claims(&claims); err != nil {
		return services.GoogleIdentity{}, fmt.Errorf("decode Google ID token claims: %w", err)
	}
	
	if !claims.EmailVerified {
		return services.GoogleIdentity{}, ErrEmailNotVerified
	}

	// Subject contém o claim "sub": identificador estável do usuário no Google.
	// O e-mail pode mudar, portanto nunca o usamos como identidade do provedor.
	if idToken.Subject == "" || claims.Email == "" {
		return services.GoogleIdentity{}, ErrMissingIdentity
	}

	return services.GoogleIdentity{
		Email:   claims.Email,
		Subject: idToken.Subject,
	}, nil
}
