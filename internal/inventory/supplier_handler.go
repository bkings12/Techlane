package inventory

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/google/uuid"
	"github.com/techlane/techlane/packages/pkg/apierrors"
	"github.com/techlane/techlane/packages/pkg/authz"
	"github.com/techlane/techlane/packages/pkg/httpx"
)

func (h *Handler) createSupplier(w http.ResponseWriter, r *http.Request) {
	claims, _ := authz.FromContext(r.Context())
	var req struct {
		Name  string  `json:"name"`
		Phone *string `json:"phone"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || strings.TrimSpace(req.Name) == "" {
		apierrors.Write(w, http.StatusBadRequest, "BAD_REQUEST", "name required", httpx.CorrelationID(r.Context()))
		return
	}
	sp, err := h.svc.CreateSupplier(r.Context(), claims.TenantID, req.Name, req.Phone)
	if err != nil {
		apierrors.Write(w, http.StatusBadRequest, "BAD_REQUEST", err.Error(), httpx.CorrelationID(r.Context()))
		return
	}
	httpx.JSON(w, http.StatusCreated, sp)
}

func (h *Handler) inviteSupplierContact(w http.ResponseWriter, r *http.Request) {
	claims, _ := authz.FromContext(r.Context())
	supplierID, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		apierrors.Write(w, http.StatusBadRequest, "BAD_REQUEST", "invalid id", httpx.CorrelationID(r.Context()))
		return
	}
	var req struct {
		Email       string  `json:"email"`
		DisplayName string  `json:"display_name"`
		Phone       *string `json:"phone"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		apierrors.Write(w, http.StatusBadRequest, "BAD_REQUEST", "invalid body", httpx.CorrelationID(r.Context()))
		return
	}
	result, err := h.svc.InviteSupplierContact(r.Context(), claims.TenantID, supplierID, req.Email, req.DisplayName, req.Phone)
	if err != nil {
		status := http.StatusBadRequest
		if strings.Contains(err.Error(), "not found") {
			status = http.StatusNotFound
		}
		apierrors.Write(w, status, "INVITE_FAILED", err.Error(), httpx.CorrelationID(r.Context()))
		return
	}
	httpx.JSON(w, http.StatusCreated, result)
}

func (h *Handler) assignPartRequest(w http.ResponseWriter, r *http.Request) {
	claims, _ := authz.FromContext(r.Context())
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		apierrors.Write(w, http.StatusBadRequest, "BAD_REQUEST", "invalid id", httpx.CorrelationID(r.Context()))
		return
	}
	var req struct {
		SupplierID uuid.UUID `json:"supplier_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.SupplierID == uuid.Nil {
		apierrors.Write(w, http.StatusBadRequest, "BAD_REQUEST", "supplier_id required", httpx.CorrelationID(r.Context()))
		return
	}
	pr, err := h.svc.AssignPartRequest(r.Context(), claims.TenantID, id, req.SupplierID)
	if err != nil {
		status := http.StatusConflict
		if strings.Contains(err.Error(), "not found") {
			status = http.StatusNotFound
		}
		apierrors.Write(w, status, "ASSIGN_FAILED", err.Error(), httpx.CorrelationID(r.Context()))
		return
	}
	httpx.JSON(w, http.StatusOK, pr)
}

func (h *Handler) acceptQuote(w http.ResponseWriter, r *http.Request) {
	claims, _ := authz.FromContext(r.Context())
	requestID, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		apierrors.Write(w, http.StatusBadRequest, "BAD_REQUEST", "invalid id", httpx.CorrelationID(r.Context()))
		return
	}
	quoteID, err := uuid.Parse(r.PathValue("quote_id"))
	if err != nil {
		apierrors.Write(w, http.StatusBadRequest, "BAD_REQUEST", "invalid quote_id", httpx.CorrelationID(r.Context()))
		return
	}
	result, err := h.svc.AcceptPartRequestQuote(r.Context(), claims.TenantID, requestID, quoteID, claims.UserID, corrID(r))
	if err != nil {
		status := http.StatusConflict
		if strings.Contains(err.Error(), "not found") {
			status = http.StatusNotFound
		}
		apierrors.Write(w, status, "ACCEPT_FAILED", err.Error(), httpx.CorrelationID(r.Context()))
		return
	}
	httpx.JSON(w, http.StatusOK, result)
}

func (h *Handler) supplierAcceptInvite(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Token    string `json:"token"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Token == "" || req.Password == "" {
		apierrors.Write(w, http.StatusBadRequest, "BAD_REQUEST", "token and password required", httpx.CorrelationID(r.Context()))
		return
	}
	session, err := h.svc.AcceptSupplierInvite(r.Context(), req.Token, req.Password)
	if err != nil {
		apierrors.Write(w, http.StatusBadRequest, "INVITE_FAILED", err.Error(), httpx.CorrelationID(r.Context()))
		return
	}
	httpx.JSON(w, http.StatusOK, session)
}

