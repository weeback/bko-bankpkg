// Package transfer provides functionality for managing single and multi-mode data transfers
// with configurable concurrency and retry mechanisms.
package transfer

import (
	"fmt"
	"time"

	"github.com/google/uuid"
)

const (
	DefaultMaxConcurrent = 2
	DefaultMaxLiveTime   = 36 * time.Hour
	RequestTimeout       = 30 * time.Second
	QueueTimeout         = 30 * time.Second
	MultiTransferWait    = 5 * time.Second
)

// Pack represents a data package that can be transferred, containing payload data
// and metadata for tracking and retry mechanisms.
type Pack struct {
	// ID is the unique identifier for the pack
	ID string
	// Payload contains the actual data to be transferred
	Payload []byte
	// Callback is an optional function to be called with the result of the transfer
	callback func(string, []byte, error) error
	// CreatedAt is the timestamp when the pack was created
	CreatedAt time.Time

	retry     int
	retriedAt time.Time
}

// Fill validates and initializes a Pack instance with default values.
// It performs the following operations:
// - Validates that the Pack instance and its Payload are not nil
// - Generates a UUID-based ID if not already set
// - Sets a no-op callback function if none is provided
// - Sets CreatedAt to current time if not already set
//
// Returns an error if the Pack is nil or has no Payload.
func (p *Pack) Fill() error {
	if p == nil {
		return NewError(ErrPackValidation, "pack cannot be nil", nil)
	}
	if p.Payload == nil {
		return NewError(ErrPackValidation, "payload cannot be empty", nil)
	}
	// auto fill ID and CreatedAt
	if p.ID == "" {
		p.ID = fmt.Sprintf("pack-%s", uuid.NewString())
	}
	if p.callback == nil {
		p.callback = func(string, []byte, error) error {
			return nil
		}
	}
	if p.CreatedAt.IsZero() {
		p.CreatedAt = time.Now()
	}
	return nil
}
