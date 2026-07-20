package payments

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"

	"github.com/google/uuid"
	"github.com/techlane/techlane/packages/pkg/apierrors"
	"github.com/techlane/techlane/packages/pkg/authz"
	"github.com/techlane/techlane/packages/pkg/httpx"
)

type Handler struct {
	svc            *Service
	customerRepair CustomerRepairGateway
}

func NewHandler(svc *Service) *Handler {
	return &Handler{svc: svc}
}

func (h *Handler) Register(mux *http.ServeMux, auth func(http.Handler) http.Handler) {
	mux.Handle("GET /payments/settings", auth(http.HandlerFunc(h.getSettings)))
	mux.Handle("PUT /payments/settings", auth(http.HandlerFunc(h.putSettings)))
	mux.Handle("POST /payments", auth(http.HandlerFunc(h.createPayment)))
	mux.Handle("GET /payments", auth(http.HandlerFunc(h.listPayments)))
	mux.Handle("GET /payments/{id}", auth(http.HandlerFunc(h.getPayment)))
	mux.Handle("POST /payments/{id}/mpesa/confirm", auth(http.HandlerFunc(h.confirmMpesa)))
	mux.Handle("POST /payments/{id}/mpesa/reconcile", auth(http.HandlerFunc(h.reconcileMpesa)))
	mux.Handle("POST /payments/{id}/bank/confirm", auth(http.HandlerFunc(h.confirmBank)))
	mux.Handle("GET /store-credits/{customer_id}", auth(http.HandlerFunc(h.getStoreCredit)))
	mux.Handle("POST /store-credits/{customer_id}/issue", auth(http.HandlerFunc(h.issueStoreCredit)))
	mux.Handle("GET /cash/pending-total", auth(http.HandlerFunc(h.pendingCashTotal)))
	mux.Handle("GET /cash/handovers", auth(http.HandlerFunc(h.listHandovers)))
	mux.Handle("POST /cash/handovers", auth(http.HandlerFunc(h.requestHandover)))
	mux.Handle("POST /cash/handovers/{id}/confirm", auth(http.HandlerFunc(h.confirmHandover)))
	mux.Handle("POST /refunds", auth(http.HandlerFunc(h.createRefund)))
	mux.Handle("GET /refunds", auth(http.HandlerFunc(h.listRefunds)))
	mux.Handle("POST /refunds/{id}/approve", auth(http.HandlerFunc(h.approveRefund)))
	mux.HandleFunc("POST /webhooks/mpesa/stk", h.mpesaSTKWebhook)
	mux.HandleFunc("POST /webhooks/mpesa/c2b", h.mpesaC2BWebhook)
	mux.HandleFunc("POST /webhooks/mpesa/c2b/validation", h.mpesaC2BValidation)
	mux.HandleFunc("POST /webhooks/mpesa/c2b/confirmation", h.mpesaC2BWebhook)
	mux.Handle("GET /payments/c2b", auth(http.HandlerFunc(h.listC2B)))
	mux.Handle("POST /payments/c2b/{id}/match", auth(http.HandlerFunc(h.matchC2B)))
	mux.HandleFunc("POST /customer/repairs/{id}/pay", h.customerRepairPay)
	mux.HandleFunc("GET /customer/repairs/{id}/payments/{payment_id}", h.customerRepairPaymentStatus)
}

func (h *Handler) getSettings(w http.ResponseWriter, r *http.Request) {
	claims, _ := authz.FromContext(r.Context())
	cfg, err := h.svc.GetProviderSettings(r.Context(), claims.TenantID)
	if err != nil {
		apierrors.Write(w, http.StatusInternalServerError, "INTERNAL", err.Error(), httpx.CorrelationID(r.Context()))
		return
	}
	httpx.JSON(w, http.StatusOK, cfg)
}

