package receipts

import (
	"fmt"
	"html"
	"net/http"
	"strconv"
	"strings"
)

const (
	colorInk    = "#02012e"
	colorNavy   = "#040257"
	colorGold   = "#f2be2a"
	colorMuted  = "#5b5b70"
	colorHair   = "#d7d7e2"
	fontDisplay = "'Segoe UI Semibold','Helvetica Neue',Helvetica,Arial,sans-serif"
	fontBody    = "'Segoe UI',-apple-system,BlinkMacSystemFont,'Helvetica Neue',Helvetica,Arial,sans-serif"
	fontMono    = "'SFMono-Regular',Menlo,Consolas,'Liberation Mono',monospace"
)

// EmailOff wraps an already-escaped address so Cloudflare Scrape Shield does not
// replace it with "[email protected]". Printers and Android WebViews never run
// Cloudflare's decode script, so obfuscated slips look broken.
func EmailOff(escapedEmail string) string {
	if strings.TrimSpace(escapedEmail) == "" {
		return ""
	}
	return "<!--email_off-->" + escapedEmail + "<!--/email_off-->"
}

// ContactLineHTML builds a phone · email · website line with CF email protection disabled.
func ContactLineHTML(sep string, phone, email, website string) string {
	parts := make([]string, 0, 3)
	if v := strings.TrimSpace(phone); v != "" {
		parts = append(parts, html.EscapeString(v))
	}
	if v := strings.TrimSpace(email); v != "" {
		parts = append(parts, EmailOff(html.EscapeString(v)))
	}
	if v := strings.TrimSpace(website); v != "" {
		parts = append(parts, html.EscapeString(v))
	}
	return strings.Join(parts, sep)
}

// SetPrintableHTMLHeaders marks a response as printable HTML that proxies must
// not transform (disables Cloudflare email obfuscation rewriting).
func SetPrintableHTMLHeaders(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store, no-transform")
}

// RenderHTML produces a self-contained, printable receipt document.
func RenderHTML(shop Shop, doc Document, set Settings, paper string) string {
	doc.applySettings(set)
	paper = NormalizePaper(paper, set.DefaultPaper)
	if paper == PaperA4 {
		return renderA4(shop, doc, set)
	}
	return renderThermal(shop, doc, set, paper)
}

// ---------------------------------------------------------------- thermal --

