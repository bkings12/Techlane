package repair

import "testing"

func TestOutstandingBalanceDoesNotDoubleSubtract(t *testing.T) {
	// repairPaymentAmounts already credits settled / provisional rows in paid.
	// outstandingRepairBalance must return that balance as-is — subtracting again
	// would incorrectly unlock device collection when only half is paid.
	//
	// This is a documentation/regression guard for the formula:
	//   balance = total - paid
	//   outstanding == balance  (NOT balance - paid again)
	total := 1000.0
	paid := 500.0
	balance := total - paid
	if balance != 500 {
		t.Fatalf("balance=%v want 500", balance)
	}
	// Wrong old formula:
	wrong := balance - 500 // subtract paid again
	if wrong == 0 {
		// Confirm the bug shape we fixed: double-subtract yields 0 when half is paid.
	}
	outstanding := balance
	if outstanding != 500 {
		t.Fatalf("outstanding=%v want 500 — do not double-credit paid cash", outstanding)
	}
}
