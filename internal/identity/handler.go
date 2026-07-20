package identity

import (
	"encoding/json"
	"errors"
	"net/http"

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
	mux.HandleFunc("POST /auth/login", h.login)
	mux.HandleFunc("POST /auth/refresh", h.refresh)
	mux.Handle("GET /me", auth(http.HandlerFunc(h.me)))

	mux.Handle("GET /branches", auth(http.HandlerFunc(h.listBranches)))
	mux.Handle("POST /branches", auth(httpx.RequirePermission("users.write")(http.HandlerFunc(h.createBranch))))
	mux.Handle("PATCH /branches/{id}", auth(httpx.RequirePermission("users.write")(http.HandlerFunc(h.updateBranch))))
	mux.Handle("DELETE /branches/{id}", auth(httpx.RequirePermission("users.write")(http.HandlerFunc(h.deleteBranch))))
	mux.Handle("GET /roles", auth(httpx.RequirePermission("users.read")(http.HandlerFunc(h.listRoles))))
	mux.Handle("POST /roles", auth(httpx.RequirePermission("roles.write")(http.HandlerFunc(h.createRole))))
	mux.Handle("GET /roles/{id}", auth(httpx.RequirePermission("users.read")(http.HandlerFunc(h.getRole))))
	mux.Handle("PATCH /roles/{id}", auth(httpx.RequirePermission("roles.write")(http.HandlerFunc(h.updateRole))))
	mux.Handle("DELETE /roles/{id}", auth(httpx.RequirePermission("roles.write")(http.HandlerFunc(h.deleteRole))))
	mux.Handle("GET /permissions", auth(httpx.RequirePermission("users.read")(http.HandlerFunc(h.listPermissions))))
	mux.Handle("POST /permissions", auth(httpx.RequirePermission("roles.write")(http.HandlerFunc(h.createPermission))))
	mux.Handle("GET /users", auth(httpx.RequirePermission("users.read")(http.HandlerFunc(h.listUsers))))
	mux.Handle("POST /users", auth(httpx.RequirePermission("users.write")(http.HandlerFunc(h.createUser))))
	mux.Handle("GET /users/{id}", auth(httpx.RequirePermission("users.read")(http.HandlerFunc(h.getUser))))
	mux.Handle("PATCH /users/{id}", auth(httpx.RequirePermission("users.write")(http.HandlerFunc(h.updateUser))))
	mux.Handle("PUT /users/{id}/commission", auth(httpx.RequirePermission("commissions.write")(http.HandlerFunc(h.setCommission))))
	mux.Handle("GET /technicians", auth(http.HandlerFunc(h.listTechnicians)))
	mux.Handle("GET /commissions", auth(httpx.RequirePermission("commissions.read")(http.HandlerFunc(h.listCommissions))))
	mux.Handle("POST /commissions/{id}/approve", auth(httpx.RequirePermission("commissions.approve")(http.HandlerFunc(h.approveCommission))))
	mux.Handle("POST /commissions/{id}/mark-paid", auth(httpx.RequirePermission("commissions.approve")(http.HandlerFunc(h.markPaid))))

	mux.Handle("POST /devices/register", auth(http.HandlerFunc(h.registerDevice)))
	mux.Handle("GET /devices", auth(http.HandlerFunc(h.listDevices)))
	mux.Handle("POST /devices/{id}/revoke", auth(http.HandlerFunc(h.revokeDevice)))
	mux.Handle("GET /shop/profile", auth(http.HandlerFunc(h.getShopProfile)))
	mux.Handle("PUT /shop/profile", auth(httpx.RequirePermission("users.write")(http.HandlerFunc(h.putShopProfile))))
}

