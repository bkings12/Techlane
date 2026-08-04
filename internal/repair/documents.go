package repair

import (
	"context"
	"errors"
	"fmt"
	"html"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/techlane/techlane/internal/receipts"
)

// CustomerReceiptDocument is everything needed to print a customer receipt.
type CustomerReceiptDocument struct {
	RepairID       uuid.UUID       `json:"repair_id"`
	BranchName     string          `json:"branch_name,omitempty"`
	TechnicianName string          `json:"technician_name,omitempty"`
	ShopName       string          `json:"shop_name"`
	ShopSlogan     string          `json:"shop_slogan,omitempty"`
	ShopPhone      string          `json:"shop_phone,omitempty"`
	ShopEmail      string          `json:"shop_email,omitempty"`
	ShopWebsite    string          `json:"shop_website,omitempty"`
	ShopTIN        string          `json:"shop_tin,omitempty"`
	ShopAddress1   string          `json:"shop_address_line1,omitempty"`
	ShopAddress2   string          `json:"shop_address_line2,omitempty"`
	ShopCity       string          `json:"shop_city,omitempty"`
	JobCode        string          `json:"job_code"`
	PickupCode     string          `json:"pickup_code,omitempty"`
	Status         string          `json:"status"`
	ProblemSummary string          `json:"problem_summary"`
	CustomerName   string          `json:"customer_name"`
	CustomerPhone  string          `json:"customer_phone,omitempty"`
	DeviceLabel    string          `json:"device_label"`
	IMEI           string          `json:"imei,omitempty"`
	// Intake-slip specifics: what the customer was told and what we took in
	// alongside the device. Absent on the final receipt, where they no longer
	// matter — the job is being handed back at that point.
	PromisedBy     *time.Time      `json:"promised_by,omitempty"`
	Accessories    []string        `json:"accessories,omitempty"`
	IntakeCondition string         `json:"intake_condition,omitempty"`
	LaborAmount    float64         `json:"labor_amount"`
	PartsAmount    float64         `json:"parts_amount"`
	SaleLinesTotal float64         `json:"sale_lines_total"`
	NetSubtotal    float64         `json:"net_subtotal"`
	VATAmount      float64         `json:"vat_amount"`
	VATRateBPS     int             `json:"vat_rate_bps"`
	VATInclusive   bool            `json:"vat_inclusive"`
	TotalDue       float64         `json:"total_due"`
	Paid           float64         `json:"paid"`
	Balance        float64         `json:"balance"`
	Currency       string          `json:"currency"`
	Payments       []PublicReceipt `json:"payments"`
	IssuedAt       time.Time       `json:"issued_at"`
}

