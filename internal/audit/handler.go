package audit

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

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
	mux.Handle("POST /audit/events", auth(http.HandlerFunc(h.appendEvent)))
	mux.Handle("GET /audit/events", auth(http.HandlerFunc(h.listEvents)))
	mux.Handle("GET /risk/alerts", auth(http.HandlerFunc(h.listAlerts)))
	mux.Handle("POST /risk/alerts", auth(http.HandlerFunc(h.createAlert)))
	mux.Handle("POST /risk/alerts/{id}/ack", auth(http.HandlerFunc(h.ackAlert)))
	mux.Handle("POST /risk/alerts/{id}/resolve", auth(http.HandlerFunc(h.resolveAlert)))
}

func (h *Handler) listEvents(w http.ResponseWriter, r *http.Request) {
	claims, _ := authz.FromContext(r.Context())
	if !claims.HasPermission("audit.read") && !claims.HasPermission("*") {
		apierrors.Write(w, http.StatusForbidden, "FORBIDDEN", "audit.read required", httpx.CorrelationID(r.Context()))
		return
	}
	filter := AuditFilter{
		Action:     r.URL.Query().Get("action"),
		EntityType: r.URL.Query().Get("entity_type"),
		Search:     r.URL.Query().Get("q"),
	}
	filter.Limit, _ = strconv.Atoi(r.URL.Query().Get("limit"))
	parseID := func(key string, target **uuid.UUID) bool {
		raw := r.URL.Query().Get(key)
		if raw == "" {
			return true
		}
		id, err := uuid.Parse(raw)
		if err != nil {
			apierrors.Write(w, http.StatusBadRequest, "BAD_REQUEST", "invalid "+key, httpx.CorrelationID(r.Context()))
			return false
		}
		*target = &id
		return true
	}
	if !parseID("entity_id", &filter.EntityID) || !parseID("actor_id", &filter.ActorID) || !parseID("branch_id", &filter.BranchID) {
		return
	}
	items, err := h.svc.ListEvents(r.Context(), claims.TenantID, filter)
	if err != nil {
		apierrors.Write(w, http.StatusInternalServerError, "INTERNAL", err.Error(), httpx.CorrelationID(r.Context()))
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"items": items})
}

func (h *Handler) appendEvent(w http.ResponseWriter, r *http.Request) {
	claims, _ := authz.FromContext(r.Context())
	var req struct {
		Action     string         `json:"action"`
		EntityType string         `json:"entity_type"`
		EntityID   *uuid.UUID     `json:"entity_id"`
		NewValue   map[string]any `json:"new_value"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Action == "" || req.EntityType == "" {
		apierrors.Write(w, http.StatusBadRequest, "BAD_REQUEST", "action and entity_type required", httpx.CorrelationID(r.Context()))
		return
	}
	ev, err := h.svc.AppendEvent(r.Context(), RecordInput{
		TenantID: claims.TenantID, ActorID: &claims.UserID, Action: req.Action,
		EntityType: req.EntityType, EntityID: req.EntityID, NewValue: req.NewValue, CorrID: corrID(r),
	})
	if err != nil {
		apierrors.Write(w, http.StatusInternalServerError, "INTERNAL", err.Error(), httpx.CorrelationID(r.Context()))
		return
	}
	httpx.JSON(w, http.StatusCreated, ev)
}

func (h *Handler) listAlerts(w http.ResponseWriter, r *http.Request) {
	claims, _ := authz.FromContext(r.Context())
	items, err := h.svc.ListRiskAlerts(r.Context(), claims.TenantID, r.URL.Query().Get("status"))
	if err != nil {
		apierrors.Write(w, http.StatusInternalServerError, "INTERNAL", err.Error(), httpx.CorrelationID(r.Context()))
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"items": items})
}

func (h *Handler) createAlert(w http.ResponseWriter, r *http.Request) {
	claims, _ := authz.FromContext(r.Context())
	var req struct {
		Kind       string         `json:"kind"`
		Severity   string         `json:"severity"`
		Title      string         `json:"title"`
		EntityType *string        `json:"entity_type"`
		EntityID   *uuid.UUID     `json:"entity_id"`
		Details    map[string]any `json:"details"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Kind == "" || req.Title == "" {
		apierrors.Write(w, http.StatusBadRequest, "BAD_REQUEST", "kind and title required", httpx.CorrelationID(r.Context()))
		return
	}
	if req.Severity == "" {
		req.Severity = "medium"
	}
	a, err := h.svc.CreateRiskAlert(r.Context(), claims.TenantID, nil, req.Kind, req.Severity, req.Title, req.EntityType, req.EntityID, req.Details)
	if err != nil {
		apierrors.Write(w, http.StatusInternalServerError, "INTERNAL", err.Error(), httpx.CorrelationID(r.Context()))
		return
	}
	httpx.JSON(w, http.StatusCreated, a)
}

func (h *Handler) ackAlert(w http.ResponseWriter, r *http.Request) {
	claims, _ := authz.FromContext(r.Context())
	alertID, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		apierrors.Write(w, http.StatusBadRequest, "BAD_REQUEST", "invalid id", httpx.CorrelationID(r.Context()))
		return
	}
	a, err := h.svc.AckAlert(r.Context(), claims.TenantID, alertID, claims.UserID)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			apierrors.Write(w, http.StatusNotFound, "NOT_FOUND", err.Error(), httpx.CorrelationID(r.Context()))
			return
		}
		apierrors.Write(w, http.StatusConflict, "CONFLICT", err.Error(), httpx.CorrelationID(r.Context()))
		return
	}
	httpx.JSON(w, http.StatusOK, a)
}

func (h *Handler) resolveAlert(w http.ResponseWriter, r *http.Request) {
	claims, _ := authz.FromContext(r.Context())
	if !claims.HasPermission("risk.read") && !claims.HasPermission("*") {
		apierrors.Write(w, http.StatusForbidden, "FORBIDDEN", "risk.read required", httpx.CorrelationID(r.Context()))
		return
	}
	alertID, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		apierrors.Write(w, http.StatusBadRequest, "BAD_REQUEST", "invalid id", httpx.CorrelationID(r.Context()))
		return
	}
	a, err := h.svc.ResolveAlert(r.Context(), claims.TenantID, alertID, claims.UserID)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			apierrors.Write(w, http.StatusNotFound, "NOT_FOUND", err.Error(), httpx.CorrelationID(r.Context()))
			return
		}
		apierrors.Write(w, http.StatusConflict, "CONFLICT", err.Error(), httpx.CorrelationID(r.Context()))
		return
	}
	httpx.JSON(w, http.StatusOK, a)
}

func corrID(r *http.Request) uuid.UUID {
	if cid := httpx.CorrelationID(r.Context()); cid != "" {
		if id, err := uuid.Parse(cid); err == nil {
			return id
		}
	}
	return uuid.New()
}
