package repair

import (
	"context"
	"fmt"

	"github.com/google/uuid"
)

// NotifyAdapter implements notify.RepairLookup for event-driven notifications.
type NotifyAdapter struct {
	Svc *Service
}

func (a NotifyAdapter) RepairNotifyContext(ctx context.Context, tenantID, repairID uuid.UUID) (jobCode, phone, shopName string, err error) {
	job, err := a.Svc.GetRepair(ctx, tenantID, repairID)
	if err != nil {
		return "", "", "", err
	}
	jobCode = job.JobCode
	if jobCode == "" {
		jobCode = job.ID.String()[:8]
	}
	if job.Customer != nil && job.Customer.Phone != nil {
		phone = *job.Customer.Phone
	}
	var name string
	if qErr := a.Svc.pool.QueryRow(ctx, `SELECT name FROM identity.tenants WHERE id = $1`, tenantID).Scan(&name); qErr == nil {
		shopName = name
	}
	if shopName == "" {
		shopName = "TechLane"
	}
	return jobCode, phone, shopName, nil
}

func (a NotifyAdapter) PaymentNotifyContext(ctx context.Context, tenantID, paymentID uuid.UUID) (repairID *uuid.UUID, jobCode, phone, amount, currency string, err error) {
	var payableType string
	var payableID uuid.UUID
	var amt float64
	err = a.Svc.pool.QueryRow(ctx, `
		SELECT a.payable_type, a.payable_id, p.amount::float8, p.currency
		FROM payments.payment_allocations a
		JOIN payments.payments p ON p.id = a.payment_id
		WHERE p.tenant_id = $1 AND p.id = $2 LIMIT 1`, tenantID, paymentID).
		Scan(&payableType, &payableID, &amt, &currency)
	if err != nil {
		return nil, "", "", "", "", err
	}
	amount = fmt.Sprintf("%.0f", amt)
	if payableType != "repair" {
		return nil, "", "", amount, currency, nil
	}
	repairID = &payableID
	jc, ph, _, lErr := a.RepairNotifyContext(ctx, tenantID, payableID)
	if lErr != nil {
		return repairID, "", "", amount, currency, lErr
	}
	return repairID, jc, ph, amount, currency, nil
}
