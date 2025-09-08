package transfer

import (
	"testing"
	"time"
)

func TestNewTransfer(t *testing.T) {
	mock := NewMockHTTP()
	tr := NewTransfer(mock)

	if tr == nil {
		t.Fatal("NewTransfer() returned nil")
	}

	// Test default values
	transfer, ok := tr.(*transfer)
	if !ok {
		t.Fatal("NewTransfer() did not return *transfer type")
	}

	if transfer.max != DefaultMaxConcurrent {
		t.Errorf("NewTransfer() max = %v, want %v", transfer.max, DefaultMaxConcurrent)
	}

	if transfer.maxLiveTime != DefaultMaxLiveTime {
		t.Errorf("NewTransfer() maxLiveTime = %v, want %v", transfer.maxLiveTime, DefaultMaxLiveTime)
	}

	if transfer.partner == nil {
		t.Error("NewTransfer() partner is nil")
	}
}

func TestTransfer_Constants(t *testing.T) {
	tests := []struct {
		name     string
		constant time.Duration
		want     time.Duration
	}{
		{
			name:     "DefaultMaxLiveTime",
			constant: DefaultMaxLiveTime,
			want:     36 * time.Hour,
		},
		{
			name:     "QueueTimeout",
			constant: QueueTimeout,
			want:     30 * time.Second,
		},
		{
			name:     "MultiTransferWait",
			constant: MultiTransferWait,
			want:     5 * time.Second,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.constant != tt.want {
				t.Errorf("%s = %v, want %v", tt.name, tt.constant, tt.want)
			}
		})
	}
}
