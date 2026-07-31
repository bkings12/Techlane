package repair

import "testing"

func TestIsCollectable(t *testing.T) {
	// QC-passed, a finished repair, and a job we are giving up on — all end with
	// the customer taking the device away at the counter.
	for _, status := range []string{StatusReadyPickup, StatusCompleted, StatusCancelled, StatusUnrepairable} {
		if !isCollectable(status) {
			t.Errorf("%s should be collectable", status)
		}
	}
	// Anything still in the shop's hands is not.
	for _, status := range []string{StatusIntake, StatusDiagnosed, StatusWaitingParts, StatusInProgress} {
		if isCollectable(status) {
			t.Errorf("%s must not be collectable — the device is still being worked on", status)
		}
	}
	// Already gone.
	if isCollectable(StatusCollected) {
		t.Error("collected must not be collectable again")
	}
}

func TestHandoverNote(t *testing.T) {
	own := handoverNote(&Handover{
		CollectedByName: "Asha Mwangi", Relationship: "self", VerificationMethod: HandoverMethodOTP,
	})
	if own != "Handed over to Asha Mwangi — code confirmed on the owner's phone" {
		t.Errorf("unexpected note: %q", own)
	}

	// A third party collecting must be visible in the timeline, along with the fact
	// that nobody confirmed a code.
	proxy := handoverNote(&Handover{
		CollectedByName: "John Otieno", Relationship: "brother", VerificationMethod: HandoverMethodStaffVouched,
	})
	if proxy != "Handed over to John Otieno (brother) — released by staff without a code" {
		t.Errorf("unexpected note: %q", proxy)
	}
}
