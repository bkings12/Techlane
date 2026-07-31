// Package storefrontcms owns everything an owner configures for the public
// web-storefront: hero/footer copy, promotional banners, timed deals and
// newsletter subscribers. It stays dependency-free (no internal/inventory
// import) — internal/commerce, which already depends on inventory, is the
// one that joins CMS content with catalog data.
package storefrontcms

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/mail"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

const maxImageBytes = 2 * 1024 * 1024

// ObjectStore is the slice of object storage storefrontcms needs for banner
// images — the same shape internal/receipts uses for logos.
type ObjectStore interface {
	Put(ctx context.Context, key string, body []byte, contentType string) error
	Get(ctx context.Context, key string) ([]byte, error)
	Delete(ctx context.Context, key string) error
}

type Service struct {
	pool       *pgxpool.Pool
	store      ObjectStore
	httpClient *http.Client
}

func NewService(pool *pgxpool.Pool) *Service {
	return &Service{pool: pool, httpClient: &http.Client{Timeout: 6 * time.Second}}
}

func (s *Service) SetObjectStore(store ObjectStore) { s.store = store }

// TechLane brand defaults: gold leads CTAs; charcoal for dark chrome;
// navy is the accent only (small highlights — avoids a blue-heavy storefront).
const (
	DefaultColorPrimary   = "#F2BE2A" // brand gold
	DefaultColorSecondary = "#1A1A1A" // near-black chrome / footer
	DefaultColorAccent    = "#060386" // brand navy — use sparingly
)

// Settings is the branding/footer/contact copy the owner edits.
type Settings struct {
	TenantID uuid.UUID `json:"tenant_id"`

	// Brand — shop name shown in header/footer; empty falls back to tenant name.
	ShopDisplayName string `json:"shop_display_name"`
	PageTitle       string `json:"page_title"`
	HasLogo         bool   `json:"has_logo"`
	LogoContentType string `json:"logo_content_type,omitempty"`
	LogoUpdatedAt   *time.Time `json:"logo_updated_at,omitempty"`

	// Theme colors (hex). Applied as CSS variables on the public storefront.
	ColorPrimary   string `json:"color_primary"`
	ColorSecondary string `json:"color_secondary"`
	ColorAccent    string `json:"color_accent"`

	// Top bar & header promo (Electro-style chrome).
	TopbarHelpHref    string `json:"topbar_help_href"`
	TopbarSupportHref string `json:"topbar_support_href"`
	TopbarContactHref string `json:"topbar_contact_href"`
	TopbarPhoneLabel  string `json:"topbar_phone_label"`
	HeaderPromoText   string `json:"header_promo_text"`

	// Homepage product-rail toggles.
	ShowFeatured    bool `json:"show_featured"`
	ShowNewArrivals bool `json:"show_new_arrivals"`
	ShowBestsellers bool `json:"show_bestsellers"`
	ShowDeals       bool `json:"show_deals"`
	ShowMostViewed  bool `json:"show_most_viewed"`

	HeroHeadline string `json:"hero_headline"`
	HeroSubtext  string `json:"hero_subtext"`
	HeroCTALabel string `json:"hero_cta_label"`
	HeroCTAHref  string `json:"hero_cta_href"`

	NewsletterHeadline string `json:"newsletter_headline"`
	NewsletterSubtext  string `json:"newsletter_subtext"`

	FooterTagline   string `json:"footer_tagline"`
	SocialFacebook  string `json:"social_facebook"`
	SocialInstagram string `json:"social_instagram"`
	SocialTwitter   string `json:"social_twitter"`
	SocialTikTok    string `json:"social_tiktok"`
	ContactPhone    string `json:"contact_phone"`
	ContactEmail    string `json:"contact_email"`
	BusinessHours   string `json:"business_hours"`
	AppStoreURL     string `json:"app_store_url"`
	PlayStoreURL    string `json:"play_store_url"`

	// Comma-separated ISO currency codes offered on the display-only
	// storefront currency switcher (e.g. "KES,USD"). Empty hides it.
	EnabledCurrencies string `json:"enabled_currencies"`

	// Four owner-editable trust badges shown in a row on the homepage.
	// Deliberately not hardcoded ("Worldwide Shipping" etc. would be false
	// claims for a branch-pickup business) — blank pairs just don't render.
	TrustBadge1Title   string `json:"trust_badge_1_title"`
	TrustBadge1Subtext string `json:"trust_badge_1_subtext"`
	TrustBadge2Title   string `json:"trust_badge_2_title"`
	TrustBadge2Subtext string `json:"trust_badge_2_subtext"`
	TrustBadge3Title   string `json:"trust_badge_3_title"`
	TrustBadge3Subtext string `json:"trust_badge_3_subtext"`
	TrustBadge4Title   string `json:"trust_badge_4_title"`
	TrustBadge4Subtext string `json:"trust_badge_4_subtext"`

	// Checkout payment method labels / hints / button CTAs (storefront).
	PayLabelSTK     string `json:"pay_label_stk"`
	PayLabelPaybill string `json:"pay_label_paybill"`
	PayLabelCash    string `json:"pay_label_cash"`
	PayHintSTK      string `json:"pay_hint_stk"`
	PayHintPaybill  string `json:"pay_hint_paybill"`
	PayHintCash     string `json:"pay_hint_cash"`
	PayCTASTK       string `json:"pay_cta_stk"`
	PayCTAPaybill   string `json:"pay_cta_paybill"`
	PayCTACash      string `json:"pay_cta_cash"`

	UpdatedAt time.Time `json:"updated_at"`
}

