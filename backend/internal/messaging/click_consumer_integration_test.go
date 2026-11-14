//go:build integration

package messaging_test

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/fredsaggio/url-shortener/internal/db/dbtest"
	"github.com/fredsaggio/url-shortener/internal/messaging"
	"github.com/fredsaggio/url-shortener/internal/models"
	"github.com/fredsaggio/url-shortener/internal/repositories"
	"github.com/fredsaggio/url-shortener/internal/services"
	"github.com/fredsaggio/url-shortener/internal/shortcodes"
	"github.com/jackc/pgx/v5/pgxpool"
	amqp "github.com/rabbitmq/amqp091-go"
)

const handledEventTimeout = 10 * time.Second

type recordingHandler struct {
	mu       sync.Mutex
	failures int
	events   chan messaging.ClickEvent
}

func newRecordingHandler(failures int) *recordingHandler {
	return &recordingHandler{failures: failures, events: make(chan messaging.ClickEvent, 16)}
}

func (h *recordingHandler) HandleClick(_ context.Context, event messaging.ClickEvent) error {
	h.events <- event

	h.mu.Lock()
	defer h.mu.Unlock()

	if h.failures > 0 {
		h.failures--
		return errors.New("database unavailable")
	}

	return nil
}

func (h *recordingHandler) next(t *testing.T) messaging.ClickEvent {
	t.Helper()

	select {
	case event := <-h.events:
		return event
	case <-time.After(handledEventTimeout):
		t.Fatal("handler did not receive an event")
		return messaging.ClickEvent{}
	}
}

func TestClickConsumerIntegration(t *testing.T) {
	_, address := startRabbitMQ(t)
	rabbitURL := rabbitMQURL(address, rabbitMQPassword)
	publisher := startPublisher(t, rabbitURL)

	t.Run("acknowledges a handled event", func(t *testing.T) {
		handler := newRecordingHandler(0)
		stop := startConsumer(t, rabbitURL, handler)

		event := messaging.NewClickEvent(42, time.Now())
		if err := publisher.Publish(t.Context(), event); err != nil {
			t.Fatalf("Publish() error = %v", err)
		}

		if got := handler.next(t); got.EventID != event.EventID || got.URLID != event.URLID {
			t.Errorf("handled %+v, want %+v", got, event)
		}

		stop()
		assertQueueLength(t, rabbitURL, messaging.ClickQueue, 0)
	})

	t.Run("redelivers a failed event until it is handled", func(t *testing.T) {
		handler := newRecordingHandler(1)
		stop := startConsumer(t, rabbitURL, handler)

		event := messaging.NewClickEvent(42, time.Now())
		if err := publisher.Publish(t.Context(), event); err != nil {
			t.Fatalf("Publish() error = %v", err)
		}

		first := handler.next(t)
		second := handler.next(t)
		if first.EventID != event.EventID || second.EventID != event.EventID {
			t.Errorf("handled %v then %v, want %v twice", first.EventID, second.EventID, event.EventID)
		}

		stop()
		assertQueueLength(t, rabbitURL, messaging.ClickQueue, 0)
	})

	t.Run("dead-letters a malformed event without handling it", func(t *testing.T) {
		ch := openChannel(t, rabbitURL)
		purgeQueue(t, ch, messaging.ClickDeadLetterQueue)

		handler := newRecordingHandler(0)
		stop := startConsumer(t, rabbitURL, handler)

		if err := ch.PublishWithContext(t.Context(), messaging.ClickExchange, messaging.ClickRoutingKey, false, false, amqp.Publishing{
			ContentType: "application/json",
			MessageId:   "malformed",
			Body:        []byte("not json"),
		}); err != nil {
			t.Fatalf("publish malformed event: %v", err)
		}

		deadline := time.Now().Add(handledEventTimeout)
		for {
			delivery, ok, err := ch.Get(messaging.ClickDeadLetterQueue, true)
			if err != nil {
				t.Fatalf("Get() dead-letter error = %v", err)
			}
			if ok {
				if delivery.MessageId != "malformed" {
					t.Errorf("dead-lettered MessageId = %q, want malformed", delivery.MessageId)
				}
				break
			}
			if time.Now().After(deadline) {
				t.Fatal("malformed event was not dead-lettered")
			}
			time.Sleep(100 * time.Millisecond)
		}

		stop()

		select {
		case event := <-handler.events:
			t.Errorf("handler received %+v, want no call", event)
		default:
		}
	})

	t.Run("counts each event once in PostgreSQL", func(t *testing.T) {
		pool := dbtest.Open(t)
		url := createConsumerTestURL(t, pool)

		handler := services.NewClickCountService(repositories.NewClickEventRepository(pool))
		stop := startConsumer(t, rabbitURL, handler)
		defer stop()

		first := messaging.NewClickEvent(url.ID, time.Now())
		second := messaging.NewClickEvent(url.ID, time.Now())

		// Publishing first twice simulates a redelivery, or a late event
		// whose click the synchronous fallback already counted.
		for _, event := range []messaging.ClickEvent{first, first, second} {
			if err := publisher.Publish(t.Context(), event); err != nil {
				t.Fatalf("Publish() error = %v", err)
			}
		}

		deadline := time.Now().Add(handledEventTimeout)
		for {
			var clickCount, processed int64
			if err := pool.QueryRow(t.Context(), `SELECT click_count FROM urls WHERE id = $1`, url.ID).Scan(&clickCount); err != nil {
				t.Fatalf("read click_count: %v", err)
			}
			if err := pool.QueryRow(t.Context(), `SELECT COUNT(*) FROM processed_click_events WHERE event_id IN ($1, $2)`, first.EventID, second.EventID).Scan(&processed); err != nil {
				t.Fatalf("count processed events: %v", err)
			}

			if clickCount == 2 && processed == 2 {
				break
			}
			if clickCount > 2 {
				t.Fatalf("click_count = %d, want 2", clickCount)
			}
			if time.Now().After(deadline) {
				t.Fatalf("click_count = %d, processed = %d, want 2 and 2", clickCount, processed)
			}
			time.Sleep(100 * time.Millisecond)
		}

		stop()
		assertQueueLength(t, rabbitURL, messaging.ClickQueue, 0)
	})
}

