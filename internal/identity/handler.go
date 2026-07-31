package identity

import (
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/techlane/techlane/packages/pkg/apierrors"
	"github.com/techlane/techlane/packages/pkg/authz"
	"github.com/techlane/techlane/packages/pkg/httpx"
)

type Handler struct {
	svc                      *Service
	onPasswordResetRequested func(email, displayName, token string)
	authLimiter              *httpx.IPRateLimiter
	signupEnabled            bool
}

func NewHandler(svc *Service) *Handler {
	return &Handler{
		svc: svc,
		// 20 attempts / 5 minutes per IP across login+refresh+MFA+reset is generous for
		// a shared shop-floor terminal but blunts scripted brute force from a single source.
		authLimiter:   httpx.NewIPRateLimiter(20, 5*time.Minute),
		signupEnabled: true,
	}
}

// SetPasswordResetNotifier wires an email sender for the forgot-password flow.
func (h *Handler) SetPasswordResetNotifier(fn func(email, displayName, token string)) {
	h.onPasswordResetRequested = fn
}

// SetSignupEnabled toggles the self-serve /auth/signup endpoint. Self-hosted
// operators who only want their own staff on the platform can disable it via
// the SIGNUP_ENABLED env var; defaults to enabled.
func (h *Handler) SetSignupEnabled(enabled bool) {
	h.signupEnabled = enabled
}

func (h *Handler) Register(mux *http.ServeMux, auth func(http.Handler) http.Handler) {
	mux.Handle("POST /auth/signup", h.authLimiter.Middleware(http.HandlerFunc(h.signup)))
	mux.Handle("POST /auth/login", h.authLimiter.Middleware(http.HandlerFunc(h.login)))
	mux.Handle("POST /auth/refresh", h.authLimiter.Middleware(http.HandlerFunc(h.refresh)))
	mux.Handle("POST /auth/mfa/verify", h.authLimiter.Middleware(http.HandlerFunc(h.mfaVerify)))
	mux.Handle("POST /auth/forgot-password", h.authLimiter.Middleware(http.HandlerFunc(h.forgotPassword)))
	mux.Handle("POST /auth/reset-password", h.authLimiter.Middleware(http.HandlerFunc(h.resetPassword)))
	mux.Handle("GET /auth/mfa/status", auth(http.HandlerFunc(h.mfaStatus)))
	mux.Handle("POST /auth/mfa/setup", auth(http.HandlerFunc(h.mfaSetup)))
	mux.Handle("POST /auth/mfa/enable", auth(http.HandlerFunc(h.mfaEnable)))
	mux.Handle("POST /auth/mfa/disable", auth(http.HandlerFunc(h.mfaDisable)))
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
	outcome, err := h.svc.Login(r.Context(), req.Email, req.Password, clientIP(r))
	if err != nil {
		var locked *ErrAccountLocked
		if errors.As(err, &locked) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusLocked)
			_ = json.NewEncoder(w).Encode(map[string]any{
				"error": map[string]any{
					"code":         "ACCOUNT_LOCKED",
					"message":      "too many failed attempts — account temporarily locked",
					"locked_until": locked.Until,
				},
			})
			return
		}
		if errors.Is(err, ErrInvalidCredentials) {
			apierrors.Write(w, http.StatusUnauthorized, "UNAUTHORIZED", "invalid credentials", httpx.CorrelationID(r.Context()))
			return
		}
		apierrors.Write(w, http.StatusInternalServerError, "INTERNAL", err.Error(), httpx.CorrelationID(r.Context()))
		return
	}
	httpx.JSON(w, http.StatusOK, outcome)
}

