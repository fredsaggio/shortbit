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
	"github.com/fredsaggio/url-shortener/internal/config"
	"github.com/fredsaggio/url-shortener/internal/db"
	"github.com/fredsaggio/url-shortener/internal/googleoidc"
	"github.com/fredsaggio/url-shortener/internal/server"
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

	pool, err := db.Connect(ctx, cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer pool.Close()

	googleClient, err := googleoidc.New(ctx, cfg.Google)

	if err != nil {
		return fmt.Errorf("initialize Google OIDC client: %w", err)
	}

	handlers, registrationCleanup, err := app.CompositionRoot(pool, cfg, googleClient)
	if err != nil {
		return fmt.Errorf("initialize application handlers: %w", err)
	}
	srv := server.NewServer(handlers, pool)

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
