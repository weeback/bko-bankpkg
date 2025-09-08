package transfer

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"
)

func TestTransfer_QueueRetryAndRecover(t *testing.T) {
	mock := NewMockHTTP()
	tr := NewTransfer(mock).(*transfer)
	tr.maxLiveTime = time.Second // Short live time for testing

	// Test pack expiration
	t.Run("expired pack dropping", func(t *testing.T) {
		pack := &Pack{
			ID:        "test-expired",
			Payload:   []byte("test"),
			CreatedAt: time.Now().Add(-2 * time.Hour),
		}
		pack.retriedAt = pack.CreatedAt

		err := tr.MultiTransfer(context.Background(), nil, pack)
		if err != nil {
			t.Errorf("MultiTransfer() error = %v", err)
		}

		// Wait for pack to be processed and dropped
		time.Sleep(100 * time.Millisecond)

		// Verify the pack was dropped (not retried)
		if pack.retry > 0 {
			t.Errorf("Expired pack was retried %d times, expected to be dropped", pack.retry)
		}
	})

	// Test panic recovery
	t.Run("panic recovery and restart", func(t *testing.T) {
		// Force a panic by setting max concurrent to 0
		mock.SetMaxConcurrent(0)

		pack := &Pack{
			ID:        "test-panic",
			Payload:   []byte("test"),
			CreatedAt: time.Now(),
		}

		// This should queue the pack but cause a panic in the processor
		err := tr.MultiTransfer(context.Background(), nil, pack)
		if err != nil {
			t.Errorf("MultiTransfer() error = %v", err)
		}

		// Wait for panic and recovery
		time.Sleep(100 * time.Millisecond)

		// Reset to valid value and try another request
		mock.SetMaxConcurrent(1)
		pack2 := &Pack{
			ID:        "test-after-panic",
			Payload:   []byte("test"),
			CreatedAt: time.Now(),
		}

		// This should work after recovery
		err = tr.MultiTransfer(context.Background(), nil, pack2)
		if err != nil {
			t.Errorf("MultiTransfer() after panic recovery error = %v", err)
		}
	})

	// Test queue restart
	t.Run("queue processor restart", func(t *testing.T) {
		// Create a helper function to wait for queue state
		waitForQueueState := func(wantState int, timeout time.Duration) (bool, string) {
			deadline := time.Now().Add(timeout)
			checkInterval := 50 * time.Millisecond
			var lastState int
			var lastQueueInfo string

			for time.Now().Before(deadline) {
				tr.mu.Lock()
				state := tr.queueState
				hasQueue := tr.queue != nil
				var queueInfo string
				if hasQueue {
					queueInfo = fmt.Sprintf("queue len: %d, cap: %d", len(tr.queue), cap(tr.queue))
				} else {
					queueInfo = "queue: nil"
				}
				tr.mu.Unlock()

				// Log only if state or queue info changes
				if state != lastState || queueInfo != lastQueueInfo {
					t.Logf("Queue state change: state=%d, %s", state, queueInfo)
					lastState = state
					lastQueueInfo = queueInfo
				}

				if state == wantState {
					if wantState == 0 || (wantState == 1 && hasQueue) {
						// Wait a bit longer to ensure stability
						time.Sleep(50 * time.Millisecond)

						// Recheck the state after waiting
						tr.mu.Lock()
						finalState := tr.queueState
						finalHasQueue := tr.queue != nil
						tr.mu.Unlock()

						if finalState == wantState && (wantState == 0 || (wantState == 1 && finalHasQueue)) {
							return true, ""
						}
					}
				}

				time.Sleep(checkInterval)
			}

			// Get final state for error message
			tr.mu.Lock()
			state := tr.queueState
			hasQueue := tr.queue != nil
			queueInfo := "nil"
			if hasQueue {
				queueInfo = fmt.Sprintf("len: %d, cap: %d", len(tr.queue), cap(tr.queue))
			}
			tr.mu.Unlock()

			return false, fmt.Sprintf("timeout waiting for state: want %d, got %d, queue: %s",
				wantState, state, queueInfo)
		}

		// Create a new transfer instance with longer timeouts
		mock := NewMockHTTP()
		mock.SetMaxConcurrent(5)
		tr := NewTransfer(mock).(*transfer)
		tr.maxLiveTime = 5 * time.Second // Increased timeout
		tr.max = 5                       // Set queue size

		// Create a pack that will be used to trigger queue initialization
		pack1 := &Pack{
			ID:        "test-init",
			Payload:   []byte("test"),
			CreatedAt: time.Now(),
		}

		t.Log("Setting up initial queue state...")
		// Send first pack to initialize queue
		ctx := context.Background()
		ctxWithTimeout, cancel := context.WithTimeout(ctx, 2*time.Second)
		if err := tr.MultiTransfer(ctxWithTimeout, nil, pack1); err != nil {
			cancel()
			t.Fatalf("Failed to send initial pack: %v", err)
		}
		cancel()

		// Wait for queue to be fully initialized and running
		t.Log("Waiting for queue initialization...")
		if ok, details := waitForQueueState(1, 2*time.Second); !ok {
			t.Fatalf("Queue failed to initialize: %s", details)
		}

		// Let the first pack be processed
		time.Sleep(200 * time.Millisecond)

		t.Log("Testing queue restart by force closing...")
		// Force the queue to close
		tr.mu.Lock()
		oldQueue := tr.queue
		tr.queue = nil // This will make the processor exit
		tr.queueState = 0
		tr.once = sync.Once{} // Reset once to allow restart
		tr.mu.Unlock()

		// Close old queue after resetting state
		if oldQueue != nil {
			close(oldQueue)
		}

		// Wait for the processor to completely stop
		if ok, details := waitForQueueState(0, 2*time.Second); !ok {
			t.Fatalf("Queue failed to stop: %s", details)
		}

		// Wait for queue processor to fully stop
		t.Log("Waiting for queue to stop...")
		if ok, details := waitForQueueState(0, 2*time.Second); !ok {
			t.Fatalf("Queue failed to stop: %s", details)
		}

		// Create and send second pack to trigger restart
		t.Log("Sending restart pack...")
		pack2 := &Pack{
			ID:        "test-restart",
			Payload:   []byte("test"),
			CreatedAt: time.Now(),
		}

		// Send the second pack with timeout
		ctxWithTimeout2, cancel2 := context.WithTimeout(ctx, 2*time.Second)
		if err := tr.MultiTransfer(ctxWithTimeout2, nil, pack2); err != nil {
			cancel2()
			t.Fatalf("Failed to send restart pack: %v", err)
		}
		cancel2()

		// Wait for queue to be restarted
		t.Log("Waiting for queue restart...")
		if ok, details := waitForQueueState(1, 2*time.Second); !ok {
			t.Fatalf("Queue failed to restart: %s", details)
		}

		// Final verification with additional checks
		tr.mu.Lock()
		defer tr.mu.Unlock()

		if tr.queue == nil {
			t.Error("Queue not recreated after restart")
		}
		if tr.queueState != 1 {
			t.Errorf("Queue not properly running after restart: state=%d, hasQueue=%v",
				tr.queueState, tr.queue != nil)
		}
		if tr.queue != nil && cap(tr.queue) != tr.max {
			t.Errorf("Queue capacity mismatch: got %d, want %d", cap(tr.queue), tr.max)
		}
	})
}
