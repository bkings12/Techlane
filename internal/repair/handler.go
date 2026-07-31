package repair

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
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

// SetReceiptRenderer switches receipt printing over to the shared, branded
// renderer. Without it the handler falls back to the plain built-in layout.
func (h *Handler) SetReceiptRenderer(r *receipts.Service) { h.receipts = r }

func (h *Handler) Register(mux *http.ServeMux, auth func(http.Handler) http.Handler) {
	mux.Handle("POST /customers", auth(http.HandlerFunc(h.createCustomer)))
	mux.Handle("GET /customers", auth(http.HandlerFunc(h.listCustomers)))
	mux.Handle("GET /customers/{id}", auth(http.HandlerFunc(h.getCustomer)))
	mux.Handle("PATCH /customers/{id}", auth(http.HandlerFunc(h.updateCustomer)))
	mux.HandleFunc("GET /public/repairs/status", h.publicRepairStatus)

	// Customer phone-OTP (public; Bearer session after verify).
	mux.HandleFunc("POST /customer/auth/otp/request", h.customerOTPRequest)
	mux.HandleFunc("POST /customer/auth/otp/verify", h.customerOTPVerify)
	mux.HandleFunc("POST /customer/auth/logout", h.customerLogout)
	mux.HandleFunc("GET /customer/me", h.customerMe)
	mux.HandleFunc("GET /customer/repairs", h.customerListRepairs)
	mux.HandleFunc("POST /customer/repairs/claim", h.customerClaimRepair)
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
	mux.Handle("POST /repairs/intake", auth(http.HandlerFunc(h.intake)))
	// Top-level path: /repairs/intake-presets/{id} conflicts with /repairs/{id}/schedule in Go ServeMux.
	mux.Handle("GET /intake-presets", auth(http.HandlerFunc(h.listIntakePresets)))
	mux.Handle("POST /intake-presets", auth(httpx.RequirePermission("repairs.presets.write")(http.HandlerFunc(h.createIntakePreset))))
	mux.Handle("PATCH /intake-presets/{id}", auth(httpx.RequirePermission("repairs.presets.write")(http.HandlerFunc(h.updateIntakePreset))))
	mux.Handle("DELETE /intake-presets/{id}", auth(httpx.RequirePermission("repairs.presets.write")(http.HandlerFunc(h.deleteIntakePreset))))
	mux.Handle("POST /repairs", auth(http.HandlerFunc(h.createRepair)))
	mux.Handle("GET /repairs", auth(http.HandlerFunc(h.listRepairs)))
	mux.Handle("GET /repairs/trash", auth(http.HandlerFunc(h.listTrashedRepairs)))
	mux.Handle("GET /repairs/{id}", auth(http.HandlerFunc(h.getRepair)))
	mux.Handle("PATCH /repairs/{id}", auth(httpx.RequirePermission("repairs.edit")(http.HandlerFunc(h.updateRepairDetails))))
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
	mux.Handle("PATCH /repairs/{id}/schedule", auth(http.HandlerFunc(h.updateSchedule)))
	mux.Handle("POST /repairs/{id}/passcode/reveal", auth(http.HandlerFunc(h.revealPasscode)))
	mux.Handle("POST /repairs/{id}/rework", auth(http.HandlerFunc(h.createRework)))
	mux.Handle("POST /repairs/{id}/authorize-work", auth(http.HandlerFunc(h.authorizeWork)))
	mux.Handle("POST /repairs/{id}/accept-paid-charge", auth(http.HandlerFunc(h.acceptPaidCharge)))
	mux.Handle("GET /repairs/{id}/margin", auth(http.HandlerFunc(h.repairMargin)))
	mux.Handle("GET /repairs/{id}/sale-lines", auth(http.HandlerFunc(h.listSaleLines)))
	mux.Handle("POST /repairs/{id}/sale-lines", auth(http.HandlerFunc(h.addSaleLine)))
	mux.Handle("DELETE /repairs/{id}/sale-lines/{line_id}", auth(http.HandlerFunc(h.removeSaleLine)))
	mux.Handle("POST /repairs/{id}/handover/send-code", auth(http.HandlerFunc(h.sendHandoverCode)))
	mux.Handle("POST /repairs/{id}/handover", auth(http.HandlerFunc(h.recordHandover)))
	mux.Handle("POST /repairs/collect", auth(http.HandlerFunc(h.collectByPickupCode)))
	mux.Handle("GET /repairs/by-pickup-code", auth(http.HandlerFunc(h.lookupByPickupCode)))
	mux.Handle("GET /repairs/{id}/intake-slip.html", auth(http.HandlerFunc(h.staffIntakeSlipHTML)))
	mux.Handle("DELETE /repairs/{id}", auth(http.HandlerFunc(h.deleteRepair)))
	mux.Handle("POST /repairs/{id}/restore", auth(http.HandlerFunc(h.restoreRepair)))
	mux.Handle("DELETE /repairs/{id}/purge", auth(http.HandlerFunc(h.purgeRepair)))
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

func (h *Handler) updateCustomer(w http.ResponseWriter, r *http.Request) {
	claims, ok := authz.FromContext(r.Context())
	if !ok {
		apierrors.Write(w, http.StatusUnauthorized, "UNAUTHORIZED", "missing claims", httpx.CorrelationID(r.Context()))
		return
	}
	if !claims.HasPermission("customers.write") {
		apierrors.Write(w, http.StatusForbidden, "FORBIDDEN", "customers.write required", httpx.CorrelationID(r.Context()))
		return
	}
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		apierrors.Write(w, http.StatusBadRequest, "BAD_REQUEST", "invalid id", httpx.CorrelationID(r.Context()))
		return
	}
	var req struct {
		FullName string  `json:"full_name"`
		Phone    *string `json:"phone"`
		Email    *string `json:"email"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || strings.TrimSpace(req.FullName) == "" {
		apierrors.Write(w, http.StatusBadRequest, "BAD_REQUEST", "full_name required", httpx.CorrelationID(r.Context()))
		return
	}
	c, err := h.svc.UpdateCustomer(r.Context(), UpdateCustomerInput{
		CustomerID: id,
		FullName:   req.FullName,
		Phone:      req.Phone,
		Email:      req.Email,
		ActorID:    claims.UserID,
		TenantID:   claims.TenantID,
	})
	if err != nil {
		msg := err.Error()
		switch {
		case strings.Contains(msg, "not found"):
			apierrors.Write(w, http.StatusNotFound, "NOT_FOUND", msg, httpx.CorrelationID(r.Context()))
		case strings.Contains(msg, "already uses"), strings.Contains(msg, "invalid phone"), strings.Contains(msg, "full_name"):
			apierrors.Write(w, http.StatusBadRequest, "BAD_REQUEST", msg, httpx.CorrelationID(r.Context()))
		default:
			apierrors.Write(w, http.StatusInternalServerError, "INTERNAL", msg, httpx.CorrelationID(r.Context()))
		}
		return
	}
	httpx.JSON(w, http.StatusOK, c)
}

func (h *Handler) publicRepairStatus(w http.ResponseWriter, r *http.Request) {
	jobCode := strings.TrimSpace(r.URL.Query().Get("job_code"))
	phone := strings.TrimSpace(r.URL.Query().Get("phone"))
	if jobCode == "" && phone == "" {
		apierrors.Write(w, http.StatusBadRequest, "BAD_REQUEST", "job_code or phone required", httpx.CorrelationID(r.Context()))
		return
	}

	var (
		job      *RepairJob
		tenantID uuid.UUID
		err      error
	)
	switch {
	case jobCode != "" && phone != "":
		job, tenantID, err = h.svc.PublicRepairStatus(r.Context(), jobCode, phone)
	case jobCode != "":
		job, tenantID, err = h.svc.PublicRepairStatusByCode(r.Context(), jobCode)
	default:
		apierrors.Write(w, http.StatusBadRequest, "BAD_REQUEST", "job_code required when looking up a single repair", httpx.CorrelationID(r.Context()))
		return
	}
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
		"id":              job.ID,
		"job_code":        job.JobCode,
		"job_number":      job.JobNumber,
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

func (h *Handler) customerClaimRepair(w http.ResponseWriter, r *http.Request) {
	tenantID, customer, ok := h.resolveCustomer(w, r)
	if !ok {
		return
	}
	var req struct {
		JobCode string `json:"job_code"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || strings.TrimSpace(req.JobCode) == "" {
		apierrors.Write(w, http.StatusBadRequest, "BAD_REQUEST", "job_code required", httpx.CorrelationID(r.Context()))
		return
	}
	job, err := h.svc.ClaimRepairJob(r.Context(), tenantID, customer.ID, req.JobCode)
	if err != nil {
		status := http.StatusInternalServerError
		if strings.Contains(err.Error(), "not found") || strings.Contains(err.Error(), "invalid") {
			status = http.StatusBadRequest
		}
		if strings.Contains(err.Error(), "another customer") {
			status = http.StatusForbidden
		}
		apierrors.Write(w, status, "CLAIM_FAILED", err.Error(), httpx.CorrelationID(r.Context()))
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{
		"job_id":    job.ID,
		"job_code":  job.JobCode,
		"tenant_id": tenantID,
	})
}

func (h *Handler) customerRepairReceipt(w http.ResponseWriter, r *http.Request) {
	doc, _, ok := h.loadCustomerReceipt(w, r)
	if !ok {
		return
	}
	httpx.JSON(w, http.StatusOK, doc)
}

func (h *Handler) customerRepairReceiptHTML(w http.ResponseWriter, r *http.Request) {
	doc, tenantID, ok := h.loadCustomerReceipt(w, r)
	if !ok {
		return
	}
	h.writeReceiptHTML(w, r, tenantID, doc, false)
}

func (h *Handler) loadCustomerReceipt(w http.ResponseWriter, r *http.Request) (*CustomerReceiptDocument, uuid.UUID, bool) {
	tenantID, customer, ok := h.resolveCustomer(w, r)
	if !ok {
		return nil, uuid.Nil, false
	}
	repairID, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		apierrors.Write(w, http.StatusBadRequest, "BAD_REQUEST", "invalid id", httpx.CorrelationID(r.Context()))
		return nil, uuid.Nil, false
	}
	if err := h.svc.AssertCustomerOwnsRepair(r.Context(), tenantID, customer.ID, repairID); err != nil {
		status := http.StatusNotFound
		if !strings.Contains(err.Error(), "not found") {
			status = http.StatusForbidden
		}
		apierrors.Write(w, status, "FORBIDDEN", err.Error(), httpx.CorrelationID(r.Context()))
		return nil, uuid.Nil, false
	}
	doc, err := h.svc.BuildCustomerReceipt(r.Context(), tenantID, repairID)
	if err != nil {
		apierrors.Write(w, http.StatusInternalServerError, "INTERNAL", err.Error(), httpx.CorrelationID(r.Context()))
		return nil, uuid.Nil, false
	}
	return doc, tenantID, true
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
	h.writeReceiptHTML(w, r, claims.TenantID, doc, false)
}

