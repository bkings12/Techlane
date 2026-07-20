package inventory

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/techlane/techlane/packages/pkg/authz"
)

func TestContactMayAccessRequestIsolation(t *testing.T) {
	a := uuid.New()
	b := uuid.New()
	if ContactMayAccessRequest(nil, a) {
		t.Fatal("unassigned request must be invisible")
	}
	if ContactMayAccessRequest(&b, a) {
		t.Fatal("supplier A must not see supplier B request")
	}
	if !ContactMayAccessRequest(&a, a) {
		t.Fatal("assigned supplier must see own request")
	}
}

func TestValidateInviteAccept(t *testing.T) {
	now := time.Now().UTC()
	exp := now.Add(time.Hour)
	if err := ValidateInviteAccept("invited", &exp, "password", now); err != nil {
		t.Fatalf("valid invite: %v", err)
	}
	if err := ValidateInviteAccept("active", &exp, "password", now); err == nil {
		t.Fatal("expected reject for active status")
	}
	past := now.Add(-time.Minute)
	if err := ValidateInviteAccept("invited", &past, "password", now); err == nil {
		t.Fatal("expected reject for expired invite")
	}
	if err := ValidateInviteAccept("invited", &exp, "short", now); err == nil {
		t.Fatal("expected reject for short password")
	}
}

func TestResolveIssueUnitCostQuoteFlow(t *testing.T) {
	acceptedID := uuid.New()
	pendingID := uuid.New()
	accepted := &PartRequestQuote{ID: acceptedID, UnitCost: 1200, Status: "accepted"}
	pending := &PartRequestQuote{ID: pendingID, UnitCost: 900, Status: "pending"}

	cost, autoID, err := ResolveIssueUnitCost(accepted, pending)
	if err != nil || cost != 1200 || autoID != nil {
		t.Fatalf("accepted quote preferred: cost=%v auto=%v err=%v", cost, autoID, err)
	}

	cost, autoID, err = ResolveIssueUnitCost(nil, pending)
	if err != nil || cost != 900 || autoID == nil || *autoID != pendingID {
		t.Fatalf("auto-accept pending: cost=%v auto=%v err=%v", cost, autoID, err)
	}

	if _, _, err := ResolveIssueUnitCost(nil, nil); err == nil {
		t.Fatal("expected error with no quotes")
	}
}

func TestQRPayloadForIssue(t *testing.T) {
	id := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	got := QRPayloadForIssue(id, "ABCD2345")
	want := "techlane://auth/11111111-1111-1111-1111-111111111111/ABCD2345"
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestHashOpaqueTokenStable(t *testing.T) {
	a := hashOpaqueToken("secret-token")
	b := hashOpaqueToken("secret-token")
	if a != b || a == "" || a == "secret-token" {
		t.Fatalf("unexpected hash %q", a)
	}
	if hashOpaqueToken("other") == a {
		t.Fatal("different tokens must hash differently")
	}
}

func TestManagerHasSupplierWriteAndReconcile(t *testing.T) {
	claims := authz.Claims{Roles: []string{"manager"}, Permissions: authz.DefaultPermissions("manager")}
	for _, need := range []string{"suppliers.read", "suppliers.write", "supplier_credit.reconcile", "parts.approve"} {
		if !claims.HasPermission(need) {
			t.Fatalf("manager missing %s", need)
		}
	}
}
