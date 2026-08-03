package receipts

import (
	"strings"
	"testing"
	"time"
)

func TestBuildESCPOSBasic(t *testing.T) {
	shop := Shop{
		Name:         "TechLane",
		Phone:        "0700",
		Email:        "hi@techlane.co.ke",
		Website:      "www.techlane.co.ke",
		TIN:          "P051234567X",
		AddressLines: []string{"Moi Avenue"},
	}
	doc := Document{
		Title:     "Sale receipt",
		Number:    "R-1",
		IssuedAt:  time.Date(2026, 8, 1, 13, 0, 0, 0, time.Local),
		Currency:  "KES",
		Lines:     []Line{{Description: "Cable", Qty: 1, UnitPrice: 200, Amount: 200}},
		Total:     200,
		Payments:  []PaymentLine{{Method: "mpesa_stk", Amount: 200, Reference: "UH1581PHDU"}},
	}
	set := Settings{
		HeaderNote:   "Gadgets & repairs",
		ThankYouText: "Asante",
		WarrantyText: "Keep this receipt for warranty",
		FooterText:   "techlane.co.ke",
		ShowPayments: true,
	}
	out := BuildESCPOS(shop, doc, set, PaperThermal80)
	s := string(out)
	for _, want := range []string{
		"TechLane", "Gadgets & repairs", "Moi Avenue", "hi@techlane.co.ke",
		"www.techlane.co.ke", "PIN/TIN", "Cable", "TOTAL", "M-Pesa", "Asante",
		"Keep this receipt", "techlane.co.ke",
	} {
		if !strings.Contains(s, want) {
			t.Fatalf("missing %q in %q", want, s)
		}
	}
	if strings.Count(s, "\r") > 0 {
		t.Fatal("CR overstrike should not be used (triples length on many POS-80s)")
	}
	if !strings.Contains(s, strings.Repeat("=", 42)) {
		t.Fatal("expected 42-col rule so body spans the 80mm paper")
	}
	// Near-max heat + density knobs (no CR overstrike, no GS ( K '1' leak).
	if !strings.Contains(s, string([]byte{0x1b, 0x37, 0x09, 0xff, 0x01})) {
		t.Fatal("missing high heat ESC 7")
	}
	if strings.Contains(s, string([]byte{0x1d, 0x28, 0x4b})) {
		t.Fatal("GS ( K leaks ASCII on POS-80 clones — do not use")
	}
	// Shop name must not be preceded by a stray printable from init cmds.
	idx := strings.Index(s, "TechLane")
	if idx < 1 {
		t.Fatal("TechLane missing")
	}
	prev := s[idx-1]
	if prev >= 0x20 && prev < 0x7f && prev != '\n' {
		t.Fatalf("printable junk %q immediately before shop name", prev)
	}
	foundCut := false
	for i := 0; i+2 < len(out); i++ {
		if out[i] == 0x1d && out[i+1] == 0x56 && out[i+2] == 0x01 {
			foundCut = true
			break
		}
	}
	if !foundCut {
		t.Fatal("missing cut command")
	}
}

func TestBuildESCPOSIncludesQR(t *testing.T) {
	out := BuildESCPOS(Shop{Name: "Shop"}, Document{
		Title: "Intake slip",
		Callout: &Callout{
			Label:     "Scan when collecting",
			QRPayload: "techlane://repair-pickup/PK-ABC123",
			Note:      "SMS has your code",
		},
	}, Settings{ThankYouText: "Thanks"}, PaperThermal80)
	// GS v 0 raster header
	if !strings.Contains(string(out), string([]byte{0x1d, 0x76, 0x30, 0x00})) {
		t.Fatal("missing GS v 0 QR raster")
	}
	if !strings.Contains(string(out), "Scan when collecting") {
		t.Fatal("missing QR label")
	}
}

// TestBuildESCPOS58mm pins the MTP-II width: every text row must fit inside
// the printer's actual 32-column paper, not the 42-column layout tuned for
// 80mm counter printers.
func TestBuildESCPOS58mm(t *testing.T) {
	shop := Shop{Name: "TechLane", Phone: "0700000000", AddressLines: []string{"Moi Avenue, Nairobi"}}
	doc := Document{
		Title:         "Sale receipt",
		Number:        "R-1",
		Currency:      "KES",
		CustomerName:  "A Very Long Customer Name Indeed",
		CustomerPhone: "0712345678",
		Branch:        "Main Branch Downtown",
		Lines: []Line{
			{Description: "Tempered glass screen protector", Qty: 2, UnitPrice: 500, Amount: 1000},
			{Description: "USB-C Cable", Qty: 1, UnitPrice: 200, Amount: 200},
		},
		Total:    1200,
		Paid:     1200,
		Payments: []PaymentLine{{Method: "cash", Amount: 1200}},
	}
	set := Settings{ThankYouText: "Asante", ShowPayments: true, ShowBalance: true}

	out := BuildESCPOS(shop, doc, set, PaperThermal58)
	s := string(out)

	if !strings.Contains(s, strings.Repeat("=", 32)) {
		t.Fatal("expected 32-col rule for 58mm paper")
	}
	if strings.Contains(s, strings.Repeat("=", 42)) {
		t.Fatal("80mm-width rule leaked into a 58mm receipt")
	}
	// Rows are measured from their known text marker rather than by
	// blanket-splitting on "\n": a row can share its line with a preceding
	// ESC/GS command whose parameter bytes happen to fall in printable ASCII
	// range (e.g. ESC 'a' 0x00 for left-align), which isn't printed text.
	for _, marker := range []string{"Customer", "Phone", "Branch", "TOTAL", "Paid", "Tempered glass", "USB-C Cable"} {
		row := rowContaining(t, s, marker)
		if len(row) > 32 {
			t.Fatalf("%q row exceeds 32 cols on 58mm paper: %q (%d chars)", marker, row, len(row))
		}
	}
}

// rowContaining returns the text from marker to the next newline (or end of
// string), failing the test if marker isn't present.
func rowContaining(t *testing.T, s, marker string) string {
	t.Helper()
	idx := strings.Index(s, marker)
	if idx < 0 {
		t.Fatalf("expected %q in receipt", marker)
	}
	rest := s[idx:]
	if nl := strings.IndexByte(rest, '\n'); nl >= 0 {
		rest = rest[:nl]
	}
	return rest
}

