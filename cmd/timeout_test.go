package cmd

import (
	"context"
	"testing"
	"time"
)

// TestTimeoutCancelsContext verifies that a WithTimeout context is cancelled
// after the specified duration and reports DeadlineExceeded.
func TestTimeoutCancelsContext(t *testing.T) {
	timeout := 50 * time.Millisecond

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	ctx, timeoutCancel := context.WithTimeout(ctx, timeout)
	defer timeoutCancel()

	select {
	case <-ctx.Done():
		if ctx.Err() != context.DeadlineExceeded {
			t.Fatalf("expected DeadlineExceeded, got %v", ctx.Err())
		}
	case <-time.After(2 * time.Second):
		t.Fatal("context was not cancelled after timeout duration")
	}
}

// TestZeroTimeoutNoDeadline verifies that when Timeout is 0 no deadline is applied.
func TestZeroTimeoutNoDeadline(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Simulate the root.go logic: only wrap with WithTimeout when Timeout > 0.
	timeout := time.Duration(0)
	if timeout > 0 {
		var timeoutCancel context.CancelFunc
		ctx, timeoutCancel = context.WithTimeout(ctx, timeout)
		defer timeoutCancel()
	}

	if _, hasDeadline := ctx.Deadline(); hasDeadline {
		t.Fatal("expected no deadline when timeout is 0, but one was set")
	}
}
