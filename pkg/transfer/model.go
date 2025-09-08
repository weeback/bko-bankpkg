package transfer

import (
	"fmt"
	"time"

	"github.com/google/uuid"
)

const (
	DefaultMaxConcurrent = 2
	DefaultMaxLiveTime   = 36 * time.Hour
	QueueTimeout         = 30 * time.Second
	MultiTransferWait    = 5 * time.Second
)

type Pack struct {
	ID        string
	Payload   []byte
	CreatedAt time.Time

	retry     int
	retriedAt time.Time
}

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
	if p.CreatedAt.IsZero() {
		p.CreatedAt = time.Now()
	}
	return nil
}
