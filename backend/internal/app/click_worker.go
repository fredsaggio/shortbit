package app

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/fredsaggio/url-shortener/internal/jobs"
	"github.com/fredsaggio/url-shortener/internal/messaging"
	"github.com/fredsaggio/url-shortener/internal/repositories"
	"github.com/fredsaggio/url-shortener/internal/services"
	"github.com/jackc/pgx/v5/pgxpool"
)

// RunClickWorker consumes click events and prunes the processed ones until
// ctx ends (ADR 0002). It runs in cmd/worker, or inside the API when
// RUN_CLICK_WORKER is set, on hosts without a separate worker service. The
// consumer is idempotent, so either way a redelivered event counts once.
func RunClickWorker(ctx context.Context, pool *pgxpool.Pool, rabbitMQURL string, prefetch int, retention time.Duration) {
	clickEventRepository := repositories.NewClickEventRepository(pool)
	clickCountService := services.NewClickCountService(clickEventRepository)
	consumer := messaging.NewClickConsumer(rabbitMQURL, prefetch, clickCountService)
	cleanup := jobs.NewProcessedClickEventCleanup(clickEventRepository, retention)

	var wg sync.WaitGroup

	wg.Go(func() {
		slog.Info("click worker started")
		consumer.Run(ctx)
		slog.Info("click worker stopped")
	})

	wg.Go(func() {
		cleanup.Run(ctx)
	})

	// The zone data is embedded (time/tzdata), so loading it does not fail
	// in practice; if it did, the salts would only outlive their day.
	location, err := time.LoadLocation(services.ClickDayZone)
	if err != nil {
		slog.Error("visitor data cleanup disabled", "error", err)
	} else {
		visitorCleanup := jobs.NewVisitorDataCleanup(clickEventRepository, location)
		wg.Go(func() {
			visitorCleanup.Run(ctx)
		})
	}

	wg.Wait()
}