func (h *Handler) supplierLogin(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		apierrors.Write(w, http.StatusBadRequest, "BAD_REQUEST", "invalid body", httpx.CorrelationID(r.Context()))
		return
	}
	session, err := h.svc.LoginSupplier(r.Context(), req.Email, req.Password)
	if err != nil {
		apierrors.Write(w, http.StatusUnauthorized, "UNAUTHORIZED", err.Error(), httpx.CorrelationID(r.Context()))
		return
	}
	httpx.JSON(w, http.StatusOK, session)
}

func (h *Handler) supplierLogout(w http.ResponseWriter, r *http.Request) {
	_ = h.svc.LogoutSupplier(r.Context(), supplierBearer(r))
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) supplierMe(w http.ResponseWriter, r *http.Request) {
	_, contact, ok := h.authenticatedSupplier(w, r)
	if !ok {
		return
	}
	httpx.JSON(w, http.StatusOK, contact)
}

func (h *Handler) supplierListRequests(w http.ResponseWriter, r *http.Request) {
	tenantID, contact, ok := h.authenticatedSupplier(w, r)
	if !ok {
		return
	}
	items, err := h.svc.ListSupplierPortalRequests(r.Context(), tenantID, contact.SupplierID, r.URL.Query().Get("status"))
	if err != nil {
		apierrors.Write(w, http.StatusInternalServerError, "INTERNAL", err.Error(), httpx.CorrelationID(r.Context()))
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"items": items})
}

func (h *Handler) supplierGetRequest(w http.ResponseWriter, r *http.Request) {
	tenantID, contact, ok := h.authenticatedSupplier(w, r)
	if !ok {
		return
	}
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		apierrors.Write(w, http.StatusBadRequest, "BAD_REQUEST", "invalid id", httpx.CorrelationID(r.Context()))
		return
	}
	pr, err := h.svc.GetSupplierPortalRequest(r.Context(), tenantID, contact.SupplierID, id)
	if err != nil {
		apierrors.Write(w, http.StatusNotFound, "NOT_FOUND", err.Error(), httpx.CorrelationID(r.Context()))
		return
	}
	httpx.JSON(w, http.StatusOK, pr)
}

