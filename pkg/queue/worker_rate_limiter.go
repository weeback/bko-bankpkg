package queue

import (
	"fmt"
	"log"
	"math"
	"time"
)

// updateMaxConcurrent updates the limit of concurrent requests
func (w *worker) updateMaxConcurrent(maxConcurrent int) {
	if maxConcurrent <= 0 {
		// If the value is invalid or = 0 (no limit), do nothing
		return
	}

	// If the value is already set and equals to the current value, no need to change
	if w.maxConcurrent == maxConcurrent && w.activeSem != nil {
		return
	}

	// Update new limit
	w.maxConcurrent = maxConcurrent

	// Create a new semaphore if it doesn't exist or size has changed
	if w.activeSem == nil || cap(w.activeSem) != maxConcurrent {
		// Create a new semaphore with the new size
		w.activeSem = make(chan struct{}, maxConcurrent)
		log.Printf("Rate limiter updated: max concurrent requests = %d", maxConcurrent)
	}
}

func (w *worker) getConcurrentStatus() (int, int, string) {
	if w.maxConcurrent == 0 {
		return math.MaxInt, 0, "FREE"
	}
	n := w.maxConcurrent - len(w.activeSem)
	log.Printf("Current concurrent requests: %d/%d", n, w.maxConcurrent)
	status := "FREE"
	switch {
	case n == w.maxConcurrent:
		log.Printf("All slots are free")
		status = "FREE"
	case n > w.maxConcurrent/10:
		log.Printf("Many slots are free")
		status = "AVAILABLE"
	case n > 0:
		log.Printf("Some slots are free")
		status = "BUSY"
	default:
		log.Printf("All slots are occupied")
		status = "OCCUPIED"
	}
	return n, w.maxConcurrent, status
}

// acquireSemaphore gets a slot from the semaphore, returns a release function
// Returns error if unable to get a slot within the timeout period
func (w *worker) acquireSemaphore(timeout time.Duration) (release func(), err error) {
	// If there's no limit, return an empty function
	if w.maxConcurrent <= 0 || w.activeSem == nil {
		return func() {}, nil
	}

	// Try to acquire the semaphore with a timeout
	select {
	case w.activeSem <- struct{}{}:
		// Successfully acquired a slot
		return func() {
			// Function to release the slot
			select {
			case <-w.activeSem:
				// Slot released successfully
			default:
				// Semaphore might have been closed
			}
		}, nil
	case <-time.After(timeout):
		// Timeout while waiting for a slot
		return nil, fmt.Errorf("rate limit exceeded: too many concurrent requests")
	}
}
