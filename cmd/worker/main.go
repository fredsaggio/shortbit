package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"sync"
	"syscall"

	"github.com/fredsaggio/url-shortener/internal/config"
	"github.com/fredsaggio/url-shortener/internal/db"
	"github.com/fredsaggio/url-shortener/internal/jobs"
	"github.com/fredsaggio/url-shortener/internal/messaging"
	"github.com/fredsaggio/url-shortener/internal/repositories"
	"github.com/fredsaggio/url-shortener/internal/services"
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

	clickEventRepository := repositories.NewClickEventRepository(pool)
	clickCountService := services.NewClickCountService(clickEventRepository)
	consumer := messaging.NewClickConsumer(cfg.RabbitMQURL, cfg.RabbitMQPrefetch, clickCountService)
	cleanup := jobs.NewProcessedClickEventCleanup(clickEventRepository, cfg.ProcessedClickEventRetention)

	var wg sync.WaitGroup

	wg.Go(func() {
		slog.Info("click worker started")
		consumer.Run(ctx)
		slog.Info("click worker stopped")
	})

	wg.Go(func() {
		cleanup.Run(ctx)
	})

	wg.Wait()

	return nil
}
