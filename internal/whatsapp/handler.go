package whatsapp

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"

	"github.com/techlane/techlane/packages/pkg/apierrors"
	"github.com/techlane/techlane/packages/pkg/authz"
	"github.com/techlane/techlane/packages/pkg/httpx"
)

type Handler struct {
	svc *Service
}

func NewHandler(svc *Service) *Handler { return &Handler{svc: svc} }

func (h *Handler) Register(mux *http.ServeMux, auth func(http.Handler) http.Handler) {
	mux.Handle("GET /whatsapp/settings", auth(http.HandlerFunc(h.getSettings)))
	mux.Handle("PUT /whatsapp/settings", auth(http.HandlerFunc(h.putSettings)))
	mux.Handle("GET /whatsapp/status", auth(http.HandlerFunc(h.status)))
	mux.Handle("GET /whatsapp/qr", auth(http.HandlerFunc(h.qr)))
	mux.Handle("POST /whatsapp/disconnect", auth(http.HandlerFunc(h.disconnect)))
	mux.Handle("POST /whatsapp/reconnect", auth(http.HandlerFunc(h.reconnect)))
	// Sidecar → platform (service key, no staff JWT).
	mux.HandleFunc("POST /whatsapp/inbound", h.inbound)
}

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

func (h *Handler) getSettings(w http.ResponseWriter, r *http.Request) {
	claims, ok := h.claims(w, r)
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
	claims, ok := h.claims(w, r)
	if !ok {
		return
	}
	var req struct {
		Enabled         *bool `json:"enabled"`
		NotifyCustomers *bool `json:"notify_customers"`
		NotifySuppliers *bool `json:"notify_suppliers"`
		AlsoSendSMS     *bool `json:"also_send_sms"`
	}
	if err := json.NewDecoder(io.LimitReader(r.Body, 1<<16)).Decode(&req); err != nil {
		apierrors.Write(w, http.StatusBadRequest, "BAD_REQUEST", "invalid json", httpx.CorrelationID(r.Context()))
		return
	}
	set, err := h.svc.UpsertSettings(r.Context(), claims.TenantID, UpsertSettingsInput{
		Enabled: req.Enabled, NotifyCustomers: req.NotifyCustomers,
		NotifySuppliers: req.NotifySuppliers, AlsoSendSMS: req.AlsoSendSMS,
		ActorID: claims.UserID,
	})
	if err != nil {
		apierrors.Write(w, http.StatusBadRequest, "BAD_REQUEST", err.Error(), httpx.CorrelationID(r.Context()))
		return
	}
	httpx.JSON(w, http.StatusOK, set)
}

func (h *Handler) status(w http.ResponseWriter, r *http.Request) {
	claims, ok := h.claims(w, r)
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

func (h *Handler) qr(w http.ResponseWriter, r *http.Request) {
	claims, ok := h.claims(w, r)
	if !ok {
		return
	}
	if h.svc.Client() == nil || !h.svc.Client().Configured() {
		apierrors.Write(w, http.StatusServiceUnavailable, "UNAVAILABLE", "WhatsApp service is not configured on the server", httpx.CorrelationID(r.Context()))
		return
	}
	qr, err := h.svc.Client().QR(r.Context(), SessionID(claims.TenantID))
	if err != nil {
		apierrors.Write(w, http.StatusBadGateway, "BAD_GATEWAY", err.Error(), httpx.CorrelationID(r.Context()))
		return
	}
	httpx.JSON(w, http.StatusOK, qr)
}

func (h *Handler) disconnect(w http.ResponseWriter, r *http.Request) {
	claims, ok := h.claims(w, r)
	if !ok {
		return
	}
	if h.svc.Client() == nil || !h.svc.Client().Configured() {
		apierrors.Write(w, http.StatusServiceUnavailable, "UNAVAILABLE", "WhatsApp service is not configured on the server", httpx.CorrelationID(r.Context()))
		return
	}
	if err := h.svc.Client().Disconnect(r.Context(), SessionID(claims.TenantID)); err != nil {
		apierrors.Write(w, http.StatusBadGateway, "BAD_GATEWAY", err.Error(), httpx.CorrelationID(r.Context()))
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (h *Handler) reconnect(w http.ResponseWriter, r *http.Request) {
	claims, ok := h.claims(w, r)
	if !ok {
		return
	}
	if h.svc.Client() == nil || !h.svc.Client().Configured() {
		apierrors.Write(w, http.StatusServiceUnavailable, "UNAVAILABLE", "WhatsApp service is not configured on the server", httpx.CorrelationID(r.Context()))
		return
	}
	if err := h.svc.Client().Reconnect(r.Context(), SessionID(claims.TenantID)); err != nil {
		apierrors.Write(w, http.StatusBadGateway, "BAD_GATEWAY", err.Error(), httpx.CorrelationID(r.Context()))
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (h *Handler) inbound(w http.ResponseWriter, r *http.Request) {
	key := strings.TrimSpace(r.Header.Get("X-WhatsApp-Service-Key"))
	expected := ""
	if h.svc.Client() != nil {
		expected = h.svc.Client().ServiceKey
	}
	if expected == "" || key == "" || key != expected {
		apierrors.Write(w, http.StatusUnauthorized, "UNAUTHORIZED", "invalid service key", httpx.CorrelationID(r.Context()))
		return
	}
	var msg InboundMessage
	if err := json.NewDecoder(io.LimitReader(r.Body, 1<<20)).Decode(&msg); err != nil {
		apierrors.Write(w, http.StatusBadRequest, "BAD_REQUEST", "invalid json", httpx.CorrelationID(r.Context()))
		return
	}
	reply, err := h.svc.HandleInbound(r.Context(), msg)
	if err != nil {
		apierrors.Write(w, http.StatusBadRequest, "BAD_REQUEST", err.Error(), httpx.CorrelationID(r.Context()))
		return
	}
	if reply != "" && h.svc.Client() != nil && h.svc.Client().Configured() {
		tenantID := strings.TrimSpace(msg.TenantID)
		_, _ = h.svc.Client().Send(r.Context(), tenantID, msg.Phone, reply)
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"ok": true, "reply": reply})
}
