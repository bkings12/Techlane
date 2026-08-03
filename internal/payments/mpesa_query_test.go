package payments

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/techlane/techlane/packages/pkg/authz"
)

func TestValidateCallbackURL(t *testing.T) {
	if err := validateCallbackURL("https://api.example.com/hook", "production"); err == nil {
		t.Fatal("example.com should fail")
	}
	if err := validateCallbackURL("http://api.shop.test/hook", "production"); err == nil {
		t.Fatal("http should fail in production")
	}
	if err := validateCallbackURL("https://api.shop.test/api/v1/webhooks/mpesa/stk", "production"); err != nil {
		t.Fatal(err)
	}
}

func TestReconcileHandlerRejectsWithoutPermission(t *testing.T) {
	h := &Handler{svc: &Service{}}
	req := httptest.NewRequest(http.MethodPost, "/payments/"+uuid.NewString()+"/mpesa/reconcile", strings.NewReader(`{"provider_ref":"FAKE"}`))
	req.SetPathValue("id", uuid.NewString())
	ctx := authz.WithClaims(context.Background(), &authz.Claims{
		UserID: uuid.New(), TenantID: uuid.New(), Roles: []string{"technician"}, Permissions: []string{"payments.read"},
	})
	rr := httptest.NewRecorder()
	h.reconcileMpesa(rr, req.WithContext(ctx))
	if rr.Code != http.StatusForbidden {
		t.Fatalf("status=%d body=%s", rr.Code, rr.Body.String())
	}
}

func TestWebhookTokenGate(t *testing.T) {
	t.Setenv("MPESA_WEBHOOK_TOKEN", "secret")
	req := httptest.NewRequest(http.MethodPost, "/webhooks/mpesa/stk", nil)
	if webhookTokenOK(req) {
		t.Fatal("expected reject without token")
	}
	req = httptest.NewRequest(http.MethodPost, "/webhooks/mpesa/stk?token=secret", nil)
	if !webhookTokenOK(req) {
		t.Fatal("expected allow with token")
	}
}

func TestSplitTenderSumValidation(t *testing.T) {
	s := &Service{}
	_, err := s.CreateSplitPayment(context.Background(), SplitPaymentInput{
		BalanceDue: 100,
		Tenders:    []TenderLine{{Method: "cash", Amount: 60}, {Method: "cash", Amount: 50}},
	})
	if err == nil || !strings.Contains(err.Error(), "exceeds") {
		t.Fatalf("expected exceed error, got %v", err)
	}
}

func TestStkQueryStillProcessing(t *testing.T) {
	cases := []struct {
		code, desc string
		want       bool
	}{
		{"4999", "The transaction is still under processing", true},
		{"500.001.1001", "The transaction is being processed", true},
		{"0", "Success", false},
		{"1032", "Request cancelled by user", false},
		{"1037", "DS timeout", false},
	}
	for _, tc := range cases {
		if got := stkQueryStillProcessing(tc.code, tc.desc); got != tc.want {
			t.Fatalf("code=%q desc=%q got=%v want=%v", tc.code, tc.desc, got, tc.want)
		}
	}
}
