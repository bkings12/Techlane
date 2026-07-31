package storefrontcms

import (
	"context"

	"github.com/google/uuid"
)

// RecordView increments the aggregate view counter for a variant. The
// frontend dedupes per browser session before calling this, so this stays a
// plain increment rather than needing per-visitor tracking here.
func (s *Service) RecordView(ctx context.Context, tenantID, variantID uuid.UUID) error {
	_, err := s.pool.Exec(ctx, `
		INSERT INTO platform.product_view_counts (tenant_id, variant_id, view_count, last_viewed_at)
		VALUES ($1, $2, 1, now())
		ON CONFLICT (tenant_id, variant_id) DO UPDATE SET
			view_count = platform.product_view_counts.view_count + 1,
			last_viewed_at = now()`,
		tenantID, variantID)
	return err
}

// TopViewedVariantIDs returns variant IDs ordered by view count, most first.
func (s *Service) TopViewedVariantIDs(ctx context.Context, tenantID uuid.UUID, limit int) ([]uuid.UUID, error) {
	if limit <= 0 {
		limit = 12
	}
	rows, err := s.pool.Query(ctx, `
		SELECT variant_id FROM platform.product_view_counts
		WHERE tenant_id = $1
		ORDER BY view_count DESC, last_viewed_at DESC
		LIMIT $2`, tenantID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	ids := make([]uuid.UUID, 0, limit)
	for rows.Next() {
		var id uuid.UUID
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}
