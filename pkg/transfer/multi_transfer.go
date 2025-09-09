package transfer

import (
	"context"

	"github.com/weeback/bko-bankpkg/pkg/logger"
	"go.uber.org/zap"
)

func (t *transfer) MultiTransfer(ctx context.Context, result any, data *Pack) (bool, error) {

	if data == nil {
		return false, NewError(ErrPackValidation, "pack cannot be nil", nil)
	}
	// Create a logger entry with context and pack ID
	entry := logger.GetLoggerFromContext(ctx).With(
		zap.String(logger.KeyFunctionName, "MultiTransfer"),
		zap.String("pack_id", data.ID),
	)

	// Validate and fill data input
	if err := data.Fill(); err != nil {
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

	// Send the payload to the partner
	if err := t.partner.Post(ctx, result, data.Payload); err != nil {
		entry.With(zap.Error(err)).
			Error("failed to send payload in multi transfer mode")
		return false, NewError(ErrTransferFailed, "failed to send payload", err)
	}
	return true, nil
}

func (t *transfer) MultiTransferWaitCallback(ctx context.Context, callback func(id string, payloadResponse []byte, execError error) error, data []Pack) error {

	// Validate all packs first
	for _, pack := range data {
		if err := pack.Fill(); err != nil {
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
		go func(p *Pack) {
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

					// Call the callback with the error
				callback(p.ID, nil, NewError(ErrQueueOccupied, "queue is fully occupied", nil))
				return
			}

			// If status is BUSY, wait for a slot in the multiQueue
			if status == "BUSY" {
				// Assign the callback if provided
				if callback != nil {
					pack.callback = callback
				}
				// Wait for a slot in the multiQueue
				if err := t.addQueueMultiTransfer(p); err != nil {
					entry.With(zap.Error(err)).
						Warn("timeout waiting for slot in multi transfer mode")

						// Call the callback with the error
					callback(p.ID, nil, NewError(ErrQueueBusy, "queue operation timed out", nil))
					return
				}
				// Successfully queued the pack for later processing.
				// The callback will be called when the pack is processed in the queue.
				// So we just return here
				return
			}

			fn := func(b []byte) error {
				// process the response with the provided callback
				if callback == nil {
					return nil
				}
				// Call the callback with the response bytes
				return callback(p.ID, b, nil)
			}

			// Send the payload to the partner
			if err := t.partner.PostWithFunc(ctx, fn, p.Payload); err != nil {
				entry.With(zap.Error(err)).
					Error("failed to send payload in multi transfer mode")

				// Call the callback with the error
				callback(p.ID, nil, NewError(ErrTransferFailed, "failed to send payload", err))
				return
			}

		}(&pack)
	}
	return nil
}
