package repair

import "testing"

func TestApplyJobMoney(t *testing.T) {
	pending := 1800.0
	approved := 2500.0
	auth := 2000.0

	t.Run("approved estimate wins for due", func(t *testing.T) {
		j := RepairJob{LaborAmount: 1000, ApprovedEstimateTotal: &approved, SaleLinesTotal: 300, PaidTotal: 500}
		applyJobMoney(&j)
		if j.AmountDue != 2800 {
			t.Fatalf("amount_due=%v want 2800", j.AmountDue)
		}
		if j.BalanceDue != 2300 {
			t.Fatalf("balance_due=%v want 2300", j.BalanceDue)
		}
		if j.QuotedValue != 2800 {
			t.Fatalf("quoted_value=%v want 2800", j.QuotedValue)
		}
	})

	t.Run("pending estimate counts as pipeline worth", func(t *testing.T) {
		j := RepairJob{PendingEstimateTotal: &pending, SaleLinesTotal: 200}
		applyJobMoney(&j)
		if j.AmountDue != 200 {
			t.Fatalf("amount_due=%v want 200 (sale lines only until estimate approved)", j.AmountDue)
		}
		if j.QuotedValue != 2000 {
			t.Fatalf("quoted_value=%v want 2000", j.QuotedValue)
		}
	})

	t.Run("authorized fills quoted when labor empty", func(t *testing.T) {
		j := RepairJob{AuthorizedAmount: &auth}
		applyJobMoney(&j)
		if j.QuotedValue != 2000 {
			t.Fatalf("quoted_value=%v want 2000", j.QuotedValue)
		}
		if j.AmountDue != 0 {
			t.Fatalf("amount_due=%v want 0 until labor/estimate lands", j.AmountDue)
		}
	})
}
