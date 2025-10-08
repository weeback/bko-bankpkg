package transfer

import (
	"context"
	"fmt"

	"github.com/weeback/bko-bankpkg/pkg/logger"
	"github.com/weeback/bko-bankpkg/pkg/queue"
	"go.uber.org/zap"
)

// SingleTransfer performs a single transfer operation by sending a payload to a partner.
// It validates the input data, checks queue availability, and processes the transfer.
//
// Parameters:
//   - ctx: The context for the transfer operation
//   - result: A pointer to store the transfer response
//   - destURL: The destination URL for the transfer, overriding any URL in the Pack
//   - data: The Pack containing the payload and transfer details
//
// Returns:
//   - error: ErrPackValidation if data validation fails
//   - error: ErrQueueOccupied if all partner slots are occupied
//   - error: ErrTransferFailed if payload transmission fails
//   - nil: on successful transfer
func (t *transfer) SingleTransfer(ctx context.Context, result any, destURL string, data *Pack) error {
	// Validate and fill data input
	if err := data.Fill(destURL); err != nil {
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

	// Accept status codes for which we won't retry
	opt := queue.AcceptStatus(400, 403, 404)

	// Log queue status
	logger.GetLoggerFromContext(ctx).Info("single transfer queue status",
		zap.Int("free_slots", free),
		zap.Int("total_slots", total),
		zap.String("status", status),
		zap.Any("options", opt),
		zap.String(logger.KeyFunctionName, "SingleTransfer"))

	// Send the payload to the partner
	if err := t.partner.Post(ctx, result, data.GetDestinationURL(), data.Payload, opt); err != nil {
		return NewError(ErrTransferFailed,
			fmt.Sprintf("failed to send payload for pack %s in single transfer mode", data.ID),
			err)
	}
	return nil
}
