package repair

import (
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestIntakeSlipHTMLIsShortAndHasPickup(t *testing.T) {
	doc := &CustomerReceiptDocument{
		RepairID:       uuid.New(),
		ShopName:       "Demo Phone Lab",
		ShopSlogan:     "Your Trusted IT Partner",
		ShopPhone:      "0723433660",
		ShopEmail:      "support@techlane.co.ke",
		BranchName:     "CBD",
		JobCode:        "JOB-200",
		PickupCode:     "PK-ABC123",
		ProblemSummary: "No power",
		CustomerName:   "Ada",
		CustomerPhone:  "0712345678",
		DeviceLabel:    "phone Samsung A14",
		LaborAmount:    2500,
		PartsAmount:    800,
		TotalDue:       3300,
		Currency:       "KES",
		IssuedAt:       time.Date(2026, 7, 26, 15, 4, 0, 0, time.UTC),
	}
	html := doc.IntakeSlipHTML()
	for _, want := range []string{
		"Intake slip",
		"JOB-200",
		"Demo Phone Lab",
		"Your Trusted IT Partner",
		"0723433660",
		"support@techlane.co.ke",
		"<!--email_off-->",
		"<!--/email_off-->",
		"Ada",
		"No power",
		"Pickup QR",
		"sent by SMS",
		"Keep this slip for collection",
	} {
		if !strings.Contains(html, want) {
			t.Fatalf("intake slip missing %q", want)
		}
	}
	// Printed PK- code stays on SMS only — slip shows the QR.
	for _, ban := range []string{
		"PK-ABC123", "Pickup code", "Labour", "Labor", "Parts", "VAT", "Total due", "Balance", "Subtotal", "Payment",
	} {
		if strings.Contains(html, ban) {
			t.Fatalf("intake slip should not include %q", ban)
		}
	}
	if !strings.Contains(html, "techlane://repair-pickup/") && !strings.Contains(html, "data:image/png") {
		t.Fatal("intake slip should embed a pickup QR image")
	}
}
