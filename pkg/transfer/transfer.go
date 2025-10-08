package transfer

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/weeback/bko-bankpkg/pkg/logger"
	"github.com/weeback/bko-bankpkg/pkg/queue"
	"go.uber.org/zap"
)

// Transfer is an interface that defines methods for transferring data in both
// single and multi-mode operations.
type Transfer interface {
	// SingleTransfer performs a one-time transfer operation for a given data pack.
	// It blocks until the transfer is complete or fails.
	SingleTransfer(ctx context.Context, result any, url string, data *Pack) error

	// MultiTransferWaitCallback performs batch transfer operations with callback-based results handling.
	// It processes multiple packs concurrently and executes the provided callback for each completed transfer.
	// The callback function is called with:
	// - id: the unique identifier of the processed pack
	// - payloadResponse: the response data from the transfer
	// - execError: any error that occurred during the transfer
	// Returns an error if the batch operation fails to start or if context is cancelled.
	MultiTransferWaitCallback(ctx context.Context, callback func(id string, payloadResponse []byte, execError error),
		url string, data []*Pack) error
}

// NewTransfer creates a new Transfer instance with the provided HTTP partner.
// It initializes the transfer with default concurrent limits and live time settings.
func NewTransfer(partner queue.MultiDestination) Transfer {
	return &transfer{
		partner:     partner,
		max:         DefaultMaxConcurrent, // default max concurrent for multi transfer
		maxRetries:  DefaultMaxRetries,    // default max retries for a pack in multi transfer
		maxLiveTime: DefaultMaxLiveTime,   // default max live time for a pack in multi transfer
	}
}

type transfer struct {
	partner queue.MultiDestination

	queue       chan *Pack
	queueState  int // 0: not started, 1: running
	max         int
	maxRetries  int
	maxLiveTime time.Duration
	mu          sync.Mutex
	once        sync.Once
}

// addQueueMultiTransfer adds a pack to the multi transfer queue for asynchronous processing.
// On first call, it initializes the queue with a capacity based on max concurrent settings
// (minimum 2) and starts the queue processor goroutine. For subsequent calls, it attempts
// to add the pack to the existing queue.
//
// If the queue is not initialized (nil), it attempts re-initialization. The function includes
// timeout protection to avoid indefinite blocking when the queue is full.
//
// Parameters:
//   - val: The pack to be queued. If nil, logs an error and returns nil.
//
// Returns:
//   - error: Returns fmt.Errorf if the queue is full after timeout, nil otherwise.
func (t *transfer) addQueueMultiTransfer(val *Pack) error {

	t.once.Do(func() {
		t.mu.Lock()
		defer t.mu.Unlock()
		// Set max to half of partner's max concurrent, minimum 2
		if t.max > 0 {
			t.queue = make(chan *Pack, t.max)
			t.queueState = 0
		} else {
			t.queue = make(chan *Pack, 2)
			t.queueState = 0
		}
		// Start the queue processor
		go t.queueMultiTransferProcess()
	})

	entry := logger.NewEntry().With(
		zap.String(logger.KeyFunctionName, "addQueueMultiTransfer"),
	)

	// Validate input
	if val == nil {
		entry.Error("attempted to queue nil pack")
		return nil
	}

	// If queue is not initialized, re-initialize
	if t.queue == nil {
		// Queue is not initialized, should not happen
		entry.Error("queue is not initialized in addQueueMultiTransfer")

		t.once = sync.Once{} // Reset once to allow re-initialization
		t.queueState = 0

		return t.addQueueMultiTransfer(val) // Retry adding to queue
	}

	// Attempt to add to queue with timeout to avoid blocking indefinitely
	select {
	case t.queue <- val:
		entry.Info("re-queued pack in multiQueueProcess",
			zap.String("pack_id", val.ID),
			zap.Int("retry", val.retry),
			zap.String(logger.KeyFunctionName, "addQueueMultiTransfer"))
	case <-time.After(QueueTimeout):
		entry.Warn("dropped pack due to full queue in addQueueMultiTransfer",
			zap.String("pack_id", val.ID),
			zap.String(logger.KeyFunctionName, "addQueueMultiTransfer"))

		// Call callback with queue full error
		go val.callback(val.ID, nil, NewError(ErrQueueBusy, "queue is full", nil))
		return fmt.Errorf("dropped pack due to full queue")
	}

	return nil
}

