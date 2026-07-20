package identity

import (
	"errors"
	"testing"

	"github.com/google/uuid"
)

func TestValidateTechnicianBranchAccess(t *testing.T) {
	branchA := uuid.New()
	branchB := uuid.New()

	tests := []struct {
		name       string
		technician bool
		branches   []uuid.UUID
		wantErr    bool
	}{
		{name: "technician with one branch", technician: true, branches: []uuid.UUID{branchA}},
		{name: "technician without branch", technician: true, wantErr: true},
		{name: "technician with two branches", technician: true, branches: []uuid.UUID{branchA, branchB}, wantErr: true},
		{name: "other role with many branches", branches: []uuid.UUID{branchA, branchB}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateTechnicianBranchAccess(tt.technician, tt.branches)
			if (err != nil) != tt.wantErr {
				t.Fatalf("validateTechnicianBranchAccess() error = %v, wantErr %v", err, tt.wantErr)
			}
			if err != nil && !errors.Is(err, ErrInvalidInput) {
				t.Fatalf("expected ErrInvalidInput, got %v", err)
			}
		})
	}
}