func renderThermal(shop Shop, doc Document, set Settings, paper string) string {
	widthMM, padMM, baseFont := 80.0, 4.0, 12.0
	if paper == PaperThermal58 {
		widthMM, padMM, baseFont = 58.0, 3.0, 11.0
	}
	contentMM := widthMM - padMM*2

	var b strings.Builder
	b.WriteString(fmt.Sprintf(`<!DOCTYPE html>
<html lang="en"><head><meta charset="utf-8">
<meta name="viewport" content="width=device-width,initial-scale=1">
<title>%s %s</title>
<style>
  @page { size: %gmm auto; margin: 0; }
  * { box-sizing: border-box; }
  html, body { margin: 0; padding: 0; background: #f3f3f7; }
  body { font-family: %s; color: %s; -webkit-font-smoothing: antialiased; }
  .sheet {
    width: %gmm; margin: 12px auto; padding: %gmm;
    background: #fff; color: %s; font-size: %gpx; line-height: 1.42;
    box-shadow: 0 10px 30px rgba(4,2,87,.14);
  }
  .logo { display: block; margin: 0 auto 6px; max-width: %gmm; max-height: 18mm; object-fit: contain; }
  .shop-name {
    text-align: center; font-family: %s; font-weight: 700;
    font-size: %gpx; letter-spacing: .06em; text-transform: uppercase;
    line-height: 1.2; margin: 0 0 3px;
  }
  .center { text-align: center; }
  .muted { color: %s; }
  .tiny { font-size: %gpx; }
  .rule { border: none; border-top: 1px dashed %s; margin: 7px 0; }
  .rule-solid { border: none; border-top: 1.5px solid %s; margin: 7px 0; }
  .doc-title {
    text-align: center; font-family: %s; font-weight: 700; text-transform: uppercase;
    letter-spacing: .14em; font-size: %gpx; margin: 2px 0 1px;
  }
  .row { display: flex; justify-content: space-between; gap: 6px; }
  .row + .row { margin-top: 2px; }
  .row .k { color: %s; flex: 0 0 auto; }
  .row .v { text-align: right; word-break: break-word; }
  .amt { font-family: %s; font-variant-numeric: tabular-nums; white-space: nowrap; }
  .col-head {
    display: flex; justify-content: space-between; font-size: %gpx;
    text-transform: uppercase; letter-spacing: .09em; color: %s; margin-bottom: 3px;
  }
  .line { display: flex; justify-content: space-between; gap: 6px; }
  .line + .line { margin-top: 4px; }
  .line .desc { flex: 1 1 auto; }
  .line .sub { display: block; font-size: %gpx; color: %s; }
  .total-row { font-family: %s; font-weight: 700; font-size: %gpx; }
  .balance-row { font-weight: 700; }
  .note { margin-top: 3px; }
  .callout { text-align: center; border: 1.5px dashed %s; border-radius: 5px; padding: 6px 4px; }
  .callout-qr { display: block; margin: 2px auto 6px; width: 120px; height: 120px; }
  .callout-label { display: block; font-size: %gpx; text-transform: uppercase; letter-spacing: .1em; color: %s; }
  .callout-value { display: block; font-family: %s; font-weight: 700; font-size: %gpx; letter-spacing: .16em; margin-top: 2px; }
  .callout-note { display: block; font-size: %gpx; color: %s; margin-top: 3px; word-break: break-all; }
  .footer { margin-top: 9px; text-align: center; }
  .footer p { margin: 0 0 3px; }
  .stamp {
    margin-top: 9px; text-align: center; font-size: %gpx; letter-spacing: .1em;
    text-transform: uppercase; color: %s;
  }
  .actions { width: %gmm; margin: 0 auto 24px; display: flex; gap: 8px; }
  .actions button {
    flex: 1; padding: 10px 12px; border: none; border-radius: 8px; cursor: pointer;
    background: %s; color: #fff; font: 600 14px %s;
  }
  @media print {
    html, body { background: #fff; }
    .sheet { width: auto; margin: 0; box-shadow: none; padding: 2mm %gmm; }
    .actions { display: none; }
  }
</style></head><body>
<div class="sheet">`,
		html.EscapeString(doc.Title), html.EscapeString(doc.Number),
		widthMM,
		fontBody, colorInk,
		widthMM, padMM, colorInk, baseFont,
		contentMM,
		fontDisplay, baseFont+2,
		colorMuted, baseFont-1.5,
		colorHair, colorInk,
		fontDisplay, baseFont,
		colorMuted,
		fontMono,
		baseFont-2, colorMuted,
		baseFont-2, colorMuted,
		fontDisplay, baseFont+3,
		colorHair, baseFont-2.5, colorMuted, fontMono, baseFont+4, baseFont-2.5, colorMuted,
		baseFont-2.5, colorMuted,
		widthMM,
		colorNavy, fontBody,
		padMM,
	))

	// Letterhead.
	if shop.LogoDataURI != "" {
		b.WriteString(fmt.Sprintf(`<img class="logo" src="%s" alt="">`, shop.LogoDataURI))
	}
	b.WriteString(fmt.Sprintf(`<div class="shop-name">%s</div>`, html.EscapeString(shop.Name)))
	if set.HeaderNote != "" {
		b.WriteString(fmt.Sprintf(`<div class="center tiny muted">%s</div>`, html.EscapeString(set.HeaderNote)))
	}
	for _, line := range shop.AddressLines {
		b.WriteString(fmt.Sprintf(`<div class="center tiny muted">%s</div>`, html.EscapeString(line)))
	}
	if contact := ContactLineHTML(" · ", shop.Phone, shop.Email, ""); contact != "" {
		b.WriteString(fmt.Sprintf(`<div class="center tiny muted">%s</div>`, contact))
	}
	if shop.Website != "" {
		b.WriteString(fmt.Sprintf(`<div class="center tiny muted">%s</div>`, html.EscapeString(shop.Website)))
	}
	if shop.TIN != "" {
		b.WriteString(fmt.Sprintf(`<div class="center tiny muted">PIN / TIN: %s</div>`, html.EscapeString(shop.TIN)))
	}

	b.WriteString(`<hr class="rule-solid">`)
	b.WriteString(fmt.Sprintf(`<div class="doc-title">%s</div>`, html.EscapeString(doc.Title)))
	if doc.Number != "" {
		b.WriteString(fmt.Sprintf(`<div class="center">No. %s</div>`, html.EscapeString(doc.Number)))
	}
	b.WriteString(fmt.Sprintf(`<div class="center tiny muted">%s</div>`, html.EscapeString(doc.IssuedAt.Format("2 Jan 2006, 15:04"))))
	if doc.StatusNote != "" {
		b.WriteString(fmt.Sprintf(`<div class="center tiny muted">%s</div>`, html.EscapeString(doc.StatusNote)))
	}
	b.WriteString(`<hr class="rule">`)

	// Detail block.
	meta := doc.Meta
	if doc.CustomerName != "" {
		meta = append([]MetaRow{{Label: "Customer", Value: doc.CustomerName}}, meta...)
	}
	if doc.CustomerPhone != "" {
		meta = append(meta, MetaRow{Label: "Phone", Value: doc.CustomerPhone})
	}
	if doc.Branch != "" {
		meta = append(meta, MetaRow{Label: "Branch", Value: doc.Branch})
	}
	for _, row := range meta {
		if strings.TrimSpace(row.Value) == "" {
			continue
		}
		b.WriteString(fmt.Sprintf(`<div class="row"><span class="k">%s</span><span class="v">%s</span></div>`,
			html.EscapeString(row.Label), html.EscapeString(row.Value)))
	}

	// Lines.
	if len(doc.Lines) > 0 {
		b.WriteString(`<hr class="rule">`)
		b.WriteString(`<div class="col-head"><span>Item</span><span>Amount</span></div>`)
		for _, line := range doc.Lines {
			sub := lineSubtitle(line, doc.Currency)
			subHTML := ""
			if sub != "" {
				subHTML = fmt.Sprintf(`<span class="sub">%s</span>`, html.EscapeString(sub))
			}
			b.WriteString(fmt.Sprintf(
				`<div class="line"><span class="desc">%s%s</span><span class="amt">%s</span></div>`,
				html.EscapeString(line.Description), subHTML, formatMoney(line.Amount)))
		}
	}

	if doc.Callout != nil {
		b.WriteString(`<hr class="rule">`)
		qr := ""
		if doc.Callout.QRDataURI != "" {
			qr = fmt.Sprintf(
				`<img class="callout-qr" src="%s" width="120" height="120" alt="Pickup QR">`,
				html.EscapeString(doc.Callout.QRDataURI),
			)
		}
		b.WriteString(fmt.Sprintf(
			`<div class="callout">%s<span class="callout-label">%s</span><span class="callout-value">%s</span>%s</div>`,
			qr,
			html.EscapeString(doc.Callout.Label),
			html.EscapeString(doc.Callout.Value),
			calloutNote(doc.Callout.Note)))
	}

	// Totals.
	b.WriteString(`<hr class="rule">`)
	if set.ShowVATBreakdown && doc.hasVAT() {
		b.WriteString(totalRowHTML("Subtotal (excl. VAT)", doc.Currency, doc.Subtotal, ""))
		b.WriteString(totalRowHTML(doc.vatLabel(), doc.Currency, doc.VATAmount, ""))
	}
	b.WriteString(totalRowHTML("Total", doc.Currency, doc.Total, "total-row"))
	if doc.showBalance(set) || doc.Paid > 0 {
		b.WriteString(totalRowHTML("Paid", doc.Currency, doc.Paid, ""))
	}
	if doc.showBalance(set) {
		b.WriteString(totalRowHTML("Balance", doc.Currency, doc.Balance, "balance-row"))
	}
	if doc.hasVAT() {
		note := fmt.Sprintf("%s added", doc.vatLabel())
		if doc.VATInclusive {
			note = fmt.Sprintf("Prices include %s", doc.vatLabel())
		}
		b.WriteString(fmt.Sprintf(`<div class="center tiny muted note">%s</div>`, html.EscapeString(note)))
	}

	// Payments.
	if len(doc.Payments) > 0 {
		b.WriteString(`<hr class="rule">`)
		b.WriteString(`<div class="col-head"><span>Payment</span><span>Amount</span></div>`)
		for _, p := range doc.Payments {
			detail := joinNonEmpty(" · ", prettyStatus(p.Status), paymentRefLabel(p))
			sub := ""
			if detail != "" {
				sub = fmt.Sprintf(`<span class="sub">%s</span>`, html.EscapeString(detail))
			}
			b.WriteString(fmt.Sprintf(
				`<div class="line"><span class="desc">%s%s</span><span class="amt">%s</span></div>`,
				html.EscapeString(prettyMethod(p.Method)), sub, formatMoney(p.Amount)))
		}
	}

	if doc.Notes != "" {
		b.WriteString(`<hr class="rule">`)
		b.WriteString(fmt.Sprintf(`<div class="tiny"><span class="muted">Notes:</span> %s</div>`, html.EscapeString(doc.Notes)))
	}

	// Footer.
	b.WriteString(`<hr class="rule-solid">`)
	b.WriteString(`<div class="footer">`)
	if set.ThankYouText != "" {
		b.WriteString(fmt.Sprintf(`<p>%s</p>`, html.EscapeString(set.ThankYouText)))
	}
	if set.WarrantyText != "" {
		b.WriteString(fmt.Sprintf(`<p class="tiny muted">%s</p>`, html.EscapeString(set.WarrantyText)))
	}
	if set.FooterText != "" {
		b.WriteString(fmt.Sprintf(`<p class="tiny muted">%s</p>`, html.EscapeString(set.FooterText)))
	}
	b.WriteString(`</div>`)
	if doc.ServedBy != "" {
		b.WriteString(fmt.Sprintf(`<div class="stamp">Served by %s</div>`, html.EscapeString(doc.ServedBy)))
	}
	b.WriteString(`</div>`)
	b.WriteString(printActions())
	b.WriteString(`</body></html>`)
	return b.String()
}