func (h *Handler) putSettings(w http.ResponseWriter, r *http.Request) {
	claims, ok := authz.FromContext(r.Context())
	if !ok {
		apierrors.Write(w, http.StatusUnauthorized, "UNAUTHORIZED", "missing claims", httpx.CorrelationID(r.Context()))
		return
	}
	if !canManagePaymentSettings(claims) {
		apierrors.Write(w, http.StatusForbidden, "FORBIDDEN", "owner or manager required", httpx.CorrelationID(r.Context()))
		return
	}
	var req struct {
		Environment         string  `json:"environment"`
		MpesaEnabled        *bool   `json:"mpesa_enabled"`
		MpesaShortcode      *string `json:"mpesa_shortcode"`
		MpesaConsumerKey    *string `json:"mpesa_consumer_key"`
		MpesaConsumerSecret *string `json:"mpesa_consumer_secret"`
		MpesaPasskey        *string `json:"mpesa_passkey"`
		MpesaCallbackURL    *string `json:"mpesa_callback_url"`
		BankEnabled         *bool   `json:"bank_enabled"`
		BankPaybill         *string `json:"bank_paybill"`
		BankAccount         *string `json:"bank_account"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		apierrors.Write(w, http.StatusBadRequest, "BAD_REQUEST", "invalid json", httpx.CorrelationID(r.Context()))
		return
	}
	cfg, err := h.svc.UpsertProviderSettings(r.Context(), claims.TenantID, UpsertProviderSettingsInput{
		Environment:         req.Environment,
		MpesaEnabled:        req.MpesaEnabled,
		MpesaShortcode:      req.MpesaShortcode,
		MpesaConsumerKey:    req.MpesaConsumerKey,
		MpesaConsumerSecret: req.MpesaConsumerSecret,
		MpesaPasskey:        req.MpesaPasskey,
		MpesaCallbackURL:    req.MpesaCallbackURL,
		BankEnabled:         req.BankEnabled,
		BankPaybill:         req.BankPaybill,
		BankAccount:         req.BankAccount,
		ActorID:             claims.UserID,
	})
	if err != nil {
		apierrors.Write(w, http.StatusBadRequest, "BAD_REQUEST", err.Error(), httpx.CorrelationID(r.Context()))
		return
	}
	httpx.JSON(w, http.StatusOK, cfg)
}

func canManagePaymentSettings(claims *authz.Claims) bool {
	if claims == nil {
		return false
	}
	if claims.HasPermission("*") {
		return true
	}
	for _, r := range claims.Roles {
		if r == "owner" || r == "manager" {
			return true
		}
	}
	return false
}

func (h *Handler) createPayment(w http.ResponseWriter, r *http.Request) {
	claims, _ := authz.FromContext(r.Context())
	var req struct {
		Method      string     `json:"method"`
		Amount      float64    `json:"amount"`
		Currency    string     `json:"currency"`
		PayableType string     `json:"payable_type"`
		PayableID   uuid.UUID  `json:"payable_id"`
		BranchID    *uuid.UUID `json:"branch_id"`
		Phone       string     `json:"phone"`
		AccountRef  string     `json:"account_reference"`
		CustomerID  *uuid.UUID `json:"customer_id"`
		Tenders     []TenderLine `json:"tenders"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || ((req.Method == "" && len(req.Tenders) == 0) || req.PayableType == "" || req.PayableID == uuid.Nil) {
		apierrors.Write(w, http.StatusBadRequest, "BAD_REQUEST", "method or tenders, payable_type, payable_id required", httpx.CorrelationID(r.Context()))
		return
	}
	if len(req.Tenders) > 0 {
		items, err := h.svc.CreateSplitPayment(r.Context(), SplitPaymentInput{
			TenantID: claims.TenantID, BranchID: req.BranchID, Currency: req.Currency,
			PayableType: req.PayableType, PayableID: req.PayableID, CustomerID: req.CustomerID,
			Tenders: req.Tenders, Phone: req.Phone, AccountRef: req.AccountRef,
			ActorID: claims.UserID, CorrID: corrID(r),
		})
		if err != nil {
			apierrors.Write(w, http.StatusBadRequest, "BAD_REQUEST", err.Error(), httpx.CorrelationID(r.Context()))
			return
		}
		httpx.JSON(w, http.StatusCreated, map[string]any{"items": items})
		return
	}
	p, err := h.svc.CreatePayment(r.Context(), CreatePaymentInput{
		TenantID: claims.TenantID, BranchID: req.BranchID, Method: req.Method, Amount: req.Amount,
		Currency: req.Currency, PayableType: req.PayableType, PayableID: req.PayableID,
		Phone: req.Phone, AccountRef: req.AccountRef, CustomerID: req.CustomerID,
		ActorID: claims.UserID, CorrID: corrID(r),
	})
	if err != nil {
		apierrors.Write(w, http.StatusBadRequest, "BAD_REQUEST", err.Error(), httpx.CorrelationID(r.Context()))
		return
	}
	httpx.JSON(w, http.StatusCreated, p)
}

func (h *Handler) listPayments(w http.ResponseWriter, r *http.Request) {
	claims, _ := authz.FromContext(r.Context())
	payableType := r.URL.Query().Get("payable_type")
	payableIDStr := r.URL.Query().Get("payable_id")
	if payableType != "" && payableIDStr != "" {
		payableID, err := uuid.Parse(payableIDStr)
		if err != nil {
			apierrors.Write(w, http.StatusBadRequest, "BAD_REQUEST", "invalid payable_id", httpx.CorrelationID(r.Context()))
			return
		}
		items, err := h.svc.ListPaymentsForPayable(r.Context(), claims.TenantID, payableType, payableID)
		if err != nil {
			apierrors.Write(w, http.StatusInternalServerError, "INTERNAL", err.Error(), httpx.CorrelationID(r.Context()))
			return
		}
		if items == nil {
			items = []Payment{}
		}
		httpx.JSON(w, http.StatusOK, map[string]any{"items": items})
		return
	}
	items, err := h.svc.ListPayments(r.Context(), claims.TenantID, 50)
	if err != nil {
		apierrors.Write(w, http.StatusInternalServerError, "INTERNAL", err.Error(), httpx.CorrelationID(r.Context()))
		return
	}
	if items == nil {
		items = []Payment{}
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"items": items})
}

