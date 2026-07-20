package repair

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
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
	mux.Handle("POST /customers", auth(http.HandlerFunc(h.createCustomer)))
	mux.Handle("GET /customers", auth(http.HandlerFunc(h.listCustomers)))
	mux.Handle("GET /customers/{id}", auth(http.HandlerFunc(h.getCustomer)))
	mux.HandleFunc("GET /public/repairs/status", h.publicRepairStatus)

	// Customer phone-OTP (public; Bearer session after verify).
	mux.HandleFunc("POST /customer/auth/otp/request", h.customerOTPRequest)
	mux.HandleFunc("POST /customer/auth/otp/verify", h.customerOTPVerify)
	mux.HandleFunc("POST /customer/auth/logout", h.customerLogout)
	mux.HandleFunc("GET /customer/me", h.customerMe)
	mux.HandleFunc("GET /customer/repairs", h.customerListRepairs)
	mux.HandleFunc("GET /customer/repairs/{id}", h.customerGetRepair)
	mux.HandleFunc("GET /customer/repairs/{id}/receipt", h.customerRepairReceipt)
	mux.HandleFunc("GET /customer/repairs/{id}/receipt.html", h.customerRepairReceiptHTML)
	mux.HandleFunc("GET /customer/repairs/{id}/receipt.pdf", h.customerRepairReceiptPDF)
	mux.HandleFunc("GET /customer/repairs/{id}/tax-invoice.pdf", h.customerRepairTaxInvoicePDF)
	mux.HandleFunc("GET /customer/repairs/{id}/warranty", h.customerGetWarranty)
	mux.HandleFunc("POST /customer/repairs/{id}/warranty/claim", h.customerClaimWarranty)
	mux.HandleFunc("POST /customer/repairs/{id}/estimates/{estimate_id}/approve", h.customerApproveEstimate)
	mux.HandleFunc("POST /customer/repairs/{id}/estimates/{estimate_id}/reject", h.customerRejectEstimate)

	mux.Handle("POST /devices", auth(http.HandlerFunc(h.createDevice)))
	mux.Handle("POST /repairs", auth(http.HandlerFunc(h.createRepair)))
	mux.Handle("GET /repairs", auth(http.HandlerFunc(h.listRepairs)))
	mux.Handle("GET /repairs/{id}", auth(http.HandlerFunc(h.getRepair)))
	mux.Handle("GET /repairs/{id}/receipt", auth(http.HandlerFunc(h.staffRepairReceipt)))
	mux.Handle("GET /repairs/{id}/receipt.html", auth(http.HandlerFunc(h.staffRepairReceiptHTML)))
	mux.Handle("GET /repairs/{id}/receipt.pdf", auth(http.HandlerFunc(h.staffRepairReceiptPDF)))
	mux.Handle("GET /repairs/{id}/tax-invoice.pdf", auth(http.HandlerFunc(h.staffRepairTaxInvoicePDF)))
	mux.Handle("GET /repairs/{id}/warranty", auth(http.HandlerFunc(h.getWarranty)))
	mux.Handle("POST /repairs/{id}/warranty", auth(http.HandlerFunc(h.createWarranty)))
	mux.Handle("POST /repairs/{id}/warranty/claim", auth(http.HandlerFunc(h.claimWarrantyStaff)))
	mux.Handle("POST /repairs/{id}/estimates", auth(http.HandlerFunc(h.createEstimate)))
	mux.Handle("GET /repairs/{id}/estimates", auth(http.HandlerFunc(h.listEstimates)))
	mux.Handle("GET /sms/settings", auth(http.HandlerFunc(h.getSMSSettings)))
	mux.Handle("PUT /sms/settings", auth(http.HandlerFunc(h.putSMSSettings)))
	mux.Handle("POST /repairs/{id}/assign", auth(http.HandlerFunc(h.assignRepair)))
	mux.Handle("POST /repairs/{id}/status", auth(http.HandlerFunc(h.changeStatus)))
	mux.Handle("DELETE /repairs/{id}", auth(http.HandlerFunc(h.deleteRepair)))
	mux.Handle("POST /repairs/{id}/notes", auth(http.HandlerFunc(h.addNote)))
	mux.Handle("GET /repairs/{id}/notes", auth(http.HandlerFunc(h.listNotes)))
	mux.Handle("POST /repairs/{id}/attachments", auth(http.HandlerFunc(h.addAttachment)))
	mux.Handle("POST /repairs/{id}/attachments/presign", auth(http.HandlerFunc(h.initiatePresignedAttachment)))
	mux.Handle("GET /repairs/{id}/attachments", auth(http.HandlerFunc(h.listAttachments)))
	mux.Handle("GET /repairs/{id}/attachments/{attachment_id}/content", auth(http.HandlerFunc(h.getAttachmentContent)))
	mux.Handle("POST /repairs/{id}/attachments/{attachment_id}/complete", auth(http.HandlerFunc(h.completePresignedAttachment)))
	mux.Handle("DELETE /repairs/{id}/attachments/{attachment_id}", auth(http.HandlerFunc(h.deleteAttachment)))
}