// writeReceiptHTML renders through the branded engine when it is wired up,
// falling back to the built-in layout so receipts never fail to print.
func (h *Handler) writeReceiptHTML(w http.ResponseWriter, r *http.Request, tenantID uuid.UUID, doc *CustomerReceiptDocument, taxInvoice bool) {
	body := ""
	if h.receipts != nil {
		rendered, err := h.receipts.Render(r.Context(), tenantID, doc.ToReceiptDocument(taxInvoice), doc.RepairID, r.URL.Query().Get("paper"))
		if err == nil {
			body = rendered
		}
	}
	if body == "" {
		body = doc.HTML()
	}
	receipts.SetPrintableHTMLHeaders(w)
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(body))
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
		TotalAmount  *float64 `json:"total_amount"`
		LaborAmount  float64  `json:"labor_amount"`
		PartsAmount  float64  `json:"parts_amount"`
		Notes        *string  `json:"notes"`
		ExpiresHours *int     `json:"expires_hours"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		apierrors.Write(w, http.StatusBadRequest, "BAD_REQUEST", "invalid body", httpx.CorrelationID(r.Context()))
		return
	}
	in := CreateEstimateInput{
		TenantID: claims.TenantID, RepairJobID: repairID,
		LaborAmount: req.LaborAmount, PartsAmount: req.PartsAmount,
		Notes: req.Notes, ExpiresHours: req.ExpiresHours, ActorID: claims.UserID,
	}
	if req.TotalAmount != nil {
		in.TotalAmount = *req.TotalAmount
	}
	est, err := h.svc.CreateEstimate(r.Context(), in)
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

func (h *Handler) intake(w http.ResponseWriter, r *http.Request) {
	claims, ok := authz.FromContext(r.Context())
	if !ok {
		apierrors.Write(w, http.StatusUnauthorized, "UNAUTHORIZED", "missing claims", httpx.CorrelationID(r.Context()))
		return
	}
	var req struct {
		BranchID            uuid.UUID  `json:"branch_id"`
		CustomerID          *uuid.UUID `json:"customer_id"`
		Anonymous           bool       `json:"anonymous"`
		CustomerName        *string    `json:"customer_name"`
		CustomerPhone       *string    `json:"customer_phone"`
		DeviceKind          string     `json:"device_kind"`
		Brand               *string    `json:"brand"`
		Model               *string    `json:"model"`
		IMEI                *string    `json:"imei"`
		SerialNumber        *string    `json:"serial_number"`
		ProblemSummary      string     `json:"problem_summary"`
		ConditionTags       []string   `json:"condition_tags"`
		EstimateLaborAmount *float64   `json:"estimate_labor_amount"`
		EstimatePartsAmount *float64   `json:"estimate_parts_amount"`
		TechnicianID        *uuid.UUID `json:"technician_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		apierrors.Write(w, http.StatusBadRequest, "BAD_REQUEST", "invalid body", httpx.CorrelationID(r.Context()))
		return
	}
	if req.BranchID == uuid.Nil || strings.TrimSpace(req.ProblemSummary) == "" {
		apierrors.Write(w, http.StatusBadRequest, "BAD_REQUEST", "branch_id and problem_summary required", httpx.CorrelationID(r.Context()))
		return
	}
	if !claims.InBranch(req.BranchID) {
		apierrors.Write(w, http.StatusForbidden, "FORBIDDEN", "branch access denied", httpx.CorrelationID(r.Context()))
		return
	}
	result, err := h.svc.Intake(r.Context(), IntakeInput{
		TenantID: claims.TenantID, BranchID: req.BranchID, ActorID: claims.UserID, CorrID: parseCorrID(r),
		CustomerID: req.CustomerID, Anonymous: req.Anonymous,
		CustomerName: req.CustomerName, CustomerPhone: req.CustomerPhone,
		DeviceKind: req.DeviceKind, Brand: req.Brand, Model: req.Model,
		IMEI: req.IMEI, SerialNumber: req.SerialNumber,
		ProblemSummary: req.ProblemSummary, ConditionTags: req.ConditionTags,
		EstimateLaborAmount: req.EstimateLaborAmount, EstimatePartsAmount: req.EstimatePartsAmount,
		TechnicianID: req.TechnicianID,
	})
	if err != nil {
		msg := err.Error()
		if strings.Contains(msg, "required") || strings.Contains(msg, "must be") ||
			strings.Contains(msg, "not found") || strings.Contains(msg, "cannot be negative") ||
			strings.Contains(msg, "walk-in") {
			apierrors.Write(w, http.StatusBadRequest, "BAD_REQUEST", msg, httpx.CorrelationID(r.Context()))
			return
		}
		apierrors.Write(w, http.StatusInternalServerError, "INTERNAL", msg, httpx.CorrelationID(r.Context()))
		return
	}
	httpx.JSON(w, http.StatusCreated, result)
}

