package wifi

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Settings struct {
	TenantID             uuid.UUID  `json:"tenant_id"`
	Enabled              bool       `json:"enabled"`
	APIBaseURL           string     `json:"api_base_url"`
	APIKeySet            bool       `json:"api_key_set"`
	SiteID               *uuid.UUID `json:"site_id"`
	PackageID            *uuid.UUID `json:"package_id"`
	DefaultDurationMins  int        `json:"default_duration_mins"`
	Configured           bool       `json:"configured"`
	UpdatedAt            time.Time  `json:"updated_at"`
}

type UpsertSettingsInput struct {
	Enabled             *bool
	APIBaseURL          *string
	APIKey              *string // empty/nil = keep existing
	SiteID              *uuid.UUID
	ClearSiteID         bool
	PackageID           *uuid.UUID
	ClearPackageID      bool
	DefaultDurationMins *int
}

type Service struct {
	pool   *pgxpool.Pool
	client *Client
}

func NewService(pool *pgxpool.Pool) *Service {
	return &Service{pool: pool, client: NewClient()}
}

func (s *Service) GetSettings(ctx context.Context, tenantID uuid.UUID) (*Settings, error) {
	var (
		enabled bool
		base    string
		apiKey  string
		siteID  *uuid.UUID
		pkgID   *uuid.UUID
		dur     int
		updated time.Time
	)
	err := s.pool.QueryRow(ctx, `
		SELECT enabled, api_base_url, api_key, site_id, package_id, default_duration_mins, updated_at
		FROM platform.bytepesa_wifi_settings WHERE tenant_id = $1`, tenantID).
		Scan(&enabled, &base, &apiKey, &siteID, &pkgID, &dur, &updated)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return &Settings{
				TenantID:            tenantID,
				APIBaseURL:          "https://api.bytepesa.co.ke",
				DefaultDurationMins: 60,
				UpdatedAt:           time.Now().UTC(),
			}, nil
		}
		return nil, err
	}
	if strings.TrimSpace(base) == "" {
		base = "https://api.bytepesa.co.ke"
	}
	if dur <= 0 {
		dur = 60
	}
	out := &Settings{
		TenantID:            tenantID,
		Enabled:             enabled,
		APIBaseURL:          base,
		APIKeySet:           strings.TrimSpace(apiKey) != "",
		SiteID:              siteID,
		PackageID:           pkgID,
		DefaultDurationMins: dur,
		UpdatedAt:           updated,
	}
	out.Configured = out.Enabled && out.APIKeySet && out.SiteID != nil
	return out, nil
}

func (s *Service) UpsertSettings(ctx context.Context, tenantID uuid.UUID, in UpsertSettingsInput) (*Settings, error) {
	cur, err := s.loadRaw(ctx, tenantID)
	if err != nil {
		return nil, err
	}
	if in.Enabled != nil {
		cur.Enabled = *in.Enabled
	}
	if in.APIBaseURL != nil {
		cur.APIBaseURL = strings.TrimRight(strings.TrimSpace(*in.APIBaseURL), "/")
	}
	if in.APIKey != nil && strings.TrimSpace(*in.APIKey) != "" {
		cur.APIKey = strings.TrimSpace(*in.APIKey)
	}
	if in.ClearSiteID {
		cur.SiteID = nil
	} else if in.SiteID != nil {
		cur.SiteID = in.SiteID
	}
	if in.ClearPackageID {
		cur.PackageID = nil
	} else if in.PackageID != nil {
		cur.PackageID = in.PackageID
	}
	if in.DefaultDurationMins != nil && *in.DefaultDurationMins > 0 {
		cur.DefaultDurationMins = *in.DefaultDurationMins
	}
	if cur.APIBaseURL == "" {
		cur.APIBaseURL = "https://api.bytepesa.co.ke"
	}
	if cur.DefaultDurationMins <= 0 {
		cur.DefaultDurationMins = 60
	}

	_, err = s.pool.Exec(ctx, `
		INSERT INTO platform.bytepesa_wifi_settings (
			tenant_id, enabled, api_base_url, api_key, site_id, package_id, default_duration_mins, updated_at
		) VALUES ($1,$2,$3,$4,$5,$6,$7, now())
		ON CONFLICT (tenant_id) DO UPDATE SET
			enabled = EXCLUDED.enabled,
			api_base_url = EXCLUDED.api_base_url,
			api_key = CASE WHEN EXCLUDED.api_key <> '' THEN EXCLUDED.api_key ELSE platform.bytepesa_wifi_settings.api_key END,
			site_id = EXCLUDED.site_id,
			package_id = EXCLUDED.package_id,
			default_duration_mins = EXCLUDED.default_duration_mins,
			updated_at = now()`,
		tenantID, cur.Enabled, cur.APIBaseURL, cur.APIKey, cur.SiteID, cur.PackageID, cur.DefaultDurationMins)
	if err != nil {
		return nil, err
	}
	return s.GetSettings(ctx, tenantID)
}