func (h *Handler) getPayment(w http.ResponseWriter, r *http.Request) {
	claims, _ := authz.FromContext(r.Context())
	paymentID, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		apierrors.Write(w, http.StatusBadRequest, "BAD_REQUEST", "invalid id", httpx.CorrelationID(r.Context()))
		return
	}
	p, err := h.svc.GetPayment(r.Context(), claims.TenantID, paymentID)
	if err != nil {
		apierrors.Write(w, http.StatusNotFound, "NOT_FOUND", err.Error(), httpx.CorrelationID(r.Context()))
		return
	}
	httpx.JSON(w, http.StatusOK, p)
}

func (h *Handler) confirmMpesa(w http.ResponseWriter, r *http.Request) {
	// Legacy path: typed provider_ref is no longer accepted as proof of payment.
	h.reconcileMpesa(w, r)
}

func (h *Handler) reconcileMpesa(w http.ResponseWriter, r *http.Request) {
	claims, _ := authz.FromContext(r.Context())
	if !canManualConfirm(claims) {
		apierrors.Write(w, http.StatusForbidden, "FORBIDDEN", "payments.manual_confirm or owner/manager required", httpx.CorrelationID(r.Context()))
		return
	}
	paymentID, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		apierrors.Write(w, http.StatusBadRequest, "BAD_REQUEST", "invalid id", httpx.CorrelationID(r.Context()))
		return
	}
	// Ignore any client-supplied provider_ref body — STK Query is source of truth.
	_, _ = io.Copy(io.Discard, io.LimitReader(r.Body, 1<<20))
	p, err := h.svc.ReconcileSTKPayment(r.Context(), claims.TenantID, paymentID)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			apierrors.Write(w, http.StatusNotFound, "NOT_FOUND", err.Error(), httpx.CorrelationID(r.Context()))
			return
		}
		apierrors.Write(w, http.StatusBadRequest, "BAD_REQUEST", err.Error(), httpx.CorrelationID(r.Context()))
		return
	}
	httpx.JSON(w, http.StatusOK, p)
}

func (h *Handler) confirmBank(w http.ResponseWriter, r *http.Request) {
	claims, _ := authz.FromContext(r.Context())
	if !canManualConfirm(claims) {
		apierrors.Write(w, http.StatusForbidden, "FORBIDDEN", "payments.manual_confirm or owner/manager required", httpx.CorrelationID(r.Context()))
		return
	}
	paymentID, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		apierrors.Write(w, http.StatusBadRequest, "BAD_REQUEST", "invalid id", httpx.CorrelationID(r.Context()))
		return
	}
	var req struct {
		ProviderRef string `json:"provider_ref"`
	}
	_ = json.NewDecoder(r.Body).Decode(&req)
	if strings.TrimSpace(req.ProviderRef) == "" {
		apierrors.Write(w, http.StatusBadRequest, "BAD_REQUEST", "provider_ref (bank reference) required for bank confirm", httpx.CorrelationID(r.Context()))
		return
	}
	p, err := h.svc.ConfirmBankPayment(r.Context(), claims.TenantID, paymentID, req.ProviderRef, claims.UserID)
	if err != nil {
		apierrors.Write(w, http.StatusBadRequest, "BAD_REQUEST", err.Error(), httpx.CorrelationID(r.Context()))
		return
	}
	httpx.JSON(w, http.StatusOK, p)
}