func (h *Handler) createRepair(w http.ResponseWriter, r *http.Request) {
	claims, ok := authz.FromContext(r.Context())
	if !ok {
		apierrors.Write(w, http.StatusUnauthorized, "UNAUTHORIZED", "missing claims", httpx.CorrelationID(r.Context()))
		return
	}
	var req struct {
		BranchID             uuid.UUID  `json:"branch_id"`
		CustomerID           *uuid.UUID `json:"customer_id"`
		DeviceID             uuid.UUID  `json:"device_id"`
		ProblemSummary       string     `json:"problem_summary"`
		ServiceType          string     `json:"service_type"`
		TechnicianID         *uuid.UUID `json:"technician_id"`
		LaborAmount          *float64   `json:"labor_amount"`
		PromisedBy           *time.Time `json:"promised_by"`
		CustomerWaiting      bool       `json:"customer_waiting"`
		EstimatedWaitMinutes *int       `json:"estimated_wait_minutes"`
		CustomerCredit       bool       `json:"customer_credit"`
		CreditDueDate        *time.Time `json:"credit_due_date"`
		IntakeAccessories    []string   `json:"intake_accessories"`
		IntakeCondition      *string    `json:"intake_condition"`
		DevicePasscode       string     `json:"device_passcode"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.BranchID == uuid.Nil || req.DeviceID == uuid.Nil || req.ProblemSummary == "" {
		apierrors.Write(w, http.StatusBadRequest, "BAD_REQUEST", "branch_id, device_id, problem_summary required", httpx.CorrelationID(r.Context()))
		return
	}
	if !claims.InBranch(req.BranchID) {
		apierrors.Write(w, http.StatusForbidden, "FORBIDDEN", "branch access denied", httpx.CorrelationID(r.Context()))
		return
	}
	var labor float64
	if req.LaborAmount != nil {
		labor = *req.LaborAmount
	}
	job, err := h.svc.CreateRepair(r.Context(), CreateRepairInput{
		BranchID: req.BranchID, CustomerID: req.CustomerID, DeviceID: req.DeviceID,
		ProblemSummary: req.ProblemSummary, ServiceType: req.ServiceType, TechnicianID: req.TechnicianID, LaborAmount: labor,
		PromisedBy:           req.PromisedBy,
		CustomerWaiting:      req.CustomerWaiting,
		EstimatedWaitMinutes: req.EstimatedWaitMinutes,
		CustomerCredit:       req.CustomerCredit,
		CreditDueDate:        req.CreditDueDate,
		IntakeAccessories:    req.IntakeAccessories, IntakeCondition: req.IntakeCondition,
		DevicePasscode: req.DevicePasscode,
		ActorID:        claims.UserID, TenantID: claims.TenantID, CorrID: parseCorrID(r),
	})
	if err != nil {
		msg := err.Error()
		if strings.Contains(msg, "estimated_wait_minutes") || strings.Contains(msg, "credit_due_date") || strings.Contains(msg, "customer is required for credit") || strings.Contains(msg, "service_type") {
			apierrors.Write(w, http.StatusBadRequest, "BAD_REQUEST", msg, httpx.CorrelationID(r.Context()))
			return
		}
		apierrors.Write(w, http.StatusInternalServerError, "INTERNAL", msg, httpx.CorrelationID(r.Context()))
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
	// Best effort: a missing handover record must not hide the job itself.
	handover, _ := h.svc.HandoverFor(r.Context(), claims.TenantID, id)
	httpx.JSON(w, http.StatusOK, map[string]any{
		"id":                     job.ID,
		"job_number":             job.JobNumber,
		"job_code":               job.JobCode,
		"pickup_code":            job.PickupCode,
		"branch_id":              job.BranchID,
		"customer_id":            job.CustomerID,
		"customer":               job.Customer,
		"device_id":              job.DeviceID,
		"device":                 job.Device,
		"technician_id":          job.TechnicianID,
		"status":                 job.Status,
		"problem_summary":        job.ProblemSummary,
		"labor_amount":           job.LaborAmount,
		"created_at":             job.CreatedAt,
		"promised_by":            job.PromisedBy,
		"customer_waiting":       job.CustomerWaiting,
		"estimated_wait_minutes": job.EstimatedWaitMin,
		"intake_accessories":     job.IntakeAccessories,
		"intake_condition":       job.IntakeCondition,
		"has_device_passcode":    job.HasDevicePasscode,
		"parent_job_id":          job.ParentJobID,
		"parent_job_code":        job.ParentJobCode,
		"rework_reason":          job.ReworkReason,
		"closure_reason":         job.ClosureReason,
		"closed_at":              job.ClosedAt,
		"version":                job.Version,
		"authorization":          job.Authorization,
		"handover":               handover,
		"timeline":               job.Timeline,
		"next_statuses":          NextStatuses(job.Status),
		"closure_reasons": map[string][]string{
			StatusCancelled:    ClosureReasonsFor(StatusCancelled),
			StatusUnrepairable: ClosureReasonsFor(StatusUnrepairable),
		},
	})
}

func (h *Handler) updateRepairDetails(w http.ResponseWriter, r *http.Request) {
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
		ExpectedVersion int        `json:"expected_version"`
		ProblemSummary  *string    `json:"problem_summary"`
		DeviceKind      *string    `json:"device_kind"`
		DeviceBrand     *string    `json:"device_brand"`
		DeviceModel     *string    `json:"device_model"`
		DeviceIMEI      *string    `json:"device_imei"`
		DeviceSerial    *string    `json:"device_serial"`
		CustomerID      *uuid.UUID `json:"customer_id"`
		Anonymous       *bool      `json:"anonymous"`
		Reason          string     `json:"reason"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		apierrors.Write(w, http.StatusBadRequest, "BAD_REQUEST", "invalid body", httpx.CorrelationID(r.Context()))
		return
	}
	if strings.TrimSpace(req.Reason) == "" {
		apierrors.Write(w, http.StatusBadRequest, "BAD_REQUEST", "reason is required", httpx.CorrelationID(r.Context()))
		return
	}
	job, err := h.svc.UpdateRepairDetails(r.Context(), UpdateRepairDetailsInput{
		TenantID:        claims.TenantID,
		RepairID:       repairID,
		ExpectedVersion: req.ExpectedVersion,
		ProblemSummary:  req.ProblemSummary,
		DeviceKind:      req.DeviceKind,
		DeviceBrand:     req.DeviceBrand,
		DeviceModel:     req.DeviceModel,
		DeviceIMEI:      req.DeviceIMEI,
		DeviceSerial:    req.DeviceSerial,
		CustomerID:      req.CustomerID,
		Anonymous:       req.Anonymous,
		Reason:          req.Reason,
		ActorID:         claims.UserID,
		CorrID:          parseCorrID(r),
	})
	if err != nil {
		switch {
		case errors.Is(err, ErrVersionConflict):
			apierrors.Write(w, http.StatusConflict, "CONFLICT", err.Error(), httpx.CorrelationID(r.Context()))
		case errors.Is(err, ErrEditNotAllowed):
			apierrors.Write(w, http.StatusConflict, "CONFLICT", err.Error(), httpx.CorrelationID(r.Context()))
		case errors.Is(err, ErrNoDetailChanges):
			apierrors.Write(w, http.StatusBadRequest, "BAD_REQUEST", err.Error(), httpx.CorrelationID(r.Context()))
		case strings.Contains(err.Error(), "not found"):
			apierrors.Write(w, http.StatusNotFound, "NOT_FOUND", err.Error(), httpx.CorrelationID(r.Context()))
		case strings.Contains(err.Error(), "required") ||
			strings.Contains(err.Error(), "cannot be empty") ||
			strings.Contains(err.Error(), "invalid"):
			apierrors.Write(w, http.StatusBadRequest, "BAD_REQUEST", err.Error(), httpx.CorrelationID(r.Context()))
		default:
			apierrors.Write(w, http.StatusInternalServerError, "INTERNAL", err.Error(), httpx.CorrelationID(r.Context()))
		}
		return
	}
	httpx.JSON(w, http.StatusOK, job)
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

func (h *Handler) updateSchedule(w http.ResponseWriter, r *http.Request) {
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
		PromisedBy *time.Time `json:"promised_by"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		apierrors.Write(w, http.StatusBadRequest, "BAD_REQUEST", "invalid promised_by", httpx.CorrelationID(r.Context()))
		return
	}
	job, err := h.svc.UpdatePromisedBy(r.Context(), claims.TenantID, repairID, req.PromisedBy, claims.UserID, parseCorrID(r))
	if err != nil {
		status := http.StatusInternalServerError
		if strings.Contains(err.Error(), "not found") {
			status = http.StatusNotFound
		}
		apierrors.Write(w, status, "SCHEDULE_FAILED", err.Error(), httpx.CorrelationID(r.Context()))
		return
	}
	httpx.JSON(w, http.StatusOK, job)
}

func (h *Handler) revealPasscode(w http.ResponseWriter, r *http.Request) {
	claims, ok := authz.FromContext(r.Context())
	if !ok {
		apierrors.Write(w, http.StatusUnauthorized, "UNAUTHORIZED", "missing claims", httpx.CorrelationID(r.Context()))
		return
	}
	if !claims.HasPermission("repairs.passcode.read") {
		apierrors.Write(w, http.StatusForbidden, "FORBIDDEN", "you cannot reveal device passcodes", httpx.CorrelationID(r.Context()))
		return
	}
	repairID, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		apierrors.Write(w, http.StatusBadRequest, "BAD_REQUEST", "invalid id", httpx.CorrelationID(r.Context()))
		return
	}
	passcode, err := h.svc.RevealDevicePasscode(r.Context(), claims.TenantID, repairID, claims.UserID, parseCorrID(r))
	if err != nil {
		apierrors.Write(w, http.StatusBadRequest, "PASSCODE_FAILED", err.Error(), httpx.CorrelationID(r.Context()))
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]string{"passcode": passcode})
}

func (h *Handler) createRework(w http.ResponseWriter, r *http.Request) {
	claims, ok := authz.FromContext(r.Context())
	if !ok {
		apierrors.Write(w, http.StatusUnauthorized, "UNAUTHORIZED", "missing claims", httpx.CorrelationID(r.Context()))
		return
	}
	var req struct {
		Reason string `json:"reason"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		apierrors.Write(w, http.StatusBadRequest, "BAD_REQUEST", "invalid body", httpx.CorrelationID(r.Context()))
		return
	}
	repairID, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		apierrors.Write(w, http.StatusBadRequest, "BAD_REQUEST", "invalid id", httpx.CorrelationID(r.Context()))
		return
	}
	job, err := h.svc.CreateRework(r.Context(), claims.TenantID, repairID, req.Reason, claims.UserID, parseCorrID(r))
	if err != nil {
		apierrors.Write(w, http.StatusConflict, "REWORK_FAILED", err.Error(), httpx.CorrelationID(r.Context()))
		return
	}
	httpx.JSON(w, http.StatusCreated, job)
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
		Status         string   `json:"status"`
		Note           *string  `json:"note"`
		LaborAmount    *float64 `json:"labor_amount"`
		ClosureReason  string   `json:"closure_reason"`
		VarianceReason string   `json:"variance_reason"`
		Force          bool     `json:"force"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Status == "" {
		apierrors.Write(w, http.StatusBadRequest, "BAD_REQUEST", "status required", httpx.CorrelationID(r.Context()))
		return
	}
	if req.Status == StatusCollected && !claims.HasPermission("repairs.collect") {
		apierrors.Write(w, http.StatusForbidden, "FORBIDDEN", "you don't have permission to mark repairs collected", httpx.CorrelationID(r.Context()))
		return
	}
	// Closing a job writes off pipeline value, so it needs the same authority as
	// approving a price — a technician cannot quietly make a job disappear.
	if IsClosure(req.Status) && !claims.HasPermission("repairs.close") {
		apierrors.Write(w, http.StatusForbidden, "FORBIDDEN", "you don't have permission to cancel or write off a repair", httpx.CorrelationID(r.Context()))
		return
	}
	job, err := h.svc.ChangeStatus(r.Context(), ChangeStatusInput{
		TenantID:       claims.TenantID,
		RepairID:       repairID,
		NewStatus:      req.Status,
		Note:           req.Note,
		LaborAmount:    req.LaborAmount,
		ClosureReason:  req.ClosureReason,
		VarianceReason: req.VarianceReason,
		ActorID:        claims.UserID,
		CorrelationID:  parseCorrID(r),
		Force:          req.Force && claims.HasPermission("repairs.collect"),
	})
	if err != nil {
		if errors.Is(err, ErrBalanceDue) {
			apierrors.Write(w, http.StatusConflict, "BALANCE_DUE", err.Error(), httpx.CorrelationID(r.Context()))
			return
		}
		if errors.Is(err, ErrHandoverRequired) {
			apierrors.Write(w, http.StatusConflict, "HANDOVER_REQUIRED", err.Error(), httpx.CorrelationID(r.Context()))
			return
		}
		if errors.Is(err, ErrInvalidClosure) {
			apierrors.Write(w, http.StatusBadRequest, "INVALID_CLOSURE", err.Error(), httpx.CorrelationID(r.Context()))
			return
		}
		if errors.Is(err, ErrWorkNotAuthorized) {
			apierrors.Write(w, http.StatusConflict, "WORK_NOT_AUTHORIZED", err.Error(), httpx.CorrelationID(r.Context()))
			return
		}
		if errors.Is(err, ErrPartsOutstanding) {
			apierrors.Write(w, http.StatusConflict, "PARTS_OUTSTANDING", err.Error(), httpx.CorrelationID(r.Context()))
			return
		}
		if errors.Is(err, ErrEstimatePending) {
			apierrors.Write(w, http.StatusConflict, "ESTIMATE_PENDING", err.Error(), httpx.CorrelationID(r.Context()))
			return
		}
		if errors.Is(err, ErrVarianceReasonRequired) {
			apierrors.Write(w, http.StatusConflict, "VARIANCE_REASON_REQUIRED", err.Error(), httpx.CorrelationID(r.Context()))
			return
		}
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

func (h *Handler) authorizeWork(w http.ResponseWriter, r *http.Request) {
	claims, ok := authz.FromContext(r.Context())
	if !ok {
		apierrors.Write(w, http.StatusUnauthorized, "UNAUTHORIZED", "missing claims", httpx.CorrelationID(r.Context()))
		return
	}
	if !claims.HasPermission("repairs.authorize_work") {
		apierrors.Write(w, http.StatusForbidden, "FORBIDDEN", "only a manager or owner can authorize work without a customer-approved estimate", httpx.CorrelationID(r.Context()))
		return
	}
	repairID, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		apierrors.Write(w, http.StatusBadRequest, "BAD_REQUEST", "invalid id", httpx.CorrelationID(r.Context()))
		return
	}
	var req struct {
		Amount *float64 `json:"amount"`
		Note   string   `json:"note"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		apierrors.Write(w, http.StatusBadRequest, "BAD_REQUEST", "invalid body", httpx.CorrelationID(r.Context()))
		return
	}
	job, err := h.svc.AuthorizeWork(r.Context(), claims.TenantID, repairID, req.Amount, req.Note, claims.UserID, parseCorrID(r))
	if err != nil {
		status := http.StatusBadRequest
		if strings.Contains(err.Error(), "not found") {
			status = http.StatusNotFound
		}
		apierrors.Write(w, status, "AUTHORIZE_FAILED", err.Error(), httpx.CorrelationID(r.Context()))
		return
	}
	httpx.JSON(w, http.StatusOK, job)
}

