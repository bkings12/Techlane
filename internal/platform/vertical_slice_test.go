package platform_test

import (
	"context"
	"testing"

	"github.com/techlane/techlane/internal/payments"
	"github.com/techlane/techlane/internal/repair"
	"github.com/techlane/techlane/packages/pkg/idempotency"
)

func TestRepairStatusTransitions(t *testing.T) {
	if err := repair.ValidateStatusTransition(repair.StatusIntake, repair.StatusDiagnosed); err != nil {
		t.Fatal(err)
	}
	if err := repair.ValidateStatusTransition(repair.StatusIntake, repair.StatusCompleted); err == nil {
		t.Fatal("expected invalid transition")
	}
}

func TestHandoverSelfApprove(t *testing.T) {
	if err := payments.ValidateHandoverConfirm("a", "a"); err != payments.ErrSelfApproveHandover {
		t.Fatalf("expected ErrSelfApproveHandover, got %v", err)
	}
}

func TestIdempotencyMismatch(t *testing.T) {
	s := idempotency.NewStore()
	h1, _ := idempotency.HashBody(map[string]string{"x": "1"})
	h2, _ := idempotency.HashBody(map[string]string{"x": "2"})
	ctx := context.Background()
	_, _, _, err := s.BeginOrReplay(ctx, "key", h1)
	if err != nil {
		t.Fatal(err)
	}
	_ = s.Complete(ctx, "key", 200, []byte(`{}`))
	_, _, _, err = s.BeginOrReplay(ctx, "key", h2)
	if err != idempotency.ErrMismatch {
		t.Fatalf("expected mismatch, got %v", err)
	}
}
