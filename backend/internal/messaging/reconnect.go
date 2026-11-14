package messaging

import (
	"context"
	"log/slog"
	"time"
)

const (
	minReconnectDelay = time.Second
	maxReconnectDelay = 30 * time.Second
)

// session returns an error when it could not connect, retried with backoff,
// and nil after a working connection closed, retried at once.
func keepConnected(ctx context.Context, unavailableMessage string, session func(context.Context) error) {
	delay := minReconnectDelay

	for {
		err := session(ctx)
		if ctx.Err() != nil {
			return
		}

		if err == nil {
			delay = minReconnectDelay
			continue
		}

		slog.Warn(unavailableMessage, "error", err, "retry_in", delay.String())

		select {
		case <-ctx.Done():
			return
		case <-time.After(delay):
		}

		delay = min(delay*2, maxReconnectDelay)
	}
}
