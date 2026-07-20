package inventory

import (
	"context"
	"errors"
	"fmt"
	"html"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// SupplierCreditVoucher is the printable credit / delivery voucher for a supplier issue.
type SupplierCreditVoucher struct {
	ShopName       string    `json:"shop_name"`
	ShopTIN        string    `json:"shop_tin,omitempty"`
	SupplierName   string    `json:"supplier_name"`
	JobCode        string    `json:"job_code"`
	Description    string    `json:"description"`
	Quantity       int       `json:"quantity"`
	UnitCost       float64   `json:"unit_cost"`
	NetAmount      float64   `json:"net_amount"`
	VATAmount      float64   `json:"vat_amount"`
	VATRateBPS     int       `json:"vat_rate_bps"`
	VATInclusive   bool      `json:"vat_inclusive"`
	AuthCode       string    `json:"auth_code"`
	IssueID        uuid.UUID `json:"issue_id"`
	IssueStatus    string    `json:"issue_status"`
	Reconciliation string    `json:"reconciliation_status"`
	QRPayload      string    `json:"qr_payload"`
	IssuedAt       time.Time `json:"issued_at"`
	Currency       string    `json:"currency"`
}

func (s *Service) BuildSupplierCreditVoucher(ctx context.Context, tenantID, supplierID, issueID uuid.UUID) (*SupplierCreditVoucher, error) {
	var v SupplierCreditVoucher
	var collectedAt *time.Time
	err := s.pool.QueryRow(ctx, `
		SELECT si.id, si.auth_code, si.status, si.unit_cost::float8, si.reconciliation_status, si.collected_at,
		       COALESCE(rj.job_code, ''), pr.description, pr.quantity,
		       COALESCE(sup.name, 'Supplier'), COALESCE(t.name, 'TechLane')
		FROM inventory.supplier_issues si
		JOIN inventory.part_requests pr ON pr.id = si.part_request_id
		JOIN inventory.suppliers sup ON sup.id = si.supplier_id
		LEFT JOIN repair.repair_jobs rj ON rj.id = si.repair_job_id
		LEFT JOIN identity.tenants t ON t.id = si.tenant_id
		WHERE si.tenant_id = $1 AND si.supplier_id = $2 AND si.id = $3`,
		tenantID, supplierID, issueID).
		Scan(&v.IssueID, &v.AuthCode, &v.IssueStatus, &v.UnitCost, &v.Reconciliation, &collectedAt,
			&v.JobCode, &v.Description, &v.Quantity, &v.SupplierName, &v.ShopName)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, fmt.Errorf("issue not found")
	}
	if err != nil {
		return nil, err
	}
	v.Currency = "KES"
	v.QRPayload = QRPayloadForIssue(v.IssueID, v.AuthCode)
	if collectedAt != nil {
		v.IssuedAt = *collectedAt
	} else {
		v.IssuedAt = time.Now().UTC()
	}
	tax := s.loadShopTaxProfile(ctx, tenantID)
	if tax.LegalName != "" {
		v.ShopName = tax.LegalName
	}
	v.ShopTIN = tax.TIN
	v.VATRateBPS = tax.VATRateBPS
	v.VATInclusive = tax.VATInclusive
	v.NetAmount, v.VATAmount = splitVAT(v.UnitCost, tax.VATRateBPS, tax.VATInclusive)
	return &v, nil
}

func (v *SupplierCreditVoucher) HTML() string {
	return fmt.Sprintf(`<!DOCTYPE html>
<html><head><meta charset="utf-8"><title>Credit voucher %s</title>
<style>
  body{font-family:ui-sans-serif,system-ui,sans-serif;max-width:640px;margin:24px auto;color:#111;padding:0 16px}
  h1{font-size:1.35rem;margin:0 0 4px}
  .muted{color:#666} .code{font-size:1.6rem;letter-spacing:.12em;font-weight:700;margin:12px 0}
  .box{border:1px solid #ddd;border-radius:8px;padding:16px;margin:16px 0}
  table{width:100%%;border-collapse:collapse} td{padding:6px 0}
  .actions{margin-top:24px} @media print{.actions{display:none} body{margin:0}}
</style></head><body>
  <h1>Supplier credit voucher</h1>
  <div class="muted">%s · %s</div>
  <div class="box">
    <div><strong>%s</strong></div>
    <div class="muted">Job %s</div>
    <table>
      <tr><td>Part</td><td><strong>%s</strong></td></tr>
      <tr><td>Qty</td><td>%d</td></tr>
      <tr><td>Credit</td><td><strong>%s %.0f</strong></td></tr>
      <tr><td>Net (ex VAT)</td><td>%s %.0f</td></tr>
      <tr><td>VAT</td><td>%s %.0f</td></tr>
      <tr><td>Status</td><td>%s · recon %s</td></tr>
    </table>
    <div class="muted">Auth code for shop pickup</div>
    <div class="code">%s</div>
    <div class="muted">QR: %s</div>
  </div>
  <div class="actions"><button onclick="window.print()">Print voucher</button></div>
</body></html>`,
		html.EscapeString(v.AuthCode),
		html.EscapeString(v.ShopName),
		html.EscapeString(v.IssuedAt.Format("2 Jan 2006 15:04")),
		html.EscapeString(v.SupplierName),
		html.EscapeString(v.JobCode),
		html.EscapeString(v.Description),
		v.Quantity,
		html.EscapeString(v.Currency), v.UnitCost,
		html.EscapeString(v.Currency), v.NetAmount,
		html.EscapeString(v.Currency), v.VATAmount,
		html.EscapeString(v.IssueStatus), html.EscapeString(v.Reconciliation),
		html.EscapeString(v.AuthCode),
		html.EscapeString(v.QRPayload),
	)
}
