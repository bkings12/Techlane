package repair

import "testing"

func TestFormatPickupPlace(t *testing.T) {
	cases := []struct {
		shop, loc, branch, want string
	}{
		{"TechLane", "Westlands", "Main", "TechLane & Westlands"},
		{"TechLane", "", "Main", "TechLane & Main"},
		{"TechLane", "", "the shop", "TechLane"},
		{"", "CBD", "Main", "the shop & CBD"},
		{"", "", "", "the shop"},
	}
	for _, tc := range cases {
		got := formatPickupPlace(tc.shop, tc.loc, tc.branch)
		if got != tc.want {
			t.Fatalf("formatPickupPlace(%q,%q,%q)=%q want %q", tc.shop, tc.loc, tc.branch, got, tc.want)
		}
	}
}
