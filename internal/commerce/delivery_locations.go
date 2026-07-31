package commerce

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

type DeliveryLocation struct {
	ID          uuid.UUID `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description,omitempty"`
	Fee         float64   `json:"fee"`
	Active      bool      `json:"active"`
	SortOrder   int       `json:"sort_order"`
	CreatedAt   time.Time `json:"created_at,omitempty"`
	UpdatedAt   time.Time `json:"updated_at,omitempty"`
}

type DeliveryLocationInput struct {
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Fee         float64  `json:"fee"`
	Active      *bool    `json:"active"`
	SortOrder   *int     `json:"sort_order"`
}

func (s *Service) ListDeliveryLocations(ctx context.Context, tenantID uuid.UUID, activeOnly bool) ([]DeliveryLocation, error) {
	q := `
		SELECT id, name, COALESCE(description, ''), fee::float8, active, sort_order, created_at, updated_at
		FROM sales.delivery_locations
		WHERE tenant_id = $1`
	if activeOnly {
		q += ` AND active = TRUE`
	}
	q += ` ORDER BY sort_order ASC, name ASC`
	rows, err := s.pool.Query(ctx, q, tenantID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]DeliveryLocation, 0)
	for rows.Next() {
		var loc DeliveryLocation
		if err := rows.Scan(&loc.ID, &loc.Name, &loc.Description, &loc.Fee, &loc.Active, &loc.SortOrder, &loc.CreatedAt, &loc.UpdatedAt); err != nil {
			return nil, err
		}
		items = append(items, loc)
	}
	return items, rows.Err()
}

func (s *Service) GetDeliveryLocation(ctx context.Context, tenantID, id uuid.UUID) (*DeliveryLocation, error) {
	var loc DeliveryLocation
	err := s.pool.QueryRow(ctx, `
		SELECT id, name, COALESCE(description, ''), fee::float8, active, sort_order, created_at, updated_at
		FROM sales.delivery_locations
		WHERE tenant_id = $1 AND id = $2`, tenantID, id).
		Scan(&loc.ID, &loc.Name, &loc.Description, &loc.Fee, &loc.Active, &loc.SortOrder, &loc.CreatedAt, &loc.UpdatedAt)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, fmt.Errorf("delivery location not found")
		}
		return nil, err
	}
	return &loc, nil
}

func (s *Service) CreateDeliveryLocation(ctx context.Context, tenantID uuid.UUID, in DeliveryLocationInput) (*DeliveryLocation, error) {
	name := strings.TrimSpace(in.Name)
	if name == "" {
		return nil, fmt.Errorf("name required")
	}
	if in.Fee < 0 {
		return nil, fmt.Errorf("fee must be >= 0")
	}
	active := true
	if in.Active != nil {
		active = *in.Active
	}
	sortOrder := 0
	if in.SortOrder != nil {
		sortOrder = *in.SortOrder
	}
	id := uuid.New()
	_, err := s.pool.Exec(ctx, `
		INSERT INTO sales.delivery_locations (id, tenant_id, name, description, fee, active, sort_order)
		VALUES ($1,$2,$3,NULLIF($4,''),$5,$6,$7)`,
		id, tenantID, name, strings.TrimSpace(in.Description), in.Fee, active, sortOrder)
	if err != nil {
		return nil, err
	}
	return s.GetDeliveryLocation(ctx, tenantID, id)
}

func (s *Service) UpdateDeliveryLocation(ctx context.Context, tenantID, id uuid.UUID, in DeliveryLocationInput) (*DeliveryLocation, error) {
	cur, err := s.GetDeliveryLocation(ctx, tenantID, id)
	if err != nil {
		return nil, err
	}
	name := strings.TrimSpace(in.Name)
	if name == "" {
		name = cur.Name
	}
	desc := strings.TrimSpace(in.Description)
	fee := in.Fee
	if fee < 0 {
		return nil, fmt.Errorf("fee must be >= 0")
	}
	active := cur.Active
	if in.Active != nil {
		active = *in.Active
	}
	sortOrder := cur.SortOrder
	if in.SortOrder != nil {
		sortOrder = *in.SortOrder
	}
	_, err = s.pool.Exec(ctx, `
		UPDATE sales.delivery_locations
		SET name=$1, description=NULLIF($2,''), fee=$3, active=$4, sort_order=$5, updated_at=now()
		WHERE tenant_id=$6 AND id=$7`,
		name, desc, fee, active, sortOrder, tenantID, id)
	if err != nil {
		return nil, err
	}
	return s.GetDeliveryLocation(ctx, tenantID, id)
}

func (s *Service) DeleteDeliveryLocation(ctx context.Context, tenantID, id uuid.UUID) error {
	ct, err := s.pool.Exec(ctx, `
		DELETE FROM sales.delivery_locations WHERE tenant_id=$1 AND id=$2`, tenantID, id)
	if err != nil {
		return err
	}
	if ct.RowsAffected() == 0 {
		return fmt.Errorf("delivery location not found")
	}
	return nil
}
