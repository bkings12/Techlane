package payments

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
)

type stubCustomerRepair struct {
	tenantID   uuid.UUID
	customerID uuid.UUID
	repairID   uuid.UUID
	owns       bool
	balance    float64
	phone      string
}

func (s stubCustomerRepair) DefaultTenantID(context.Context) (uuid.UUID, error) {
	return s.tenantID, nil
}

func (s stubCustomerRepair) AuthenticateCustomer(context.Context, uuid.UUID, string) (uuid.UUID, *string, error) {
	p := s.phone
	return s.customerID, &p, nil
}

func (s stubCustomerRepair) AssertCustomerOwnsRepair(_ context.Context, _, customerID, repairID uuid.UUID) error {
	if !s.owns || customerID != s.customerID || repairID != s.repairID {
		return errors.New("repair not found")
	}
	return nil
}

func (s stubCustomerRepair) RepairPaymentContext(context.Context, uuid.UUID, uuid.UUID) (uuid.UUID, float64, string, string, error) {
	return uuid.New(), s.balance, s.phone, "JOB-101", nil
}

func TestRepairCustomerAdapterOwnership(t *testing.T) {
	tenant := uuid.New()
	customer := uuid.New()
	repairID := uuid.New()
	gw := stubCustomerRepair{
		tenantID: tenant, customerID: customer, repairID: repairID,
		owns: true, balance: 1500, phone: "254712345678",
	}
	if err := gw.AssertCustomerOwnsRepair(context.Background(), tenant, customer, repairID); err != nil {
		t.Fatal(err)
	}
	if err := gw.AssertCustomerOwnsRepair(context.Background(), tenant, uuid.New(), repairID); err == nil {
		t.Fatal("expected ownership failure")
	}
	_, bal, _, _, err := gw.RepairPaymentContext(context.Background(), tenant, repairID)
	if err != nil || bal != 1500 {
		t.Fatalf("balance=%v err=%v", bal, err)
	}
}
