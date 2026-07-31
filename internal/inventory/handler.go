package inventory

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"time"

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

// SetReceiptRenderer prints supplier vouchers with the shop's receipt branding.
func (h *Handler) SetReceiptRenderer(r *receipts.Service) { h.receipts = r }

func (h *Handler) Register(mux *http.ServeMux, auth func(http.Handler) http.Handler) {
	mux.Handle("POST /part-requests", auth(httpx.RequirePermission("parts.request")(http.HandlerFunc(h.createPartRequest))))
	mux.Handle("GET /part-requests", auth(http.HandlerFunc(h.listPartRequests)))
	mux.Handle("POST /part-requests/{id}/approve", auth(httpx.RequirePermission("parts.approve")(http.HandlerFunc(h.approvePartRequest))))
	mux.Handle("POST /part-requests/{id}/assign", auth(httpx.RequirePermission("parts.approve")(http.HandlerFunc(h.assignPartRequest))))
	mux.Handle("POST /part-requests/{id}/quotes/{quote_id}/accept", auth(httpx.RequirePermission("parts.approve")(http.HandlerFunc(h.acceptQuote))))
	mux.Handle("POST /part-requests/{id}/issue-from-stock", auth(httpx.RequirePermission("parts.approve")(http.HandlerFunc(h.issuePartFromStock))))
	mux.Handle("POST /supplier-issues/{id}/collect", auth(httpx.RequirePermission("parts.collect")(http.HandlerFunc(h.collectIssue))))
	mux.Handle("POST /supplier-issues/{id}/reconcile", auth(httpx.RequirePermission("supplier_credit.reconcile")(http.HandlerFunc(h.reconcileIssue))))
	mux.Handle("GET /supplier-issues/orphans", auth(httpx.RequirePermission("suppliers.read")(http.HandlerFunc(h.listOrphans))))
	mux.Handle("GET /supplier-issues/pending-reconciliation", auth(httpx.RequirePermission("suppliers.read")(http.HandlerFunc(h.listPendingReconciliation))))
	mux.Handle("GET /suppliers", auth(http.HandlerFunc(h.listSuppliers)))
	mux.Handle("POST /suppliers", auth(httpx.RequirePermission("suppliers.write")(http.HandlerFunc(h.createSupplier))))
	mux.Handle("GET /suppliers/{id}/credit", auth(httpx.RequirePermission("suppliers.read")(http.HandlerFunc(h.listSupplierCredit))))
	mux.Handle("POST /suppliers/{id}/contacts/invite", auth(httpx.RequirePermission("suppliers.write")(http.HandlerFunc(h.inviteSupplierContact))))
	mux.Handle("GET /categories", auth(http.HandlerFunc(h.listCategories)))
	mux.Handle("POST /categories", auth(http.HandlerFunc(h.createCategory)))
	mux.Handle("PATCH /categories/{id}", auth(http.HandlerFunc(h.updateCategory)))
	mux.Handle("DELETE /categories/{id}", auth(http.HandlerFunc(h.deleteCategory)))
	mux.Handle("GET /products", auth(http.HandlerFunc(h.listProducts)))
	mux.Handle("POST /products", auth(http.HandlerFunc(h.createProduct)))
	mux.Handle("PATCH /products/{id}", auth(http.HandlerFunc(h.updateProduct)))
	mux.Handle("POST /products/{id}/image", auth(http.HandlerFunc(h.uploadProductImage)))
	mux.Handle("DELETE /products/{id}/image", auth(http.HandlerFunc(h.deleteProductImage)))
	mux.HandleFunc("GET /inventory/public/products/{productID}/image", h.publicProductImage)
	mux.Handle("GET /variants", auth(http.HandlerFunc(h.listVariants)))
	mux.Handle("POST /variants", auth(http.HandlerFunc(h.createVariant)))
	mux.Handle("PATCH /variants/{id}", auth(http.HandlerFunc(h.updateVariant)))
	mux.Handle("GET /catalog", auth(http.HandlerFunc(h.listCatalog)))
	mux.Handle("GET /stock-locations", auth(http.HandlerFunc(h.listLocations)))
	mux.Handle("POST /stock-locations/ensure", auth(http.HandlerFunc(h.ensureLocations)))
	mux.Handle("GET /inventory/balances", auth(http.HandlerFunc(h.listBalances)))
	mux.Handle("GET /inventory/movements", auth(http.HandlerFunc(h.listMovements)))
	mux.Handle("POST /inventory/receive", auth(http.HandlerFunc(h.receive)))
	mux.Handle("POST /inventory/adjust", auth(http.HandlerFunc(h.adjust)))
	mux.Handle("POST /inventory/reserve", auth(http.HandlerFunc(h.reserve)))
	mux.Handle("POST /inventory/transfer", auth(http.HandlerFunc(h.transfer)))

	// Supplier portal — opaque session tokens, not staff JWT.
	mux.HandleFunc("POST /supplier/auth/accept-invite", h.supplierAcceptInvite)
	mux.HandleFunc("POST /supplier/auth/login", h.supplierLogin)
	mux.HandleFunc("POST /supplier/auth/logout", h.supplierLogout)
	mux.HandleFunc("GET /supplier/me", h.supplierMe)
	mux.HandleFunc("GET /supplier/requests", h.supplierListRequests)
	mux.HandleFunc("GET /supplier/requests/{id}", h.supplierGetRequest)
	mux.HandleFunc("POST /supplier/requests/{id}/quote", h.supplierQuote)
	mux.HandleFunc("POST /supplier/requests/{id}/decline", h.supplierDecline)
	mux.HandleFunc("POST /supplier/requests/{id}/ready", h.supplierMarkReady)
	mux.HandleFunc("POST /supplier/requests/{id}/issue", h.supplierIssue)
	mux.HandleFunc("GET /supplier/issues", h.supplierListIssues)
	mux.HandleFunc("POST /supplier/issues/{id}/collect", h.supplierCollectIssue)
	mux.HandleFunc("GET /supplier/issues/{id}/voucher", h.supplierIssueVoucher)
	mux.HandleFunc("GET /supplier/issues/{id}/voucher.html", h.supplierIssueVoucherHTML)
	mux.HandleFunc("GET /supplier/issues/{id}/voucher.pdf", h.supplierIssueVoucherPDF)
	mux.HandleFunc("GET /supplier/credit", h.supplierCredit)
}

