package commerce_test

import (
	"testing"

	"github.com/techlane/techlane/internal/payments"
)

func TestCashOnPickupStartsPendingUntilStaffConfirm(t *testing.T) {
	if got := payments.InitialPaymentStatus("cash_on_pickup"); got != "pending" {
		t.Fatalf("cash_on_pickup status = %q, want pending", got)
	}
	if !payments.IsCashOnPickup("cash_on_pickup") {
		t.Fatal("expected IsCashOnPickup")
	}
	if payments.IsCashMethod("cash_on_pickup") {
		t.Fatal("cash_on_pickup must not use dual-control cash handover")
	}
}

func TestSTKStartsInitiatedForProviderCallback(t *testing.T) {
	if got := payments.InitialPaymentStatus("mpesa_stk"); got != "initiated" {
		t.Fatalf("mpesa_stk status = %q, want initiated", got)
	}
	if !payments.IsDigitalMethod("mpesa_stk") {
		t.Fatal("expected digital STK")
	}
}
