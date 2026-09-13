package app

import (
	"net/http"

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

func CompositionRoot(pool db.DB, sessionConfig config.SessionConfig) *server.Handlers {
	passwordHasher := argon2.Argon2id{}

	userRepository := repositories.NewUserRepository(pool)
	userService := services.NewUserService(userRepository, passwordHasher)
	userHandler := handlers.NewUserHandler(userService)

	userSessionRepository := repositories.NewUserSessionRepository(pool)
	authService := services.NewAuthService(userRepository, userSessionRepository, passwordHasher, sessiontoken.Generate, sessionConfig.TTL)
	sessionHandler := handlers.NewSessionHandler(authService, sessionConfig.CookieSecure)

	meHandler := middleware.Authenticator(authService)(http.HandlerFunc(userHandler.GetUserInfo))

	return &server.Handlers{
		UserHandler:    userHandler,
		SessionHandler: sessionHandler,
		MeHandler:      meHandler,
	}
}
