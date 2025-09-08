package transfer

import (
	"context"
	"fmt"
	"time"
)

func (t *transfer) MultiTransfer(ctx context.Context, result any, data *Pack) error {
	// Validate and fill data input
	if err := data.Fill(); err != nil {
		return NewError(ErrPackValidation, "failed to validate and fill pack data for multi transfer", err)
	}
	
	// Check the partner queue status
	free, total, status := t.partner.GetConcurrentStatus()
	// Check status and handle accordingly
	if status == "OCCUPIED" {
		return NewError(ErrQueueOccupied, 
			fmt.Sprintf("all %d slots are occupied in multi transfer mode", total),
			nil)
	}

	// If status is BUSY, wait for a slot in the multiQueue
	if status == "BUSY" {
		select {
		case <-t.addQueueMultiTransfer(data): // wait for a slot
			return nil
		case <-time.After(5 * time.Second):
			return NewError(ErrQueueBusy,
				fmt.Sprintf("timeout waiting for slot in multi transfer mode for pack %s", data.ID),
				nil)
		}
	}

	// Log queue status
	fmt.Printf("MULTI mode: free=%d, total=%d, status=%s\n", free, total, status)

	// Send the payload to the partner
	if err := t.partner.Post(ctx, result, data.Payload); err != nil {
		return NewError(ErrTransferFailed,
			fmt.Sprintf("failed to send payload for pack %s in multi transfer mode", data.ID),
			err)
	}
	return nil
}
