package payments

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"

	"github.com/google/uuid"
	"github.com/techlane/techlane/packages/pkg/apierrors"
	"github.com/techlane/techlane/packages/pkg/httpx"
)

// CustomerRepairGateway lets payments verify customer ownership and compute balance due.
type CustomerRepairGateway interface {
	DefaultTenantID(ctx context.Context) (uuid.UUID, error)
	AuthenticateCustomer(ctx context.Context, tenantID uuid.UUID, token string) (customerID uuid.UUID, phone *string, err error)
	AssertCustomerOwnsRepair(ctx context.Context, tenantID, customerID, repairID uuid.UUID) error
	RepairPaymentContext(ctx context.Context, tenantID, repairID uuid.UUID) (branchID uuid.UUID, balance float64, defaultPhone string, accountRef string, err error)
}

// RepairCustomerAdapter adapts repair.Service methods for customer payments.
type RepairCustomerAdapter struct {
	Authenticate     func(ctx context.Context, tenantID uuid.UUID, token string) (customerID uuid.UUID, phone *string, err error)
	DefaultTenant    func(ctx context.Context) (uuid.UUID, error)
	AssertOwnsRepair func(ctx context.Context, tenantID, customerID, repairID uuid.UUID) error
	PaymentContext   func(ctx context.Context, tenantID, repairID uuid.UUID) (branchID uuid.UUID, balance float64, defaultPhone string, accountRef string, err error)
}

func (a RepairCustomerAdapter) DefaultTenantID(ctx context.Context) (uuid.UUID, error) {
	return a.DefaultTenant(ctx)
}

func (a RepairCustomerAdapter) AuthenticateCustomer(ctx context.Context, tenantID uuid.UUID, token string) (uuid.UUID, *string, error) {
	return a.Authenticate(ctx, tenantID, token)
}

func (a RepairCustomerAdapter) AssertCustomerOwnsRepair(ctx context.Context, tenantID, customerID, repairID uuid.UUID) error {
	return a.AssertOwnsRepair(ctx, tenantID, customerID, repairID)
}

func (a RepairCustomerAdapter) RepairPaymentContext(ctx context.Context, tenantID, repairID uuid.UUID) (uuid.UUID, float64, string, string, error) {
	return a.PaymentContext(ctx, tenantID, repairID)
}

func (h *Handler) SetCustomerRepairGateway(g CustomerRepairGateway) {
	h.customerRepair = g
}

