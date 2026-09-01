package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/fredsaggio/url-shortener/internal/db"
)

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGTERM, syscall.SIGINT)
	defer cancel()
	if err := Run(ctx, os.Getenv); err != nil {
		log.Fatal(err)
	}
}

func Run(ctx context.Context, getEnv func(string) string) error {

	connStr := getEnv("DATABASE_URL")

	if connStr == "" {
		return errors.New("DATABASE_URL is not set")
	}

	pool, err := db.Connect(ctx, connStr)
	if err != nil {
		return err
	}
	defer pool.Close()

	mux := http.NewServeMux()

	server := &http.Server{
		Addr:              ":8080",
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}

	serverErr := make(chan error, 1)

	go func() {
		serverErr <- server.ListenAndServe()
	}()

	select {
	case err := <-serverErr:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}

		return err

	case <-ctx.Done():
		log.Println("shutting down server...")

		shutdownCtx, cancel := context.WithTimeout(
			context.Background(),
			10*time.Second,
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