func (h *Handler) acceptPaidCharge(w http.ResponseWriter, r *http.Request) {
	claims, ok := authz.FromContext(r.Context())
	if !ok {
		apierrors.Write(w, http.StatusUnauthorized, "UNAUTHORIZED", "missing claims", httpx.CorrelationID(r.Context()))
		return
	}
	// Cashiers who take payment need this more than managers — they collected what
	// the customer paid and need the card to match without another STK.
	if !claims.HasPermission("repairs.authorize_work") &&
		!claims.HasPermission("payments.initiate") &&
		!claims.HasPermission("*") {
		apierrors.Write(w, http.StatusForbidden, "FORBIDDEN", "not allowed to adjust the job charge", httpx.CorrelationID(r.Context()))
		return
	}
	repairID, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		apierrors.Write(w, http.StatusBadRequest, "BAD_REQUEST", "invalid id", httpx.CorrelationID(r.Context()))
		return
	}
	job, err := h.svc.AcceptPaidAsCharge(r.Context(), claims.TenantID, repairID, claims.UserID, parseCorrID(r))
	if err != nil {
		status := http.StatusBadRequest
		if strings.Contains(err.Error(), "not found") {
			status = http.StatusNotFound
		}
		apierrors.Write(w, status, "ACCEPT_PAID_FAILED", err.Error(), httpx.CorrelationID(r.Context()))
		return
	}
	httpx.JSON(w, http.StatusOK, job)
}

