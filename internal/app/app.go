package app

import (
	"fmt"
	"net/http"
	"time"

	"github.com/fredsaggio/url-shortener/internal/argon2"
	"github.com/fredsaggio/url-shortener/internal/config"
	"github.com/fredsaggio/url-shortener/internal/db"
	"github.com/fredsaggio/url-shortener/internal/email"
	"github.com/fredsaggio/url-shortener/internal/googleoidc"
	"github.com/fredsaggio/url-shortener/internal/handlers"
	"github.com/fredsaggio/url-shortener/internal/jobs"
	"github.com/fredsaggio/url-shortener/internal/middleware"
	"github.com/fredsaggio/url-shortener/internal/repositories"
	"github.com/fredsaggio/url-shortener/internal/server"
	"github.com/fredsaggio/url-shortener/internal/services"
	"github.com/fredsaggio/url-shortener/internal/sessiontoken"
	"github.com/fredsaggio/url-shortener/internal/shortcodes"
	"github.com/fredsaggio/url-shortener/internal/verificationcode"
)

const (
	loginEmailRateLimitRequestsPerSecond = 10.0 / 60.0
	loginEmailRateLimitBurst             = 5
	loginEmailRateLimitMaxEntries        = 10_000
	loginEmailRateLimitStaleAfter        = 10 * time.Minute

	registrationEmailRateLimitRequestsPerSecond = 3.0 / (10.0 * 60.0)
	registrationEmailRateLimitBurst             = 3
	registrationEmailRateLimitMaxEntries        = 10_000
	registrationEmailRateLimitStaleAfter        = 30 * time.Minute

	passwordResetEmailRateLimitRequestsPerSecond = 3.0 / (10.0 * 60.0)
	passwordResetEmailRateLimitBurst             = 3
	passwordResetEmailRateLimitMaxEntries        = 10_000
	passwordResetEmailRateLimitStaleAfter        = 30 * time.Minute
)

func CompositionRoot(pool db.DB, cfg config.Config, googleClient services.GoogleOIDCClient) (*server.Handlers, *jobs.PasswordRegistrationCleanup, error) {
	passwordHasher := argon2.Argon2id{}
	userRepository := repositories.NewUserRepository(pool)
	passwordRegisterCleanup := jobs.NewPasswordRegistrationCleanup(userRepository)

	codeSender := email.NewResendSender(cfg.Resend.APIKey, cfg.Email.From)

	userService := services.NewUserService(userRepository,
		passwordHasher,
		sessiontoken.Generate,
		verificationcode.Generate,
		codeSender,
		services.PasswordRegistrationConfig{CodeTTL: cfg.PasswordRegistration.CodeTTL, AttemptTTL: cfg.PasswordRegistration.AttemptTTL},
	)
	registrationEmailRateLimiter := middleware.NewRateLimiter(registrationEmailRateLimitRequestsPerSecond, registrationEmailRateLimitBurst, registrationEmailRateLimitMaxEntries, registrationEmailRateLimitStaleAfter)
	userHandler := handlers.NewUserHandler(userService, registrationEmailRateLimiter, cfg.Session.CookieSecure)
	passwordResetService := services.NewPasswordResetService(userRepository, passwordHasher, codeSender, sessiontoken.Generate, verificationcode.GeneratePasswordReset,
		services.PasswordResetConfig{CodeTTL: cfg.PasswordReset.CodeTTL, AttemptTTL: cfg.PasswordReset.AttemptTTL})
	passwordResetEmailRateLimiter := middleware.NewRateLimiter(passwordResetEmailRateLimitRequestsPerSecond, passwordResetEmailRateLimitBurst, passwordResetEmailRateLimitMaxEntries, passwordResetEmailRateLimitStaleAfter)
	passwordResetHandler := handlers.NewPasswordResetHandler(passwordResetService, passwordResetEmailRateLimiter, cfg.Session.CookieSecure)

	userSessionRepository := repositories.NewUserSessionRepository(pool)

	authService := services.NewAuthService(userRepository, userSessionRepository, passwordHasher, sessiontoken.Generate, cfg.Session.TTL, cfg.Session.RememberedTTL)

	googleAuthService := services.NewGoogleAuthService(userRepository, authService, googleClient)
	googleAuthHandler := handlers.NewGoogleAuthHandler(googleAuthService, googleoidc.GenerateAuthorizationValues, cfg.Session.CookieSecure)

	loginEmailRateLimiter := middleware.NewRateLimiter(loginEmailRateLimitRequestsPerSecond, loginEmailRateLimitBurst, loginEmailRateLimitMaxEntries, loginEmailRateLimitStaleAfter)
	sessionHandler := handlers.NewSessionHandler(authService, loginEmailRateLimiter, cfg.Session.CookieSecure)

	shortCodeGenerator, err := shortcodes.NewGenerator()
	if err != nil {
		return nil, nil, fmt.Errorf("initialize shortcode generator: %w", err)
	}
	urlRepository := repositories.NewURLRepository(pool)
	urlService, err := services.NewURLService(urlRepository, shortCodeGenerator, passwordHasher, services.URLServiceConfig{
		PublicBaseURL:       cfg.URL.BaseURL,
		MaxOriginalURLBytes: cfg.URL.MaxOriginalURLBytes,
	})
	if err != nil {
		return nil, nil, fmt.Errorf("initialize URL service: %w", err)
	}
	urlHandler := handlers.NewURLHandler(urlService)

	createURLHandler := middleware.Authenticator(authService)(
		http.HandlerFunc(urlHandler.Create),
	)
	listURLHandler := middleware.Authenticator(authService)(
		http.HandlerFunc(urlHandler.List),
	)

	meHandler := middleware.Authenticator(authService)(http.HandlerFunc(userHandler.GetUserInfo))

	return &server.Handlers{
		UserHandler:          userHandler,
		SessionHandler:       sessionHandler,
		GoogleAuthHandler:    googleAuthHandler,
		PasswordResetHandler: passwordResetHandler,
		CreateURLHandler:     createURLHandler,
		ListURLHandler:       listURLHandler,
		MeHandler:            meHandler,
	}, passwordRegisterCleanup, nil
}
