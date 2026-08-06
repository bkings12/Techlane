package wifi

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/google/uuid"
	"github.com/techlane/techlane/internal/receipts"
	"github.com/techlane/techlane/packages/pkg/apierrors"
	"github.com/techlane/techlane/packages/pkg/authz"
	"github.com/techlane/techlane/packages/pkg/httpx"
)

type Handler struct {
	svc      *Service
	receipts *receipts.Service
}

func NewHandler(svc *Service) *Handler {
	return &Handler{svc: svc}
}

func (h *Handler) SetReceiptRenderer(r *receipts.Service) { h.receipts = r }

func (h *Handler) Register(mux *http.ServeMux, auth func(http.Handler) http.Handler) {
	mux.Handle("GET /wifi/settings", auth(http.HandlerFunc(h.getSettings)))
	mux.Handle("PUT /wifi/settings", auth(http.HandlerFunc(h.putSettings)))
	mux.Handle("POST /wifi/vouchers", auth(http.HandlerFunc(h.issueVoucher)))
	mux.Handle("GET /wifi/vouchers/{id}", auth(http.HandlerFunc(h.getVoucher)))
	mux.Handle("GET /wifi/vouchers/{id}/slip.html", auth(http.HandlerFunc(h.slipHTML)))
}

func (h *Handler) getSettings(w http.ResponseWriter, r *http.Request) {
	claims, ok := authz.FromContext(r.Context())
	if !ok {
		apierrors.Write(w, http.StatusUnauthorized, "UNAUTHORIZED", "missing claims", httpx.CorrelationID(r.Context()))
		return
	}
	out, err := h.svc.GetSettings(r.Context(), claims.TenantID)
	if err != nil {
		apierrors.Write(w, http.StatusInternalServerError, "INTERNAL", err.Error(), httpx.CorrelationID(r.Context()))
		return
	}
	httpx.JSON(w, http.StatusOK, out)
}

func (h *Handler) putSettings(w http.ResponseWriter, r *http.Request) {
	claims, ok := authz.FromContext(r.Context())
	if !ok {
		apierrors.Write(w, http.StatusUnauthorized, "UNAUTHORIZED", "missing claims", httpx.CorrelationID(r.Context()))
		return
	}
	if !hasOwnerOrManager(claims) {
		apierrors.Write(w, http.StatusForbidden, "FORBIDDEN", "owner or manager required", httpx.CorrelationID(r.Context()))
		return
	}
	var body struct {
		Enabled             *bool   `json:"enabled"`
		APIBaseURL          *string `json:"api_base_url"`
		APIKey              *string `json:"api_key"`
		SiteID              *string `json:"site_id"`
		PackageID           *string `json:"package_id"`
		DefaultDurationMins *int    `json:"default_duration_mins"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		apierrors.Write(w, http.StatusBadRequest, "BAD_REQUEST", "invalid JSON body", httpx.CorrelationID(r.Context()))
		return
	}
	in := UpsertSettingsInput{
		Enabled:             body.Enabled,
		APIBaseURL:          body.APIBaseURL,
		APIKey:              body.APIKey,
		DefaultDurationMins: body.DefaultDurationMins,
	}
	if body.SiteID != nil {
		if strings.TrimSpace(*body.SiteID) == "" {
			in.ClearSiteID = true
		} else if id, err := uuid.Parse(strings.TrimSpace(*body.SiteID)); err == nil {
			in.SiteID = &id
		} else {
			apierrors.Write(w, http.StatusBadRequest, "BAD_REQUEST", "invalid site_id", httpx.CorrelationID(r.Context()))
			return
		}
	}
	if body.PackageID != nil {
		if strings.TrimSpace(*body.PackageID) == "" {
			in.ClearPackageID = true
		} else if id, err := uuid.Parse(strings.TrimSpace(*body.PackageID)); err == nil {
			in.PackageID = &id
		} else {
			apierrors.Write(w, http.StatusBadRequest, "BAD_REQUEST", "invalid package_id", httpx.CorrelationID(r.Context()))
			return
		}
	}
	out, err := h.svc.UpsertSettings(r.Context(), claims.TenantID, in)
	if err != nil {
		apierrors.Write(w, http.StatusBadRequest, "BAD_REQUEST", err.Error(), httpx.CorrelationID(r.Context()))
		return
	}
	httpx.JSON(w, http.StatusOK, out)
}

func (h *Handler) issueVoucher(w http.ResponseWriter, r *http.Request) {
	claims, ok := authz.FromContext(r.Context())
	if !ok {
		apierrors.Write(w, http.StatusUnauthorized, "UNAUTHORIZED", "missing claims", httpx.CorrelationID(r.Context()))
		return
	}
	var body struct {
		DurationMins int     `json:"duration_mins"`
		Phone        string  `json:"phone"`
		RepairID     *string `json:"repair_id"`
		SaleID       *string `json:"sale_id"`
		Reference    string  `json:"reference"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		apierrors.Write(w, http.StatusBadRequest, "BAD_REQUEST", "invalid JSON body", httpx.CorrelationID(r.Context()))
		return
	}
	in := IssueInput{
		DurationMins: body.DurationMins,
		Phone:        body.Phone,
		Reference:    body.Reference,
	}
	if body.RepairID != nil && strings.TrimSpace(*body.RepairID) != "" {
		id, err := uuid.Parse(strings.TrimSpace(*body.RepairID))
		if err != nil {
			apierrors.Write(w, http.StatusBadRequest, "BAD_REQUEST", "invalid repair_id", httpx.CorrelationID(r.Context()))
			return
		}
		in.RepairID = &id
	}
	if body.SaleID != nil && strings.TrimSpace(*body.SaleID) != "" {
		id, err := uuid.Parse(strings.TrimSpace(*body.SaleID))
		if err != nil {
			apierrors.Write(w, http.StatusBadRequest, "BAD_REQUEST", "invalid sale_id", httpx.CorrelationID(r.Context()))
			return
		}
		in.SaleID = &id
	}
	out, err := h.svc.Issue(r.Context(), claims.TenantID, in)
	if err != nil {
		apierrors.Write(w, http.StatusBadRequest, "BAD_REQUEST", err.Error(), httpx.CorrelationID(r.Context()))
		return
	}
	httpx.JSON(w, http.StatusCreated, out)
}

