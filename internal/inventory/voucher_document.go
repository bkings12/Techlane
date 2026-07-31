package inventory

import (
	"strconv"
	"strings"

	"github.com/techlane/techlane/internal/receipts"
)

// ToReceiptDocument maps a supplier credit voucher onto the shared printable
// model so it carries the same branding as customer receipts.
func (v *SupplierCreditVoucher) ToReceiptDocument() receipts.Document {
	doc := receipts.Document{
		Kind:            receipts.KindVoucher,
		Title:           "Supplier credit voucher",
		Reference:       v.AuthCode,
		IssuedAt:        v.IssuedAt,
		Currency:        v.Currency,
		CustomerName:    v.SupplierName,
		VATRateBPS:      v.VATRateBPS,
		VATInclusive:    v.VATInclusive,
		Subtotal:        v.NetAmount,
		VATAmount:       v.VATAmount,
		Total:           v.UnitCost,
		SuppressBalance: true,
		Callout: &receipts.Callout{
			Label: "Auth code for shop pickup",
			Value: v.AuthCode,
			Note:  v.QRPayload,
		},
	}
	doc.Meta = []receipts.MetaRow{
		{Label: "Job", Value: v.JobCode},
		{Label: "Status", Value: prettyLabel(v.IssueStatus)},
		{Label: "Reconciliation", Value: prettyLabel(v.Reconciliation)},
	}
	doc.Lines = []receipts.Line{{
		Description: v.Description,
		Detail:      "Qty " + strconv.Itoa(v.Quantity),
		Qty:         float64(v.Quantity),
		UnitPrice:   unitPrice(v.UnitCost, v.Quantity),
		Amount:      v.UnitCost,
	}}
	return doc
}

func unitPrice(total float64, qty int) float64 {
	if qty <= 0 {
		return total
	}
	return total / float64(qty)
}

func prettyLabel(text string) string {
	text = strings.ReplaceAll(strings.TrimSpace(text), "_", " ")
	if text == "" {
		return ""
	}
	return strings.ToUpper(text[:1]) + text[1:]
}
