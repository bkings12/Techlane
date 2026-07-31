package repair

import "testing"

func TestComputeMargin(t *testing.T) {
	margin, pct := ComputeMargin(5000, 2000)
	if margin != 3000 {
		t.Errorf("expected margin 3000, got %v", margin)
	}
	if pct == nil || *pct != 60 {
		t.Errorf("expected 60%%, got %v", pct)
	}

	// A job that cost more than it charged is a loss, and says so.
	margin, pct = ComputeMargin(1000, 2500)
	if margin != -1500 {
		t.Errorf("expected margin -1500, got %v", margin)
	}
	if pct == nil || *pct != -150 {
		t.Errorf("expected -150%%, got %v", pct)
	}

	// No revenue means no percentage — not an infinite one.
	margin, pct = ComputeMargin(0, 800)
	if margin != -800 {
		t.Errorf("expected margin -800, got %v", margin)
	}
	if pct != nil {
		t.Errorf("a job with no charge must not report a margin percentage, got %v", *pct)
	}

	// A warranty rework: no charge, no cost, no percentage.
	if margin, pct := ComputeMargin(0, 0); margin != 0 || pct != nil {
		t.Errorf("expected zero margin and no percentage, got %v / %v", margin, pct)
	}
}