func (h *Handler) supplierQuote(w http.ResponseWriter, r *http.Request) {
	tenantID, contact, ok := h.authenticatedSupplier(w, r)
	if !ok {
		return
	}
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		apierrors.Write(w, http.StatusBadRequest, "BAD_REQUEST", "invalid id", httpx.CorrelationID(r.Context()))
		return
	}
	var req struct {
		UnitCost float64 `json:"unit_cost"`
		Notes    *string `json:"notes"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		apierrors.Write(w, http.StatusBadRequest, "BAD_REQUEST", "invalid body", httpx.CorrelationID(r.Context()))
		return
	}
	quote, err := h.svc.SubmitSupplierQuote(r.Context(), tenantID, contact.SupplierID, id, req.UnitCost, req.Notes, contact.ID)
	if err != nil {
		status := http.StatusConflict
		if strings.Contains(err.Error(), "not found") {
			status = http.StatusNotFound
		}
		apierrors.Write(w, status, "QUOTE_FAILED", err.Error(), httpx.CorrelationID(r.Context()))
		return
	}
	httpx.JSON(w, http.StatusCreated, quote)
}

func (h *Handler) supplierDecline(w http.ResponseWriter, r *http.Request) {
	tenantID, contact, ok := h.authenticatedSupplier(w, r)
	if !ok {
		return
	}
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		apierrors.Write(w, http.StatusBadRequest, "BAD_REQUEST", "invalid id", httpx.CorrelationID(r.Context()))
		return
	}
	var req struct {
		Notes *string `json:"notes"`
	}
	_ = json.NewDecoder(r.Body).Decode(&req)
	quote, err := h.svc.DeclineSupplierRequest(r.Context(), tenantID, contact.SupplierID, id, req.Notes)
	if err != nil {
		status := http.StatusConflict
		if strings.Contains(err.Error(), "not found") {
			status = http.StatusNotFound
		}
		apierrors.Write(w, status, "DECLINE_FAILED", err.Error(), httpx.CorrelationID(r.Context()))
		return
	}
	httpx.JSON(w, http.StatusOK, quote)
}

func (h *Handler) supplierMarkReady(w http.ResponseWriter, r *http.Request) {
	tenantID, contact, ok := h.authenticatedSupplier(w, r)
	if !ok {
		return
	}
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		apierrors.Write(w, http.StatusBadRequest, "BAD_REQUEST", "invalid id", httpx.CorrelationID(r.Context()))
		return
	}
	pr, err := h.svc.MarkSupplierRequestReady(r.Context(), tenantID, contact.SupplierID, id)
	if err != nil {
		status := http.StatusConflict
		if strings.Contains(err.Error(), "not found") {
			status = http.StatusNotFound
		}
		apierrors.Write(w, status, "READY_FAILED", err.Error(), httpx.CorrelationID(r.Context()))
		return
	}
	httpx.JSON(w, http.StatusOK, pr)
}

func (h *Handler) supplierIssue(w http.ResponseWriter, r *http.Request) {
	tenantID, contact, ok := h.authenticatedSupplier(w, r)
	if !ok {
		return
	}
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		apierrors.Write(w, http.StatusBadRequest, "BAD_REQUEST", "invalid id", httpx.CorrelationID(r.Context()))
		return
	}
	result, err := h.svc.IssueAsSupplier(r.Context(), tenantID, contact.SupplierID, id, contact.ID)
	if err != nil {
		status := http.StatusConflict
		if strings.Contains(err.Error(), "not found") {
			status = http.StatusNotFound
		}
		apierrors.Write(w, status, "ISSUE_FAILED", err.Error(), httpx.CorrelationID(r.Context()))
		return
	}
	httpx.JSON(w, http.StatusCreated, result)
}

func (h *Handler) supplierCollectIssue(w http.ResponseWriter, r *http.Request) {
	tenantID, contact, ok := h.authenticatedSupplier(w, r)
	if !ok {
		return
	}
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		apierrors.Write(w, http.StatusBadRequest, "BAD_REQUEST", "invalid id", httpx.CorrelationID(r.Context()))
		return
	}
	var req struct {
		AuthCode string `json:"auth_code"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || strings.TrimSpace(req.AuthCode) == "" {
		apierrors.Write(w, http.StatusBadRequest, "BAD_REQUEST", "auth_code required", httpx.CorrelationID(r.Context()))
		return
	}
	si, err := h.svc.CollectIssueAsSupplier(r.Context(), tenantID, contact.SupplierID, id, strings.TrimSpace(req.AuthCode), contact.ID)
	if err != nil {
		status := http.StatusConflict
		if strings.Contains(err.Error(), "not found") {
			status = http.StatusNotFound
		}
		apierrors.Write(w, status, "COLLECT_FAILED", err.Error(), httpx.CorrelationID(r.Context()))
		return
	}
	httpx.JSON(w, http.StatusOK, si)
}

func (h *Handler) supplierListIssues(w http.ResponseWriter, r *http.Request) {
	tenantID, contact, ok := h.authenticatedSupplier(w, r)
	if !ok {
		return
	}
	items, err := h.svc.ListSupplierPortalIssues(r.Context(), tenantID, contact.SupplierID)
	if err != nil {
		apierrors.Write(w, http.StatusInternalServerError, "INTERNAL", err.Error(), httpx.CorrelationID(r.Context()))
		return
	}
	out := make([]map[string]any, 0, len(items))
	for _, si := range items {
		out = append(out, map[string]any{
			"id": si.ID, "part_request_id": si.PartRequestID, "repair_job_id": si.RepairJobID,
			"job_code": si.JobCode, "supplier_id": si.SupplierID, "auth_code": si.AuthCode,
			"status": si.Status, "unit_cost": si.UnitCost, "collected_at": si.CollectedAt,
			"reconciliation_status": si.ReconciliationStatus,
			"qr_payload": QRPayloadForIssue(si.ID, si.AuthCode),
		})
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"items": out})
}

