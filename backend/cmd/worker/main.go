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
	"github.com/fredsaggio/url-shortener/internal/logging"
	"github.com/fredsaggio/url-shortener/internal/telemetry"
)

func main() {
	logger := logging.NewJSONLogger(os.Stdout)
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

	stopTelemetry, err := app.StartTelemetry(ctx, getEnv, "shortbit-worker")
	if err != nil {
		return err
	}
	defer stopTelemetry()

	pool, err := db.Connect(ctx, cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer pool.Close()
	if err := telemetry.ObserveDBPool(pool); err != nil {
		slog.Warn("database pool metrics unavailable", "error", err)
	}

	app.RunClickWorker(ctx, pool, cfg.RabbitMQURL, cfg.RabbitMQPrefetch, cfg.ProcessedClickEventRetention)

	return nil
}
