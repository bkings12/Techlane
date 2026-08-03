package repair

import (
	"strings"

	"github.com/techlane/techlane/internal/receipts"
)

// ToReceiptDocument maps a repair receipt onto the shared printable model so
// repair jobs, POS sales and invoices all render from one design.
func (d *CustomerReceiptDocument) ToReceiptDocument(taxInvoice bool) receipts.Document {
	kind, title := receipts.KindRepair, "Repair receipt"
	if taxInvoice {
		kind, title = receipts.KindTaxInvoice, "Tax invoice"
	}

	doc := receipts.Document{
		Kind:          kind,
		Title:         title,
		Reference:     d.JobCode,
		IssuedAt:      d.IssuedAt,
		Currency:      d.Currency,
		CustomerName:  d.CustomerName,
		CustomerPhone: d.CustomerPhone,
		Notes:         d.ProblemSummary,
		Subtotal:      d.NetSubtotal,
		VATAmount:     d.VATAmount,
		VATRateBPS:    d.VATRateBPS,
		VATInclusive:  d.VATInclusive,
		Total:         d.TotalDue,
		Paid:          d.Paid,
		Balance:       d.Balance,
		Branch:        d.BranchName,
		ServedBy:      d.TechnicianName,
	}

	doc.Meta = []receipts.MetaRow{
		{Label: "Job", Value: d.JobCode},
		{Label: "Device", Value: capitalize(d.DeviceLabel)},
	}
	if d.IMEI != "" {
		doc.Meta = append(doc.Meta, receipts.MetaRow{Label: "IMEI / serial", Value: d.IMEI})
	}
	if d.Status != "" {
		doc.Meta = append(doc.Meta, receipts.MetaRow{
			Label: "Status",
			Value: capitalize(strings.ReplaceAll(d.Status, "_", " ")),
		})
	}

	if d.LaborAmount > 0 || (d.PartsAmount == 0 && d.SaleLinesTotal == 0) {
		doc.Lines = append(doc.Lines, receipts.Line{Description: "Labour", Amount: d.LaborAmount})
	}
	if d.PartsAmount > 0 {
		doc.Lines = append(doc.Lines, receipts.Line{Description: "Parts", Amount: d.PartsAmount})
	}
	if d.SaleLinesTotal > 0 {
		doc.Lines = append(doc.Lines, receipts.Line{Description: "Accessories / extras", Amount: d.SaleLinesTotal})
	}

	for _, p := range d.Payments {
		ref := ""
		if p.ProviderRef != nil {
			ref = receipts.CustomerPaymentRef(*p.ProviderRef)
		}
		at := p.CreatedAt
		doc.Payments = append(doc.Payments, receipts.PaymentLine{
			Method:    p.Method,
			Amount:    p.Amount,
			Status:    p.Status,
			Reference: ref,
			At:        &at,
		})
	}

	if d.PickupCode != "" && d.Status != "collected" {
		payload := PickupQRPayload(d.PickupCode)
		callout := &receipts.Callout{
			Label:     "Bring this code when collecting",
			Value:     d.PickupCode,
			Note:      "Scan the QR or type the code at the counter. Device is released only after payment.",
			QRPayload: payload,
		}
		if uri, err := receipts.QRDataURIPNG(payload); err == nil {
			callout.QRDataURI = uri
		}
		doc.Callout = callout
	}
	return doc
}

// ToIntakeDocument is the short check-in ticket for thermal printers (no prices).
func (d *CustomerReceiptDocument) ToIntakeDocument() receipts.Document {
	doc := receipts.Document{
		Kind:          receipts.KindRepair,
		Title:         "Intake slip",
		Reference:     d.JobCode,
		Number:        d.JobCode,
		IssuedAt:      d.IssuedAt,
		Currency:      d.Currency,
		CustomerName:  d.CustomerName,
		CustomerPhone: d.CustomerPhone,
		Notes:         d.ProblemSummary,
		Branch:        d.BranchName,
	}
	doc.Meta = []receipts.MetaRow{
		{Label: "Job", Value: d.JobCode},
		{Label: "Device", Value: capitalize(d.DeviceLabel)},
	}
	if d.IMEI != "" {
		doc.Meta = append(doc.Meta, receipts.MetaRow{Label: "IMEI / serial", Value: d.IMEI})
	}
	if d.PickupCode != "" {
		payload := PickupQRPayload(d.PickupCode)
		// Match HTML intake: QR on the slip; PK- code stays on SMS.
		doc.Callout = &receipts.Callout{
			Label:     "Scan when collecting",
			Note:      "Your pickup code was sent by SMS. Final charges appear on your repair receipt.",
			QRPayload: payload,
		}
	}
	return doc
}

func capitalize(text string) string {
	if text == "" {
		return ""
	}
	return strings.ToUpper(text[:1]) + text[1:]
}
