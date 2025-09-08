package queue

import (
	"math"
	"net/url"
	"sync"
	"time"

	"github.com/weeback/bko-bankpkg/pkg/logger"
)

const maxCounter int = 1_000_000

// counter represents a server endpoint with tracking metrics
type counter struct {
	label       string        // unique identifier for this endpoint
	count       int           // number of times this endpoint has been used
	took        time.Duration // latency measurement for this endpoint
	tookUpdated time.Time
	url         *url.URL   // full URL of the endpoint
	mu          sync.Mutex // mutex for thread-safe access to count and took
}

func (c *counter) increment() {

	c.mu.Lock()
	defer c.mu.Unlock() // Use defer to ensure unlock is called

	// Check for overflow
	if c.count == math.MaxInt {
		c.count = 1
		return
	}

	// reset the counter if it is too large
	if c.count > maxCounter {
		c.count = 1
		return
	}

	// increment the counter
	c.count++
}

func (c *counter) decreasing() {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Check for underflow
	if c.count == 0 {
		c.count = 0
		return
	}

	// decreasing the counter
	c.count--
}

func (c *counter) resetCountersIfNeeded() {
	c.mu.Lock()
	defer c.mu.Unlock() // Sử dụng defer để đảm bảo unlock được gọi

	if c.count > maxCounter {
		logger.NewEntry().Debug("reset the counter")
		c.count = 0
	}
}

func (c *counter) tookWithLabel(label string, took time.Duration) {
	if c.label != label {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.took = took
	c.tookUpdated = time.Now()
}

func (c *counter) resetTookIfNeeded(expired time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if time.Since(c.tookUpdated) > expired {
		c.took = 0
		c.tookUpdated = time.Now()
	}
}
