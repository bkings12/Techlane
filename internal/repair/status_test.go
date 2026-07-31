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
		{StatusInProgress, StatusCompleted, true}, // must pass QC first
		{StatusInProgress, StatusReadyPickup, false},
		{StatusReadyPickup, StatusCompleted, false},
		{StatusReadyPickup, StatusInProgress, false}, // QC failed — back to bench
		{StatusCompleted, StatusCollected, false},
		{StatusCollected, StatusIntake, true},
		{StatusWaitingParts, StatusInProgress, false},
		{StatusWaitingParts, StatusDiagnosed, true}, // no free bounce back
		{StatusCompleted, StatusWaitingParts, true},
		{StatusCompleted, StatusInProgress, true},
		// A job can be closed out from any active stage...
		{StatusIntake, StatusCancelled, false},
		{StatusDiagnosed, StatusUnrepairable, false},
		{StatusWaitingParts, StatusCancelled, false},
		{StatusInProgress, StatusUnrepairable, false},
		// ...but the device still has to be handed back, and nothing else follows.
		{StatusCancelled, StatusCollected, false},
		{StatusUnrepairable, StatusCollected, false},
		{StatusCancelled, StatusInProgress, true},
		{StatusUnrepairable, StatusCompleted, true},
		{StatusCompleted, StatusCancelled, true},
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
	if CanDelete(StatusCancelled) || CanDelete(StatusUnrepairable) {
		t.Fatal("closed jobs are a record of a lost job — they must not be deletable")
	}
}

func TestCanEditDetails(t *testing.T) {
	if !CanEditDetails(StatusIntake) || !CanEditDetails(StatusCompleted) {
		t.Fatal("pre-handover jobs should allow detail corrections")
	}
	if CanEditDetails(StatusCollected) {
		t.Fatal("collected jobs must not allow detail rewrites")
	}
}

func TestIsOpen(t *testing.T) {
	open := []string{StatusIntake, StatusDiagnosed, StatusWaitingParts, StatusInProgress, StatusReadyPickup, StatusCompleted}
	for _, s := range open {
		if !IsOpen(s) {
			t.Errorf("%s should count as an open job", s)
		}
	}
	for _, s := range []string{StatusCollected, StatusCancelled, StatusUnrepairable} {
		if IsOpen(s) {
			t.Errorf("%s should not count as an open job", s)
		}
	}
}

func TestCanRequestParts(t *testing.T) {
	for _, s := range []string{StatusIntake, StatusDiagnosed, StatusWaitingParts, StatusInProgress} {
		if !CanRequestParts(s) {
			t.Errorf("%s should allow part requests", s)
		}
	}
	for _, s := range []string{StatusReadyPickup, StatusCompleted, StatusCollected, StatusCancelled, StatusUnrepairable} {
		if CanRequestParts(s) {
			t.Errorf("%s must not allow part requests", s)
		}
	}
}

func TestRequiresClearPartsAndEstimates(t *testing.T) {
	if !RequiresClearPartsAndEstimates(StatusReadyPickup) || !RequiresClearPartsAndEstimates(StatusCompleted) {
		t.Fatal("ready and completed must require clear parts/estimates")
	}
	if RequiresClearPartsAndEstimates(StatusInProgress) {
		t.Fatal("in_progress should not require the finish gate")
	}
	if !LeavingWaitingPartsForBench(StatusWaitingParts, StatusInProgress) {
		t.Fatal("waiting_parts → in_progress must check outstanding parts")
	}
}

func TestValidateClosureReason(t *testing.T) {
	if err := ValidateClosureReason(StatusCancelled, "customer_declined_quote"); err != nil {
		t.Errorf("valid cancellation reason rejected: %v", err)
	}
	if err := ValidateClosureReason(StatusUnrepairable, "beyond_economical_repair"); err != nil {
		t.Errorf("valid write-off reason rejected: %v", err)
	}
	if err := ValidateClosureReason(StatusCancelled, ""); err == nil {
		t.Error("closing a job without a reason must be rejected")
	}
	if err := ValidateClosureReason(StatusCancelled, "  "); err == nil {
		t.Error("whitespace is not a reason")
	}
	// A write-off reason is not a cancellation reason and vice versa.
	if err := ValidateClosureReason(StatusCancelled, "beyond_economical_repair"); err == nil {
		t.Error("reason codes must be scoped to their closure status")
	}
	// Non-closure statuses carry no reason requirement.
	if err := ValidateClosureReason(StatusCompleted, ""); err != nil {
		t.Errorf("non-closure status should not require a reason: %v", err)
	}
}