// --------------------------------------------------------------------- A4 --

func renderA4(shop Shop, doc Document, set Settings) string {
	var b strings.Builder
	b.WriteString(fmt.Sprintf(`<!DOCTYPE html>
<html lang="en"><head><meta charset="utf-8">
<meta name="viewport" content="width=device-width,initial-scale=1">
<title>%s %s</title>
<style>
  @page { size: A4; margin: 14mm; }
  * { box-sizing: border-box; }
  html, body { margin: 0; padding: 0; background: #eeeef4; }
  body { font-family: %s; color: %s; font-size: 13px; line-height: 1.5; -webkit-font-smoothing: antialiased; }
  .sheet { width: 210mm; min-height: 297mm; margin: 20px auto; background: #fff; box-shadow: 0 14px 40px rgba(4,2,87,.16); }
  .band { background: %s; color: #fff; padding: 20mm 16mm 10mm; display: flex; justify-content: space-between; gap: 16mm; }
  .band .brand { display: flex; gap: 12px; align-items: flex-start; }
  .band .logo { max-width: 30mm; max-height: 20mm; object-fit: contain; background: #fff; padding: 4px; border-radius: 6px; }
  .band h1 { font-family: %s; font-size: 24px; letter-spacing: .02em; margin: 0 0 4px; }
  .band .sub, .band .sub a { color: rgba(255,255,255,.76); font-size: 12px; margin: 0; }
  .band .doc { text-align: right; flex: 0 0 auto; }
  .band .doc .kind { font-family: %s; text-transform: uppercase; letter-spacing: .18em; font-size: 13px; color: %s; margin: 0 0 6px; }
  .band .doc .num { font-family: %s; font-size: 20px; margin: 0; }
  .band .doc .date { color: rgba(255,255,255,.76); font-size: 12px; margin: 4px 0 0; }
  .accent { height: 4px; background: %s; }
  .body { padding: 12mm 16mm 16mm; }
  .grid { display: grid; grid-template-columns: 1fr 1fr; gap: 10mm; margin-bottom: 10mm; }
  .card-label { text-transform: uppercase; letter-spacing: .14em; font-size: 10px; color: %s; margin: 0 0 6px; }
  .party { font-family: %s; font-size: 15px; margin: 0 0 2px; }
  .kv { display: flex; gap: 8px; font-size: 12.5px; }
  .kv + .kv { margin-top: 2px; }
  .kv .k { color: %s; min-width: 86px; }
  table { width: 100%%; border-collapse: collapse; }
  thead th {
    text-align: left; font-size: 10px; text-transform: uppercase; letter-spacing: .13em;
    color: %s; padding: 8px 10px; border-bottom: 1.5px solid %s; font-weight: 600;
  }
  tbody td { padding: 10px; border-bottom: 1px solid %s; vertical-align: top; }
  tbody tr:nth-child(even) td { background: #fafaff; }
  td.num, th.num { text-align: right; font-family: %s; font-variant-numeric: tabular-nums; white-space: nowrap; }
  .line-detail { display: block; color: %s; font-size: 11.5px; margin-top: 2px; }
  .totals { display: flex; justify-content: flex-end; margin-top: 8mm; }
  .totals table { width: 78mm; }
  .totals td, .totals tr:nth-child(even) td { padding: 6px 10px; border: none; background: transparent; }
  .totals td.num { font-family: %s; }
  .totals tr.grand td {
    border-top: 1.5px solid %s; border-bottom: 1.5px solid %s;
    font-family: %s; font-size: 16px; padding-top: 10px; padding-bottom: 10px;
  }
  .totals tr.balance td { font-weight: 700; }
  .callout {
    margin-top: 8mm; text-align: center; border: 2px dashed %s; border-radius: 8px; padding: 12px;
  }
  .callout-qr { display: block; margin: 4px auto 10px; width: 140px; height: 140px; }
  .callout-label { display: block; font-size: 10px; text-transform: uppercase; letter-spacing: .14em; color: %s; }
  .callout-value { display: block; font-family: %s; font-weight: 700; font-size: 26px; letter-spacing: .2em; margin-top: 4px; color: %s; }
  .callout-note { display: block; font-size: 11px; color: %s; margin-top: 6px; word-break: break-all; }
  .notes { margin-top: 8mm; padding: 10px 12px; background: #f6f6fc; border-left: 3px solid %s; font-size: 12.5px; }
  .foot { margin-top: 12mm; padding-top: 6mm; border-top: 1px solid %s; text-align: center; }
  .foot .thanks { font-family: %s; font-size: 14px; margin: 0 0 4px; }
  .foot p { margin: 0 0 4px; color: %s; font-size: 11.5px; }
  .served { margin-top: 6mm; text-align: center; font-size: 10px; letter-spacing: .12em; text-transform: uppercase; color: %s; }
  .actions { width: 210mm; margin: 0 auto 30px; display: flex; gap: 10px; }
  .actions button {
    flex: 1; padding: 12px; border: none; border-radius: 10px; cursor: pointer;
    background: %s; color: #fff; font: 600 14px %s;
  }
  @media print {
    html, body { background: #fff; }
    .sheet { width: auto; min-height: 0; margin: 0; box-shadow: none; }
    .band { padding: 0 0 8mm; background: #fff; color: %s; border-bottom: 2px solid %s; }
    .band .sub, .band .doc .date { color: %s; }
    .band .doc .kind { color: %s; }
    .band .logo { padding: 0; }
    .accent { display: none; }
    .body { padding: 8mm 0 0; }
    .actions { display: none; }
  }
</style></head><body>
<div class="sheet">
  <div class="band">
    <div class="brand">`,
		html.EscapeString(doc.Title), html.EscapeString(doc.Number),
		fontBody, colorInk,
		colorNavy,
		fontDisplay,
		fontDisplay, colorGold,
		fontDisplay,
		colorGold,
		colorMuted,
		fontDisplay,
		colorMuted,
		colorMuted, colorNavy,
		colorHair,
		fontMono,
		colorMuted,
		fontMono,
		colorNavy, colorNavy, fontDisplay,
		colorGold, colorMuted, fontMono, colorNavy, colorMuted,
		colorGold,
		colorHair,
		fontDisplay, colorMuted, colorMuted,
		colorNavy, fontBody,
		colorInk, colorNavy, colorMuted, colorNavy,
	))

	if shop.LogoDataURI != "" {
		b.WriteString(fmt.Sprintf(`<img class="logo" src="%s" alt="">`, shop.LogoDataURI))
	}
	b.WriteString(`<div>`)
	b.WriteString(fmt.Sprintf(`<h1>%s</h1>`, html.EscapeString(shop.Name)))
	if set.HeaderNote != "" {
		b.WriteString(fmt.Sprintf(`<p class="sub">%s</p>`, html.EscapeString(set.HeaderNote)))
	}
	for _, line := range shop.AddressLines {
		b.WriteString(fmt.Sprintf(`<p class="sub">%s</p>`, html.EscapeString(line)))
	}
	if contact := ContactLineHTML(" · ", shop.Phone, shop.Email, shop.Website); contact != "" {
		b.WriteString(fmt.Sprintf(`<p class="sub">%s</p>`, contact))
	}
	if shop.TIN != "" {
		b.WriteString(fmt.Sprintf(`<p class="sub">PIN / TIN: %s</p>`, html.EscapeString(shop.TIN)))
	}
	b.WriteString(`</div></div><div class="doc">`)
	b.WriteString(fmt.Sprintf(`<p class="kind">%s</p>`, html.EscapeString(doc.Title)))
	if doc.Number != "" {
		b.WriteString(fmt.Sprintf(`<p class="num">%s</p>`, html.EscapeString(doc.Number)))
	}
	b.WriteString(fmt.Sprintf(`<p class="date">%s</p>`, html.EscapeString(doc.IssuedAt.Format("2 January 2006, 15:04"))))
	if doc.StatusNote != "" {
		b.WriteString(fmt.Sprintf(`<p class="date">%s</p>`, html.EscapeString(doc.StatusNote)))
	}
	b.WriteString(`</div></div><div class="accent"></div><div class="body">`)

	// Bill-to / details.
	b.WriteString(`<div class="grid"><div>`)
	b.WriteString(`<p class="card-label">Billed to</p>`)
	name := doc.CustomerName
	if name == "" {
		name = "Walk-in customer"
	}
	b.WriteString(fmt.Sprintf(`<p class="party">%s</p>`, html.EscapeString(name)))
	if doc.CustomerPhone != "" {
		b.WriteString(fmt.Sprintf(`<div class="kv"><span class="k">Phone</span><span>%s</span></div>`, html.EscapeString(doc.CustomerPhone)))
	}
	if doc.Branch != "" {
		b.WriteString(fmt.Sprintf(`<div class="kv"><span class="k">Branch</span><span>%s</span></div>`, html.EscapeString(doc.Branch)))
	}
	b.WriteString(`</div><div>`)
	if len(doc.Meta) > 0 {
		b.WriteString(`<p class="card-label">Details</p>`)
		for _, row := range doc.Meta {
			if strings.TrimSpace(row.Value) == "" {
				continue
			}
			b.WriteString(fmt.Sprintf(`<div class="kv"><span class="k">%s</span><span>%s</span></div>`,
				html.EscapeString(row.Label), html.EscapeString(row.Value)))
		}
	}
	b.WriteString(`</div></div>`)

	// Line items.
	showQty := documentHasQty(doc)
	b.WriteString(`<table><thead><tr><th>Description</th>`)
	if showQty {
		b.WriteString(`<th class="num">Qty</th><th class="num">Unit price</th>`)
	}
	if set.ShowVATBreakdown && doc.hasVAT() {
		b.WriteString(`<th class="num">Net</th><th class="num">VAT</th>`)
	}
	b.WriteString(`<th class="num">Amount</th></tr></thead><tbody>`)
	for _, line := range doc.Lines {
		b.WriteString(`<tr><td>` + html.EscapeString(line.Description))
		if line.Detail != "" {
			b.WriteString(fmt.Sprintf(`<span class="line-detail">%s</span>`, html.EscapeString(line.Detail)))
		}
		b.WriteString(`</td>`)
		if showQty {
			b.WriteString(fmt.Sprintf(`<td class="num">%s</td><td class="num">%s</td>`,
				formatQty(line.Qty), formatMoney(line.UnitPrice)))
		}
		if set.ShowVATBreakdown && doc.hasVAT() {
			net, vat := SplitVAT(line.Amount, doc.VATRateBPS, doc.VATInclusive)
			b.WriteString(fmt.Sprintf(`<td class="num">%s</td><td class="num">%s</td>`, formatMoney(net), formatMoney(vat)))
		}
		b.WriteString(fmt.Sprintf(`<td class="num">%s</td></tr>`, formatMoney(line.Amount)))
	}
	b.WriteString(`</tbody></table>`)

	if doc.Callout != nil {
		qr := ""
		if doc.Callout.QRDataURI != "" {
			qr = fmt.Sprintf(
				`<img class="callout-qr" src="%s" width="140" height="140" alt="Pickup QR">`,
				html.EscapeString(doc.Callout.QRDataURI),
			)
		}
		b.WriteString(fmt.Sprintf(
			`<div class="callout">%s<span class="callout-label">%s</span><span class="callout-value">%s</span>%s</div>`,
			qr,
			html.EscapeString(doc.Callout.Label),
			html.EscapeString(doc.Callout.Value),
			calloutNote(doc.Callout.Note)))
	}

	// Totals.
	b.WriteString(`<div class="totals"><table><tbody>`)
	if set.ShowVATBreakdown && doc.hasVAT() {
		b.WriteString(a4TotalRow("Subtotal (excl. VAT)", doc.Currency, doc.Subtotal, ""))
		b.WriteString(a4TotalRow(doc.vatLabel(), doc.Currency, doc.VATAmount, ""))
	}
	b.WriteString(a4TotalRow("Total", doc.Currency, doc.Total, "grand"))
	if doc.showBalance(set) || doc.Paid > 0 {
		b.WriteString(a4TotalRow("Paid", doc.Currency, doc.Paid, ""))
	}
	if doc.showBalance(set) {
		b.WriteString(a4TotalRow("Balance due", doc.Currency, doc.Balance, "balance"))
	}
	b.WriteString(`</tbody></table></div>`)

	// Payments.
	if len(doc.Payments) > 0 {
		b.WriteString(`<div style="margin-top:8mm"><p class="card-label">Payments received</p>`)
		b.WriteString(`<table><thead><tr><th>Method</th><th>Reference</th><th>Status</th><th class="num">Amount</th></tr></thead><tbody>`)
		for _, p := range doc.Payments {
			at := ""
			if p.At != nil {
				at = p.At.Format("2 Jan 2006, 15:04")
			}
			ref := paymentRefLabel(p)
			if ref == "" {
				ref = "—"
			}
			b.WriteString(fmt.Sprintf(`<tr><td>%s%s</td><td>%s</td><td>%s</td><td class="num">%s</td></tr>`,
				html.EscapeString(prettyMethod(p.Method)),
				optionalDetail(at),
				html.EscapeString(ref),
				html.EscapeString(prettyStatus(p.Status)),
				formatMoney(p.Amount)))
		}
		b.WriteString(`</tbody></table></div>`)
	}

	if doc.Notes != "" {
		b.WriteString(fmt.Sprintf(`<div class="notes"><strong>Notes:</strong> %s</div>`, html.EscapeString(doc.Notes)))
	}
	if doc.hasVAT() {
		note := fmt.Sprintf("%s has been added to the net amounts above.", doc.vatLabel())
		if doc.VATInclusive {
			note = fmt.Sprintf("All prices shown are inclusive of %s.", doc.vatLabel())
		}
		b.WriteString(fmt.Sprintf(`<p style="margin-top:6mm;font-size:11.5px;color:%s">%s</p>`, colorMuted, html.EscapeString(note)))
	}

	// Footer.
	b.WriteString(`<div class="foot">`)
	if set.ThankYouText != "" {
		b.WriteString(fmt.Sprintf(`<p class="thanks">%s</p>`, html.EscapeString(set.ThankYouText)))
	}
	if set.WarrantyText != "" {
		b.WriteString(fmt.Sprintf(`<p>%s</p>`, html.EscapeString(set.WarrantyText)))
	}
	if set.FooterText != "" {
		b.WriteString(fmt.Sprintf(`<p>%s</p>`, html.EscapeString(set.FooterText)))
	}
	b.WriteString(`</div>`)
	if doc.ServedBy != "" {
		b.WriteString(fmt.Sprintf(`<div class="served">Served by %s</div>`, html.EscapeString(doc.ServedBy)))
	}
	b.WriteString(`</div></div>`)
	b.WriteString(printActions())
	b.WriteString(`</body></html>`)
	return b.String()
}

