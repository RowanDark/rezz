package ratelimit

import (
	"context"
	"math/rand"
	"time"
)

// Wait sleeps for delay + random jitter, or until ctx is cancelled.
// jitter is a fraction of delay added randomly: actual = delay + rand(0, delay*jitter)
// If jitter is 0, waits exactly delay. If delay is 0, returns immediately.
func Wait(ctx context.Context, delay time.Duration, jitter float64) {
	if delay <= 0 {
		return
	}
	actual := delay
	if jitter > 0 {
		maxJitter := time.Duration(float64(delay) * jitter)
		actual += time.Duration(rand.Int63n(int64(maxJitter) + 1))
	}
	select {
	case <-time.After(actual):
	case <-ctx.Done():
	}
}
