package repair

import (
	"bytes"
	"fmt"
	"strings"

	"github.com/jung-kurt/gofpdf"
)

func renderReceiptPDF(title string, d *CustomerReceiptDocument, taxInvoice bool) ([]byte, error) {
	pdf := gofpdf.New("P", "mm", "A4", "")
	pdf.SetMargins(15, 15, 15)
	pdf.AddPage()
	pdf.SetFont("Arial", "B", 16)
	pdf.Cell(0, 8, title)
	pdf.Ln(10)
	pdf.SetFont("Arial", "", 11)
	writeLine := func(label, value string) {
		if value == "" {
			return
		}
		pdf.SetFont("Arial", "B", 10)
		pdf.Cell(40, 6, label)
		pdf.SetFont("Arial", "", 10)
		pdf.Cell(0, 6, value)
		pdf.Ln(6)
	}
	writeLine("Shop:", d.ShopName)
	if d.ShopTIN != "" {
		writeLine("TIN:", d.ShopTIN)
	}
	if addr := strings.TrimSpace(strings.Join([]string{d.ShopAddress1, d.ShopAddress2, d.ShopCity}, ", ")); addr != "" {
		writeLine("Address:", addr)
	}
	writeLine("Job:", d.JobCode)
	writeLine("Customer:", d.CustomerName)
	writeLine("Device:", d.DeviceLabel)
	writeLine("Status:", strings.ReplaceAll(d.Status, "_", " "))
	pdf.Ln(4)
	pdf.SetFont("Arial", "B", 10)
	pdf.Cell(80, 6, "Item")
	pdf.Cell(40, 6, "Net")
	pdf.Cell(40, 6, "VAT")
	pdf.Cell(30, 6, "Total")
	pdf.Ln(7)
	pdf.SetFont("Arial", "", 10)
	addRow := func(name string, total float64) {
		net, vat := splitLineVAT(total, d.VATRateBPS, d.VATInclusive)
		pdf.Cell(80, 6, name)
		pdf.Cell(40, 6, fmt.Sprintf("%s %.2f", d.Currency, net))
		pdf.Cell(40, 6, fmt.Sprintf("%s %.2f", d.Currency, vat))
		pdf.Cell(30, 6, fmt.Sprintf("%s %.2f", d.Currency, total))
		pdf.Ln(6)
	}
	if len(d.LabourLines) > 0 || len(d.PartLines) > 0 || len(d.ProductLines) > 0 {
		for _, li := range append(append([]JobLineItem{}, d.LabourLines...), d.PartLines...) {
			addRow(li.Description, li.LineTotal)
		}
		for _, li := range d.ProductLines {
			addRow(li.Description, li.LineTotal)
		}
	} else {
		addRow("Labor", d.LaborAmount)
		addRow("Parts", d.PartsAmount)
		if d.SaleLinesTotal > 0 {
			addRow("Accessories / extras", d.SaleLinesTotal)
		}
	}
	pdf.Ln(4)
	writeLine("Subtotal (ex VAT):", fmt.Sprintf("%s %.2f", d.Currency, d.NetSubtotal))
	writeLine("VAT:", fmt.Sprintf("%s %.2f", d.Currency, d.VATAmount))
	writeLine("Total due:", fmt.Sprintf("%s %.2f", d.Currency, d.TotalDue))
	writeLine("Paid:", fmt.Sprintf("%s %.2f", d.Currency, d.Paid))
	writeLine("Balance:", fmt.Sprintf("%s %.2f", d.Currency, d.Balance))
	if taxInvoice {
		pdf.Ln(8)
		pdf.SetFont("Arial", "I", 9)
		pdf.MultiCell(0, 5, "Tax invoice — prices are VAT inclusive where marked.", "", "L", false)
	}
	var buf bytes.Buffer
	if err := pdf.Output(&buf); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func splitLineVAT(amount float64, rateBPS int, inclusive bool) (net, vat float64) {
	return splitVAT(amount, rateBPS, inclusive)
}

func (d *CustomerReceiptDocument) ReceiptPDF() ([]byte, error) {
	return renderReceiptPDF("Repair receipt", d, false)
}

func (d *CustomerReceiptDocument) TaxInvoicePDF() ([]byte, error) {
	return renderReceiptPDF("Tax invoice", d, true)
}