func (h *Handler) supplierIssueVoucher(w http.ResponseWriter, r *http.Request) {
	v, _, ok := h.loadSupplierVoucher(w, r)
	if !ok {
		return
	}
	httpx.JSON(w, http.StatusOK, v)
}

func (h *Handler) supplierIssueVoucherHTML(w http.ResponseWriter, r *http.Request) {
	v, tenantID, ok := h.loadSupplierVoucher(w, r)
	if !ok {
		return
	}
	body := ""
	if h.receipts != nil {
		if rendered, err := h.receipts.Render(r.Context(), tenantID, v.ToReceiptDocument(), v.IssueID, r.URL.Query().Get("paper")); err == nil {
			body = rendered
		}
	}
	if body == "" {
		body = v.HTML()
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(body))
}

func (h *Handler) supplierIssueVoucherPDF(w http.ResponseWriter, r *http.Request) {
	v, tenantID, ok := h.loadSupplierVoucher(w, r)
	if !ok {
		return
	}
	var pdf []byte
	var err error
	if h.receipts != nil {
		pdf, err = h.receipts.RenderPDF(r.Context(), tenantID, v.ToReceiptDocument(), v.IssueID)
	}
	if len(pdf) == 0 {
		pdf, err = v.VoucherPDF()
	}
	if err != nil {
		apierrors.Write(w, http.StatusInternalServerError, "INTERNAL", err.Error(), httpx.CorrelationID(r.Context()))
		return
	}
	w.Header().Set("Content-Type", "application/pdf")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`inline; filename=%q`, v.AuthCode+"-voucher.pdf"))
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(pdf)
}

func (h *Handler) loadSupplierVoucher(w http.ResponseWriter, r *http.Request) (*SupplierCreditVoucher, uuid.UUID, bool) {
	tenantID, contact, ok := h.authenticatedSupplier(w, r)
	if !ok {
		return nil, uuid.Nil, false
	}
	issueID, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		apierrors.Write(w, http.StatusBadRequest, "BAD_REQUEST", "invalid id", httpx.CorrelationID(r.Context()))
		return nil, uuid.Nil, false
	}
	v, err := h.svc.BuildSupplierCreditVoucher(r.Context(), tenantID, contact.SupplierID, issueID)
	if err != nil {
		status := http.StatusInternalServerError
		if strings.Contains(err.Error(), "not found") {
			status = http.StatusNotFound
		}
		apierrors.Write(w, status, "VOUCHER_FAILED", err.Error(), httpx.CorrelationID(r.Context()))
		return nil, uuid.Nil, false
	}
	return v, tenantID, true
}

func (h *Handler) supplierCredit(w http.ResponseWriter, r *http.Request) {
	tenantID, contact, ok := h.authenticatedSupplier(w, r)
	if !ok {
		return
	}
	summary, err := h.svc.GetSupplierPortalCredit(r.Context(), tenantID, contact.SupplierID)
	if err != nil {
		apierrors.Write(w, http.StatusInternalServerError, "INTERNAL", err.Error(), httpx.CorrelationID(r.Context()))
		return
	}
	httpx.JSON(w, http.StatusOK, summary)
}

func (h *Handler) authenticatedSupplier(w http.ResponseWriter, r *http.Request) (uuid.UUID, *SupplierContact, bool) {
	tenantID, contact, err := h.svc.AuthenticateSupplier(r.Context(), supplierBearer(r))
	if err != nil {
		apierrors.Write(w, http.StatusUnauthorized, "UNAUTHORIZED", err.Error(), httpx.CorrelationID(r.Context()))
		return uuid.Nil, nil, false
	}
	return tenantID, contact, true
}

func supplierBearer(r *http.Request) string {
	return authz.BearerToken(r.Header.Get("Authorization"))
}