func (h *Handler) login(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		apierrors.Write(w, http.StatusBadRequest, "BAD_REQUEST", "invalid json", httpx.CorrelationID(r.Context()))
		return
	}
	pair, err := h.svc.Login(r.Context(), req.Email, req.Password)
	if err != nil {
		if errors.Is(err, ErrInvalidCredentials) {
			apierrors.Write(w, http.StatusUnauthorized, "UNAUTHORIZED", "invalid credentials", httpx.CorrelationID(r.Context()))
			return
		}
		apierrors.Write(w, http.StatusInternalServerError, "INTERNAL", err.Error(), httpx.CorrelationID(r.Context()))
		return
	}
	httpx.JSON(w, http.StatusOK, pair)
}

func (h *Handler) refresh(w http.ResponseWriter, r *http.Request) {
	var req struct {
		RefreshToken string `json:"refresh_token"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		apierrors.Write(w, http.StatusBadRequest, "BAD_REQUEST", "invalid json", httpx.CorrelationID(r.Context()))
		return
	}
	pair, err := h.svc.Refresh(r.Context(), req.RefreshToken)
	if err != nil {
		if errors.Is(err, ErrInvalidRefresh) {
			apierrors.Write(w, http.StatusUnauthorized, "UNAUTHORIZED", "invalid refresh token", httpx.CorrelationID(r.Context()))
			return
		}
		apierrors.Write(w, http.StatusInternalServerError, "INTERNAL", err.Error(), httpx.CorrelationID(r.Context()))
		return
	}
	httpx.JSON(w, http.StatusOK, pair)
}

func (h *Handler) me(w http.ResponseWriter, r *http.Request) {
	claims, ok := authz.FromContext(r.Context())
	if !ok {
		apierrors.Write(w, http.StatusUnauthorized, "UNAUTHORIZED", "missing claims", httpx.CorrelationID(r.Context()))
		return
	}
	profile, err := h.svc.GetMe(r.Context(), claims.UserID)
	if err != nil {
		apierrors.Write(w, http.StatusInternalServerError, "INTERNAL", err.Error(), httpx.CorrelationID(r.Context()))
		return
	}
	httpx.JSON(w, http.StatusOK, profile)
}

func (h *Handler) listBranches(w http.ResponseWriter, r *http.Request) {
	claims, _ := authz.FromContext(r.Context())
	items, err := h.svc.ListBranches(r.Context(), claims.TenantID)
	if err != nil {
		apierrors.Write(w, http.StatusInternalServerError, "INTERNAL", err.Error(), httpx.CorrelationID(r.Context()))
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"items": items})
}

func (h *Handler) createBranch(w http.ResponseWriter, r *http.Request) {
	claims, _ := authz.FromContext(r.Context())
	var req struct {
		Name string `json:"name"`
		Code string `json:"code"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		apierrors.Write(w, http.StatusBadRequest, "BAD_REQUEST", "invalid json", httpx.CorrelationID(r.Context()))
		return
	}
	branch, err := h.svc.CreateBranch(r.Context(), claims.TenantID, req.Name, req.Code)
	if err != nil {
		writeIdentityErr(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusCreated, branch)
}

func (h *Handler) updateBranch(w http.ResponseWriter, r *http.Request) {
	claims, _ := authz.FromContext(r.Context())
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		apierrors.Write(w, http.StatusBadRequest, "BAD_REQUEST", "invalid id", httpx.CorrelationID(r.Context()))
		return
	}
	var req struct {
		Name *string `json:"name"`
		Code *string `json:"code"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		apierrors.Write(w, http.StatusBadRequest, "BAD_REQUEST", "invalid json", httpx.CorrelationID(r.Context()))
		return
	}
	branch, err := h.svc.UpdateBranch(r.Context(), claims.TenantID, id, req.Name, req.Code)
	if err != nil {
		writeIdentityErr(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, branch)
}

func (h *Handler) deleteBranch(w http.ResponseWriter, r *http.Request) {
	claims, _ := authz.FromContext(r.Context())
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		apierrors.Write(w, http.StatusBadRequest, "BAD_REQUEST", "invalid id", httpx.CorrelationID(r.Context()))
		return
	}
	if err := h.svc.DeleteBranch(r.Context(), claims.TenantID, id); err != nil {
		writeIdentityErr(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) listRoles(w http.ResponseWriter, r *http.Request) {
	claims, _ := authz.FromContext(r.Context())
	items, err := h.svc.ListRolesForTenant(r.Context(), claims.TenantID)
	if err != nil {
		apierrors.Write(w, http.StatusInternalServerError, "INTERNAL", err.Error(), httpx.CorrelationID(r.Context()))
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"items": items})
}

func (h *Handler) getRole(w http.ResponseWriter, r *http.Request) {
	claims, _ := authz.FromContext(r.Context())
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		apierrors.Write(w, http.StatusBadRequest, "BAD_REQUEST", "invalid id", httpx.CorrelationID(r.Context()))
		return
	}
	role, err := h.svc.GetRole(r.Context(), claims.TenantID, id)
	if err != nil {
		writeIdentityErr(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, role)
}

func (h *Handler) createRole(w http.ResponseWriter, r *http.Request) {
	claims, _ := authz.FromContext(r.Context())
	var req struct {
		Key         string   `json:"key"`
		Name        string   `json:"name"`
		Description string   `json:"description"`
		Permissions []string `json:"permissions"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		apierrors.Write(w, http.StatusBadRequest, "BAD_REQUEST", "invalid json", httpx.CorrelationID(r.Context()))
		return
	}
	role, err := h.svc.CreateRole(r.Context(), claims.TenantID, claims.UserID, CreateRoleInput{
		Key: req.Key, Name: req.Name, Description: req.Description, Permissions: req.Permissions,
	})
	if err != nil {
		writeIdentityErr(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusCreated, role)
}

func (h *Handler) updateRole(w http.ResponseWriter, r *http.Request) {
	claims, _ := authz.FromContext(r.Context())
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		apierrors.Write(w, http.StatusBadRequest, "BAD_REQUEST", "invalid id", httpx.CorrelationID(r.Context()))
		return
	}
	var req struct {
		Name        *string  `json:"name"`
		Description *string  `json:"description"`
		Permissions []string `json:"permissions"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		apierrors.Write(w, http.StatusBadRequest, "BAD_REQUEST", "invalid json", httpx.CorrelationID(r.Context()))
		return
	}
	role, err := h.svc.UpdateRole(r.Context(), claims.TenantID, id, claims.UserID, UpdateRoleInput{
		Name: req.Name, Description: req.Description, Permissions: req.Permissions,
	})
	if err != nil {
		writeIdentityErr(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, role)
}

func (h *Handler) deleteRole(w http.ResponseWriter, r *http.Request) {
	claims, _ := authz.FromContext(r.Context())
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		apierrors.Write(w, http.StatusBadRequest, "BAD_REQUEST", "invalid id", httpx.CorrelationID(r.Context()))
		return
	}
	if err := h.svc.DeleteRole(r.Context(), claims.TenantID, id); err != nil {
		writeIdentityErr(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) listPermissions(w http.ResponseWriter, r *http.Request) {
	items, err := h.svc.ListPermissionCatalog(r.Context())
	if err != nil {
		apierrors.Write(w, http.StatusInternalServerError, "INTERNAL", err.Error(), httpx.CorrelationID(r.Context()))
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"items": items})
}

func (h *Handler) createPermission(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Code        string `json:"code"`
		Description string `json:"description"`
		Category    string `json:"category"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		apierrors.Write(w, http.StatusBadRequest, "BAD_REQUEST", "invalid json", httpx.CorrelationID(r.Context()))
		return
	}
	p, err := h.svc.CreatePermission(r.Context(), req.Code, req.Description, req.Category)
	if err != nil {
		writeIdentityErr(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusCreated, p)
}

func (h *Handler) listUsers(w http.ResponseWriter, r *http.Request) {
	claims, _ := authz.FromContext(r.Context())
	role := r.URL.Query().Get("role")
	status := r.URL.Query().Get("status")
	items, err := h.svc.ListUsers(r.Context(), claims.TenantID, role, status)
	if err != nil {
		apierrors.Write(w, http.StatusInternalServerError, "INTERNAL", err.Error(), httpx.CorrelationID(r.Context()))
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"items": items})
}