func DefaultSettings(tenantID uuid.UUID) Settings {
	return Settings{
		TenantID:           tenantID,
		ColorPrimary:       DefaultColorPrimary,
		ColorSecondary:     DefaultColorSecondary,
		ColorAccent:        DefaultColorAccent,
		ShowFeatured:       true,
		ShowNewArrivals:    true,
		ShowBestsellers:    true,
		ShowDeals:          true,
		ShowMostViewed:     true,
		HeroHeadline:       "Quality repairs and genuine parts",
		HeroSubtext:        "Shop devices and accessories, or book a repair with our technicians.",
		HeroCTALabel:       "Shop now",
		HeroCTAHref:        "/shop",
		NewsletterHeadline: "Join our newsletter",
		NewsletterSubtext:  "Get updates on new arrivals and deals.",
		FooterTagline:      "Your trusted repair shop.",
		PayLabelSTK:        "M-Pesa STK push",
		PayLabelPaybill:    "M-Pesa paybill",
		PayLabelCash:       "Cash on pickup",
		PayHintSTK:         "Phone (07… or 2547…)",
		PayHintPaybill:     "After checkout you will pay by paybill with an ORD-… account reference.",
		PayHintCash:        "Stock is held for pickup. Pay cash at the branch counter — not available for delivery.",
		PayCTASTK:          "Pay with M-Pesa STK",
		PayCTAPaybill:      "Checkout with paybill",
		PayCTACash:         "Reserve for cash pickup",
		UpdatedAt:          time.Now().UTC(),
	}
}

func (s *Service) GetSettings(ctx context.Context, tenantID uuid.UUID) (Settings, error) {
	out := DefaultSettings(tenantID)
	var shopName, pageTitle, colorPrimary, colorSecondary, colorAccent *string
	var topHelp, topSupport, topContact, topPhone, headerPromo *string
	var showFeatured, showNew, showBest, showDeals, showViewed *bool
	var logoType *string
	var heroHeadline, heroSubtext, heroCTALabel, heroCTAHref *string
	var nlHeadline, nlSubtext *string
	var footerTagline, fb, ig, tw, tiktok, phone, email, hours, appURL, playURL *string
	var enabledCurrencies *string
	var badge1t, badge1s, badge2t, badge2s, badge3t, badge3s, badge4t, badge4s *string
	var payLabelSTK, payLabelPaybill, payLabelCash *string
	var payHintSTK, payHintPaybill, payHintCash *string
	var payCTASTK, payCTAPaybill, payCTACash *string

	err := s.pool.QueryRow(ctx, `
		SELECT shop_display_name, page_title, color_primary, color_secondary, color_accent,
		       topbar_help_href, topbar_support_href, topbar_contact_href, topbar_phone_label, header_promo_text,
		       show_featured, show_new_arrivals, show_bestsellers, show_deals, show_most_viewed,
		       logo_object_key IS NOT NULL OR logo_bytes IS NOT NULL, logo_content_type, logo_updated_at,
		       hero_headline, hero_subtext, hero_cta_label, hero_cta_href,
		       newsletter_headline, newsletter_subtext,
		       footer_tagline, social_facebook, social_instagram, social_twitter, social_tiktok,
		       contact_phone, contact_email, business_hours, app_store_url, play_store_url,
		       enabled_currencies,
		       trust_badge_1_title, trust_badge_1_subtext, trust_badge_2_title, trust_badge_2_subtext,
		       trust_badge_3_title, trust_badge_3_subtext, trust_badge_4_title, trust_badge_4_subtext,
		       pay_label_stk, pay_label_paybill, pay_label_cash,
		       pay_hint_stk, pay_hint_paybill, pay_hint_cash,
		       pay_cta_stk, pay_cta_paybill, pay_cta_cash,
		       updated_at
		FROM platform.storefront_settings WHERE tenant_id = $1`, tenantID).
		Scan(&shopName, &pageTitle, &colorPrimary, &colorSecondary, &colorAccent,
			&topHelp, &topSupport, &topContact, &topPhone, &headerPromo,
			&showFeatured, &showNew, &showBest, &showDeals, &showViewed,
			&out.HasLogo, &logoType, &out.LogoUpdatedAt,
			&heroHeadline, &heroSubtext, &heroCTALabel, &heroCTAHref,
			&nlHeadline, &nlSubtext,
			&footerTagline, &fb, &ig, &tw, &tiktok,
			&phone, &email, &hours, &appURL, &playURL,
			&enabledCurrencies,
			&badge1t, &badge1s, &badge2t, &badge2s, &badge3t, &badge3s, &badge4t, &badge4s,
			&payLabelSTK, &payLabelPaybill, &payLabelCash,
			&payHintSTK, &payHintPaybill, &payHintCash,
			&payCTASTK, &payCTAPaybill, &payCTACash,
			&out.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return out, nil
	}
	if err != nil {
		return out, err
	}

	out.ShopDisplayName = deref(shopName)
	out.PageTitle = deref(pageTitle)
	if c := strings.TrimSpace(deref(colorPrimary)); c != "" {
		out.ColorPrimary = c
	}
	if c := strings.TrimSpace(deref(colorSecondary)); c != "" {
		out.ColorSecondary = c
	}
	if c := strings.TrimSpace(deref(colorAccent)); c != "" {
		out.ColorAccent = c
	}
	out.TopbarHelpHref = deref(topHelp)
	out.TopbarSupportHref = deref(topSupport)
	out.TopbarContactHref = deref(topContact)
	out.TopbarPhoneLabel = deref(topPhone)
	out.HeaderPromoText = deref(headerPromo)
	if showFeatured != nil {
		out.ShowFeatured = *showFeatured
	}
	if showNew != nil {
		out.ShowNewArrivals = *showNew
	}
	if showBest != nil {
		out.ShowBestsellers = *showBest
	}
	if showDeals != nil {
		out.ShowDeals = *showDeals
	}
	if showViewed != nil {
		out.ShowMostViewed = *showViewed
	}
	out.LogoContentType = deref(logoType)
	applyDeref(&out.HeroHeadline, heroHeadline)
	applyDeref(&out.HeroSubtext, heroSubtext)
	applyDeref(&out.HeroCTALabel, heroCTALabel)
	applyDeref(&out.HeroCTAHref, heroCTAHref)
	applyDeref(&out.NewsletterHeadline, nlHeadline)
	applyDeref(&out.NewsletterSubtext, nlSubtext)
	applyDeref(&out.FooterTagline, footerTagline)
	out.SocialFacebook = deref(fb)
	out.SocialInstagram = deref(ig)
	out.SocialTwitter = deref(tw)
	out.SocialTikTok = deref(tiktok)
	out.ContactPhone = deref(phone)
	out.ContactEmail = deref(email)
	out.BusinessHours = deref(hours)
	out.AppStoreURL = deref(appURL)
	out.PlayStoreURL = deref(playURL)
	out.EnabledCurrencies = deref(enabledCurrencies)
	out.TrustBadge1Title = deref(badge1t)
	out.TrustBadge1Subtext = deref(badge1s)
	out.TrustBadge2Title = deref(badge2t)
	out.TrustBadge2Subtext = deref(badge2s)
	out.TrustBadge3Title = deref(badge3t)
	out.TrustBadge3Subtext = deref(badge3s)
	out.TrustBadge4Title = deref(badge4t)
	out.TrustBadge4Subtext = deref(badge4s)
	applyDeref(&out.PayLabelSTK, payLabelSTK)
	applyDeref(&out.PayLabelPaybill, payLabelPaybill)
	applyDeref(&out.PayLabelCash, payLabelCash)
	applyDeref(&out.PayHintSTK, payHintSTK)
	applyDeref(&out.PayHintPaybill, payHintPaybill)
	applyDeref(&out.PayHintCash, payHintCash)
	applyDeref(&out.PayCTASTK, payCTASTK)
	applyDeref(&out.PayCTAPaybill, payCTAPaybill)
	applyDeref(&out.PayCTACash, payCTACash)
	return out, nil
}

