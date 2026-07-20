package payments

import "testing"

func TestC2BAmountTolerance(t *testing.T) {
	cases := []struct {
		expected, got float64
		mismatch      bool
	}{
		{100, 100, false},
		{100, 100.4, false},
		{100, 100.6, true},
		{500, 499, true},
	}
	for _, c := range cases {
		diff := c.expected - c.got
		if diff < 0 {
			diff = -diff
		}
		got := diff > 0.5
		if got != c.mismatch {
			t.Fatalf("expected=%v got=%v want mismatch=%v", c.expected, c.got, c.mismatch)
		}
	}
}
