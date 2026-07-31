package receipts

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/google/uuid"
)

func sampleShop() Shop {
	return Shop{
		Name:         "Unganisha Tech Hub",
		TIN:          "P051234567X",
		AddressLines: []string{"Bihi Towers, 3rd floor", "Moi Avenue", "Nairobi"},
		Phone:        "+254 720 111 222",
		Email:        "hello@techlane.co.ke",
		Website:      "techlane.co.ke",
	}
}

func sampleSettings() Settings {
	set := DefaultSettings(uuid.New())
	set.HeaderNote = "Authorised device service centre"
	set.FooterText = "Goods once collected are checked and accepted by the customer."
	return set
}

func TestFormatMoney(t *testing.T) {
	cases := map[float64]string{
		0:          "0.00",
		5:          "5.00",
		1234.5:     "1,234.50",
		1000000:    "1,000,000.00",
		999.999:    "1,000.00",
		-1500.25:   "-1,500.25",
		12345678.9: "12,345,678.90",
	}
	for in, want := range cases {
		if got := formatMoney(in); got != want {
			t.Errorf("formatMoney(%v) = %q, want %q", in, got, want)
		}
	}
}

func TestNormalizePaper(t *testing.T) {
	cases := []struct{ in, fallback, want string }{
		{"80mm", "", PaperThermal80},
		{"58", "", PaperThermal58},
		{"A4", "", PaperA4},
		{"", PaperA4, PaperA4},
		{"nonsense", PaperThermal58, PaperThermal58},
		{"", "", PaperThermal80},
	}
	for _, c := range cases {
		if got := NormalizePaper(c.in, c.fallback); got != c.want {
			t.Errorf("NormalizePaper(%q, %q) = %q, want %q", c.in, c.fallback, got, c.want)
		}
	}
}

func TestRenderHTMLIncludesBrandingAndTotals(t *testing.T) {
	doc := SampleDocument(KindRepair, "KES", 1600, true)
	out := RenderHTML(sampleShop(), doc, sampleSettings(), PaperThermal80)

	for _, want := range []string{
		"Unganisha Tech Hub",
		"Authorised device service centre",
		"Bihi Towers, 3rd floor",
		"P051234567X",
		"Repair receipt",
		"356938035643809",
		"5,500.00",
		"Thank you for your business",
		"window.print()",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("thermal receipt missing %q", want)
		}
	}
}

func TestRenderHTMLRespectsToggles(t *testing.T) {
	set := sampleSettings()
	set.ShowIMEI = false
	set.ShowPayments = false
	set.ShowServedBy = false
	set.ShowVATBreakdown = false

	doc := SampleDocument(KindRepair, "KES", 1600, true)
	out := RenderHTML(sampleShop(), doc, set, PaperThermal80)

	for _, unwanted := range []string{"356938035643809", "SJ42KD9QM1", "Served by", "Subtotal (excl. VAT)"} {
		if strings.Contains(out, unwanted) {
			t.Errorf("receipt should not contain %q when the toggle is off", unwanted)
		}
	}
}

func TestApplySettingsDoesNotMutateSharedMeta(t *testing.T) {
	doc := SampleDocument(KindRepair, "KES", 1600, true)
	original := len(doc.Meta)
	set := sampleSettings()
	set.ShowIMEI = false

	copyDoc := doc
	copyDoc.applySettings(set)

	if len(doc.Meta) != original {
		t.Fatalf("source document meta was mutated: %d -> %d", original, len(doc.Meta))
	}
	if len(copyDoc.Meta) != original-1 {
		t.Fatalf("expected IMEI row to be dropped, got %d rows", len(copyDoc.Meta))
	}
}

func TestRenderPDFProducesDocument(t *testing.T) {
	for _, kind := range []string{KindRepair, KindSale, KindTaxInvoice} {
		doc := SampleDocument(kind, "KES", 1600, true)
		pdf, err := RenderPDF(sampleShop(), doc, sampleSettings())
		if err != nil {
			t.Fatalf("%s: %v", kind, err)
		}
		if len(pdf) < 1000 {
			t.Fatalf("%s: pdf looks empty (%d bytes)", kind, len(pdf))
		}
		if string(pdf[:4]) != "%PDF" {
			t.Fatalf("%s: missing PDF header", kind)
		}
	}
}

// TestWriteSamples is a developer aid: RECEIPT_SAMPLE_DIR=/tmp/receipts go test
// ./internal/receipts -run TestWriteSamples writes every layout out for review.
func TestWriteSamples(t *testing.T) {
	dir := os.Getenv("RECEIPT_SAMPLE_DIR")
	if dir == "" {
		t.Skip("set RECEIPT_SAMPLE_DIR to write sample receipts")
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	shop, set := sampleShop(), sampleSettings()
	for _, kind := range []string{KindRepair, KindSale, KindTaxInvoice, KindVoucher} {
		doc := SampleDocument(kind, "KES", 1600, true)
		if kind == KindVoucher {
			doc.Title = "Supplier credit voucher"
			doc.Callout = &Callout{Label: "Auth code for shop pickup", Value: "7F4K-92QD", Note: "techlane://issue/7f4k92qd"}
		}
		for _, paper := range []string{PaperThermal58, PaperThermal80, PaperA4} {
			name := filepath.Join(dir, kind+"-"+paper+".html")
			if err := os.WriteFile(name, []byte(RenderHTML(shop, doc, set, paper)), 0o644); err != nil {
				t.Fatal(err)
			}
		}
		pdf, err := RenderPDF(shop, doc, set)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, kind+".pdf"), pdf, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	t.Logf("wrote samples to %s", dir)
}

func TestPrettyStatusCustomerFacing(t *testing.T) {
	cases := map[string]string{
		"pending_handover": "Paid",
		"provisional":      "Paid",
		"allocated":        "Paid",
		"confirmed":        "Paid",
		"pending":          "Pending",
		"initiated":        "Pending",
		"failed":           "Failed",
		"refunded":         "Refunded",
		"":                 "",
	}
	for in, want := range cases {
		if got := prettyStatus(in); got != want {
			t.Errorf("prettyStatus(%q)=%q want %q", in, got, want)
		}
	}
}

func TestRenderHTMLCashPendingHandoverPrintsPaid(t *testing.T) {
	doc := SampleDocument(KindRepair, "KES", 1600, true)
	now := doc.IssuedAt
	doc.Payments = []PaymentLine{
		{Method: "cash", Amount: 5500, Status: "pending_handover", At: &now},
	}
	out := RenderHTML(sampleShop(), doc, sampleSettings(), PaperThermal80)
	if !strings.Contains(out, "Paid") {
		t.Fatal("receipt should show Paid for counter cash")
	}
	if strings.Contains(out, "Pending Handover") || strings.Contains(out, "pending_handover") {
		t.Fatal("receipt must not expose internal till-handover status to the customer")
	}
}
