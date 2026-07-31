package notify

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// SMSTemplateView is returned to the owner settings UI.
type SMSTemplateView struct {
	Key          string   `json:"key"`
	Label        string   `json:"label"`
	Description  string   `json:"description"`
	Audience     string   `json:"audience"`
	Helpers      []string `json:"helpers"`
	DefaultBody  string   `json:"default_body"`
	Body         string   `json:"body"`
	IsCustomized bool     `json:"is_customized"`
	UpdatedAt    *time.Time `json:"updated_at,omitempty"`
}

func (s *Service) loadTemplateBody(ctx context.Context, tenantID uuid.UUID, key string) (string, error) {
	var body string
	err := s.pool.QueryRow(ctx, `
		SELECT body FROM notify.sms_templates WHERE tenant_id = $1 AND template_key = $2`,
		tenantID, key).Scan(&body)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", nil
	}
	return body, err
}

func (s *Service) ListSMSTemplates(ctx context.Context, tenantID uuid.UUID) ([]SMSTemplateView, error) {
	custom := map[string]struct {
		Body      string
		UpdatedAt time.Time
	}{}
	rows, err := s.pool.Query(ctx, `
		SELECT template_key, body, updated_at FROM notify.sms_templates WHERE tenant_id = $1`, tenantID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var key, body string
		var updated time.Time
		if err := rows.Scan(&key, &body, &updated); err != nil {
			return nil, err
		}
		custom[key] = struct {
			Body      string
			UpdatedAt time.Time
		}{Body: body, UpdatedAt: updated}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	defs := DefaultTemplateDefs()
	out := make([]SMSTemplateView, 0, len(defs))
	for _, d := range defs {
		view := SMSTemplateView{
			Key:         d.Key,
			Label:       d.Label,
			Description: d.Description,
			Audience:    d.Audience,
			Helpers:     d.Helpers,
			DefaultBody: d.DefaultBody,
			Body:        d.DefaultBody,
		}
		if c, ok := custom[d.Key]; ok && strings.TrimSpace(c.Body) != "" {
			view.Body = c.Body
			view.IsCustomized = true
			t := c.UpdatedAt
			view.UpdatedAt = &t
		}
		out = append(out, view)
	}
	return out, nil
}

func (s *Service) UpsertSMSTemplate(ctx context.Context, tenantID uuid.UUID, key, body string, actorID uuid.UUID) (*SMSTemplateView, error) {
	key = strings.TrimSpace(key)
	if _, ok := defaultBodyFor(key); !ok {
		return nil, fmt.Errorf("unknown template %q", key)
	}
	body = strings.TrimSpace(body)
	if body == "" {
		// Reset to default — delete override.
		_, _ = s.pool.Exec(ctx, `
			DELETE FROM notify.sms_templates WHERE tenant_id = $1 AND template_key = $2`, tenantID, key)
	} else {
		_, err := s.pool.Exec(ctx, `
			INSERT INTO notify.sms_templates (tenant_id, template_key, body, updated_at, updated_by)
			VALUES ($1, $2, $3, now(), $4)
			ON CONFLICT (tenant_id, template_key) DO UPDATE SET
				body = EXCLUDED.body,
				updated_at = now(),
				updated_by = EXCLUDED.updated_by`,
			tenantID, key, body, actorID)
		if err != nil {
			return nil, err
		}
	}
	items, err := s.ListSMSTemplates(ctx, tenantID)
	if err != nil {
		return nil, err
	}
	for i := range items {
		if items[i].Key == key {
			return &items[i], nil
		}
	}
	return nil, fmt.Errorf("template %q not found after save", key)
}
