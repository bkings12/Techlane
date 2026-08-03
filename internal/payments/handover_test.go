package payments

import "testing"

func TestInitialPaymentStatus(t *testing.T) {
	if InitialPaymentStatus("cash") != "confirmed" {
		t.Fatal("cash should be confirmed immediately")
	}
	if InitialPaymentStatus("cash_on_pickup") != "pending" {
		t.Fatal("cash_on_pickup should be pending until staff marks collected")
	}
	if InitialPaymentStatus("mpesa_stk") != "initiated" {
		t.Fatal("mpesa should be initiated")
	}
	if InitialPaymentStatus("store_credit") != "allocated" {
		t.Fatal("store credit should be allocated")
	}
}