func (h *Handler) signup(w http.ResponseWriter, r *http.Request) {
	if !h.signupEnabled {
		apierrors.Write(w, http.StatusForbidden, "FORBIDDEN", "self-serve signup is disabled on this instance", httpx.CorrelationID(r.Context()))
		return
	}
	var req struct {
		CompanyName string `json:"company_name"`
		OwnerName   string `json:"owner_name"`
		Email       string `json:"email"`
		Password    string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		apierrors.Write(w, http.StatusBadRequest, "BAD_REQUEST", "invalid json", httpx.CorrelationID(r.Context()))
		return
	}
	outcome, err := h.svc.Signup(r.Context(), SignupInput{
		CompanyName: req.CompanyName, OwnerName: req.OwnerName, Email: req.Email, Password: req.Password,
	})
	if err != nil {
		writeIdentityErr(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusCreated, outcome)
}

func (h *Handler) mfaVerify(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ChallengeToken string `json:"mfa_challenge"`
		Code           string `json:"code"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		apierrors.Write(w, http.StatusBadRequest, "BAD_REQUEST", "invalid json", httpx.CorrelationID(r.Context()))
		return
	}
	pair, err := h.svc.VerifyMFAChallenge(r.Context(), req.ChallengeToken, req.Code, clientIP(r))
	if err != nil {
		if errors.Is(err, ErrMFAInvalidChallenge) || errors.Is(err, ErrMFAInvalidCode) {
			apierrors.Write(w, http.StatusUnauthorized, "UNAUTHORIZED", err.Error(), httpx.CorrelationID(r.Context()))
			return
		}
		apierrors.Write(w, http.StatusInternalServerError, "INTERNAL", err.Error(), httpx.CorrelationID(r.Context()))
		return
	}
	httpx.JSON(w, http.StatusOK, pair)
}

func (h *Handler) mfaStatus(w http.ResponseWriter, r *http.Request) {
	claims, ok := authz.FromContext(r.Context())
	if !ok {
		apierrors.Write(w, http.StatusUnauthorized, "UNAUTHORIZED", "missing claims", httpx.CorrelationID(r.Context()))
		return
	}
	status, err := h.svc.GetMFAStatus(r.Context(), claims.UserID)
	if err != nil {
		apierrors.Write(w, http.StatusInternalServerError, "INTERNAL", err.Error(), httpx.CorrelationID(r.Context()))
		return
	}
	httpx.JSON(w, http.StatusOK, status)
}

func (h *Handler) mfaSetup(w http.ResponseWriter, r *http.Request) {
	claims, ok := authz.FromContext(r.Context())
	if !ok {
		apierrors.Write(w, http.StatusUnauthorized, "UNAUTHORIZED", "missing claims", httpx.CorrelationID(r.Context()))
		return
	}
	result, err := h.svc.SetupMFA(r.Context(), claims.UserID, claims.Email)
	if err != nil {
		if errors.Is(err, ErrMFAAlreadyEnabled) {
			apierrors.Write(w, http.StatusConflict, "CONFLICT", err.Error(), httpx.CorrelationID(r.Context()))
			return
		}
		apierrors.Write(w, http.StatusInternalServerError, "INTERNAL", err.Error(), httpx.CorrelationID(r.Context()))
		return
	}
	httpx.JSON(w, http.StatusOK, result)
}

func (h *Handler) mfaEnable(w http.ResponseWriter, r *http.Request) {
	claims, ok := authz.FromContext(r.Context())
	if !ok {
		apierrors.Write(w, http.StatusUnauthorized, "UNAUTHORIZED", "missing claims", httpx.CorrelationID(r.Context()))
		return
	}
	var req struct {
		Code string `json:"code"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		apierrors.Write(w, http.StatusBadRequest, "BAD_REQUEST", "invalid json", httpx.CorrelationID(r.Context()))
		return
	}
	result, err := h.svc.EnableMFA(r.Context(), claims.UserID, claims.TenantID, req.Code)
	if err != nil {
		switch {
		case errors.Is(err, ErrMFAInvalidCode):
			apierrors.Write(w, http.StatusUnauthorized, "UNAUTHORIZED", err.Error(), httpx.CorrelationID(r.Context()))
		case errors.Is(err, ErrMFANotSetup), errors.Is(err, ErrMFAAlreadyEnabled):
			apierrors.Write(w, http.StatusConflict, "CONFLICT", err.Error(), httpx.CorrelationID(r.Context()))
		default:
			apierrors.Write(w, http.StatusInternalServerError, "INTERNAL", err.Error(), httpx.CorrelationID(r.Context()))
		}
		return
	}
	httpx.JSON(w, http.StatusOK, result)
}

