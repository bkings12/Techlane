package payments

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// ProviderSettings stores Daraja/M-Pesa credentials used for STK/C2B and bank paybill.
type ProviderSettings struct {
	TenantID            uuid.UUID `json:"tenant_id"`
	Environment         string    `json:"environment"`
	MpesaEnabled        bool      `json:"mpesa_enabled"`
	MpesaShortcode      string    `json:"mpesa_shortcode"`
	MpesaConsumerKey    string    `json:"mpesa_consumer_key"`
	MpesaCallbackURL    string    `json:"mpesa_callback_url"`
	ConsumerSecretSet   bool      `json:"consumer_secret_set"`
	PasskeySet          bool      `json:"passkey_set"`
	BankEnabled         bool      `json:"bank_enabled"`
	BankPaybill         string    `json:"bank_paybill"`
	BankAccount         string    `json:"bank_account"`
	Configured          bool      `json:"configured"`
	BankConfigured      bool      `json:"bank_configured"`
	UpdatedAt           time.Time `json:"updated_at"`
}

type UpsertProviderSettingsInput struct {
	Environment        string
	MpesaEnabled       *bool
	MpesaShortcode     *string
	MpesaConsumerKey   *string
	MpesaConsumerSecret *string // nil/empty = keep existing
	MpesaPasskey       *string  // nil/empty = keep existing
	MpesaCallbackURL   *string
	BankEnabled        *bool
	BankPaybill        *string
	BankAccount        *string
	ActorID            uuid.UUID
}

func maskSecret(s string) bool {
	return strings.TrimSpace(s) != ""
}

func (s *Service) GetProviderSettings(ctx context.Context, tenantID uuid.UUID) (*ProviderSettings, error) {
	var row struct {
		Environment      string
		MpesaEnabled     bool
		Shortcode        string
		ConsumerKey      string
		ConsumerSecret   string
		Passkey          string
		CallbackURL      string
		BankEnabled      bool
		BankPaybill      string
		BankAccount      string
		UpdatedAt        time.Time
	}
	err := s.pool.QueryRow(ctx, `
		SELECT environment, mpesa_enabled, mpesa_shortcode, mpesa_consumer_key, mpesa_consumer_secret,
		       mpesa_passkey, mpesa_callback_url, bank_enabled, bank_paybill, bank_account, updated_at
		FROM payments.provider_settings WHERE tenant_id = $1`, tenantID).
		Scan(&row.Environment, &row.MpesaEnabled, &row.Shortcode, &row.ConsumerKey, &row.ConsumerSecret,
			&row.Passkey, &row.CallbackURL, &row.BankEnabled, &row.BankPaybill, &row.BankAccount, &row.UpdatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return &ProviderSettings{
				TenantID:    tenantID,
				Environment: "sandbox",
				UpdatedAt:   time.Now().UTC(),
			}, nil
		}
		return nil, err
	}
	out := &ProviderSettings{
		TenantID:          tenantID,
		Environment:       row.Environment,
		MpesaEnabled:      row.MpesaEnabled,
		MpesaShortcode:    row.Shortcode,
		MpesaConsumerKey:  row.ConsumerKey,
		MpesaCallbackURL:  row.CallbackURL,
		ConsumerSecretSet: maskSecret(row.ConsumerSecret),
		PasskeySet:        maskSecret(row.Passkey),
		BankEnabled:       row.BankEnabled,
		BankPaybill:       row.BankPaybill,
		BankAccount:       row.BankAccount,
		UpdatedAt:         row.UpdatedAt,
	}
	out.Configured = row.MpesaEnabled && row.Shortcode != "" && row.ConsumerKey != "" && maskSecret(row.ConsumerSecret) && maskSecret(row.Passkey)
	out.BankConfigured = out.Configured && row.BankEnabled && row.BankPaybill != "" && row.BankAccount != ""
	return out, nil
}