func (h *Handler) listTechnicians(w http.ResponseWriter, r *http.Request) {
	claims, ok := authz.FromContext(r.Context())
	if !ok {
		apierrors.Write(w, http.StatusUnauthorized, "UNAUTHORIZED", "missing claims", httpx.CorrelationID(r.Context()))
		return
	}
	items, err := h.svc.ListTechnicians(r.Context(), claims.TenantID)
	if err != nil {
		apierrors.Write(w, http.StatusInternalServerError, "INTERNAL", err.Error(), httpx.CorrelationID(r.Context()))
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"items": items})
}

func (h *Handler) getUser(w http.ResponseWriter, r *http.Request) {
	claims, _ := authz.FromContext(r.Context())
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		apierrors.Write(w, http.StatusBadRequest, "BAD_REQUEST", "invalid id", httpx.CorrelationID(r.Context()))
		return
	}
	user, err := h.svc.GetUser(r.Context(), claims.TenantID, id)
	if err != nil {
		writeIdentityErr(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, user)
}

func (h *Handler) createUser(w http.ResponseWriter, r *http.Request) {
	claims, _ := authz.FromContext(r.Context())
	var req struct {
		Email        string   `json:"email"`
		DisplayName  string   `json:"display_name"`
		Password     string   `json:"password"`
		Roles        []string `json:"roles"`
		BranchIDs    []string `json:"branch_ids"`
		Phone        *string  `json:"phone"`
		EmployeeCode *string  `json:"employee_code"`
		IsTechnician bool     `json:"is_technician"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		apierrors.Write(w, http.StatusBadRequest, "BAD_REQUEST", "invalid json", httpx.CorrelationID(r.Context()))
		return
	}
	branchIDs := make([]uuid.UUID, 0, len(req.BranchIDs))
	for _, s := range req.BranchIDs {
		id, err := uuid.Parse(s)
		if err != nil {
			apierrors.Write(w, http.StatusBadRequest, "BAD_REQUEST", "invalid branch_id", httpx.CorrelationID(r.Context()))
			return
		}
		branchIDs = append(branchIDs, id)
	}
	user, err := h.svc.CreateUser(r.Context(), claims.TenantID, claims.UserID, CreateUserInput{
		Email: req.Email, DisplayName: req.DisplayName, Password: req.Password,
		Roles: req.Roles, BranchIDs: branchIDs, Phone: req.Phone, EmployeeCode: req.EmployeeCode,
		IsTechnician: req.IsTechnician,
	})
	if err != nil {
		writeIdentityErr(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusCreated, user)
}

func (h *Handler) updateUser(w http.ResponseWriter, r *http.Request) {
	claims, _ := authz.FromContext(r.Context())
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		apierrors.Write(w, http.StatusBadRequest, "BAD_REQUEST", "invalid id", httpx.CorrelationID(r.Context()))
		return
	}
	var req struct {
		DisplayName  *string  `json:"display_name"`
		Status       *string  `json:"status"`
		Roles        []string `json:"roles"`
		BranchIDs    []string `json:"branch_ids"`
		Phone        *string  `json:"phone"`
		EmployeeCode *string  `json:"employee_code"`
		IsTechnician *bool    `json:"is_technician"`
		Password     *string  `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		apierrors.Write(w, http.StatusBadRequest, "BAD_REQUEST", "invalid json", httpx.CorrelationID(r.Context()))
		return
	}
	in := UpdateUserInput{
		DisplayName: req.DisplayName, Status: req.Status, Roles: req.Roles,
		Phone: req.Phone, EmployeeCode: req.EmployeeCode, IsTechnician: req.IsTechnician, Password: req.Password,
	}
	if req.BranchIDs != nil {
		in.BranchIDs = make([]uuid.UUID, 0, len(req.BranchIDs))
		for _, s := range req.BranchIDs {
			bid, err := uuid.Parse(s)
			if err != nil {
				apierrors.Write(w, http.StatusBadRequest, "BAD_REQUEST", "invalid branch_id", httpx.CorrelationID(r.Context()))
				return
			}
			in.BranchIDs = append(in.BranchIDs, bid)
		}
	}
	user, err := h.svc.UpdateUser(r.Context(), claims.TenantID, id, claims.UserID, in)
	if err != nil {
		writeIdentityErr(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, user)
}

func (h *Handler) setCommission(w http.ResponseWriter, r *http.Request) {
	claims, _ := authz.FromContext(r.Context())
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		apierrors.Write(w, http.StatusBadRequest, "BAD_REQUEST", "invalid id", httpx.CorrelationID(r.Context()))
		return
	}
	var req CommissionConfigInput
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		apierrors.Write(w, http.StatusBadRequest, "BAD_REQUEST", "invalid json", httpx.CorrelationID(r.Context()))
		return
	}
	user, err := h.svc.SetCommission(r.Context(), claims.TenantID, id, claims.UserID, req)
	if err != nil {
		writeIdentityErr(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, user)
}

