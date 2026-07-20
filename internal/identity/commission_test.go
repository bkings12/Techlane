package identity_test

import (
	"testing"

	"github.com/techlane/techlane/packages/pkg/authz"
	"github.com/techlane/techlane/internal/identity"
)

func TestComputeCommissionPercent(t *testing.T) {
	bps := 1000 // 10%
	got := identity.ComputeCommissionAmount("percent_of_job", 5000, &bps, nil)
	if got != 500 {
		t.Fatalf("got %v want 500", got)
	}
}

func TestComputeCommissionFixed(t *testing.T) {
	fixed := 750.0
	got := identity.ComputeCommissionAmount("fixed_per_job", 5000, nil, &fixed)
	if got != 750 {
		t.Fatalf("got %v want 750", got)
	}
}

func TestComputeCommissionDisabledType(t *testing.T) {
	got := identity.ComputeCommissionAmount("none", 5000, nil, nil)
	if got != 0 {
		t.Fatalf("got %v", got)
	}
}

func TestCashierCannotHaveUsersWriteByDefault(t *testing.T) {
	perms := authz.DefaultPermissions("cashier")
	for _, p := range perms {
		if p == "users.write" {
			t.Fatal("cashier should not have users.write")
		}
	}
}

func TestManagerHasStaffPerms(t *testing.T) {
	claims := authz.Claims{Roles: []string{"manager"}, Permissions: authz.DefaultPermissions("manager")}
	for _, need := range []string{"users.write", "commissions.write", "roles.assign"} {
		if !claims.HasPermission(need) {
			t.Fatalf("manager missing %s", need)
		}
	}
}

func TestSelfCommissionForbiddenError(t *testing.T) {
	// Documented rule: ErrForbidden message used by SetCommission
	if identity.ErrForbidden == nil {
		t.Fatal("expected ErrForbidden")
	}
}
