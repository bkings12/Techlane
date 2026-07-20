package sync

import (
	"encoding/json"
	"errors"
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

func NewHandler(svc *Service) *Handler {
	return &Handler{svc: svc}
}

func (h *Handler) Register(mux *http.ServeMux, auth func(http.Handler) http.Handler) {
	mux.Handle("POST /sync/commands", auth(http.HandlerFunc(h.submit)))
	mux.Handle("GET /sync/commands", auth(http.HandlerFunc(h.list)))
	mux.Handle("POST /sync/commands/{id}/resolve", auth(http.HandlerFunc(h.resolve)))
}

func (h *Handler) submit(w http.ResponseWriter, r *http.Request) {
	claims, _ := authz.FromContext(r.Context())
	var req struct {
		ActionID       uuid.UUID      `json:"action_id"`
		CommandType    string         `json:"command_type"`
		BranchID       *uuid.UUID     `json:"branch_id"`
		DeviceID       *uuid.UUID     `json:"device_id"`
		LocalTimestamp *time.Time     `json:"local_timestamp"`
		Payload        map[string]any `json:"payload"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.ActionID == uuid.Nil || req.CommandType == "" {
		apierrors.Write(w, http.StatusBadRequest, "BAD_REQUEST", "action_id and command_type required", httpx.CorrelationID(r.Context()))
		return
	}
	if req.Payload == nil {
		req.Payload = map[string]any{}
	}
	if idem := r.Header.Get("Idempotency-Key"); idem != "" {
		if parsed, err := uuid.Parse(idem); err == nil {
			req.ActionID = parsed
		}
	}
	result, err := h.svc.Submit(r.Context(), CommandInput{
		ActionID: req.ActionID, TenantID: claims.TenantID, BranchID: req.BranchID,
		DeviceID: req.DeviceID, UserID: claims.UserID, CommandType: req.CommandType,
		LocalTimestamp: req.LocalTimestamp, Payload: req.Payload,
	})
	if err != nil {
		if errors.Is(err, ErrPayloadMismatch) {
			apierrors.Write(w, http.StatusConflict, "CONFLICT", err.Error(), httpx.CorrelationID(r.Context()))
			return
		}
		if errors.Is(err, ErrDeviceRevoked) {
			apierrors.Write(w, http.StatusForbidden, "FORBIDDEN", err.Error(), httpx.CorrelationID(r.Context()))
			return
		}
		apierrors.Write(w, http.StatusInternalServerError, "INTERNAL", err.Error(), httpx.CorrelationID(r.Context()))
		return
	}
	httpx.JSON(w, http.StatusOK, result)
}

func (h *Handler) list(w http.ResponseWriter, r *http.Request) {
	claims, _ := authz.FromContext(r.Context())
	if !claims.HasPermission("audit.read") && !claims.HasPermission("*") && !claims.HasPermission("risk.read") {
		apierrors.Write(w, http.StatusForbidden, "FORBIDDEN", "audit.read required", httpx.CorrelationID(r.Context()))
		return
	}
	items, err := h.svc.ListCommands(r.Context(), claims.TenantID, r.URL.Query().Get("status"), 50)
	if err != nil {
		apierrors.Write(w, http.StatusInternalServerError, "INTERNAL", err.Error(), httpx.CorrelationID(r.Context()))
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"items": items})
}

func (h *Handler) resolve(w http.ResponseWriter, r *http.Request) {
	claims, _ := authz.FromContext(r.Context())
	if !claims.HasPermission("audit.read") && !claims.HasPermission("*") {
		apierrors.Write(w, http.StatusForbidden, "FORBIDDEN", "audit.read required", httpx.CorrelationID(r.Context()))
		return
	}
	actionID, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		apierrors.Write(w, http.StatusBadRequest, "BAD_REQUEST", "invalid id", httpx.CorrelationID(r.Context()))
		return
	}
	var req struct {
		Resolution string `json:"resolution"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Resolution == "" {
		apierrors.Write(w, http.StatusBadRequest, "BAD_REQUEST", "resolution required (discard|retry)", httpx.CorrelationID(r.Context()))
		return
	}
	result, err := h.svc.ResolveCommand(r.Context(), claims.TenantID, actionID, claims.UserID, strings.TrimSpace(req.Resolution))
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			apierrors.Write(w, http.StatusNotFound, "NOT_FOUND", err.Error(), httpx.CorrelationID(r.Context()))
			return
		}
		if errors.Is(err, ErrNotRetryable) || strings.Contains(err.Error(), "resolution must") {
			apierrors.Write(w, http.StatusBadRequest, "BAD_REQUEST", err.Error(), httpx.CorrelationID(r.Context()))
			return
		}
		apierrors.Write(w, http.StatusConflict, "CONFLICT", err.Error(), httpx.CorrelationID(r.Context()))
		return
	}
	httpx.JSON(w, http.StatusOK, result)
}