func startPublisher(t *testing.T, rabbitURL string) *messaging.ClickPublisher {
	t.Helper()

	publisher := messaging.NewClickPublisher(rabbitURL, 2*time.Second)

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		defer close(done)
		publisher.Run(ctx)
	}()
	t.Cleanup(func() {
		cancel()
		<-done
	})

	waitPublisherReady(t, publisher)
	purgeQueue(t, openChannel(t, rabbitURL), messaging.ClickQueue)

	return publisher
}

// startConsumer returns an idempotent stop that waits for the consumer to
// release its deliveries.
func startConsumer(t *testing.T, rabbitURL string, handler messaging.ClickHandler) func() {
	t.Helper()

	consumer := messaging.NewClickConsumer(rabbitURL, 10, handler)

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		defer close(done)
		consumer.Run(ctx)
	}()

	stop := sync.OnceFunc(func() {
		cancel()
		<-done
	})
	t.Cleanup(stop)

	return stop
}

func assertQueueLength(t *testing.T, rabbitURL string, queue string, want int) {
	t.Helper()

	ch := openChannel(t, rabbitURL)

	state, err := ch.QueueDeclarePassive(queue, true, false, false, false, nil)
	if err != nil {
		t.Fatalf("inspect %s: %v", queue, err)
	}

	if state.Messages != want {
		t.Errorf("%s has %d messages, want %d", queue, state.Messages, want)
	}
}

func createConsumerTestURL(t *testing.T, pool *pgxpool.Pool) models.URL {
	t.Helper()

	user, err := repositories.NewUserRepository(pool).CreateWithPassword(t.Context(), "click-worker@example.com", "$argon2id$integration-test-hash")
	if err != nil {
		t.Fatalf("CreateWithPassword() error = %v", err)
	}

	urlRepository := repositories.NewURLRepository(pool)

	id, err := urlRepository.ReserveID(t.Context())
	if err != nil {
		t.Fatalf("ReserveID() error = %v", err)
	}

	generator, err := shortcodes.NewGenerator()
	if err != nil {
		t.Fatalf("NewGenerator() error = %v", err)
	}

	code, err := generator.Generate(id)
	if err != nil {
		t.Fatalf("Generate(%d) error = %v", id, err)
	}

	url, err := urlRepository.Create(t.Context(), models.URL{
		ID:          id,
		ShortCode:   code,
		UserID:      user.ID,
		OriginalURL: "https://example.com/worker",
		Visibility:  models.VisibilityPublic,
	})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	return url
}
