package payments

import "testing"

func TestStkPushTargetsBankRoutesWithMpesaAuth(t *testing.T) {
	raw := rawSettings{
		Shortcode:   "174379",
		BankEnabled: true,
		BankPaybill: "222222",
		BankAccount: "ACC1234567890EXTRA",
	}
	auth, partyB, acct := stkPushTargets(raw, "POS-ABCDEFGH")
	if auth != "174379" {
		t.Fatalf("expected M-Pesa auth shortcode, got %q", auth)
	}
	if partyB != "222222" {
		t.Fatalf("expected bank paybill PartyB, got %q", partyB)
	}
	if acct != "ACC123456789" { // truncated to 12
		t.Fatalf("expected truncated bank account, got %q", acct)
	}
}

func TestStkPushTargetsMpesaFallback(t *testing.T) {
	raw := rawSettings{Shortcode: "174379"}
	auth, partyB, acct := stkPushTargets(raw, "POS-ABCDEFGH")
	if auth != "174379" || partyB != "174379" {
		t.Fatalf("expected mpesa shortcode for auth and PartyB, got %q / %q", auth, partyB)
	}
	if acct != "POS-ABCDEFGH" {
		t.Fatalf("expected sale account ref, got %q", acct)
	}
}