func (s *Service) BuildCustomerReceipt(ctx context.Context, tenantID, repairID uuid.UUID) (*CustomerReceiptDocument, error) {
	job, err := s.GetRepair(ctx, tenantID, repairID)
	if err != nil {
		return nil, err
	}
	shopName := "TechLane"
	tax := s.loadShopTaxProfile(ctx, tenantID)
	if tax.LegalName != "" {
		shopName = tax.LegalName
	} else {
		var name string
		if err := s.pool.QueryRow(ctx, `SELECT name FROM identity.tenants WHERE id = $1`, tenantID).Scan(&name); err == nil && strings.TrimSpace(name) != "" {
			shopName = name
		}
	}

	customerName := "Walk-in customer"
	customerPhone := ""
	if job.Customer != nil {
		if job.Customer.FullName != "" {
			customerName = job.Customer.FullName
		}
		if job.Customer.Phone != nil {
			customerPhone = *job.Customer.Phone
		}
	} else if job.CustomerName != nil && *job.CustomerName != "" {
		customerName = *job.CustomerName
	}

	deviceLabel := "Device"
	imei := ""
	if job.Device != nil {
		parts := []string{}
		if job.Device.Kind != "" {
			parts = append(parts, job.Device.Kind)
		}
		if job.Device.Brand != nil && *job.Device.Brand != "" {
			parts = append(parts, *job.Device.Brand)
		}
		if job.Device.Model != nil && *job.Device.Model != "" {
			parts = append(parts, *job.Device.Model)
		}
		if len(parts) > 0 {
			deviceLabel = strings.Join(parts, " ")
		}
		if job.Device.IMEI != nil {
			imei = *job.Device.IMEI
		}
	}

	labor := job.LaborAmount
	partsAmt := 0.0
	var estLabor, estParts float64
	estErr := s.pool.QueryRow(ctx, `
		SELECT labor_amount::float8, parts_amount::float8
		FROM repair.repair_estimates
		WHERE tenant_id = $1 AND repair_job_id = $2 AND status = 'approved'
		ORDER BY decided_at DESC NULLS LAST, created_at DESC LIMIT 1`, tenantID, repairID).
		Scan(&estLabor, &estParts)
	if estErr == nil {
		labor = estLabor
		partsAmt = estParts
	} else if !errors.Is(estErr, pgx.ErrNoRows) {
		return nil, estErr
	}

	saleExtra, err := s.saleLinesTotal(ctx, tenantID, repairID)
	if err != nil {
		return nil, err
	}

	// Same paid recognition as Money / handover (includes counter cash still in till).
	_, balanceDue, total, _, _, err := s.repairPaymentAmounts(ctx, tenantID, repairID)
	if err != nil {
		return nil, err
	}
	paid := total - balanceDue
	if paid < 0 {
		paid = 0
	}

	receipts, err := s.PublicRepairReceipts(ctx, tenantID, repairID)
	if err != nil {
		return nil, err
	}
	netSubtotal, vatAmount := splitVAT(total, tax.VATRateBPS, tax.VATInclusive)
	balance := balanceDue
	if balance < 0 {
		balance = 0
	}
	jobCode := job.JobCode
	if jobCode == "" {
		jobCode = job.ID.String()[:8]
	}
	pickupCode := strings.TrimSpace(job.PickupCode)
	if pickupCode == "" && job.Status != StatusCollected {
		if code, err := s.EnsurePickupCode(ctx, tenantID, repairID); err == nil {
			pickupCode = code
		}
	}

	var branchName string
	_ = s.pool.QueryRow(ctx, `SELECT name FROM identity.branches WHERE tenant_id = $1 AND id = $2`, tenantID, job.BranchID).Scan(&branchName)
	var technicianName string
	if job.TechnicianID != nil {
		_ = s.pool.QueryRow(ctx, `SELECT display_name FROM identity.users WHERE id = $1`, *job.TechnicianID).Scan(&technicianName)
	}

	return &CustomerReceiptDocument{
		RepairID: job.ID, BranchName: branchName, TechnicianName: technicianName,
		ShopName: shopName, ShopTIN: tax.TIN, ShopAddress1: tax.AddressLine1, ShopAddress2: tax.AddressLine2, ShopCity: tax.City,
		JobCode: jobCode, PickupCode: pickupCode, Status: job.Status,
		ProblemSummary: job.ProblemSummary, CustomerName: customerName, CustomerPhone: customerPhone,
		DeviceLabel: deviceLabel, IMEI: imei,
		PromisedBy: job.PromisedBy, Accessories: job.IntakeAccessories,
		IntakeCondition: derefStr(job.IntakeCondition),
		LaborAmount: labor, PartsAmount: partsAmt, SaleLinesTotal: saleExtra,
		NetSubtotal: netSubtotal, VATAmount: vatAmount, VATRateBPS: tax.VATRateBPS, VATInclusive: tax.VATInclusive,
		TotalDue: total, Paid: paid, Balance: balance,
		Currency: tax.CurrencyCode, Payments: receipts, IssuedAt: time.Now().UTC(),
	}, nil
}