// ---------------------------------------------------------------- helpers --

func printActions() string {
	return `<div class="actions"><button type="button" onclick="window.print()">Print</button></div>
<script>window.addEventListener('load',function(){if(location.search.indexOf('autoprint=1')>-1){setTimeout(function(){window.print()},350)}})</script>`
}

func totalRowHTML(label, currency string, amount float64, class string) string {
	cls := "row"
	if class != "" {
		cls += " " + class
	}
	return fmt.Sprintf(`<div class="%s"><span class="k">%s</span><span class="v amt">%s %s</span></div>`,
		cls, html.EscapeString(label), html.EscapeString(currency), formatMoney(amount))
}

func a4TotalRow(label, currency string, amount float64, class string) string {
	cls := ""
	if class != "" {
		cls = fmt.Sprintf(` class="%s"`, class)
	}
	return fmt.Sprintf(`<tr%s><td>%s</td><td class="num">%s %s</td></tr>`,
		cls, html.EscapeString(label), html.EscapeString(currency), formatMoney(amount))
}

func calloutNote(note string) string {
	if note == "" {
		return ""
	}
	return fmt.Sprintf(`<span class="callout-note">%s</span>`, html.EscapeString(note))
}

func optionalDetail(text string) string {
	if text == "" {
		return ""
	}
	return fmt.Sprintf(`<span class="line-detail">%s</span>`, html.EscapeString(text))
}