// UpsertSettingsInput carries only the fields the caller wants changed.
type UpsertSettingsInput struct {
	ShopDisplayName *string
	PageTitle       *string
	ColorPrimary    *string
	ColorSecondary  *string
	ColorAccent     *string

	TopbarHelpHref    *string
	TopbarSupportHref *string
	TopbarContactHref *string
	TopbarPhoneLabel  *string
	HeaderPromoText   *string

	ShowFeatured    *bool
	ShowNewArrivals *bool
	ShowBestsellers *bool
	ShowDeals       *bool
	ShowMostViewed  *bool

	HeroHeadline *string
	HeroSubtext  *string
	HeroCTALabel *string
	HeroCTAHref  *string

	NewsletterHeadline *string
	NewsletterSubtext  *string

	FooterTagline   *string
	SocialFacebook  *string
	SocialInstagram *string
	SocialTwitter   *string
	SocialTikTok    *string
	ContactPhone    *string
	ContactEmail    *string
	BusinessHours   *string
	AppStoreURL     *string
	PlayStoreURL    *string

	EnabledCurrencies *string

	TrustBadge1Title   *string
	TrustBadge1Subtext *string
	TrustBadge2Title   *string
	TrustBadge2Subtext *string
	TrustBadge3Title   *string
	TrustBadge3Subtext *string
	TrustBadge4Title   *string
	TrustBadge4Subtext *string

	PayLabelSTK     *string
	PayLabelPaybill *string
	PayLabelCash    *string
	PayHintSTK      *string
	PayHintPaybill  *string
	PayHintCash     *string
	PayCTASTK       *string
	PayCTAPaybill   *string
	PayCTACash      *string
}

func normalizeHexColor(v string, fallback string) string {
	v = strings.TrimSpace(strings.ToLower(v))
	if v == "" {
		return fallback
	}
	if !strings.HasPrefix(v, "#") {
		v = "#" + v
	}
	if len(v) != 4 && len(v) != 7 {
		return fallback
	}
	for _, c := range v[1:] {
		if (c < '0' || c > '9') && (c < 'a' || c > 'f') {
			return fallback
		}
	}
	return v
}

