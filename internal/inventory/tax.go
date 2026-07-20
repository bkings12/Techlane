package inventory

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

type shopTaxProfile struct {
	LegalName    string
	TIN          string
	VATRateBPS   int
	VATInclusive bool
}

func (s *Service) loadShopTaxProfile(ctx context.Context, tenantID uuid.UUID) shopTaxProfile {
	var p shopTaxProfile
	var legalName, tin *string
	err := s.pool.QueryRow(ctx, `
		SELECT legal_name, tin, vat_rate_bps, vat_inclusive
		FROM identity.shop_profiles WHERE tenant_id = $1`, tenantID).
		Scan(&legalName, &tin, &p.VATRateBPS, &p.VATInclusive)
	if errors.Is(err, pgx.ErrNoRows) {
		p.VATRateBPS = 1600
		p.VATInclusive = true
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
