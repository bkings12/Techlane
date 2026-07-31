package notify

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/google/uuid"
	"github.com/techlane/techlane/packages/pkg/apierrors"
	"github.com/techlane/techlane/packages/pkg/authz"
	"github.com/techlane/techlane/packages/pkg/httpx"
)

type Handler struct {
	svc          *Service
	repairLookup RepairLookup
}

func NewHandler(svc *Service) *Handler {
	return &Handler{svc: svc}
}

// SetRepairLookup enables staff actions that rebuild SMS from live job/customer data.
func (h *Handler) SetRepairLookup(lookup RepairLookup) {
	h.repairLookup = lookup
}

func (h *Handler) Register(mux *http.ServeMux, auth func(http.Handler) http.Handler) {
	mux.Handle("GET /notifications", auth(http.HandlerFunc(h.listNotifications)))
	mux.Handle("POST /notifications/{id}/ack", auth(http.HandlerFunc(h.ackNotification)))
	mux.Handle("GET /sms/templates", auth(http.HandlerFunc(h.listSMSTemplates)))
	mux.Handle("PUT /sms/templates/{key}", auth(http.HandlerFunc(h.putSMSTemplate)))
	mux.Handle("POST /repairs/{id}/sms/resend-intake", auth(http.HandlerFunc(h.resendIntakeSMS)))
}

func canManageSMSTemplates(claims *authz.Claims) bool {
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

func (h *Handler) listSMSTemplates(w http.ResponseWriter, r *http.Request) {
	claims, ok := authz.FromContext(r.Context())
	if !ok {
		apierrors.Write(w, http.StatusUnauthorized, "UNAUTHORIZED", "missing claims", httpx.CorrelationID(r.Context()))
		return
	}
	if !canManageSMSTemplates(claims) {
		apierrors.Write(w, http.StatusForbidden, "FORBIDDEN", "owner required", httpx.CorrelationID(r.Context()))
		return
	}
	items, err := h.svc.ListSMSTemplates(r.Context(), claims.TenantID)
	if err != nil {
		apierrors.Write(w, http.StatusInternalServerError, "INTERNAL", err.Error(), httpx.CorrelationID(r.Context()))
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"items": items})
}

func (h *Handler) putSMSTemplate(w http.ResponseWriter, r *http.Request) {
	claims, ok := authz.FromContext(r.Context())
	if !ok {
		apierrors.Write(w, http.StatusUnauthorized, "UNAUTHORIZED", "missing claims", httpx.CorrelationID(r.Context()))
		return
	}
	if !canManageSMSTemplates(claims) {
		apierrors.Write(w, http.StatusForbidden, "FORBIDDEN", "owner required", httpx.CorrelationID(r.Context()))
		return
	}
	key := r.PathValue("key")
	var req struct {
		Body string `json:"body"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		apierrors.Write(w, http.StatusBadRequest, "BAD_REQUEST", "invalid json", httpx.CorrelationID(r.Context()))
		return
	}
	item, err := h.svc.UpsertSMSTemplate(r.Context(), claims.TenantID, key, req.Body, claims.UserID)
	if err != nil {
		status := http.StatusInternalServerError
		if strings.Contains(err.Error(), "unknown template") {
			status = http.StatusBadRequest
		}
		apierrors.Write(w, status, "BAD_REQUEST", err.Error(), httpx.CorrelationID(r.Context()))
		return
	}
	httpx.JSON(w, http.StatusOK, item)
}

func (h *Handler) listNotifications(w http.ResponseWriter, r *http.Request) {
	claims, ok := authz.FromContext(r.Context())
	if !ok {
		apierrors.Write(w, http.StatusUnauthorized, "UNAUTHORIZED", "missing claims", httpx.CorrelationID(r.Context()))
		return
	}
	unackedOnly := strings.EqualFold(r.URL.Query().Get("unacked"), "true") ||
		r.URL.Query().Get("unacked") == "1"
	items, err := h.svc.ListStaffInbox(r.Context(), claims.TenantID, unackedOnly, 50)
	if err != nil {
		apierrors.Write(w, http.StatusInternalServerError, "INTERNAL", err.Error(), httpx.CorrelationID(r.Context()))
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"items": items})
}

func (h *Handler) ackNotification(w http.ResponseWriter, r *http.Request) {
	claims, ok := authz.FromContext(r.Context())
	if !ok {
		apierrors.Write(w, http.StatusUnauthorized, "UNAUTHORIZED", "missing claims", httpx.CorrelationID(r.Context()))
		return
	}
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		apierrors.Write(w, http.StatusBadRequest, "BAD_REQUEST", "invalid id", httpx.CorrelationID(r.Context()))
		return
	}
	if err := h.svc.AckStaffInbox(r.Context(), claims.TenantID, id); err != nil {
		status := http.StatusInternalServerError
		if strings.Contains(err.Error(), "not found") {
			status = http.StatusNotFound
		}
		apierrors.Write(w, status, "NOTIFY_FAILED", err.Error(), httpx.CorrelationID(r.Context()))
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) resendIntakeSMS(w http.ResponseWriter, r *http.Request) {
	claims, ok := authz.FromContext(r.Context())
	if !ok {
		apierrors.Write(w, http.StatusUnauthorized, "UNAUTHORIZED", "missing claims", httpx.CorrelationID(r.Context()))
		return
	}
	if !claims.HasPermission("customers.write") && !claims.HasPermission("repairs.create") && !claims.HasPermission("*") {
		apierrors.Write(w, http.StatusForbidden, "FORBIDDEN", "customers.write or repairs.create required", httpx.CorrelationID(r.Context()))
		return
	}
	if h.repairLookup == nil {
		apierrors.Write(w, http.StatusServiceUnavailable, "UNAVAILABLE", "notify repair lookup not configured", httpx.CorrelationID(r.Context()))
		return
	}
	repairID, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		apierrors.Write(w, http.StatusBadRequest, "BAD_REQUEST", "invalid id", httpx.CorrelationID(r.Context()))
		return
	}
	templateKey, phone, err := h.svc.ResendRepairIntakeSMS(r.Context(), claims.TenantID, repairID, h.repairLookup)
	if err != nil {
		status := http.StatusInternalServerError
		msg := err.Error()
		if strings.Contains(msg, "no phone") {
			status = http.StatusBadRequest
		} else if strings.Contains(msg, "not found") {
			status = http.StatusNotFound
		}
		apierrors.Write(w, status, "NOTIFY_FAILED", msg, httpx.CorrelationID(r.Context()))
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{
		"status":       "queued",
		"template_key": templateKey,
		"phone":        phone,
	})
}
