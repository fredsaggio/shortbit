//go:build integration

package integration_test

import (
	"context"

	"github.com/fredsaggio/url-shortener/internal/messaging"
)

type noopHeatTracker struct{}

func (noopHeatTracker) Record(context.Context, string) (int64, error) { return 0, nil }

type noopDestinationCache struct{}

func (noopDestinationCache) Get(context.Context, string) (int64, string, bool, error) {
	return 0, "", false, nil
}

func (noopDestinationCache) Set(context.Context, string, int64, string) error { return nil }

// unavailableClickPublisher makes the redirect count clicks synchronously, so
// a test can read click_count right after the redirect.
type unavailableClickPublisher struct{}

func (unavailableClickPublisher) Publish(context.Context, messaging.ClickEvent) error {
	return messaging.ErrPublisherUnavailable
}
