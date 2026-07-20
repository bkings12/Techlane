package sales

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
	mux.Handle("POST /pos/checkout", auth(http.HandlerFunc(h.checkout)))
	mux.Handle("POST /sales", auth(http.HandlerFunc(h.createSale)))
	mux.Handle("GET /sales", auth(http.HandlerFunc(h.listSales)))
	mux.Handle("GET /sales/{id}", auth(http.HandlerFunc(h.getSale)))
	mux.Handle("POST /sales/{id}/complete", auth(http.HandlerFunc(h.completeSale)))
	mux.Handle("POST /sales/{id}/reverse", auth(http.HandlerFunc(h.reverseSale)))
}

func (h *Handler) listSales(w http.ResponseWriter, r *http.Request) {
	claims, _ := authz.FromContext(r.Context())
	var branchID *uuid.UUID
	if raw := r.URL.Query().Get("branch_id"); raw != "" {
		id, err := uuid.Parse(raw)
		if err != nil {
			apierrors.Write(w, http.StatusBadRequest, "BAD_REQUEST", "invalid branch_id", httpx.CorrelationID(r.Context()))
			return
		}
		branchID = &id
	}
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	items, err := h.svc.ListSales(r.Context(), claims.TenantID, branchID, r.URL.Query().Get("status"), limit)
	if err != nil {
		apierrors.Write(w, http.StatusInternalServerError, "INTERNAL", err.Error(), httpx.CorrelationID(r.Context()))
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"items": items})
}

func (h *Handler) checkout(w http.ResponseWriter, r *http.Request) {
	claims, _ := authz.FromContext(r.Context())
	var req struct {
		BranchID   uuid.UUID       `json:"branch_id"`
		LocationID uuid.UUID       `json:"location_id"`
		Items      []SaleItemInput `json:"items"`
		Method     string          `json:"method"`
		Phone      string          `json:"phone"`
		AccountRef string          `json:"account_reference"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.BranchID == uuid.Nil || req.LocationID == uuid.Nil || len(req.Items) == 0 {
		apierrors.Write(w, http.StatusBadRequest, "BAD_REQUEST", "branch_id, location_id, and items required", httpx.CorrelationID(r.Context()))
		return
	}
	if req.Method == "" {
		req.Method = "cash"
	}
	result, err := h.svc.Checkout(r.Context(), CheckoutInput{
		TenantID: claims.TenantID, BranchID: req.BranchID, LocationID: req.LocationID,
		Items: req.Items, Method: req.Method, Phone: req.Phone, AccountRef: req.AccountRef,
		ActorID: claims.UserID, CorrID: corrID(r),
	})
	if err != nil {
		apierrors.Write(w, http.StatusBadRequest, "BAD_REQUEST", err.Error(), httpx.CorrelationID(r.Context()))
		return
	}
	httpx.JSON(w, http.StatusCreated, result)
}

func (h *Handler) createSale(w http.ResponseWriter, r *http.Request) {
	claims, _ := authz.FromContext(r.Context())
	var req struct {
		BranchID   uuid.UUID       `json:"branch_id"`
		CustomerID *uuid.UUID      `json:"customer_id"`
		Channel    string          `json:"channel"`
		Items      []SaleItemInput `json:"items"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.BranchID == uuid.Nil || len(req.Items) == 0 {
		apierrors.Write(w, http.StatusBadRequest, "BAD_REQUEST", "branch_id and items required", httpx.CorrelationID(r.Context()))
		return
	}
	sale, err := h.svc.CreateSale(r.Context(), CreateSaleInput{
		TenantID: claims.TenantID, BranchID: req.BranchID, CustomerID: req.CustomerID,
		Channel: req.Channel, Items: req.Items, ActorID: claims.UserID, CorrID: corrID(r),
	})
	if err != nil {
		apierrors.Write(w, http.StatusBadRequest, "BAD_REQUEST", err.Error(), httpx.CorrelationID(r.Context()))
		return
	}
	httpx.JSON(w, http.StatusCreated, sale)
}

func (h *Handler) getSale(w http.ResponseWriter, r *http.Request) {
	claims, _ := authz.FromContext(r.Context())
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		apierrors.Write(w, http.StatusBadRequest, "BAD_REQUEST", "invalid id", httpx.CorrelationID(r.Context()))
		return
	}
	sale, err := h.svc.GetSale(r.Context(), claims.TenantID, id)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			apierrors.Write(w, http.StatusNotFound, "NOT_FOUND", err.Error(), httpx.CorrelationID(r.Context()))
			return
		}
		apierrors.Write(w, http.StatusInternalServerError, "INTERNAL", err.Error(), httpx.CorrelationID(r.Context()))
		return
	}
	httpx.JSON(w, http.StatusOK, sale)
}

func (h *Handler) completeSale(w http.ResponseWriter, r *http.Request) {
	claims, _ := authz.FromContext(r.Context())
	saleID, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		apierrors.Write(w, http.StatusBadRequest, "BAD_REQUEST", "invalid id", httpx.CorrelationID(r.Context()))
		return
	}
	var req struct {
		LocationID uuid.UUID `json:"location_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.LocationID == uuid.Nil {
		apierrors.Write(w, http.StatusBadRequest, "BAD_REQUEST", "location_id required", httpx.CorrelationID(r.Context()))
		return
	}
	sale, err := h.svc.CompleteSale(r.Context(), claims.TenantID, saleID, req.LocationID, claims.UserID, corrID(r))
	if err != nil {
		apierrors.Write(w, http.StatusConflict, "CONFLICT", err.Error(), httpx.CorrelationID(r.Context()))
		return
	}
	httpx.JSON(w, http.StatusOK, sale)
}

func (h *Handler) reverseSale(w http.ResponseWriter, r *http.Request) {
	claims, _ := authz.FromContext(r.Context())
	saleID, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		apierrors.Write(w, http.StatusBadRequest, "BAD_REQUEST", "invalid id", httpx.CorrelationID(r.Context()))
		return
	}
	var req struct {
		LocationID uuid.UUID `json:"location_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.LocationID == uuid.Nil {
		apierrors.Write(w, http.StatusBadRequest, "BAD_REQUEST", "location_id required", httpx.CorrelationID(r.Context()))
		return
	}
	sale, err := h.svc.ReverseSale(r.Context(), claims.TenantID, saleID, req.LocationID, claims.UserID, corrID(r))
	if err != nil {
		apierrors.Write(w, http.StatusConflict, "CONFLICT", err.Error(), httpx.CorrelationID(r.Context()))
		return
	}
	httpx.JSON(w, http.StatusOK, sale)
}

func corrID(r *http.Request) uuid.UUID {
	if cid := httpx.CorrelationID(r.Context()); cid != "" {
		if id, err := uuid.Parse(cid); err == nil {
			return id
		}
	}
	return uuid.New()
}
