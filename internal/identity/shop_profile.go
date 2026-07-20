package identity

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

const DefaultVATRateBPS = 1600

type ShopProfile struct {
	TenantID     uuid.UUID `json:"tenant_id"`
	LegalName    string    `json:"legal_name"`
	TIN          string    `json:"tin,omitempty"`
	AddressLine1 string    `json:"address_line1,omitempty"`
	AddressLine2 string    `json:"address_line2,omitempty"`
	City         string    `json:"city,omitempty"`
	Country      string    `json:"country"`
	VATRateBPS   int       `json:"vat_rate_bps"`
	VATInclusive bool      `json:"vat_inclusive"`
	UpdatedAt    time.Time `json:"updated_at"`
}

type UpsertShopProfileInput struct {
	LegalName    *string
	TIN          *string
	AddressLine1 *string
	AddressLine2 *string
	City         *string
	Country      *string
	VATRateBPS   *int
	VATInclusive *bool
}

func (s *Service) GetShopProfile(ctx context.Context, tenantID uuid.UUID) (*ShopProfile, error) {
	var p ShopProfile
	var legalName, tin, addr1, addr2, city *string
	err := s.pool.QueryRow(ctx, `
		SELECT tenant_id, legal_name, tin, address_line1, address_line2, city, country, vat_rate_bps, vat_inclusive, updated_at
		FROM identity.shop_profiles WHERE tenant_id = $1`, tenantID).
		Scan(&p.TenantID, &legalName, &tin, &addr1, &addr2, &city, &p.Country, &p.VATRateBPS, &p.VATInclusive, &p.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		p = defaultShopProfile(tenantID)
		var tenantName string
		if qErr := s.pool.QueryRow(ctx, `SELECT name FROM identity.tenants WHERE id = $1`, tenantID).Scan(&tenantName); qErr == nil {
			p.LegalName = tenantName
		}
		return &p, nil
	}
	if err != nil {
		return nil, err
	}
	if legalName != nil {
		p.LegalName = *legalName
	}
	if tin != nil {
		p.TIN = *tin
	}
	if addr1 != nil {
		p.AddressLine1 = *addr1
	}
	if addr2 != nil {
		p.AddressLine2 = *addr2
	}
	if city != nil {
		p.City = *city
	}
	if p.LegalName == "" {
		var tenantName string
		if qErr := s.pool.QueryRow(ctx, `SELECT name FROM identity.tenants WHERE id = $1`, tenantID).Scan(&tenantName); qErr == nil {
			p.LegalName = tenantName
		}
	}
	return &p, nil
}

func defaultShopProfile(tenantID uuid.UUID) ShopProfile {
	return ShopProfile{
		TenantID: tenantID, Country: "KE",
		VATRateBPS: DefaultVATRateBPS, VATInclusive: true,
		UpdatedAt: time.Now().UTC(),
	}
}

func (s *Service) UpsertShopProfile(ctx context.Context, tenantID uuid.UUID, in UpsertShopProfileInput) (*ShopProfile, error) {
	cur, err := s.GetShopProfile(ctx, tenantID)
	if err != nil {
		return nil, err
	}
	if in.LegalName != nil {
		cur.LegalName = strings.TrimSpace(*in.LegalName)
	}
	if in.TIN != nil {
		cur.TIN = strings.TrimSpace(*in.TIN)
	}
	if in.AddressLine1 != nil {
		cur.AddressLine1 = strings.TrimSpace(*in.AddressLine1)
	}
	if in.AddressLine2 != nil {
		cur.AddressLine2 = strings.TrimSpace(*in.AddressLine2)
	}
	if in.City != nil {
		cur.City = strings.TrimSpace(*in.City)
	}
	if in.Country != nil && strings.TrimSpace(*in.Country) != "" {
		cur.Country = strings.TrimSpace(*in.Country)
	}
	if in.VATRateBPS != nil {
		if *in.VATRateBPS < 0 || *in.VATRateBPS > 10000 {
			return nil, errors.New("vat_rate_bps must be between 0 and 10000")
		}
		cur.VATRateBPS = *in.VATRateBPS
	}
	if in.VATInclusive != nil {
		cur.VATInclusive = *in.VATInclusive
	}
	now := time.Now().UTC()
	_, err = s.pool.Exec(ctx, `
		INSERT INTO identity.shop_profiles
			(tenant_id, legal_name, tin, address_line1, address_line2, city, country, vat_rate_bps, vat_inclusive, updated_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)
		ON CONFLICT (tenant_id) DO UPDATE SET
			legal_name = EXCLUDED.legal_name,
			tin = EXCLUDED.tin,
			address_line1 = EXCLUDED.address_line1,
			address_line2 = EXCLUDED.address_line2,
			city = EXCLUDED.city,
			country = EXCLUDED.country,
			vat_rate_bps = EXCLUDED.vat_rate_bps,
			vat_inclusive = EXCLUDED.vat_inclusive,
			updated_at = EXCLUDED.updated_at`,
		tenantID, nullIfEmpty(cur.LegalName), nullIfEmpty(cur.TIN), nullIfEmpty(cur.AddressLine1),
		nullIfEmpty(cur.AddressLine2), nullIfEmpty(cur.City), cur.Country, cur.VATRateBPS, cur.VATInclusive, now)
	if err != nil {
		return nil, err
	}
	return s.GetShopProfile(ctx, tenantID)
}

func nullIfEmpty(s string) *string {
	if strings.TrimSpace(s) == "" {
		return nil
	}
	v := strings.TrimSpace(s)
	return &v
}

// SplitVAT computes net and VAT amounts from a gross or net total.
func SplitVAT(total float64, rateBPS int, inclusive bool) (net, vat float64) {
	if rateBPS <= 0 {
		return total, 0
	}
	if inclusive {
		vat = total * float64(rateBPS) / float64(10000+rateBPS)
		net = total - vat
		return net, vat
	}
	net = total
	vat = total * float64(rateBPS) / 10000
	return net, vat
}
