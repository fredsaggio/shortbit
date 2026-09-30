//go:build integration

package integration_test

import "context"

type noopHeatTracker struct{}

func (noopHeatTracker) Record(context.Context, string) (int64, error) { return 0, nil }