func canManualConfirm(claims *authz.Claims) bool {
	if claims == nil {
		return false
	}
	if claims.HasPermission("*") || claims.HasPermission("payments.manual_confirm") {
		return true
	}
	for _, r := range claims.Roles {
		if r == "owner" || r == "manager" {
			return true
		}
	}
	return false
}

func webhookTokenOK(r *http.Request) bool {
	want := strings.TrimSpace(os.Getenv("MPESA_WEBHOOK_TOKEN"))
	if want == "" {
		return true
	}
	return r.URL.Query().Get("token") == want
}

func (h *Handler) mpesaSTKWebhook(w http.ResponseWriter, r *http.Request) {
	if !webhookTokenOK(r) {
		httpx.JSON(w, http.StatusUnauthorized, map[string]string{"ResultCode": "1", "ResultDesc": "unauthorized"})
		return
	}
	raw, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
	if err != nil {
		httpx.JSON(w, http.StatusBadRequest, map[string]string{"ResultCode": "1", "ResultDesc": "invalid body"})
		return
	}
	var body struct {
		Body struct {
			StkCallback struct {
				MerchantRequestID string `json:"MerchantRequestID"`
				CheckoutRequestID string `json:"CheckoutRequestID"`
				ResultCode        int    `json:"ResultCode"`
				ResultDesc        string `json:"ResultDesc"`
				CallbackMetadata  *struct {
					Item []struct {
						Name  string `json:"Name"`
						Value any    `json:"Value"`
					} `json:"Item"`
				} `json:"CallbackMetadata"`
			} `json:"stkCallback"`
		} `json:"Body"`
	}
	if err := json.Unmarshal(raw, &body); err != nil {
		httpx.JSON(w, http.StatusBadRequest, map[string]string{"ResultCode": "1", "ResultDesc": "invalid json"})
		return
	}
	cb := body.Body.StkCallback
	if cb.CheckoutRequestID == "" {
		httpx.JSON(w, http.StatusOK, map[string]string{"ResultCode": "0", "ResultDesc": "Accepted"})
		return
	}
	var tenantID, paymentID uuid.UUID
	err = h.svc.pool.QueryRow(r.Context(), `
		SELECT tenant_id, payment_id FROM payments.mpesa_stk_transactions WHERE checkout_request_id = $1 LIMIT 1`,
		cb.CheckoutRequestID).Scan(&tenantID, &paymentID)
	if err != nil {
		httpx.JSON(w, http.StatusOK, map[string]string{"ResultCode": "0", "ResultDesc": "Accepted"})
		return
	}
	_, _ = h.svc.pool.Exec(r.Context(), `
		UPDATE payments.mpesa_stk_transactions SET raw_callback = $1::jsonb, updated_at = now()
		WHERE payment_id = $2`, string(raw), paymentID)
	if cb.ResultCode == 0 {
		// Treat callback as a hint — confirm via STK Query when possible.
		if _, err := h.svc.ReconcileSTKPayment(r.Context(), tenantID, paymentID); err != nil {
			ref := cb.CheckoutRequestID
			if cb.CallbackMetadata != nil {
				for _, it := range cb.CallbackMetadata.Item {
					if it.Name == "MpesaReceiptNumber" {
						ref = fmt.Sprint(it.Value)
						break
					}
				}
			}
			_, _ = h.svc.ConfirmMpesaWebhook(r.Context(), tenantID, paymentID, ref)
		}
	} else {
		_, _ = h.svc.pool.Exec(r.Context(), `
			UPDATE payments.mpesa_stk_transactions SET status = 'failed', result_code = $1, result_desc = $2, updated_at = now()
			WHERE payment_id = $3`, fmt.Sprintf("%d", cb.ResultCode), cb.ResultDesc, paymentID)
		_, _ = h.svc.pool.Exec(r.Context(), `
			UPDATE payments.payments SET status = 'failed', updated_at = now() WHERE id = $1`, paymentID)
	}
	httpx.JSON(w, http.StatusOK, map[string]string{"ResultCode": "0", "ResultDesc": "Accepted"})
}