func (h *Handler) createPartRequest(w http.ResponseWriter, r *http.Request) {
	claims, _ := authz.FromContext(r.Context())
	var req struct {
		BranchID    uuid.UUID  `json:"branch_id"`
		RepairJobID uuid.UUID  `json:"repair_job_id"`
		VariantID   *uuid.UUID `json:"variant_id"`
		Description string     `json:"description"`
		Quantity    int        `json:"quantity"`
		SupplierID  *uuid.UUID `json:"supplier_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.RepairJobID == uuid.Nil || req.Description == "" {
		apierrors.Write(w, http.StatusBadRequest, "BAD_REQUEST", "repair_job_id and description required", httpx.CorrelationID(r.Context()))
		return
	}
	if req.BranchID == uuid.Nil {
		if len(claims.BranchIDs) > 0 {
			req.BranchID, _ = uuid.Parse(claims.BranchIDs[0])
		}
	}
	pr, err := h.svc.CreatePartRequest(r.Context(), claims.TenantID, req.BranchID, req.RepairJobID, req.VariantID, req.Description, req.Quantity, req.SupplierID, claims.UserID, corrID(r), nil)
	if err != nil {
		if strings.Contains(err.Error(), "cannot request parts") {
			apierrors.Write(w, http.StatusConflict, "JOB_NOT_OPEN", err.Error(), httpx.CorrelationID(r.Context()))
			return
		}
		status := http.StatusInternalServerError
		code := "INTERNAL"
		if strings.Contains(err.Error(), "supplier not found") || strings.Contains(err.Error(), "already exists") {
			status = http.StatusBadRequest
			code = "BAD_REQUEST"
		}
		apierrors.Write(w, status, code, err.Error(), httpx.CorrelationID(r.Context()))
		return
	}
	httpx.JSON(w, http.StatusCreated, pr)
}

func (h *Handler) listPartRequests(w http.ResponseWriter, r *http.Request) {
	claims, _ := authz.FromContext(r.Context())
	if !claims.HasPermission("parts.request") && !claims.HasPermission("parts.approve") && !claims.HasPermission("inventory.read") {
		apierrors.Write(w, http.StatusForbidden, "FORBIDDEN", "permission denied", httpx.CorrelationID(r.Context()))
		return
	}
	var repairID *uuid.UUID
	if raw := r.URL.Query().Get("repair_job_id"); raw != "" {
		id, err := uuid.Parse(raw)
		if err != nil {
			apierrors.Write(w, http.StatusBadRequest, "BAD_REQUEST", "invalid repair_job_id", httpx.CorrelationID(r.Context()))
			return
		}
		repairID = &id
	}
	items, err := h.svc.ListPartRequests(r.Context(), claims.TenantID, repairID, r.URL.Query().Get("status"))
	if err != nil {
		apierrors.Write(w, http.StatusInternalServerError, "INTERNAL", err.Error(), httpx.CorrelationID(r.Context()))
		return
	}
	if items == nil {
		items = []PartRequest{}
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"items": items})
}

func (h *Handler) approvePartRequest(w http.ResponseWriter, r *http.Request) {
	claims, _ := authz.FromContext(r.Context())
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		apierrors.Write(w, http.StatusBadRequest, "BAD_REQUEST", "invalid id", httpx.CorrelationID(r.Context()))
		return
	}
	var req struct {
		SupplierID *uuid.UUID `json:"supplier_id"`
		UnitCost   float64    `json:"unit_cost"`
	}
	_ = json.NewDecoder(r.Body).Decode(&req)
	si, err := h.svc.ApprovePartRequest(r.Context(), claims.TenantID, id, req.SupplierID, req.UnitCost, claims.UserID, corrID(r))
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			apierrors.Write(w, http.StatusNotFound, "NOT_FOUND", err.Error(), httpx.CorrelationID(r.Context()))
			return
		}
		apierrors.Write(w, http.StatusConflict, "CONFLICT", err.Error(), httpx.CorrelationID(r.Context()))
		return
	}
	httpx.JSON(w, http.StatusOK, si)
}

func (h *Handler) issuePartFromStock(w http.ResponseWriter, r *http.Request) {
	claims, _ := authz.FromContext(r.Context())
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		apierrors.Write(w, http.StatusBadRequest, "BAD_REQUEST", "invalid id", httpx.CorrelationID(r.Context()))
		return
	}
	var req struct {
		VariantID  uuid.UUID `json:"variant_id"`
		LocationID uuid.UUID `json:"location_id"`
		Quantity   int       `json:"quantity"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.VariantID == uuid.Nil || req.LocationID == uuid.Nil {
		apierrors.Write(w, http.StatusBadRequest, "BAD_REQUEST", "variant_id and location_id required", httpx.CorrelationID(r.Context()))
		return
	}
	pr, err := h.svc.IssuePartFromStock(r.Context(), claims.TenantID, id, req.VariantID, req.LocationID, req.Quantity, claims.UserID, corrID(r))
	if err != nil {
		if errors.Is(err, ErrInsufficientStock) {
			apierrors.Write(w, http.StatusConflict, "INSUFFICIENT_STOCK", err.Error(), httpx.CorrelationID(r.Context()))
			return
		}
		if strings.Contains(err.Error(), "not found") {
			apierrors.Write(w, http.StatusNotFound, "NOT_FOUND", err.Error(), httpx.CorrelationID(r.Context()))
			return
		}
		apierrors.Write(w, http.StatusConflict, "CONFLICT", err.Error(), httpx.CorrelationID(r.Context()))
		return
	}
	httpx.JSON(w, http.StatusOK, pr)
}

func (h *Handler) collectIssue(w http.ResponseWriter, r *http.Request) {
	claims, _ := authz.FromContext(r.Context())
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		apierrors.Write(w, http.StatusBadRequest, "BAD_REQUEST", "invalid id", httpx.CorrelationID(r.Context()))
		return
	}
	var req struct {
		AuthCode string `json:"auth_code"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.AuthCode == "" {
		apierrors.Write(w, http.StatusBadRequest, "BAD_REQUEST", "auth_code required", httpx.CorrelationID(r.Context()))
		return
	}
	si, err := h.svc.CollectSupplierIssue(r.Context(), claims.TenantID, id, req.AuthCode, claims.UserID, corrID(r))
	if err != nil {
		if strings.Contains(err.Error(), "not found") || strings.Contains(err.Error(), "invalid auth") {
			apierrors.Write(w, http.StatusBadRequest, "BAD_REQUEST", err.Error(), httpx.CorrelationID(r.Context()))
			return
		}
		apierrors.Write(w, http.StatusConflict, "CONFLICT", err.Error(), httpx.CorrelationID(r.Context()))
		return
	}
	httpx.JSON(w, http.StatusOK, si)
}

func (h *Handler) listOrphans(w http.ResponseWriter, r *http.Request) {
	claims, _ := authz.FromContext(r.Context())
	items, err := h.svc.OrphanIssues(r.Context(), claims.TenantID)
	if err != nil {
		apierrors.Write(w, http.StatusInternalServerError, "INTERNAL", err.Error(), httpx.CorrelationID(r.Context()))
		return
	}
	if items == nil {
		items = []SupplierIssue{}
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"items": items})
}

func (h *Handler) listPendingReconciliation(w http.ResponseWriter, r *http.Request) {
	claims, _ := authz.FromContext(r.Context())
	items, err := h.svc.ListPendingReconciliation(r.Context(), claims.TenantID)
	if err != nil {
		apierrors.Write(w, http.StatusInternalServerError, "INTERNAL", err.Error(), httpx.CorrelationID(r.Context()))
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"items": items})
}

func (h *Handler) listSuppliers(w http.ResponseWriter, r *http.Request) {
	claims, ok := authz.FromContext(r.Context())
	if !ok {
		apierrors.Write(w, http.StatusUnauthorized, "UNAUTHORIZED", "missing claims", httpx.CorrelationID(r.Context()))
		return
	}
	// Technicians requesting parts need the supplier list even without suppliers.write.
	if !claims.HasPermission("suppliers.read") && !claims.HasPermission("parts.request") {
		apierrors.Write(w, http.StatusForbidden, "FORBIDDEN", "permission denied", httpx.CorrelationID(r.Context()))
		return
	}
	items, err := h.svc.ListSuppliers(r.Context(), claims.TenantID)
	if err != nil {
		apierrors.Write(w, http.StatusInternalServerError, "INTERNAL", err.Error(), httpx.CorrelationID(r.Context()))
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"items": items})
}

func (h *Handler) listSupplierCredit(w http.ResponseWriter, r *http.Request) {
	claims, _ := authz.FromContext(r.Context())
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		apierrors.Write(w, http.StatusBadRequest, "BAD_REQUEST", "invalid id", httpx.CorrelationID(r.Context()))
		return
	}
	items, err := h.svc.ListCreditEntries(r.Context(), claims.TenantID, id)
	if err != nil {
		apierrors.Write(w, http.StatusInternalServerError, "INTERNAL", err.Error(), httpx.CorrelationID(r.Context()))
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"items": items})
}

func (h *Handler) reconcileIssue(w http.ResponseWriter, r *http.Request) {
	claims, _ := authz.FromContext(r.Context())
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		apierrors.Write(w, http.StatusBadRequest, "BAD_REQUEST", "invalid id", httpx.CorrelationID(r.Context()))
		return
	}
	si, err := h.svc.ReconcileSupplierIssue(r.Context(), claims.TenantID, id, claims.UserID, corrID(r))
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			apierrors.Write(w, http.StatusNotFound, "NOT_FOUND", err.Error(), httpx.CorrelationID(r.Context()))
			return
		}
		apierrors.Write(w, http.StatusConflict, "CONFLICT", err.Error(), httpx.CorrelationID(r.Context()))
		return
	}
	httpx.JSON(w, http.StatusOK, si)
}

func (h *Handler) listCategories(w http.ResponseWriter, r *http.Request) {
	claims, _ := authz.FromContext(r.Context())
	if r.URL.Query().Get("tree") == "1" {
		items, err := h.svc.ListCategoryTree(r.Context(), claims.TenantID)
		if err != nil {
			apierrors.Write(w, http.StatusInternalServerError, "INTERNAL", err.Error(), httpx.CorrelationID(r.Context()))
			return
		}
		httpx.JSON(w, http.StatusOK, map[string]any{"items": items})
		return
	}
	items, err := h.svc.ListCategories(r.Context(), claims.TenantID)
	if err != nil {
		apierrors.Write(w, http.StatusInternalServerError, "INTERNAL", err.Error(), httpx.CorrelationID(r.Context()))
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"items": items})
}

func (h *Handler) createCategory(w http.ResponseWriter, r *http.Request) {
	claims, _ := authz.FromContext(r.Context())
	var req struct {
		Name     string     `json:"name"`
		ParentID *uuid.UUID `json:"parent_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || strings.TrimSpace(req.Name) == "" {
		apierrors.Write(w, http.StatusBadRequest, "BAD_REQUEST", "name required", httpx.CorrelationID(r.Context()))
		return
	}
	c, err := h.svc.CreateCategory(r.Context(), claims.TenantID, req.Name, req.ParentID)
	if err != nil {
		status := http.StatusInternalServerError
		if strings.Contains(err.Error(), "already exists") || strings.Contains(err.Error(), "not found") || strings.Contains(err.Error(), "required") {
			status = http.StatusBadRequest
		}
		apierrors.Write(w, status, "CATEGORY_FAILED", err.Error(), httpx.CorrelationID(r.Context()))
		return
	}
	httpx.JSON(w, http.StatusCreated, c)
}

