package transfer

import "fmt"

// ErrorType represents the category of error that occurred during transfer operations.
// It helps in identifying and handling specific error scenarios appropriately.
type ErrorType string

const (
	// ErrPackValidation indicates errors related to pack validation
	ErrPackValidation ErrorType = "PACK_VALIDATION"
	// ErrQueueOccupied indicates when all queue slots are occupied
	ErrQueueOccupied ErrorType = "QUEUE_OCCUPIED"
	// ErrQueueBusy indicates when queue is busy and timeout occurred
	ErrQueueBusy ErrorType = "QUEUE_BUSY"
	// ErrTransferFailed indicates general transfer failures
	ErrTransferFailed ErrorType = "TRANSFER_FAILED"
	// ErrQueueTimeout indicates queue operation timeout
	ErrQueueTimeout ErrorType = "QUEUE_TIMEOUT"
)

// TransferError is a custom error type that provides detailed information about
// transfer operation failures, including the error type, message, and underlying error.
type TransferError struct {
	Type    ErrorType
	Message string
	Err     error
}

func (e *TransferError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("%s: %s: %v", e.Type, e.Message, e.Err)
	}
	return fmt.Sprintf("%s: %s", e.Type, e.Message)
}

func (e *TransferError) Unwrap() error {
	return e.Err
}

// NewError creates a new TransferError with the specified error type, message, and
// optional underlying error. This is the recommended way to create transfer errors.
func NewError(errType ErrorType, message string, err error) *TransferError {
	return &TransferError{
		Type:    errType,
		Message: message,
		Err:     err,
	}
}