func (h *Handler) mpesaC2BValidation(w http.ResponseWriter, r *http.Request) {
	if !webhookTokenOK(r) {
		httpx.JSON(w, http.StatusUnauthorized, map[string]any{"ResultCode": 1, "ResultDesc": "unauthorized"})
		return
	}
	var body map[string]any
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		httpx.JSON(w, http.StatusOK, map[string]any{"ResultCode": 0, "ResultDesc": "Accepted"})
		return
	}
	str := func(key string) string {
		if v, ok := body[key]; ok && v != nil {
			return fmt.Sprint(v)
		}
		return ""
	}
	ok, desc := h.svc.ValidateC2B(r.Context(), str("BusinessShortCode"), str("BillRefNumber"))
	code := 0
	if !ok {
		code = 1
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"ResultCode": code, "ResultDesc": desc})
}

func (h *Handler) mpesaC2BWebhook(w http.ResponseWriter, r *http.Request) {
	if !webhookTokenOK(r) {
		httpx.JSON(w, http.StatusUnauthorized, map[string]any{"ResultCode": 1, "ResultDesc": "unauthorized"})
		return
	}
	var body map[string]any
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		httpx.JSON(w, http.StatusBadRequest, map[string]any{"ResultCode": 1, "ResultDesc": "invalid json"})
		return
	}
	str := func(key string) string {
		if v, ok := body[key]; ok && v != nil {
			return fmt.Sprint(v)
		}
		return ""
	}
	amountStr := str("TransAmount")
	var amount float64
	_, _ = fmt.Sscanf(amountStr, "%f", &amount)
	payload, _ := json.Marshal(body)
	err := h.svc.ProcessC2BConfirmation(r.Context(), C2BPayload{
		TransID:           str("TransID"),
		TransType:         str("TransactionType"),
		TransTime:         str("TransTime"),
		Amount:            amount,
		BusinessShortCode: str("BusinessShortCode"),
		BillRefNumber:     str("BillRefNumber"),
		InvoiceNumber:     str("InvoiceNumber"),
		MSISDN:            str("MSISDN"),
		FirstName:         str("FirstName"),
		MiddleName:        str("MiddleName"),
		LastName:          str("LastName"),
		OrgAccountBalance: str("OrgAccountBalance"),
		ThirdPartyTransID: str("ThirdPartyTransID"),
		Raw:               payload,
	})
	if err != nil {
		// Still accept to Safaricom — retry storms should not reverse money.
		_ = err
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"ResultCode": 0, "ResultDesc": "Accepted"})
}

func (h *Handler) listC2B(w http.ResponseWriter, r *http.Request) {
	claims, _ := authz.FromContext(r.Context())
	if !claims.HasPermission("payments.read") && !claims.HasPermission("payments.initiate") && !claims.HasPermission("*") {
		apierrors.Write(w, http.StatusForbidden, "FORBIDDEN", "payments.read required", httpx.CorrelationID(r.Context()))
		return
	}
	items, err := h.svc.ListC2BTransactions(r.Context(), claims.TenantID, r.URL.Query().Get("status"))
	if err != nil {
		apierrors.Write(w, http.StatusInternalServerError, "INTERNAL", err.Error(), httpx.CorrelationID(r.Context()))
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"items": items})
}

