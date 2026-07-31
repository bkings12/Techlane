package audit

import (
	"context"
	"time"

	"github.com/google/uuid"
)

// ErrorEvent is a captured panic or 5xx response — a self-hosted, minimal
// stand-in for an external error tracker (Sentry/Bugsnag) until one is wired.
type ErrorEvent struct {
	ID            uuid.UUID  `json:"id"`
	TenantID      *uuid.UUID `json:"tenant_id,omitempty"`
	Method        string     `json:"method"`
	Route         string     `json:"route"`
	Status        int        `json:"status"`
	Message       string     `json:"message"`
	Stack         string     `json:"stack,omitempty"`
	CorrelationID string     `json:"correlation_id,omitempty"`
	CreatedAt     time.Time  `json:"created_at"`
}

// RecordError implements httpx.ErrorSink so the HTTP layer can stay decoupled
// from persistence details.
func (s *Service) RecordError(ctx context.Context, method, route string, status int, message, stack, corrID string) error {
	id := uuid.New()
	_, err := s.pool.Exec(ctx, `
		INSERT INTO audit.error_events (id, method, route, status, message, stack, correlation_id)
		VALUES ($1, $2, $3, $4, $5, $6, $7)`,
		id, method, route, status, truncate(message, 2000), truncate(stack, 8000), corrID)
	return err
}

func (s *Service) ListErrorEvents(ctx context.Context, limit int) ([]ErrorEvent, error) {
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	rows, err := s.pool.Query(ctx, `
		SELECT id, tenant_id, method, route, status, message, stack, correlation_id, created_at
		FROM audit.error_events ORDER BY created_at DESC LIMIT $1`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]ErrorEvent, 0)
	for rows.Next() {
		var e ErrorEvent
		var corrID *string
		if err := rows.Scan(&e.ID, &e.TenantID, &e.Method, &e.Route, &e.Status, &e.Message, &e.Stack, &corrID, &e.CreatedAt); err != nil {
			return nil, err
		}
		if corrID != nil {
			e.CorrelationID = *corrID
		}
		items = append(items, e)
	}
	return items, rows.Err()
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}