func (h *Handler) createCustomer(w http.ResponseWriter, r *http.Request) {
	claims, ok := authz.FromContext(r.Context())
	if !ok {
		apierrors.Write(w, http.StatusUnauthorized, "UNAUTHORIZED", "missing claims", httpx.CorrelationID(r.Context()))
		return
	}
	var req struct {
		FullName string  `json:"full_name"`
		Phone    *string `json:"phone"`
		Email    *string `json:"email"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.FullName == "" {
		apierrors.Write(w, http.StatusBadRequest, "BAD_REQUEST", "full_name required", httpx.CorrelationID(r.Context()))
		return
	}
	corrID := parseCorrID(r)
	c, err := h.svc.CreateCustomer(r.Context(), CreateCustomerInput{
		FullName: req.FullName, Phone: req.Phone, Email: req.Email,
		ActorID: claims.UserID, TenantID: claims.TenantID, CorrID: corrID,
	})
	if err != nil {
		apierrors.Write(w, http.StatusInternalServerError, "INTERNAL", err.Error(), httpx.CorrelationID(r.Context()))
		return
	}
	httpx.JSON(w, http.StatusCreated, c)
}

func (h *Handler) listCustomers(w http.ResponseWriter, r *http.Request) {
	claims, ok := authz.FromContext(r.Context())
	if !ok {
		apierrors.Write(w, http.StatusUnauthorized, "UNAUTHORIZED", "missing claims", httpx.CorrelationID(r.Context()))
		return
	}
	q := r.URL.Query().Get("q")
	items, err := h.svc.ListCustomers(r.Context(), claims.TenantID, q, 20)
	if err != nil {
		apierrors.Write(w, http.StatusInternalServerError, "INTERNAL", err.Error(), httpx.CorrelationID(r.Context()))
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"items": items})
}

func (h *Handler) getCustomer(w http.ResponseWriter, r *http.Request) {
	claims, _ := authz.FromContext(r.Context())
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		apierrors.Write(w, http.StatusBadRequest, "BAD_REQUEST", "invalid id", httpx.CorrelationID(r.Context()))
		return
	}
	customer, devices, repairs, err := h.svc.GetCustomer(r.Context(), claims.TenantID, id)
	if err != nil {
		status := http.StatusInternalServerError
		if strings.Contains(err.Error(), "not found") {
			status = http.StatusNotFound
		}
		apierrors.Write(w, status, "CUSTOMER_FAILED", err.Error(), httpx.CorrelationID(r.Context()))
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{
		"customer": customer,
		"devices":  devices,
		"repairs":  repairs,
	})
}

func (h *Handler) publicRepairStatus(w http.ResponseWriter, r *http.Request) {
	jobCode := strings.TrimSpace(r.URL.Query().Get("job_code"))
	phone := strings.TrimSpace(r.URL.Query().Get("phone"))
	if jobCode == "" || phone == "" {
		apierrors.Write(w, http.StatusBadRequest, "BAD_REQUEST", "job_code and phone required", httpx.CorrelationID(r.Context()))
		return
	}
	job, tenantID, err := h.svc.PublicRepairStatus(r.Context(), jobCode, phone)
	if err != nil {
		status := http.StatusInternalServerError
		if strings.Contains(err.Error(), "not found") {
			status = http.StatusNotFound
		}
		apierrors.Write(w, status, "REPAIR_LOOKUP_FAILED", err.Error(), httpx.CorrelationID(r.Context()))
		return
	}
	receipts, err := h.svc.PublicRepairReceipts(r.Context(), tenantID, job.ID)
	if err != nil {
		apierrors.Write(w, http.StatusInternalServerError, "INTERNAL", err.Error(), httpx.CorrelationID(r.Context()))
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{
		"job_code":        job.JobCode,
		"status":          job.Status,
		"problem_summary": job.ProblemSummary,
		"device":          PublicDeviceView(job.Device),
		"timeline":        PublicTimelineView(job.Timeline),
		"created_at":      job.CreatedAt,
		"receipts":        receipts,
	})
}

func bearerToken(r *http.Request) string {
	value := strings.TrimSpace(r.Header.Get("Authorization"))
	if len(value) > 7 && strings.EqualFold(value[:7], "Bearer ") {
		return strings.TrimSpace(value[7:])
	}
	return ""
}

func (h *Handler) resolveCustomer(w http.ResponseWriter, r *http.Request) (uuid.UUID, *Customer, bool) {
	tenantID, err := h.svc.DefaultTenantID(r.Context())
	if err != nil {
		apierrors.Write(w, http.StatusServiceUnavailable, "UNAVAILABLE", err.Error(), httpx.CorrelationID(r.Context()))
		return uuid.Nil, nil, false
	}
	customer, err := h.svc.AuthenticateCustomer(r.Context(), tenantID, bearerToken(r))
	if err != nil {
		apierrors.Write(w, http.StatusUnauthorized, "UNAUTHORIZED", err.Error(), httpx.CorrelationID(r.Context()))
		return uuid.Nil, nil, false
	}
	return tenantID, customer, true
}

func (h *Handler) customerOTPRequest(w http.ResponseWriter, r *http.Request) {
	tenantID, err := h.svc.DefaultTenantID(r.Context())
	if err != nil {
		apierrors.Write(w, http.StatusServiceUnavailable, "UNAVAILABLE", err.Error(), httpx.CorrelationID(r.Context()))
		return
	}
	var req struct {
		Phone string `json:"phone"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || strings.TrimSpace(req.Phone) == "" {
		apierrors.Write(w, http.StatusBadRequest, "BAD_REQUEST", "phone required", httpx.CorrelationID(r.Context()))
		return
	}
	if err := h.svc.RequestOTP(r.Context(), tenantID, req.Phone); err != nil {
		status := http.StatusBadRequest
		if strings.Contains(err.Error(), "wait") || strings.Contains(err.Error(), "too many") {
			status = http.StatusTooManyRequests
		}
		apierrors.Write(w, status, "OTP_REQUEST_FAILED", err.Error(), httpx.CorrelationID(r.Context()))
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"status": "sent"})
}