func (h *Handler) matchC2B(w http.ResponseWriter, r *http.Request) {
	claims, _ := authz.FromContext(r.Context())
	if !claims.HasPermission("refunds.approve") && !claims.HasPermission("payments.initiate") && !claims.HasPermission("*") {
		// Managers/accountants typically approve money exceptions; owner has *.
		if !claims.HasPermission("cash.handover.confirm") {
			apierrors.Write(w, http.StatusForbidden, "FORBIDDEN", "insufficient permission to match C2B", httpx.CorrelationID(r.Context()))
			return
		}
	}
	c2bID, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		apierrors.Write(w, http.StatusBadRequest, "BAD_REQUEST", "invalid id", httpx.CorrelationID(r.Context()))
		return
	}
	var req struct {
		PaymentID uuid.UUID `json:"payment_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.PaymentID == uuid.Nil {
		apierrors.Write(w, http.StatusBadRequest, "BAD_REQUEST", "payment_id required", httpx.CorrelationID(r.Context()))
		return
	}
	pay, err := h.svc.MatchC2BToPayment(r.Context(), claims.TenantID, c2bID, req.PaymentID, claims.UserID)
	if err != nil {
		apierrors.Write(w, http.StatusConflict, "CONFLICT", err.Error(), httpx.CorrelationID(r.Context()))
		return
	}
	httpx.JSON(w, http.StatusOK, pay)
}

func (h *Handler) pendingCashTotal(w http.ResponseWriter, r *http.Request) {
	claims, _ := authz.FromContext(r.Context())
	total, err := h.svc.PendingCashTotal(r.Context(), claims.TenantID, claims.UserID)
	if err != nil {
		apierrors.Write(w, http.StatusInternalServerError, "INTERNAL", err.Error(), httpx.CorrelationID(r.Context()))
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"amount": total})
}

func (h *Handler) listHandovers(w http.ResponseWriter, r *http.Request) {
	claims, _ := authz.FromContext(r.Context())
	items, err := h.svc.ListCashHandovers(r.Context(), claims.TenantID, r.URL.Query().Get("status"))
	if err != nil {
		apierrors.Write(w, http.StatusInternalServerError, "INTERNAL", err.Error(), httpx.CorrelationID(r.Context()))
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"items": items})
}

func (h *Handler) requestHandover(w http.ResponseWriter, r *http.Request) {
	claims, _ := authz.FromContext(r.Context())
	var req struct {
		BranchID uuid.UUID  `json:"branch_id"`
		ToUserID *uuid.UUID `json:"to_user_id"`
		Amount   float64    `json:"amount"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Amount <= 0 {
		apierrors.Write(w, http.StatusBadRequest, "BAD_REQUEST", "amount required", httpx.CorrelationID(r.Context()))
		return
	}
	if req.BranchID == uuid.Nil && len(claims.BranchIDs) > 0 {
		req.BranchID, _ = uuid.Parse(claims.BranchIDs[0])
	}
	ho, err := h.svc.RequestCashHandover(r.Context(), claims.TenantID, req.BranchID, claims.UserID, req.ToUserID, req.Amount, corrID(r))
	if err != nil {
		apierrors.Write(w, http.StatusBadRequest, "BAD_REQUEST", err.Error(), httpx.CorrelationID(r.Context()))
		return
	}
	httpx.JSON(w, http.StatusCreated, ho)
}

func (h *Handler) confirmHandover(w http.ResponseWriter, r *http.Request) {
	claims, _ := authz.FromContext(r.Context())
	handoverID, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		apierrors.Write(w, http.StatusBadRequest, "BAD_REQUEST", "invalid id", httpx.CorrelationID(r.Context()))
		return
	}
	var req struct {
		CountedAmount *float64 `json:"counted_amount"`
	}
	_ = json.NewDecoder(r.Body).Decode(&req)
	ho, err := h.svc.ConfirmCashHandover(r.Context(), claims.TenantID, handoverID, claims.UserID, req.CountedAmount)
	if err != nil {
		if errors.Is(err, ErrSelfApproveHandover) {
			apierrors.Write(w, http.StatusConflict, "CONFLICT", err.Error(), httpx.CorrelationID(r.Context()))
			return
		}
		if strings.Contains(err.Error(), "not found") {
			apierrors.Write(w, http.StatusNotFound, "NOT_FOUND", err.Error(), httpx.CorrelationID(r.Context()))
			return
		}
		apierrors.Write(w, http.StatusConflict, "CONFLICT", err.Error(), httpx.CorrelationID(r.Context()))
		return
	}
	httpx.JSON(w, http.StatusOK, ho)
}