func (h *Handler) updateCategory(w http.ResponseWriter, r *http.Request) {
	claims, _ := authz.FromContext(r.Context())
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		apierrors.Write(w, http.StatusBadRequest, "BAD_REQUEST", "invalid id", httpx.CorrelationID(r.Context()))
		return
	}
	var req struct {
		Name     *string    `json:"name"`
		ParentID *uuid.UUID `json:"parent_id"`
		// ClearParent moves the category to the root when true.
		ClearParent bool `json:"clear_parent"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		apierrors.Write(w, http.StatusBadRequest, "BAD_REQUEST", "invalid body", httpx.CorrelationID(r.Context()))
		return
	}
	var parentArg **uuid.UUID
	if req.ClearParent {
		var nilID *uuid.UUID
		parentArg = &nilID
	} else if req.ParentID != nil {
		parentArg = &req.ParentID
	}
	c, err := h.svc.UpdateCategory(r.Context(), claims.TenantID, id, req.Name, parentArg)
	if err != nil {
		status := http.StatusInternalServerError
		if strings.Contains(err.Error(), "not found") {
			status = http.StatusNotFound
		} else if strings.Contains(err.Error(), "cannot") || strings.Contains(err.Error(), "already exists") || strings.Contains(err.Error(), "required") {
			status = http.StatusBadRequest
		}
		apierrors.Write(w, status, "CATEGORY_FAILED", err.Error(), httpx.CorrelationID(r.Context()))
		return
	}
	httpx.JSON(w, http.StatusOK, c)
}

func (h *Handler) deleteCategory(w http.ResponseWriter, r *http.Request) {
	claims, _ := authz.FromContext(r.Context())
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		apierrors.Write(w, http.StatusBadRequest, "BAD_REQUEST", "invalid id", httpx.CorrelationID(r.Context()))
		return
	}
	if err := h.svc.DeleteCategory(r.Context(), claims.TenantID, id); err != nil {
		status := http.StatusInternalServerError
		if strings.Contains(err.Error(), "not found") {
			status = http.StatusNotFound
		} else if strings.Contains(err.Error(), "first") {
			status = http.StatusConflict
		}
		apierrors.Write(w, status, "CATEGORY_FAILED", err.Error(), httpx.CorrelationID(r.Context()))
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) listProducts(w http.ResponseWriter, r *http.Request) {
	claims, _ := authz.FromContext(r.Context())
	items, err := h.svc.ListProducts(r.Context(), claims.TenantID)
	if err != nil {
		apierrors.Write(w, http.StatusInternalServerError, "INTERNAL", err.Error(), httpx.CorrelationID(r.Context()))
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"items": items})
}

func (h *Handler) createProduct(w http.ResponseWriter, r *http.Request) {
	claims, _ := authz.FromContext(r.Context())
	var req struct {
		Name        string     `json:"name"`
		Brand       *string    `json:"brand"`
		CategoryID  *uuid.UUID `json:"category_id"`
		Category    *string    `json:"category"`
		Description *string    `json:"description"`
		ImageURL    *string    `json:"image_url"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Name == "" {
		apierrors.Write(w, http.StatusBadRequest, "BAD_REQUEST", "name required", httpx.CorrelationID(r.Context()))
		return
	}
	p, err := h.svc.CreateProduct(
		r.Context(), claims.TenantID, req.Name, req.Brand, req.CategoryID, req.Category, req.Description, req.ImageURL,
	)
	if err != nil {
		status := http.StatusInternalServerError
		if strings.Contains(err.Error(), "not found") || strings.Contains(err.Error(), "required") {
			status = http.StatusBadRequest
		}
		apierrors.Write(w, status, "PRODUCT_FAILED", err.Error(), httpx.CorrelationID(r.Context()))
		return
	}
	httpx.JSON(w, http.StatusCreated, p)
}

