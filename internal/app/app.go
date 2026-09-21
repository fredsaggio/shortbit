package app

import (
	"net/http"
	"time"

	"github.com/fredsaggio/url-shortener/internal/argon2"
	"github.com/fredsaggio/url-shortener/internal/config"
	"github.com/fredsaggio/url-shortener/internal/db"
	"github.com/fredsaggio/url-shortener/internal/handlers"
	"github.com/fredsaggio/url-shortener/internal/middleware"
	"github.com/fredsaggio/url-shortener/internal/repositories"
	"github.com/fredsaggio/url-shortener/internal/server"
	"github.com/fredsaggio/url-shortener/internal/services"
	"github.com/fredsaggio/url-shortener/internal/sessiontoken"
)
// Tenho que atualizar aqui esse composition root, é o próximo passo caso você tenha esquecido.
const (
	loginEmailRateLimitRequestsPerSecond = 10.0 / 60.0
	loginEmailRateLimitBurst             = 5
	loginEmailRateLimitMaxEntries        = 10_000
	loginEmailRateLimitStaleAfter        = 10 * time.Minute
)

func CompositionRoot(pool db.DB, sessionConfig config.SessionConfig) *server.Handlers {
	passwordHasher := argon2.Argon2id{}

	userRepository := repositories.NewUserRepository(pool)
	userService := services.NewUserService(userRepository, passwordHasher)
	userHandler := handlers.NewUserHandler(userService)

	userSessionRepository := repositories.NewUserSessionRepository(pool)
	authService := services.NewAuthService(userRepository, userSessionRepository, passwordHasher, sessiontoken.Generate, sessionConfig.TTL)
	loginEmailRateLimiter := middleware.NewRateLimiter(loginEmailRateLimitRequestsPerSecond, loginEmailRateLimitBurst, loginEmailRateLimitMaxEntries, loginEmailRateLimitStaleAfter)
	sessionHandler := handlers.NewSessionHandler(authService, loginEmailRateLimiter, sessionConfig.CookieSecure)

	meHandler := middleware.Authenticator(authService)(http.HandlerFunc(userHandler.GetUserInfo))

	return &server.Handlers{
		UserHandler:    userHandler,
		SessionHandler: sessionHandler,
		MeHandler:      meHandler,
	}
}
