package app

import (
	"context"
	"time"
)

// waitRetryContext uses a cancellable timer rather than sleeping unconditionally.
func waitRetryContext(ctx context.Context, delay time.Duration) error {
	if delay < 0 {
		delay = 0
	}
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return ctx.Err()
	}
}
