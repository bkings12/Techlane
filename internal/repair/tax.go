package repair

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

type ShopTaxProfile struct {
	LegalName    string
	TIN          string
	AddressLine1 string
	AddressLine2 string
	City         string
	Country      string
	VATRateBPS   int
	VATInclusive bool
}

func (s *Service) loadShopTaxProfile(ctx context.Context, tenantID uuid.UUID) ShopTaxProfile {
	var p ShopTaxProfile
	var legalName, tin, addr1, addr2, city *string
	err := s.pool.QueryRow(ctx, `
		SELECT legal_name, tin, address_line1, address_line2, city, country, vat_rate_bps, vat_inclusive
		FROM identity.shop_profiles WHERE tenant_id = $1`, tenantID).
		Scan(&legalName, &tin, &addr1, &addr2, &city, &p.Country, &p.VATRateBPS, &p.VATInclusive)
	if errors.Is(err, pgx.ErrNoRows) {
		p.VATRateBPS = 1600
		p.VATInclusive = true
		p.Country = "KE"
	} else if err != nil {
		p.VATRateBPS = 1600
		p.VATInclusive = true
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
		_ = s.pool.QueryRow(ctx, `SELECT name FROM identity.tenants WHERE id = $1`, tenantID).Scan(&p.LegalName)
	}
	return p
}

func splitVAT(total float64, rateBPS int, inclusive bool) (net, vat float64) {
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
