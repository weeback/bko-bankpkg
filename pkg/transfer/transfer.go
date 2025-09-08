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
	MultiTransfer(ctx context.Context, result any, data *Pack) error
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

	// Validate input
	if val == nil {
		logger.NewEntry().Error("attempted to queue nil pack")
		return nil
	}

	// If queue is not initialized, re-initialize
	if t.queue == nil {
		// Queue is not initialized, should not happen
		logger.NewEntry().Error("queue is not initialized in addQueueMultiTransfer")

		t.once = sync.Once{} // Reset once to allow re-initialization
		t.queueState = 0

		return t.addQueueMultiTransfer(val) // Retry adding to queue
	}

	// Attempt to add to queue with timeout to avoid blocking indefinitely
	select {
	case t.queue <- val:
		logger.NewEntry().Info("re-queued pack in multiQueueProcess",
			zap.String("pack_id", val.ID),
			zap.Int("retry", val.retry),
			zap.String(logger.KeyFunctionName, "addQueueMultiTransfer"))
	case <-time.After(QueueTimeout):
		logger.NewEntry().Warn("dropped pack due to full queue in addQueueMultiTransfer",
			zap.String("pack_id", val.ID),
			zap.String(logger.KeyFunctionName, "addQueueMultiTransfer"))
		return fmt.Errorf("dropped pack due to full queue")
	}

	return nil
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
		if d := time.Since(q.retriedAt); d > t.maxLiveTime {
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
		t.postOrRetry(q, RequestTimeout)
	}
}

func (t *transfer) postOrRetry(q *Pack, timeout time.Duration) {

	// Context with timeout for the post operation
	ctx, cancel := context.WithTimeout(context.TODO(), timeout)
	defer cancel()

	// Send the payload to the partner
	if err := t.partner.Post(ctx, nil, q.Payload); err != nil {
		logger.NewEntry().Error("failed to send payload in multiQueueProcess",
			zap.String("pack_id", q.ID),
			zap.Error(err),
			zap.String(logger.KeyFunctionName, "queueMultiTransferProcess"))
		// Retry logic
		q.retry++
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
		return
	}
	// Log success
	logger.NewEntry().Info("successfully sent pack in multiQueueProcess",
		zap.String("pack_id", q.ID),
		zap.String(logger.KeyFunctionName, "queueMultiTransferProcess"))
}