func lineSubtitle(line Line, currency string) string {
	if line.Qty > 0 && line.UnitPrice > 0 {
		unit := fmt.Sprintf("%s × %s %s", formatQty(line.Qty), currency, formatMoney(line.UnitPrice))
		return joinNonEmpty(" · ", unit, line.Detail)
	}
	return line.Detail
}

func documentHasQty(doc Document) bool {
	for _, line := range doc.Lines {
		if line.Qty > 0 {
			return true
		}
	}
	return false
}

func formatQty(qty float64) string {
	if qty == float64(int64(qty)) {
		return strconv.FormatInt(int64(qty), 10)
	}
	return strconv.FormatFloat(qty, 'f', 2, 64)
}

// formatMoney renders 1234.5 as "1,234.50".
func formatMoney(amount float64) string {
	neg := amount < 0
	if neg {
		amount = -amount
	}
	whole := int64(amount)
	cents := int64((amount-float64(whole))*100 + 0.5)
	if cents >= 100 {
		whole++
		cents -= 100
	}
	digits := strconv.FormatInt(whole, 10)
	var grouped strings.Builder
	for i, r := range digits {
		if i > 0 && (len(digits)-i)%3 == 0 {
			grouped.WriteByte(',')
		}
		grouped.WriteRune(r)
	}
	out := fmt.Sprintf("%s.%02d", grouped.String(), cents)
	if neg {
		return "-" + out
	}
	return out
}