func (h *Handler) customerRepairPay(w http.ResponseWriter, r *http.Request) {
	if h.customerRepair == nil {
		apierrors.Write(w, http.StatusServiceUnavailable, "UNAVAILABLE", "customer payments not configured", httpx.CorrelationID(r.Context()))
		return
	}
	tenantID, err := h.customerRepair.DefaultTenantID(r.Context())
	if err != nil {
		apierrors.Write(w, http.StatusServiceUnavailable, "UNAVAILABLE", err.Error(), httpx.CorrelationID(r.Context()))
		return
	}
	token := bearerToken(r)
	customerID, customerPhone, err := h.customerRepair.AuthenticateCustomer(r.Context(), tenantID, token)
	if err != nil {
		apierrors.Write(w, http.StatusUnauthorized, "UNAUTHORIZED", err.Error(), httpx.CorrelationID(r.Context()))
		return
	}
	repairID, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		apierrors.Write(w, http.StatusBadRequest, "BAD_REQUEST", "invalid repair id", httpx.CorrelationID(r.Context()))
		return
	}
	if err := h.customerRepair.AssertCustomerOwnsRepair(r.Context(), tenantID, customerID, repairID); err != nil {
		status := http.StatusNotFound
		if !strings.Contains(err.Error(), "not found") {
			status = http.StatusForbidden
		}
		apierrors.Write(w, status, "FORBIDDEN", err.Error(), httpx.CorrelationID(r.Context()))
		return
	}
	var req struct {
		Method string `json:"method"`
		Phone  string `json:"phone"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		apierrors.Write(w, http.StatusBadRequest, "BAD_REQUEST", "invalid body", httpx.CorrelationID(r.Context()))
		return
	}
	if req.Method == "" {
		req.Method = "mpesa_stk"
	}
	if req.Method != "mpesa_stk" {
		apierrors.Write(w, http.StatusBadRequest, "BAD_REQUEST", "only mpesa_stk is supported for customer pay", httpx.CorrelationID(r.Context()))
		return
	}
	branchID, balance, defaultPhone, accountRef, err := h.customerRepair.RepairPaymentContext(r.Context(), tenantID, repairID)
	if err != nil {
		apierrors.Write(w, http.StatusBadRequest, "BAD_REQUEST", err.Error(), httpx.CorrelationID(r.Context()))
		return
	}
	if balance <= 0 {
		apierrors.Write(w, http.StatusConflict, "NO_BALANCE_DUE", "nothing to pay", httpx.CorrelationID(r.Context()))
		return
	}
	phone := strings.TrimSpace(req.Phone)
	if phone == "" {
		phone = defaultPhone
	}
	if phone == "" && customerPhone != nil {
		phone = *customerPhone
	}
	if phone == "" {
		apierrors.Write(w, http.StatusBadRequest, "BAD_REQUEST", "phone required for M-Pesa STK", httpx.CorrelationID(r.Context()))
		return
	}
	p, err := h.svc.CreatePayment(r.Context(), CreatePaymentInput{
		TenantID: tenantID, BranchID: &branchID, Method: req.Method, Amount: balance,
		Currency: "KES", PayableType: "repair", PayableID: repairID,
		Phone: phone, AccountRef: accountRef,
		ActorID: customerID, CorrID: corrID(r),
	})
	if err != nil {
		apierrors.Write(w, http.StatusBadRequest, "PAYMENT_FAILED", err.Error(), httpx.CorrelationID(r.Context()))
		return
	}
	httpx.JSON(w, http.StatusCreated, p)
}

func (h *Handler) customerRepairPaymentStatus(w http.ResponseWriter, r *http.Request) {
	if h.customerRepair == nil {
		apierrors.Write(w, http.StatusServiceUnavailable, "UNAVAILABLE", "customer payments not configured", httpx.CorrelationID(r.Context()))
		return
	}
	tenantID, err := h.customerRepair.DefaultTenantID(r.Context())
	if err != nil {
		apierrors.Write(w, http.StatusServiceUnavailable, "UNAVAILABLE", err.Error(), httpx.CorrelationID(r.Context()))
		return
	}
	token := bearerToken(r)
	customerID, _, err := h.customerRepair.AuthenticateCustomer(r.Context(), tenantID, token)
	if err != nil {
		apierrors.Write(w, http.StatusUnauthorized, "UNAUTHORIZED", err.Error(), httpx.CorrelationID(r.Context()))
		return
	}
	repairID, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		apierrors.Write(w, http.StatusBadRequest, "BAD_REQUEST", "invalid repair id", httpx.CorrelationID(r.Context()))
		return
	}
	paymentID, err := uuid.Parse(r.PathValue("payment_id"))
	if err != nil {
		apierrors.Write(w, http.StatusBadRequest, "BAD_REQUEST", "invalid payment id", httpx.CorrelationID(r.Context()))
		return
	}
	if err := h.customerRepair.AssertCustomerOwnsRepair(r.Context(), tenantID, customerID, repairID); err != nil {
		apierrors.Write(w, http.StatusNotFound, "NOT_FOUND", "repair not found", httpx.CorrelationID(r.Context()))
		return
	}
	items, err := h.svc.ListPaymentsForPayable(r.Context(), tenantID, "repair", repairID)
	if err != nil {
		apierrors.Write(w, http.StatusInternalServerError, "INTERNAL", err.Error(), httpx.CorrelationID(r.Context()))
		return
	}
	for i := range items {
		if items[i].ID == paymentID {
			httpx.JSON(w, http.StatusOK, items[i])
			return
		}
	}
	apierrors.Write(w, http.StatusNotFound, "NOT_FOUND", "payment not found", httpx.CorrelationID(r.Context()))
}

func bearerToken(r *http.Request) string {
	value := strings.TrimSpace(r.Header.Get("Authorization"))
	if len(value) > 7 && strings.EqualFold(value[:7], "Bearer ") {
		return strings.TrimSpace(value[7:])
	}
	return ""
}
