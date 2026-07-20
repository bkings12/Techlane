package identity

import "testing"

func TestSplitVATInclusive(t *testing.T) {
	net, vat := SplitVAT(116, 1600, true)
	total := net + vat
	if total < 115.9 || total > 116.1 {
		t.Fatalf("inclusive split should sum to total: net=%.2f vat=%.2f total=%.2f", net, vat, total)
	}
}

func TestSplitVATExclusive(t *testing.T) {
	net, vat := SplitVAT(100, 1600, false)
	if net != 100 {
		t.Fatalf("expected net 100, got %.2f", net)
	}
	if vat != 16 {
		t.Fatalf("expected vat 16, got %.2f", vat)
	}
}
