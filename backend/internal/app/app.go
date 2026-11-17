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
	"github.com/fredsaggio/url-shortener/internal/visits"
)

const (
	// Per account: 5 attempts at once, then 5 more every 15 minutes.
	passwordChangeRateLimitRequestsPerSecond = 5.0 / (15.0 * 60.0)
	passwordChangeRateLimitBurst             = 5
	passwordChangeRateLimitMaxEntries        = 10_000
	passwordChangeRateLimitStaleAfter        = 30 * time.Minute

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

func CompositionRoot(pool db.DB, cfg config.Config, googleClient services.GoogleOIDCClient, heatTracker services.HeatTracker, destinationCache services.DestinationCache, clickPublisher services.ClickPublisher) (*server.Handlers, *jobs.PasswordRegistrationCleanup, error) {
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
	registrationEmailRateLimiter := middleware.NewRateLimiter(registrationEmailRateLimitRequestsPerSecond, registrationEmailRateLimitBurst, registrationEmailRateLimitMaxEntries, registrationEmailRateLimitStaleAfter).Named("registration_email")
	userHandler := handlers.NewUserHandler(userService, registrationEmailRateLimiter, cfg.Session.CookieSecure)
	passwordResetService := services.NewPasswordResetService(userRepository, passwordHasher, codeSender, sessiontoken.Generate, verificationcode.GeneratePasswordReset,
		services.PasswordResetConfig{CodeTTL: cfg.PasswordReset.CodeTTL, AttemptTTL: cfg.PasswordReset.AttemptTTL})
	passwordResetEmailRateLimiter := middleware.NewRateLimiter(passwordResetEmailRateLimitRequestsPerSecond, passwordResetEmailRateLimitBurst, passwordResetEmailRateLimitMaxEntries, passwordResetEmailRateLimitStaleAfter).Named("password_reset_email")
	passwordResetHandler := handlers.NewPasswordResetHandler(passwordResetService, passwordResetEmailRateLimiter, cfg.Session.CookieSecure)

	userSessionRepository := repositories.NewUserSessionRepository(pool)

	authService := services.NewAuthService(userRepository, userSessionRepository, passwordHasher, sessiontoken.Generate, cfg.Session.TTL, cfg.Session.RememberedTTL)

	googleAuthService := services.NewGoogleAuthService(userRepository, authService, googleClient)
	googleAuthHandler := handlers.NewGoogleAuthHandler(googleAuthService, googleoidc.GenerateAuthorizationValues, cfg.Session.CookieSecure, cfg.URL.FrontendURL)

	loginEmailRateLimiter := middleware.NewRateLimiter(loginEmailRateLimitRequestsPerSecond, loginEmailRateLimitBurst, loginEmailRateLimitMaxEntries, loginEmailRateLimitStaleAfter).Named("login_email")
	sessionHandler := handlers.NewSessionHandler(authService, loginEmailRateLimiter, cfg.Session.CookieSecure)

	shortCodeGenerator, err := shortcodes.NewGenerator()
	if err != nil {
		return nil, nil, fmt.Errorf("initialize shortcode generator: %w", err)
	}
	urlRepository := repositories.NewURLRepository(pool)
	urlService, err := services.NewURLService(urlRepository, shortCodeGenerator, passwordHasher, destinationCache, services.URLServiceConfig{
		PublicBaseURL:       cfg.URL.BaseURL,
		MaxOriginalURLBytes: cfg.URL.MaxOriginalURLBytes,
	})
	if err != nil {
		return nil, nil, fmt.Errorf("initialize URL service: %w", err)
	}

	// O cliente Redis é criado na main, que é responsável por fechá-lo ao desligar a aplicação.
	clickEventRepository := repositories.NewClickEventRepository(pool)
	visitAnalyzer, err := newVisitAnalyzer(cfg.Visits, repositories.NewVisitorSaltRepository(pool))
	if err != nil {
		return nil, nil, err
	}
	redirectService := services.NewRedirectService(urlRepository, urlRepository, heatTracker, destinationCache, cfg.HotURL.Threshold, clickPublisher, clickEventRepository, visitAnalyzer)
	urlHandler := handlers.NewURLHandler(urlService, redirectService)
	linkAccessSessionRepository := repositories.NewLinkAccessSessionRepository(pool)
	linkAccessSessionService := services.NewLinkAccessSessionService(urlRepository, linkAccessSessionRepository, cfg.LinkAccessSession.TTL, sessiontoken.Generate, passwordHasher)
	linkAccessSessionHandler := handlers.NewLinkAccessSessionHandler(linkAccessSessionService, cfg.Session.CookieSecure)

	createURLHandler := middleware.Authenticator(authService)(
		http.HandlerFunc(urlHandler.Create),
	)
	listURLHandler := middleware.Authenticator(authService)(
		http.HandlerFunc(urlHandler.List),
	)
	getURLHandler := middleware.Authenticator(authService)(
		http.HandlerFunc(urlHandler.GetByShortCode),
	)
	updateURLHandler := middleware.Authenticator(authService)(http.HandlerFunc(urlHandler.Update))
	deleteURLHandler := middleware.Authenticator(authService)(http.HandlerFunc(urlHandler.Delete))
	clickStatsService, err := services.NewClickStatsService(urlRepository)
	if err != nil {
		return nil, nil, fmt.Errorf("initialize click stats service: %w", err)
	}
	clickStatsHandler := middleware.Authenticator(authService)(http.HandlerFunc(handlers.NewClickStatsHandler(clickStatsService).DailyClicks))

	accountService := services.NewAccountService(userRepository, userSessionRepository, passwordHasher, sessiontoken.Generate, cfg.Session.TTL, destinationCache)
	passwordChangeRateLimiter := middleware.NewRateLimiter(passwordChangeRateLimitRequestsPerSecond, passwordChangeRateLimitBurst, passwordChangeRateLimitMaxEntries, passwordChangeRateLimitStaleAfter).Named("account_password")
	accountHandler := handlers.NewAccountHandler(accountService, passwordChangeRateLimiter, cfg.Session.CookieSecure)
	meHandler := middleware.Authenticator(authService)(http.HandlerFunc(accountHandler.Get))
	updateMeHandler := middleware.Authenticator(authService)(http.HandlerFunc(accountHandler.Update))
	changePasswordHandler := middleware.Authenticator(authService)(http.HandlerFunc(accountHandler.ChangePassword))
	deleteMeHandler := middleware.Authenticator(authService)(http.HandlerFunc(accountHandler.Delete))

	return &server.Handlers{
		UserHandler:              userHandler,
		SessionHandler:           sessionHandler,
		GoogleAuthHandler:        googleAuthHandler,
		PasswordResetHandler:     passwordResetHandler,
		URLHandler:               urlHandler,
		LinkAccessSessionHandler: linkAccessSessionHandler,
		CreateURLHandler:         createURLHandler,
		ListURLHandler:           listURLHandler,
		GetURLHandler:            getURLHandler,
		UpdateURLHandler:         updateURLHandler,
		DeleteURLHandler:         deleteURLHandler,
		ClickStatsHandler:        clickStatsHandler,
		MeHandler:                meHandler,
		UpdateMeHandler:          updateMeHandler,
		ChangePasswordHandler:    changePasswordHandler,
		DeleteMeHandler:          deleteMeHandler,
	}, passwordRegisterCleanup, nil
}

// newVisitAnalyzer opens the country database, which stays open for the life
// of the process. Without GEOIP_DB_PATH clicks have no country.
func newVisitAnalyzer(cfg config.VisitsConfig, salts visits.SaltStore) (*visits.Analyzer, error) {
	location, err := time.LoadLocation(services.ClickDayZone)
	if err != nil {
		return nil, fmt.Errorf("load %s: %w", services.ClickDayZone, err)
	}

	var countries visits.CountryLocator
	if cfg.CountryDBPath != "" {
		db, err := visits.OpenCountryDB(cfg.CountryDBPath)
		if err != nil {
			return nil, fmt.Errorf("GEOIP_DB_PATH: %w", err)
		}
		countries = db
	}
	return visits.NewAnalyzer(countries, salts, location), nil
}
