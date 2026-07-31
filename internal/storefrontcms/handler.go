package storefrontcms

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/techlane/techlane/packages/pkg/apierrors"
	"github.com/techlane/techlane/packages/pkg/authz"
	"github.com/techlane/techlane/packages/pkg/httpx"
)

type Handler struct {
	svc *Service
}

func NewHandler(svc *Service) *Handler { return &Handler{svc: svc} }

func (h *Handler) Register(mux *http.ServeMux, auth func(http.Handler) http.Handler) {
	mux.Handle("GET /storefront/settings", auth(http.HandlerFunc(h.getSettings)))
	mux.Handle("PUT /storefront/settings", auth(http.HandlerFunc(h.putSettings)))
	mux.Handle("POST /storefront/settings/logo", auth(http.HandlerFunc(h.uploadLogo)))
	mux.Handle("DELETE /storefront/settings/logo", auth(http.HandlerFunc(h.deleteLogo)))

	mux.Handle("GET /storefront/banners", auth(http.HandlerFunc(h.listBanners)))
	mux.Handle("POST /storefront/banners", auth(http.HandlerFunc(h.createBanner)))
	mux.Handle("PUT /storefront/banners/{id}", auth(http.HandlerFunc(h.updateBanner)))
	mux.Handle("DELETE /storefront/banners/{id}", auth(http.HandlerFunc(h.deleteBanner)))
	mux.Handle("POST /storefront/banners/{id}/image", auth(http.HandlerFunc(h.uploadBannerImage)))
	mux.Handle("DELETE /storefront/banners/{id}/image", auth(http.HandlerFunc(h.deleteBannerImage)))

	mux.Handle("GET /storefront/deals", auth(http.HandlerFunc(h.listDeals)))
	mux.Handle("POST /storefront/deals", auth(http.HandlerFunc(h.createDeal)))
	mux.Handle("PUT /storefront/deals/{id}", auth(http.HandlerFunc(h.updateDeal)))
	mux.Handle("DELETE /storefront/deals/{id}", auth(http.HandlerFunc(h.deleteDeal)))

	mux.Handle("GET /storefront/subscribers", auth(http.HandlerFunc(h.listSubscribers)))

	mux.Handle("GET /storefront/reviews", auth(http.HandlerFunc(h.listAllReviews)))
	mux.Handle("POST /storefront/reviews/{id}/status", auth(http.HandlerFunc(h.setReviewStatus)))

	// Unauthenticated: content-addressed by unguessable UUID, no tenant
	// resolution needed — same trust model as commerce.CollectInBranch's
	// collection-code lookup.
	mux.HandleFunc("GET /storefront/public/banners/{id}/image", h.publicBannerImage)
	mux.HandleFunc("GET /storefront/public/logo/{tenant_id}", h.publicLogo)
}

// canManage mirrors internal/receipts' rule: owners (or a wildcard
// permission) control anything customer-facing and branded.
func canManage(claims *authz.Claims) bool {
	if claims == nil {
		return false
	}
	if claims.HasPermission("*") {
		return true
	}
	for _, role := range claims.Roles {
		if role == "owner" {
			return true
		}
	}
	return false
}

func (h *Handler) claims(w http.ResponseWriter, r *http.Request) (*authz.Claims, bool) {
	claims, ok := authz.FromContext(r.Context())
	if !ok {
		apierrors.Write(w, http.StatusUnauthorized, "UNAUTHORIZED", "missing claims", httpx.CorrelationID(r.Context()))
		return nil, false
	}
	if !canManage(claims) {
		apierrors.Write(w, http.StatusForbidden, "FORBIDDEN", "owner required", httpx.CorrelationID(r.Context()))
		return nil, false
	}
	return claims, true
}

func writeErr(w http.ResponseWriter, r *http.Request, err error) {
	status := http.StatusInternalServerError
	if strings.Contains(err.Error(), "not found") {
		status = http.StatusNotFound
	} else if strings.Contains(err.Error(), "required") || strings.Contains(err.Error(), "must be") {
		status = http.StatusBadRequest
	}
	apierrors.Write(w, status, "STOREFRONT_FAILED", err.Error(), httpx.CorrelationID(r.Context()))
}