func (h *Handler) mfaDisable(w http.ResponseWriter, r *http.Request) {
	claims, ok := authz.FromContext(r.Context())
	if !ok {
		apierrors.Write(w, http.StatusUnauthorized, "UNAUTHORIZED", "missing claims", httpx.CorrelationID(r.Context()))
		return
	}
	var req struct {
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		apierrors.Write(w, http.StatusBadRequest, "BAD_REQUEST", "invalid json", httpx.CorrelationID(r.Context()))
		return
	}
	if err := h.svc.DisableMFA(r.Context(), claims.UserID, claims.TenantID, req.Password); err != nil {
		if errors.Is(err, ErrInvalidCredentials) {
			apierrors.Write(w, http.StatusUnauthorized, "UNAUTHORIZED", "invalid password", httpx.CorrelationID(r.Context()))
			return
		}
		apierrors.Write(w, http.StatusInternalServerError, "INTERNAL", err.Error(), httpx.CorrelationID(r.Context()))
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) forgotPassword(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Email string `json:"email"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		apierrors.Write(w, http.StatusBadRequest, "BAD_REQUEST", "invalid json", httpx.CorrelationID(r.Context()))
		return
	}
	reset, err := h.svc.RequestPasswordReset(r.Context(), req.Email)
	if err != nil {
		apierrors.Write(w, http.StatusInternalServerError, "INTERNAL", err.Error(), httpx.CorrelationID(r.Context()))
		return
	}
	if reset != nil && h.onPasswordResetRequested != nil {
		h.onPasswordResetRequested(reset.Email, reset.DisplayName, reset.Token)
	}
	// Always respond the same way, whether or not the email exists, to avoid account enumeration.
	httpx.JSON(w, http.StatusOK, map[string]any{"message": "if that email exists, a reset link has been sent"})
}

func (h *Handler) resetPassword(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Token    string `json:"token"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		apierrors.Write(w, http.StatusBadRequest, "BAD_REQUEST", "invalid json", httpx.CorrelationID(r.Context()))
		return
	}
	if err := h.svc.ResetPassword(r.Context(), req.Token, req.Password); err != nil {
		switch {
		case errors.Is(err, ErrInvalidResetToken):
			apierrors.Write(w, http.StatusBadRequest, "BAD_REQUEST", "invalid or expired reset link", httpx.CorrelationID(r.Context()))
		case errors.Is(err, ErrInvalidInput):
			apierrors.Write(w, http.StatusBadRequest, "BAD_REQUEST", "password must be at least 8 characters", httpx.CorrelationID(r.Context()))
		default:
			apierrors.Write(w, http.StatusInternalServerError, "INTERNAL", err.Error(), httpx.CorrelationID(r.Context()))
		}
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"message": "password updated"})
}

// clientIP extracts the caller's address, preferring a proxy-set header (Caddy
// is the front door in production) and falling back to the raw remote addr.
func clientIP(r *http.Request) string {
	if fwd := r.Header.Get("X-Forwarded-For"); fwd != "" {
		if idx := strings.IndexByte(fwd, ','); idx != -1 {
			return strings.TrimSpace(fwd[:idx])
		}
		return strings.TrimSpace(fwd)
	}
	if ip := r.Header.Get("X-Real-IP"); ip != "" {
		return ip
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
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
		Name     string  `json:"name"`
		Code     string  `json:"code"`
		Location string  `json:"location"`
		Phone    *string `json:"phone"`
		Hours    *string `json:"hours"`
		MapURL   *string `json:"map_url"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		apierrors.Write(w, http.StatusBadRequest, "BAD_REQUEST", "invalid json", httpx.CorrelationID(r.Context()))
		return
	}
	branch, err := h.svc.CreateBranch(r.Context(), claims.TenantID, req.Name, req.Code, req.Location,
		BranchContactInput{Phone: req.Phone, Hours: req.Hours, MapURL: req.MapURL})
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
		Name     *string `json:"name"`
		Code     *string `json:"code"`
		Location *string `json:"location"`
		Phone    *string `json:"phone"`
		Hours    *string `json:"hours"`
		MapURL   *string `json:"map_url"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		apierrors.Write(w, http.StatusBadRequest, "BAD_REQUEST", "invalid json", httpx.CorrelationID(r.Context()))
		return
	}
	branch, err := h.svc.UpdateBranch(r.Context(), claims.TenantID, id, req.Name, req.Code, req.Location,
		BranchContactInput{Phone: req.Phone, Hours: req.Hours, MapURL: req.MapURL})
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
		CurrencyCode *string `json:"currency_code"`
		Locale       *string `json:"locale"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		apierrors.Write(w, http.StatusBadRequest, "BAD_REQUEST", "invalid json", httpx.CorrelationID(r.Context()))
		return
	}
	profile, err := h.svc.UpsertShopProfile(r.Context(), claims.TenantID, UpsertShopProfileInput{
		LegalName: req.LegalName, TIN: req.TIN, AddressLine1: req.AddressLine1,
		AddressLine2: req.AddressLine2, City: req.City, Country: req.Country,
		VATRateBPS: req.VATRateBPS, VATInclusive: req.VATInclusive,
		CurrencyCode: req.CurrencyCode, Locale: req.Locale,
	})
	if err != nil {
		apierrors.Write(w, http.StatusBadRequest, "BAD_REQUEST", err.Error(), httpx.CorrelationID(r.Context()))
		return
	}
	httpx.JSON(w, http.StatusOK, profile)
}
