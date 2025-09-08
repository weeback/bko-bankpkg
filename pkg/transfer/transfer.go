package transfer

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/weeback/bko-bankpkg/pkg/queue"
)

type Transfer interface {
	SingleTransfer(ctx context.Context, result any, data *Pack) error
	MultiTransfer(ctx context.Context, result any, data *Pack) error
}

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

func (t *transfer) addQueueMultiTransfer(val *Pack) chan struct{} {
	t.once.Do(func() {
		t.mu.Lock()
		defer t.mu.Unlock()
		// Set max to half of partner's max concurrent, minimum 2
		if t.max > 0 {
			t.queue = make(chan *Pack, t.max)
		} else {
			t.queue = make(chan *Pack, 2)
		}
		// Start the queue processor
		go t.queueMultiTransferProcess()
	})

	//
	var ok = make(chan struct{}, 1)
	go func() {
		t.queue <- val
		ok <- struct{}{}
	}()

	return ok
}

func (t *transfer) queueMultiTransferProcess() {

	// Ensure only one instance is running
	t.mu.Lock()
	if t.queueState == 1 {
		// Already running
		t.mu.Unlock()
		return
	}
	t.queueState = 1
	t.mu.Unlock()

	// Handle errors when processing the queue
	defer func() {
		if r := recover(); r != nil {
			fmt.Printf("Recovered in multiQueueProcess: %v\n", r)
		}
		// Restart the processor by resetting once
		t.mu.Lock()
		t.once = sync.Once{}
		t.mu.Unlock()
	}()

	for q := range t.queue {
		// Drop old packs
		if d := time.Since(q.retriedAt); d > t.maxLiveTime {
			fmt.Printf("multiQueueProcess: dropping pack %s after %s and %d retries\n", q.ID, d.String(), q.retry)
			continue
		}
		// Send the payload to the partner
		if err := t.partner.Post(context.Background(), nil, q.Payload); err != nil {
			fmt.Printf("multiQueueProcess: failed to send payload for pack %s: %v\n", q.ID, err)
			// Retry logic
			q.retry++
			q.retriedAt = time.Now()
			// Re-add to queue for retry
			go func(p *Pack) {
				select {
				case <-t.addQueueMultiTransfer(p):
					fmt.Printf("multiQueueProcess: re-queued pack %s for retry %d\n", p.ID, p.retry)
				case <-time.After(QueueTimeout):
					fmt.Printf("multiQueueProcess: dropped pack %s due to full queue\n", p.ID)
				}
			}(q)
			continue
		}
		fmt.Printf("multiQueueProcess: successfully sent pack %s\n", q.ID)
	}
}