func (h *Handler) createRefund(w http.ResponseWriter, r *http.Request) {
	claims, _ := authz.FromContext(r.Context())
	if !claims.HasPermission("refunds.create") && !claims.HasPermission("payments.initiate") && !claims.HasPermission("*") {
		apierrors.Write(w, http.StatusForbidden, "FORBIDDEN", "refunds.create required", httpx.CorrelationID(r.Context()))
		return
	}
	var req struct {
		PaymentID uuid.UUID `json:"payment_id"`
		Amount    float64   `json:"amount"`
		Reason    *string   `json:"reason"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.PaymentID == uuid.Nil {
		apierrors.Write(w, http.StatusBadRequest, "BAD_REQUEST", "payment_id required", httpx.CorrelationID(r.Context()))
		return
	}
	ref, err := h.svc.CreateRefund(r.Context(), claims.TenantID, req.PaymentID, claims.UserID, corrID(r), req.Amount, req.Reason, nil)
	if err != nil {
		apierrors.Write(w, http.StatusBadRequest, "BAD_REQUEST", err.Error(), httpx.CorrelationID(r.Context()))
		return
	}
	httpx.JSON(w, http.StatusCreated, ref)
}

func (h *Handler) listRefunds(w http.ResponseWriter, r *http.Request) {
	claims, _ := authz.FromContext(r.Context())
	items, err := h.svc.ListRefunds(r.Context(), claims.TenantID, r.URL.Query().Get("status"))
	if err != nil {
		apierrors.Write(w, http.StatusInternalServerError, "INTERNAL", err.Error(), httpx.CorrelationID(r.Context()))
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"items": items})
}

func (h *Handler) approveRefund(w http.ResponseWriter, r *http.Request) {
	claims, _ := authz.FromContext(r.Context())
	if !claims.HasPermission("refunds.approve") && !claims.HasPermission("*") {
		apierrors.Write(w, http.StatusForbidden, "FORBIDDEN", "refunds.approve required", httpx.CorrelationID(r.Context()))
		return
	}
	refundID, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		apierrors.Write(w, http.StatusBadRequest, "BAD_REQUEST", "invalid id", httpx.CorrelationID(r.Context()))
		return
	}
	ref, err := h.svc.ApproveRefund(r.Context(), claims.TenantID, refundID, claims.UserID)
	if err != nil {
		if errors.Is(err, ErrSelfApproveRefund) {
			apierrors.Write(w, http.StatusConflict, "CONFLICT", err.Error(), httpx.CorrelationID(r.Context()))
			return
		}
		apierrors.Write(w, http.StatusConflict, "CONFLICT", err.Error(), httpx.CorrelationID(r.Context()))
		return
	}
	httpx.JSON(w, http.StatusOK, ref)
}

func (h *Handler) getStoreCredit(w http.ResponseWriter, r *http.Request) {
	claims, _ := authz.FromContext(r.Context())
	customerID, err := uuid.Parse(r.PathValue("customer_id"))
	if err != nil {
		apierrors.Write(w, http.StatusBadRequest, "BAD_REQUEST", "invalid customer_id", httpx.CorrelationID(r.Context()))
		return
	}
	sc, err := h.svc.GetStoreCredit(r.Context(), claims.TenantID, customerID)
	if err != nil {
		apierrors.Write(w, http.StatusInternalServerError, "INTERNAL", err.Error(), httpx.CorrelationID(r.Context()))
		return
	}
	httpx.JSON(w, http.StatusOK, sc)
}

func (h *Handler) issueStoreCredit(w http.ResponseWriter, r *http.Request) {
	claims, _ := authz.FromContext(r.Context())
	if !claims.HasPermission("store_credit.manage") && !canManagePaymentSettings(claims) {
		apierrors.Write(w, http.StatusForbidden, "FORBIDDEN", "store_credit.manage required", httpx.CorrelationID(r.Context()))
		return
	}
	customerID, err := uuid.Parse(r.PathValue("customer_id"))
	if err != nil {
		apierrors.Write(w, http.StatusBadRequest, "BAD_REQUEST", "invalid customer_id", httpx.CorrelationID(r.Context()))
		return
	}
	var req struct {
		Amount float64 `json:"amount"`
		Note   string  `json:"note"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Amount <= 0 {
		apierrors.Write(w, http.StatusBadRequest, "BAD_REQUEST", "amount required", httpx.CorrelationID(r.Context()))
		return
	}
	sc, err := h.svc.IssueStoreCredit(r.Context(), claims.TenantID, customerID, claims.UserID, req.Amount, req.Note)
	if err != nil {
		apierrors.Write(w, http.StatusBadRequest, "BAD_REQUEST", err.Error(), httpx.CorrelationID(r.Context()))
		return
	}
	httpx.JSON(w, http.StatusCreated, sc)
}

func corrID(r *http.Request) uuid.UUID {
	if cid := httpx.CorrelationID(r.Context()); cid != "" {
		if id, err := uuid.Parse(cid); err == nil {
			return id
		}
	}
	return uuid.New()
}
