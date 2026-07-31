package receipts

import (
	"strings"
	"time"
)

// SampleDocument powers the settings preview so an owner can see the effect of
// a change without printing a real job.
func SampleDocument(kind, currency string, vatRateBPS int, vatInclusive bool) Document {
	issued := time.Now()
	paidAt := issued.Add(-6 * time.Minute)

	doc := Document{
		Kind:          kind,
		Number:        "RCT-00042",
		IssuedAt:      issued,
		Currency:      currency,
		CustomerName:  "Amina Wanjiru",
		CustomerPhone: "+254 712 345 678",
		Branch:        "Main branch",
		ServedBy:      "Daniel K.",
		VATRateBPS:    vatRateBPS,
		VATInclusive:  vatInclusive,
	}

	switch kind {
	case KindSale:
		doc.Title = "Sales receipt"
		doc.Reference = "SL-2091"
		doc.Meta = []MetaRow{{Label: "Sale", Value: "SL-2091"}, {Label: "Channel", Value: "Counter"}}
		doc.Lines = []Line{
			{Description: "Tempered glass protector", Detail: "SKU TG-A12", Qty: 2, UnitPrice: 450, Amount: 900},
			{Description: "USB-C fast charger 25W", Detail: "SKU CH-25W", Qty: 1, UnitPrice: 1800, Amount: 1800},
			{Description: "Silicone case — black", Detail: "SKU CS-A12-BK", Qty: 1, UnitPrice: 800, Amount: 800},
		}
		doc.Total = 3500
		doc.Paid = 3500
		doc.Payments = []PaymentLine{
			{Method: "mpesa", Amount: 3500, Status: "confirmed", Reference: "SJ42KD9QM1", At: &paidAt},
		}
	case KindTaxInvoice:
		doc.Title = "Tax invoice"
		doc.Reference = "TL-1024"
		doc.Meta = []MetaRow{
			{Label: "Job", Value: "TL-1024"},
			{Label: "Device", Value: "Phone Samsung Galaxy A12"},
		}
		doc.Lines = []Line{
			{Description: "Labour — screen replacement", Amount: 2000},
			{Description: "Parts", Detail: "OEM LCD assembly", Amount: 3500},
		}
		doc.Total = 5500
		doc.Paid = 5500
	default:
		doc.Title = "Repair receipt"
		doc.Reference = "TL-1024"
		doc.Meta = []MetaRow{
			{Label: "Job", Value: "TL-1024"},
			{Label: "Device", Value: "Phone Samsung Galaxy A12"},
			{Label: "IMEI", Value: "356938035643809"},
			{Label: "Status", Value: "Ready for pickup"},
		}
		doc.Notes = "Cracked screen, touch unresponsive on the left edge."
		doc.Lines = []Line{
			{Description: "Labour — screen replacement", Amount: 2000},
			{Description: "Parts", Detail: "OEM LCD assembly", Amount: 3500},
		}
		doc.Total = 5500
		doc.Paid = 4000
		doc.Payments = []PaymentLine{
			{Method: "mpesa", Amount: 3000, Status: "confirmed", Reference: "SJ42KD9QM1", At: &paidAt},
			{Method: "cash", Amount: 1000, Status: "allocated", At: &paidAt},
		}
	}

	doc.Balance = doc.Total - doc.Paid
	if doc.Balance < 0 {
		doc.Balance = 0
	}
	doc.Subtotal, doc.VATAmount = SplitVAT(doc.Total, doc.VATRateBPS, doc.VATInclusive)
	return doc
}

// NormalizeKind keeps preview requests on a known document type.
func NormalizeKind(kind string) string {
	switch strings.ToLower(strings.TrimSpace(kind)) {
	case KindSale:
		return KindSale
	case KindTaxInvoice:
		return KindTaxInvoice
	case KindVoucher:
		return KindVoucher
	}
	return KindRepair
}
