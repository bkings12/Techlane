package receipts

import (
	"fmt"
	"strings"
	"unicode"
)

// 80mm Font A character columns. Using 32 made item/total rows look
// left-squeezed on POS-80 (header centered; body short of the paper width).
const escposCols = 42

// BuildESCPOS builds a counter ESC/POS slip matching the thermal HTML letterhead
// (shop, slogan, address, phone/email/website, TIN) without CR overstrike.
func BuildESCPOS(shop Shop, doc Document, set Settings) []byte {
	payments := append([]PaymentLine(nil), doc.Payments...)
	doc.applySettings(set)
	// Counter thermal always shows tender lines even if HTML receipts hide them.
	doc.Payments = payments
	if len(doc.Payments) == 0 && doc.Paid > 0 {
		doc.Payments = []PaymentLine{{Method: "paid", Amount: doc.Paid}}
	}
	var b strings.Builder
	w := func(s string) { b.WriteString(s) }
	cmd := func(bytes ...byte) {
		for _, x := range bytes {
			b.WriteByte(x)
		}
	}
	center := func() { cmd(0x1b, 0x61, 0x01) }
	left := func() { cmd(0x1b, 0x61, 0x00) }
	wrapCenter := func(s string) {
		s = escposClean(s)
		if s == "" {
			return
		}
		for _, part := range escposWrap(s, escposCols) {
			w(part + "\n")
		}
	}

	cmd(0x1b, 0x40)       // init
	cmd(0x1b, 0x74, 0x00) // PC437
	cmd(0x1b, 0x4d, 0x00) // Font A (12×24 — denser than Font B)
	// Heat/darkness for POS-80 class clones. Avoid GS ( K — its fn byte 0x31
	// is ASCII '1' and prints as a literal "1" before the shop name on printers
	// that don't implement that Epson command.
	cmd(0x1b, 0x37, 0x09, 0xff, 0x01) // ESC 7: max dots, max heat, min cool-down
	cmd(0x1d, 0x7c, 0x0f)             // GS | print density (clone; non-printable args)
	cmd(0x1b, 0x21, 0x08)             // emphasized
	cmd(0x1b, 0x45, 0x01)             // bold
	cmd(0x1b, 0x47, 0x01)             // double-strike

	// —— Letterhead (centered) ——
	center()
	cmd(0x1d, 0x21, 0x11) // double width + height for shop name
	wrapCenter(shop.Name)
	cmd(0x1d, 0x21, 0x00)
	if note := strings.TrimSpace(set.HeaderNote); note != "" {
		wrapCenter(note)
	}
	for _, line := range shop.AddressLines {
		wrapCenter(line)
	}
	contact := joinNonEmpty(" / ", shop.Phone, shop.Email)
	if contact != "" {
		wrapCenter(contact)
	}
	if shop.Website != "" {
		wrapCenter(shop.Website)
	}
	if shop.TIN != "" {
		wrapCenter("PIN/TIN: " + shop.TIN)
	}
	w(escposRule())

	// Document title + meta (still centered)
	title := strings.TrimSpace(doc.Title)
	if title == "" {
		title = "Sale receipt"
	}
	wrapCenter(strings.ToUpper(title))
	ref := strings.TrimSpace(doc.Number)
	if ref == "" {
		ref = strings.TrimSpace(doc.Reference)
	}
	if ref != "" {
		wrapCenter("No. " + ref)
	}
	if !doc.IssuedAt.IsZero() {
		wrapCenter(doc.IssuedAt.Local().Format("02 Jan 2006 15:04"))
	}
	if doc.StatusNote != "" {
		wrapCenter(doc.StatusNote)
	}
	w(escposRule())

	// —— Customer / branch (full-width rows) ——
	left()
	if doc.CustomerName != "" {
		w(escposKV("Customer", trimPad(doc.CustomerName, escposCols-10)))
	}
	if doc.CustomerPhone != "" {
		w(escposKV("Phone", doc.CustomerPhone))
	}
	if doc.Branch != "" {
		w(escposKV("Branch", trimPad(doc.Branch, escposCols-8)))
	}
	for _, row := range doc.Meta {
		if strings.TrimSpace(row.Value) == "" {
			continue
		}
		w(escposKV(trimPad(row.Label, 12), trimPad(row.Value, escposCols-14)))
	}
	if doc.CustomerName != "" || doc.CustomerPhone != "" || doc.Branch != "" || len(doc.Meta) > 0 {
		w(escposRule())
	}

	// —— Lines ——
	hasMoney := false
	for _, line := range doc.Lines {
		desc := strings.TrimSpace(line.Description)
		if desc == "" {
			continue
		}
		hasMoney = true
		amt := money(doc.Currency, line.Amount)
		if line.Qty > 1 {
			for _, part := range escposWrap(desc, escposCols) {
				w(part + "\n")
			}
			w(escposKV(fmt.Sprintf("  %g x %s", line.Qty, money(doc.Currency, line.UnitPrice)), amt))
		} else {
			w(escposKV(trimPad(desc, escposCols-len(amt)-1), amt))
		}
		if d := strings.TrimSpace(line.Detail); d != "" {
			for _, part := range escposWrap(d, escposCols) {
				w("  " + trimPad(part, escposCols-2) + "\n")
			}
		}
	}
	if hasMoney || doc.Total > 0.009 || doc.Paid > 0.009 {
		w(escposRule())

		// —— Totals ——
		if set.ShowVATBreakdown && doc.hasVAT() {
			w(escposKV("Subtotal", money(doc.Currency, doc.Subtotal)))
			w(escposKV(doc.vatLabel(), money(doc.Currency, doc.VATAmount)))
		}
		cmd(0x1d, 0x21, 0x10) // double height total
		w(escposKV("TOTAL", money(doc.Currency, doc.Total)))
		cmd(0x1d, 0x21, 0x00)
		if doc.showBalance(set) || doc.Paid > 0 {
			w(escposKV("Paid", money(doc.Currency, doc.Paid)))
		}
		if doc.showBalance(set) && doc.Balance > 0.009 {
			w(escposKV("Balance", money(doc.Currency, doc.Balance)))
		}

		for _, p := range doc.Payments {
			label := prettyPayMethod(p.Method)
			if ref := strings.TrimSpace(p.Reference); ref != "" {
				if len(ref) > 14 {
					ref = ref[:14]
				}
				label = label + " " + ref
			}
			w(escposKV(trimPad(label, escposCols-12), money(doc.Currency, p.Amount)))
		}
	}

	if notes := strings.TrimSpace(doc.Notes); notes != "" {
		w(escposRule())
		for _, part := range escposWrap("Notes: "+notes, escposCols) {
			w(part + "\n")
		}
	}

	if doc.Callout != nil {
		payload := strings.TrimSpace(doc.Callout.QRPayload)
		hasValue := strings.TrimSpace(doc.Callout.Value) != ""
		if payload != "" || hasValue {
			w(escposRule())
			center()
			if label := strings.TrimSpace(doc.Callout.Label); label != "" {
				wrapCenter(label)
			}
			if payload != "" {
				if qr := BuildESCPOSQR(payload); len(qr) > 0 {
					_, _ = b.Write(qr)
				}
			}
			if hasValue {
				cmd(0x1d, 0x21, 0x11)
				wrapCenter(doc.Callout.Value)
				cmd(0x1d, 0x21, 0x00)
			}
			if note := strings.TrimSpace(doc.Callout.Note); note != "" {
				wrapCenter(note)
			}
			left()
		}
	}

	// —— Footer (centered) ——
	center()
	w(escposRule())
	thanks := strings.TrimSpace(set.ThankYouText)
	if thanks == "" {
		thanks = "Thank you"
	}
	wrapCenter(thanks)
	if warr := strings.TrimSpace(set.WarrantyText); warr != "" {
		wrapCenter(warr)
	}
	if foot := strings.TrimSpace(set.FooterText); foot != "" {
		wrapCenter(foot)
	}
	if doc.ServedBy != "" && set.ShowServedBy {
		wrapCenter("Served by " + doc.ServedBy)
	}
	w("\n")
	cmd(0x1b, 0x47, 0x00)
	cmd(0x1b, 0x45, 0x00)
	cmd(0x1b, 0x21, 0x00)
	left()
	cmd(0x1d, 0x56, 0x01) // cut

	return []byte(b.String())
}

