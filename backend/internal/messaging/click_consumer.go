package messaging

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"
	"uuid"

	amqp "github.com/rabbitmq/amqp091-go"
)

const (
	clickConsumerTag = "shortbit-worker"

	handleClickTimeout = 10 * time.Second

	minFailureDelay = time.Second
	maxFailureDelay = 30 * time.Second
)

type ClickHandler interface {
	HandleClick(ctx context.Context, event ClickEvent) error
}

type ClickConsumer struct {
	url      string
	prefetch int
	handler  ClickHandler

	failureDelay time.Duration
}

func NewClickConsumer(url string, prefetch int, handler ClickHandler) *ClickConsumer {
	return &ClickConsumer{url: url, prefetch: prefetch, handler: handler}
}

func (c *ClickConsumer) Run(ctx context.Context) {
	keepConnected(ctx, "RabbitMQ unavailable; click worker waiting", func(ctx context.Context) error {
		conn, ch, deliveries, err := c.open()
		if err != nil {
			return err
		}
		defer conn.Close()

		slog.Info("RabbitMQ click consumer ready")
		c.consume(ctx, ch, deliveries)
		return nil
	})
}

func (c *ClickConsumer) open() (*amqp.Connection, *amqp.Channel, <-chan amqp.Delivery, error) {
	conn, err := Connect(c.url, "shortbit-worker")
	if err != nil {
		return nil, nil, nil, err
	}

	ch, err := conn.Channel()
	if err != nil {
		_ = conn.Close()
		return nil, nil, nil, fmt.Errorf("open RabbitMQ channel: %w", err)
	}

	if err := ch.Qos(c.prefetch, 0, false); err != nil {
		_ = conn.Close()
		return nil, nil, nil, fmt.Errorf("set RabbitMQ prefetch: %w", err)
	}

	if err := DeclareClickTopology(ch); err != nil {
		_ = conn.Close()
		return nil, nil, nil, err
	}

	deliveries, err := ch.Consume(ClickQueue, clickConsumerTag, false, false, false, false, nil)
	if err != nil {
		_ = conn.Close()
		return nil, nil, nil, fmt.Errorf("consume click queue: %w", err)
	}

	return conn, ch, deliveries, nil
}

func (c *ClickConsumer) consume(ctx context.Context, ch *amqp.Channel, deliveries <-chan amqp.Delivery) {
	for {
		select {
		case <-ctx.Done():
			c.release(ch, deliveries)
			return
		case delivery, ok := <-deliveries:
			if !ok {
				slog.Warn("RabbitMQ click deliveries closed; reconnecting")
				return
			}
			c.handle(ctx, delivery)
		}
	}
}

// basic.nack, unlike a closed channel, does not count toward the delivery limit.
func (c *ClickConsumer) release(ch *amqp.Channel, deliveries <-chan amqp.Delivery) {
	if err := ch.Cancel(clickConsumerTag, false); err != nil {
		slog.Warn("cancel RabbitMQ click consumer failed", "error", err)
		return
	}

	for delivery := range deliveries {
		if err := delivery.Nack(false, true); err != nil {
			slog.Warn("requeue click event on shutdown failed", "message_id", delivery.MessageId, "error", err)
		}
	}
}

func (c *ClickConsumer) handle(ctx context.Context, delivery amqp.Delivery) {
	var event ClickEvent
	if err := json.Unmarshal(delivery.Body, &event); err != nil || event.EventID == uuid.Nil() || event.URLID <= 0 {
		slog.Error("invalid click event dead-lettered", "message_id", delivery.MessageId, "error", err)
		if err := delivery.Reject(false); err != nil {
			slog.Warn("dead-letter invalid click event failed", "message_id", delivery.MessageId, "error", err)
		}
		return
	}

	// The event in hand finishes during shutdown instead of being rolled back.
	handleCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), handleClickTimeout)
	err := c.handler.HandleClick(handleCtx, event)
	cancel()

	if err != nil {
		c.retryLater(ctx, delivery, event, err)
		return
	}

	c.failureDelay = 0

	// A lost ack makes RabbitMQ redeliver the event, which the handler
	// recognizes as already processed.
	if err := delivery.Ack(false); err != nil {
		slog.Warn("ack click event failed", "event_id", event.EventID, "error", err)
	}
}

// The delay keeps a database outage from burning the delivery limit in
// milliseconds. basic.reject is used because, since RabbitMQ 4.3, basic.nack
// does not count toward the limit and a poison event would loop forever.
func (c *ClickConsumer) retryLater(ctx context.Context, delivery amqp.Delivery, event ClickEvent, handleErr error) {
	if c.failureDelay == 0 {
		c.failureDelay = minFailureDelay
	} else {
		c.failureDelay = min(c.failureDelay*2, maxFailureDelay)
	}

	slog.Error("click event failed; retrying", "event_id", event.EventID, "url_id", event.URLID, "retry_in", c.failureDelay.String(), "error", handleErr)

	select {
	case <-ctx.Done():
		if err := delivery.Nack(false, true); err != nil {
			slog.Warn("requeue click event on shutdown failed", "event_id", event.EventID, "error", err)
		}
		return
	case <-time.After(c.failureDelay):
	}

	if err := delivery.Reject(true); err != nil {
		slog.Warn("requeue failed click event failed", "event_id", event.EventID, "error", err)
	}
}