func (h *Handler) customerOTPVerify(w http.ResponseWriter, r *http.Request) {
	tenantID, err := h.svc.DefaultTenantID(r.Context())
	if err != nil {
		apierrors.Write(w, http.StatusServiceUnavailable, "UNAVAILABLE", err.Error(), httpx.CorrelationID(r.Context()))
		return
	}
	var req struct {
		Phone string `json:"phone"`
		Code  string `json:"code"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || strings.TrimSpace(req.Phone) == "" || strings.TrimSpace(req.Code) == "" {
		apierrors.Write(w, http.StatusBadRequest, "BAD_REQUEST", "phone and code required", httpx.CorrelationID(r.Context()))
		return
	}
	session, err := h.svc.VerifyOTP(r.Context(), tenantID, req.Phone, req.Code)
	if err != nil {
		apierrors.Write(w, http.StatusUnauthorized, "OTP_VERIFY_FAILED", err.Error(), httpx.CorrelationID(r.Context()))
		return
	}
	httpx.JSON(w, http.StatusOK, session)
}

func (h *Handler) customerLogout(w http.ResponseWriter, r *http.Request) {
	tenantID, err := h.svc.DefaultTenantID(r.Context())
	if err != nil {
		apierrors.Write(w, http.StatusServiceUnavailable, "UNAVAILABLE", err.Error(), httpx.CorrelationID(r.Context()))
		return
	}
	if err := h.svc.LogoutCustomer(r.Context(), tenantID, bearerToken(r)); err != nil {
		apierrors.Write(w, http.StatusUnauthorized, "UNAUTHORIZED", err.Error(), httpx.CorrelationID(r.Context()))
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) customerMe(w http.ResponseWriter, r *http.Request) {
	_, customer, ok := h.resolveCustomer(w, r)
	if !ok {
		return
	}
	httpx.JSON(w, http.StatusOK, customer)
}

func (h *Handler) customerListRepairs(w http.ResponseWriter, r *http.Request) {
	tenantID, customer, ok := h.resolveCustomer(w, r)
	if !ok {
		return
	}
	items, err := h.svc.ListCustomerRepairs(r.Context(), tenantID, customer.ID)
	if err != nil {
		apierrors.Write(w, http.StatusInternalServerError, "INTERNAL", err.Error(), httpx.CorrelationID(r.Context()))
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"items": items})
}

func (h *Handler) customerRepairReceipt(w http.ResponseWriter, r *http.Request) {
	doc, ok := h.loadCustomerReceipt(w, r)
	if !ok {
		return
	}
	httpx.JSON(w, http.StatusOK, doc)
}

func (h *Handler) customerRepairReceiptHTML(w http.ResponseWriter, r *http.Request) {
	doc, ok := h.loadCustomerReceipt(w, r)
	if !ok {
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(doc.HTML()))
}

func (h *Handler) loadCustomerReceipt(w http.ResponseWriter, r *http.Request) (*CustomerReceiptDocument, bool) {
	tenantID, customer, ok := h.resolveCustomer(w, r)
	if !ok {
		return nil, false
	}
	repairID, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		apierrors.Write(w, http.StatusBadRequest, "BAD_REQUEST", "invalid id", httpx.CorrelationID(r.Context()))
		return nil, false
	}
	if err := h.svc.AssertCustomerOwnsRepair(r.Context(), tenantID, customer.ID, repairID); err != nil {
		status := http.StatusNotFound
		if !strings.Contains(err.Error(), "not found") {
			status = http.StatusForbidden
		}
		apierrors.Write(w, status, "FORBIDDEN", err.Error(), httpx.CorrelationID(r.Context()))
		return nil, false
	}
	doc, err := h.svc.BuildCustomerReceipt(r.Context(), tenantID, repairID)
	if err != nil {
		apierrors.Write(w, http.StatusInternalServerError, "INTERNAL", err.Error(), httpx.CorrelationID(r.Context()))
		return nil, false
	}
	return doc, true
}

func (h *Handler) staffRepairReceipt(w http.ResponseWriter, r *http.Request) {
	claims, ok := authz.FromContext(r.Context())
	if !ok {
		apierrors.Write(w, http.StatusUnauthorized, "UNAUTHORIZED", "missing claims", httpx.CorrelationID(r.Context()))
		return
	}
	repairID, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		apierrors.Write(w, http.StatusBadRequest, "BAD_REQUEST", "invalid id", httpx.CorrelationID(r.Context()))
		return
	}
	doc, err := h.svc.BuildCustomerReceipt(r.Context(), claims.TenantID, repairID)
	if err != nil {
		status := http.StatusInternalServerError
		if strings.Contains(err.Error(), "not found") {
			status = http.StatusNotFound
		}
		apierrors.Write(w, status, "RECEIPT_FAILED", err.Error(), httpx.CorrelationID(r.Context()))
		return
	}
	httpx.JSON(w, http.StatusOK, doc)
}

func (h *Handler) staffRepairReceiptHTML(w http.ResponseWriter, r *http.Request) {
	claims, ok := authz.FromContext(r.Context())
	if !ok {
		apierrors.Write(w, http.StatusUnauthorized, "UNAUTHORIZED", "missing claims", httpx.CorrelationID(r.Context()))
		return
	}
	repairID, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		apierrors.Write(w, http.StatusBadRequest, "BAD_REQUEST", "invalid id", httpx.CorrelationID(r.Context()))
		return
	}
	doc, err := h.svc.BuildCustomerReceipt(r.Context(), claims.TenantID, repairID)
	if err != nil {
		status := http.StatusInternalServerError
		if strings.Contains(err.Error(), "not found") {
			status = http.StatusNotFound
		}
		apierrors.Write(w, status, "RECEIPT_FAILED", err.Error(), httpx.CorrelationID(r.Context()))
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(doc.HTML()))
}

func (h *Handler) customerGetRepair(w http.ResponseWriter, r *http.Request) {
	tenantID, customer, ok := h.resolveCustomer(w, r)
	if !ok {
		return
	}
	repairID, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		apierrors.Write(w, http.StatusBadRequest, "BAD_REQUEST", "invalid id", httpx.CorrelationID(r.Context()))
		return
	}
	job, estimates, receipts, err := h.svc.GetCustomerRepair(r.Context(), tenantID, customer.ID, repairID)
	if err != nil {
		status := http.StatusInternalServerError
		if strings.Contains(err.Error(), "not found") {
			status = http.StatusNotFound
		}
		apierrors.Write(w, status, "REPAIR_FAILED", err.Error(), httpx.CorrelationID(r.Context()))
		return
	}
	var pending *RepairEstimate
	for i := range estimates {
		if estimates[i].Status == EstimatePending {
			pending = &estimates[i]
			break
		}
	}
	_, balance, _, _, balErr := h.svc.RepairPaymentContext(r.Context(), tenantID, repairID)
	if balErr != nil {
		balance = 0
	}
	deviceBrand, deviceModel := "", ""
	if job.Device != nil {
		if job.Device.Brand != nil {
			deviceBrand = *job.Device.Brand
		}
		if job.Device.Model != nil {
			deviceModel = *job.Device.Model
		}
	}
	httpx.JSON(w, http.StatusOK, map[string]any{
		"id":               job.ID,
		"job_number":       job.JobNumber,
		"job_code":         job.JobCode,
		"branch_id":        job.BranchID,
		"status":           job.Status,
		"problem_summary":  job.ProblemSummary,
		"labor_amount":     job.LaborAmount,
		"created_at":       job.CreatedAt,
		"device":           PublicDeviceView(job.Device),
		"device_brand":     deviceBrand,
		"device_model":     deviceModel,
		"customer_name":    job.CustomerName,
		"timeline":         PublicTimelineView(job.Timeline),
		"estimates":        estimates,
		"estimate":         pending,
		"pending_estimate": pending,
		"balance_due":      balance,
		"amount_due":       balance,
		"receipts":         receipts,
	})
}

func (h *Handler) customerApproveEstimate(w http.ResponseWriter, r *http.Request) {
	h.customerDecideEstimate(w, r, true)
}

func (h *Handler) customerRejectEstimate(w http.ResponseWriter, r *http.Request) {
	h.customerDecideEstimate(w, r, false)
}

func (h *Handler) customerDecideEstimate(w http.ResponseWriter, r *http.Request, approve bool) {
	tenantID, customer, ok := h.resolveCustomer(w, r)
	if !ok {
		return
	}
	repairID, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		apierrors.Write(w, http.StatusBadRequest, "BAD_REQUEST", "invalid repair id", httpx.CorrelationID(r.Context()))
		return
	}
	estimateID, err := uuid.Parse(r.PathValue("estimate_id"))
	if err != nil {
		apierrors.Write(w, http.StatusBadRequest, "BAD_REQUEST", "invalid estimate id", httpx.CorrelationID(r.Context()))
		return
	}
	var est *RepairEstimate
	if approve {
		est, err = h.svc.ApproveEstimate(r.Context(), tenantID, repairID, estimateID, customer.ID)
	} else {
		est, err = h.svc.RejectEstimate(r.Context(), tenantID, repairID, estimateID, customer.ID)
	}
	if err != nil {
		status := http.StatusConflict
		if strings.Contains(err.Error(), "not found") {
			status = http.StatusNotFound
		}
		apierrors.Write(w, status, "ESTIMATE_FAILED", err.Error(), httpx.CorrelationID(r.Context()))
		return
	}
	httpx.JSON(w, http.StatusOK, est)
}

func (h *Handler) createEstimate(w http.ResponseWriter, r *http.Request) {
	claims, ok := authz.FromContext(r.Context())
	if !ok {
		apierrors.Write(w, http.StatusUnauthorized, "UNAUTHORIZED", "missing claims", httpx.CorrelationID(r.Context()))
		return
	}
	repairID, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		apierrors.Write(w, http.StatusBadRequest, "BAD_REQUEST", "invalid id", httpx.CorrelationID(r.Context()))
		return
	}
	var req struct {
		LaborAmount  float64 `json:"labor_amount"`
		PartsAmount  float64 `json:"parts_amount"`
		Notes        *string `json:"notes"`
		ExpiresHours *int    `json:"expires_hours"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		apierrors.Write(w, http.StatusBadRequest, "BAD_REQUEST", "invalid body", httpx.CorrelationID(r.Context()))
		return
	}
	est, err := h.svc.CreateEstimate(r.Context(), CreateEstimateInput{
		TenantID: claims.TenantID, RepairJobID: repairID,
		LaborAmount: req.LaborAmount, PartsAmount: req.PartsAmount,
		Notes: req.Notes, ExpiresHours: req.ExpiresHours, ActorID: claims.UserID,
	})
	if err != nil {
		status := http.StatusBadRequest
		if strings.Contains(err.Error(), "not found") {
			status = http.StatusNotFound
		}
		apierrors.Write(w, status, "ESTIMATE_FAILED", err.Error(), httpx.CorrelationID(r.Context()))
		return
	}
	httpx.JSON(w, http.StatusCreated, est)
}

