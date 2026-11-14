//go:build integration

package integration_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"sync"
	"testing"
	"time"

	"github.com/fredsaggio/url-shortener/internal/app"
	"github.com/fredsaggio/url-shortener/internal/db/dbtest"
	"github.com/fredsaggio/url-shortener/internal/messaging"
	"github.com/fredsaggio/url-shortener/internal/messaging/messagingtest"
	"github.com/fredsaggio/url-shortener/internal/repositories"
	"github.com/fredsaggio/url-shortener/internal/server"
	"github.com/fredsaggio/url-shortener/internal/services"
	"github.com/jackc/pgx/v5/pgxpool"
)

const clickCountTimeout = 15 * time.Second

// lateConfirmPublisher publishes for real but reports a confirm timeout, like
// a broker that stored the event and answered after the deadline.
type lateConfirmPublisher struct {
	inner *messaging.ClickPublisher
}

func (p lateConfirmPublisher) Publish(ctx context.Context, event messaging.ClickEvent) error {
	if err := p.inner.Publish(ctx, event); err != nil {
		return err
	}
	return context.DeadlineExceeded
}

func TestPublicURLRedirectThroughQueueIntegration(t *testing.T) {
	pool := dbtest.Open(t)
	_, address := messagingtest.Start(t)
	rabbitURL := messagingtest.URL(address, messagingtest.Password)
	publisher := startClickPublisher(t, rabbitURL)

	t.Run("counts the click only after the worker consumes it", func(t *testing.T) {
		router, sessionCookie := newQueueTestRouter(t, pool, publisher, "queue-owner@example.com")
		code := createPublicURL(t, router, sessionCookie)

		response := performRequestWithCookie(t, router, http.MethodGet, "/"+code, nil)
		response.Body.Close()
		if response.StatusCode != http.StatusFound || response.Header.Get("Location") != "https://example.com/queued" {
			t.Fatalf("redirect = (status %d, Location %q), want (302, destination)", response.StatusCode, response.Header.Get("Location"))
		}

		if got := clickCountByCode(t, pool, code); got != 0 {
			t.Fatalf("click count before the worker = %d, want 0: the redirect must not write it", got)
		}

		stop := startClickWorker(t, pool, rabbitURL)
		defer stop()

		waitClickCount(t, pool, code, 1)
	})

	t.Run("does not count twice when a late event follows the fallback", func(t *testing.T) {
		router, sessionCookie := newQueueTestRouter(t, pool, lateConfirmPublisher{inner: publisher}, "late-confirm-owner@example.com")
		code := createPublicURL(t, router, sessionCookie)

		response := performRequestWithCookie(t, router, http.MethodGet, "/"+code, nil)
		response.Body.Close()
		if response.StatusCode != http.StatusFound {
			t.Fatalf("redirect status = %d, want 302", response.StatusCode)
		}

		if got := clickCountByCode(t, pool, code); got != 1 {
			t.Fatalf("click count after the fallback = %d, want 1", got)
		}

		stop := startClickWorker(t, pool, rabbitURL)
		defer stop()

		// The worker must consume the late event and skip it as processed.
		waitQueueEmpty(t, rabbitURL)
		stop()

		if got := clickCountByCode(t, pool, code); got != 1 {
			t.Errorf("click count after the late event = %d, want 1", got)
		}
	})
}

func newQueueTestRouter(t *testing.T, pool *pgxpool.Pool, publisher services.ClickPublisher, email string) (http.Handler, *http.Cookie) {
	t.Helper()

	createConfirmedPasswordUser(t, pool, email, "senha12345")

	applicationHandlers, _, err := app.CompositionRoot(pool, testConfig(24*time.Hour), unusedGoogleOIDCClient{}, noopHeatTracker{}, noopDestinationCache{}, publisher)
	if err != nil {
		t.Fatalf("CompositionRoot() error = %v", err)
	}
	router := server.NewServer(applicationHandlers, pool).NewRouterHTTP()

	login := performJSONRequest(t, router, http.MethodPost, "/sessions", `{"email":"`+email+`","password":"senha12345"}`)
	defer login.Body.Close()
	if login.StatusCode != http.StatusNoContent {
		t.Fatalf("login status = %d, want 204; body = %q", login.StatusCode, readResponseBody(t, login))
	}

	return router, requireCookie(t, login.Cookies(), "user_session")
}

