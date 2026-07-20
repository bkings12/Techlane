package authz_test

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/techlane/techlane/packages/pkg/authz"
)

func TestTokenRoundTrip(t *testing.T) {
	secret := "test-secret-at-least-32-characters!!"
	uid := uuid.New()
	tid := uuid.New()
	token, err := authz.IssueAccessToken(secret, authz.Claims{
		UserID:      uid,
		TenantID:    tid,
		Email:       "owner@techlane.local",
		Roles:       []string{"owner"},
		Permissions: []string{"*"},
		BranchIDs:   []string{},
	}, 15*time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	claims, err := authz.ParseAccessToken(secret, token)
	if err != nil {
		t.Fatal(err)
	}
	if claims.UserID != uid || claims.TenantID != tid {
		t.Fatalf("claims mismatch")
	}
	if !claims.HasPermission("anything") {
		t.Fatal("owner should have all perms")
	}
}