func (h *Handler) getSettings(w http.ResponseWriter, r *http.Request) {
	claims, ok := h.claims(w, r)
	if !ok {
		return
	}
	set, err := h.svc.GetSettings(r.Context(), claims.TenantID)
	if err != nil {
		writeErr(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, set)
}

func (h *Handler) putSettings(w http.ResponseWriter, r *http.Request) {
	claims, ok := h.claims(w, r)
	if !ok {
		return
	}
	var req struct {
		ShopDisplayName *string `json:"shop_display_name"`
		PageTitle       *string `json:"page_title"`
		ColorPrimary    *string `json:"color_primary"`
		ColorSecondary  *string `json:"color_secondary"`
		ColorAccent     *string `json:"color_accent"`

		TopbarHelpHref    *string `json:"topbar_help_href"`
		TopbarSupportHref *string `json:"topbar_support_href"`
		TopbarContactHref *string `json:"topbar_contact_href"`
		TopbarPhoneLabel  *string `json:"topbar_phone_label"`
		HeaderPromoText   *string `json:"header_promo_text"`

		ShowFeatured    *bool `json:"show_featured"`
		ShowNewArrivals *bool `json:"show_new_arrivals"`
		ShowBestsellers *bool `json:"show_bestsellers"`
		ShowDeals       *bool `json:"show_deals"`
		ShowMostViewed  *bool `json:"show_most_viewed"`

		HeroHeadline *string `json:"hero_headline"`
		HeroSubtext  *string `json:"hero_subtext"`
		HeroCTALabel *string `json:"hero_cta_label"`
		HeroCTAHref  *string `json:"hero_cta_href"`

		NewsletterHeadline *string `json:"newsletter_headline"`
		NewsletterSubtext  *string `json:"newsletter_subtext"`

		FooterTagline   *string `json:"footer_tagline"`
		SocialFacebook  *string `json:"social_facebook"`
		SocialInstagram *string `json:"social_instagram"`
		SocialTwitter   *string `json:"social_twitter"`
		SocialTikTok    *string `json:"social_tiktok"`
		ContactPhone    *string `json:"contact_phone"`
		ContactEmail    *string `json:"contact_email"`
		BusinessHours   *string `json:"business_hours"`
		AppStoreURL     *string `json:"app_store_url"`
		PlayStoreURL    *string `json:"play_store_url"`

		EnabledCurrencies *string `json:"enabled_currencies"`

		TrustBadge1Title   *string `json:"trust_badge_1_title"`
		TrustBadge1Subtext *string `json:"trust_badge_1_subtext"`
		TrustBadge2Title   *string `json:"trust_badge_2_title"`
		TrustBadge2Subtext *string `json:"trust_badge_2_subtext"`
		TrustBadge3Title   *string `json:"trust_badge_3_title"`
		TrustBadge3Subtext *string `json:"trust_badge_3_subtext"`
		TrustBadge4Title   *string `json:"trust_badge_4_title"`
		TrustBadge4Subtext *string `json:"trust_badge_4_subtext"`

		PayLabelSTK     *string `json:"pay_label_stk"`
		PayLabelPaybill *string `json:"pay_label_paybill"`
		PayLabelCash    *string `json:"pay_label_cash"`
		PayHintSTK      *string `json:"pay_hint_stk"`
		PayHintPaybill  *string `json:"pay_hint_paybill"`
		PayHintCash     *string `json:"pay_hint_cash"`
		PayCTASTK       *string `json:"pay_cta_stk"`
		PayCTAPaybill   *string `json:"pay_cta_paybill"`
		PayCTACash      *string `json:"pay_cta_cash"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		apierrors.Write(w, http.StatusBadRequest, "BAD_REQUEST", "invalid body", httpx.CorrelationID(r.Context()))
		return
	}
	set, err := h.svc.UpsertSettings(r.Context(), claims.TenantID, UpsertSettingsInput{
		ShopDisplayName: req.ShopDisplayName, PageTitle: req.PageTitle,
		ColorPrimary: req.ColorPrimary, ColorSecondary: req.ColorSecondary, ColorAccent: req.ColorAccent,
		TopbarHelpHref: req.TopbarHelpHref, TopbarSupportHref: req.TopbarSupportHref, TopbarContactHref: req.TopbarContactHref,
		TopbarPhoneLabel: req.TopbarPhoneLabel, HeaderPromoText: req.HeaderPromoText,
		ShowFeatured: req.ShowFeatured, ShowNewArrivals: req.ShowNewArrivals, ShowBestsellers: req.ShowBestsellers,
		ShowDeals: req.ShowDeals, ShowMostViewed: req.ShowMostViewed,
		HeroHeadline: req.HeroHeadline, HeroSubtext: req.HeroSubtext, HeroCTALabel: req.HeroCTALabel, HeroCTAHref: req.HeroCTAHref,
		NewsletterHeadline: req.NewsletterHeadline, NewsletterSubtext: req.NewsletterSubtext,
		FooterTagline: req.FooterTagline, SocialFacebook: req.SocialFacebook, SocialInstagram: req.SocialInstagram,
		SocialTwitter: req.SocialTwitter, SocialTikTok: req.SocialTikTok,
		ContactPhone: req.ContactPhone, ContactEmail: req.ContactEmail, BusinessHours: req.BusinessHours,
		AppStoreURL: req.AppStoreURL, PlayStoreURL: req.PlayStoreURL,
		EnabledCurrencies: req.EnabledCurrencies,
		TrustBadge1Title:  req.TrustBadge1Title, TrustBadge1Subtext: req.TrustBadge1Subtext,
		TrustBadge2Title: req.TrustBadge2Title, TrustBadge2Subtext: req.TrustBadge2Subtext,
		TrustBadge3Title: req.TrustBadge3Title, TrustBadge3Subtext: req.TrustBadge3Subtext,
		TrustBadge4Title: req.TrustBadge4Title, TrustBadge4Subtext: req.TrustBadge4Subtext,
		PayLabelSTK: req.PayLabelSTK, PayLabelPaybill: req.PayLabelPaybill, PayLabelCash: req.PayLabelCash,
		PayHintSTK: req.PayHintSTK, PayHintPaybill: req.PayHintPaybill, PayHintCash: req.PayHintCash,
		PayCTASTK: req.PayCTASTK, PayCTAPaybill: req.PayCTAPaybill, PayCTACash: req.PayCTACash,
	}, claims.UserID)
	if err != nil {
		writeErr(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, set)
}

func (h *Handler) uploadLogo(w http.ResponseWriter, r *http.Request) {
	claims, ok := h.claims(w, r)
	if !ok {
		return
	}
	body, contentType, err := readImageUpload(r)
	if err != nil {
		apierrors.Write(w, http.StatusBadRequest, "BAD_REQUEST", err.Error(), httpx.CorrelationID(r.Context()))
		return
	}
	if err := h.svc.SaveLogo(r.Context(), claims.TenantID, body, contentType, claims.UserID); err != nil {
		writeErr(w, r, err)
		return
	}
	set, err := h.svc.GetSettings(r.Context(), claims.TenantID)
	if err != nil {
		writeErr(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, set)
}

func (h *Handler) deleteLogo(w http.ResponseWriter, r *http.Request) {
	claims, ok := h.claims(w, r)
	if !ok {
		return
	}
	if err := h.svc.DeleteLogo(r.Context(), claims.TenantID, claims.UserID); err != nil {
		writeErr(w, r, err)
		return
	}
	set, err := h.svc.GetSettings(r.Context(), claims.TenantID)
	if err != nil {
		writeErr(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, set)
}

func (h *Handler) publicLogo(w http.ResponseWriter, r *http.Request) {
	tenantID, err := uuid.Parse(r.PathValue("tenant_id"))
	if err != nil {
		apierrors.Write(w, http.StatusBadRequest, "BAD_REQUEST", "invalid tenant id", httpx.CorrelationID(r.Context()))
		return
	}
	body, contentType, err := h.svc.LogoImage(r.Context(), tenantID)
	if err != nil {
		apierrors.Write(w, http.StatusNotFound, "NOT_FOUND", "logo not found", httpx.CorrelationID(r.Context()))
		return
	}
	if contentType == "" {
		contentType = "image/png"
	}
	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Cache-Control", "public, max-age=3600")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(body)
}

func (h *Handler) listBanners(w http.ResponseWriter, r *http.Request) {
	claims, ok := h.claims(w, r)
	if !ok {
		return
	}
	items, err := h.svc.ListBanners(r.Context(), claims.TenantID, false)
	if err != nil {
		writeErr(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"items": items})
}

func decodeBannerInput(r *http.Request) (BannerInput, error) {
	var req struct {
		Headline    *string    `json:"headline"`
		Subtext     *string    `json:"subtext"`
		CTALabel    *string    `json:"cta_label"`
		CTAHref     *string    `json:"cta_href"`
		Placement   *string    `json:"placement"`
		DealID      *uuid.UUID `json:"deal_id"`
		ClearDealID bool       `json:"clear_deal_id"`
		SortOrder   *int       `json:"sort_order"`
		Active      *bool      `json:"active"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return BannerInput{}, err
	}
	in := BannerInput{
		Headline: req.Headline, Subtext: req.Subtext, CTALabel: req.CTALabel, CTAHref: req.CTAHref,
		Placement: req.Placement, SortOrder: req.SortOrder, Active: req.Active,
	}
	if req.ClearDealID {
		var nilID *uuid.UUID
		in.DealID = &nilID
	} else if req.DealID != nil {
		in.DealID = &req.DealID
	}
	return in, nil
}

func (h *Handler) createBanner(w http.ResponseWriter, r *http.Request) {
	claims, ok := h.claims(w, r)
	if !ok {
		return
	}
	in, err := decodeBannerInput(r)
	if err != nil {
		apierrors.Write(w, http.StatusBadRequest, "BAD_REQUEST", "invalid body", httpx.CorrelationID(r.Context()))
		return
	}
	banner, err := h.svc.CreateBanner(r.Context(), claims.TenantID, in)
	if err != nil {
		writeErr(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusCreated, banner)
}

func (h *Handler) updateBanner(w http.ResponseWriter, r *http.Request) {
	claims, ok := h.claims(w, r)
	if !ok {
		return
	}
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		apierrors.Write(w, http.StatusBadRequest, "BAD_REQUEST", "invalid id", httpx.CorrelationID(r.Context()))
		return
	}
	in, err := decodeBannerInput(r)
	if err != nil {
		apierrors.Write(w, http.StatusBadRequest, "BAD_REQUEST", "invalid body", httpx.CorrelationID(r.Context()))
		return
	}
	banner, err := h.svc.UpdateBanner(r.Context(), claims.TenantID, id, in)
	if err != nil {
		writeErr(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, banner)
}

func (h *Handler) deleteBanner(w http.ResponseWriter, r *http.Request) {
	claims, ok := h.claims(w, r)
	if !ok {
		return
	}
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		apierrors.Write(w, http.StatusBadRequest, "BAD_REQUEST", "invalid id", httpx.CorrelationID(r.Context()))
		return
	}
	if err := h.svc.DeleteBanner(r.Context(), claims.TenantID, id); err != nil {
		writeErr(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

const maxImageUploadBytes = maxImageBytes

func readImageUpload(r *http.Request) ([]byte, string, error) {
	contentType := r.Header.Get("Content-Type")
	if strings.HasPrefix(contentType, "multipart/form-data") {
		if err := r.ParseMultipartForm(maxImageUploadBytes + 8192); err != nil {
			return nil, "", errors.New("could not read the uploaded file")
		}
		file, header, err := r.FormFile("file")
		if err != nil {
			return nil, "", errors.New("attach the image as the 'file' field")
		}
		defer file.Close()
		body, err := io.ReadAll(io.LimitReader(file, maxImageUploadBytes+1))
		if err != nil {
			return nil, "", errors.New("could not read the uploaded file")
		}
		declared := ""
		if header != nil {
			declared = header.Header.Get("Content-Type")
		}
		return body, declared, nil
	}
	body, err := io.ReadAll(io.LimitReader(r.Body, maxImageUploadBytes+1))
	if err != nil {
		return nil, "", errors.New("could not read the uploaded file")
	}
	return body, contentType, nil
}

func (h *Handler) uploadBannerImage(w http.ResponseWriter, r *http.Request) {
	claims, ok := h.claims(w, r)
	if !ok {
		return
	}
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		apierrors.Write(w, http.StatusBadRequest, "BAD_REQUEST", "invalid id", httpx.CorrelationID(r.Context()))
		return
	}
	body, contentType, err := readImageUpload(r)
	if err != nil {
		apierrors.Write(w, http.StatusBadRequest, "BAD_REQUEST", err.Error(), httpx.CorrelationID(r.Context()))
		return
	}
	if err := h.svc.SaveBannerImage(r.Context(), claims.TenantID, id, body, contentType); err != nil {
		writeErr(w, r, err)
		return
	}
	banner, err := h.svc.getBanner(r.Context(), claims.TenantID, id)
	if err != nil {
		writeErr(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, banner)
}

func (h *Handler) deleteBannerImage(w http.ResponseWriter, r *http.Request) {
	claims, ok := h.claims(w, r)
	if !ok {
		return
	}
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		apierrors.Write(w, http.StatusBadRequest, "BAD_REQUEST", "invalid id", httpx.CorrelationID(r.Context()))
		return
	}
	if err := h.svc.DeleteBannerImage(r.Context(), claims.TenantID, id); err != nil {
		writeErr(w, r, err)
		return
	}
	banner, err := h.svc.getBanner(r.Context(), claims.TenantID, id)
	if err != nil {
		writeErr(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, banner)
}

func (h *Handler) publicBannerImage(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		apierrors.Write(w, http.StatusBadRequest, "BAD_REQUEST", "invalid id", httpx.CorrelationID(r.Context()))
		return
	}
	body, contentType, err := h.svc.BannerImage(r.Context(), id)
	if err != nil {
		apierrors.Write(w, http.StatusNotFound, "NOT_FOUND", "image not found", httpx.CorrelationID(r.Context()))
		return
	}
	if contentType == "" {
		contentType = "image/png"
	}
	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Cache-Control", "public, max-age=3600")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(body)
}

func (h *Handler) listDeals(w http.ResponseWriter, r *http.Request) {
	claims, ok := h.claims(w, r)
	if !ok {
		return
	}
	items, err := h.svc.ListDeals(r.Context(), claims.TenantID)
	if err != nil {
		writeErr(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"items": items})
}

func decodeDealInput(r *http.Request) (DealInput, error) {
	var req struct {
		VariantID *uuid.UUID `json:"variant_id"`
		Title     *string    `json:"title"`
		DealPrice *float64   `json:"deal_price"`
		EndsAt    *string    `json:"ends_at"`
		ClearEnds bool       `json:"clear_ends_at"`
		Active    *bool      `json:"active"`
		SortOrder *int       `json:"sort_order"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return DealInput{}, err
	}
	in := DealInput{
		VariantID: req.VariantID, Title: req.Title, DealPrice: req.DealPrice,
		Active: req.Active, SortOrder: req.SortOrder,
	}
	if req.ClearEnds {
		var nilTime *time.Time
		in.EndsAt = &nilTime
	} else if req.EndsAt != nil && *req.EndsAt != "" {
		parsed, err := time.Parse(time.RFC3339, *req.EndsAt)
		if err != nil {
			return DealInput{}, errors.New("ends_at must be an RFC3339 timestamp")
		}
		parsedPtr := &parsed
		in.EndsAt = &parsedPtr
	}
	return in, nil
}

func (h *Handler) createDeal(w http.ResponseWriter, r *http.Request) {
	claims, ok := h.claims(w, r)
	if !ok {
		return
	}
	in, err := decodeDealInput(r)
	if err != nil {
		apierrors.Write(w, http.StatusBadRequest, "BAD_REQUEST", err.Error(), httpx.CorrelationID(r.Context()))
		return
	}
	deal, err := h.svc.CreateDeal(r.Context(), claims.TenantID, in)
	if err != nil {
		writeErr(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusCreated, deal)
}

func (h *Handler) updateDeal(w http.ResponseWriter, r *http.Request) {
	claims, ok := h.claims(w, r)
	if !ok {
		return
	}
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		apierrors.Write(w, http.StatusBadRequest, "BAD_REQUEST", "invalid id", httpx.CorrelationID(r.Context()))
		return
	}
	in, err := decodeDealInput(r)
	if err != nil {
		apierrors.Write(w, http.StatusBadRequest, "BAD_REQUEST", err.Error(), httpx.CorrelationID(r.Context()))
		return
	}
	deal, err := h.svc.UpdateDeal(r.Context(), claims.TenantID, id, in)
	if err != nil {
		writeErr(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, deal)
}

func (h *Handler) deleteDeal(w http.ResponseWriter, r *http.Request) {
	claims, ok := h.claims(w, r)
	if !ok {
		return
	}
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		apierrors.Write(w, http.StatusBadRequest, "BAD_REQUEST", "invalid id", httpx.CorrelationID(r.Context()))
		return
	}
	if err := h.svc.DeleteDeal(r.Context(), claims.TenantID, id); err != nil {
		writeErr(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

func (h *Handler) listSubscribers(w http.ResponseWriter, r *http.Request) {
	claims, ok := h.claims(w, r)
	if !ok {
		return
	}
	items, err := h.svc.ListSubscribers(r.Context(), claims.TenantID)
	if err != nil {
		writeErr(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"items": items})
}

func (h *Handler) listAllReviews(w http.ResponseWriter, r *http.Request) {
	claims, ok := h.claims(w, r)
	if !ok {
		return
	}
	items, err := h.svc.ListAllReviews(r.Context(), claims.TenantID, r.URL.Query().Get("status"))
	if err != nil {
		writeErr(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"items": items})
}

func (h *Handler) setReviewStatus(w http.ResponseWriter, r *http.Request) {
	claims, ok := h.claims(w, r)
	if !ok {
		return
	}
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		apierrors.Write(w, http.StatusBadRequest, "BAD_REQUEST", "invalid id", httpx.CorrelationID(r.Context()))
		return
	}
	var req struct {
		Status string `json:"status"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		apierrors.Write(w, http.StatusBadRequest, "BAD_REQUEST", "invalid body", httpx.CorrelationID(r.Context()))
		return
	}
	if err := h.svc.SetReviewStatus(r.Context(), claims.TenantID, id, req.Status); err != nil {
		writeErr(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]string{"status": "updated"})
}