func (h *Handler) listCommissions(w http.ResponseWriter, r *http.Request) {
	claims, _ := authz.FromContext(r.Context())
	status := r.URL.Query().Get("status")
	var userID *uuid.UUID
	if s := r.URL.Query().Get("user_id"); s != "" {
		id, err := uuid.Parse(s)
		if err != nil {
			apierrors.Write(w, http.StatusBadRequest, "BAD_REQUEST", "invalid user_id", httpx.CorrelationID(r.Context()))
			return
		}
		userID = &id
	}
	items, err := h.svc.ListCommissions(r.Context(), claims.TenantID, status, userID)
	if err != nil {
		apierrors.Write(w, http.StatusInternalServerError, "INTERNAL", err.Error(), httpx.CorrelationID(r.Context()))
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"items": items})
}

func (h *Handler) approveCommission(w http.ResponseWriter, r *http.Request) {
	claims, _ := authz.FromContext(r.Context())
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		apierrors.Write(w, http.StatusBadRequest, "BAD_REQUEST", "invalid id", httpx.CorrelationID(r.Context()))
		return
	}
	entry, err := h.svc.ApproveCommission(r.Context(), claims.TenantID, id, claims.UserID)
	if err != nil {
		writeIdentityErr(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, entry)
}

func (h *Handler) markPaid(w http.ResponseWriter, r *http.Request) {
	claims, _ := authz.FromContext(r.Context())
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		apierrors.Write(w, http.StatusBadRequest, "BAD_REQUEST", "invalid id", httpx.CorrelationID(r.Context()))
		return
	}
	entry, err := h.svc.MarkCommissionPaid(r.Context(), claims.TenantID, id, claims.UserID)
	if err != nil {
		writeIdentityErr(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, entry)
}