func (h *Handler) updateProduct(w http.ResponseWriter, r *http.Request) {
	claims, _ := authz.FromContext(r.Context())
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		apierrors.Write(w, http.StatusBadRequest, "BAD_REQUEST", "invalid id", httpx.CorrelationID(r.Context()))
		return
	}
	var req struct {
		Name                *string    `json:"name"`
		Brand               *string    `json:"brand"`
		CategoryID          *uuid.UUID `json:"category_id"`
		ClearCategory       bool       `json:"clear_category"`
		Category            *string    `json:"category"`
		Description         *string    `json:"description"`
		ImageURL            *string    `json:"image_url"`
		POSVisible          *bool      `json:"pos_visible"`
		OnlineVisible       *bool      `json:"online_visible"`
		Featured            *bool      `json:"featured"`
		NewArrival          *bool      `json:"new_arrival"`
		Bestseller          *bool      `json:"bestseller"`
		StorefrontSortOrder *int       `json:"storefront_sort_order"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		apierrors.Write(w, http.StatusBadRequest, "BAD_REQUEST", "invalid body", httpx.CorrelationID(r.Context()))
		return
	}
	var categoryIDArg **uuid.UUID
	if req.ClearCategory {
		var nilID *uuid.UUID
		categoryIDArg = &nilID
	} else if req.CategoryID != nil {
		categoryIDArg = &req.CategoryID
	}
	product, err := h.svc.UpdateProduct(
		r.Context(), claims.TenantID, id, req.Name, req.Brand, categoryIDArg, req.Category,
		req.Description, req.ImageURL, req.POSVisible, req.OnlineVisible,
		req.Featured, req.NewArrival, req.Bestseller, req.StorefrontSortOrder,
	)
	if err != nil {
		status := http.StatusInternalServerError
		if strings.Contains(err.Error(), "not found") {
			status = http.StatusNotFound
		}
		apierrors.Write(w, status, "PRODUCT_FAILED", err.Error(), httpx.CorrelationID(r.Context()))
		return
	}
	httpx.JSON(w, http.StatusOK, product)
}

func readProductImageUpload(r *http.Request) ([]byte, string, error) {
	contentType := r.Header.Get("Content-Type")
	if strings.HasPrefix(contentType, "multipart/form-data") {
		if err := r.ParseMultipartForm(maxImageBytes + 8192); err != nil {
			return nil, "", errors.New("could not read the uploaded file")
		}
		file, header, err := r.FormFile("file")
		if err != nil {
			return nil, "", errors.New("attach the image as the 'file' field")
		}
		defer file.Close()
		body, err := io.ReadAll(io.LimitReader(file, maxImageBytes+1))
		if err != nil {
			return nil, "", errors.New("could not read the uploaded file")
		}
		declared := ""
		if header != nil {
			declared = header.Header.Get("Content-Type")
		}
		return body, declared, nil
	}
	body, err := io.ReadAll(io.LimitReader(r.Body, maxImageBytes+1))
	if err != nil {
		return nil, "", errors.New("could not read the uploaded file")
	}
	return body, contentType, nil
}

func (h *Handler) uploadProductImage(w http.ResponseWriter, r *http.Request) {
	claims, _ := authz.FromContext(r.Context())
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		apierrors.Write(w, http.StatusBadRequest, "BAD_REQUEST", "invalid id", httpx.CorrelationID(r.Context()))
		return
	}
	body, contentType, err := readProductImageUpload(r)
	if err != nil {
		apierrors.Write(w, http.StatusBadRequest, "BAD_REQUEST", err.Error(), httpx.CorrelationID(r.Context()))
		return
	}
	if err := h.svc.SaveProductImage(r.Context(), claims.TenantID, id, body, contentType); err != nil {
		status := http.StatusBadRequest
		if strings.Contains(err.Error(), "not found") {
			status = http.StatusNotFound
		}
		apierrors.Write(w, status, "IMAGE_FAILED", err.Error(), httpx.CorrelationID(r.Context()))
		return
	}
	items, err := h.svc.ListProducts(r.Context(), claims.TenantID)
	if err != nil {
		apierrors.Write(w, http.StatusInternalServerError, "INTERNAL", err.Error(), httpx.CorrelationID(r.Context()))
		return
	}
	for _, p := range items {
		if p.ID == id {
			httpx.JSON(w, http.StatusOK, p)
			return
		}
	}
	apierrors.Write(w, http.StatusNotFound, "NOT_FOUND", "product not found", httpx.CorrelationID(r.Context()))
}

func (h *Handler) deleteProductImage(w http.ResponseWriter, r *http.Request) {
	claims, _ := authz.FromContext(r.Context())
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		apierrors.Write(w, http.StatusBadRequest, "BAD_REQUEST", "invalid id", httpx.CorrelationID(r.Context()))
		return
	}
	if err := h.svc.DeleteProductImage(r.Context(), claims.TenantID, id); err != nil {
		apierrors.Write(w, http.StatusBadRequest, "IMAGE_FAILED", err.Error(), httpx.CorrelationID(r.Context()))
		return
	}
	items, err := h.svc.ListProducts(r.Context(), claims.TenantID)
	if err != nil {
		apierrors.Write(w, http.StatusInternalServerError, "INTERNAL", err.Error(), httpx.CorrelationID(r.Context()))
		return
	}
	for _, p := range items {
		if p.ID == id {
			httpx.JSON(w, http.StatusOK, p)
			return
		}
	}
	apierrors.Write(w, http.StatusNotFound, "NOT_FOUND", "product not found", httpx.CorrelationID(r.Context()))
}

func (h *Handler) publicProductImage(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("productID"))
	if err != nil {
		apierrors.Write(w, http.StatusBadRequest, "BAD_REQUEST", "invalid id", httpx.CorrelationID(r.Context()))
		return
	}
	body, contentType, err := h.svc.ProductImage(r.Context(), id)
	if err != nil {
		apierrors.Write(w, http.StatusNotFound, "NOT_FOUND", "image not found", httpx.CorrelationID(r.Context()))
		return
	}
	if contentType == "" {
		contentType = "image/jpeg"
	}
	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Cache-Control", "public, max-age=86400")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(body)
}

func (h *Handler) listVariants(w http.ResponseWriter, r *http.Request) {
	claims, _ := authz.FromContext(r.Context())
	var productID *uuid.UUID
	if s := r.URL.Query().Get("product_id"); s != "" {
		id, err := uuid.Parse(s)
		if err != nil {
			apierrors.Write(w, http.StatusBadRequest, "BAD_REQUEST", "invalid product_id", httpx.CorrelationID(r.Context()))
			return
		}
		productID = &id
	}
	items, err := h.svc.ListVariants(r.Context(), claims.TenantID, productID)
	if err != nil {
		apierrors.Write(w, http.StatusInternalServerError, "INTERNAL", err.Error(), httpx.CorrelationID(r.Context()))
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"items": items})
}

func (h *Handler) listCatalog(w http.ResponseWriter, r *http.Request) {
	claims, _ := authz.FromContext(r.Context())
	var locID *uuid.UUID
	if s := r.URL.Query().Get("location_id"); s != "" {
		id, err := uuid.Parse(s)
		if err != nil {
			apierrors.Write(w, http.StatusBadRequest, "BAD_REQUEST", "invalid location_id", httpx.CorrelationID(r.Context()))
			return
		}
		locID = &id
	}
	items, err := h.svc.ListPOSCatalog(r.Context(), claims.TenantID, locID)
	if err != nil {
		apierrors.Write(w, http.StatusInternalServerError, "INTERNAL", err.Error(), httpx.CorrelationID(r.Context()))
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"items": items})
}

func (h *Handler) listLocations(w http.ResponseWriter, r *http.Request) {
	claims, _ := authz.FromContext(r.Context())
	var branchID *uuid.UUID
	if s := r.URL.Query().Get("branch_id"); s != "" {
		id, err := uuid.Parse(s)
		if err != nil {
			apierrors.Write(w, http.StatusBadRequest, "BAD_REQUEST", "invalid branch_id", httpx.CorrelationID(r.Context()))
			return
		}
		branchID = &id
	}
	items, err := h.svc.ListStockLocations(r.Context(), claims.TenantID, branchID)
	if err != nil {
		apierrors.Write(w, http.StatusInternalServerError, "INTERNAL", err.Error(), httpx.CorrelationID(r.Context()))
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"items": items})
}

func (h *Handler) ensureLocations(w http.ResponseWriter, r *http.Request) {
	claims, _ := authz.FromContext(r.Context())
	if _, _, err := h.svc.EnsureStockLocations(r.Context(), claims.TenantID); err != nil {
		apierrors.Write(w, http.StatusInternalServerError, "INTERNAL", err.Error(), httpx.CorrelationID(r.Context()))
		return
	}
	items, err := h.svc.ListStockLocations(r.Context(), claims.TenantID, nil)
	if err != nil {
		apierrors.Write(w, http.StatusInternalServerError, "INTERNAL", err.Error(), httpx.CorrelationID(r.Context()))
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"items": items})
}

func (h *Handler) createVariant(w http.ResponseWriter, r *http.Request) {
	claims, _ := authz.FromContext(r.Context())
	var req struct {
		ProductID uuid.UUID `json:"product_id"`
		SKU       string    `json:"sku"`
		SellPrice float64   `json:"sell_price"`
		CostPrice float64   `json:"cost_price"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.ProductID == uuid.Nil || req.SKU == "" {
		apierrors.Write(w, http.StatusBadRequest, "BAD_REQUEST", "product_id and sku required", httpx.CorrelationID(r.Context()))
		return
	}
	v, err := h.svc.CreateVariant(r.Context(), claims.TenantID, req.ProductID, req.SKU, req.SellPrice, req.CostPrice)
	if err != nil {
		msg := err.Error()
		status := http.StatusInternalServerError
		if strings.Contains(msg, "duplicate") || strings.Contains(msg, "unique") {
			status = http.StatusConflict
			msg = "SKU already exists — pick a different code"
		}
		apierrors.Write(w, status, "VARIANT_FAILED", msg, httpx.CorrelationID(r.Context()))
		return
	}
	httpx.JSON(w, http.StatusCreated, v)
}

