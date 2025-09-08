package transfer

import (
	"context"
	"testing"
)

func TestTransfer_SingleTransfer(t *testing.T) {
	tests := []struct {
		name      string
		pack      *Pack
		mockSetup func(*MockHTTP)
		wantErr   bool
		errType   ErrorType
	}{
		{
			name: "successful transfer",
			pack: &Pack{
				Payload: []byte("test data"),
			},
			mockSetup: func(m *MockHTTP) {},
			wantErr:   false,
		},
		{
			name:      "invalid pack",
			pack:      &Pack{},
			mockSetup: func(m *MockHTTP) {},
			wantErr:   true,
			errType:   ErrPackValidation,
		},
		{
			name: "queue occupied",
			pack: &Pack{
				Payload: []byte("test data"),
			},
			mockSetup: func(m *MockHTTP) {
				m.forceOccupied = true
			},
			wantErr: true,
			errType: ErrQueueOccupied,
		},
		{
			name: "transfer failed",
			pack: &Pack{
				Payload: []byte("test data"),
			},
			mockSetup: func(m *MockHTTP) {
				m.forceError = true
			},
			wantErr: true,
			errType: ErrTransferFailed,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := NewMockHTTP()
			tt.mockSetup(mock)

			tr := NewTransfer(mock)
			var result interface{}

			err := tr.SingleTransfer(context.Background(), result, tt.pack)
			if (err != nil) != tt.wantErr {
				t.Errorf("Transfer.SingleTransfer() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if err != nil {
				tErr, ok := err.(*TransferError)
				if !ok {
					t.Errorf("Transfer.SingleTransfer() error is not TransferError, got %T", err)
					return
				}
				if tErr.Type != tt.errType {
					t.Errorf("Transfer.SingleTransfer() error type = %v, want %v", tErr.Type, tt.errType)
				}

				// For validation errors, we shouldn't check queue status
				if tErr.Type == ErrPackValidation {
					return
				}
			}

			// Check if GetConcurrentStatus was called
			if mock.getConcStatusCount == 0 {
				t.Error("Transfer.SingleTransfer() did not check queue status")
			}

			// For successful transfers, verify Post was called
			if !tt.wantErr && mock.postCallCount == 0 {
				t.Error("Transfer.SingleTransfer() did not call Post for successful transfer")
			}
		})
	}
}