func (d *CustomerReceiptDocument) HTML() string {
	var payRows strings.Builder
	if len(d.Payments) == 0 {
		payRows.WriteString(`<tr><td colspan="3" class="muted">No payments recorded</td></tr>`)
	} else {
		for _, p := range d.Payments {
			ref := ""
			if p.ProviderRef != nil {
				ref = receipts.CustomerPaymentRef(*p.ProviderRef)
			}
			payRows.WriteString(fmt.Sprintf(
				`<tr><td>%s</td><td>%s %.0f</td><td>%s %s</td></tr>`,
				html.EscapeString(strings.ReplaceAll(p.Method, "_", " ")),
				html.EscapeString(p.Currency), p.Amount,
				html.EscapeString(p.Status), html.EscapeString(ref),
			))
		}
	}
	imeiRow := ""
	if d.IMEI != "" {
		imeiRow = fmt.Sprintf(`<div class="meta">IMEI / serial: %s</div>`, html.EscapeString(d.IMEI))
	}
	phoneRow := ""
	if d.CustomerPhone != "" {
		phoneRow = fmt.Sprintf(` · %s`, html.EscapeString(d.CustomerPhone))
	}
	tinRow := ""
	if d.ShopTIN != "" {
		tinRow = fmt.Sprintf(`<div class="meta">TIN: %s</div>`, html.EscapeString(d.ShopTIN))
	}
	vatNote := ""
	if d.VATInclusive {
		vatNote = fmt.Sprintf(`<div class="meta">Prices include VAT (%.2f%%)</div>`, float64(d.VATRateBPS)/100)
	} else {
		vatNote = fmt.Sprintf(`<div class="meta">VAT %.2f%% added</div>`, float64(d.VATRateBPS)/100)
	}
	saleRow := ""
	if d.SaleLinesTotal > 0.009 {
		saleRow = fmt.Sprintf(`<tr><td>Accessories / extras</td><td></td><td>%s %.0f</td></tr>`,
			html.EscapeString(d.Currency), d.SaleLinesTotal)
	}
	return fmt.Sprintf(`<!DOCTYPE html>
<html><head><meta charset="utf-8"><title>Receipt %s</title>
<style>
  body{font-family:ui-sans-serif,system-ui,sans-serif;max-width:640px;margin:24px auto;color:#111;padding:0 16px}
  h1{font-size:1.4rem;margin:0 0 4px}
  .muted{color:#666} .meta{margin:4px 0;font-size:.95rem}
  table{width:100%%;border-collapse:collapse;margin:16px 0}
  th,td{text-align:left;padding:8px 6px;border-bottom:1px solid #e5e5e5}
  .totals td{border:none;font-weight:600} .totals .due{font-size:1.1rem}
  .actions{margin-top:24px} @media print{.actions{display:none} body{margin:0}}
</style></head><body>
  <h1>%s</h1>
  <div class="muted">Repair receipt · %s</div>
  %s
  %s
  <div class="meta"><strong>Job %s</strong> · %s</div>
  <div class="meta">%s%s</div>
  <div class="meta">%s</div>
  %s
  <div class="meta">%s</div>
  <table>
    <thead><tr><th>Item</th><th></th><th>Amount</th></tr></thead>
    <tbody>
      <tr><td>Labor</td><td></td><td>%s %.0f</td></tr>
      <tr><td>Parts</td><td></td><td>%s %.0f</td></tr>
      %s
    </tbody>
  </table>
  <table>
    <thead><tr><th>Payment</th><th>Amount</th><th>Status</th></tr></thead>
    <tbody>%s</tbody>
  </table>
  <table class="totals">
    <tr><td>Subtotal (ex VAT)</td><td></td><td>%s %.0f</td></tr>
    <tr><td>VAT</td><td></td><td>%s %.0f</td></tr>
    <tr><td>Total due</td><td></td><td>%s %.0f</td></tr>
    <tr><td>Paid</td><td></td><td>%s %.0f</td></tr>
    <tr class="due"><td>Balance</td><td></td><td>%s %.0f</td></tr>
  </table>
  <div class="actions"><button onclick="window.print()">Print</button></div>
</body></html>`,
		html.EscapeString(d.JobCode),
		html.EscapeString(d.ShopName),
		html.EscapeString(d.IssuedAt.Format("2 Jan 2006 15:04")),
		tinRow,
		vatNote,
		html.EscapeString(d.JobCode), html.EscapeString(strings.ReplaceAll(d.Status, "_", " ")),
		html.EscapeString(d.CustomerName), phoneRow,
		html.EscapeString(d.DeviceLabel),
		imeiRow,
		html.EscapeString(d.ProblemSummary),
		html.EscapeString(d.Currency), d.LaborAmount,
		html.EscapeString(d.Currency), d.PartsAmount,
		saleRow,
		payRows.String(),
		html.EscapeString(d.Currency), d.NetSubtotal,
		html.EscapeString(d.Currency), d.VATAmount,
		html.EscapeString(d.Currency), d.TotalDue,
		html.EscapeString(d.Currency), d.Paid,
		html.EscapeString(d.Currency), d.Balance,
	)
}