type rawSettings struct {
	Enabled             bool
	APIBaseURL          string
	APIKey              string
	SiteID              *uuid.UUID
	PackageID           *uuid.UUID
	DefaultDurationMins int
}

func (s *Service) loadRaw(ctx context.Context, tenantID uuid.UUID) (rawSettings, error) {
	var row rawSettings
	err := s.pool.QueryRow(ctx, `
		SELECT enabled, api_base_url, api_key, site_id, package_id, default_duration_mins
		FROM platform.bytepesa_wifi_settings WHERE tenant_id = $1`, tenantID).
		Scan(&row.Enabled, &row.APIBaseURL, &row.APIKey, &row.SiteID, &row.PackageID, &row.DefaultDurationMins)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return rawSettings{
				APIBaseURL:          "https://api.bytepesa.co.ke",
				DefaultDurationMins: 60,
			}, nil
		}
		return rawSettings{}, err
	}
	return row, nil
}

type IssueInput struct {
	DurationMins int
	Phone        string
	RepairID     *uuid.UUID
	SaleID       *uuid.UUID
	Reference    string
}

type StoredVoucher struct {
	ID           uuid.UUID `json:"id"`
	Code         string    `json:"code"`
	RedeemURL    string    `json:"redeem_url"`
	QRPayload    string    `json:"qr_payload"`
	DurationMins int       `json:"duration_mins"`
	ExpiresAt    *string   `json:"expires_at"`
	PackageName  string    `json:"package_name"`
	Reference    string    `json:"reference"`
}

func (s *Service) Issue(ctx context.Context, tenantID uuid.UUID, in IssueInput) (*StoredVoucher, error) {
	raw, err := s.loadRaw(ctx, tenantID)
	if err != nil {
		return nil, err
	}
	if !raw.Enabled || strings.TrimSpace(raw.APIKey) == "" || raw.SiteID == nil {
		return nil, errors.New("Guest WiFi is not configured — enable it under Settings → Guest WiFi")
	}
	dur := in.DurationMins
	if dur <= 0 {
		dur = raw.DefaultDurationMins
	}
	if dur <= 0 {
		dur = 60
	}
	req := IssueRequest{
		SiteID:       raw.SiteID.String(),
		DurationMins: dur,
		Quantity:     1,
		Phone:        strings.TrimSpace(in.Phone),
		Reference:    strings.TrimSpace(in.Reference),
	}
	if raw.PackageID != nil {
		req.PackageID = raw.PackageID.String()
	}
	res, err := s.client.IssueVoucher(ctx, raw.APIBaseURL, raw.APIKey, req)
	if err != nil {
		return nil, err
	}
	if len(res.Vouchers) == 0 {
		return nil, errors.New("BytePesa returned no vouchers")
	}
	v := res.Vouchers[0]
	id := uuid.New()
	var expires *string
	if strings.TrimSpace(v.ExpiresAt) != "" {
		e := v.ExpiresAt
		expires = &e
	}
	_, err = s.pool.Exec(ctx, `
		INSERT INTO platform.bytepesa_wifi_vouchers (
			id, tenant_id, code, redeem_url, qr_payload, duration_mins, expires_at, package_name, repair_id, sale_id, reference
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)`,
		id, tenantID, v.Code, v.RedeemURL, v.QRPayload, v.DurationMins, expires, v.PackageName, in.RepairID, in.SaleID, nullIfEmpty(in.Reference))
	if err != nil {
		return nil, err
	}
	return &StoredVoucher{
		ID:           id,
		Code:         v.Code,
		RedeemURL:    v.RedeemURL,
		QRPayload:    v.QRPayload,
		DurationMins: v.DurationMins,
		ExpiresAt:    expires,
		PackageName:  v.PackageName,
		Reference:    in.Reference,
	}, nil
}

func (s *Service) GetVoucher(ctx context.Context, tenantID, id uuid.UUID) (*StoredVoucher, error) {
	var out StoredVoucher
	var expires *time.Time
	err := s.pool.QueryRow(ctx, `
		SELECT id, code, redeem_url, qr_payload, duration_mins, expires_at, COALESCE(package_name,''), COALESCE(reference,'')
		FROM platform.bytepesa_wifi_vouchers WHERE id = $1 AND tenant_id = $2`, id, tenantID).
		Scan(&out.ID, &out.Code, &out.RedeemURL, &out.QRPayload, &out.DurationMins, &expires, &out.PackageName, &out.Reference)
	if err != nil {
		return nil, err
	}
	if expires != nil {
		e := expires.UTC().Format(time.RFC3339)
		out.ExpiresAt = &e
	}
	return &out, nil
}

func nullIfEmpty(s string) any {
	if strings.TrimSpace(s) == "" {
		return nil
	}
	return strings.TrimSpace(s)
}
