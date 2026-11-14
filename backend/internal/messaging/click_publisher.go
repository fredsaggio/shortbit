package messaging

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
)

var (
	ErrPublisherUnavailable = errors.New("click publisher unavailable")
	ErrClickNotConfirmed    = errors.New("click event rejected by RabbitMQ")
	ErrClickReturned        = errors.New("click event returned as unroutable by RabbitMQ")
)

const (
	returnsBufferSize = 256
	// A return whose publisher already gave up is never claimed; drop it
	// after a period far longer than any publish timeout.
	returnRetention = time.Minute
)

// ClickPublisher publishes click events with publisher confirms. Run keeps
// the connection alive in the background; while it is down, Publish fails
// immediately so the redirect can fall back to synchronous counting.
type ClickPublisher struct {
	url     string
	timeout time.Duration

	stateMu sync.RWMutex
	ch      *amqp.Channel
	returns chan amqp.Return
	blocked bool

	returnsMu sync.Mutex
	returned  map[string]time.Time
}

func NewClickPublisher(url string, timeout time.Duration) *ClickPublisher {
	return &ClickPublisher{
		url:      url,
		timeout:  timeout,
		returned: make(map[string]time.Time),
	}
}

// Run connects and reconnects with exponential backoff until ctx is done.
func (p *ClickPublisher) Run(ctx context.Context) {
	keepConnected(ctx, "RabbitMQ unavailable; counting clicks synchronously", func(ctx context.Context) error {
		conn, ch, returns, err := p.open()
		if err != nil {
			return err
		}

		slog.Info("RabbitMQ click publisher ready")
		p.serve(ctx, conn, ch, returns)
		return nil
	})
}

func (p *ClickPublisher) open() (*amqp.Connection, *amqp.Channel, chan amqp.Return, error) {
	conn, err := Connect(p.url, "shortbit-api")
	if err != nil {
		return nil, nil, nil, err
	}

	ch, err := conn.Channel()
	if err != nil {
		_ = conn.Close()
		return nil, nil, nil, fmt.Errorf("open RabbitMQ channel: %w", err)
	}

	if err := ch.Confirm(false); err != nil {
		_ = conn.Close()
		return nil, nil, nil, fmt.Errorf("enable publisher confirms: %w", err)
	}

	if err := DeclareClickTopology(ch); err != nil {
		_ = conn.Close()
		return nil, nil, nil, err
	}

	returns := ch.NotifyReturn(make(chan amqp.Return, returnsBufferSize))

	return conn, ch, returns, nil
}

// serve publishes the channel to Publish and blocks until the connection or
// channel closes, or ctx is done.
func (p *ClickPublisher) serve(ctx context.Context, conn *amqp.Connection, ch *amqp.Channel, returns chan amqp.Return) {
	connClosed := conn.NotifyClose(make(chan *amqp.Error, 1))
	chClosed := ch.NotifyClose(make(chan *amqp.Error, 1))
	blockings := conn.NotifyBlocked(make(chan amqp.Blocking, 1))

	p.setChannel(ch, returns)
	defer func() {
		p.setChannel(nil, nil)
		_ = conn.Close()
	}()

	for {
		select {
		case <-ctx.Done():
			return
		case err := <-connClosed:
			slog.Warn("RabbitMQ connection closed; reconnecting", "error", err)
			return
		case err := <-chClosed:
			slog.Warn("RabbitMQ channel closed; reconnecting", "error", err)
			return
		case blocking, ok := <-blockings:
			if !ok {
				return
			}
			// A blocked broker stops reading, and a publish would hang on
			// the socket write beyond the confirm timeout.
			p.setBlocked(blocking.Active)
			if blocking.Active {
				slog.Warn("RabbitMQ blocked publishing", "reason", blocking.Reason)
			} else {
				slog.Info("RabbitMQ unblocked publishing")
			}
		}
	}
}

func (p *ClickPublisher) setChannel(ch *amqp.Channel, returns chan amqp.Return) {
	p.stateMu.Lock()
	defer p.stateMu.Unlock()

	p.ch = ch
	p.returns = returns
	p.blocked = false
}

func (p *ClickPublisher) setBlocked(blocked bool) {
	p.stateMu.Lock()
	defer p.stateMu.Unlock()

	p.blocked = blocked
}

// Publish returns nil only after RabbitMQ confirmed that the event reached the
// click queue. A timeout does not prove the event was lost: it may still be
// enqueued and later counted by the worker.
func (p *ClickPublisher) Publish(ctx context.Context, event ClickEvent) error {
	p.stateMu.RLock()
	ch, returns, blocked := p.ch, p.returns, p.blocked
	p.stateMu.RUnlock()

	if ch == nil || blocked {
		return ErrPublisherUnavailable
	}

	body, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("encode click event: %w", err)
	}

	ctx, cancel := context.WithTimeout(ctx, p.timeout)
	defer cancel()

	messageID := event.EventID.String()

	confirmation, err := ch.PublishWithDeferredConfirmWithContext(ctx, ClickExchange, ClickRoutingKey, true, false, amqp.Publishing{
		ContentType:  "application/json",
		DeliveryMode: amqp.Persistent,
		MessageId:    messageID,
		Timestamp:    event.OccurredAt,
		Body:         body,
	})
	if err != nil {
		return fmt.Errorf("publish click event: %w", err)
	}

	acked, err := confirmation.WaitContext(ctx)
	returned := p.claimReturn(returns, messageID)

	if err != nil {
		return fmt.Errorf("wait click event confirmation: %w", err)
	}

	if !acked {
		return ErrClickNotConfirmed
	}

	if returned {
		return ErrClickReturned
	}

	return nil
}

// claimReturn reports whether RabbitMQ returned messageID. The broker sends a
// basic.return before the basic.ack of the same message, and the client hands
// it to the returns channel before processing the ack, so after the confirm
// the return is either still buffered or already recorded by an earlier
// drain. Draining is done by publishers, not by a background goroutine,
// because a goroutine could still be holding a received return when the ack
// wakes the publisher.
func (p *ClickPublisher) claimReturn(returns chan amqp.Return, messageID string) bool {
	p.returnsMu.Lock()
	defer p.returnsMu.Unlock()

	now := time.Now()

drain:
	for {
		select {
		case ret, ok := <-returns:
			if !ok {
				break drain
			}
			p.returned[ret.MessageId] = now
		default:
			break drain
		}
	}

	for id, receivedAt := range p.returned {
		if now.Sub(receivedAt) > returnRetention {
			delete(p.returned, id)
		}
	}

	_, returned := p.returned[messageID]
	delete(p.returned, messageID)

	return returned
}
