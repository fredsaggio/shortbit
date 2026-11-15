package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/fredsaggio/url-shortener/internal/app"
	"github.com/fredsaggio/url-shortener/internal/cache"
	"github.com/fredsaggio/url-shortener/internal/config"
	"github.com/fredsaggio/url-shortener/internal/db"
	"github.com/fredsaggio/url-shortener/internal/frontend"
	"github.com/fredsaggio/url-shortener/internal/googleoidc"
	"github.com/fredsaggio/url-shortener/internal/messaging"
	"github.com/fredsaggio/url-shortener/internal/server"

	"github.com/redis/go-redis/v9"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	slog.SetDefault(logger)

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGTERM, syscall.SIGINT)
	defer cancel()
	if err := Run(ctx, os.Getenv); err != nil {
		slog.Error("application stopped", "error", err)
		os.Exit(1)
	}
}

func Run(ctx context.Context, getEnv func(string) string) error {
	cfg, err := config.Load(getEnv)
	if err != nil {
		return err
	}

	if cfg.RunMigrations {
		if err := db.Migrate(ctx, cfg.DatabaseURL); err != nil {
			return err
		}
		slog.Info("database migrations up to date")
	}

	pool, err := db.Connect(ctx, cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer pool.Close()

	redisOptions, err := newRedisOptions(cfg.Redis)
	if err != nil {
		return err
	}
	redisClient := redis.NewClient(redisOptions)

	heatTracker := cache.NewRedisHeatTracker(redisClient, cfg.Redis.Timeout, cfg.HotURL.Window)
	destinationCache := cache.NewRedisDestinationCache(redisClient, cfg.Redis.Timeout, cfg.HotURL.CacheTTL)

	defer redisClient.Close()

	pingCtx, cancelPing := context.WithTimeout(ctx, cfg.Redis.Timeout)
	err = redisClient.Ping(pingCtx).Err()
	cancelPing()

	if err != nil {
		slog.Warn("Redis unavailable; continuing without cache", "error", err)
	} else {
		slog.Info("Redis connection ready")
	}

	clickPublisher := messaging.NewClickPublisher(cfg.RabbitMQ.URL, cfg.RabbitMQ.PublishTimeout)
	publisherCtx, stopPublisher := context.WithCancel(ctx)
	publisherDone := make(chan struct{})

	go func() {
		defer close(publisherDone)
		clickPublisher.Run(publisherCtx)
	}()

	defer func() {
		stopPublisher()
		<-publisherDone
	}()

	googleClient, err := googleoidc.New(ctx, cfg.Google)

	if err != nil {
		return fmt.Errorf("initialize Google OIDC client: %w", err)
	}

	handlers, registrationCleanup, err := app.CompositionRoot(pool, cfg, googleClient, heatTracker, destinationCache, clickPublisher)
	if err != nil {
		return fmt.Errorf("initialize application handlers: %w", err)
	}
	serverOptions := []server.Option{server.WithClientIPHeader(cfg.HTTP.ClientIPHeader)}
	// Secure cookies mean the API runs behind HTTPS, so browsers can be told
	// to never use plain HTTP for it again.
	if cfg.Session.CookieSecure {
		serverOptions = append(serverOptions, server.WithHSTS())
	}
	if cfg.HTTP.FrontendDir != "" {
		web, err := frontend.NewHandler(cfg.HTTP.FrontendDir)
		if err != nil {
			return err
		}
		serverOptions = append(serverOptions, server.WithFrontend(web))
		slog.Info("serving the web app", "dir", cfg.HTTP.FrontendDir)
	}
	srv := server.NewServer(handlers, pool, serverOptions...)

	handler := srv.NewRouterHTTP()

	server := &http.Server{
		Addr:              cfg.HTTP.Addr,
		Handler:           handler,
		ReadHeaderTimeout: cfg.HTTP.ReadHeaderTimeout,
		ReadTimeout:       cfg.HTTP.ReadTimeout,
		WriteTimeout:      cfg.HTTP.WriteTimeout,
		IdleTimeout:       cfg.HTTP.IdleTimeout,
	}

	serverErr := make(chan error, 1)

	go func() {
		serverErr <- server.ListenAndServe()
	}()

	jobCtx, stopJob := context.WithCancel(ctx)
	jobDone := make(chan struct{})

	go func() {
		defer close(jobDone)

		slog.Info("password registration cleanup job started")
		registrationCleanup.Run(jobCtx)
		slog.Info("password registration cleanup job stopped")
	}()

	defer func() {
		stopJob()
		<-jobDone
	}()

	if cfg.ClickWorker.RunInAPI {
		workerCtx, stopWorker := context.WithCancel(ctx)
		workerDone := make(chan struct{})

		go func() {
			defer close(workerDone)
			app.RunClickWorker(workerCtx, pool, cfg.RabbitMQ.URL, cfg.ClickWorker.RabbitMQPrefetch, cfg.ClickWorker.ProcessedClickEventRetention)
		}()

		defer func() {
			stopWorker()
			<-workerDone
		}()
	}

	select {
	case err := <-serverErr:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}

		return err

	case <-ctx.Done():
		slog.Info("shutting down server...")

		shutdownCtx, cancel := context.WithTimeout(
			context.Background(),
			cfg.HTTP.ShutdownTimeout,
		)
		defer cancel()

		if err := server.Shutdown(shutdownCtx); err != nil {
			_ = server.Close()
			return err
		}

		err := <-serverErr
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}

		return err
	}
}

// newRedisOptions builds the client options from REDIS_URL (hosted Redis,
// with password and TLS) or from the plain REDIS_ADDR used locally.
func newRedisOptions(cfg config.RedisConfig) (*redis.Options, error) {
	options := &redis.Options{Addr: cfg.Addr}
	if cfg.URL != "" {
		parsed, err := redis.ParseURL(cfg.URL)
		if err != nil {
			return nil, fmt.Errorf("parse REDIS_URL: %w", err)
		}
		options = parsed
	}

	// Cache calls must never hold a redirect: honor the per-call timeout and
	// fail fast instead of retrying (see the cache package).
	options.ContextTimeoutEnabled = true
	options.MaxRetries = -1

	return options, nil
}