func (s *Service) UpsertProviderSettings(ctx context.Context, tenantID uuid.UUID, in UpsertProviderSettingsInput) (*ProviderSettings, error) {
	cur, err := s.loadRawSettings(ctx, tenantID)
	if err != nil {
		return nil, err
	}

	env := cur.Environment
	if in.Environment != "" {
		env = in.Environment
	}
	if env != "sandbox" && env != "production" {
		return nil, fmt.Errorf("environment must be sandbox or production")
	}

	mpesaEnabled := cur.MpesaEnabled
	if in.MpesaEnabled != nil {
		mpesaEnabled = *in.MpesaEnabled
	}
	shortcode := cur.Shortcode
	if in.MpesaShortcode != nil {
		shortcode = strings.TrimSpace(*in.MpesaShortcode)
	}
	consumerKey := cur.ConsumerKey
	if in.MpesaConsumerKey != nil {
		consumerKey = strings.TrimSpace(*in.MpesaConsumerKey)
	}
	consumerSecret := cur.ConsumerSecret
	if in.MpesaConsumerSecret != nil && strings.TrimSpace(*in.MpesaConsumerSecret) != "" {
		consumerSecret = strings.TrimSpace(*in.MpesaConsumerSecret)
	}
	passkey := cur.Passkey
	if in.MpesaPasskey != nil && strings.TrimSpace(*in.MpesaPasskey) != "" {
		passkey = strings.TrimSpace(*in.MpesaPasskey)
	}
	callback := cur.CallbackURL
	if in.MpesaCallbackURL != nil {
		callback = strings.TrimSpace(*in.MpesaCallbackURL)
	}
	if callback != "" {
		if err := validateCallbackURL(callback, env); err != nil {
			return nil, err
		}
	} else if env == "production" && mpesaEnabled {
		if def, err := defaultCallbackURL(); err == nil {
			callback = def
		} else {
			return nil, fmt.Errorf("mpesa_callback_url required in production (or set PUBLIC_API_BASE)")
		}
	}
	bankEnabled := cur.BankEnabled
	if in.BankEnabled != nil {
		bankEnabled = *in.BankEnabled
	}
	bankPaybill := cur.BankPaybill
	if in.BankPaybill != nil {
		bankPaybill = strings.TrimSpace(*in.BankPaybill)
	}
	bankAccount := cur.BankAccount
	if in.BankAccount != nil {
		bankAccount = strings.TrimSpace(*in.BankAccount)
	}

	if bankEnabled && (bankPaybill == "" || bankAccount == "") {
		return nil, fmt.Errorf("bank_paybill and bank_account required when bank payments are enabled")
	}
	if bankEnabled && !mpesaEnabled {
		return nil, fmt.Errorf("enable M-Pesa credentials first — bank paybill uses the same Daraja credentials")
	}

	_, err = s.pool.Exec(ctx, `
		INSERT INTO payments.provider_settings (
			tenant_id, environment, mpesa_enabled, mpesa_shortcode, mpesa_consumer_key, mpesa_consumer_secret,
			mpesa_passkey, mpesa_callback_url, bank_enabled, bank_paybill, bank_account, updated_at, updated_by
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11, now(), $12)
		ON CONFLICT (tenant_id) DO UPDATE SET
			environment = EXCLUDED.environment,
			mpesa_enabled = EXCLUDED.mpesa_enabled,
			mpesa_shortcode = EXCLUDED.mpesa_shortcode,
			mpesa_consumer_key = EXCLUDED.mpesa_consumer_key,
			mpesa_consumer_secret = EXCLUDED.mpesa_consumer_secret,
			mpesa_passkey = EXCLUDED.mpesa_passkey,
			mpesa_callback_url = EXCLUDED.mpesa_callback_url,
			bank_enabled = EXCLUDED.bank_enabled,
			bank_paybill = EXCLUDED.bank_paybill,
			bank_account = EXCLUDED.bank_account,
			updated_at = now(),
			updated_by = EXCLUDED.updated_by`,
		tenantID, env, mpesaEnabled, shortcode, consumerKey, consumerSecret,
		passkey, callback, bankEnabled, bankPaybill, bankAccount, in.ActorID)
	if err != nil {
		return nil, err
	}
	return s.GetProviderSettings(ctx, tenantID)
}

