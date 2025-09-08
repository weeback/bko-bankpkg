package transfer

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestTransfer_MultiTransfer(t *testing.T) {
	tests := []struct {
		name      string
		pack      *Pack
		mockSetup func(*MockHTTP)
		wantErr   bool
		errType   ErrorType
	}{
		{
			name: "successful immediate transfer",
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
			name: "queue busy",
			pack: &Pack{
				Payload: []byte("test data"),
			},
			mockSetup: func(m *MockHTTP) {
				m.forceBusy = true
			},
			wantErr: false, // Should queue the transfer
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

			err := tr.MultiTransfer(context.Background(), result, tt.pack)
			if (err != nil) != tt.wantErr {
				t.Errorf("Transfer.MultiTransfer() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if err != nil {
				tErr, ok := err.(*TransferError)
				if !ok {
					t.Errorf("Transfer.MultiTransfer() error is not TransferError, got %T", err)
					return
				}
				if tErr.Type != tt.errType {
					t.Errorf("Transfer.MultiTransfer() error type = %v, want %v", tErr.Type, tt.errType)
				}
			}

			if err != nil {
				tErr, ok := err.(*TransferError)
				if !ok {
					t.Errorf("Transfer.MultiTransfer() error is not TransferError, got %T", err)
					return
				}
				if tErr.Type != tt.errType {
					t.Errorf("Transfer.MultiTransfer() error type = %v, want %v", tErr.Type, tt.errType)
				}

				// For validation errors, we shouldn't check queue status
				if tErr.Type == ErrPackValidation {
					return
				}
			}

			// Check if GetConcurrentStatus was called
			if mock.getConcStatusCount == 0 {
				t.Error("Transfer.MultiTransfer() did not check queue status")
			}

			// For successful immediate transfers, verify Post was called
			if !tt.wantErr && !mock.forceBusy && mock.postCallCount == 0 {
				t.Error("Transfer.MultiTransfer() did not call Post for successful transfer")
			}
		})
	}
}

func TestTransfer_QueueMultiTransferProcess(t *testing.T) {
	mock := NewMockHTTP()
	tr := NewTransfer(mock).(*transfer) // Convert to concrete type for testing

	// Test queue initialization
	pack := &Pack{
		ID:      "test-1",
		Payload: []byte("test data"),
	}

	// Add a pack to the queue and ensure it gets processed
	err := tr.MultiTransfer(context.Background(), nil, pack)
	if err != nil {
		t.Fatalf("Failed to queue pack: %v", err)
	}

	// Test retrying with error
	mock.forceError = true
	pack = &Pack{
		ID:      "test-2",
		Payload: []byte("test data"),
	}

	// Add another pack that will fail
	err = tr.MultiTransfer(context.Background(), nil, pack)
	if err == nil {
		t.Error("Expected error when forcing mock error, got nil")
	}

	var tErr *TransferError
	if !errors.As(err, &tErr) || tErr.Type != ErrTransferFailed {
		t.Errorf("Expected TransferError with type ErrTransferFailed, got %v", err)
	}
}

func TestTransfer_QueueBusyTimeout(t *testing.T) {
	mock := NewMockHTTP()
	tr := NewTransfer(mock)

	// Initialize queue first
	pack1 := &Pack{
		ID:      "init-pack",
		Payload: []byte("test data"),
	}
	if err := tr.MultiTransfer(context.Background(), nil, pack1); err != nil {
		t.Fatalf("Failed to initialize queue: %v", err)
	}

	// Now force busy status
	mock.forceBusy = true

	// Create a pack for the busy test
	pack2 := &Pack{
		ID:      "test-1",
		Payload: []byte("test data"),
	}

	// This should timeout after 5 seconds
	startTime := time.Now()
	err := tr.MultiTransfer(context.Background(), nil, pack2)
	duration := time.Since(startTime)

	if err == nil {
		t.Error("Expected error due to queue busy timeout, got nil")
		return
	}

	var tErr *TransferError
	if !errors.As(err, &tErr) {
		t.Errorf("Expected TransferError, got %T: %v", err, err)
		return
	}

	if tErr.Type != ErrQueueBusy {
		t.Errorf("Expected error type %v, got %v", ErrQueueBusy, tErr.Type)
	}

	// Check if it timed out after approximately 5 seconds (with some margin)
	if duration < 4*time.Second || duration > 6*time.Second {
		t.Errorf("Expected timeout after ~5 seconds, took %v", duration)
	}
}
