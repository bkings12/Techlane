package commerce

import (
	"context"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/techlane/techlane/internal/repair"
)

var nonDigits = regexp.MustCompile(`[^0-9]`)

// TrackOrderHit is a public-safe online order match for guest track.
type TrackOrderHit struct {
	ID             string    `json:"id"`
	Ref            string    `json:"ref"`
	Status         string    `json:"status"`
	Total          float64   `json:"total"`
	FulfilmentType string    `json:"fulfilment_type,omitempty"`
	CreatedAt      time.Time `json:"created_at"`
}

// TrackRepairHit is a public-safe repair job match for guest track.
type TrackRepairHit struct {
	ID             string         `json:"id"`
	JobCode        string         `json:"job_code"`
	JobNumber      int            `json:"job_number"`
	Status         string         `json:"status"`
	ProblemSummary string         `json:"problem_summary,omitempty"`
	Device         map[string]any `json:"device,omitempty"`
	CreatedAt      time.Time      `json:"created_at"`
}

// TrackResult is the combined guest lookup response.
type TrackResult struct {
	Query   string           `json:"query"`
	Orders  []TrackOrderHit  `json:"orders"`
	Repairs []TrackRepairHit `json:"repairs"`
}

// PublicTrack finds online orders and repair jobs by order id/ref, job code/number, or phone.
func (s *Service) PublicTrack(ctx context.Context, tenantID uuid.UUID, raw string) (*TrackResult, error) {
	q := strings.TrimSpace(raw)
	out := &TrackResult{
		Query:   q,
		Orders:  make([]TrackOrderHit, 0),
		Repairs: make([]TrackRepairHit, 0),
	}
	if q == "" {
		return out, nil
	}

	seenOrders := map[uuid.UUID]struct{}{}
	seenRepairs := map[uuid.UUID]struct{}{}

	addOrder := func(id uuid.UUID, status string, total float64, fulfilment string, created time.Time) {
		if _, ok := seenOrders[id]; ok {
			return
		}
		seenOrders[id] = struct{}{}
		out.Orders = append(out.Orders, TrackOrderHit{
			ID:             id.String(),
			Ref:            orderRef(id),
			Status:         status,
			Total:          total,
			FulfilmentType: fulfilment,
			CreatedAt:      created,
		})
	}
	addRepair := func(id uuid.UUID, code string, number int, status, summary string, created time.Time, device map[string]any) {
		if _, ok := seenRepairs[id]; ok {
			return
		}
		seenRepairs[id] = struct{}{}
		out.Repairs = append(out.Repairs, TrackRepairHit{
			ID:             id.String(),
			JobCode:        code,
			JobNumber:      number,
			Status:         status,
			ProblemSummary: summary,
			Device:         device,
			CreatedAt:      created,
		})
	}

	upper := strings.ToUpper(q)
	digits := nonDigits.ReplaceAllString(q, "")

	// Exact order UUID.
	if id, err := uuid.Parse(q); err == nil {
		if o, err := s.GetOrder(ctx, tenantID, id); err == nil && o != nil {
			addOrder(o.ID, o.Status, o.Total, o.FulfilmentType, o.CreatedAt)
		}
	}

	// ORD-XXXXXXXX (first 8 hex chars of UUID).
	refKey := strings.TrimPrefix(upper, "ORD-")
	refKey = strings.ReplaceAll(refKey, "-", "")
	if len(refKey) >= 8 && isHex(refKey[:8]) {
		prefix := strings.ToLower(refKey[:8])
		rows, err := s.pool.Query(ctx, `
			SELECT id, status, total::float8, fulfilment_type, created_at
			FROM sales.orders
			WHERE tenant_id = $1 AND channel = 'online'
			  AND REPLACE(id::text, '-', '') LIKE $2 || '%'
			ORDER BY created_at DESC LIMIT 10`, tenantID, prefix)
		if err != nil {
			return nil, err
		}
		for rows.Next() {
			var id uuid.UUID
			var status, fulfilment string
			var total float64
			var created time.Time
			if err := rows.Scan(&id, &status, &total, &fulfilment, &created); err != nil {
				rows.Close()
				return nil, err
			}
			addOrder(id, status, total, fulfilment, created)
		}
		rows.Close()
		if err := rows.Err(); err != nil {
			return nil, err
		}
	}

	// Collection / pickup code on paid orders.
	if len(q) >= 4 && len(q) <= 32 {
		rows, err := s.pool.Query(ctx, `
			SELECT id, status, total::float8, fulfilment_type, created_at
			FROM sales.orders
			WHERE tenant_id = $1 AND channel = 'online'
			  AND UPPER(COALESCE(collection_code, '')) = UPPER($2)
			ORDER BY created_at DESC LIMIT 10`, tenantID, q)
		if err != nil {
			return nil, err
		}
		for rows.Next() {
			var id uuid.UUID
			var status, fulfilment string
			var total float64
			var created time.Time
			if err := rows.Scan(&id, &status, &total, &fulfilment, &created); err != nil {
				rows.Close()
				return nil, err
			}
			addOrder(id, status, total, fulfilment, created)
		}
		rows.Close()
	}

	// Repair by job code (JOB-110) or bare job number.
	jobQuery := upper
	if strings.HasPrefix(jobQuery, "JOB") && !strings.HasPrefix(jobQuery, "JOB-") && len(digits) > 0 {
		jobQuery = "JOB-" + digits
	}
	rows, err := s.pool.Query(ctx, `
		SELECT j.id, j.job_code, j.job_number, j.status, j.problem_summary, j.created_at,
		       d.kind, d.brand, d.model
		FROM repair.repair_jobs j
		LEFT JOIN repair.devices d ON d.id = j.device_id
		WHERE j.tenant_id = $1
		  AND j.deleted_at IS NULL
		  AND (
		    UPPER(j.job_code) = UPPER($2)
		    OR ($3 <> '' AND j.job_number::text = $3)
		    OR UPPER(j.job_code) LIKE UPPER($2) || '%'
		  )
		ORDER BY
		  CASE WHEN UPPER(j.job_code) = UPPER($2) THEN 0
		       WHEN $3 <> '' AND j.job_number::text = $3 THEN 0
		       ELSE 1 END,
		  j.created_at DESC
		LIMIT 20`, tenantID, jobQuery, digits)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var id uuid.UUID
		var code, status, summary string
		var number int
		var created time.Time
		var kind string
		var brand, model *string
		if err := rows.Scan(&id, &code, &number, &status, &summary, &created, &kind, &brand, &model); err != nil {
			rows.Close()
			return nil, err
		}
		addRepair(id, code, number, status, summary, created, publicDevice(kind, brand, model))
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}

	// Phone lookup (orders + repairs).
	variants := repair.PhoneMatchVariants(q)
	if len(variants) > 0 && len(digits) >= 9 {
		orderRows, err := s.pool.Query(ctx, `
			SELECT o.id, o.status, o.total::float8, o.fulfilment_type, o.created_at
			FROM sales.orders o
			LEFT JOIN repair.customers c ON c.id = o.customer_id AND c.tenant_id = o.tenant_id
			WHERE o.tenant_id = $1 AND o.channel = 'online'
			  AND (
			    regexp_replace(COALESCE(o.guest_phone, ''), '[^0-9]', '', 'g') = ANY($2::text[])
			    OR regexp_replace(COALESCE(c.phone, ''), '[^0-9]', '', 'g') = ANY($2::text[])
			  )
			ORDER BY o.created_at DESC LIMIT 20`, tenantID, variants)
		if err != nil {
			return nil, err
		}
		for orderRows.Next() {
			var id uuid.UUID
			var status, fulfilment string
			var total float64
			var created time.Time
			if err := orderRows.Scan(&id, &status, &total, &fulfilment, &created); err != nil {
				orderRows.Close()
				return nil, err
			}
			addOrder(id, status, total, fulfilment, created)
		}
		orderRows.Close()

		repairRows, err := s.pool.Query(ctx, `
			SELECT j.id, j.job_code, j.job_number, j.status, j.problem_summary, j.created_at,
			       d.kind, d.brand, d.model
			FROM repair.repair_jobs j
			JOIN repair.customers c ON c.id = j.customer_id
			LEFT JOIN repair.devices d ON d.id = j.device_id
			WHERE j.tenant_id = $1
			  AND j.deleted_at IS NULL
			  AND regexp_replace(COALESCE(c.phone, ''), '[^0-9]', '', 'g') = ANY($2::text[])
			ORDER BY j.created_at DESC LIMIT 20`, tenantID, variants)
		if err != nil {
			return nil, err
		}
		for repairRows.Next() {
			var id uuid.UUID
			var code, status, summary string
			var number int
			var created time.Time
			var kind string
			var brand, model *string
			if err := repairRows.Scan(&id, &code, &number, &status, &summary, &created, &kind, &brand, &model); err != nil {
				repairRows.Close()
				return nil, err
			}
			addRepair(id, code, number, status, summary, created, publicDevice(kind, brand, model))
		}
		repairRows.Close()
	}

	return out, nil
}

func orderRef(id uuid.UUID) string {
	short := strings.ToUpper(strings.ReplaceAll(id.String(), "-", ""))
	if len(short) > 8 {
		short = short[:8]
	}
	return "ORD-" + short
}

func isHex(s string) bool {
	for _, c := range s {
		if (c < '0' || c > '9') && (c < 'A' || c > 'F') && (c < 'a' || c > 'f') {
			return false
		}
	}
	return true
}

func publicDevice(kind string, brand, model *string) map[string]any {
	d := &repair.Device{Kind: kind, Brand: brand, Model: model}
	return repair.PublicDeviceView(d)
}
