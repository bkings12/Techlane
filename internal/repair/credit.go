package repair

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// SendCreditReminders sends one reminder per credit agreement when its due date
// is three to five days away. Failed sends are not marked, so the next worker
// pass retries them after the provider recovers.
func (s *Service) SendCreditReminders(ctx context.Context, tenantID uuid.UUID) (int, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT j.id, j.job_code, j.credit_due_date, c.phone,
		       COALESCE(NULLIF(t.name, ''), 'TechLane')
		FROM repair.repair_jobs j
		JOIN repair.customers c ON c.id = j.customer_id AND c.tenant_id = j.tenant_id
		LEFT JOIN identity.tenants t ON t.id = j.tenant_id
		WHERE j.tenant_id = $1
		  AND j.customer_credit = true
		  AND j.status = 'collected'
		  AND j.credit_reminder_sent_at IS NULL
		  AND j.credit_due_date BETWEEN current_date + 3 AND current_date + 5
		  AND NULLIF(c.phone, '') IS NOT NULL`, tenantID)
	if err != nil {
		return 0, err
	}
	defer rows.Close()

	type reminder struct {
		id      uuid.UUID
		jobCode string
		due     time.Time
		phone   string
		shop    string
	}
	var reminders []reminder
	for rows.Next() {
		var item reminder
		if err := rows.Scan(&item.id, &item.jobCode, &item.due, &item.phone, &item.shop); err != nil {
			return 0, err
		}
		reminders = append(reminders, item)
	}
	if err := rows.Err(); err != nil {
		return 0, err
	}

	sent := 0
	for _, item := range reminders {
		balance, err := s.outstandingRepairBalance(ctx, tenantID, item.id)
		if err != nil || balance <= 0.009 {
			continue
		}
		message := fmt.Sprintf(
			"%s: reminder that KES %.2f for %s is due on %s. Please pay by the agreed date. Thank you.",
			item.shop, balance, item.jobCode, item.due.Format("02 Jan 2006"),
		)
		if err := s.resolveSMSSender(ctx, tenantID).SendMessage(ctx, item.phone, message); err != nil {
			continue
		}
		if _, err := s.pool.Exec(ctx, `
			UPDATE repair.repair_jobs SET credit_reminder_sent_at = now()
			WHERE tenant_id = $1 AND id = $2 AND credit_reminder_sent_at IS NULL`,
			tenantID, item.id); err == nil {
			sent++
		}
	}
	return sent, nil
}
