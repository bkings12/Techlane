package repair

import "testing"

func TestValidateStatusTransition(t *testing.T) {
	cases := []struct {
		from    string
		to      string
		wantErr bool
	}{
		{StatusIntake, StatusDiagnosed, false},
		{StatusIntake, StatusCompleted, true},
		{StatusDiagnosed, StatusInProgress, false},
		{StatusInProgress, StatusCompleted, false},
		{StatusCompleted, StatusCollected, false},
		{StatusCollected, StatusIntake, true},
		{StatusWaitingParts, StatusInProgress, false},
	}
	for _, tc := range cases {
		err := ValidateStatusTransition(tc.from, tc.to)
		if tc.wantErr && err == nil {
			t.Errorf("expected error for %s -> %s", tc.from, tc.to)
		}
		if !tc.wantErr && err != nil {
			t.Errorf("unexpected error for %s -> %s: %v", tc.from, tc.to, err)
		}
	}
}

func TestCanDelete(t *testing.T) {
	if CanDelete(StatusCompleted) {
		t.Fatal("completed repairs must not be deletable")
	}
	if !CanDelete(StatusIntake) {
		t.Fatal("intake repairs should be deletable")
	}
}
