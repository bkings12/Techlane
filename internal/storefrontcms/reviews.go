package storefrontcms

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// Review is a verified-purchase product review left by a customer account
// (internal/commerce's repair.customers-backed session), not a staff user.
type Review struct {
	ID        uuid.UUID `json:"id"`
	ProductID uuid.UUID `json:"product_id"`
	Rating    int       `json:"rating"`
	Title     string    `json:"title,omitempty"`
	Body      string    `json:"body,omitempty"`
	Status    string    `json:"status"`
	CreatedAt time.Time `json:"created_at"`

	// Populated for admin moderation lists only.
	ProductName  string `json:"product_name,omitempty"`
	CustomerName string `json:"customer_name,omitempty"`
}

type RatingSummary struct {
	Average float64 `json:"average"`
	Count   int     `json:"count"`
}

// CreateOrUpdateReview enforces the verified-purchase gate itself: a
// customer may only review a product they actually received (a 'delivered'
// sales.orders row containing a variant of that product).
func (s *Service) CreateOrUpdateReview(ctx context.Context, tenantID, customerID, productID uuid.UUID, rating int, title, body string) (*Review, error) {
	if rating < 1 || rating > 5 {
		return nil, errors.New("rating must be between 1 and 5")
	}
	var orderID uuid.UUID
	err := s.pool.QueryRow(ctx, `
		SELECT o.id FROM sales.orders o
		JOIN sales.order_items oi ON oi.order_id = o.id AND oi.tenant_id = o.tenant_id
		JOIN inventory.product_variants v ON v.id = oi.variant_id AND v.tenant_id = o.tenant_id
		WHERE o.tenant_id = $1 AND o.customer_id = $2 AND v.product_id = $3 AND o.status = 'delivered'
		ORDER BY o.created_at DESC
		LIMIT 1`, tenantID, customerID, productID).Scan(&orderID)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, errors.New("you can review this product once your order has been collected")
	}
	if err != nil {
		return nil, err
	}

	id := uuid.New()
	_, err = s.pool.Exec(ctx, `
		INSERT INTO platform.product_reviews (id, tenant_id, product_id, customer_id, order_id, rating, title, body)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8)
		ON CONFLICT (tenant_id, customer_id, product_id) DO UPDATE SET
			order_id = EXCLUDED.order_id, rating = EXCLUDED.rating,
			title = EXCLUDED.title, body = EXCLUDED.body, updated_at = now()`,
		id, tenantID, productID, customerID, orderID, rating, nullIfBlank(title), nullIfBlank(body))
	if err != nil {
		return nil, err
	}
	return s.getReviewByCustomer(ctx, tenantID, customerID, productID)
}

func (s *Service) getReviewByCustomer(ctx context.Context, tenantID, customerID, productID uuid.UUID) (*Review, error) {
	var rv Review
	var title, bodyText *string
	err := s.pool.QueryRow(ctx, `
		SELECT id, product_id, rating, title, body, status, created_at
		FROM platform.product_reviews WHERE tenant_id = $1 AND customer_id = $2 AND product_id = $3`,
		tenantID, customerID, productID).
		Scan(&rv.ID, &rv.ProductID, &rv.Rating, &title, &bodyText, &rv.Status, &rv.CreatedAt)
	if err != nil {
		return nil, err
	}
	rv.Title = deref(title)
	rv.Body = deref(bodyText)
	return &rv, nil
}

// ListProductReviews returns published reviews for a product, newest first.
func (s *Service) ListProductReviews(ctx context.Context, tenantID, productID uuid.UUID) ([]Review, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, product_id, rating, title, body, status, created_at
		FROM platform.product_reviews
		WHERE tenant_id = $1 AND product_id = $2 AND status = 'published'
		ORDER BY created_at DESC`, tenantID, productID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]Review, 0)
	for rows.Next() {
		var rv Review
		var title, bodyText *string
		if err := rows.Scan(&rv.ID, &rv.ProductID, &rv.Rating, &title, &bodyText, &rv.Status, &rv.CreatedAt); err != nil {
			return nil, err
		}
		rv.Title = deref(title)
		rv.Body = deref(bodyText)
		items = append(items, rv)
	}
	return items, rows.Err()
}

// ProductRatingSummaries batches the average/count lookup so overlaying
// ratings onto a catalog page costs one query, not one per item.
func (s *Service) ProductRatingSummaries(ctx context.Context, tenantID uuid.UUID, productIDs []uuid.UUID) (map[uuid.UUID]RatingSummary, error) {
	out := map[uuid.UUID]RatingSummary{}
	if len(productIDs) == 0 {
		return out, nil
	}
	rows, err := s.pool.Query(ctx, `
		SELECT product_id, AVG(rating)::float8, COUNT(*)
		FROM platform.product_reviews
		WHERE tenant_id = $1 AND status = 'published' AND product_id = ANY($2)
		GROUP BY product_id`, tenantID, productIDs)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var pid uuid.UUID
		var sum RatingSummary
		if err := rows.Scan(&pid, &sum.Average, &sum.Count); err != nil {
			return nil, err
		}
		out[pid] = sum
	}
	return out, rows.Err()
}

// ListAllReviews is the admin moderation feed — every status, newest first.
func (s *Service) ListAllReviews(ctx context.Context, tenantID uuid.UUID, status string) ([]Review, error) {
	q := `
		SELECT r.id, r.product_id, r.rating, r.title, r.body, r.status, r.created_at,
		       p.name, c.full_name
		FROM platform.product_reviews r
		JOIN inventory.products p ON p.id = r.product_id
		JOIN repair.customers c ON c.id = r.customer_id
		WHERE r.tenant_id = $1`
	args := []any{tenantID}
	if status != "" {
		q += ` AND r.status = $2`
		args = append(args, status)
	}
	q += ` ORDER BY r.created_at DESC LIMIT 500`
	rows, err := s.pool.Query(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]Review, 0)
	for rows.Next() {
		var rv Review
		var title, bodyText *string
		if err := rows.Scan(&rv.ID, &rv.ProductID, &rv.Rating, &title, &bodyText, &rv.Status, &rv.CreatedAt, &rv.ProductName, &rv.CustomerName); err != nil {
			return nil, err
		}
		rv.Title = deref(title)
		rv.Body = deref(bodyText)
		items = append(items, rv)
	}
	return items, rows.Err()
}

func (s *Service) SetReviewStatus(ctx context.Context, tenantID, id uuid.UUID, status string) error {
	status = strings.TrimSpace(strings.ToLower(status))
	if status != "published" && status != "hidden" {
		return fmt.Errorf("status must be 'published' or 'hidden'")
	}
	tag, err := s.pool.Exec(ctx, `
		UPDATE platform.product_reviews SET status = $3, updated_at = now()
		WHERE tenant_id = $1 AND id = $2`, tenantID, id, status)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("review not found")
	}
	return nil
}
