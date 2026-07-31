package receipts

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"

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
	mux.Handle("GET /receipt-settings", auth(http.HandlerFunc(h.getSettings)))
	mux.Handle("PUT /receipt-settings", auth(http.HandlerFunc(h.putSettings)))
	mux.Handle("POST /receipt-settings/logo", auth(http.HandlerFunc(h.uploadLogo)))
	mux.Handle("DELETE /receipt-settings/logo", auth(http.HandlerFunc(h.deleteLogo)))
	mux.Handle("GET /receipt-settings/preview", auth(http.HandlerFunc(h.preview)))
	mux.Handle("POST /receipt-settings/preview", auth(http.HandlerFunc(h.previewDraft)))
}

// canManage mirrors the SMS template rule: owners (or a wildcard permission)
// control anything customer-facing and branded.
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

func (h *Handler) claims(w http.ResponseWriter, r *http.Request, requireOwner bool) (*authz.Claims, bool) {
	claims, ok := authz.FromContext(r.Context())
	if !ok {
		apierrors.Write(w, http.StatusUnauthorized, "UNAUTHORIZED", "missing claims", httpx.CorrelationID(r.Context()))
		return nil, false
	}
	if requireOwner && !canManage(claims) {
		apierrors.Write(w, http.StatusForbidden, "FORBIDDEN", "owner required", httpx.CorrelationID(r.Context()))
		return nil, false
	}
	return claims, true
}

func (h *Handler) getSettings(w http.ResponseWriter, r *http.Request) {
	claims, ok := h.claims(w, r, true)
	if !ok {
		return
	}
	set, err := h.svc.GetSettings(r.Context(), claims.TenantID)
	if err != nil {
		apierrors.Write(w, http.StatusInternalServerError, "INTERNAL", err.Error(), httpx.CorrelationID(r.Context()))
		return
	}
	httpx.JSON(w, http.StatusOK, set)
}

