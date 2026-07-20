package reporting

import (
	"net/http"
	"strconv"

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
	mux.Handle("GET /reports/summary", auth(http.HandlerFunc(h.summary)))
	mux.Handle("GET /reports/operations", auth(http.HandlerFunc(h.operations)))
}

func (h *Handler) summary(w http.ResponseWriter, r *http.Request) {
	claims, _ := authz.FromContext(r.Context())
	if !claims.HasPermission("reports.read") && !claims.HasPermission("*") && !claims.HasPermission("risk.read") {
		apierrors.Write(w, http.StatusForbidden, "FORBIDDEN", "reports.read required", httpx.CorrelationID(r.Context()))
		return
	}
	days, _ := strconv.Atoi(r.URL.Query().Get("days"))
	sum, err := h.svc.Summary(r.Context(), claims.TenantID, days)
	if err != nil {
		apierrors.Write(w, http.StatusInternalServerError, "INTERNAL", err.Error(), httpx.CorrelationID(r.Context()))
		return
	}
	httpx.JSON(w, http.StatusOK, sum)
}

func (h *Handler) operations(w http.ResponseWriter, r *http.Request) {
	claims, _ := authz.FromContext(r.Context())
	if !claims.HasPermission("reports.read") && !claims.HasPermission("*") {
		apierrors.Write(w, http.StatusForbidden, "FORBIDDEN", "reports.read required", httpx.CorrelationID(r.Context()))
		return
	}
	days, _ := strconv.Atoi(r.URL.Query().Get("days"))
	report, err := h.svc.Operations(r.Context(), claims.TenantID, days)
	if err != nil {
		apierrors.Write(w, http.StatusInternalServerError, "INTERNAL", err.Error(), httpx.CorrelationID(r.Context()))
		return
	}
	httpx.JSON(w, http.StatusOK, report)
}