func (h *Handler) listEstimates(w http.ResponseWriter, r *http.Request) {
	claims, ok := authz.FromContext(r.Context())
	if !ok {
		apierrors.Write(w, http.StatusUnauthorized, "UNAUTHORIZED", "missing claims", httpx.CorrelationID(r.Context()))
		return
	}
	repairID, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		apierrors.Write(w, http.StatusBadRequest, "BAD_REQUEST", "invalid id", httpx.CorrelationID(r.Context()))
		return
	}
	items, err := h.svc.ListEstimates(r.Context(), claims.TenantID, repairID)
	if err != nil {
		apierrors.Write(w, http.StatusInternalServerError, "INTERNAL", err.Error(), httpx.CorrelationID(r.Context()))
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"items": items})
}

func (h *Handler) createDevice(w http.ResponseWriter, r *http.Request) {
	claims, ok := authz.FromContext(r.Context())
	if !ok {
		apierrors.Write(w, http.StatusUnauthorized, "UNAUTHORIZED", "missing claims", httpx.CorrelationID(r.Context()))
		return
	}
	var req struct {
		CustomerID   *uuid.UUID `json:"customer_id"`
		Anonymous    bool       `json:"anonymous"`
		Kind         string     `json:"kind"`
		Brand        *string    `json:"brand"`
		Model        *string    `json:"model"`
		IMEI         *string    `json:"imei"`
		SerialNumber *string    `json:"serial_number"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Kind == "" {
		apierrors.Write(w, http.StatusBadRequest, "BAD_REQUEST", "kind required", httpx.CorrelationID(r.Context()))
		return
	}
	d, err := h.svc.CreateDevice(r.Context(), CreateDeviceInput{
		CustomerID: req.CustomerID, Anonymous: req.Anonymous, Kind: req.Kind,
		Brand: req.Brand, Model: req.Model, IMEI: req.IMEI, SerialNumber: req.SerialNumber,
		ActorID: claims.UserID, TenantID: claims.TenantID, CorrID: parseCorrID(r),
	})
	if err != nil {
		apierrors.Write(w, http.StatusInternalServerError, "INTERNAL", err.Error(), httpx.CorrelationID(r.Context()))
		return
	}
	httpx.JSON(w, http.StatusCreated, d)
}

func (h *Handler) createRepair(w http.ResponseWriter, r *http.Request) {
	claims, ok := authz.FromContext(r.Context())
	if !ok {
		apierrors.Write(w, http.StatusUnauthorized, "UNAUTHORIZED", "missing claims", httpx.CorrelationID(r.Context()))
		return
	}
	var req struct {
		BranchID       uuid.UUID  `json:"branch_id"`
		CustomerID     *uuid.UUID `json:"customer_id"`
		DeviceID       uuid.UUID  `json:"device_id"`
		ProblemSummary string     `json:"problem_summary"`
		TechnicianID   *uuid.UUID `json:"technician_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.BranchID == uuid.Nil || req.DeviceID == uuid.Nil || req.ProblemSummary == "" {
		apierrors.Write(w, http.StatusBadRequest, "BAD_REQUEST", "branch_id, device_id, problem_summary required", httpx.CorrelationID(r.Context()))
		return
	}
	if !claims.InBranch(req.BranchID) {
		apierrors.Write(w, http.StatusForbidden, "FORBIDDEN", "branch access denied", httpx.CorrelationID(r.Context()))
		return
	}
	job, err := h.svc.CreateRepair(r.Context(), CreateRepairInput{
		BranchID: req.BranchID, CustomerID: req.CustomerID, DeviceID: req.DeviceID,
		ProblemSummary: req.ProblemSummary, TechnicianID: req.TechnicianID,
		ActorID: claims.UserID, TenantID: claims.TenantID, CorrID: parseCorrID(r),
	})
	if err != nil {
		apierrors.Write(w, http.StatusInternalServerError, "INTERNAL", err.Error(), httpx.CorrelationID(r.Context()))
		return
	}
	httpx.JSON(w, http.StatusCreated, job)
}

func (h *Handler) listRepairs(w http.ResponseWriter, r *http.Request) {
	claims, ok := authz.FromContext(r.Context())
	if !ok {
		apierrors.Write(w, http.StatusUnauthorized, "UNAUTHORIZED", "missing claims", httpx.CorrelationID(r.Context()))
		return
	}
	filter := ListRepairsFilter{
		Status: r.URL.Query().Get("status"),
		Search: r.URL.Query().Get("q"),
	}
	if s := r.URL.Query().Get("branch_id"); s != "" {
		id, err := uuid.Parse(s)
		if err != nil {
			apierrors.Write(w, http.StatusBadRequest, "BAD_REQUEST", "invalid branch_id", httpx.CorrelationID(r.Context()))
			return
		}
		filter.BranchID = &id
	}
	if s := r.URL.Query().Get("technician_id"); s != "" {
		if s == "me" {
			filter.TechnicianID = &claims.UserID
		} else {
			id, err := uuid.Parse(s)
			if err != nil {
				apierrors.Write(w, http.StatusBadRequest, "BAD_REQUEST", "invalid technician_id", httpx.CorrelationID(r.Context()))
				return
			}
			filter.TechnicianID = &id
		}
	}
	items, err := h.svc.ListRepairs(r.Context(), claims.TenantID, filter)
	if err != nil {
		apierrors.Write(w, http.StatusInternalServerError, "INTERNAL", err.Error(), httpx.CorrelationID(r.Context()))
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"items": items})
}