func createPublicURL(t *testing.T, router http.Handler, sessionCookie *http.Cookie) string {
	t.Helper()

	response := performJSONRequestWithCookie(t, router, http.MethodPost, "/urls", `{"url":"https://example.com/queued","visibility":"public"}`, sessionCookie)
	defer response.Body.Close()
	if response.StatusCode != http.StatusCreated {
		t.Fatalf("create URL status = %d, want 201; body = %q", response.StatusCode, readResponseBody(t, response))
	}

	var result struct {
		ShortCode string `json:"short_code"`
	}
	if err := json.NewDecoder(response.Body).Decode(&result); err != nil || result.ShortCode == "" {
		t.Fatalf("decode created URL = %+v, error = %v", result, err)
	}

	return result.ShortCode
}

func startClickPublisher(t *testing.T, rabbitURL string) *messaging.ClickPublisher {
	t.Helper()

	publisher := messaging.NewClickPublisher(rabbitURL, 2*time.Second)
	stop := runInBackground(t, publisher.Run)
	t.Cleanup(stop)

	deadline := time.Now().Add(clickCountTimeout)
	for {
		err := publisher.Publish(t.Context(), messaging.NewClickEvent(1, time.Now()))
		if !errors.Is(err, messaging.ErrPublisherUnavailable) {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("publisher not ready: %v", err)
		}
		time.Sleep(100 * time.Millisecond)
	}

	// Drop the readiness probe so it does not reach the worker.
	purgeClickQueue(t, rabbitURL)

	return publisher
}

func startClickWorker(t *testing.T, pool *pgxpool.Pool, rabbitURL string) func() {
	t.Helper()

	handler := services.NewClickCountService(repositories.NewClickEventRepository(pool))
	consumer := messaging.NewClickConsumer(rabbitURL, 10, handler)

	stop := runInBackground(t, consumer.Run)
	t.Cleanup(stop)
	return stop
}

func runInBackground(t *testing.T, run func(context.Context)) func() {
	t.Helper()

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		defer close(done)
		run(ctx)
	}()

	return sync.OnceFunc(func() {
		cancel()
		<-done
	})
}

func purgeClickQueue(t *testing.T, rabbitURL string) {
	t.Helper()

	conn, err := messaging.Connect(rabbitURL, "shortbit-test")
	if err != nil {
		t.Fatalf("Connect() error = %v", err)
	}
	defer conn.Close()

	ch, err := conn.Channel()
	if err != nil {
		t.Fatalf("open channel: %v", err)
	}

	if _, err := ch.QueuePurge(messaging.ClickQueue, false); err != nil {
		t.Fatalf("purge click queue: %v", err)
	}
}

func waitQueueEmpty(t *testing.T, rabbitURL string) {
	t.Helper()

	conn, err := messaging.Connect(rabbitURL, "shortbit-test")
	if err != nil {
		t.Fatalf("Connect() error = %v", err)
	}
	defer conn.Close()

	ch, err := conn.Channel()
	if err != nil {
		t.Fatalf("open channel: %v", err)
	}

	deadline := time.Now().Add(clickCountTimeout)
	for {
		state, err := ch.QueueDeclarePassive(messaging.ClickQueue, true, false, false, false, nil)
		if err != nil {
			t.Fatalf("inspect click queue: %v", err)
		}
		if state.Messages == 0 {
			// Ready count drops on delivery; give the worker time to ack.
			time.Sleep(500 * time.Millisecond)
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("click queue still has %d messages", state.Messages)
		}
		time.Sleep(100 * time.Millisecond)
	}
}

func clickCountByCode(t *testing.T, pool *pgxpool.Pool, code string) int64 {
	t.Helper()

	var count int64
	if err := pool.QueryRow(t.Context(), "SELECT click_count FROM urls WHERE short_code = $1", code).Scan(&count); err != nil {
		t.Fatalf("read click count for %q: %v", code, err)
	}
	return count
}

func waitClickCount(t *testing.T, pool *pgxpool.Pool, code string, want int64) {
	t.Helper()

	deadline := time.Now().Add(clickCountTimeout)
	for {
		got := clickCountByCode(t, pool, code)
		if got == want {
			return
		}
		if got > want || time.Now().After(deadline) {
			t.Fatalf("click count = %d, want %d", got, want)
		}
		time.Sleep(100 * time.Millisecond)
	}
}
