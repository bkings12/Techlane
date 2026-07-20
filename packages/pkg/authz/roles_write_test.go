package authz_test

import (
	"testing"

	"github.com/techlane/techlane/packages/pkg/authz"
)

func TestSystemPermissionCatalogNonEmpty(t *testing.T) {
	cats := authz.SystemPermissionCatalog()
	if len(cats) < 20 {
		t.Fatalf("expected rich catalog, got %d", len(cats))
	}
	if !authz.IsKnownPermission("users.write") {
		t.Fatal("users.write should be known")
	}
	if !authz.IsKnownPermission("*") {
		t.Fatal("* should be known")
	}
}

func TestManagerHasRolesWrite(t *testing.T) {
	claims := authz.Claims{Roles: []string{"manager"}, Permissions: authz.DefaultPermissions("manager")}
	if !claims.HasPermission("roles.write") {
		t.Fatal("manager should create custom roles")
	}
}
