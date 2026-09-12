package app

import (
	"github.com/fredsaggio/url-shortener/internal/db"
	"github.com/fredsaggio/url-shortener/internal/handlers"
	"github.com/fredsaggio/url-shortener/internal/password"
	"github.com/fredsaggio/url-shortener/internal/repositories"
	"github.com/fredsaggio/url-shortener/internal/server"
	"github.com/fredsaggio/url-shortener/internal/services"
)

func CompositionRoot(pool db.DB) *server.Handlers {
	passwordHasher := password.Argon2id{}

	userRepository := repositories.NewUserRepository(pool)
	userService := services.NewUserService(userRepository, passwordHasher)
	userHandler := handlers.NewUserHandler(userService)

	return &server.Handlers{
		UserHandler: userHandler,
	}
}
