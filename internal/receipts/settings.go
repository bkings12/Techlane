// Package receipts owns the printable documents customers walk away with:
// repair receipts, POS sale receipts, tax invoices and supplier vouchers.
// Layout and per-tenant branding live here so every document looks the same.
package receipts

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Paper sizes a receipt can be rendered onto.
const (
	PaperThermal80 = "thermal80"
	PaperThermal58 = "thermal58"
	PaperA4        = "a4"
)

const (
	maxLogoBytes = 512 * 1024
	logoCacheTTL = 5 * time.Minute
)

// Settings is the per-tenant receipt configuration the owner edits.
type Settings struct {
	TenantID uuid.UUID `json:"tenant_id"`

	HeaderNote  string `json:"header_note"`
	Phone       string `json:"phone"`
	Email       string `json:"email"`
	Website     string `json:"website"`
	ShowLogo    bool   `json:"show_logo"`
	ShowAddress bool   `json:"show_address"`
	ShowTIN     bool   `json:"show_tin"`

	ThankYouText string `json:"thank_you_text"`
	FooterText   string `json:"footer_text"`
	WarrantyText string `json:"warranty_text"`

	ShowVATBreakdown bool `json:"show_vat_breakdown"`
	ShowIMEI         bool `json:"show_imei"`
	ShowPayments     bool `json:"show_payments"`
	ShowBalance      bool `json:"show_balance"`
	ShowServedBy     bool `json:"show_served_by"`

	DefaultPaper string `json:"default_paper"`

	NumberPrefix string `json:"number_prefix"`
	NextNumber   int64  `json:"next_number"`

	HasLogo         bool       `json:"has_logo"`
	LogoContentType string     `json:"logo_content_type,omitempty"`
	LogoUpdatedAt   *time.Time `json:"logo_updated_at,omitempty"`

	UpdatedAt time.Time `json:"updated_at"`
}

// ObjectStore is the slice of object storage receipts needs for logo files.
type ObjectStore interface {
	Put(ctx context.Context, key string, body []byte, contentType string) error
	Get(ctx context.Context, key string) ([]byte, error)
	Delete(ctx context.Context, key string) error
}

type logoCacheEntry struct {
	dataURI string
	loaded  time.Time
}

type Service struct {
	pool  *pgxpool.Pool
	store ObjectStore

	mu        sync.Mutex
	logoCache map[uuid.UUID]logoCacheEntry
}

func NewService(pool *pgxpool.Pool) *Service {
	return &Service{pool: pool, logoCache: map[uuid.UUID]logoCacheEntry{}}
}

func (s *Service) SetObjectStore(store ObjectStore) { s.store = store }

// DefaultSettings are what a shop gets before the owner touches anything.
func DefaultSettings(tenantID uuid.UUID) Settings {
	return Settings{
		TenantID:         tenantID,
		ShowLogo:         true,
		ShowAddress:      true,
		ShowTIN:          true,
		ThankYouText:     "Thank you for your business",
		WarrantyText:     "Keep this receipt — it is required for warranty claims, returns and device collection.",
		ShowVATBreakdown: true,
		ShowIMEI:         true,
		ShowPayments:     true,
		ShowBalance:      true,
		ShowServedBy:     true,
		DefaultPaper:     PaperThermal80,
		NumberPrefix:     "RCT-",
		NextNumber:       1,
		UpdatedAt:        time.Now().UTC(),
	}
}

// NormalizePaper coerces user input to a supported paper size.
func NormalizePaper(paper, fallback string) string {
	switch strings.ToLower(strings.TrimSpace(paper)) {
	case PaperThermal80, "80", "80mm":
		return PaperThermal80
	case PaperThermal58, "58", "58mm":
		return PaperThermal58
	case PaperA4, "letter":
		return PaperA4
	}
	if fallback != "" && fallback != paper {
		return NormalizePaper(fallback, "")
	}
	return PaperThermal80
}

