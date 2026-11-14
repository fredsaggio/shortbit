package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/fredsaggio/url-shortener/internal/app"
	"github.com/fredsaggio/url-shortener/internal/config"
	"github.com/fredsaggio/url-shortener/internal/db"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	slog.SetDefault(logger)

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGTERM, syscall.SIGINT)
	defer cancel()
	if err := Run(ctx, os.Getenv); err != nil {
		slog.Error("worker stopped", "error", err)
		os.Exit(1)
	}
}

func Run(ctx context.Context, getEnv func(string) string) error {
	cfg, err := config.LoadWorker(getEnv)
	if err != nil {
		return err
	}

	pool, err := db.Connect(ctx, cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer pool.Close()

	app.RunClickWorker(ctx, pool, cfg.RabbitMQURL, cfg.RabbitMQPrefetch, cfg.ProcessedClickEventRetention)

	return nil
}
