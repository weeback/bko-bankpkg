package transfer

import (
	"context"
	"time"

	"github.com/weeback/bko-bankpkg/pkg/logger"
	"go.uber.org/zap"
)

func (t *transfer) MultiTransfer(ctx context.Context, result any, data *Pack) error {
	// Validate and fill data input
	if err := data.Fill(); err != nil {
		log := logger.GetLoggerFromContext(ctx).With(
			zap.String(logger.KeyFunctionName, "MultiTransfer"),
			zap.Error(err))
		log.Error("failed to validate and fill pack data for multi transfer")
		return NewError(ErrPackValidation, "failed to validate pack data", err)
	}

	// Check the partner queue status
	free, total, status := t.partner.GetConcurrentStatus()
	// Check status and handle accordingly
	if status == "OCCUPIED" {
		log := logger.GetLoggerFromContext(ctx).With(
			zap.String(logger.KeyFunctionName, "MultiTransfer"),
			zap.Int("total_slots", total))
		log.Warn("all slots are occupied in multi transfer mode")
		return NewError(ErrQueueOccupied, "queue is fully occupied", nil)
	}

	// If status is BUSY, wait for a slot in the multiQueue
	if status == "BUSY" {
		select {
		case <-t.addQueueMultiTransfer(data): // wait for a slot
			return nil
		case <-time.After(5 * time.Second):
			log := logger.GetLoggerFromContext(ctx).With(
				zap.String(logger.KeyFunctionName, "MultiTransfer"),
				zap.String("pack_id", data.ID))
			log.Warn("timeout waiting for slot in multi transfer mode")
			return NewError(ErrQueueBusy, "queue operation timed out", nil)
		}
	}

	// Log queue status
	logger.GetLoggerFromContext(ctx).Info("multi transfer queue status",
		zap.Int("free_slots", free),
		zap.Int("total_slots", total),
		zap.String("status", status),
		zap.String(logger.KeyFunctionName, "MultiTransfer"))

	// Send the payload to the partner
	if err := t.partner.Post(ctx, result, data.Payload); err != nil {
		log := logger.GetLoggerFromContext(ctx).With(
			zap.String(logger.KeyFunctionName, "MultiTransfer"),
			zap.String("pack_id", data.ID),
			zap.Error(err))
		log.Error("failed to send payload in multi transfer mode")
		return NewError(ErrTransferFailed, "failed to send payload", err)
	}
	return nil
}
