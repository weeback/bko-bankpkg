// Package transfer provides functionality for managing single and multi-mode data transfers
// with configurable concurrency and retry mechanisms.
package transfer

import (
	"fmt"
	"net/url"
	"time"

	"github.com/google/uuid"
	"github.com/weeback/bko-bankpkg/pkg/logger"
)

const (
	DefaultMaxConcurrent = 2
	DefaultMaxRetries    = 3
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
	// destination is URL of the destination for the transfer
	destination *url.URL
	// Callback is an optional function to be called with the result of the transfer
	callback func(string, []byte, error)
	// CreatedAt is the timestamp when the pack was created
	CreatedAt time.Time

	retry     int
	retriedAt time.Time
}

// Fill validates and initializes a Pack instance with default values.
// It performs the following operations:
// - Validates that the Pack instance and its Payload are not nil
// - Generates a UUID-based ID if not already set
// - Overrides destURL if a non-empty URL is provided
// - Sets a no-op callback function if none is provided
// - Sets CreatedAt to current time if not already set
//
// Returns an error if the Pack is nil or has no Payload.
func (p *Pack) Fill(destURL string) error {
	if p == nil {
		return NewError(ErrPackValidation, "pack cannot be nil", nil)
	}
	if p.Payload == nil {
		return NewError(ErrPackValidation, "payload cannot be empty", nil)
	}
	// Override destURL if provided
	if len(destURL) > 0 {
		parsedURL, err := url.Parse(destURL)
		if err != nil {
			return NewError(ErrPackValidation, "invalid destination URL", err)
		}
		p.destination = parsedURL
	}
	// Set no-op callback if none provided
	if p.callback == nil {
		p.callback = func(string, []byte, error) {
			logger.NewEntry().Info("no-op callback executed")
		}
	}
	// auto fill ID and CreatedAt
	if p.ID == "" {
		p.ID = fmt.Sprintf("pack-%s", uuid.NewString())
	}
	if p.CreatedAt.IsZero() {
		p.CreatedAt = time.Now()
	}
	return nil
}

func (p *Pack) GetDestinationURL() string {
	if p.destination != nil {
		return p.destination.String()
	}
	return "https://<empty>.invalid/destination-not-set"
}