func (s *Service) UpsertSettings(ctx context.Context, tenantID uuid.UUID, in UpsertSettingsInput, actorID uuid.UUID) (Settings, error) {
	cur, err := s.GetSettings(ctx, tenantID)
	if err != nil {
		return cur, err
	}

	applyString(&cur.ShopDisplayName, in.ShopDisplayName)
	applyString(&cur.PageTitle, in.PageTitle)
	if in.ColorPrimary != nil {
		cur.ColorPrimary = normalizeHexColor(*in.ColorPrimary, DefaultColorPrimary)
	}
	if in.ColorSecondary != nil {
		cur.ColorSecondary = normalizeHexColor(*in.ColorSecondary, DefaultColorSecondary)
	}
	if in.ColorAccent != nil {
		cur.ColorAccent = normalizeHexColor(*in.ColorAccent, DefaultColorAccent)
	}
	applyString(&cur.TopbarHelpHref, in.TopbarHelpHref)
	applyString(&cur.TopbarSupportHref, in.TopbarSupportHref)
	applyString(&cur.TopbarContactHref, in.TopbarContactHref)
	applyString(&cur.TopbarPhoneLabel, in.TopbarPhoneLabel)
	applyString(&cur.HeaderPromoText, in.HeaderPromoText)
	if in.ShowFeatured != nil {
		cur.ShowFeatured = *in.ShowFeatured
	}
	if in.ShowNewArrivals != nil {
		cur.ShowNewArrivals = *in.ShowNewArrivals
	}
	if in.ShowBestsellers != nil {
		cur.ShowBestsellers = *in.ShowBestsellers
	}
	if in.ShowDeals != nil {
		cur.ShowDeals = *in.ShowDeals
	}
	if in.ShowMostViewed != nil {
		cur.ShowMostViewed = *in.ShowMostViewed
	}

	applyString(&cur.HeroHeadline, in.HeroHeadline)
	applyString(&cur.HeroSubtext, in.HeroSubtext)
	applyString(&cur.HeroCTALabel, in.HeroCTALabel)
	applyString(&cur.HeroCTAHref, in.HeroCTAHref)
	applyString(&cur.NewsletterHeadline, in.NewsletterHeadline)
	applyString(&cur.NewsletterSubtext, in.NewsletterSubtext)
	applyString(&cur.FooterTagline, in.FooterTagline)
	applyString(&cur.SocialFacebook, in.SocialFacebook)
	applyString(&cur.SocialInstagram, in.SocialInstagram)
	applyString(&cur.SocialTwitter, in.SocialTwitter)
	applyString(&cur.SocialTikTok, in.SocialTikTok)
	applyString(&cur.ContactPhone, in.ContactPhone)
	applyString(&cur.ContactEmail, in.ContactEmail)
	applyString(&cur.BusinessHours, in.BusinessHours)
	applyString(&cur.AppStoreURL, in.AppStoreURL)
	applyString(&cur.PlayStoreURL, in.PlayStoreURL)
	applyString(&cur.EnabledCurrencies, in.EnabledCurrencies)
	applyString(&cur.TrustBadge1Title, in.TrustBadge1Title)
	applyString(&cur.TrustBadge1Subtext, in.TrustBadge1Subtext)
	applyString(&cur.TrustBadge2Title, in.TrustBadge2Title)
	applyString(&cur.TrustBadge2Subtext, in.TrustBadge2Subtext)
	applyString(&cur.TrustBadge3Title, in.TrustBadge3Title)
	applyString(&cur.TrustBadge3Subtext, in.TrustBadge3Subtext)
	applyString(&cur.TrustBadge4Title, in.TrustBadge4Title)
	applyString(&cur.TrustBadge4Subtext, in.TrustBadge4Subtext)
	applyString(&cur.PayLabelSTK, in.PayLabelSTK)
	applyString(&cur.PayLabelPaybill, in.PayLabelPaybill)
	applyString(&cur.PayLabelCash, in.PayLabelCash)
	applyString(&cur.PayHintSTK, in.PayHintSTK)
	applyString(&cur.PayHintPaybill, in.PayHintPaybill)
	applyString(&cur.PayHintCash, in.PayHintCash)
	applyString(&cur.PayCTASTK, in.PayCTASTK)
	applyString(&cur.PayCTAPaybill, in.PayCTAPaybill)
	applyString(&cur.PayCTACash, in.PayCTACash)

	_, err = s.pool.Exec(ctx, `
		INSERT INTO platform.storefront_settings (
			tenant_id,
			shop_display_name, page_title, color_primary, color_secondary, color_accent,
			topbar_help_href, topbar_support_href, topbar_contact_href, topbar_phone_label, header_promo_text,
			show_featured, show_new_arrivals, show_bestsellers, show_deals, show_most_viewed,
			hero_headline, hero_subtext, hero_cta_label, hero_cta_href,
			newsletter_headline, newsletter_subtext,
			footer_tagline, social_facebook, social_instagram, social_twitter, social_tiktok,
			contact_phone, contact_email, business_hours, app_store_url, play_store_url,
			enabled_currencies,
			trust_badge_1_title, trust_badge_1_subtext, trust_badge_2_title, trust_badge_2_subtext,
			trust_badge_3_title, trust_badge_3_subtext, trust_badge_4_title, trust_badge_4_subtext,
			pay_label_stk, pay_label_paybill, pay_label_cash,
			pay_hint_stk, pay_hint_paybill, pay_hint_cash,
			pay_cta_stk, pay_cta_paybill, pay_cta_cash,
			updated_at, updated_by
		) VALUES (
			$1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,$19,$20,
			$21,$22,$23,$24,$25,$26,$27,$28,$29,$30,$31,$32,$33,$34,$35,$36,$37,$38,$39,$40,$41,
			$42,$43,$44,$45,$46,$47,$48,$49,$50,
			now(),$51
		)
		ON CONFLICT (tenant_id) DO UPDATE SET
			shop_display_name = $2, page_title = $3, color_primary = $4, color_secondary = $5, color_accent = $6,
			topbar_help_href = $7, topbar_support_href = $8, topbar_contact_href = $9, topbar_phone_label = $10, header_promo_text = $11,
			show_featured = $12, show_new_arrivals = $13, show_bestsellers = $14, show_deals = $15, show_most_viewed = $16,
			hero_headline = $17, hero_subtext = $18, hero_cta_label = $19, hero_cta_href = $20,
			newsletter_headline = $21, newsletter_subtext = $22,
			footer_tagline = $23, social_facebook = $24, social_instagram = $25, social_twitter = $26, social_tiktok = $27,
			contact_phone = $28, contact_email = $29, business_hours = $30, app_store_url = $31, play_store_url = $32,
			enabled_currencies = $33,
			trust_badge_1_title = $34, trust_badge_1_subtext = $35, trust_badge_2_title = $36, trust_badge_2_subtext = $37,
			trust_badge_3_title = $38, trust_badge_3_subtext = $39, trust_badge_4_title = $40, trust_badge_4_subtext = $41,
			pay_label_stk = $42, pay_label_paybill = $43, pay_label_cash = $44,
			pay_hint_stk = $45, pay_hint_paybill = $46, pay_hint_cash = $47,
			pay_cta_stk = $48, pay_cta_paybill = $49, pay_cta_cash = $50,
			updated_at = now(), updated_by = $51`,
		tenantID,
		nullIfBlank(cur.ShopDisplayName), nullIfBlank(cur.PageTitle),
		nullIfBlank(cur.ColorPrimary), nullIfBlank(cur.ColorSecondary), nullIfBlank(cur.ColorAccent),
		nullIfBlank(cur.TopbarHelpHref), nullIfBlank(cur.TopbarSupportHref), nullIfBlank(cur.TopbarContactHref),
		nullIfBlank(cur.TopbarPhoneLabel), nullIfBlank(cur.HeaderPromoText),
		cur.ShowFeatured, cur.ShowNewArrivals, cur.ShowBestsellers, cur.ShowDeals, cur.ShowMostViewed,
		nullIfBlank(cur.HeroHeadline), nullIfBlank(cur.HeroSubtext), nullIfBlank(cur.HeroCTALabel), nullIfBlank(cur.HeroCTAHref),
		nullIfBlank(cur.NewsletterHeadline), nullIfBlank(cur.NewsletterSubtext),
		nullIfBlank(cur.FooterTagline), nullIfBlank(cur.SocialFacebook), nullIfBlank(cur.SocialInstagram), nullIfBlank(cur.SocialTwitter), nullIfBlank(cur.SocialTikTok),
		nullIfBlank(cur.ContactPhone), nullIfBlank(cur.ContactEmail), nullIfBlank(cur.BusinessHours), nullIfBlank(cur.AppStoreURL), nullIfBlank(cur.PlayStoreURL),
		nullIfBlank(cur.EnabledCurrencies),
		nullIfBlank(cur.TrustBadge1Title), nullIfBlank(cur.TrustBadge1Subtext), nullIfBlank(cur.TrustBadge2Title), nullIfBlank(cur.TrustBadge2Subtext),
		nullIfBlank(cur.TrustBadge3Title), nullIfBlank(cur.TrustBadge3Subtext), nullIfBlank(cur.TrustBadge4Title), nullIfBlank(cur.TrustBadge4Subtext),
		nullIfBlank(cur.PayLabelSTK), nullIfBlank(cur.PayLabelPaybill), nullIfBlank(cur.PayLabelCash),
		nullIfBlank(cur.PayHintSTK), nullIfBlank(cur.PayHintPaybill), nullIfBlank(cur.PayHintCash),
		nullIfBlank(cur.PayCTASTK), nullIfBlank(cur.PayCTAPaybill), nullIfBlank(cur.PayCTACash),
		actorID)
	if err != nil {
		return cur, err
	}
	return s.GetSettings(ctx, tenantID)
}