func prettyPayMethod(method string) string {
	switch strings.ToLower(strings.TrimSpace(method)) {
	case "mpesa_stk", "mpesa", "m-pesa":
		return "M-Pesa"
	case "mpesa_c2b", "c2b":
		return "Paybill"
	case "cash":
		return "Cash"
	case "card":
		return "Card"
	case "bank":
		return "Bank"
	default:
		m := strings.TrimSpace(method)
		if m == "" {
			return "Paid"
		}
		return m
	}
}

func money(currency string, amount float64) string {
	cur := strings.TrimSpace(currency)
	if cur == "" {
		cur = "KES"
	}
	return fmt.Sprintf("%s %.2f", cur, amount)
}

func escposRule() string {
	// Solid rule reads darker/cleaner than sparse dashes on thermal paper.
	return strings.Repeat("=", escposCols) + "\n"
}

func escposLine(s string) string {
	s = escposClean(s)
	if s == "" {
		return "\n"
	}
	return s + "\n"
}

func escposKV(label, value string) string {
	label = escposClean(label)
	value = escposClean(value)
	width := escposCols
	if len(label)+1+len(value) <= width {
		pad := width - len(label) - len(value)
		if pad < 1 {
			pad = 1
		}
		return label + strings.Repeat(" ", pad) + value + "\n"
	}
	return label + "\n" + strings.Repeat(" ", max(0, width-len(value))) + value + "\n"
}

func escposWrap(s string, width int) []string {
	s = escposClean(s)
	if s == "" {
		return nil
	}
	if width < 8 {
		width = 8
	}
	words := strings.Fields(s)
	var lines []string
	var cur string
	for _, word := range words {
		if cur == "" {
			cur = word
			continue
		}
		if len(cur)+1+len(word) <= width {
			cur += " " + word
			continue
		}
		lines = append(lines, cur)
		cur = word
	}
	if cur != "" {
		lines = append(lines, cur)
	}
	// Hard-split any leftover overlong token.
	var out []string
	for _, line := range lines {
		for len(line) > width {
			out = append(out, line[:width])
			line = line[width:]
		}
		if line != "" {
			out = append(out, line)
		}
	}
	return out
}

func trimPad(s string, maxLen int) string {
	s = escposClean(s)
	if maxLen < 4 {
		return s
	}
	if len(s) > maxLen {
		return s[:maxLen-1] + "."
	}
	return s
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func escposClean(s string) string {
	s = strings.ReplaceAll(s, "\r", "")
	s = strings.ReplaceAll(s, "\n", " ")
	s = strings.Join(strings.Fields(s), " ")
	var b strings.Builder
	for _, r := range s {
		switch {
		case r == '–' || r == '—' || r == '−':
			b.WriteByte('-')
		case r == '‘' || r == '’' || r == '‚':
			b.WriteByte('\'')
		case r == '“' || r == '”':
			b.WriteByte('"')
		case r == '…':
			b.WriteString("...")
		case r == '·':
			b.WriteByte('.')
		case r <= unicode.MaxASCII && (unicode.IsPrint(r) || r == ' '):
			b.WriteRune(r)
		}
	}
	return b.String()
}
