package payments

import "testing"

func TestValidateHandoverConfirm(t *testing.T) {
	if err := ValidateHandoverConfirm("user-a", "user-b"); err != nil {
		t.Fatalf("expected ok: %v", err)
	}
	if err := ValidateHandoverConfirm("user-a", "user-a"); err != ErrSelfApproveHandover {
		t.Fatalf("expected self-approve error, got %v", err)
	}
}

func TestInitialPaymentStatus(t *testing.T) {
	if InitialPaymentStatus("cash") != "pending_handover" {
		t.Fatal("cash should be pending_handover")
	}
	if InitialPaymentStatus("cash_on_pickup") != "pending" {
		t.Fatal("cash_on_pickup should be pending until staff marks collected")
	}
	if InitialPaymentStatus("mpesa_stk") != "initiated" {
		t.Fatal("mpesa should be initiated")
	}
}