func (h *Handler) updateVariant(w http.ResponseWriter, r *http.Request) {
	claims, _ := authz.FromContext(r.Context())
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		apierrors.Write(w, http.StatusBadRequest, "BAD_REQUEST", "invalid id", httpx.CorrelationID(r.Context()))
		return
	}
	var req struct {
		SKU       *string  `json:"sku"`
		SellPrice *float64 `json:"sell_price"`
		CostPrice *float64 `json:"cost_price"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		apierrors.Write(w, http.StatusBadRequest, "BAD_REQUEST", "invalid body", httpx.CorrelationID(r.Context()))
		return
	}
	variant, err := h.svc.UpdateVariant(r.Context(), claims.TenantID, id, req.SKU, req.SellPrice, req.CostPrice)
	if err != nil {
		status := http.StatusInternalServerError
		if strings.Contains(err.Error(), "not found") {
			status = http.StatusNotFound
		}
		apierrors.Write(w, status, "VARIANT_FAILED", err.Error(), httpx.CorrelationID(r.Context()))
		return
	}
	httpx.JSON(w, http.StatusOK, variant)
}

func (h *Handler) listBalances(w http.ResponseWriter, r *http.Request) {
	claims, _ := authz.FromContext(r.Context())
	var locID *uuid.UUID
	if s := r.URL.Query().Get("location_id"); s != "" {
		id, err := uuid.Parse(s)
		if err != nil {
			apierrors.Write(w, http.StatusBadRequest, "BAD_REQUEST", "invalid location_id", httpx.CorrelationID(r.Context()))
			return
		}
		locID = &id
	}
	items, err := h.svc.ListBalances(r.Context(), claims.TenantID, locID)
	if err != nil {
		apierrors.Write(w, http.StatusInternalServerError, "INTERNAL", err.Error(), httpx.CorrelationID(r.Context()))
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"items": items})
}

func (h *Handler) listMovements(w http.ResponseWriter, r *http.Request) {
	claims, _ := authz.FromContext(r.Context())
	var locID *uuid.UUID
	if s := r.URL.Query().Get("location_id"); s != "" {
		id, err := uuid.Parse(s)
		if err != nil {
			apierrors.Write(w, http.StatusBadRequest, "BAD_REQUEST", "invalid location_id", httpx.CorrelationID(r.Context()))
			return
		}
		locID = &id
	}
	items, err := h.svc.ListMovements(r.Context(), claims.TenantID, locID, 40)
	if err != nil {
		apierrors.Write(w, http.StatusInternalServerError, "INTERNAL", err.Error(), httpx.CorrelationID(r.Context()))
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"items": items})
}

func (h *Handler) receive(w http.ResponseWriter, r *http.Request) {
	claims, _ := authz.FromContext(r.Context())
	if !claims.HasPermission("inventory.adjust") && !claims.HasPermission("*") {
		apierrors.Write(w, http.StatusForbidden, "FORBIDDEN", "inventory.adjust required", httpx.CorrelationID(r.Context()))
		return
	}
	var req struct {
		VariantID  uuid.UUID `json:"variant_id"`
		LocationID uuid.UUID `json:"location_id"`
		Quantity   int       `json:"quantity"`
		Note       string    `json:"note"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.VariantID == uuid.Nil || req.LocationID == uuid.Nil || req.Quantity <= 0 {
		apierrors.Write(w, http.StatusBadRequest, "BAD_REQUEST", "variant_id, location_id, and positive quantity required", httpx.CorrelationID(r.Context()))
		return
	}
	if err := h.svc.ReceiveStock(r.Context(), claims.TenantID, req.VariantID, req.LocationID, req.Quantity, claims.UserID, corrID(r), req.Note); err != nil {
		apierrors.Write(w, http.StatusConflict, "CONFLICT", err.Error(), httpx.CorrelationID(r.Context()))
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) adjust(w http.ResponseWriter, r *http.Request) {
	claims, _ := authz.FromContext(r.Context())
	if !claims.HasPermission("inventory.adjust") && !claims.HasPermission("*") {
		apierrors.Write(w, http.StatusForbidden, "FORBIDDEN", "inventory.adjust required", httpx.CorrelationID(r.Context()))
		return
	}
	var req struct {
		VariantID  uuid.UUID `json:"variant_id"`
		LocationID uuid.UUID `json:"location_id"`
		QtyDelta   int       `json:"qty_delta"`
		Reason     string    `json:"reason"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.VariantID == uuid.Nil || req.LocationID == uuid.Nil || req.QtyDelta == 0 {
		apierrors.Write(w, http.StatusBadRequest, "BAD_REQUEST", "variant_id, location_id, and non-zero qty_delta required", httpx.CorrelationID(r.Context()))
		return
	}
	if err := h.svc.AdjustStock(r.Context(), claims.TenantID, req.VariantID, req.LocationID, req.QtyDelta, claims.UserID, corrID(r), req.Reason); err != nil {
		apierrors.Write(w, http.StatusConflict, "CONFLICT", err.Error(), httpx.CorrelationID(r.Context()))
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) reserve(w http.ResponseWriter, r *http.Request) {
	claims, _ := authz.FromContext(r.Context())
	var req struct {
		VariantID  uuid.UUID `json:"variant_id"`
		LocationID uuid.UUID `json:"location_id"`
		Quantity   int       `json:"quantity"`
		TTLSeconds int       `json:"ttl_seconds"`
		RefType    string    `json:"reference_type"`
		RefID      uuid.UUID `json:"reference_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.VariantID == uuid.Nil || req.LocationID == uuid.Nil {
		apierrors.Write(w, http.StatusBadRequest, "BAD_REQUEST", "variant_id and location_id required", httpx.CorrelationID(r.Context()))
		return
	}
	ttl := time.Duration(req.TTLSeconds) * time.Second
	if ttl <= 0 {
		ttl = 15 * time.Minute
	}
	res, err := h.svc.ReserveInventory(r.Context(), claims.TenantID, req.VariantID, req.LocationID, req.Quantity, ttl, req.RefType, req.RefID)
	if err != nil {
		apierrors.Write(w, http.StatusConflict, "CONFLICT", err.Error(), httpx.CorrelationID(r.Context()))
		return
	}
	httpx.JSON(w, http.StatusCreated, res)
}

func (h *Handler) transfer(w http.ResponseWriter, r *http.Request) {
	claims, _ := authz.FromContext(r.Context())
	if !claims.HasPermission("inventory.adjust") && !claims.HasPermission("*") {
		apierrors.Write(w, http.StatusForbidden, "FORBIDDEN", "inventory.adjust required", httpx.CorrelationID(r.Context()))
		return
	}
	var req struct {
		VariantID uuid.UUID `json:"variant_id"`
		FromLoc   uuid.UUID `json:"from_location_id"`
		ToLoc     uuid.UUID `json:"to_location_id"`
		Quantity  int       `json:"quantity"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.VariantID == uuid.Nil || req.FromLoc == uuid.Nil || req.ToLoc == uuid.Nil || req.Quantity <= 0 {
		apierrors.Write(w, http.StatusBadRequest, "BAD_REQUEST", "variant_id, from_location_id, to_location_id, and positive quantity required", httpx.CorrelationID(r.Context()))
		return
	}
	if err := h.svc.TransferStock(r.Context(), claims.TenantID, req.VariantID, req.FromLoc, req.ToLoc, req.Quantity, claims.UserID, corrID(r)); err != nil {
		status := http.StatusConflict
		if strings.Contains(err.Error(), "must differ") || strings.Contains(err.Error(), "invalid location") {
			status = http.StatusBadRequest
		}
		apierrors.Write(w, status, "CONFLICT", err.Error(), httpx.CorrelationID(r.Context()))
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func corrID(r *http.Request) uuid.UUID {
	if cid := httpx.CorrelationID(r.Context()); cid != "" {
		if id, err := uuid.Parse(cid); err == nil {
			return id
		}
	}
	return uuid.New()
}
