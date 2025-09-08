package transfer

import (
	"testing"
	"time"
)

func TestPack_Fill(t *testing.T) {
	tests := []struct {
		name    string
		pack    *Pack
		wantErr bool
		errType ErrorType
	}{
		{
			name:    "nil pack",
			pack:    nil,
			wantErr: true,
			errType: ErrPackValidation,
		},
		{
			name:    "empty payload",
			pack:    &Pack{},
			wantErr: true,
			errType: ErrPackValidation,
		},
		{
			name: "valid pack with no ID or CreatedAt",
			pack: &Pack{
				Payload: []byte("test"),
			},
			wantErr: false,
		},
		{
			name: "valid pack with existing ID",
			pack: &Pack{
				ID:      "existing-id",
				Payload: []byte("test"),
			},
			wantErr: false,
		},
		{
			name: "valid pack with existing CreatedAt",
			pack: &Pack{
				Payload:   []byte("test"),
				CreatedAt: time.Now(),
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.pack.Fill()
			if (err != nil) != tt.wantErr {
				t.Errorf("Pack.Fill() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if err != nil {
				var tErr *TransferError
				if e, ok := err.(*TransferError); !ok {
					t.Errorf("Pack.Fill() error is not TransferError, got %T", err)
					return
				} else {
					tErr = e
				}

				if tErr.Type != tt.errType {
					t.Errorf("Pack.Fill() error type = %v, want %v", tErr.Type, tt.errType)
				}
				return
			}

			if !tt.wantErr {
				if tt.pack.ID == "" {
					t.Error("Pack.Fill() did not generate ID")
				}
				if tt.pack.CreatedAt.IsZero() {
					t.Error("Pack.Fill() did not set CreatedAt")
				}
			}
		})
	}
}
