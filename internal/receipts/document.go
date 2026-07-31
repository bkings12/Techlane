package receipts

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// Document kinds. These also key the receipt number series.
const (
	KindRepair     = "repair"
	KindSale       = "sale"
	KindTaxInvoice = "tax_invoice"
	KindVoucher    = "voucher"
)

// Shop is the letterhead: who issued this document.
type Shop struct {
	Name         string
	TIN          string
	AddressLines []string
	Phone        string
	Email        string
	Website      string
	LogoDataURI  string
}

// MetaRow is a label/value pair in the document's detail block.
type MetaRow struct {
	Label string
	Value string
}

// Line is a billable row. Qty of 0 renders as a plain line with no unit maths.
type Line struct {
	Description string
	Detail      string
	Qty         float64
	UnitPrice   float64
	Amount      float64
}

// Callout is a prominent code block printed inside a document.
type Callout struct {
	Label string
	Value string
	Note  string
	// QRDataURI is an optional data:image/png;base64,… rendered above the code
	// so the counter can scan the slip instead of typing digits.
	QRDataURI string
}

// PaymentLine is one settled or pending payment against the document.
type PaymentLine struct {
	Method    string
	Amount    float64
	Status    string
	Reference string
	At        *time.Time
}

// Document is the paper-agnostic model every receipt renders from.
type Document struct {
	Kind      string
	Title     string
	Number    string
	Reference string
	IssuedAt  time.Time
	Currency  string

	CustomerName  string
	CustomerPhone string

	Meta  []MetaRow
	Lines []Line
	Notes string

	Subtotal     float64
	VATAmount    float64
	VATRateBPS   int
	VATInclusive bool
	Total        float64
	Paid         float64
	Balance      float64

	Payments []PaymentLine

	Branch   string
	ServedBy string

	// Callout is a boxed code the holder must present, e.g. a supplier pickup
	// authorisation code.
	Callout *Callout

	// StatusNote prints under the title, e.g. "Reversed" or "Awaiting payment".
	StatusNote string

	// SuppressBalance hides the paid/balance rows on documents where they make
	// no sense, such as a supplier credit voucher.
	SuppressBalance bool
}

func (d Document) hasVAT() bool { return d.VATRateBPS > 0 && d.VATAmount > 0.005 }

func (d Document) vatLabel() string {
	rate := float64(d.VATRateBPS) / 100
	if rate == float64(int(rate)) {
		return fmt.Sprintf("VAT %d%%", int(rate))
	}
	return fmt.Sprintf("VAT %.2f%%", rate)
}

// LoadShop assembles the letterhead from the tax profile plus receipt settings.
func (s *Service) LoadShop(ctx context.Context, tenantID uuid.UUID, set Settings) Shop {
	shop := Shop{Phone: set.Phone, Email: set.Email, Website: set.Website}

	var legalName, tin, addr1, addr2, city *string
	err := s.pool.QueryRow(ctx, `
		SELECT legal_name, tin, address_line1, address_line2, city
		FROM identity.shop_profiles WHERE tenant_id = $1`, tenantID).
		Scan(&legalName, &tin, &addr1, &addr2, &city)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return shop
	}
	shop.Name = deref(legalName)
	if set.ShowTIN {
		shop.TIN = deref(tin)
	}
	if set.ShowAddress {
		for _, part := range []string{deref(addr1), deref(addr2), deref(city)} {
			if strings.TrimSpace(part) != "" {
				shop.AddressLines = append(shop.AddressLines, strings.TrimSpace(part))
			}
		}
	}
	if shop.Name == "" {
		var tenantName string
		if qErr := s.pool.QueryRow(ctx, `SELECT name FROM identity.tenants WHERE id = $1`, tenantID).Scan(&tenantName); qErr == nil {
			shop.Name = tenantName
		}
	}
	if shop.Name == "" {
		shop.Name = "TechLane"
	}
	if set.ShowLogo {
		shop.LogoDataURI = s.LogoDataURI(ctx, tenantID)
	}
	return shop
}

// Currency resolves the tenant's ISO code, falling back to KES.
func (s *Service) Currency(ctx context.Context, tenantID uuid.UUID) string {
	var code *string
	if err := s.pool.QueryRow(ctx, `
		SELECT currency_code FROM identity.shop_profiles WHERE tenant_id = $1`, tenantID).Scan(&code); err == nil {
		if c := strings.TrimSpace(deref(code)); c != "" {
			return strings.ToUpper(c)
		}
	}
	return "KES"
}

// VATProfile returns the tenant's VAT rate and whether prices include it.
func (s *Service) VATProfile(ctx context.Context, tenantID uuid.UUID) (rateBPS int, inclusive bool) {
	rateBPS, inclusive = 1600, true
	_ = s.pool.QueryRow(ctx, `
		SELECT vat_rate_bps, vat_inclusive FROM identity.shop_profiles WHERE tenant_id = $1`, tenantID).
		Scan(&rateBPS, &inclusive)
	return rateBPS, inclusive
}

// showBalance reports whether the paid/balance rows belong on this document.
func (d Document) showBalance(set Settings) bool { return set.ShowBalance && !d.SuppressBalance }

// applySettings strips anything the owner switched off before rendering.
func (d *Document) applySettings(set Settings) {
	if !set.ShowPayments {
		d.Payments = nil
	}
	if !set.ShowServedBy {
		d.ServedBy = ""
	}
	if !set.ShowIMEI {
		d.Meta = filterMeta(d.Meta, "IMEI")
	}
}

func filterMeta(rows []MetaRow, prefix string) []MetaRow {
	out := rows[:0:0]
	for _, row := range rows {
		if strings.HasPrefix(strings.ToUpper(row.Label), strings.ToUpper(prefix)) {
			continue
		}
		out = append(out, row)
	}
	return out
}
