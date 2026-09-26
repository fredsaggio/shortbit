package services

import (
	"context"
	"errors"
	"fmt"
	"uuid"

	"github.com/fredsaggio/url-shortener/internal/models"
	"github.com/fredsaggio/url-shortener/internal/repositories"
)

const googleProvider = "google"

var ErrGoogleAuthenticationFailed = errors.New("error in google authentication")

type GoogleIdentity struct {
	// Email é o endereço confirmado pelo provedor de identidade.
	Email string
	// Subject é o claim "sub" do ID Token. Ele identifica o usuário de forma
	// estável dentro do Google, mesmo que o endereço de e-mail seja alterado.
	Subject string
}

// GoogleOIDCClient define a parte do protocolo OpenID Connect que o service
// precisa. A implementação concreta conversa com o Google; os testes usam um stub.
type GoogleOIDCClient interface {
	// AuthorizationURL monta a URL que inicia o login no Google. state liga o
	// callback ao navegador, nonce liga o futuro ID Token a esta tentativa e
	// codeVerifier é o segredo temporário usado pelo PKCE.
	AuthorizationURL(state, nonce, codeVerifier string) string
	// ExchangeAndVerify troca o authorization code por tokens, valida o ID Token
	// e devolve apenas a identidade já verificada.
	ExchangeAndVerify(ctx context.Context, code, expectedNonce, codeVerifier string) (GoogleIdentity, error)
}

// GoogleUserRepository descreve somente as operações de usuário necessárias
// ao login com Google. A implementação concreta continua sendo UserRepository.
type GoogleUserRepository interface {
	CreateWithIdentity(ctx context.Context, email, provider, providerUserID string) (models.User, error)
	FindUserByProviderIdentity(ctx context.Context, provider, providerUserID string) (models.User, error)
	LinkIdentityToPasswordUserByEmail(ctx context.Context, email, provider, providerUserID string) (models.User, error)
}

type UserSessionCreator interface {
	CreateSession(ctx context.Context, userID uuid.UUID, rememberMe bool) (LoginResult, error)
}

type GoogleAuthService struct {
	oidcClient     GoogleOIDCClient
	googleRepo     GoogleUserRepository
	sessionCreator UserSessionCreator
}

func NewGoogleAuthService(userRepo GoogleUserRepository, sessionCreator UserSessionCreator, oidcClient GoogleOIDCClient) *GoogleAuthService {
	return &GoogleAuthService{
		googleRepo:     userRepo,
		sessionCreator: sessionCreator,
		oidcClient:     oidcClient,
	}
}

func (s *GoogleAuthService) AuthorizationURL(state, nonce, codeVerifier string) string {
	return s.oidcClient.AuthorizationURL(state, nonce, codeVerifier)
}

func (s *GoogleAuthService) CompleteLogin(ctx context.Context, code, expectedNonce, codeVerifier string) (LoginResult, error) {
	identity, err := s.oidcClient.ExchangeAndVerify(ctx, code, expectedNonce, codeVerifier)

	if err != nil {
		return LoginResult{}, fmt.Errorf("verify Google identity: %w", err)
	}

	// A fronteira OIDC chama o identificador de Subject. Daqui em diante usamos
	// providerUserID, o nome genérico aceito pelo domínio e pelo repository.
	return s.loginWithProvider(ctx, identity.Email, identity.Subject)
}

func (s *GoogleAuthService) loginWithProvider(ctx context.Context, email, providerUserID string) (LoginResult, error) {
	user, err := s.googleRepo.FindUserByProviderIdentity(ctx, googleProvider, providerUserID)

	if err == nil {
		return s.sessionCreator.CreateSession(ctx, user.ID, false)
	}

	if !errors.Is(err, repositories.ErrAuthIdentityNotFound) {
		return LoginResult{}, fmt.Errorf("find user by Google identity: %w", err)
	}

	normalizedEmail := normalizeEmail(email)
	if !isValidEmail(normalizedEmail) {
		return LoginResult{}, ErrInvalidEmail
	}

	user, err = s.googleRepo.LinkIdentityToPasswordUserByEmail(ctx, normalizedEmail, googleProvider, providerUserID)

	switch {
	case err == nil:

	case errors.Is(err, repositories.ErrAuthIdentityAlreadyExists):
		user, err = s.googleRepo.FindUserByProviderIdentity(ctx, googleProvider, providerUserID)
		if err != nil {
			return LoginResult{}, fmt.Errorf("find concurrently linked Google identity: %w", err)
		}

	case errors.Is(err, repositories.ErrProviderAlreadyLinked):
		return LoginResult{}, ErrGoogleAuthenticationFailed

	case errors.Is(err, repositories.ErrPasswordAccountNotFound):
		user, err = s.googleRepo.CreateWithIdentity(ctx, normalizedEmail, googleProvider, providerUserID)

		switch {
		case err == nil:

		case errors.Is(err, repositories.ErrEmailAlreadyExists):
			user, err = s.googleRepo.FindUserByProviderIdentity(ctx, googleProvider, providerUserID)

			switch {
			case err == nil:

			case errors.Is(err, repositories.ErrAuthIdentityNotFound):
				user, err = s.googleRepo.LinkIdentityToPasswordUserByEmail(ctx, normalizedEmail, googleProvider, providerUserID)

				switch {
				case err == nil:

				case errors.Is(err, repositories.ErrAuthIdentityAlreadyExists):
					user, err = s.googleRepo.FindUserByProviderIdentity(ctx, googleProvider, providerUserID)
					if err != nil {
						return LoginResult{}, fmt.Errorf("find concurrently linked Google identity after email conflict: %w", err)
					}

				case errors.Is(err, repositories.ErrPasswordAccountNotFound),
					errors.Is(err, repositories.ErrProviderAlreadyLinked):
					return LoginResult{}, ErrGoogleAuthenticationFailed

				default:
					return LoginResult{}, fmt.Errorf("link Google identity after email conflict: %w", err)
				}

			default:
				return LoginResult{}, fmt.Errorf("find Google identity after email conflict: %w", err)
			}

		default:
			return LoginResult{}, fmt.Errorf("create user with Google identity: %w", err)
		}

	default:
		return LoginResult{}, fmt.Errorf("link Google identity to password account: %w", err)
	}

	return s.sessionCreator.CreateSession(ctx, user.ID, false)
}
