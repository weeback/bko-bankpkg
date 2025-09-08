package transfer

import (
	"errors"
	"strings"
	"testing"
)

func TestTransferError(t *testing.T) {
	tests := []struct {
		name        string
		errType     ErrorType
		message     string
		wrappedErr  error
		wantMessage string
	}{
		{
			name:        "error without wrapped error",
			errType:     ErrPackValidation,
			message:     "validation failed",
			wrappedErr:  nil,
			wantMessage: "PACK_VALIDATION: validation failed",
		},
		{
			name:        "error with wrapped error",
			errType:     ErrTransferFailed,
			message:     "transfer failed",
			wrappedErr:  errors.New("network error"),
			wantMessage: "TRANSFER_FAILED: transfer failed: network error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := NewError(tt.errType, tt.message, tt.wrappedErr)

			// Test Error() method
			if got := err.Error(); got != tt.wantMessage {
				t.Errorf("TransferError.Error() = %v, want %v", got, tt.wantMessage)
			}

			// Test Unwrap() method
			if got := err.Unwrap(); got != tt.wrappedErr {
				t.Errorf("TransferError.Unwrap() = %v, want %v", got, tt.wrappedErr)
			}

			// Test error type
			if err.Type != tt.errType {
				t.Errorf("TransferError.Type = %v, want %v", err.Type, tt.errType)
			}

			// Test message
			if !strings.Contains(err.Message, tt.message) {
				t.Errorf("TransferError.Message = %v, want to contain %v", err.Message, tt.message)
			}
		})
	}
}