func (h *Handler) getRepair(w http.ResponseWriter, r *http.Request) {
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
	job, err := h.svc.GetRepair(r.Context(), claims.TenantID, id)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			apierrors.Write(w, http.StatusNotFound, "NOT_FOUND", err.Error(), httpx.CorrelationID(r.Context()))
			return
		}
		apierrors.Write(w, http.StatusInternalServerError, "INTERNAL", err.Error(), httpx.CorrelationID(r.Context()))
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{
		"id":              job.ID,
		"job_number":      job.JobNumber,
		"job_code":        job.JobCode,
		"branch_id":       job.BranchID,
		"customer_id":     job.CustomerID,
		"customer":        job.Customer,
		"device_id":       job.DeviceID,
		"device":          job.Device,
		"technician_id":   job.TechnicianID,
		"status":          job.Status,
		"problem_summary": job.ProblemSummary,
		"labor_amount":    job.LaborAmount,
		"created_at":      job.CreatedAt,
		"timeline":        job.Timeline,
		"next_statuses":   NextStatuses(job.Status),
	})
}

func (h *Handler) addNote(w http.ResponseWriter, r *http.Request) {
	claims, ok := authz.FromContext(r.Context())
	if !ok {
		apierrors.Write(w, http.StatusUnauthorized, "UNAUTHORIZED", "missing claims", httpx.CorrelationID(r.Context()))
		return
	}
	repairID, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		apierrors.Write(w, http.StatusBadRequest, "BAD_REQUEST", "invalid id", httpx.CorrelationID(r.Context()))
		return
	}
	var req struct {
		Note string `json:"note"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || strings.TrimSpace(req.Note) == "" {
		apierrors.Write(w, http.StatusBadRequest, "BAD_REQUEST", "note required", httpx.CorrelationID(r.Context()))
		return
	}
	n, err := h.svc.AddNote(r.Context(), claims.TenantID, repairID, strings.TrimSpace(req.Note), claims.UserID, parseCorrID(r), nil)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			apierrors.Write(w, http.StatusNotFound, "NOT_FOUND", err.Error(), httpx.CorrelationID(r.Context()))
			return
		}
		apierrors.Write(w, http.StatusInternalServerError, "INTERNAL", err.Error(), httpx.CorrelationID(r.Context()))
		return
	}
	httpx.JSON(w, http.StatusCreated, n)
}

func (h *Handler) listNotes(w http.ResponseWriter, r *http.Request) {
	claims, ok := authz.FromContext(r.Context())
	if !ok {
		apierrors.Write(w, http.StatusUnauthorized, "UNAUTHORIZED", "missing claims", httpx.CorrelationID(r.Context()))
		return
	}
	repairID, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		apierrors.Write(w, http.StatusBadRequest, "BAD_REQUEST", "invalid id", httpx.CorrelationID(r.Context()))
		return
	}
	items, err := h.svc.ListNotes(r.Context(), claims.TenantID, repairID)
	if err != nil {
		apierrors.Write(w, http.StatusInternalServerError, "INTERNAL", err.Error(), httpx.CorrelationID(r.Context()))
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"items": items})
}

func (h *Handler) addAttachment(w http.ResponseWriter, r *http.Request) {
	claims, ok := authz.FromContext(r.Context())
	if !ok {
		apierrors.Write(w, http.StatusUnauthorized, "UNAUTHORIZED", "missing claims", httpx.CorrelationID(r.Context()))
		return
	}
	repairID, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		apierrors.Write(w, http.StatusBadRequest, "BAD_REQUEST", "invalid repair id", httpx.CorrelationID(r.Context()))
		return
	}
	var req struct {
		FileName    string `json:"file_name"`
		ContentType string `json:"content_type"`
		DataBase64  string `json:"data_base64"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 8<<20)).Decode(&req); err != nil {
		apierrors.Write(w, http.StatusBadRequest, "BAD_REQUEST", "invalid attachment payload", httpx.CorrelationID(r.Context()))
		return
	}
	if req.FileName == "" || req.ContentType == "" || req.DataBase64 == "" {
		apierrors.Write(w, http.StatusBadRequest, "BAD_REQUEST", "file_name, content_type, and data_base64 required", httpx.CorrelationID(r.Context()))
		return
	}
	if i := strings.Index(req.DataBase64, ","); strings.HasPrefix(req.DataBase64, "data:") && i >= 0 {
		req.DataBase64 = req.DataBase64[i+1:]
	}
	content, err := base64.StdEncoding.DecodeString(req.DataBase64)
	if err != nil || len(content) == 0 || len(content) > 5<<20 {
		apierrors.Write(w, http.StatusBadRequest, "BAD_REQUEST", "attachment must be valid base64 and no larger than 5 MB", httpx.CorrelationID(r.Context()))
		return
	}
	item, err := h.svc.AddAttachment(
		r.Context(), claims.TenantID, repairID, req.FileName, req.ContentType,
		content, claims.UserID, parseCorrID(r), nil,
	)
	if err != nil {
		status := http.StatusInternalServerError
		if strings.Contains(err.Error(), "not found") {
			status = http.StatusNotFound
		}
		apierrors.Write(w, status, "ATTACHMENT_FAILED", err.Error(), httpx.CorrelationID(r.Context()))
		return
	}
	httpx.JSON(w, http.StatusCreated, item)
}