func joinNonEmpty(sep string, parts ...string) string {
	kept := make([]string, 0, len(parts))
	for _, p := range parts {
		if strings.TrimSpace(p) != "" {
			kept = append(kept, strings.TrimSpace(p))
		}
	}
	return strings.Join(kept, sep)
}

func prettyMethod(method string) string {
	switch strings.ToLower(strings.TrimSpace(method)) {
	case "mpesa", "m_pesa", "mpesa_stk", "mpesa_c2b", "m-pesa":
		return "M-Pesa"
	case "cash":
		return "Cash"
	case "card":
		return "Card"
	case "bank", "bank_transfer", "bank_paybill":
		return "Bank"
	case "":
		return "Payment"
	}
	return capitalizeWords(strings.ReplaceAll(method, "_", " "))
}

func paymentRefLabel(p PaymentLine) string {
	ref := CustomerPaymentRef(p.Reference)
	if ref == "" {
		return ""
	}
	switch strings.ToLower(strings.TrimSpace(p.Method)) {
	case "mpesa", "m_pesa", "mpesa_stk", "mpesa_c2b", "m-pesa":
		return ref
	default:
		return ref
	}
}

// prettyStatus maps internal payment statuses onto customer-facing labels.
func prettyStatus(status string) string {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "":
		return ""
	case "provisional", "allocated", "confirmed":
		return "Paid"
	case "initiated", "requested", "pending":
		return "Pending"
	case "failed":
		return "Failed"
	case "refunded", "reversed":
		return "Refunded"
	default:
		return capitalizeWords(strings.ReplaceAll(status, "_", " "))
	}
}

func capitalizeWords(text string) string {
	words := strings.Fields(text)
	for i, w := range words {
		words[i] = strings.ToUpper(w[:1]) + w[1:]
	}
	return strings.Join(words, " ")
}

// SplitVAT divides an amount into its net and VAT parts.
func SplitVAT(total float64, rateBPS int, inclusive bool) (net, vat float64) {
	if rateBPS <= 0 {
		return total, 0
	}
	if inclusive {
		vat = total * float64(rateBPS) / float64(10000+rateBPS)
		return total - vat, vat
	}
	return total, total * float64(rateBPS) / 10000
}
