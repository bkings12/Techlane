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
)

// CustomerReceiptDocument is everything needed to print a customer receipt.
type CustomerReceiptDocument struct {
	ShopName       string          `json:"shop_name"`
	ShopTIN        string          `json:"shop_tin,omitempty"`
	ShopAddress1   string          `json:"shop_address_line1,omitempty"`
	ShopAddress2   string          `json:"shop_address_line2,omitempty"`
	ShopCity       string          `json:"shop_city,omitempty"`
	JobCode        string          `json:"job_code"`
	Status         string          `json:"status"`
	ProblemSummary string          `json:"problem_summary"`
	CustomerName   string          `json:"customer_name"`
	CustomerPhone  string          `json:"customer_phone,omitempty"`
	DeviceLabel    string          `json:"device_label"`
	IMEI           string          `json:"imei,omitempty"`
	LaborAmount    float64         `json:"labor_amount"`
	PartsAmount    float64         `json:"parts_amount"`
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

	receipts, err := s.PublicRepairReceipts(ctx, tenantID, repairID)
	if err != nil {
		return nil, err
	}
	var paid float64
	for _, r := range receipts {
		if r.Status == "allocated" || r.Status == "confirmed" {
			paid += r.Amount
		}
	}
	total := labor + partsAmt
	netSubtotal, vatAmount := splitVAT(total, tax.VATRateBPS, tax.VATInclusive)
	balance := total - paid
	if balance < 0 {
		balance = 0
	}
	jobCode := job.JobCode
	if jobCode == "" {
		jobCode = job.ID.String()[:8]
	}

	return &CustomerReceiptDocument{
		ShopName: shopName, ShopTIN: tax.TIN, ShopAddress1: tax.AddressLine1, ShopAddress2: tax.AddressLine2, ShopCity: tax.City,
		JobCode: jobCode, Status: job.Status,
		ProblemSummary: job.ProblemSummary, CustomerName: customerName, CustomerPhone: customerPhone,
		DeviceLabel: deviceLabel, IMEI: imei,
		LaborAmount: labor, PartsAmount: partsAmt,
		NetSubtotal: netSubtotal, VATAmount: vatAmount, VATRateBPS: tax.VATRateBPS, VATInclusive: tax.VATInclusive,
		TotalDue: total, Paid: paid, Balance: balance,
		Currency: "KES", Payments: receipts, IssuedAt: time.Now().UTC(),
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
				ref = *p.ProviderRef
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
		payRows.String(),
		html.EscapeString(d.Currency), d.NetSubtotal,
		html.EscapeString(d.Currency), d.VATAmount,
		html.EscapeString(d.Currency), d.TotalDue,
		html.EscapeString(d.Currency), d.Paid,
		html.EscapeString(d.Currency), d.Balance,
	)
}