// SaveLogo stores the shop logo used in the Electro header.
func (s *Service) SaveLogo(ctx context.Context, tenantID uuid.UUID, body []byte, contentType string, actorID uuid.UUID) error {
	if len(body) == 0 {
		return errors.New("image file is empty")
	}
	if len(body) > maxImageBytes {
		return fmt.Errorf("image must be %d KB or smaller", maxImageBytes/1024)
	}
	detected, ok := sniffImage(body, contentType)
	if !ok {
		return errors.New("image must be a PNG, JPEG or WebP image")
	}

	// Ensure a settings row exists so the logo update has a target.
	if _, err := s.UpsertSettings(ctx, tenantID, UpsertSettingsInput{}, actorID); err != nil {
		return err
	}

	key := ""
	var dbBytes []byte = body
	if s.store != nil {
		key = fmt.Sprintf("tenants/%s/storefront/logo", tenantID)
		if err := s.store.Put(ctx, key, body, detected); err != nil {
			key = ""
		} else {
			dbBytes = nil
		}
	}

	_, err := s.pool.Exec(ctx, `
		UPDATE platform.storefront_settings
		SET logo_object_key = $2, logo_bytes = $3, logo_content_type = $4, logo_updated_at = now(), updated_at = now(), updated_by = $5
		WHERE tenant_id = $1`,
		tenantID, nullIfBlank(key), dbBytes, detected, actorID)
	return err
}

func (s *Service) DeleteLogo(ctx context.Context, tenantID uuid.UUID, actorID uuid.UUID) error {
	var key *string
	_ = s.pool.QueryRow(ctx, `SELECT logo_object_key FROM platform.storefront_settings WHERE tenant_id = $1`, tenantID).Scan(&key)
	if key != nil && *key != "" && s.store != nil {
		_ = s.store.Delete(ctx, *key)
	}
	_, err := s.pool.Exec(ctx, `
		UPDATE platform.storefront_settings
		SET logo_object_key = NULL, logo_bytes = NULL, logo_content_type = NULL, logo_updated_at = NULL, updated_at = now(), updated_by = $2
		WHERE tenant_id = $1`, tenantID, actorID)
	return err
}

// LogoImage returns raw logo bytes for the public unauthenticated image route.
func (s *Service) LogoImage(ctx context.Context, tenantID uuid.UUID) ([]byte, string, error) {
	var key, contentType *string
	var raw []byte
	err := s.pool.QueryRow(ctx, `
		SELECT logo_object_key, logo_content_type, logo_bytes
		FROM platform.storefront_settings WHERE tenant_id = $1`, tenantID).Scan(&key, &contentType, &raw)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, "", fmt.Errorf("logo not found")
	}
	if err != nil {
		return nil, "", err
	}
	if len(raw) == 0 && key != nil && s.store != nil {
		if fetched, fetchErr := s.store.Get(ctx, *key); fetchErr == nil {
			raw = fetched
		}
	}
	if len(raw) == 0 {
		return nil, "", fmt.Errorf("logo not found")
	}
	return raw, deref(contentType), nil
}

// Banner is a hero-slider slide or promo tile, depending on Placement.
type Banner struct {
	ID       uuid.UUID `json:"id"`
	Headline string    `json:"headline"`
	Subtext  string    `json:"subtext"`
	CTALabel string    `json:"cta_label"`
	CTAHref  string    `json:"cta_href"`

	HasImage         bool       `json:"has_image"`
	ImageContentType string     `json:"image_content_type,omitempty"`
	ImageUpdatedAt   *time.Time `json:"image_updated_at,omitempty"`

	// Placement is "hero" (slider), "side" (cards beside slider), or "mid"
	// (banner row below products). Legacy "promo_tile" means "side".
	Placement string     `json:"placement"`
	DealID    *uuid.UUID `json:"deal_id,omitempty"`

	// Populated when DealID is set, via a join, so the frontend can show a
	// real was/now price on the slide without a second request.
	DealVariantID *uuid.UUID `json:"deal_variant_id,omitempty"`
	DealPrice     *float64   `json:"deal_price,omitempty"`
	DealBasePrice *float64   `json:"deal_base_price,omitempty"`

	SortOrder int  `json:"sort_order"`
	Active    bool `json:"active"`

	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

const bannerColumns = `b.id, b.headline, b.subtext, b.cta_label, b.cta_href,
	b.image_object_key IS NOT NULL OR b.image_bytes IS NOT NULL, b.image_content_type, b.image_updated_at,
	b.placement, b.deal_id, d.variant_id, d.deal_price::float8, v.sell_price::float8,
	b.sort_order, b.active, b.created_at, b.updated_at`

const bannerFrom = `FROM platform.storefront_banners b
	LEFT JOIN platform.storefront_deals d ON d.id = b.deal_id
	LEFT JOIN inventory.product_variants v ON v.id = d.variant_id`

func scanBanner(row pgx.Row) (Banner, error) {
	var b Banner
	var headline, subtext, ctaLabel, ctaHref, imageType *string
	err := row.Scan(&b.ID, &headline, &subtext, &ctaLabel, &ctaHref,
		&b.HasImage, &imageType, &b.ImageUpdatedAt,
		&b.Placement, &b.DealID, &b.DealVariantID, &b.DealPrice, &b.DealBasePrice,
		&b.SortOrder, &b.Active, &b.CreatedAt, &b.UpdatedAt)
	b.Headline = deref(headline)
	b.Subtext = deref(subtext)
	b.CTALabel = deref(ctaLabel)
	b.CTAHref = deref(ctaHref)
	b.ImageContentType = deref(imageType)
	return b, err
}

func (s *Service) ListBanners(ctx context.Context, tenantID uuid.UUID, activeOnly bool) ([]Banner, error) {
	q := `SELECT ` + bannerColumns + ` ` + bannerFrom + ` WHERE b.tenant_id = $1`
	if activeOnly {
		q += ` AND b.active`
	}
	q += ` ORDER BY b.sort_order, b.created_at`
	rows, err := s.pool.Query(ctx, q, tenantID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]Banner, 0)
	for rows.Next() {
		b, err := scanBanner(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, b)
	}
	return items, rows.Err()
}

