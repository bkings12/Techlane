package authz

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

type ctxKey string

const ClaimsKey ctxKey = "authz_claims"

type Claims struct {
	UserID      uuid.UUID `json:"uid"`
	TenantID    uuid.UUID `json:"tid"`
	Email       string    `json:"email"`
	Roles       []string  `json:"roles"`
	Permissions []string  `json:"perms"`
	BranchIDs   []string  `json:"branches"`
	DeviceID    string    `json:"did,omitempty"`
	jwt.RegisteredClaims
}

func (c Claims) HasPermission(p string) bool {
	for _, role := range c.Roles {
		if role == "owner" {
			return true
		}
	}
	for _, x := range c.Permissions {
		if x == p || x == "*" {
			return true
		}
	}
	return false
}

func (c Claims) InBranch(branchID uuid.UUID) bool {
	if len(c.BranchIDs) == 0 {
		return true
	}
	id := branchID.String()
	for _, b := range c.BranchIDs {
		if b == id {
			return true
		}
	}
	for _, role := range c.Roles {
		if role == "owner" {
			return true
		}
	}
	return false
}

func IssueAccessToken(secret string, claims Claims, ttl time.Duration) (string, error) {
	now := time.Now()
	claims.RegisteredClaims = jwt.RegisteredClaims{
		Subject:   claims.UserID.String(),
		IssuedAt:  jwt.NewNumericDate(now),
		ExpiresAt: jwt.NewNumericDate(now.Add(ttl)),
		ID:        uuid.NewString(),
	}
	t := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return t.SignedString([]byte(secret))
}

func ParseAccessToken(secret, token string) (*Claims, error) {
	parsed, err := jwt.ParseWithClaims(token, &Claims{}, func(t *jwt.Token) (any, error) {
		if t.Method != jwt.SigningMethodHS256 {
			return nil, errors.New("unexpected signing method")
		}
		return []byte(secret), nil
	})
	if err != nil {
		return nil, err
	}
	claims, ok := parsed.Claims.(*Claims)
	if !ok || !parsed.Valid {
		return nil, errors.New("invalid token")
	}
	return claims, nil
}

func FromContext(ctx context.Context) (*Claims, bool) {
	c, ok := ctx.Value(ClaimsKey).(*Claims)
	return c, ok
}

func WithClaims(ctx context.Context, c *Claims) context.Context {
	return context.WithValue(ctx, ClaimsKey, c)
}

func BearerToken(h string) string {
	if h == "" {
		return ""
	}
	parts := strings.SplitN(h, " ", 2)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "bearer") {
		return ""
	}
	return parts[1]
}

// DefaultPermissions returns MVP permission sets by role.
func DefaultPermissions(role string) []string {
	switch role {
	case "owner":
		return []string{"*"}
	case "manager":
		return []string{
			"users.read", "users.write", "roles.assign", "roles.write",
			"repairs.create", "repairs.assign", "repairs.status.update", "repairs.collect",
			"parts.approve", "parts.collect", "parts.request", "cash.handover.confirm", "refunds.create", "refunds.approve",
			"audit.read", "risk.read", "reports.read", "inventory.read", "inventory.adjust", "sales.create", "payments.initiate", "payments.read",
			"commissions.read", "commissions.write", "commissions.approve", "branches.read",
			"suppliers.read", "suppliers.write", "supplier_credit.reconcile",
		}
	case "accountant":
		return []string{
			"payments.read", "cash.handover.confirm", "refunds.create", "refunds.approve",
			"audit.read", "risk.read", "reports.read",
			"commissions.read", "commissions.approve", "users.read",
			"suppliers.read", "supplier_credit.reconcile",
		}
	case "inventory":
		return []string{
			"inventory.read", "inventory.adjust", "parts.request", "parts.approve", "parts.collect",
			"suppliers.read", "suppliers.write", "supplier_credit.reconcile",
		}
	case "technician":
		return []string{"repairs.create", "repairs.status.update", "parts.request", "parts.collect", "customers.write", "devices.write", "repairs.read"}
	case "cashier":
		return []string{"repairs.create", "sales.create", "payments.initiate", "cash.receive", "cash.handover.request", "customers.write", "refunds.create"}
	case "branch_admin":
		return []string{"users.read", "users.write", "roles.assign", "roles.write", "branches.read"}
	default:
		return nil
	}
}

// AllRoles returns the system role presets.
func AllRoles() []string {
	return []string{"owner", "manager", "accountant", "inventory", "technician", "cashier", "branch_admin"}
}

// RoleCatalog returns roles with their default permissions for Settings UI.
func RoleCatalog() []map[string]any {
	out := make([]map[string]any, 0, len(AllRoles()))
	for _, role := range AllRoles() {
		out = append(out, map[string]any{
			"id":          role,
			"name":        role,
			"permissions": DefaultPermissions(role),
		})
	}
	return out
}

// ValidRole reports whether role is a known preset.
func ValidRole(role string) bool {
	for _, r := range AllRoles() {
		if r == role {
			return true
		}
	}
	return false
}