func (h *Handler) registerDevice(w http.ResponseWriter, r *http.Request) {
	claims, _ := authz.FromContext(r.Context())
	var req struct {
		ID          *uuid.UUID `json:"id"`
		DeviceName  *string    `json:"device_name"`
		Platform    *string    `json:"platform"`
		Fingerprint *string    `json:"fingerprint"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		apierrors.Write(w, http.StatusBadRequest, "BAD_REQUEST", "invalid json", httpx.CorrelationID(r.Context()))
		return
	}
	device, err := h.svc.RegisterDevice(r.Context(), claims.TenantID, claims.UserID, RegisterDeviceInput{
		ID: req.ID, DeviceName: req.DeviceName, Platform: req.Platform, Fingerprint: req.Fingerprint,
	})
	if err != nil {
		writeIdentityErr(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, device)
}

func (h *Handler) listDevices(w http.ResponseWriter, r *http.Request) {
	claims, _ := authz.FromContext(r.Context())
	items, err := h.svc.ListDevices(r.Context(), claims.TenantID, claims.UserID)
	if err != nil {
		apierrors.Write(w, http.StatusInternalServerError, "INTERNAL", err.Error(), httpx.CorrelationID(r.Context()))
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"items": items})
}

func (h *Handler) revokeDevice(w http.ResponseWriter, r *http.Request) {
	claims, _ := authz.FromContext(r.Context())
	deviceID, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		apierrors.Write(w, http.StatusBadRequest, "BAD_REQUEST", "invalid id", httpx.CorrelationID(r.Context()))
		return
	}
	if err := h.svc.RevokeDevice(r.Context(), claims.TenantID, claims.UserID, deviceID); err != nil {
		writeIdentityErr(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func writeIdentityErr(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, ErrNotFound):
		apierrors.Write(w, http.StatusNotFound, "NOT_FOUND", err.Error(), httpx.CorrelationID(r.Context()))
	case errors.Is(err, ErrForbidden):
		apierrors.Write(w, http.StatusForbidden, "FORBIDDEN", err.Error(), httpx.CorrelationID(r.Context()))
	case errors.Is(err, ErrInvalidInput):
		apierrors.Write(w, http.StatusBadRequest, "BAD_REQUEST", err.Error(), httpx.CorrelationID(r.Context()))
	case errors.Is(err, ErrDuplicateUser):
		apierrors.Write(w, http.StatusConflict, "CONFLICT", err.Error(), httpx.CorrelationID(r.Context()))
	default:
		apierrors.Write(w, http.StatusInternalServerError, "INTERNAL", err.Error(), httpx.CorrelationID(r.Context()))
	}
}

func (h *Handler) getShopProfile(w http.ResponseWriter, r *http.Request) {
	claims, ok := authz.FromContext(r.Context())
	if !ok {
		apierrors.Write(w, http.StatusUnauthorized, "UNAUTHORIZED", "missing claims", httpx.CorrelationID(r.Context()))
		return
	}
	profile, err := h.svc.GetShopProfile(r.Context(), claims.TenantID)
	if err != nil {
		apierrors.Write(w, http.StatusInternalServerError, "INTERNAL", err.Error(), httpx.CorrelationID(r.Context()))
		return
	}
	httpx.JSON(w, http.StatusOK, profile)
}

func (h *Handler) putShopProfile(w http.ResponseWriter, r *http.Request) {
	claims, ok := authz.FromContext(r.Context())
	if !ok {
		apierrors.Write(w, http.StatusUnauthorized, "UNAUTHORIZED", "missing claims", httpx.CorrelationID(r.Context()))
		return
	}
	var req struct {
		LegalName    *string `json:"legal_name"`
		TIN          *string `json:"tin"`
		AddressLine1 *string `json:"address_line1"`
		AddressLine2 *string `json:"address_line2"`
		City         *string `json:"city"`
		Country      *string `json:"country"`
		VATRateBPS   *int    `json:"vat_rate_bps"`
		VATInclusive *bool   `json:"vat_inclusive"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		apierrors.Write(w, http.StatusBadRequest, "BAD_REQUEST", "invalid json", httpx.CorrelationID(r.Context()))
		return
	}
	profile, err := h.svc.UpsertShopProfile(r.Context(), claims.TenantID, UpsertShopProfileInput{
		LegalName: req.LegalName, TIN: req.TIN, AddressLine1: req.AddressLine1,
		AddressLine2: req.AddressLine2, City: req.City, Country: req.Country,
		VATRateBPS: req.VATRateBPS, VATInclusive: req.VATInclusive,
	})
	if err != nil {
		apierrors.Write(w, http.StatusBadRequest, "BAD_REQUEST", err.Error(), httpx.CorrelationID(r.Context()))
		return
	}
	httpx.JSON(w, http.StatusOK, profile)
}