type BannerInput struct {
	Headline  *string
	Subtext   *string
	CTALabel  *string
	CTAHref   *string
	Placement *string
	DealID    **uuid.UUID
	SortOrder *int
	Active    *bool
}

func normalizePlacement(p string) (string, error) {
	switch strings.TrimSpace(p) {
	case "hero":
		return "hero", nil
	case "side", "promo_tile":
		// promo_tile kept as a legacy alias for side cards beside the hero.
		return "side", nil
	case "mid", "mid_banner", "static_banner":
		return "mid", nil
	default:
		return "", fmt.Errorf("placement must be 'hero', 'side', or 'mid'")
	}
}

func (s *Service) CreateBanner(ctx context.Context, tenantID uuid.UUID, in BannerInput) (*Banner, error) {
	id := uuid.New()
	sortOrder := 0
	if in.SortOrder != nil {
		sortOrder = *in.SortOrder
	}
	active := true
	if in.Active != nil {
		active = *in.Active
	}
	placement := "hero"
	if in.Placement != nil {
		p, err := normalizePlacement(*in.Placement)
		if err != nil {
			return nil, err
		}
		placement = p
	}
	var dealID *uuid.UUID
	if in.DealID != nil {
		dealID = *in.DealID
	}
	_, err := s.pool.Exec(ctx, `
		INSERT INTO platform.storefront_banners (id, tenant_id, headline, subtext, cta_label, cta_href, placement, deal_id, sort_order, active)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)`,
		id, tenantID, nullIfBlankPtr(in.Headline), nullIfBlankPtr(in.Subtext), nullIfBlankPtr(in.CTALabel), nullIfBlankPtr(in.CTAHref),
		placement, dealID, sortOrder, active)
	if err != nil {
		return nil, err
	}
	return s.getBanner(ctx, tenantID, id)
}

func (s *Service) UpdateBanner(ctx context.Context, tenantID, id uuid.UUID, in BannerInput) (*Banner, error) {
	var placement *string
	if in.Placement != nil {
		p, err := normalizePlacement(*in.Placement)
		if err != nil {
			return nil, err
		}
		placement = &p
	}
	var dealID *uuid.UUID
	setDealID := in.DealID != nil
	if setDealID {
		dealID = *in.DealID
	}

	tag, err := s.pool.Exec(ctx, `
		UPDATE platform.storefront_banners SET
			headline = CASE WHEN $3::boolean THEN $4 ELSE headline END,
			subtext = CASE WHEN $5::boolean THEN $6 ELSE subtext END,
			cta_label = CASE WHEN $7::boolean THEN $8 ELSE cta_label END,
			cta_href = CASE WHEN $9::boolean THEN $10 ELSE cta_href END,
			placement = COALESCE($11, placement),
			deal_id = CASE WHEN $12::boolean THEN $13 ELSE deal_id END,
			sort_order = COALESCE($14, sort_order),
			active = COALESCE($15, active),
			updated_at = now()
		WHERE tenant_id = $1 AND id = $2`,
		tenantID, id,
		in.Headline != nil, nullIfBlankPtr(in.Headline),
		in.Subtext != nil, nullIfBlankPtr(in.Subtext),
		in.CTALabel != nil, nullIfBlankPtr(in.CTALabel),
		in.CTAHref != nil, nullIfBlankPtr(in.CTAHref),
		placement,
		setDealID, dealID,
		in.SortOrder, in.Active)
	if err != nil {
		return nil, err
	}
	if tag.RowsAffected() == 0 {
		return nil, fmt.Errorf("banner not found")
	}
	return s.getBanner(ctx, tenantID, id)
}

func (s *Service) DeleteBanner(ctx context.Context, tenantID, id uuid.UUID) error {
	key, err := s.bannerImageKey(ctx, id)
	if err == nil && key != "" && s.store != nil {
		_ = s.store.Delete(ctx, key)
	}
	tag, err := s.pool.Exec(ctx, `DELETE FROM platform.storefront_banners WHERE tenant_id = $1 AND id = $2`, tenantID, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("banner not found")
	}
	return nil
}

func (s *Service) getBanner(ctx context.Context, tenantID, id uuid.UUID) (*Banner, error) {
	row := s.pool.QueryRow(ctx, `SELECT `+bannerColumns+` `+bannerFrom+` WHERE b.tenant_id = $1 AND b.id = $2`, tenantID, id)
	b, err := scanBanner(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, fmt.Errorf("banner not found")
	}
	if err != nil {
		return nil, err
	}
	return &b, nil
}

func (s *Service) bannerImageKey(ctx context.Context, id uuid.UUID) (string, error) {
	var key *string
	err := s.pool.QueryRow(ctx, `SELECT image_object_key FROM platform.storefront_banners WHERE id = $1`, id).Scan(&key)
	return deref(key), err
}

