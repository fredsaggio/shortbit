//go:build integration

package messaging_test

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/fredsaggio/url-shortener/internal/messaging"
	amqp "github.com/rabbitmq/amqp091-go"
)

const publisherReadyTimeout = 30 * time.Second

func TestClickPublisherIntegration(t *testing.T) {
	container, address := startRabbitMQ(t)
	rabbitURL := rabbitMQURL(address, rabbitMQPassword)

	publisher := messaging.NewClickPublisher(rabbitURL, 2*time.Second)

	ctx, cancel := context.WithCancel(t.Context())
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

	t.Run("confirms event and delivers it to the click queue", func(t *testing.T) {
		ch := openChannel(t, rabbitURL)
		purgeQueue(t, ch, messaging.ClickQueue)

		event := messaging.NewClickEvent(42, time.Now())
		if err := publisher.Publish(t.Context(), event); err != nil {
			t.Fatalf("Publish() error = %v", err)
		}

		delivery, ok, err := ch.Get(messaging.ClickQueue, true)
		if err != nil || !ok {
			t.Fatalf("Get() ok = %v, error = %v, want a delivery", ok, err)
		}

		if delivery.MessageId != event.EventID.String() {
			t.Errorf("MessageId = %q, want %q", delivery.MessageId, event.EventID)
		}

		if delivery.DeliveryMode != amqp.Persistent {
			t.Errorf("DeliveryMode = %d, want persistent", delivery.DeliveryMode)
		}

		if delivery.ContentType != "application/json" {
			t.Errorf("ContentType = %q, want application/json", delivery.ContentType)
		}

		var got messaging.ClickEvent
		if err := json.Unmarshal(delivery.Body, &got); err != nil {
			t.Fatalf("decode body: %v", err)
		}

		if got.EventID != event.EventID || got.URLID != event.URLID || !got.OccurredAt.Equal(event.OccurredAt) {
			t.Errorf("body = %+v, want %+v", got, event)
		}
	})

	t.Run("reports an unroutable event", func(t *testing.T) {
		ch := openChannel(t, rabbitURL)

		if err := ch.QueueUnbind(messaging.ClickQueue, messaging.ClickRoutingKey, messaging.ClickExchange, nil); err != nil {
			t.Fatalf("unbind click queue: %v", err)
		}
		t.Cleanup(func() {
			if err := ch.QueueBind(messaging.ClickQueue, messaging.ClickRoutingKey, messaging.ClickExchange, false, nil); err != nil {
				t.Errorf("rebind click queue: %v", err)
			}
		})

		err := publisher.Publish(t.Context(), messaging.NewClickEvent(42, time.Now()))

		if !errors.Is(err, messaging.ErrClickReturned) {
			t.Fatalf("Publish() error = %v, want %v", err, messaging.ErrClickReturned)
		}
	})

	t.Run("dead-letters an event after the delivery limit", func(t *testing.T) {
		ch := openChannel(t, rabbitURL)
		purgeQueue(t, ch, messaging.ClickQueue)
		purgeQueue(t, ch, messaging.ClickDeadLetterQueue)

		event := messaging.NewClickEvent(42, time.Now())
		if err := publisher.Publish(t.Context(), event); err != nil {
			t.Fatalf("Publish() error = %v", err)
		}

		// The quorum queue's delivery limit is 20. Since RabbitMQ 4.3 only
		// basic.reject and consumer crashes count as failed deliveries;
		// basic.nack with requeue does not.
		for range 25 {
			delivery, ok, err := ch.Get(messaging.ClickQueue, false)
			if err != nil {
				t.Fatalf("Get() error = %v", err)
			}
			if !ok {
				break
			}
			if err := delivery.Reject(true); err != nil {
				t.Fatalf("Reject() error = %v", err)
			}
		}

		deadline := time.Now().Add(10 * time.Second)
		for {
			delivery, ok, err := ch.Get(messaging.ClickDeadLetterQueue, true)
			if err != nil {
				t.Fatalf("Get() dead-letter error = %v", err)
			}
			if ok {
				if delivery.MessageId != event.EventID.String() {
					t.Errorf("dead-lettered MessageId = %q, want %q", delivery.MessageId, event.EventID)
				}
				return
			}
			if time.Now().After(deadline) {
				t.Fatal("event was not dead-lettered")
			}
			time.Sleep(100 * time.Millisecond)
		}
	})

	t.Run("reconnects after the connection is closed", func(t *testing.T) {
		exitCode, _, err := container.Exec(t.Context(), []string{"rabbitmqctl", "close_all_connections", "integration test"})
		if err != nil || exitCode != 0 {
			t.Fatalf("close connections: exit code %d, error %v", exitCode, err)
		}

		waitPublisherReady(t, publisher)
	})
}

// waitPublisherReady publishes until the publisher has a confirmed channel.
func waitPublisherReady(t *testing.T, publisher *messaging.ClickPublisher) {
	t.Helper()

	deadline := time.Now().Add(publisherReadyTimeout)
	for {
		err := publisher.Publish(t.Context(), messaging.NewClickEvent(1, time.Now()))
		if err == nil {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("publisher not ready: %v", err)
		}
		time.Sleep(100 * time.Millisecond)
	}
}

func openChannel(t *testing.T, rabbitURL string) *amqp.Channel {
	t.Helper()

	conn, err := messaging.Connect(rabbitURL, "shortbit-test")
	if err != nil {
		t.Fatalf("Connect() error = %v", err)
	}
	t.Cleanup(func() { _ = conn.Close() })

	ch, err := conn.Channel()
	if err != nil {
		t.Fatalf("open channel: %v", err)
	}

	return ch
}

func purgeQueue(t *testing.T, ch *amqp.Channel, queue string) {
	t.Helper()

	if _, err := ch.QueuePurge(queue, false); err != nil {
		t.Fatalf("purge %s: %v", queue, err)
	}
}