type rawSettings struct {
	Environment    string
	MpesaEnabled   bool
	Shortcode      string
	ConsumerKey    string
	ConsumerSecret string
	Passkey        string
	CallbackURL    string
	BankEnabled    bool
	BankPaybill    string
	BankAccount    string
}

func (s *Service) loadRawSettings(ctx context.Context, tenantID uuid.UUID) (rawSettings, error) {
	var row rawSettings
	err := s.pool.QueryRow(ctx, `
		SELECT environment, mpesa_enabled, mpesa_shortcode, mpesa_consumer_key, mpesa_consumer_secret,
		       mpesa_passkey, mpesa_callback_url, bank_enabled, bank_paybill, bank_account
		FROM payments.provider_settings WHERE tenant_id = $1`, tenantID).
		Scan(&row.Environment, &row.MpesaEnabled, &row.Shortcode, &row.ConsumerKey, &row.ConsumerSecret,
			&row.Passkey, &row.CallbackURL, &row.BankEnabled, &row.BankPaybill, &row.BankAccount)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return rawSettings{Environment: "sandbox"}, nil
		}
		return rawSettings{}, err
	}
	return row, nil
}

// RequireDigitalReady checks credentials before initiating digital collection.
func (s *Service) RequireDigitalReady(ctx context.Context, tenantID uuid.UUID, method string) error {
	cfg, err := s.GetProviderSettings(ctx, tenantID)
	if err != nil {
		return err
	}
	switch method {
	case "mpesa_stk", "mpesa_c2b":
		if !cfg.Configured {
			return fmt.Errorf("M-Pesa credentials not configured — set them under Settings → Payments")
		}
	case "bank_paybill", "bank_transfer":
		if !cfg.BankConfigured {
			return fmt.Errorf("bank paybill not configured — set paybill and account under Settings → Payments")
		}
	case "store_credit":
		return nil
	case "card":
		return fmt.Errorf("card payments are not configured")
	}
	return nil
}

func validateCallbackURL(raw, env string) error {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || u.Scheme == "" || u.Host == "" {
		return fmt.Errorf("mpesa_callback_url must be an absolute URL")
	}
	if env == "production" && u.Scheme != "https" {
		return fmt.Errorf("mpesa_callback_url must use https in production")
	}
	if strings.Contains(strings.ToLower(u.Host), "example.com") {
		return fmt.Errorf("mpesa_callback_url must not use example.com")
	}
	return nil
}

func defaultCallbackURL() (string, error) {
	base := strings.TrimRight(strings.TrimSpace(os.Getenv("PUBLIC_API_BASE")), "/")
	if base == "" {
		return "", fmt.Errorf("PUBLIC_API_BASE not set")
	}
	u := base + "/api/v1/webhooks/mpesa/stk"
	if token := strings.TrimSpace(os.Getenv("MPESA_WEBHOOK_TOKEN")); token != "" {
		u += "?token=" + url.QueryEscape(token)
	}
	return u, nil
}

func (s *Service) resolveSTKCallbackURL(raw rawSettings) (string, error) {
	callback := strings.TrimSpace(raw.CallbackURL)
	if callback == "" {
		def, err := defaultCallbackURL()
		if err != nil {
			if raw.Environment == "production" {
				return "", fmt.Errorf("mpesa callback URL not configured (set Settings → Payments or PUBLIC_API_BASE)")
			}
			return "", fmt.Errorf("mpesa callback URL not configured")
		}
		callback = def
	}
	if err := validateCallbackURL(callback, raw.Environment); err != nil {
		return "", err
	}
	return callback, nil
}
