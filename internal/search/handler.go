package search

import (
	"net/http"

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
	mux.Handle("GET /search", auth(http.HandlerFunc(h.search)))
}

func (h *Handler) search(w http.ResponseWriter, r *http.Request) {
	claims, ok := authz.FromContext(r.Context())
	if !ok {
		apierrors.Write(w, http.StatusUnauthorized, "UNAUTHORIZED", "missing claims", httpx.CorrelationID(r.Context()))
		return
	}
	q := r.URL.Query().Get("q")
	results, err := h.svc.Search(r.Context(), claims.TenantID, q, 6)
	if err != nil {
		apierrors.Write(w, http.StatusInternalServerError, "INTERNAL", err.Error(), httpx.CorrelationID(r.Context()))
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"results": results})
}
