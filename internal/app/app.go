package app

import (
	"github.com/fredsaggio/url-shortener/internal/db"
	"github.com/fredsaggio/url-shortener/internal/server"
)

func CompositionRoot(pool db.DB) *server.Handlers {
	return &server.Handlers{}
}