func (s *Service) GetSettings(ctx context.Context, tenantID uuid.UUID) (Settings, error) {
	out := DefaultSettings(tenantID)
	var headerNote, phone, email, website *string
	var thankYou, footer, warranty, logoType *string
	var logoUpdated *time.Time
	var hasLogoKey, hasLogoBytes bool

	err := s.pool.QueryRow(ctx, `
		SELECT header_note, phone, email, website, show_logo, show_address, show_tin,
		       thank_you_text, footer_text, warranty_text,
		       show_vat_breakdown, show_imei, show_payments, show_balance, show_served_by,
		       default_paper, number_prefix, next_number,
		       logo_object_key IS NOT NULL, logo_bytes IS NOT NULL, logo_content_type, logo_updated_at,
		       updated_at
		FROM platform.receipt_settings WHERE tenant_id = $1`, tenantID).
		Scan(&headerNote, &phone, &email, &website, &out.ShowLogo, &out.ShowAddress, &out.ShowTIN,
			&thankYou, &footer, &warranty,
			&out.ShowVATBreakdown, &out.ShowIMEI, &out.ShowPayments, &out.ShowBalance, &out.ShowServedBy,
			&out.DefaultPaper, &out.NumberPrefix, &out.NextNumber,
			&hasLogoKey, &hasLogoBytes, &logoType, &logoUpdated,
			&out.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return out, nil
	}
	if err != nil {
		return out, err
	}

	out.HeaderNote = deref(headerNote)
	out.Phone = deref(phone)
	out.Email = deref(email)
	out.Website = deref(website)
	out.ThankYouText = deref(thankYou)
	out.FooterText = deref(footer)
	out.WarrantyText = deref(warranty)
	out.HasLogo = hasLogoKey || hasLogoBytes
	out.LogoContentType = deref(logoType)
	out.LogoUpdatedAt = logoUpdated
	out.DefaultPaper = NormalizePaper(out.DefaultPaper, PaperThermal80)
	return out, nil
}

// UpsertSettingsInput carries only the fields the caller wants changed.
type UpsertSettingsInput struct {
	HeaderNote  *string
	Phone       *string
	Email       *string
	Website     *string
	ShowLogo    *bool
	ShowAddress *bool
	ShowTIN     *bool

	ThankYouText *string
	FooterText   *string
	WarrantyText *string

	ShowVATBreakdown *bool
	ShowIMEI         *bool
	ShowPayments     *bool
	ShowBalance      *bool
	ShowServedBy     *bool

	DefaultPaper *string
	NumberPrefix *string
	NextNumber   *int64
}

func (s *Service) UpsertSettings(ctx context.Context, tenantID uuid.UUID, in UpsertSettingsInput, actorID uuid.UUID) (Settings, error) {
	cur, err := s.GetSettings(ctx, tenantID)
	if err != nil {
		return cur, err
	}

	applyString(&cur.HeaderNote, in.HeaderNote)
	applyString(&cur.Phone, in.Phone)
	applyString(&cur.Email, in.Email)
	applyString(&cur.Website, in.Website)
	applyBool(&cur.ShowLogo, in.ShowLogo)
	applyBool(&cur.ShowAddress, in.ShowAddress)
	applyBool(&cur.ShowTIN, in.ShowTIN)
	applyString(&cur.ThankYouText, in.ThankYouText)
	applyString(&cur.FooterText, in.FooterText)
	applyString(&cur.WarrantyText, in.WarrantyText)
	applyBool(&cur.ShowVATBreakdown, in.ShowVATBreakdown)
	applyBool(&cur.ShowIMEI, in.ShowIMEI)
	applyBool(&cur.ShowPayments, in.ShowPayments)
	applyBool(&cur.ShowBalance, in.ShowBalance)
	applyBool(&cur.ShowServedBy, in.ShowServedBy)

	if in.DefaultPaper != nil {
		cur.DefaultPaper = NormalizePaper(*in.DefaultPaper, cur.DefaultPaper)
	}
	if in.NumberPrefix != nil {
		prefix := strings.TrimSpace(*in.NumberPrefix)
		if len(prefix) > 12 {
			return cur, errors.New("number prefix must be 12 characters or fewer")
		}
		cur.NumberPrefix = prefix
	}
	if in.NextNumber != nil {
		if *in.NextNumber < 1 {
			return cur, errors.New("next receipt number must be 1 or greater")
		}
		cur.NextNumber = *in.NextNumber
	}

	if err := s.ensureRow(ctx, tenantID); err != nil {
		return cur, err
	}
	_, err = s.pool.Exec(ctx, `
		UPDATE platform.receipt_settings SET
			header_note = $2, phone = $3, email = $4, website = $5,
			show_logo = $6, show_address = $7, show_tin = $8,
			thank_you_text = $9, footer_text = $10, warranty_text = $11,
			show_vat_breakdown = $12, show_imei = $13, show_payments = $14,
			show_balance = $15, show_served_by = $16,
			default_paper = $17, number_prefix = $18, next_number = $19,
			updated_at = now(), updated_by = $20
		WHERE tenant_id = $1`,
		tenantID, nullIfBlank(cur.HeaderNote), nullIfBlank(cur.Phone), nullIfBlank(cur.Email), nullIfBlank(cur.Website),
		cur.ShowLogo, cur.ShowAddress, cur.ShowTIN,
		nullIfBlank(cur.ThankYouText), nullIfBlank(cur.FooterText), nullIfBlank(cur.WarrantyText),
		cur.ShowVATBreakdown, cur.ShowIMEI, cur.ShowPayments, cur.ShowBalance, cur.ShowServedBy,
		cur.DefaultPaper, cur.NumberPrefix, cur.NextNumber, actorID)
	if err != nil {
		return cur, err
	}
	return s.GetSettings(ctx, tenantID)
}