func (h *Handler) sendHandoverCode(w http.ResponseWriter, r *http.Request) {
	claims, ok := authz.FromContext(r.Context())
	if !ok {
		apierrors.Write(w, http.StatusUnauthorized, "UNAUTHORIZED", "missing claims", httpx.CorrelationID(r.Context()))
		return
	}
	if !claims.HasPermission("repairs.collect") {
		apierrors.Write(w, http.StatusForbidden, "FORBIDDEN", "you don't have permission to release devices", httpx.CorrelationID(r.Context()))
		return
	}
	repairID, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		apierrors.Write(w, http.StatusBadRequest, "BAD_REQUEST", "invalid id", httpx.CorrelationID(r.Context()))
		return
	}
	if err := h.svc.RequestHandoverOTP(r.Context(), claims.TenantID, repairID); err != nil {
		status := http.StatusBadRequest
		code := "HANDOVER_CODE_FAILED"
		switch {
		case errors.Is(err, ErrNoCustomerPhone):
			code = "NO_CUSTOMER_PHONE"
		case errors.Is(err, ErrHandoverNotReady):
			status, code = http.StatusConflict, "NOT_READY"
		case strings.Contains(err.Error(), "not found"):
			status, code = http.StatusNotFound, "NOT_FOUND"
		}
		apierrors.Write(w, status, code, err.Error(), httpx.CorrelationID(r.Context()))
		return
	}
	httpx.JSON(w, http.StatusAccepted, map[string]any{"status": "sent"})
}

