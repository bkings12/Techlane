package sales

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

// SetReceiptRenderer enables printable POS sale receipts.
func (h *Handler) SetReceiptRenderer(r *receipts.Service) { h.receipts = r }

func (h *Handler) Register(mux *http.ServeMux, auth func(http.Handler) http.Handler) {
	mux.Handle("POST /pos/checkout", auth(http.HandlerFunc(h.checkout)))
	mux.Handle("POST /sales", auth(http.HandlerFunc(h.createSale)))
	mux.Handle("GET /sales", auth(http.HandlerFunc(h.listSales)))
	mux.Handle("GET /sales/{id}", auth(http.HandlerFunc(h.getSale)))
	mux.Handle("GET /sales/{id}/receipt.html", auth(http.HandlerFunc(h.saleReceiptHTML)))
	mux.Handle("GET /sales/{id}/receipt.escpos", auth(http.HandlerFunc(h.saleReceiptESCPOS)))
	mux.Handle("GET /sales/{id}/receipt.pdf", auth(http.HandlerFunc(h.saleReceiptPDF)))
	mux.Handle("POST /sales/{id}/complete", auth(http.HandlerFunc(h.completeSale)))
	mux.Handle("POST /sales/{id}/reverse", auth(http.HandlerFunc(h.reverseSale)))
}

func (h *Handler) loadSaleReceipt(w http.ResponseWriter, r *http.Request) (receipts.Document, uuid.UUID, uuid.UUID, bool) {
	claims, ok := authz.FromContext(r.Context())
	if !ok {
		apierrors.Write(w, http.StatusUnauthorized, "UNAUTHORIZED", "missing claims", httpx.CorrelationID(r.Context()))
		return receipts.Document{}, uuid.Nil, uuid.Nil, false
	}
	if h.receipts == nil {
		apierrors.Write(w, http.StatusServiceUnavailable, "UNAVAILABLE", "receipt rendering is not configured", httpx.CorrelationID(r.Context()))
		return receipts.Document{}, uuid.Nil, uuid.Nil, false
	}
	saleID, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		apierrors.Write(w, http.StatusBadRequest, "BAD_REQUEST", "invalid id", httpx.CorrelationID(r.Context()))
		return receipts.Document{}, uuid.Nil, uuid.Nil, false
	}
	doc, err := h.svc.BuildSaleReceipt(r.Context(), claims.TenantID, saleID)
	if err != nil {
		status := http.StatusInternalServerError
		if strings.Contains(err.Error(), "not found") {
			status = http.StatusNotFound
		}
		apierrors.Write(w, status, "RECEIPT_FAILED", err.Error(), httpx.CorrelationID(r.Context()))
		return receipts.Document{}, uuid.Nil, uuid.Nil, false
	}
	return doc, claims.TenantID, saleID, true
}

func (h *Handler) saleReceiptHTML(w http.ResponseWriter, r *http.Request) {
	doc, tenantID, saleID, ok := h.loadSaleReceipt(w, r)
	if !ok {
		return
	}
	body, err := h.receipts.Render(r.Context(), tenantID, doc, saleID, r.URL.Query().Get("paper"))
	if err != nil {
		apierrors.Write(w, http.StatusInternalServerError, "INTERNAL", err.Error(), httpx.CorrelationID(r.Context()))
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(body))
}

func (h *Handler) saleReceiptESCPOS(w http.ResponseWriter, r *http.Request) {
	doc, tenantID, saleID, ok := h.loadSaleReceipt(w, r)
	if !ok {
		return
	}
	body, err := h.receipts.RenderESCPOS(r.Context(), tenantID, doc, saleID, r.URL.Query().Get("paper"))
	if err != nil {
		apierrors.Write(w, http.StatusInternalServerError, "INTERNAL", err.Error(), httpx.CorrelationID(r.Context()))
		return
	}
	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`inline; filename=%q`, doc.Reference+"-receipt.escpos"))
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(body)
}

func (h *Handler) saleReceiptPDF(w http.ResponseWriter, r *http.Request) {
	doc, tenantID, saleID, ok := h.loadSaleReceipt(w, r)
	if !ok {
		return
	}
	pdf, err := h.receipts.RenderPDF(r.Context(), tenantID, doc, saleID)
	if err != nil {
		apierrors.Write(w, http.StatusInternalServerError, "INTERNAL", err.Error(), httpx.CorrelationID(r.Context()))
		return
	}
	w.Header().Set("Content-Type", "application/pdf")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`inline; filename=%q`, doc.Reference+"-receipt.pdf"))
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(pdf)
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
		AllowPriceOverride: claims.HasPermission("sales.price_override"),
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
		AllowPriceOverride: claims.HasPermission("sales.price_override"),
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