// IntakeSlipHTML is the short check-in ticket: identity + pickup QR only.
// No prices — the final receipt covers money when the job is done.
func (d *CustomerReceiptDocument) IntakeSlipHTML() string {
	phone := ""
	if strings.TrimSpace(d.CustomerPhone) != "" {
		phone = " · " + html.EscapeString(d.CustomerPhone)
	}
	imei := ""
	if strings.TrimSpace(d.IMEI) != "" {
		imei = fmt.Sprintf(`<div class="row"><span class="k">IMEI</span><span class="v">%s</span></div>`, html.EscapeString(d.IMEI))
	}
	problem := strings.TrimSpace(d.ProblemSummary)
	if problem == "" {
		problem = "—"
	}
	branch := ""
	if strings.TrimSpace(d.BranchName) != "" {
		branch = fmt.Sprintf(`<div class="muted tiny">%s</div>`, html.EscapeString(d.BranchName))
	}
	slogan := ""
	if strings.TrimSpace(d.ShopSlogan) != "" {
		slogan = fmt.Sprintf(`<div class="center tiny muted">%s</div>`, html.EscapeString(d.ShopSlogan))
	}
	contactParts := make([]string, 0, 3)
	if v := strings.TrimSpace(d.ShopPhone); v != "" {
		contactParts = append(contactParts, html.EscapeString(v))
	}
	if v := strings.TrimSpace(d.ShopEmail); v != "" {
		contactParts = append(contactParts, receipts.EmailOff(html.EscapeString(v)))
	}
	if v := strings.TrimSpace(d.ShopWebsite); v != "" {
		contactParts = append(contactParts, html.EscapeString(v))
	}
	contact := ""
	if len(contactParts) > 0 {
		contact = fmt.Sprintf(`<div class="center tiny muted">%s</div>`, strings.Join(contactParts, " · "))
	}

	qrBlock := ""
	code := strings.TrimSpace(d.PickupCode)
	if code != "" {
		payload := PickupQRPayload(code)
		img := ""
		if uri, err := receipts.QRDataURIPNG(payload); err == nil {
			img = fmt.Sprintf(`<img class="qr" src="%s" width="140" height="140" alt="Pickup QR">`, html.EscapeString(uri))
		}
		qrBlock = fmt.Sprintf(`
  <div class="callout">
    %s
    <div class="callout-note">Scan this QR when collecting. Your code was sent by SMS.</div>
  </div>`, img)
	}

	return fmt.Sprintf(`<!DOCTYPE html>
<html lang="en"><head><meta charset="utf-8">
<meta name="viewport" content="width=device-width,initial-scale=1">
<title>Intake %s</title>
<style>
  @page { size: 80mm auto; margin: 0; }
  * { box-sizing: border-box; }
  html, body { margin: 0; padding: 0; background: #fff; }
  body {
    font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Helvetica, Arial, sans-serif;
    color: #02012e; -webkit-font-smoothing: antialiased;
  }
  .sheet {
    width: 72mm; margin: 0 auto; padding: 4mm 3mm 6mm;
    font-size: 12px; line-height: 1.35;
  }
  .shop {
    text-align: center; font-weight: 700; font-size: 15px;
    letter-spacing: .04em; text-transform: uppercase; margin: 0 0 2px;
  }
  .title {
    text-align: center; font-weight: 700; letter-spacing: .16em;
    text-transform: uppercase; font-size: 11px; margin: 6px 0 8px;
  }
  .muted { color: #5b5b70; }
  .tiny { font-size: 10px; }
  .center { text-align: center; }
  .rule { border: none; border-top: 1px dashed #c8c8d6; margin: 8px 0; }
  .row { display: flex; justify-content: space-between; gap: 8px; margin-top: 3px; }
  .row .k { color: #5b5b70; flex: 0 0 auto; }
  .row .v { text-align: right; word-break: break-word; }
  .problem { margin-top: 6px; }
  .callout {
    margin-top: 10px; text-align: center;
    border: 1.5px dashed #060386; border-radius: 6px; padding: 8px 6px;
  }
  .qr { display: block; margin: 0 auto 6px; width: 140px; height: 140px; }
  .callout-note { font-size: 10px; color: #5b5b70; margin-top: 4px; }
  .footer { margin-top: 10px; text-align: center; font-size: 10px; color: #5b5b70; }
  .footer p { margin: 0 0 3px; }
  @media print {
    .sheet { width: 80mm; margin: 0; box-shadow: none; }
  }
</style></head><body>
<div class="sheet">
  <div class="shop">%s</div>
  %s
  %s
  %s
  <div class="title">Intake slip</div>
  <div class="center"><strong>%s</strong></div>
  <div class="center muted tiny">%s</div>
  <hr class="rule">
  <div class="row"><span class="k">Customer</span><span class="v">%s%s</span></div>
  <div class="row"><span class="k">Device</span><span class="v">%s</span></div>
  %s
  <div class="problem"><span class="muted">Issue</span><br>%s</div>
  %s
  <div class="footer">
    <p>Keep this slip for collection.</p>
    <p>Final charges appear on your repair receipt.</p>
  </div>
</div>
</body></html>`,
		html.EscapeString(d.JobCode),
		html.EscapeString(d.ShopName),
		slogan,
		contact,
		branch,
		html.EscapeString(d.JobCode),
		html.EscapeString(d.IssuedAt.Format("2 Jan 2006 · 15:04")),
		html.EscapeString(d.CustomerName), phone,
		html.EscapeString(d.DeviceLabel),
		imei,
		html.EscapeString(problem),
		qrBlock,
	)
}