// SaveBannerImage stores the banner slide image. Object storage is the store
// of record when configured; otherwise the bytes live in the row.
func (s *Service) SaveBannerImage(ctx context.Context, tenantID, id uuid.UUID, body []byte, contentType string) error {
	if len(body) == 0 {
		return errors.New("image file is empty")
	}
	if len(body) > maxImageBytes {
		return fmt.Errorf("image must be %d KB or smaller", maxImageBytes/1024)
	}
	detected, ok := sniffImage(body, contentType)
	if !ok {
		return errors.New("image must be a PNG, JPEG or WebP image")
	}
	var exists bool
	if err := s.pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM platform.storefront_banners WHERE tenant_id = $1 AND id = $2)`, tenantID, id).Scan(&exists); err != nil {
		return err
	}
	if !exists {
		return fmt.Errorf("banner not found")
	}

	key := ""
	var dbBytes []byte = body
	if s.store != nil {
		key = fmt.Sprintf("tenants/%s/storefront/banners/%s", tenantID, id)
		if err := s.store.Put(ctx, key, body, detected); err != nil {
			key = ""
		} else {
			dbBytes = nil
		}
	}

	_, err := s.pool.Exec(ctx, `
		UPDATE platform.storefront_banners
		SET image_object_key = $3, image_bytes = $4, image_content_type = $5, image_updated_at = now(), updated_at = now()
		WHERE tenant_id = $1 AND id = $2`,
		tenantID, id, nullIfBlank(key), dbBytes, detected)
	return err
}

func (s *Service) DeleteBannerImage(ctx context.Context, tenantID, id uuid.UUID) error {
	key, err := s.bannerImageKey(ctx, id)
	if err != nil {
		return err
	}
	if key != "" && s.store != nil {
		_ = s.store.Delete(ctx, key)
	}
	_, err = s.pool.Exec(ctx, `
		UPDATE platform.storefront_banners
		SET image_object_key = NULL, image_bytes = NULL, image_content_type = NULL, image_updated_at = NULL, updated_at = now()
		WHERE tenant_id = $1 AND id = $2`, tenantID, id)
	return err
}

// BannerImage returns the raw bytes for the public, unauthenticated image
// route — unlike receipts' logo, this is served directly as <img src>, never
// as a data URI.
func (s *Service) BannerImage(ctx context.Context, id uuid.UUID) ([]byte, string, error) {
	var key, contentType *string
	var raw []byte
	err := s.pool.QueryRow(ctx, `
		SELECT image_object_key, image_content_type, image_bytes
		FROM platform.storefront_banners WHERE id = $1`, id).Scan(&key, &contentType, &raw)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, "", fmt.Errorf("banner not found")
	}
	if err != nil {
		return nil, "", err
	}
	if len(raw) == 0 && key != nil && s.store != nil {
		if fetched, fetchErr := s.store.Get(ctx, *key); fetchErr == nil {
			raw = fetched
		}
	}
	if len(raw) == 0 {
		return nil, "", fmt.Errorf("banner has no image")
	}
	return raw, deref(contentType), nil
}

// Deal is a timed discount on one product variant. DealPrice is what
// internal/commerce.StartCheckout actually charges — never decorative.
type Deal struct {
	ID        uuid.UUID  `json:"id"`
	VariantID uuid.UUID  `json:"variant_id"`
	Title     string     `json:"title"`
	DealPrice float64    `json:"deal_price"`
	EndsAt    *time.Time `json:"ends_at,omitempty"`
	Active    bool       `json:"active"`
	SortOrder int        `json:"sort_order"`
	CreatedAt time.Time  `json:"created_at"`
	UpdatedAt time.Time  `json:"updated_at"`

	// Populated by ListDeals/ListActiveDeals for display convenience only.
	ProductID   uuid.UUID  `json:"product_id,omitempty"`
	ProductName string     `json:"product_name,omitempty"`
	SKU         string     `json:"sku,omitempty"`
	BasePrice   float64    `json:"base_price,omitempty"`
	HasImage    bool       `json:"has_image,omitempty"`
	ImageUpdated *time.Time `json:"image_updated_at,omitempty"`
}

const dealColumns = `d.id, d.variant_id, d.title, d.deal_price::float8, d.ends_at, d.active, d.sort_order, d.created_at, d.updated_at,
	p.id, p.name, v.sku, v.sell_price::float8,
	(p.image_object_key IS NOT NULL OR p.image_bytes IS NOT NULL), p.image_updated_at`

func scanDeal(row pgx.Row) (Deal, error) {
	var d Deal
	var title *string
	err := row.Scan(&d.ID, &d.VariantID, &title, &d.DealPrice, &d.EndsAt, &d.Active, &d.SortOrder, &d.CreatedAt, &d.UpdatedAt,
		&d.ProductID, &d.ProductName, &d.SKU, &d.BasePrice, &d.HasImage, &d.ImageUpdated)
	d.Title = deref(title)
	return d, err
}

func (s *Service) ListDeals(ctx context.Context, tenantID uuid.UUID) ([]Deal, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT `+dealColumns+`
		FROM platform.storefront_deals d
		JOIN inventory.product_variants v ON v.id = d.variant_id
		JOIN inventory.products p ON p.id = v.product_id
		WHERE d.tenant_id = $1
		ORDER BY d.sort_order, d.created_at`, tenantID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]Deal, 0)
	for rows.Next() {
		d, err := scanDeal(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, d)
	}
	return items, rows.Err()
}