func (s *Service) ensureRow(ctx context.Context, tenantID uuid.UUID) error {
	d := DefaultSettings(tenantID)
	_, err := s.pool.Exec(ctx, `
		INSERT INTO platform.receipt_settings
			(tenant_id, thank_you_text, warranty_text, default_paper, number_prefix, next_number)
		VALUES ($1, $2, $3, $4, $5, $6)
		ON CONFLICT (tenant_id) DO NOTHING`,
		tenantID, d.ThankYouText, d.WarrantyText, d.DefaultPaper, d.NumberPrefix, d.NextNumber)
	return err
}

// SaveLogo stores the shop logo. Object storage is the store of record when it
// is configured; otherwise the bytes live in the settings row so small
// deployments still get a branded receipt.
func (s *Service) SaveLogo(ctx context.Context, tenantID uuid.UUID, body []byte, contentType string) error {
	if len(body) == 0 {
		return errors.New("logo file is empty")
	}
	if len(body) > maxLogoBytes {
		return fmt.Errorf("logo must be %d KB or smaller", maxLogoBytes/1024)
	}
	detected, ok := sniffImage(body, contentType)
	if !ok {
		return errors.New("logo must be a PNG, JPEG, WebP, GIF or SVG image")
	}
	if err := s.ensureRow(ctx, tenantID); err != nil {
		return err
	}

	key := ""
	var dbBytes []byte = body
	if s.store != nil {
		key = fmt.Sprintf("tenants/%s/branding/receipt-logo", tenantID)
		if err := s.store.Put(ctx, key, body, detected); err != nil {
			// Fall back to the database rather than leaving the owner with no logo.
			key = ""
		} else {
			dbBytes = nil
		}
	}

	_, err := s.pool.Exec(ctx, `
		UPDATE platform.receipt_settings
		SET logo_object_key = $2, logo_bytes = $3, logo_content_type = $4, logo_updated_at = now(), updated_at = now()
		WHERE tenant_id = $1`,
		tenantID, nullIfBlank(key), dbBytes, detected)
	if err != nil {
		return err
	}
	s.invalidateLogo(tenantID)
	return nil
}

func (s *Service) DeleteLogo(ctx context.Context, tenantID uuid.UUID) error {
	var key *string
	err := s.pool.QueryRow(ctx, `SELECT logo_object_key FROM platform.receipt_settings WHERE tenant_id = $1`, tenantID).Scan(&key)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return err
	}
	if key != nil && s.store != nil {
		_ = s.store.Delete(ctx, *key)
	}
	_, err = s.pool.Exec(ctx, `
		UPDATE platform.receipt_settings
		SET logo_object_key = NULL, logo_bytes = NULL, logo_content_type = NULL, logo_updated_at = NULL, updated_at = now()
		WHERE tenant_id = $1`, tenantID)
	if err != nil {
		return err
	}
	s.invalidateLogo(tenantID)
	return nil
}