func (h *Handler) putSettings(w http.ResponseWriter, r *http.Request) {
	claims, ok := h.claims(w, r, true)
	if !ok {
		return
	}
	var req struct {
		HeaderNote  *string `json:"header_note"`
		Phone       *string `json:"phone"`
		Email       *string `json:"email"`
		Website     *string `json:"website"`
		ShowLogo    *bool   `json:"show_logo"`
		ShowAddress *bool   `json:"show_address"`
		ShowTIN     *bool   `json:"show_tin"`

		ThankYouText *string `json:"thank_you_text"`
		FooterText   *string `json:"footer_text"`
		WarrantyText *string `json:"warranty_text"`

		ShowVATBreakdown *bool `json:"show_vat_breakdown"`
		ShowIMEI         *bool `json:"show_imei"`
		ShowPayments     *bool `json:"show_payments"`
		ShowBalance      *bool `json:"show_balance"`
		ShowServedBy     *bool `json:"show_served_by"`

		DefaultPaper *string `json:"default_paper"`
		NumberPrefix *string `json:"number_prefix"`
		NextNumber   *int64  `json:"next_number"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		apierrors.Write(w, http.StatusBadRequest, "BAD_REQUEST", "invalid json", httpx.CorrelationID(r.Context()))
		return
	}
	set, err := h.svc.UpsertSettings(r.Context(), claims.TenantID, UpsertSettingsInput(req), claims.UserID)
	if err != nil {
		apierrors.Write(w, http.StatusBadRequest, "BAD_REQUEST", err.Error(), httpx.CorrelationID(r.Context()))
		return
	}
	httpx.JSON(w, http.StatusOK, set)
}

func (h *Handler) uploadLogo(w http.ResponseWriter, r *http.Request) {
	claims, ok := h.claims(w, r, true)
	if !ok {
		return
	}
	body, contentType, err := readUpload(r)
	if err != nil {
		apierrors.Write(w, http.StatusBadRequest, "BAD_REQUEST", err.Error(), httpx.CorrelationID(r.Context()))
		return
	}
	if err := h.svc.SaveLogo(r.Context(), claims.TenantID, body, contentType); err != nil {
		apierrors.Write(w, http.StatusBadRequest, "BAD_REQUEST", err.Error(), httpx.CorrelationID(r.Context()))
		return
	}
	set, err := h.svc.GetSettings(r.Context(), claims.TenantID)
	if err != nil {
		apierrors.Write(w, http.StatusInternalServerError, "INTERNAL", err.Error(), httpx.CorrelationID(r.Context()))
		return
	}
	httpx.JSON(w, http.StatusOK, set)
}

func readUpload(r *http.Request) ([]byte, string, error) {
	contentType := r.Header.Get("Content-Type")
	if strings.HasPrefix(contentType, "multipart/form-data") {
		if err := r.ParseMultipartForm(maxLogoBytes + 8192); err != nil {
			return nil, "", errors.New("could not read the uploaded file")
		}
		file, header, err := r.FormFile("file")
		if err != nil {
			return nil, "", errors.New("attach the image as the 'file' field")
		}
		defer file.Close()
		body, err := io.ReadAll(io.LimitReader(file, maxLogoBytes+1))
		if err != nil {
			return nil, "", errors.New("could not read the uploaded file")
		}
		declared := ""
		if header != nil {
			declared = header.Header.Get("Content-Type")
		}
		return body, declared, nil
	}
	body, err := io.ReadAll(io.LimitReader(r.Body, maxLogoBytes+1))
	if err != nil {
		return nil, "", errors.New("could not read the uploaded file")
	}
	return body, contentType, nil
}

func (h *Handler) deleteLogo(w http.ResponseWriter, r *http.Request) {
	claims, ok := h.claims(w, r, true)
	if !ok {
		return
	}
	if err := h.svc.DeleteLogo(r.Context(), claims.TenantID); err != nil {
		apierrors.Write(w, http.StatusInternalServerError, "INTERNAL", err.Error(), httpx.CorrelationID(r.Context()))
		return
	}
	set, err := h.svc.GetSettings(r.Context(), claims.TenantID)
	if err != nil {
		apierrors.Write(w, http.StatusInternalServerError, "INTERNAL", err.Error(), httpx.CorrelationID(r.Context()))
		return
	}
	httpx.JSON(w, http.StatusOK, set)
}

// preview renders a sample document with the tenant's saved branding. It never
// touches the receipt number series.
func (h *Handler) preview(w http.ResponseWriter, r *http.Request) {
	claims, ok := h.claims(w, r, true)
	if !ok {
		return
	}
	set, err := h.svc.GetSettings(r.Context(), claims.TenantID)
	if err != nil {
		apierrors.Write(w, http.StatusInternalServerError, "INTERNAL", err.Error(), httpx.CorrelationID(r.Context()))
		return
	}
	h.writePreview(w, r, claims.TenantID, set)
}

// previewDraft renders unsaved edits so the settings page can show the result
// of a change before the owner commits to it.
func (h *Handler) previewDraft(w http.ResponseWriter, r *http.Request) {
	claims, ok := h.claims(w, r, true)
	if !ok {
		return
	}
	set, err := h.svc.GetSettings(r.Context(), claims.TenantID)
	if err != nil {
		apierrors.Write(w, http.StatusInternalServerError, "INTERNAL", err.Error(), httpx.CorrelationID(r.Context()))
		return
	}
	// Decoding onto the saved settings means omitted fields keep their value.
	if err := json.NewDecoder(io.LimitReader(r.Body, 64*1024)).Decode(&set); err != nil {
		apierrors.Write(w, http.StatusBadRequest, "BAD_REQUEST", "invalid json", httpx.CorrelationID(r.Context()))
		return
	}
	set.DefaultPaper = NormalizePaper(set.DefaultPaper, PaperThermal80)
	h.writePreview(w, r, claims.TenantID, set)
}

func (h *Handler) writePreview(w http.ResponseWriter, r *http.Request, tenantID uuid.UUID, set Settings) {
	shop := h.svc.LoadShop(r.Context(), tenantID, set)
	rate, inclusive := h.svc.VATProfile(r.Context(), tenantID)
	doc := SampleDocument(NormalizeKind(r.URL.Query().Get("kind")), h.svc.Currency(r.Context(), tenantID), rate, inclusive)
	doc.Number = FormatNumber(set.NumberPrefix, set.NextNumber)

	paper := NormalizePaper(r.URL.Query().Get("paper"), set.DefaultPaper)
	SetPrintableHTMLHeaders(w)
	w.Header().Set("Cache-Control", "no-store, no-transform")
	_, _ = w.Write([]byte(RenderHTML(shop, doc, set, paper)))
}
