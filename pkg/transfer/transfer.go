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
	SingleTransfer(ctx context.Context, result any, data *Pack) error

	// MultiTransfer handles transfer operations in a concurrent manner, with support
	// for queuing and retries. It may return immediately if the transfer is queued.
	MultiTransfer(ctx context.Context, result any, data *Pack) (bool, error)

	// MultiTransferWaitCallback performs batch transfer operations with callback-based results handling.
	// It processes multiple packs concurrently and executes the provided callback for each completed transfer.
	// The callback function is called with:
	// - id: the unique identifier of the processed pack
	// - payloadResponse: the response data from the transfer
	// - execError: any error that occurred during the transfer
	// Returns an error if the batch operation fails to start or if context is cancelled.
	MultiTransferWaitCallback(ctx context.Context, callback func(id string, payloadResponse []byte, execError error) error, data []Pack) error
}

// NewTransfer creates a new Transfer instance with the provided HTTP partner.
// It initializes the transfer with default concurrent limits and live time settings.
func NewTransfer(partner queue.HTTP) Transfer {
	return &transfer{
		partner:     partner,
		max:         DefaultMaxConcurrent, // default max concurrent for multi transfer
		maxLiveTime: DefaultMaxLiveTime,   // default max live time for a pack in multi transfer
	}
}

type transfer struct {
	partner queue.HTTP

	queue       chan *Pack
	queueState  int // 0: not started, 1: running
	max         int
	maxLiveTime time.Duration
	mu          sync.Mutex
	once        sync.Once
}

func (t *transfer) addQueueMultiTransfer(val *Pack) error {
	// Validate input first
	if val == nil {
		logger.NewEntry().Error("attempted to queue nil pack")
		return nil
	}

	t.mu.Lock()

	// Initialize queue if not already done
	if t.queue == nil {
		// Set max to half of partner's max concurrent, minimum 2
		size := t.max
		if size <= 0 {
			size = 2
		}
		t.queue = make(chan *Pack, size)
		t.queueState = 0
		t.once = sync.Once{}             // Reset once to allow restart if needed
		go t.queueMultiTransferProcess() // Start the processor
	}

	t.mu.Unlock()

	// Wait for queue processor to start
	startTime := time.Now()
	for time.Since(startTime) < QueueTimeout {
		t.mu.Lock()
		if t.queueState == 1 {
			t.mu.Unlock()
			break
		}
		t.mu.Unlock()
		time.Sleep(50 * time.Millisecond)
	}

	t.mu.Lock()
	currentQueue := t.queue
	t.mu.Unlock()

	// If queue still isn't running, return error
	if currentQueue == nil {
		return fmt.Errorf("queue initialization failed")
	}

	// Attempt to add to queue with timeout
	select {
	case currentQueue <- val:
		logger.NewEntry().Info("queued pack",
			zap.String("pack_id", val.ID),
			zap.Int("retry", val.retry),
			zap.String(logger.KeyFunctionName, "addQueueMultiTransfer"))
		return nil
	case <-time.After(QueueTimeout):
		logger.NewEntry().Warn("dropped pack due to full queue",
			zap.String("pack_id", val.ID),
			zap.String(logger.KeyFunctionName, "addQueueMultiTransfer"))
		return fmt.Errorf("dropped pack due to full queue")
	}
}

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

	// Handle errors when processing the queue
	defer func() {
		if r := recover(); r != nil {
			logger.NewEntry().Error("recovered from panic in multiQueueProcess",
				zap.Any("recover", r),
				zap.String(logger.KeyFunctionName, "queueMultiTransferProcess"))
		}
		// Restart the processor by resetting once
		t.mu.Lock()
		t.queueState = 0
		t.once = sync.Once{}
		t.mu.Unlock()
	}()

	for q := range currentQueue {
		// Drop old packs
		if d := time.Since(q.CreatedAt); d > t.maxLiveTime {
			logger.NewEntry().Warn("dropping old pack in multiQueueProcess",
				zap.String("pack_id", q.ID),
				zap.String("duration", d.String()),
				zap.Int("retries", q.retry),
				zap.String(logger.KeyFunctionName, "queueMultiTransferProcess"))
			continue
		}
		// Skip if recently retried
		if d := time.Since(q.retriedAt); d < MultiTransferWait {
			// Retry logic to avoid rapid retries
			q.retriedAt = time.Now()
			// Re-add to queue for retry
			go func(p *Pack) {
				if err := t.addQueueMultiTransfer(p); err != nil {
					logger.NewEntry().Info("re-queued pack in multiQueueProcess",
						zap.Error(err),
						zap.String("pack_id", p.ID),
						zap.Int("retry", p.retry),
						zap.String(logger.KeyFunctionName, "queueMultiTransferProcess"))
				}
			}(q)
			// Continue to next pack
			continue
		}

		// Send the payload to the partner
		t.postOrRetryMultiTransfer(q, RequestTimeout)
	}
}

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
			q.callback(q.ID, nil, NewError(ErrQueueBusy, "queue operation timed out", nil))
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
		return q.callback(q.ID, b, nil)
	}

	// Context with timeout for the post operation
	ctx, cancel := context.WithTimeout(context.TODO(), timeout)
	defer cancel()

	// Send the payload to the partner
	if err := t.partner.PostWithFunc(ctx, postFunc, q.Payload); err != nil {
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
