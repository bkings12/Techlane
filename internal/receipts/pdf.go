package receipts

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"strings"

	"github.com/jung-kurt/gofpdf"
)

// PDF geometry, in mm.
const (
	pdfMarginX  = 15.0
	pdfMarginY  = 14.0
	pdfPageW    = 210.0
	pdfContentW = pdfPageW - pdfMarginX*2
)

var (
	rgbNavy  = [3]int{4, 2, 87}
	rgbGold  = [3]int{242, 190, 42}
	rgbInk   = [3]int{2, 1, 46}
	rgbMuted = [3]int{91, 91, 112}
	rgbHair  = [3]int{215, 215, 226}
	rgbZebra = [3]int{250, 250, 255}
)

// RenderPDF draws the A4 version of a document, matching the HTML design.
func RenderPDF(shop Shop, doc Document, set Settings) ([]byte, error) {
	pdf := gofpdf.New("P", "mm", "A4", "")
	pdf.SetMargins(pdfMarginX, pdfMarginY, pdfMarginX)
	pdf.SetAutoPageBreak(true, 18)
	pdf.AddPage()

	drawPDFHeader(pdf, shop, doc, set)
	drawPDFParties(pdf, doc)
	drawPDFLines(pdf, doc, set)
	drawPDFCallout(pdf, doc)
	drawPDFTotals(pdf, doc, set)
	drawPDFPayments(pdf, doc)
	drawPDFFooter(pdf, doc, set)

	var buf bytes.Buffer
	if err := pdf.Output(&buf); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func drawPDFHeader(pdf *gofpdf.Fpdf, shop Shop, doc Document, set Settings) {
	top := pdf.GetY()
	textX := pdfMarginX

	if logo := decodeLogoForPDF(shop.LogoDataURI); logo != nil {
		opt := gofpdf.ImageOptions{ImageType: logo.imageType, ReadDpi: true}
		pdf.RegisterImageOptionsReader(logo.name, opt, bytes.NewReader(logo.data))
		if info := pdf.GetImageInfo(logo.name); info != nil {
			pdf.ImageOptions(logo.name, pdfMarginX, top, 0, 16, false, opt, 0, "")
			textX = pdfMarginX + 22
		}
	}

	pdf.SetXY(textX, top)
	setColor(pdf, rgbNavy)
	pdf.SetFont("Arial", "B", 19)
	pdf.CellFormat(pdfContentW-70, 8, tr(shop.Name), "", 2, "L", false, 0, "")

	pdf.SetFont("Arial", "", 9)
	setColor(pdf, rgbMuted)
	for _, line := range headerSubLines(shop, set) {
		pdf.SetX(textX)
		pdf.CellFormat(pdfContentW-70, 4.4, tr(line), "", 2, "L", false, 0, "")
	}
	leftBottom := pdf.GetY()

	// Document block, right aligned.
	pdf.SetXY(pdfMarginX+pdfContentW-70, top)
	setColor(pdf, rgbGold)
	pdf.SetFont("Arial", "B", 10)
	pdf.CellFormat(70, 5, tr(strings.ToUpper(doc.Title)), "", 2, "R", false, 0, "")
	setColor(pdf, rgbNavy)
	pdf.SetFont("Arial", "B", 15)
	if doc.Number != "" {
		pdf.SetX(pdfMarginX + pdfContentW - 70)
		pdf.CellFormat(70, 7, tr(doc.Number), "", 2, "R", false, 0, "")
	}
	setColor(pdf, rgbMuted)
	pdf.SetFont("Arial", "", 9)
	pdf.SetX(pdfMarginX + pdfContentW - 70)
	pdf.CellFormat(70, 4.6, tr(doc.IssuedAt.Format("2 January 2006, 15:04")), "", 2, "R", false, 0, "")
	if doc.StatusNote != "" {
		pdf.SetX(pdfMarginX + pdfContentW - 70)
		pdf.CellFormat(70, 4.6, tr(doc.StatusNote), "", 2, "R", false, 0, "")
	}
	rightBottom := pdf.GetY()

	y := leftBottom
	if rightBottom > y {
		y = rightBottom
	}
	y += 3
	fillRect(pdf, pdfMarginX, y, pdfContentW, 1.2, rgbGold)
	pdf.SetY(y + 6)
}

func headerSubLines(shop Shop, set Settings) []string {
	lines := make([]string, 0, 6)
	if set.HeaderNote != "" {
		lines = append(lines, set.HeaderNote)
	}
	lines = append(lines, shop.AddressLines...)
	if contact := joinNonEmpty(" · ", shop.Phone, shop.Email, shop.Website); contact != "" {
		lines = append(lines, contact)
	}
	if shop.TIN != "" {
		lines = append(lines, "PIN / TIN: "+shop.TIN)
	}
	return lines
}

func drawPDFParties(pdf *gofpdf.Fpdf, doc Document) {
	top := pdf.GetY()
	colW := pdfContentW / 2

	label := func(x float64, text string) {
		pdf.SetXY(x, pdf.GetY())
		setColor(pdf, rgbMuted)
		pdf.SetFont("Arial", "B", 7.5)
		pdf.CellFormat(colW-4, 4, tr(strings.ToUpper(text)), "", 2, "L", false, 0, "")
	}
	kv := func(x float64, k, v string) {
		if strings.TrimSpace(v) == "" {
			return
		}
		pdf.SetX(x)
		setColor(pdf, rgbMuted)
		pdf.SetFont("Arial", "", 9)
		pdf.CellFormat(24, 4.8, tr(k), "", 0, "L", false, 0, "")
		setColor(pdf, rgbInk)
		pdf.CellFormat(colW-28, 4.8, tr(v), "", 2, "L", false, 0, "")
		pdf.SetX(x)
	}

	name := doc.CustomerName
	if name == "" {
		name = "Walk-in customer"
	}
	label(pdfMarginX, "Billed to")
	pdf.SetX(pdfMarginX)
	setColor(pdf, rgbInk)
	pdf.SetFont("Arial", "B", 11)
	pdf.CellFormat(colW-4, 5.6, tr(name), "", 2, "L", false, 0, "")
	kv(pdfMarginX, "Phone", doc.CustomerPhone)
	kv(pdfMarginX, "Branch", doc.Branch)
	leftBottom := pdf.GetY()

	pdf.SetXY(pdfMarginX+colW, top)
	if len(doc.Meta) > 0 {
		label(pdfMarginX+colW, "Details")
		for _, row := range doc.Meta {
			kv(pdfMarginX+colW, row.Label, row.Value)
		}
	}
	rightBottom := pdf.GetY()

	y := leftBottom
	if rightBottom > y {
		y = rightBottom
	}
	pdf.SetY(y + 6)
}

func drawPDFLines(pdf *gofpdf.Fpdf, doc Document, set Settings) {
	if len(doc.Lines) == 0 {
		return
	}
	showQty := documentHasQty(doc)
	showVAT := set.ShowVATBreakdown && doc.hasVAT()

	numCols := 1
	if showQty {
		numCols += 2
	}
	if showVAT {
		numCols += 2
	}
	numW := 26.0
	descW := pdfContentW - float64(numCols)*numW

	setColor(pdf, rgbMuted)
	pdf.SetFont("Arial", "B", 7.5)
	y := pdf.GetY()
	pdf.SetXY(pdfMarginX, y)
	pdf.CellFormat(descW, 6, tr("DESCRIPTION"), "", 0, "L", false, 0, "")
	if showQty {
		pdf.CellFormat(numW, 6, tr("QTY"), "", 0, "R", false, 0, "")
		pdf.CellFormat(numW, 6, tr("UNIT PRICE"), "", 0, "R", false, 0, "")
	}
	if showVAT {
		pdf.CellFormat(numW, 6, tr("NET"), "", 0, "R", false, 0, "")
		pdf.CellFormat(numW, 6, tr("VAT"), "", 0, "R", false, 0, "")
	}
	pdf.CellFormat(numW, 6, tr("AMOUNT"), "", 1, "R", false, 0, "")
	fillRect(pdf, pdfMarginX, pdf.GetY(), pdfContentW, 0.5, rgbNavy)
	pdf.SetY(pdf.GetY() + 1.5)

	for i, line := range doc.Lines {
		rowH := 7.0
		if line.Detail != "" {
			rowH = 11.0
		}
		rowY := pdf.GetY()
		if i%2 == 1 {
			fillRect(pdf, pdfMarginX, rowY-0.5, pdfContentW, rowH, rgbZebra)
		}
		pdf.SetXY(pdfMarginX, rowY)
		setColor(pdf, rgbInk)
		pdf.SetFont("Arial", "", 9.5)
		pdf.CellFormat(descW, 5, tr(line.Description), "", 0, "L", false, 0, "")
		if showQty {
			pdf.CellFormat(numW, 5, tr(formatQty(line.Qty)), "", 0, "R", false, 0, "")
			pdf.CellFormat(numW, 5, tr(formatMoney(line.UnitPrice)), "", 0, "R", false, 0, "")
		}
		if showVAT {
			net, vat := SplitVAT(line.Amount, doc.VATRateBPS, doc.VATInclusive)
			pdf.CellFormat(numW, 5, tr(formatMoney(net)), "", 0, "R", false, 0, "")
			pdf.CellFormat(numW, 5, tr(formatMoney(vat)), "", 0, "R", false, 0, "")
		}
		pdf.CellFormat(numW, 5, tr(formatMoney(line.Amount)), "", 1, "R", false, 0, "")
		if line.Detail != "" {
			pdf.SetX(pdfMarginX)
			setColor(pdf, rgbMuted)
			pdf.SetFont("Arial", "", 8)
			pdf.CellFormat(descW, 4, tr(line.Detail), "", 1, "L", false, 0, "")
		}
		fillRect(pdf, pdfMarginX, pdf.GetY()+0.6, pdfContentW, 0.2, rgbHair)
		pdf.SetY(pdf.GetY() + 2)
	}
}

func drawPDFCallout(pdf *gofpdf.Fpdf, doc Document) {
	if doc.Callout == nil {
		return
	}
	pdf.SetY(pdf.GetY() + 6)
	top := pdf.GetY()
	height := 22.0
	if doc.Callout.Note != "" {
		height = 28.0
	}
	pdf.SetDrawColor(rgbGold[0], rgbGold[1], rgbGold[2])
	pdf.SetLineWidth(0.6)
	pdf.Rect(pdfMarginX, top, pdfContentW, height, "D")
	pdf.SetLineWidth(0.2)

	pdf.SetY(top + 3)
	pdf.SetX(pdfMarginX)
	setColor(pdf, rgbMuted)
	pdf.SetFont("Arial", "B", 7.5)
	pdf.CellFormat(pdfContentW, 4, tr(strings.ToUpper(doc.Callout.Label)), "", 1, "C", false, 0, "")
	pdf.SetX(pdfMarginX)
	setColor(pdf, rgbNavy)
	pdf.SetFont("Courier", "B", 20)
	pdf.CellFormat(pdfContentW, 9, tr(doc.Callout.Value), "", 1, "C", false, 0, "")
	if doc.Callout.Note != "" {
		pdf.SetX(pdfMarginX)
		setColor(pdf, rgbMuted)
		pdf.SetFont("Arial", "", 7.5)
		pdf.CellFormat(pdfContentW, 4, tr(doc.Callout.Note), "", 1, "C", false, 0, "")
	}
	pdf.SetY(top + height + 2)
}

func drawPDFTotals(pdf *gofpdf.Fpdf, doc Document, set Settings) {
	pdf.SetY(pdf.GetY() + 4)
	boxW := 78.0
	x := pdfMarginX + pdfContentW - boxW

	row := func(label, value string, style string) {
		pdf.SetX(x)
		switch style {
		case "grand":
			fillRect(pdf, x, pdf.GetY(), boxW, 0.5, rgbNavy)
			pdf.SetY(pdf.GetY() + 1.6)
			pdf.SetX(x)
			setColor(pdf, rgbNavy)
			pdf.SetFont("Arial", "B", 13)
			pdf.CellFormat(boxW-34, 8, tr(label), "", 0, "L", false, 0, "")
			pdf.CellFormat(34, 8, tr(value), "", 1, "R", false, 0, "")
			fillRect(pdf, x, pdf.GetY(), boxW, 0.5, rgbNavy)
			pdf.SetY(pdf.GetY() + 1.6)
		case "bold":
			setColor(pdf, rgbInk)
			pdf.SetFont("Arial", "B", 10)
			pdf.CellFormat(boxW-34, 6, tr(label), "", 0, "L", false, 0, "")
			pdf.CellFormat(34, 6, tr(value), "", 1, "R", false, 0, "")
		default:
			setColor(pdf, rgbMuted)
			pdf.SetFont("Arial", "", 9.5)
			pdf.CellFormat(boxW-34, 5.6, tr(label), "", 0, "L", false, 0, "")
			setColor(pdf, rgbInk)
			pdf.CellFormat(34, 5.6, tr(value), "", 1, "R", false, 0, "")
		}
	}

	money := func(v float64) string { return doc.Currency + " " + formatMoney(v) }
	if set.ShowVATBreakdown && doc.hasVAT() {
		row("Subtotal (excl. VAT)", money(doc.Subtotal), "")
		row(doc.vatLabel(), money(doc.VATAmount), "")
	}
	row("Total", money(doc.Total), "grand")
	if doc.showBalance(set) || doc.Paid > 0 {
		row("Paid", money(doc.Paid), "")
	}
	if doc.showBalance(set) {
		row("Balance due", money(doc.Balance), "bold")
	}

	if doc.hasVAT() {
		note := fmt.Sprintf("%s has been added to the net amounts above.", doc.vatLabel())
		if doc.VATInclusive {
			note = fmt.Sprintf("All prices shown are inclusive of %s.", doc.vatLabel())
		}
		pdf.SetY(pdf.GetY() + 4)
		pdf.SetX(pdfMarginX)
		setColor(pdf, rgbMuted)
		pdf.SetFont("Arial", "I", 8.5)
		pdf.MultiCell(pdfContentW, 4.4, tr(note), "", "L", false)
	}
}

func drawPDFPayments(pdf *gofpdf.Fpdf, doc Document) {
	if len(doc.Payments) == 0 {
		return
	}
	pdf.SetY(pdf.GetY() + 5)
	pdf.SetX(pdfMarginX)
	setColor(pdf, rgbMuted)
	pdf.SetFont("Arial", "B", 7.5)
	pdf.CellFormat(pdfContentW, 5, tr("PAYMENTS RECEIVED"), "", 1, "L", false, 0, "")

	methodW, refW, statusW := 40.0, 60.0, 40.0
	amountW := pdfContentW - methodW - refW - statusW
	pdf.SetX(pdfMarginX)
	pdf.SetFont("Arial", "B", 7.5)
	pdf.CellFormat(methodW, 5.5, tr("METHOD"), "", 0, "L", false, 0, "")
	pdf.CellFormat(refW, 5.5, tr("REFERENCE"), "", 0, "L", false, 0, "")
	pdf.CellFormat(statusW, 5.5, tr("STATUS"), "", 0, "L", false, 0, "")
	pdf.CellFormat(amountW, 5.5, tr("AMOUNT"), "", 1, "R", false, 0, "")
	fillRect(pdf, pdfMarginX, pdf.GetY(), pdfContentW, 0.4, rgbNavy)
	pdf.SetY(pdf.GetY() + 1.4)

	for _, p := range doc.Payments {
		ref := paymentRefLabel(p)
		if ref == "" {
			ref = "-"
		}
		pdf.SetX(pdfMarginX)
		setColor(pdf, rgbInk)
		pdf.SetFont("Arial", "", 9.5)
		pdf.CellFormat(methodW, 5.6, tr(prettyMethod(p.Method)), "", 0, "L", false, 0, "")
		pdf.CellFormat(refW, 5.6, tr(ref), "", 0, "L", false, 0, "")
		pdf.CellFormat(statusW, 5.6, tr(prettyStatus(p.Status)), "", 0, "L", false, 0, "")
		pdf.CellFormat(amountW, 5.6, tr(doc.Currency+" "+formatMoney(p.Amount)), "", 1, "R", false, 0, "")
		fillRect(pdf, pdfMarginX, pdf.GetY()+0.3, pdfContentW, 0.2, rgbHair)
		pdf.SetY(pdf.GetY() + 1.4)
	}
}

func drawPDFFooter(pdf *gofpdf.Fpdf, doc Document, set Settings) {
	if doc.Notes != "" {
		pdf.SetY(pdf.GetY() + 5)
		pdf.SetX(pdfMarginX)
		setColor(pdf, rgbInk)
		pdf.SetFont("Arial", "", 9)
		pdf.MultiCell(pdfContentW, 4.6, tr("Notes: "+doc.Notes), "", "L", false)
	}

	lines := make([]string, 0, 3)
	if set.WarrantyText != "" {
		lines = append(lines, set.WarrantyText)
	}
	if set.FooterText != "" {
		lines = append(lines, set.FooterText)
	}
	if set.ThankYouText == "" && len(lines) == 0 && doc.ServedBy == "" {
		return
	}

	pdf.SetY(pdf.GetY() + 8)
	fillRect(pdf, pdfMarginX, pdf.GetY(), pdfContentW, 0.2, rgbHair)
	pdf.SetY(pdf.GetY() + 4)

	if set.ThankYouText != "" {
		pdf.SetX(pdfMarginX)
		setColor(pdf, rgbNavy)
		pdf.SetFont("Arial", "B", 11)
		pdf.CellFormat(pdfContentW, 6, tr(set.ThankYouText), "", 1, "C", false, 0, "")
	}
	setColor(pdf, rgbMuted)
	pdf.SetFont("Arial", "", 8.5)
	for _, line := range lines {
		pdf.SetX(pdfMarginX)
		pdf.MultiCell(pdfContentW, 4.2, tr(line), "", "C", false)
	}
	if doc.ServedBy != "" {
		pdf.SetY(pdf.GetY() + 3)
		pdf.SetX(pdfMarginX)
		pdf.SetFont("Arial", "", 7.5)
		pdf.CellFormat(pdfContentW, 4, tr("SERVED BY "+strings.ToUpper(doc.ServedBy)), "", 1, "C", false, 0, "")
	}
}

// ---------------------------------------------------------------- helpers --

type pdfLogo struct {
	name      string
	imageType string
	data      []byte
}

// decodeLogoForPDF unpacks the data URI. SVG is skipped: gofpdf cannot draw it.
func decodeLogoForPDF(dataURI string) *pdfLogo {
	if dataURI == "" || !strings.HasPrefix(dataURI, "data:") {
		return nil
	}
	comma := strings.Index(dataURI, ",")
	if comma < 0 {
		return nil
	}
	meta := dataURI[5:comma]
	if !strings.Contains(meta, "base64") {
		return nil
	}
	mime := strings.SplitN(meta, ";", 2)[0]
	var imageType string
	switch mime {
	case "image/png":
		imageType = "PNG"
	case "image/jpeg":
		imageType = "JPG"
	case "image/gif":
		imageType = "GIF"
	default:
		return nil
	}
	raw, err := base64.StdEncoding.DecodeString(dataURI[comma+1:])
	if err != nil || len(raw) == 0 {
		return nil
	}
	return &pdfLogo{name: "receipt-logo", imageType: imageType, data: raw}
}

func setColor(pdf *gofpdf.Fpdf, rgb [3]int) {
	pdf.SetTextColor(rgb[0], rgb[1], rgb[2])
}

func fillRect(pdf *gofpdf.Fpdf, x, y, w, h float64, rgb [3]int) {
	pdf.SetFillColor(rgb[0], rgb[1], rgb[2])
	pdf.Rect(x, y, w, h, "F")
}

// tr maps the few non-Latin-1 characters the UI uses onto glyphs the core PDF
// fonts can actually draw.
func tr(text string) string {
	replacer := strings.NewReplacer(
		"·", "-",
		"—", "-",
		"–", "-",
		"×", "x",
		"’", "'",
		"‘", "'",
		"“", `"`,
		"”", `"`,
		"…", "...",
	)
	return replacer.Replace(text)
}
