package transfer

import (
	"context"

	"github.com/weeback/bko-bankpkg/pkg/logger"
	"github.com/weeback/bko-bankpkg/pkg/queue"
	"go.uber.org/zap"
)

// MultiTransfer handles the transfer of a single pack in a concurrent environment with queue management.
// It validates the input pack, checks the partner queue status, and either processes the transfer
// immediately or queues it based on the current queue state.
//
// Parameters:
//   - ctx: Context for the operation, which should contain a logger
//   - result: Pointer to store the response from the partner
//   - destURL: The destination URL for the transfer, overriding any URL in the Pack
//   - data: Pack containing the payload and metadata for transfer
//
// Returns:
//   - bool: true if the transfer was processed immediately, false if queued or failed
//   - error: nil on success, or one of:
//   - ErrPackValidation if pack is nil or validation fails
//   - ErrQueueOccupied if all slots are occupied
//   - ErrQueueBusy if queuing times out
//   - ErrTransferFailed if payload sending fails
//
// The function handles three queue states:
//   - "OCCUPIED": All slots are taken, returns error immediately
//   - "BUSY": Some slots are available, queues the request
//   - Otherwise: Processes the transfer immediately
//
// Deprecated: MultiTransfer is deprecated and will be removed in a future version.
// Use MultiTransferWaitCallback instead, which provides better handling of concurrent transfers
// and callback-based response handling.
func (t *transfer) MultiTransfer(ctx context.Context, result any, destURL string, data *Pack) (bool, error) {

	if data == nil {
		return false, NewError(ErrPackValidation, "pack cannot be nil", nil)
	}
	// Create a logger entry with context and pack ID
	entry := logger.GetLoggerFromContext(ctx).With(
		zap.String(logger.KeyFunctionName, "MultiTransfer"),
		zap.String("pack_id", data.ID),
	)

	// Validate and fill data input
	if err := data.Fill(destURL); err != nil {
		entry.With(zap.Error(err)).
			Error("failed to validate and fill pack data for multi transfer")
		return false, NewError(ErrPackValidation, "failed to validate pack data", err)
	}

	// Check the partner queue status
	free, total, status := t.partner.GetConcurrentStatus()
	// Log queue status
	entry.Info("multi transfer queue status",
		zap.Int("free_slots", free),
		zap.Int("total_slots", total),
		zap.String("status", status))

	// Check status and handle accordingly
	if status == "OCCUPIED" {
		entry.With(zap.Int("total_slots", total), zap.String("status", status)).
			Warn("all slots are occupied in multi transfer mode")
		return false, NewError(ErrQueueOccupied, "queue is fully occupied", nil)
	}

	// If status is BUSY, wait for a slot in the multiQueue
	if status == "BUSY" {
		if err := t.addQueueMultiTransfer(data); err != nil {
			entry.With(zap.Error(err)).
				Warn("timeout waiting for slot in multi transfer mode")
			return false, NewError(ErrQueueBusy, "queue operation timed out", nil)
		}
		// Successfully queued the pack for later processing
		// Return false to indicate it was queued, not sent immediately
		return false, nil
	}

	// Accept status codes for which we won't retry
	opt := queue.AcceptStatus(400, 403, 404)

	// Send the payload to the partner
	if err := t.partner.Post(ctx, result, data.GetDestinationURL(), data.Payload, opt); err != nil {
		entry.With(zap.Error(err)).
			Error("failed to send payload in multi transfer mode")
		return false, NewError(ErrTransferFailed, "failed to send payload", err)
	}
	return true, nil
}

// MultiTransferWaitCallback processes multiple packs in a concurrent environment with callback functionality.
// It validates each pack in the input slice and queues them for processing. For each pack, the provided
// callback function will be called upon completion or error.
//
// Parameters:
//
//   - ctx: Context for the operation, which should contain a logger
//
//   - callback: Function to be called for each pack after processing or on error.
//     The callback receives:
//     `id` is the pack ID,
//     `payloadResponse` is response bytes from successful processing,
//     `execError` is any error that occurred during processing.
//
//   - destURL: The destination URL for the transfer, overriding any URL in the Pack
//
//   - data: Slice of Packs to be processed
//
// Returns:
//   - error: nil on successful queueing of all packs, or:
//   - ErrPackValidation if the pack slice is empty or validation fails for any pack
//
// The function will:
//  1. Validate all packs before processing
//  2. For each pack:
//     - Set the provided callback or use a no-op callback if nil
//     - Queue the pack for processing
//     - If queueing fails, execute callback with ErrQueueBusy
//  3. Continue processing remaining packs even if some fail
func (t *transfer) MultiTransferWaitCallback(ctx context.Context,
	callback func(id string, payloadResponse []byte, execError error),
	destURL string, data []*Pack) error {
	if len(data) == 0 {
		return NewError(ErrPackValidation, "empty pack list", nil)
	}
	// Validate all packs first
	for _, pack := range data {
		if err := pack.Fill(destURL); err != nil {
			logger.GetLoggerFromContext(ctx).With(
				zap.String(logger.KeyFunctionName, "MultiTransferWaitCallback"),
				zap.Error(err),
			).Error("failed to validate and fill pack data for multi transfer")

			return NewError(ErrPackValidation, "failed to validate pack data", err)
		}
	}

	// Process each pack
	for _, pack := range data {
		// Create a logger entry with context and pack ID
		entry := logger.GetLoggerFromContext(ctx).With(
			zap.String(logger.KeyFunctionName, "MultiTransferWaitCallback"),
			zap.String("pack_id", pack.ID),
		)

		// Assign the callback if provided
		if callback != nil {
			pack.callback = callback
		} else {
			// Fallback to no-op if callback is nil
			pack.callback = defaultCallback
		}
		// Wait for a slot in the multiQueue
		if err := t.addQueueMultiTransfer(pack); err != nil {
			entry.With(zap.Error(err)).
				Warn("timeout waiting for slot in multi transfer mode")

				// Call the callback with the error
			go pack.callback(pack.ID, nil, NewError(ErrQueueBusy, "queue operation timed out", err))
		}
	}

	return nil
}