func (h *Handler) recordHandover(w http.ResponseWriter, r *http.Request) {
	claims, ok := authz.FromContext(r.Context())
	if !ok {
		apierrors.Write(w, http.StatusUnauthorized, "UNAUTHORIZED", "missing claims", httpx.CorrelationID(r.Context()))
		return
	}
	if !claims.HasPermission("repairs.collect") {
		apierrors.Write(w, http.StatusForbidden, "FORBIDDEN", "you don't have permission to release devices", httpx.CorrelationID(r.Context()))
		return
	}
	repairID, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		apierrors.Write(w, http.StatusBadRequest, "BAD_REQUEST", "invalid id", httpx.CorrelationID(r.Context()))
		return
	}
	var req struct {
		CollectedByName string `json:"collected_by_name"`
		Relationship    string `json:"relationship"`
		IDNumber        string `json:"id_number"`
		Note            string `json:"note"`
		OTPCode         string `json:"otp_code"`
		PickupCode      string `json:"pickup_code"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		apierrors.Write(w, http.StatusBadRequest, "BAD_REQUEST", "invalid body", httpx.CorrelationID(r.Context()))
		return
	}
	handover, err := h.svc.RecordHandover(r.Context(), claims.TenantID, repairID, HandoverInput{
		CollectedByName: req.CollectedByName,
		Relationship:    req.Relationship,
		IDNumber:        req.IDNumber,
		Note:            req.Note,
		OTPCode:         req.OTPCode,
		PickupCode:      req.PickupCode,
		// Releasing without the owner confirming a code is a judgement call
		// somebody has to own, so it takes the same authority as writing off a job.
		CanVouch: claims.HasPermission("repairs.release_unverified"),
		ActorID:  claims.UserID,
		CorrID:   parseCorrID(r),
	})
	if err != nil {
		status := http.StatusBadRequest
		code := "HANDOVER_FAILED"
		switch {
		case errors.Is(err, ErrBalanceDue):
			status, code = http.StatusConflict, "BALANCE_DUE"
		case errors.Is(err, ErrHandoverExists):
			status, code = http.StatusConflict, "ALREADY_HANDED_OVER"
		case errors.Is(err, ErrHandoverNotReady):
			status, code = http.StatusConflict, "NOT_READY"
		case errors.Is(err, ErrHandoverVouchNotAllowed):
			status, code = http.StatusForbidden, "CODE_REQUIRED"
		case strings.Contains(err.Error(), "not found"):
			status, code = http.StatusNotFound, "NOT_FOUND"
		}
		apierrors.Write(w, status, code, err.Error(), httpx.CorrelationID(r.Context()))
		return
	}
	httpx.JSON(w, http.StatusCreated, handover)
}

func (h *Handler) lookupByPickupCode(w http.ResponseWriter, r *http.Request) {
	claims, ok := authz.FromContext(r.Context())
	if !ok {
		apierrors.Write(w, http.StatusUnauthorized, "UNAUTHORIZED", "missing claims", httpx.CorrelationID(r.Context()))
		return
	}
	code := strings.TrimSpace(r.URL.Query().Get("code"))
	if code == "" {
		apierrors.Write(w, http.StatusBadRequest, "BAD_REQUEST", "code required", httpx.CorrelationID(r.Context()))
		return
	}
	repairID, err := h.svc.FindRepairByPickupCode(r.Context(), claims.TenantID, code)
	if err != nil {
		status := http.StatusInternalServerError
		if strings.Contains(err.Error(), "no repair matches") || strings.Contains(err.Error(), "required") {
			status = http.StatusNotFound
		}
		apierrors.Write(w, status, "NOT_FOUND", err.Error(), httpx.CorrelationID(r.Context()))
		return
	}
	job, err := h.svc.GetRepair(r.Context(), claims.TenantID, repairID)
	if err != nil {
		apierrors.Write(w, http.StatusInternalServerError, "INTERNAL", err.Error(), httpx.CorrelationID(r.Context()))
		return
	}
	_, balance, _, _, _ := h.svc.RepairPaymentContext(r.Context(), claims.TenantID, repairID)
	canRelease := isCollectable(job.Status) && balance <= 0.01
	httpx.JSON(w, http.StatusOK, map[string]any{
		"id":              job.ID,
		"job_code":        job.JobCode,
		"pickup_code":     job.PickupCode,
		"status":          job.Status,
		"problem_summary": job.ProblemSummary,
		"customer":        job.Customer,
		"device":          job.Device,
		"balance_due":     balance,
		"can_release":     canRelease,
		"release_blocked": map[string]any{
			"not_ready":   !isCollectable(job.Status),
			"balance_due": balance > 0.01,
		},
	})
}

func (h *Handler) collectByPickupCode(w http.ResponseWriter, r *http.Request) {
	claims, ok := authz.FromContext(r.Context())
	if !ok {
		apierrors.Write(w, http.StatusUnauthorized, "UNAUTHORIZED", "missing claims", httpx.CorrelationID(r.Context()))
		return
	}
	if !claims.HasPermission("repairs.collect") {
		apierrors.Write(w, http.StatusForbidden, "FORBIDDEN", "you don't have permission to release devices", httpx.CorrelationID(r.Context()))
		return
	}
	var req struct {
		PickupCode      string `json:"pickup_code"`
		CollectedByName string `json:"collected_by_name"`
		Relationship    string `json:"relationship"`
		IDNumber        string `json:"id_number"`
		Note            string `json:"note"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || strings.TrimSpace(req.PickupCode) == "" {
		apierrors.Write(w, http.StatusBadRequest, "BAD_REQUEST", "pickup_code required", httpx.CorrelationID(r.Context()))
		return
	}
	handover, err := h.svc.CollectByPickupCode(r.Context(), claims.TenantID, req.PickupCode, HandoverInput{
		CollectedByName: req.CollectedByName,
		Relationship:    req.Relationship,
		IDNumber:        req.IDNumber,
		Note:            req.Note,
		ActorID:         claims.UserID,
		CorrID:          parseCorrID(r),
	})
	if err != nil {
		status := http.StatusBadRequest
		code := "COLLECT_FAILED"
		switch {
		case errors.Is(err, ErrBalanceDue):
			status, code = http.StatusConflict, "BALANCE_DUE"
		case errors.Is(err, ErrHandoverExists):
			status, code = http.StatusConflict, "ALREADY_HANDED_OVER"
		case errors.Is(err, ErrHandoverNotReady):
			status, code = http.StatusConflict, "NOT_READY"
		case strings.Contains(err.Error(), "no repair matches"):
			status, code = http.StatusNotFound, "NOT_FOUND"
		}
		apierrors.Write(w, status, code, err.Error(), httpx.CorrelationID(r.Context()))
		return
	}
	httpx.JSON(w, http.StatusCreated, handover)
}

func (h *Handler) staffIntakeSlipHTML(w http.ResponseWriter, r *http.Request) {
	doc, tenantID, ok := h.loadStaffReceipt(w, r)
	if !ok {
		return
	}
	h.applyReceiptLetterhead(r.Context(), tenantID, doc)
	receipts.SetPrintableHTMLHeaders(w)
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(doc.IntakeSlipHTML()))
}

// applyReceiptLetterhead copies slogan / phone / email from receipt settings so
// intake slips match the branded header used on repair receipts.
func (h *Handler) applyReceiptLetterhead(ctx context.Context, tenantID uuid.UUID, doc *CustomerReceiptDocument) {
	if h.receipts == nil || doc == nil {
		return
	}
	set, err := h.receipts.GetSettings(ctx, tenantID)
	if err != nil {
		return
	}
	shop := h.receipts.LoadShop(ctx, tenantID, set)
	if strings.TrimSpace(doc.ShopName) == "" && shop.Name != "" {
		doc.ShopName = shop.Name
	}
	doc.ShopSlogan = strings.TrimSpace(set.HeaderNote)
	if phone := strings.TrimSpace(shop.Phone); phone != "" {
		doc.ShopPhone = phone
	} else {
		doc.ShopPhone = strings.TrimSpace(set.Phone)
	}
	if email := strings.TrimSpace(shop.Email); email != "" {
		doc.ShopEmail = email
	} else {
		doc.ShopEmail = strings.TrimSpace(set.Email)
	}
	if site := strings.TrimSpace(shop.Website); site != "" {
		doc.ShopWebsite = site
	} else {
		doc.ShopWebsite = strings.TrimSpace(set.Website)
	}
}

func (h *Handler) listSaleLines(w http.ResponseWriter, r *http.Request) {
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
	items, err := h.svc.ListJobSaleLines(r.Context(), claims.TenantID, repairID)
	if err != nil {
		apierrors.Write(w, http.StatusInternalServerError, "INTERNAL", err.Error(), httpx.CorrelationID(r.Context()))
		return
	}
	if items == nil {
		items = []JobSaleLine{}
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"items": items})
}

