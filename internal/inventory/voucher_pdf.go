package inventory

import (
	"bytes"
	"fmt"

	"github.com/jung-kurt/gofpdf"
)

func (v *SupplierCreditVoucher) VoucherPDF() ([]byte, error) {
	pdf := gofpdf.New("P", "mm", "A4", "")
	pdf.SetMargins(15, 15, 15)
	pdf.AddPage()
	pdf.SetFont("Arial", "B", 16)
	pdf.Cell(0, 8, "Supplier credit voucher")
	pdf.Ln(10)
	pdf.SetFont("Arial", "", 11)
	lines := []struct{ label, value string }{
		{"Shop", v.ShopName},
		{"Supplier", v.SupplierName},
		{"Job", v.JobCode},
		{"Part", v.Description},
		{"Quantity", fmt.Sprintf("%d", v.Quantity)},
		{"Credit", fmt.Sprintf("%s %.2f", v.Currency, v.UnitCost)},
		{"Net (ex VAT)", fmt.Sprintf("%s %.2f", v.Currency, v.NetAmount)},
		{"VAT", fmt.Sprintf("%s %.2f", v.Currency, v.VATAmount)},
		{"Status", fmt.Sprintf("%s · recon %s", v.IssueStatus, v.Reconciliation)},
		{"Auth code", v.AuthCode},
	}
	for _, line := range lines {
		if line.value == "" {
			continue
		}
		pdf.SetFont("Arial", "B", 10)
		pdf.Cell(45, 6, line.label)
		pdf.SetFont("Arial", "", 10)
		pdf.Cell(0, 6, line.value)
		pdf.Ln(6)
	}
	if v.VATInclusive {
		pdf.Ln(4)
		pdf.SetFont("Arial", "I", 9)
		pdf.Cell(0, 5, fmt.Sprintf("Unit cost is VAT inclusive (%.2f%%).", float64(v.VATRateBPS)/100))
	}
	var buf bytes.Buffer
	if err := pdf.Output(&buf); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