func (h *Handler) getVoucher(w http.ResponseWriter, r *http.Request) {
	claims, ok := authz.FromContext(r.Context())
	if !ok {
		apierrors.Write(w, http.StatusUnauthorized, "UNAUTHORIZED", "missing claims", httpx.CorrelationID(r.Context()))
		return
	}
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		apierrors.Write(w, http.StatusBadRequest, "BAD_REQUEST", "invalid voucher id", httpx.CorrelationID(r.Context()))
		return
	}
	out, err := h.svc.GetVoucher(r.Context(), claims.TenantID, id)
	if err != nil {
		apierrors.Write(w, http.StatusNotFound, "NOT_FOUND", err.Error(), httpx.CorrelationID(r.Context()))
		return
	}
	httpx.JSON(w, http.StatusOK, out)
}

func (h *Handler) slipHTML(w http.ResponseWriter, r *http.Request) {
	claims, ok := authz.FromContext(r.Context())
	if !ok {
		apierrors.Write(w, http.StatusUnauthorized, "UNAUTHORIZED", "missing claims", httpx.CorrelationID(r.Context()))
		return
	}
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		apierrors.Write(w, http.StatusBadRequest, "BAD_REQUEST", "invalid voucher id", httpx.CorrelationID(r.Context()))
		return
	}
	v, err := h.svc.GetVoucher(r.Context(), claims.TenantID, id)
	if err != nil {
		apierrors.Write(w, http.StatusNotFound, "NOT_FOUND", err.Error(), httpx.CorrelationID(r.Context()))
		return
	}
	title := "Guest WiFi"
	if v.PackageName != "" {
		title = v.PackageName
	}
	qrImg := ""
	if dataURI, err := receipts.QRDataURIPNG(v.QRPayload); err == nil {
		qrImg = fmt.Sprintf(`<img alt="WiFi QR" src="%s" width="220" height="220" />`, dataURI)
	}
	expiresNote := ""
	if v.ExpiresAt != nil {
		expiresNote = " · expires " + htmlEscape(*v.ExpiresAt)
	}
	html := fmt.Sprintf(`<!DOCTYPE html><html><head><meta charset="utf-8"><title>%s</title>
<style>
body{font-family:system-ui,sans-serif;padding:24px;text-align:center;color:#111}
code{font-size:1.4rem;letter-spacing:.08em}
.muted{color:#666;font-size:.9rem;margin-top:8px}
.qr{margin:20px auto}
</style></head><body>
<h1>%s</h1>
<p class="muted">Scan to connect · valid about %s minutes</p>
<div class="qr">%s</div>
<p><code>%s</code></p>
<p class="muted">Or open WiFi portal and enter the code%s</p>
</body></html>`,
		htmlEscape(title),
		htmlEscape(title),
		strconv.Itoa(v.DurationMins),
		qrImg,
		htmlEscape(v.Code),
		expiresNote,
	)
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write([]byte(html))
}

func htmlEscape(s string) string {
	r := strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;", `"`, "&quot;")
	return r.Replace(s)
}

func hasOwnerOrManager(claims *authz.Claims) bool {
	if claims == nil {
		return false
	}
	if claims.HasPermission("*") {
		return true
	}
	for _, role := range claims.Roles {
		if role == "owner" || role == "manager" {
			return true
		}
	}
	return false
}