func (h *Handler) initiatePresignedAttachment(w http.ResponseWriter, r *http.Request) {
	claims, ok := authz.FromContext(r.Context())
	if !ok {
		apierrors.Write(w, http.StatusUnauthorized, "UNAUTHORIZED", "missing claims", httpx.CorrelationID(r.Context()))
		return
	}
	repairID, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		apierrors.Write(w, http.StatusBadRequest, "BAD_REQUEST", "invalid repair id", httpx.CorrelationID(r.Context()))
		return
	}
	var req struct {
		FileName    string `json:"file_name"`
		ContentType string `json:"content_type"`
		SizeBytes   int    `json:"size_bytes"`
		SHA256Hex   string `json:"sha256_hex"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&req); err != nil {
		apierrors.Write(w, http.StatusBadRequest, "BAD_REQUEST", "invalid presign payload", httpx.CorrelationID(r.Context()))
		return
	}
	out, err := h.svc.InitiatePresignedAttachment(
		r.Context(), claims.TenantID, repairID,
		req.FileName, req.ContentType, req.SizeBytes, req.SHA256Hex,
		claims.UserID, parseCorrID(r),
	)
	if err != nil {
		status := http.StatusInternalServerError
		switch {
		case strings.Contains(err.Error(), "not found"):
			status = http.StatusNotFound
		case strings.Contains(err.Error(), "not configured"),
			strings.Contains(err.Error(), "required"),
			strings.Contains(err.Error(), "not allowed"),
			strings.Contains(err.Error(), "too long"),
			strings.Contains(err.Error(), "must be"),
			strings.Contains(err.Error(), "larger"):
			status = http.StatusBadRequest
		}
		apierrors.Write(w, status, "ATTACHMENT_FAILED", err.Error(), httpx.CorrelationID(r.Context()))
		return
	}
	httpx.JSON(w, http.StatusCreated, out)
}

func (h *Handler) completePresignedAttachment(w http.ResponseWriter, r *http.Request) {
	claims, ok := authz.FromContext(r.Context())
	if !ok {
		apierrors.Write(w, http.StatusUnauthorized, "UNAUTHORIZED", "missing claims", httpx.CorrelationID(r.Context()))
		return
	}
	repairID, attachmentID, ok := parseAttachmentIDs(w, r)
	if !ok {
		return
	}
	var req struct {
		StorageKey string `json:"storage_key"`
		SHA256Hex  string `json:"sha256_hex"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&req); err != nil {
		apierrors.Write(w, http.StatusBadRequest, "BAD_REQUEST", "invalid complete payload", httpx.CorrelationID(r.Context()))
		return
	}
	item, err := h.svc.CompletePresignedAttachment(
		r.Context(), claims.TenantID, repairID, attachmentID, req.StorageKey, req.SHA256Hex,
	)
	if err != nil {
		status := http.StatusInternalServerError
		switch {
		case strings.Contains(err.Error(), "not found"):
			status = http.StatusNotFound
		case strings.Contains(err.Error(), "not configured"),
			strings.Contains(err.Error(), "required"),
			strings.Contains(err.Error(), "mismatch"),
			strings.Contains(err.Error(), "invalid"),
			strings.Contains(err.Error(), "not pending"),
			strings.Contains(err.Error(), "not a presigned"):
			status = http.StatusBadRequest
		}
		apierrors.Write(w, status, "ATTACHMENT_FAILED", err.Error(), httpx.CorrelationID(r.Context()))
		return
	}
	httpx.JSON(w, http.StatusOK, item)
}

func (h *Handler) listAttachments(w http.ResponseWriter, r *http.Request) {
	claims, _ := authz.FromContext(r.Context())
	repairID, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		apierrors.Write(w, http.StatusBadRequest, "BAD_REQUEST", "invalid repair id", httpx.CorrelationID(r.Context()))
		return
	}
	items, err := h.svc.ListAttachments(r.Context(), claims.TenantID, repairID)
	if err != nil {
		apierrors.Write(w, http.StatusInternalServerError, "INTERNAL", err.Error(), httpx.CorrelationID(r.Context()))
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"items": items})
}

func (h *Handler) getAttachmentContent(w http.ResponseWriter, r *http.Request) {
	claims, _ := authz.FromContext(r.Context())
	repairID, attachmentID, ok := parseAttachmentIDs(w, r)
	if !ok {
		return
	}
	fileName, contentType, content, err := h.svc.GetAttachmentContent(r.Context(), claims.TenantID, repairID, attachmentID)
	if err != nil {
		status := http.StatusInternalServerError
		if strings.Contains(err.Error(), "not found") {
			status = http.StatusNotFound
		}
		apierrors.Write(w, status, "ATTACHMENT_FAILED", err.Error(), httpx.CorrelationID(r.Context()))
		return
	}
	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Content-Disposition", fmt.Sprintf(`inline; filename=%q`, fileName))
	w.Header().Set("Cache-Control", "private, max-age=3600")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(content)
}

func (h *Handler) deleteAttachment(w http.ResponseWriter, r *http.Request) {
	claims, _ := authz.FromContext(r.Context())
	repairID, attachmentID, ok := parseAttachmentIDs(w, r)
	if !ok {
		return
	}
	if err := h.svc.DeleteAttachment(r.Context(), claims.TenantID, repairID, attachmentID); err != nil {
		status := http.StatusInternalServerError
		if strings.Contains(err.Error(), "not found") {
			status = http.StatusNotFound
		}
		apierrors.Write(w, status, "ATTACHMENT_FAILED", err.Error(), httpx.CorrelationID(r.Context()))
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func parseAttachmentIDs(w http.ResponseWriter, r *http.Request) (uuid.UUID, uuid.UUID, bool) {
	repairID, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		apierrors.Write(w, http.StatusBadRequest, "BAD_REQUEST", "invalid repair id", httpx.CorrelationID(r.Context()))
		return uuid.Nil, uuid.Nil, false
	}
	attachmentID, err := uuid.Parse(r.PathValue("attachment_id"))
	if err != nil {
		apierrors.Write(w, http.StatusBadRequest, "BAD_REQUEST", "invalid attachment id", httpx.CorrelationID(r.Context()))
		return uuid.Nil, uuid.Nil, false
	}
	return repairID, attachmentID, true
}

func (h *Handler) assignRepair(w http.ResponseWriter, r *http.Request) {
	claims, ok := authz.FromContext(r.Context())
	if !ok {
		apierrors.Write(w, http.StatusUnauthorized, "UNAUTHORIZED", "missing claims", httpx.CorrelationID(r.Context()))
		return
	}
	repairID, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		apierrors.Write(w, http.StatusBadRequest, "BAD_REQUEST", "invalid id", httpx.CorrelationID(r.Context()))
		return
	}
	var req struct {
		TechnicianID uuid.UUID `json:"technician_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.TechnicianID == uuid.Nil {
		apierrors.Write(w, http.StatusBadRequest, "BAD_REQUEST", "technician_id required", httpx.CorrelationID(r.Context()))
		return
	}
	job, err := h.svc.Assign(r.Context(), claims.TenantID, repairID, req.TechnicianID, claims.UserID, parseCorrID(r))
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			apierrors.Write(w, http.StatusNotFound, "NOT_FOUND", err.Error(), httpx.CorrelationID(r.Context()))
			return
		}
		apierrors.Write(w, http.StatusInternalServerError, "INTERNAL", err.Error(), httpx.CorrelationID(r.Context()))
		return
	}
	httpx.JSON(w, http.StatusOK, job)
}

func (h *Handler) changeStatus(w http.ResponseWriter, r *http.Request) {
	claims, ok := authz.FromContext(r.Context())
	if !ok {
		apierrors.Write(w, http.StatusUnauthorized, "UNAUTHORIZED", "missing claims", httpx.CorrelationID(r.Context()))
		return
	}
	repairID, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		apierrors.Write(w, http.StatusBadRequest, "BAD_REQUEST", "invalid id", httpx.CorrelationID(r.Context()))
		return
	}
	var req struct {
		Status      string   `json:"status"`
		Note        *string  `json:"note"`
		LaborAmount *float64 `json:"labor_amount"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Status == "" {
		apierrors.Write(w, http.StatusBadRequest, "BAD_REQUEST", "status required", httpx.CorrelationID(r.Context()))
		return
	}
	job, err := h.svc.ChangeStatus(r.Context(), claims.TenantID, repairID, req.Status, req.Note, req.LaborAmount, claims.UserID, parseCorrID(r))
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			apierrors.Write(w, http.StatusNotFound, "NOT_FOUND", err.Error(), httpx.CorrelationID(r.Context()))
			return
		}
		if strings.Contains(err.Error(), "transition") {
			apierrors.Write(w, http.StatusConflict, "CONFLICT", err.Error(), httpx.CorrelationID(r.Context()))
			return
		}
		apierrors.Write(w, http.StatusInternalServerError, "INTERNAL", err.Error(), httpx.CorrelationID(r.Context()))
		return
	}
	httpx.JSON(w, http.StatusOK, job)
}