func (h *Handler) addSaleLine(w http.ResponseWriter, r *http.Request) {
	claims, ok := authz.FromContext(r.Context())
	if !ok {
		apierrors.Write(w, http.StatusUnauthorized, "UNAUTHORIZED", "missing claims", httpx.CorrelationID(r.Context()))
		return
	}
	if !claims.HasPermission("sales.create") && !claims.HasPermission("payments.initiate") && !claims.HasPermission("*") {
		apierrors.Write(w, http.StatusForbidden, "FORBIDDEN", "sales.create or payments.initiate required", httpx.CorrelationID(r.Context()))
		return
	}
	repairID, err := uuid.Parse(r.PathValue("id"))
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
	line, err := h.svc.AddJobSaleLine(r.Context(), claims.TenantID, repairID, req.VariantID, req.LocationID, req.Quantity, claims.UserID, parseCorrID(r))
	if err != nil {
		status := http.StatusBadRequest
		if strings.Contains(err.Error(), "not found") {
			status = http.StatusNotFound
		}
		apierrors.Write(w, status, "SALE_LINE_FAILED", err.Error(), httpx.CorrelationID(r.Context()))
		return
	}
	httpx.JSON(w, http.StatusCreated, line)
}

func (h *Handler) removeSaleLine(w http.ResponseWriter, r *http.Request) {
	claims, ok := authz.FromContext(r.Context())
	if !ok {
		apierrors.Write(w, http.StatusUnauthorized, "UNAUTHORIZED", "missing claims", httpx.CorrelationID(r.Context()))
		return
	}
	if !claims.HasPermission("sales.create") && !claims.HasPermission("payments.initiate") && !claims.HasPermission("*") {
		apierrors.Write(w, http.StatusForbidden, "FORBIDDEN", "sales.create or payments.initiate required", httpx.CorrelationID(r.Context()))
		return
	}
	repairID, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		apierrors.Write(w, http.StatusBadRequest, "BAD_REQUEST", "invalid id", httpx.CorrelationID(r.Context()))
		return
	}
	lineID, err := uuid.Parse(r.PathValue("line_id"))
	if err != nil {
		apierrors.Write(w, http.StatusBadRequest, "BAD_REQUEST", "invalid line_id", httpx.CorrelationID(r.Context()))
		return
	}
	if err := h.svc.RemoveJobSaleLine(r.Context(), claims.TenantID, repairID, lineID, claims.UserID, parseCorrID(r)); err != nil {
		status := http.StatusBadRequest
		if strings.Contains(err.Error(), "not found") {
			status = http.StatusNotFound
		}
		apierrors.Write(w, status, "SALE_LINE_FAILED", err.Error(), httpx.CorrelationID(r.Context()))
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) repairMargin(w http.ResponseWriter, r *http.Request) {
	claims, ok := authz.FromContext(r.Context())
	if !ok {
		apierrors.Write(w, http.StatusUnauthorized, "UNAUTHORIZED", "missing claims", httpx.CorrelationID(r.Context()))
		return
	}
	// Cost and margin are commercial figures, not bench information.
	if !claims.HasPermission("reports.read") {
		apierrors.Write(w, http.StatusForbidden, "FORBIDDEN", "you don't have permission to view job costs", httpx.CorrelationID(r.Context()))
		return
	}
	repairID, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		apierrors.Write(w, http.StatusBadRequest, "BAD_REQUEST", "invalid id", httpx.CorrelationID(r.Context()))
		return
	}
	m, err := h.svc.JobMarginFor(r.Context(), claims.TenantID, repairID)
	if err != nil {
		status := http.StatusInternalServerError
		if strings.Contains(err.Error(), "no rows") {
			status = http.StatusNotFound
		}
		apierrors.Write(w, status, "MARGIN_FAILED", err.Error(), httpx.CorrelationID(r.Context()))
		return
	}
	httpx.JSON(w, http.StatusOK, m)
}

func (h *Handler) deleteRepair(w http.ResponseWriter, r *http.Request) {
	claims, ok := authz.FromContext(r.Context())
	if !ok {
		apierrors.Write(w, http.StatusUnauthorized, "UNAUTHORIZED", "missing claims", httpx.CorrelationID(r.Context()))
		return
	}
	if !claims.HasPermission("repairs.close") {
		apierrors.Write(w, http.StatusForbidden, "FORBIDDEN", "repair deletion requires manager access", httpx.CorrelationID(r.Context()))
		return
	}
	repairID, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		apierrors.Write(w, http.StatusBadRequest, "BAD_REQUEST", "invalid id", httpx.CorrelationID(r.Context()))
		return
	}
	if err := h.svc.DeleteRepair(r.Context(), claims.TenantID, repairID, claims.UserID); err != nil {
		if strings.Contains(err.Error(), "cannot delete") {
			apierrors.Write(w, http.StatusConflict, "CONFLICT", err.Error(), httpx.CorrelationID(r.Context()))
			return
		}
		apierrors.Write(w, http.StatusInternalServerError, "INTERNAL", err.Error(), httpx.CorrelationID(r.Context()))
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) listTrashedRepairs(w http.ResponseWriter, r *http.Request) {
	claims, ok := authz.FromContext(r.Context())
	if !ok {
		apierrors.Write(w, http.StatusUnauthorized, "UNAUTHORIZED", "missing claims", httpx.CorrelationID(r.Context()))
		return
	}
	if !claims.HasPermission("repairs.close") {
		apierrors.Write(w, http.StatusForbidden, "FORBIDDEN", "trash access requires manager access", httpx.CorrelationID(r.Context()))
		return
	}
	items, err := h.svc.ListTrashedRepairs(r.Context(), claims.TenantID)
	if err != nil {
		apierrors.Write(w, http.StatusInternalServerError, "INTERNAL", err.Error(), httpx.CorrelationID(r.Context()))
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"items": items})
}

