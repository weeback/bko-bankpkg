package transfer

import (
	"context"
	"fmt"
)

func (t *transfer) SingleTransfer(ctx context.Context, result any, data *Pack) error {
	// Validate and fill data input
	if err := data.Fill(); err != nil {
		return NewError(ErrPackValidation, "failed to validate and fill pack data", err)
	}

	// Check the partner queue status
	free, total, status := t.partner.GetConcurrentStatus()
	// Check status and handle accordingly
	if status == "OCCUPIED" {
		return NewError(ErrQueueOccupied,
			fmt.Sprintf("all %d slots are occupied in single transfer mode", total),
			nil)
	}

	// Log queue status
	fmt.Printf("SINGLE mode: free=%d, total=%d, status=%s\n", free, total, status)

	// Send the payload to the partner
	if err := t.partner.Post(ctx, result, data.Payload); err != nil {
		return NewError(ErrTransferFailed,
			fmt.Sprintf("failed to send payload for pack %s in single transfer mode", data.ID),
			err)
	}
	return nil
}