func (h *Handler) deleteRepair(w http.ResponseWriter, r *http.Request) {
	claims, ok := authz.FromContext(r.Context())
	if !ok {
		apierrors.Write(w, http.StatusUnauthorized, "UNAUTHORIZED", "missing claims", httpx.CorrelationID(r.Context()))
		return
	}
	repairID, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		apierrors.Write(w, http.StatusBadRequest, "BAD_REQUEST", "invalid id", httpx.CorrelationID(r.Context()))
		return
	}
	if err := h.svc.DeleteRepair(r.Context(), claims.TenantID, repairID); err != nil {
		if strings.Contains(err.Error(), "cannot delete") {
			apierrors.Write(w, http.StatusConflict, "CONFLICT", err.Error(), httpx.CorrelationID(r.Context()))
			return
		}
		apierrors.Write(w, http.StatusInternalServerError, "INTERNAL", err.Error(), httpx.CorrelationID(r.Context()))
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func parseCorrID(r *http.Request) uuid.UUID {
	if cid := httpx.CorrelationID(r.Context()); cid != "" {
		if id, err := uuid.Parse(cid); err == nil {
			return id
		}
	}
	return uuid.New()
}

func (h *Handler) getSMSSettings(w http.ResponseWriter, r *http.Request) {
	claims, ok := authz.FromContext(r.Context())
	if !ok {
		apierrors.Write(w, http.StatusUnauthorized, "UNAUTHORIZED", "missing claims", httpx.CorrelationID(r.Context()))
		return
	}
	if !canManageSMSSettings(claims) {
		apierrors.Write(w, http.StatusForbidden, "FORBIDDEN", "owner required", httpx.CorrelationID(r.Context()))
		return
	}
	cfg, err := h.svc.GetSMSSettings(r.Context(), claims.TenantID)
	if err != nil {
		apierrors.Write(w, http.StatusInternalServerError, "INTERNAL", err.Error(), httpx.CorrelationID(r.Context()))
		return
	}
	httpx.JSON(w, http.StatusOK, cfg)
}

func (h *Handler) putSMSSettings(w http.ResponseWriter, r *http.Request) {
	claims, ok := authz.FromContext(r.Context())
	if !ok {
		apierrors.Write(w, http.StatusUnauthorized, "UNAUTHORIZED", "missing claims", httpx.CorrelationID(r.Context()))
		return
	}
	if !canManageSMSSettings(claims) {
		apierrors.Write(w, http.StatusForbidden, "FORBIDDEN", "owner required", httpx.CorrelationID(r.Context()))
		return
	}
	var req struct {
		Enabled  *bool   `json:"enabled"`
		Provider *string `json:"provider"`
		APIKey   *string `json:"api_key"`
		SenderID *string `json:"sender_id"`
		BaseURL  *string `json:"base_url"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		apierrors.Write(w, http.StatusBadRequest, "BAD_REQUEST", "invalid json", httpx.CorrelationID(r.Context()))
		return
	}
	cfg, err := h.svc.UpsertSMSSettings(r.Context(), claims.TenantID, UpsertSMSSettingsInput{
		Enabled:  req.Enabled,
		Provider: req.Provider,
		APIKey:   req.APIKey,
		SenderID: req.SenderID,
		BaseURL:  req.BaseURL,
		ActorID:  claims.UserID,
	})
	if err != nil {
		apierrors.Write(w, http.StatusBadRequest, "BAD_REQUEST", err.Error(), httpx.CorrelationID(r.Context()))
		return
	}
	httpx.JSON(w, http.StatusOK, cfg)
}

func canManageSMSSettings(claims *authz.Claims) bool {
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

func (h *Handler) getWarranty(w http.ResponseWriter, r *http.Request) {
	claims, ok := authz.FromContext(r.Context())
	if !ok {
		apierrors.Write(w, http.StatusUnauthorized, "UNAUTHORIZED", "missing claims", httpx.CorrelationID(r.Context()))
		return
	}
	repairID, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		apierrors.Write(w, http.StatusBadRequest, "BAD_REQUEST", "invalid id", httpx.CorrelationID(r.Context()))
		return
	}
	warr, err := h.svc.GetWarranty(r.Context(), claims.TenantID, repairID)
	if err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, ErrWarrantyNotFound) || strings.Contains(err.Error(), "not found") {
			status = http.StatusNotFound
		}
		apierrors.Write(w, status, "WARRANTY_FAILED", err.Error(), httpx.CorrelationID(r.Context()))
		return
	}
	httpx.JSON(w, http.StatusOK, warr)
}

func (h *Handler) createWarranty(w http.ResponseWriter, r *http.Request) {
	claims, ok := authz.FromContext(r.Context())
	if !ok {
		apierrors.Write(w, http.StatusUnauthorized, "UNAUTHORIZED", "missing claims", httpx.CorrelationID(r.Context()))
		return
	}
	repairID, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		apierrors.Write(w, http.StatusBadRequest, "BAD_REQUEST", "invalid id", httpx.CorrelationID(r.Context()))
		return
	}
	var req struct {
		DurationDays *int `json:"duration_days"`
	}
	_ = json.NewDecoder(r.Body).Decode(&req)
	warr, err := h.svc.CreateWarranty(r.Context(), CreateWarrantyInput{
		TenantID: claims.TenantID, RepairJobID: repairID, DurationDays: req.DurationDays,
	})
	if err != nil {
		status := http.StatusBadRequest
		if strings.Contains(err.Error(), "not found") {
			status = http.StatusNotFound
		}
		apierrors.Write(w, status, "WARRANTY_FAILED", err.Error(), httpx.CorrelationID(r.Context()))
		return
	}
	httpx.JSON(w, http.StatusCreated, warr)
}

func (h *Handler) claimWarrantyStaff(w http.ResponseWriter, r *http.Request) {
	claims, ok := authz.FromContext(r.Context())
	if !ok {
		apierrors.Write(w, http.StatusUnauthorized, "UNAUTHORIZED", "missing claims", httpx.CorrelationID(r.Context()))
		return
	}
	repairID, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		apierrors.Write(w, http.StatusBadRequest, "BAD_REQUEST", "invalid id", httpx.CorrelationID(r.Context()))
		return
	}
	var req struct {
		Note string `json:"note"`
	}
	_ = json.NewDecoder(r.Body).Decode(&req)
	warr, err := h.svc.ClaimWarranty(r.Context(), ClaimWarrantyInput{
		TenantID: claims.TenantID, RepairJobID: repairID, Note: strings.TrimSpace(req.Note),
	})
	if err != nil {
		status := http.StatusConflict
		if strings.Contains(err.Error(), "not found") {
			status = http.StatusNotFound
		}
		apierrors.Write(w, status, "WARRANTY_FAILED", err.Error(), httpx.CorrelationID(r.Context()))
		return
	}
	httpx.JSON(w, http.StatusOK, warr)
}

func (h *Handler) customerGetWarranty(w http.ResponseWriter, r *http.Request) {
	tenantID, customer, ok := h.resolveCustomer(w, r)
	if !ok {
		return
	}
	repairID, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		apierrors.Write(w, http.StatusBadRequest, "BAD_REQUEST", "invalid id", httpx.CorrelationID(r.Context()))
		return
	}
	if err := h.svc.AssertCustomerOwnsRepair(r.Context(), tenantID, customer.ID, repairID); err != nil {
		status := http.StatusNotFound
		if !strings.Contains(err.Error(), "not found") {
			status = http.StatusForbidden
		}
		apierrors.Write(w, status, "FORBIDDEN", err.Error(), httpx.CorrelationID(r.Context()))
		return
	}
	warr, err := h.svc.GetWarranty(r.Context(), tenantID, repairID)
	if err != nil {
		status := http.StatusNotFound
		if !errors.Is(err, ErrWarrantyNotFound) {
			status = http.StatusInternalServerError
		}
		apierrors.Write(w, status, "WARRANTY_FAILED", err.Error(), httpx.CorrelationID(r.Context()))
		return
	}
	httpx.JSON(w, http.StatusOK, warr)
}

func (h *Handler) customerClaimWarranty(w http.ResponseWriter, r *http.Request) {
	tenantID, customer, ok := h.resolveCustomer(w, r)
	if !ok {
		return
	}
	repairID, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		apierrors.Write(w, http.StatusBadRequest, "BAD_REQUEST", "invalid id", httpx.CorrelationID(r.Context()))
		return
	}
	var req struct {
		Note string `json:"note"`
	}
	_ = json.NewDecoder(r.Body).Decode(&req)
	warr, err := h.svc.ClaimWarranty(r.Context(), ClaimWarrantyInput{
		TenantID: tenantID, RepairJobID: repairID, CustomerID: &customer.ID,
		Note: strings.TrimSpace(req.Note),
	})
	if err != nil {
		status := http.StatusConflict
		if strings.Contains(err.Error(), "not found") || strings.Contains(err.Error(), "Forbidden") {
			status = http.StatusForbidden
		}
		apierrors.Write(w, status, "WARRANTY_FAILED", err.Error(), httpx.CorrelationID(r.Context()))
		return
	}
	httpx.JSON(w, http.StatusOK, warr)
}

func (h *Handler) staffRepairReceiptPDF(w http.ResponseWriter, r *http.Request) {
	doc, ok := h.loadStaffReceipt(w, r)
	if !ok {
		return
	}
	h.writeReceiptPDF(w, doc, false)
}

func (h *Handler) staffRepairTaxInvoicePDF(w http.ResponseWriter, r *http.Request) {
	doc, ok := h.loadStaffReceipt(w, r)
	if !ok {
		return
	}
	h.writeReceiptPDF(w, doc, true)
}

func (h *Handler) customerRepairReceiptPDF(w http.ResponseWriter, r *http.Request) {
	doc, ok := h.loadCustomerReceipt(w, r)
	if !ok {
		return
	}
	h.writeReceiptPDF(w, doc, false)
}

func (h *Handler) customerRepairTaxInvoicePDF(w http.ResponseWriter, r *http.Request) {
	doc, ok := h.loadCustomerReceipt(w, r)
	if !ok {
		return
	}
	h.writeReceiptPDF(w, doc, true)
}

func (h *Handler) loadStaffReceipt(w http.ResponseWriter, r *http.Request) (*CustomerReceiptDocument, bool) {
	claims, ok := authz.FromContext(r.Context())
	if !ok {
		apierrors.Write(w, http.StatusUnauthorized, "UNAUTHORIZED", "missing claims", httpx.CorrelationID(r.Context()))
		return nil, false
	}
	repairID, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		apierrors.Write(w, http.StatusBadRequest, "BAD_REQUEST", "invalid id", httpx.CorrelationID(r.Context()))
		return nil, false
	}
	doc, err := h.svc.BuildCustomerReceipt(r.Context(), claims.TenantID, repairID)
	if err != nil {
		status := http.StatusInternalServerError
		if strings.Contains(err.Error(), "not found") {
			status = http.StatusNotFound
		}
		apierrors.Write(w, status, "RECEIPT_FAILED", err.Error(), httpx.CorrelationID(r.Context()))
		return nil, false
	}
	return doc, true
}

func (h *Handler) writeReceiptPDF(w http.ResponseWriter, doc *CustomerReceiptDocument, taxInvoice bool) {
	var pdf []byte
	var err error
	if taxInvoice {
		pdf, err = doc.TaxInvoicePDF()
	} else {
		pdf, err = doc.ReceiptPDF()
	}
	if err != nil {
		apierrors.Write(w, http.StatusInternalServerError, "INTERNAL", err.Error(), "")
		return
	}
	filename := doc.JobCode + "-receipt.pdf"
	if taxInvoice {
		filename = doc.JobCode + "-tax-invoice.pdf"
	}
	w.Header().Set("Content-Type", "application/pdf")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`inline; filename=%q`, filename))
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(pdf)
}
