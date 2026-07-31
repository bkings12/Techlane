package loyalty

import (
	"encoding/json"
	"net/http"

	"github.com/google/uuid"
	"github.com/techlane/techlane/packages/pkg/apierrors"
	"github.com/techlane/techlane/packages/pkg/authz"
	"github.com/techlane/techlane/packages/pkg/httpx"
)

type Handler struct {
	svc *Service
}

func NewHandler(svc *Service) *Handler {
	return &Handler{svc: svc}
}

func (h *Handler) Register(mux *http.ServeMux, auth func(http.Handler) http.Handler) {
	mux.Handle("GET /loyalty/settings", auth(http.HandlerFunc(h.getSettings)))
	mux.Handle("PUT /loyalty/settings", auth(http.HandlerFunc(h.putSettings)))
	mux.Handle("GET /loyalty/customers/{id}", auth(http.HandlerFunc(h.getAccount)))
	mux.Handle("GET /loyalty/webhooks", auth(http.HandlerFunc(h.listWebhooks)))
	mux.Handle("POST /loyalty/webhooks", auth(http.HandlerFunc(h.createWebhook)))
	mux.Handle("DELETE /loyalty/webhooks/{id}", auth(http.HandlerFunc(h.deleteWebhook)))
}

func requirePermission(w http.ResponseWriter, r *http.Request, code string) (*authz.Claims, bool) {
	claims, _ := authz.FromContext(r.Context())
	if claims == nil || (!claims.HasPermission(code) && !claims.HasPermission("*")) {
		apierrors.Write(w, http.StatusForbidden, "FORBIDDEN", code+" required", httpx.CorrelationID(r.Context()))
		return claims, false
	}
	return claims, true
}

func (h *Handler) getSettings(w http.ResponseWriter, r *http.Request) {
	claims, ok := requirePermission(w, r, "loyalty.read")
	if !ok {
		return
	}
	st, err := h.svc.GetSettings(r.Context(), claims.TenantID)
	if err != nil {
		apierrors.Write(w, http.StatusInternalServerError, "INTERNAL", err.Error(), httpx.CorrelationID(r.Context()))
		return
	}
	httpx.JSON(w, http.StatusOK, st)
}

func (h *Handler) putSettings(w http.ResponseWriter, r *http.Request) {
	claims, ok := requirePermission(w, r, "loyalty.manage")
	if !ok {
		return
	}
	var body struct {
		Enabled                  bool    `json:"enabled"`
		PointsPerCompletedRepair int     `json:"points_per_completed_repair"`
		PointsPerCurrencyUnit    float64 `json:"points_per_currency_unit"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		apierrors.Write(w, http.StatusBadRequest, "BAD_REQUEST", "invalid JSON body", httpx.CorrelationID(r.Context()))
		return
	}
	st, err := h.svc.UpdateSettings(r.Context(), claims.TenantID, body.Enabled, body.PointsPerCompletedRepair, body.PointsPerCurrencyUnit)
	if err != nil {
		apierrors.Write(w, http.StatusBadRequest, "BAD_REQUEST", err.Error(), httpx.CorrelationID(r.Context()))
		return
	}
	httpx.JSON(w, http.StatusOK, st)
}

func (h *Handler) getAccount(w http.ResponseWriter, r *http.Request) {
	claims, ok := requirePermission(w, r, "loyalty.read")
	if !ok {
		return
	}
	customerID, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		apierrors.Write(w, http.StatusBadRequest, "BAD_REQUEST", "invalid customer id", httpx.CorrelationID(r.Context()))
		return
	}
	acct, ledger, err := h.svc.GetAccount(r.Context(), claims.TenantID, customerID)
	if err != nil {
		apierrors.Write(w, http.StatusInternalServerError, "INTERNAL", err.Error(), httpx.CorrelationID(r.Context()))
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"account": acct, "ledger": ledger})
}

func (h *Handler) listWebhooks(w http.ResponseWriter, r *http.Request) {
	claims, ok := requirePermission(w, r, "webhooks.manage")
	if !ok {
		return
	}
	items, err := h.svc.ListWebhooks(r.Context(), claims.TenantID)
	if err != nil {
		apierrors.Write(w, http.StatusInternalServerError, "INTERNAL", err.Error(), httpx.CorrelationID(r.Context()))
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"items": items})
}

func (h *Handler) createWebhook(w http.ResponseWriter, r *http.Request) {
	claims, ok := requirePermission(w, r, "webhooks.manage")
	if !ok {
		return
	}
	var body struct {
		URL        string   `json:"url"`
		EventTypes []string `json:"event_types"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		apierrors.Write(w, http.StatusBadRequest, "BAD_REQUEST", "invalid JSON body", httpx.CorrelationID(r.Context()))
		return
	}
	var actorID *uuid.UUID
	if claims.UserID != uuid.Nil {
		actorID = &claims.UserID
	}
	sub, err := h.svc.RegisterWebhook(r.Context(), claims.TenantID, body.URL, body.EventTypes, actorID)
	if err != nil {
		apierrors.Write(w, http.StatusBadRequest, "BAD_REQUEST", err.Error(), httpx.CorrelationID(r.Context()))
		return
	}
	httpx.JSON(w, http.StatusCreated, sub)
}

func (h *Handler) deleteWebhook(w http.ResponseWriter, r *http.Request) {
	claims, ok := requirePermission(w, r, "webhooks.manage")
	if !ok {
		return
	}
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		apierrors.Write(w, http.StatusBadRequest, "BAD_REQUEST", "invalid id", httpx.CorrelationID(r.Context()))
		return
	}
	if err := h.svc.DeleteWebhook(r.Context(), claims.TenantID, id); err != nil {
		apierrors.Write(w, http.StatusNotFound, "NOT_FOUND", "webhook not found", httpx.CorrelationID(r.Context()))
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
