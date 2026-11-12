//go:build integration

package integration_test

import (
	"context"

	"github.com/fredsaggio/url-shortener/internal/messaging"
)

type noopHeatTracker struct{}

func (noopHeatTracker) Record(context.Context, string) (int64, error) { return 0, nil }

// unavailableClickPublisher makes the redirect count clicks synchronously, so
// a test can read click_count right after the redirect.
type unavailableClickPublisher struct{}

func (unavailableClickPublisher) Publish(context.Context, messaging.ClickEvent) error {
	return messaging.ErrPublisherUnavailable
}