// ListActiveDeals returns deals that are active and not yet expired.
func (s *Service) ListActiveDeals(ctx context.Context, tenantID uuid.UUID) ([]Deal, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT `+dealColumns+`
		FROM platform.storefront_deals d
		JOIN inventory.product_variants v ON v.id = d.variant_id
		JOIN inventory.products p ON p.id = v.product_id
		WHERE d.tenant_id = $1 AND d.active AND (d.ends_at IS NULL OR d.ends_at > now())
		ORDER BY d.sort_order, d.created_at`, tenantID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]Deal, 0)
	for rows.Next() {
		d, err := scanDeal(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, d)
	}
	return items, rows.Err()
}

// ActiveDealForVariant is the exact lookup internal/commerce.StartCheckout
// makes for every cart line — must stay cheap (backed by
// idx_storefront_deals_active_lookup).
func (s *Service) ActiveDealForVariant(ctx context.Context, tenantID, variantID uuid.UUID) (*Deal, bool, error) {
	var d Deal
	var title *string
	err := s.pool.QueryRow(ctx, `
		SELECT id, variant_id, title, deal_price::float8, ends_at, active, sort_order, created_at, updated_at
		FROM platform.storefront_deals
		WHERE tenant_id = $1 AND variant_id = $2 AND active AND (ends_at IS NULL OR ends_at > now())
		ORDER BY deal_price ASC
		LIMIT 1`, tenantID, variantID).
		Scan(&d.ID, &d.VariantID, &title, &d.DealPrice, &d.EndsAt, &d.Active, &d.SortOrder, &d.CreatedAt, &d.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	d.Title = deref(title)
	return &d, true, nil
}

type DealInput struct {
	VariantID *uuid.UUID
	Title     *string
	DealPrice *float64
	EndsAt    **time.Time
	Active    *bool
	SortOrder *int
}

func (s *Service) CreateDeal(ctx context.Context, tenantID uuid.UUID, in DealInput) (*Deal, error) {
	if in.VariantID == nil || *in.VariantID == uuid.Nil {
		return nil, fmt.Errorf("variant_id required")
	}
	if in.DealPrice == nil || *in.DealPrice <= 0 {
		return nil, fmt.Errorf("deal_price must be greater than zero")
	}
	var basePrice float64
	err := s.pool.QueryRow(ctx, `
		SELECT sell_price::float8 FROM inventory.product_variants WHERE tenant_id = $1 AND id = $2`,
		tenantID, *in.VariantID).Scan(&basePrice)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, fmt.Errorf("product variant not found")
	}
	if err != nil {
		return nil, err
	}
	if *in.DealPrice >= basePrice {
		return nil, fmt.Errorf("deal price must be lower than the current sell price (%.2f)", basePrice)
	}

	id := uuid.New()
	active := true
	if in.Active != nil {
		active = *in.Active
	}
	sortOrder := 0
	if in.SortOrder != nil {
		sortOrder = *in.SortOrder
	}
	var endsAt *time.Time
	if in.EndsAt != nil {
		endsAt = *in.EndsAt
	}
	_, err = s.pool.Exec(ctx, `
		INSERT INTO platform.storefront_deals (id, tenant_id, variant_id, title, deal_price, ends_at, active, sort_order)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8)`,
		id, tenantID, *in.VariantID, nullIfBlankPtr(in.Title), *in.DealPrice, endsAt, active, sortOrder)
	if err != nil {
		return nil, err
	}
	return s.getDeal(ctx, tenantID, id)
}

func (s *Service) UpdateDeal(ctx context.Context, tenantID, id uuid.UUID, in DealInput) (*Deal, error) {
	if in.DealPrice != nil {
		var variantID uuid.UUID
		if in.VariantID != nil {
			variantID = *in.VariantID
		} else {
			if err := s.pool.QueryRow(ctx, `SELECT variant_id FROM platform.storefront_deals WHERE tenant_id = $1 AND id = $2`, tenantID, id).Scan(&variantID); err != nil {
				if errors.Is(err, pgx.ErrNoRows) {
					return nil, fmt.Errorf("deal not found")
				}
				return nil, err
			}
		}
		var basePrice float64
		if err := s.pool.QueryRow(ctx, `SELECT sell_price::float8 FROM inventory.product_variants WHERE tenant_id = $1 AND id = $2`, tenantID, variantID).Scan(&basePrice); err != nil {
			return nil, err
		}
		if *in.DealPrice <= 0 || *in.DealPrice >= basePrice {
			return nil, fmt.Errorf("deal price must be lower than the current sell price (%.2f)", basePrice)
		}
	}

	var endsAt *time.Time
	setEndsAt := in.EndsAt != nil
	if setEndsAt {
		endsAt = *in.EndsAt
	}

	tag, err := s.pool.Exec(ctx, `
		UPDATE platform.storefront_deals SET
			variant_id = COALESCE($3, variant_id),
			title = CASE WHEN $4::boolean THEN $5 ELSE title END,
			deal_price = COALESCE($6, deal_price),
			ends_at = CASE WHEN $7::boolean THEN $8 ELSE ends_at END,
			active = COALESCE($9, active),
			sort_order = COALESCE($10, sort_order),
			updated_at = now()
		WHERE tenant_id = $1 AND id = $2`,
		tenantID, id,
		in.VariantID,
		in.Title != nil, nullIfBlankPtr(in.Title),
		in.DealPrice,
		setEndsAt, endsAt,
		in.Active, in.SortOrder)
	if err != nil {
		return nil, err
	}
	if tag.RowsAffected() == 0 {
		return nil, fmt.Errorf("deal not found")
	}
	return s.getDeal(ctx, tenantID, id)
}

func (s *Service) DeleteDeal(ctx context.Context, tenantID, id uuid.UUID) error {
	tag, err := s.pool.Exec(ctx, `DELETE FROM platform.storefront_deals WHERE tenant_id = $1 AND id = $2`, tenantID, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("deal not found")
	}
	return nil
}

func (s *Service) getDeal(ctx context.Context, tenantID, id uuid.UUID) (*Deal, error) {
	row := s.pool.QueryRow(ctx, `
		SELECT `+dealColumns+`
		FROM platform.storefront_deals d
		JOIN inventory.product_variants v ON v.id = d.variant_id
		JOIN inventory.products p ON p.id = v.product_id
		WHERE d.tenant_id = $1 AND d.id = $2`, tenantID, id)
	d, err := scanDeal(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, fmt.Errorf("deal not found")
	}
	if err != nil {
		return nil, err
	}
	return &d, nil
}

type Subscriber struct {
	ID        uuid.UUID `json:"id"`
	Email     string    `json:"email"`
	CreatedAt time.Time `json:"created_at"`
}

// Subscribe records a newsletter signup. It always succeeds on a duplicate
// email so the public endpoint never leaks whether an address already
// subscribed.
func (s *Service) Subscribe(ctx context.Context, tenantID uuid.UUID, email string) error {
	email = strings.ToLower(strings.TrimSpace(email))
	if _, err := mail.ParseAddress(email); err != nil {
		return fmt.Errorf("a valid email is required")
	}
	_, err := s.pool.Exec(ctx, `
		INSERT INTO platform.newsletter_subscribers (id, tenant_id, email)
		VALUES ($1, $2, $3)
		ON CONFLICT (tenant_id, lower(email)) DO NOTHING`,
		uuid.New(), tenantID, email)
	return err
}

func (s *Service) ListSubscribers(ctx context.Context, tenantID uuid.UUID) ([]Subscriber, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, email, created_at FROM platform.newsletter_subscribers
		WHERE tenant_id = $1 ORDER BY created_at DESC`, tenantID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]Subscriber, 0)
	for rows.Next() {
		var sub Subscriber
		if err := rows.Scan(&sub.ID, &sub.Email, &sub.CreatedAt); err != nil {
			return nil, err
		}
		items = append(items, sub)
	}
	return items, rows.Err()
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

func nullIfBlankPtr(v *string) *string {
	if v == nil {
		return nil
	}
	return nullIfBlank(*v)
}

func applyString(dst *string, src *string) {
	if src != nil {
		*dst = strings.TrimSpace(*src)
	}
}

func applyDeref(dst *string, src *string) {
	if src != nil {
		*dst = *src
	}
}