// queueMultiTransferProcess handles the processing of queued transfer packs in a concurrent-safe manner.
// It implements the following features:
//   - Ensures single instance execution using mutex locking
//   - Implements panic recovery to maintain queue processing stability
//   - Processes queued packs with the following checks:
//   - Drops packs that exceed maxLiveTime
//   - Implements retry backoff using MultiTransferWait
//   - Requeues packs that need retry with updated retry timestamps
//   - Forwards valid packs to postOrRetryMultiTransfer for processing
//
// The function automatically resets its state and allows reinitialization on completion or panic,
// ensuring continuous queue processing capability.
func (t *transfer) queueMultiTransferProcess() {
	// Ensure only one instance is running
	t.mu.Lock()
	if t.queueState == 1 {
		// Already running
		t.mu.Unlock()
		return
	}

	// Mark as running
	t.queueState = 1

	// Capture the current queue to avoid
	currentQueue := t.queue

	t.mu.Unlock()

	entry := logger.NewEntry().With(
		zap.String(logger.KeyFunctionName, "queueMultiTransferProcess"),
	)

	// Handle errors when processing the queue
	defer func() {
		if r := recover(); r != nil {
			entry.Error("recovered from panic in multiQueueProcess",
				zap.Any("recover", r))
		}
		// Restart the processor by resetting once
		t.mu.Lock()
		t.queueState = 0
		t.once = sync.Once{}
		t.mu.Unlock()
	}()

	for q := range currentQueue {
		// Drop packs that exceed max retries
		if q.retry > t.maxRetries {
			entry.Warn("dropping pack due to max retries exceeded in multiQueueProcess",
				zap.String("pack_id", q.ID),
				zap.Int("retries", q.retry))

			go q.callback(q.ID, nil, NewError(ErrMaxRetriesExceeded, "max retries exceeded", nil))
			continue
		}
		// Drop old packs
		if d := time.Since(q.CreatedAt); d > t.maxLiveTime {
			entry.Warn("dropping old pack in multiQueueProcess",
				zap.String("pack_id", q.ID),
				zap.String("duration", d.String()),
				zap.Int("retries", q.retry))

			go q.callback(q.ID, nil, NewError(ErrPackExpired, "pack expired", nil))
			continue
		}
		// Skip if recently retried
		if d := time.Since(q.retriedAt); d < MultiTransferWait {
			// Wait before retrying
			time.Sleep(MultiTransferWait - d)
			// Re-add to queue - no retry count increment
			go func(p *Pack) {
				if err := t.addQueueMultiTransfer(p); err != nil {
					entry.Info("re-queued pack in multiQueueProcess",
						zap.Error(err),
						zap.String("pack_id", p.ID),
						zap.Int("retry", p.retry))
				}
			}(q)
			// Continue to next pack
			continue
		}

		// Send the payload to the partner
		t.postOrRetryMultiTransfer(q, RequestTimeout)
	}
}

// postOrRetryMultiTransfer handles the posting of a data pack to a partner service with retry capability.
// It implements the following features:
//   - Monitors partner queue status and handles busy/occupied states
//   - Implements retry mechanism for failed transfers
//   - Manages timeouts for post operations
//   - Processes responses through callbacks
//   - Provides comprehensive logging of transfer states
//
// Parameters:
//   - destURL: The destination URL for the post operation
//   - q: The Pack containing the payload and callback for processing
//   - timeout: Maximum duration to wait for the post operation to complete
//
// The function will:
// 1. Check partner queue status
// 2. If partner is busy, increment retry count and re-queue the pack
// 3. If partner is available, attempt to post the payload
// 4. On failure, increment retry count and re-queue
// 5. On success, process response through callback if provided
func (t *transfer) postOrRetryMultiTransfer(q *Pack, timeout time.Duration) {

	// Create a logger entry with pack ID
	entry := logger.NewEntry().With(
		zap.String(logger.KeyFunctionName, "postOrRetryMultiTransfer"),
		zap.String("pack_id", q.ID),
	)

	// Check the partner queue status
	free, total, status := t.partner.GetConcurrentStatus()
	// Log queue status
	entry.Info("multi transfer queue status",
		zap.Int("free_slots", free),
		zap.Int("total_slots", total),
		zap.String("status", status))

	// If status is BUSY, wait for a slot in the multiQueue
	if status == "OCCUPIED" || status == "BUSY" {
		// Retry logic
		q.retry++
		q.retriedAt = time.Now()
		// Wait for a slot in the multiQueue
		if err := t.addQueueMultiTransfer(q); err != nil {
			entry.With(zap.Error(err)).
				Warn("timeout waiting for slot in multi transfer mode")

			// Call the callback with the error
			go q.callback(q.ID, nil, NewError(ErrQueueBusy, "queue operation timed out", nil))
			return
		}
		// Successfully queued the pack for later processing.
		// The callback will be called when the pack is processed in the queue.
		// So we just return here
		return
	}

	postFunc := func(b []byte) error {
		// process the response with the provided callback
		if q.callback == nil {
			return nil
		}
		// Call the callback with the response bytes
		go q.callback(q.ID, b, nil)
		return nil
	}

	// Accept status codes for which we won't retry
	opt := queue.AcceptStatus(400, 403, 404)

	// Context with timeout for the post operation
	ctx, cancel := context.WithTimeout(context.TODO(), timeout)
	defer cancel()

	// Send the payload to the partner
	if err := t.partner.PostWithFunc(ctx, postFunc, q.GetDestinationURL(), q.Payload, opt); err != nil {
		entry.Error("failed to send payload in multiQueueProcess",
			zap.String("pack_id", q.ID),
			zap.Error(err),
			zap.String(logger.KeyFunctionName, "postOrRetryMultiTransfer"))
		// Retry logic
		q.retry++
		q.retriedAt = time.Now()
		// Re-add to queue for retry
		go func(p *Pack) {
			if err := t.addQueueMultiTransfer(p); err != nil {
				entry.Info("re-queued pack in multiQueueProcess",
					zap.Error(err),
					zap.String("pack_id", p.ID),
					zap.Int("retry", p.retry),
					zap.String(logger.KeyFunctionName, "postOrRetryMultiTransfer"))
			}
		}(q)
		return
	}
	// Log success
	entry.Info("successfully sent pack in multiQueueProcess",
		zap.String("pack_id", q.ID),
		zap.String(logger.KeyFunctionName, "postOrRetryMultiTransfer"))
}
