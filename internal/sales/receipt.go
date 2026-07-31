package sales

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/techlane/techlane/internal/receipts"
)

// BuildSaleReceipt assembles the printable document for a POS sale.
func (s *Service) BuildSaleReceipt(ctx context.Context, tenantID, saleID uuid.UUID) (receipts.Document, error) {
	var (
		branchID          uuid.UUID
		customerID        *uuid.UUID
		createdBy         *uuid.UUID
		channel, status   string
		subtotal, taxT    float64
		discount, total   float64
		createdAt         time.Time
		vatRateBPS        int
		vatInclusive      bool
		currency          = "KES"
		branchName        string
		cashierName       string
		custName, custTel string
	)

	err := s.pool.QueryRow(ctx, `
		SELECT branch_id, customer_id, channel, status, subtotal::float8, tax_total::float8,
		       discount_total::float8, total::float8, created_at, created_by
		FROM sales.sales WHERE tenant_id = $1 AND id = $2`, tenantID, saleID).
		Scan(&branchID, &customerID, &channel, &status, &subtotal, &taxT, &discount, &total, &createdAt, &createdBy)
	if errors.Is(err, pgx.ErrNoRows) {
		return receipts.Document{}, fmt.Errorf("sale not found")
	}
	if err != nil {
		return receipts.Document{}, err
	}

	var currencyCode *string
	_ = s.pool.QueryRow(ctx, `
		SELECT vat_rate_bps, vat_inclusive, currency_code
		FROM identity.shop_profiles WHERE tenant_id = $1`, tenantID).
		Scan(&vatRateBPS, &vatInclusive, &currencyCode)
	if vatRateBPS == 0 && currencyCode == nil {
		vatRateBPS, vatInclusive = 1600, true
	}
	if currencyCode != nil && strings.TrimSpace(*currencyCode) != "" {
		currency = strings.ToUpper(strings.TrimSpace(*currencyCode))
	}

	_ = s.pool.QueryRow(ctx, `SELECT name FROM identity.branches WHERE tenant_id = $1 AND id = $2`, tenantID, branchID).Scan(&branchName)
	if createdBy != nil {
		_ = s.pool.QueryRow(ctx, `SELECT display_name FROM identity.users WHERE id = $1`, *createdBy).Scan(&cashierName)
	}
	if customerID != nil {
		var phone *string
		_ = s.pool.QueryRow(ctx, `SELECT full_name, phone FROM repair.customers WHERE tenant_id = $1 AND id = $2`,
			tenantID, *customerID).Scan(&custName, &phone)
		if phone != nil {
			custTel = *phone
		}
	}

	doc := receipts.Document{
		Kind:          receipts.KindSale,
		Title:         "Sales receipt",
		Reference:     shortRef(saleID),
		IssuedAt:      createdAt,
		Currency:      currency,
		CustomerName:  custName,
		CustomerPhone: custTel,
		Branch:        branchName,
		ServedBy:      cashierName,
		VATRateBPS:    vatRateBPS,
		VATInclusive:  vatInclusive,
		Total:         total,
	}
	doc.Meta = []receipts.MetaRow{
		{Label: "Sale", Value: shortRef(saleID)},
		{Label: "Channel", Value: prettyChannel(channel)},
	}
	if status != "completed" {
		doc.StatusNote = strings.ToUpper(strings.ReplaceAll(status, "_", " "))
	}

	rows, err := s.pool.Query(ctx, `
		SELECT COALESCE(p.brand || ' ' || p.name, p.name, si.description, 'Item'), v.sku,
		       si.quantity, si.unit_price::float8, si.line_total::float8
		FROM sales.sale_items si
		LEFT JOIN inventory.product_variants v ON v.id = si.variant_id
		LEFT JOIN inventory.products p ON p.id = v.product_id
		WHERE si.tenant_id = $1 AND si.sale_id = $2
		ORDER BY si.id`, tenantID, saleID)
	if err != nil {
		return receipts.Document{}, err
	}
	defer rows.Close()
	for rows.Next() {
		var name, sku *string
		var qty int
		var unit, line float64
		if err := rows.Scan(&name, &sku, &qty, &unit, &line); err != nil {
			return receipts.Document{}, err
		}
		desc := "Item"
		if name != nil && strings.TrimSpace(*name) != "" {
			desc = strings.TrimSpace(*name)
		}
		detail := ""
		if sku != nil && strings.TrimSpace(*sku) != "" {
			detail = "SKU " + strings.TrimSpace(*sku)
		}
		doc.Lines = append(doc.Lines, receipts.Line{
			Description: desc, Detail: detail,
			Qty: float64(qty), UnitPrice: unit, Amount: line,
		})
	}
	if err := rows.Err(); err != nil {
		return receipts.Document{}, err
	}

	if discount > 0 {
		doc.Lines = append(doc.Lines, receipts.Line{Description: "Discount", Amount: -discount})
	}

	payRows, err := s.pool.Query(ctx, `
		SELECT p.method, p.amount::float8, p.status,
			COALESCE(stk.mpesa_receipt, ''),
			COALESCE(stk.raw_callback::text, ''),
			COALESCE(c2b.trans_id, ''),
			COALESCE(bank.provider_ref, ''),
			COALESCE(p.provider_ref, ''),
			p.created_at
		FROM payments.payments p
		JOIN payments.payment_allocations a ON a.payment_id = p.id
		LEFT JOIN LATERAL (
			SELECT mpesa_receipt, raw_callback FROM payments.mpesa_stk_transactions
			WHERE payment_id = p.id ORDER BY created_at DESC LIMIT 1
		) stk ON true
		LEFT JOIN LATERAL (
			SELECT trans_id FROM payments.mpesa_c2b_transactions
			WHERE payment_id = p.id AND status IS DISTINCT FROM 'superseded'
			ORDER BY created_at DESC LIMIT 1
		) c2b ON true
		LEFT JOIN LATERAL (
			SELECT provider_ref FROM payments.bank_transactions
			WHERE payment_id = p.id ORDER BY created_at DESC LIMIT 1
		) bank ON true
		WHERE a.tenant_id = $1 AND a.payable_type = 'sale' AND a.payable_id = $2
		ORDER BY p.created_at`, tenantID, saleID)
	if err != nil {
		return receipts.Document{}, err
	}
	defer payRows.Close()
	for payRows.Next() {
		var method, payStatus string
		var amount float64
		var stkReceipt, rawCallback, c2bTrans, bankRef, providerRef string
		var at time.Time
		if err := payRows.Scan(&method, &amount, &payStatus, &stkReceipt, &rawCallback, &c2bTrans, &bankRef, &providerRef, &at); err != nil {
			return receipts.Document{}, err
		}
		reference := receipts.CustomerPaymentRef(
			stkReceipt,
			receipts.MpesaReceiptFromSTKCallback(rawCallback),
			c2bTrans,
			bankRef,
			providerRef,
		)
		stamp := at
		doc.Payments = append(doc.Payments, receipts.PaymentLine{
			Method: method, Amount: amount, Status: payStatus, Reference: reference, At: &stamp,
		})
		if payStatus == "allocated" || payStatus == "confirmed" {
			doc.Paid += amount
		}
	}
	if err := payRows.Err(); err != nil {
		return receipts.Document{}, err
	}

	doc.Balance = doc.Total - doc.Paid
	if doc.Balance < 0 {
		doc.Balance = 0
	}
	doc.Subtotal, doc.VATAmount = receipts.SplitVAT(doc.Total, doc.VATRateBPS, doc.VATInclusive)
	return doc, nil
}

func shortRef(id uuid.UUID) string {
	return "SL-" + strings.ToUpper(id.String()[:8])
}

func prettyChannel(channel string) string {
	switch strings.ToLower(channel) {
	case "pos":
		return "Counter"
	case "online":
		return "Online"
	case "":
		return "Counter"
	}
	return strings.ToUpper(channel[:1]) + channel[1:]
}
