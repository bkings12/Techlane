package repair

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// SMSSettings is the owner-facing SMS provider configuration (secrets masked).
type SMSSettings struct {
	TenantID   uuid.UUID `json:"tenant_id"`
	Provider   string    `json:"provider"`
	Enabled    bool      `json:"enabled"`
	SenderID   string    `json:"sender_id"`
	BaseURL    string    `json:"base_url"`
	APIKeySet  bool      `json:"api_key_set"`
	Configured bool      `json:"configured"`
	UpdatedAt  time.Time `json:"updated_at"`
}

type UpsertSMSSettingsInput struct {
	Enabled  *bool
	Provider *string
	APIKey   *string // empty/nil = keep existing
	SenderID *string
	BaseURL  *string
	ActorID  uuid.UUID
}

type rawSMSSettings struct {
	Provider string
	Enabled  bool
	APIKey   string
	SenderID string
	BaseURL  string
}

func (s *Service) GetSMSSettings(ctx context.Context, tenantID uuid.UUID) (*SMSSettings, error) {
	row, err := s.loadRawSMSSettings(ctx, tenantID)
	if err != nil {
		return nil, err
	}
	out := &SMSSettings{
		TenantID:  tenantID,
		Provider:  row.Provider,
		Enabled:   row.Enabled,
		SenderID:  row.SenderID,
		BaseURL:   row.BaseURL,
		APIKeySet: strings.TrimSpace(row.APIKey) != "",
		UpdatedAt: time.Now().UTC(),
	}
	_ = s.pool.QueryRow(ctx, `SELECT updated_at FROM repair.sms_settings WHERE tenant_id = $1`, tenantID).Scan(&out.UpdatedAt)
	out.Configured = out.Enabled && out.APIKeySet && strings.TrimSpace(out.SenderID) != ""
	return out, nil
}

func (s *Service) loadRawSMSSettings(ctx context.Context, tenantID uuid.UUID) (rawSMSSettings, error) {
	var row rawSMSSettings
	err := s.pool.QueryRow(ctx, `
		SELECT provider, enabled, api_key, sender_id, base_url
		FROM repair.sms_settings WHERE tenant_id = $1`, tenantID).
		Scan(&row.Provider, &row.Enabled, &row.APIKey, &row.SenderID, &row.BaseURL)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return rawSMSSettings{
				Provider: "blessedtexts",
				BaseURL:  defaultBlessedTextsBaseURL,
			}, nil
		}
		return rawSMSSettings{}, err
	}
	if strings.TrimSpace(row.BaseURL) == "" {
		row.BaseURL = defaultBlessedTextsBaseURL
	}
	if strings.TrimSpace(row.Provider) == "" {
		row.Provider = "blessedtexts"
	}
	return row, nil
}

func (s *Service) UpsertSMSSettings(ctx context.Context, tenantID uuid.UUID, in UpsertSMSSettingsInput) (*SMSSettings, error) {
	cur, err := s.loadRawSMSSettings(ctx, tenantID)
	if err != nil {
		return nil, err
	}

	enabled := cur.Enabled
	if in.Enabled != nil {
		enabled = *in.Enabled
	}
	provider := cur.Provider
	if in.Provider != nil && strings.TrimSpace(*in.Provider) != "" {
		provider = strings.TrimSpace(*in.Provider)
	}
	if provider != "blessedtexts" {
		return nil, fmt.Errorf("unsupported SMS provider %q", provider)
	}
	apiKey := cur.APIKey
	if in.APIKey != nil && strings.TrimSpace(*in.APIKey) != "" {
		apiKey = strings.TrimSpace(*in.APIKey)
	}
	senderID := cur.SenderID
	if in.SenderID != nil {
		senderID = strings.TrimSpace(*in.SenderID)
	}
	baseURL := cur.BaseURL
	if in.BaseURL != nil {
		baseURL = strings.TrimRight(strings.TrimSpace(*in.BaseURL), "/")
		if baseURL == "" {
			baseURL = defaultBlessedTextsBaseURL
		}
	}
	if enabled && (apiKey == "" || senderID == "") {
		return nil, fmt.Errorf("api_key and sender_id are required when SMS is enabled")
	}

	_, err = s.pool.Exec(ctx, `
		INSERT INTO repair.sms_settings (
			tenant_id, provider, enabled, api_key, sender_id, base_url, updated_at, updated_by
		) VALUES ($1,$2,$3,$4,$5,$6, now(), $7)
		ON CONFLICT (tenant_id) DO UPDATE SET
			provider = EXCLUDED.provider,
			enabled = EXCLUDED.enabled,
			api_key = EXCLUDED.api_key,
			sender_id = EXCLUDED.sender_id,
			base_url = EXCLUDED.base_url,
			updated_at = now(),
			updated_by = EXCLUDED.updated_by`,
		tenantID, provider, enabled, apiKey, senderID, baseURL, in.ActorID)
	if err != nil {
		return nil, err
	}
	return s.GetSMSSettings(ctx, tenantID)
}

// ResolveSMSSender prefers tenant owner settings, then the process-level sender (env/dev).
func (s *Service) ResolveSMSSender(ctx context.Context, tenantID uuid.UUID) SMSSender {
	return s.resolveSMSSender(ctx, tenantID)
}

// resolveSMSSender prefers tenant owner settings, then the process-level sender (env/dev).
func (s *Service) resolveSMSSender(ctx context.Context, tenantID uuid.UUID) SMSSender {
	row, err := s.loadRawSMSSettings(ctx, tenantID)
	if err == nil && row.Enabled && strings.TrimSpace(row.APIKey) != "" && strings.TrimSpace(row.SenderID) != "" {
		return NewBlessedTextsSMSSender(row.APIKey, row.SenderID, row.BaseURL)
	}
	if s.sms != nil {
		return s.sms
	}
	return NoopSMSSender{}
}