func (h *Handler) restoreRepair(w http.ResponseWriter, r *http.Request) {
	claims, ok := authz.FromContext(r.Context())
	if !ok {
		apierrors.Write(w, http.StatusUnauthorized, "UNAUTHORIZED", "missing claims", httpx.CorrelationID(r.Context()))
		return
	}
	if !claims.HasPermission("repairs.close") {
		apierrors.Write(w, http.StatusForbidden, "FORBIDDEN", "restore requires manager access", httpx.CorrelationID(r.Context()))
		return
	}
	repairID, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		apierrors.Write(w, http.StatusBadRequest, "BAD_REQUEST", "invalid id", httpx.CorrelationID(r.Context()))
		return
	}
	if err := h.svc.RestoreRepair(r.Context(), claims.TenantID, repairID, claims.UserID); err != nil {
		apierrors.Write(w, http.StatusNotFound, "NOT_FOUND", err.Error(), httpx.CorrelationID(r.Context()))
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) purgeRepair(w http.ResponseWriter, r *http.Request) {
	claims, ok := authz.FromContext(r.Context())
	if !ok {
		apierrors.Write(w, http.StatusUnauthorized, "UNAUTHORIZED", "missing claims", httpx.CorrelationID(r.Context()))
		return
	}
	if !claims.HasPermission("*") {
		apierrors.Write(w, http.StatusForbidden, "FORBIDDEN", "only an owner can permanently delete records", httpx.CorrelationID(r.Context()))
		return
	}
	repairID, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		apierrors.Write(w, http.StatusBadRequest, "BAD_REQUEST", "invalid id", httpx.CorrelationID(r.Context()))
		return
	}
	if err := h.svc.PurgeRepair(r.Context(), claims.TenantID, repairID); err != nil {
		apierrors.Write(w, http.StatusConflict, "PURGE_FAILED", err.Error(), httpx.CorrelationID(r.Context()))
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
		if errors.Is(err, ErrWarrantyNotFound) {
			// No warranty yet is normal for open jobs — return empty success so
			// the repair detail page does not log a console 404 on every load.
			w.WriteHeader(http.StatusNoContent)
			return
		}
		apierrors.Write(w, http.StatusInternalServerError, "WARRANTY_FAILED", err.Error(), httpx.CorrelationID(r.Context()))
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
		if errors.Is(err, ErrWarrantyNotFound) {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		apierrors.Write(w, http.StatusInternalServerError, "WARRANTY_FAILED", err.Error(), httpx.CorrelationID(r.Context()))
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
	doc, tenantID, ok := h.loadStaffReceipt(w, r)
	if !ok {
		return
	}
	h.writeReceiptPDF(w, r, tenantID, doc, false)
}

func (h *Handler) staffRepairTaxInvoicePDF(w http.ResponseWriter, r *http.Request) {
	doc, tenantID, ok := h.loadStaffReceipt(w, r)
	if !ok {
		return
	}
	h.writeReceiptPDF(w, r, tenantID, doc, true)
}

func (h *Handler) customerRepairReceiptPDF(w http.ResponseWriter, r *http.Request) {
	doc, tenantID, ok := h.loadCustomerReceipt(w, r)
	if !ok {
		return
	}
	h.writeReceiptPDF(w, r, tenantID, doc, false)
}

func (h *Handler) customerRepairTaxInvoicePDF(w http.ResponseWriter, r *http.Request) {
	doc, tenantID, ok := h.loadCustomerReceipt(w, r)
	if !ok {
		return
	}
	h.writeReceiptPDF(w, r, tenantID, doc, true)
}

func (h *Handler) loadStaffReceipt(w http.ResponseWriter, r *http.Request) (*CustomerReceiptDocument, uuid.UUID, bool) {
	claims, ok := authz.FromContext(r.Context())
	if !ok {
		apierrors.Write(w, http.StatusUnauthorized, "UNAUTHORIZED", "missing claims", httpx.CorrelationID(r.Context()))
		return nil, uuid.Nil, false
	}
	repairID, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		apierrors.Write(w, http.StatusBadRequest, "BAD_REQUEST", "invalid id", httpx.CorrelationID(r.Context()))
		return nil, uuid.Nil, false
	}
	doc, err := h.svc.BuildCustomerReceipt(r.Context(), claims.TenantID, repairID)
	if err != nil {
		status := http.StatusInternalServerError
		if strings.Contains(err.Error(), "not found") {
			status = http.StatusNotFound
		}
		apierrors.Write(w, status, "RECEIPT_FAILED", err.Error(), httpx.CorrelationID(r.Context()))
		return nil, uuid.Nil, false
	}
	return doc, claims.TenantID, true
}

func (h *Handler) writeReceiptPDF(w http.ResponseWriter, r *http.Request, tenantID uuid.UUID, doc *CustomerReceiptDocument, taxInvoice bool) {
	var pdf []byte
	var err error
	if h.receipts != nil {
		pdf, err = h.receipts.RenderPDF(r.Context(), tenantID, doc.ToReceiptDocument(taxInvoice), doc.RepairID)
	}
	if len(pdf) == 0 {
		if taxInvoice {
			pdf, err = doc.TaxInvoicePDF()
		} else {
			pdf, err = doc.ReceiptPDF()
		}
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

func (h *Handler) listIntakePresets(w http.ResponseWriter, r *http.Request) {
	claims, ok := authz.FromContext(r.Context())
	if !ok {
		apierrors.Write(w, http.StatusUnauthorized, "UNAUTHORIZED", "missing claims", httpx.CorrelationID(r.Context()))
		return
	}
	kind := strings.TrimSpace(r.URL.Query().Get("kind"))
	includeInactive := r.URL.Query().Get("include_inactive") == "1" ||
		strings.EqualFold(r.URL.Query().Get("include_inactive"), "true")
	items, err := h.svc.ListIntakePresets(r.Context(), claims.TenantID, kind, includeInactive)
	if err != nil {
		if errors.Is(err, ErrPresetInvalid) {
			apierrors.Write(w, http.StatusBadRequest, "BAD_REQUEST", err.Error(), httpx.CorrelationID(r.Context()))
			return
		}
		apierrors.Write(w, http.StatusInternalServerError, "INTERNAL", err.Error(), httpx.CorrelationID(r.Context()))
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"items": items})
}

func (h *Handler) createIntakePreset(w http.ResponseWriter, r *http.Request) {
	claims, ok := authz.FromContext(r.Context())
	if !ok {
		apierrors.Write(w, http.StatusUnauthorized, "UNAUTHORIZED", "missing claims", httpx.CorrelationID(r.Context()))
		return
	}
	var req struct {
		Kind  string `json:"kind"`
		Label string `json:"label"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		apierrors.Write(w, http.StatusBadRequest, "BAD_REQUEST", "invalid body", httpx.CorrelationID(r.Context()))
		return
	}
	item, err := h.svc.CreateIntakePreset(r.Context(), claims.TenantID, req.Kind, req.Label)
	if err != nil {
		writePresetErr(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusCreated, item)
}

func (h *Handler) updateIntakePreset(w http.ResponseWriter, r *http.Request) {
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
	var req struct {
		Label    *string `json:"label"`
		IsActive *bool   `json:"is_active"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		apierrors.Write(w, http.StatusBadRequest, "BAD_REQUEST", "invalid body", httpx.CorrelationID(r.Context()))
		return
	}
	item, err := h.svc.UpdateIntakePreset(r.Context(), claims.TenantID, id, req.Label, req.IsActive)
	if err != nil {
		writePresetErr(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, item)
}

func (h *Handler) deleteIntakePreset(w http.ResponseWriter, r *http.Request) {
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
	if err := h.svc.DeleteIntakePreset(r.Context(), claims.TenantID, id); err != nil {
		writePresetErr(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func writePresetErr(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, ErrPresetNotFound):
		apierrors.Write(w, http.StatusNotFound, "NOT_FOUND", err.Error(), httpx.CorrelationID(r.Context()))
	case errors.Is(err, ErrPresetConflict):
		apierrors.Write(w, http.StatusConflict, "CONFLICT", err.Error(), httpx.CorrelationID(r.Context()))
	case errors.Is(err, ErrPresetInvalid):
		apierrors.Write(w, http.StatusBadRequest, "BAD_REQUEST", err.Error(), httpx.CorrelationID(r.Context()))
	default:
		apierrors.Write(w, http.StatusInternalServerError, "INTERNAL", err.Error(), httpx.CorrelationID(r.Context()))
	}
}