// LogoDataURI returns the logo inlined as a data URI. Receipts are printed from
// popup windows and Android WebViews that carry no auth headers, so embedding
// beats linking to a protected URL.
func (s *Service) LogoDataURI(ctx context.Context, tenantID uuid.UUID) string {
	s.mu.Lock()
	entry, ok := s.logoCache[tenantID]
	s.mu.Unlock()
	if ok && time.Since(entry.loaded) < logoCacheTTL {
		return entry.dataURI
	}

	var key, contentType *string
	var raw []byte
	err := s.pool.QueryRow(ctx, `
		SELECT logo_object_key, logo_content_type, logo_bytes
		FROM platform.receipt_settings WHERE tenant_id = $1`, tenantID).Scan(&key, &contentType, &raw)
	if err != nil {
		return ""
	}
	if len(raw) == 0 && key != nil && s.store != nil {
		if fetched, fetchErr := s.store.Get(ctx, *key); fetchErr == nil {
			raw = fetched
		}
	}
	uri := ""
	if len(raw) > 0 {
		uri = dataURI(deref(contentType), raw)
	}
	s.mu.Lock()
	s.logoCache[tenantID] = logoCacheEntry{dataURI: uri, loaded: time.Now()}
	s.mu.Unlock()
	return uri
}

func (s *Service) invalidateLogo(tenantID uuid.UUID) {
	s.mu.Lock()
	delete(s.logoCache, tenantID)
	s.mu.Unlock()
}

// ReceiptNumber returns the serial for a document, allocating one on first
// print and reusing it on every reprint.
func (s *Service) ReceiptNumber(ctx context.Context, tenantID uuid.UUID, docType string, docID uuid.UUID) string {
	var formatted string
	err := s.pool.QueryRow(ctx, `
		SELECT formatted FROM platform.receipt_numbers
		WHERE tenant_id = $1 AND doc_type = $2 AND doc_id = $3`, tenantID, docType, docID).Scan(&formatted)
	if err == nil {
		return formatted
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return ""
	}
	if err := s.ensureRow(ctx, tenantID); err != nil {
		return ""
	}

	var number int64
	var prefix string
	err = s.pool.QueryRow(ctx, `
		UPDATE platform.receipt_settings SET next_number = next_number + 1
		WHERE tenant_id = $1
		RETURNING next_number - 1, number_prefix`, tenantID).Scan(&number, &prefix)
	if err != nil {
		return ""
	}
	formatted = FormatNumber(prefix, number)
	_, err = s.pool.Exec(ctx, `
		INSERT INTO platform.receipt_numbers (tenant_id, doc_type, doc_id, number, formatted)
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (tenant_id, doc_type, doc_id) DO NOTHING`,
		tenantID, docType, docID, number, formatted)
	if err != nil {
		return formatted
	}
	// Another print of the same document may have won the race; prefer its number.
	var stored string
	if scanErr := s.pool.QueryRow(ctx, `
		SELECT formatted FROM platform.receipt_numbers
		WHERE tenant_id = $1 AND doc_type = $2 AND doc_id = $3`, tenantID, docType, docID).Scan(&stored); scanErr == nil && stored != "" {
		return stored
	}
	return formatted
}

func FormatNumber(prefix string, number int64) string {
	return fmt.Sprintf("%s%05d", prefix, number)
}

func deref(v *string) string {
	if v == nil {
		return ""
	}
	return *v
}

func nullIfBlank(v string) *string {
	trimmed := strings.TrimSpace(v)
	if trimmed == "" {
		return nil
	}
	return &trimmed
}

func applyString(dst *string, src *string) {
	if src != nil {
		*dst = strings.TrimSpace(*src)
	}
}

func applyBool(dst *bool, src *bool) {
	if src != nil {
		*dst = *src
	}
}
