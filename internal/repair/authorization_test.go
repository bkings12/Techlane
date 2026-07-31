package repair

import "testing"

func f(v float64) *float64 { return &v }

func TestExceedsAuthorizedAmount(t *testing.T) {
	cases := []struct {
		name       string
		authorized *float64
		final      float64
		want       bool
	}{
		{"no authorized figure means nothing to compare", nil, 9999, false},
		{"charging exactly what was agreed", f(5000), 5000, false},
		{"charging less than agreed is always fine", f(5000), 3500, false},
		{"rounding up by a shilling is absorbed", f(5000), 5001, false},
		{"charging materially more needs a reason", f(5000), 5500, true},
		{"free job that becomes chargeable needs a reason", f(0), 1500, true},
	}
	for _, tc := range cases {
		if got := exceedsAuthorizedAmount(tc.authorized, tc.final); got != tc.want {
			t.Errorf("%s: got %v want %v", tc.name, got, tc.want)
		}
	}
}

func TestLaborVariance(t *testing.T) {
	if v := LaborVariance(nil, 500); v != 0 {
		t.Errorf("unauthorized job should report no variance, got %v", v)
	}
	if v := LaborVariance(f(5000), 6500); v != 1500 {
		t.Errorf("expected overrun of 1500, got %v", v)
	}
	if v := LaborVariance(f(5000), 4000); v != -1000 {
		t.Errorf("expected discount of -1000, got %v", v)
	}
	if v := LaborVariance(f(5000), 5000.001); v != 0 {
		t.Errorf("sub-cent drift should read as zero, got %v", v)
	}
}
