package repair

import (
	"fmt"
	"strings"
)

const (
	StatusIntake       = "intake"
	StatusDiagnosed    = "diagnosed"
	StatusWaitingParts = "waiting_parts"
	StatusInProgress   = "in_progress"
	StatusReadyPickup  = "ready_for_pickup"
	StatusCompleted    = "completed"
	StatusCollected    = "collected"
	// Closure states: the job ended without a repair. The device still has to
	// go back to the customer, so both can still transition to "collected" —
	// custody of the device is tracked to the end either way.
	StatusCancelled    = "cancelled"
	StatusUnrepairable = "unrepairable"
)

// Bench flow is deliberately linear. Side doors (waiting_parts, back to repair
// after a failed QC) exist, but you cannot jump from repair straight to
// "completed" or bounce freely between completed and earlier stages.
var allowedTransitions = map[string][]string{
	StatusIntake:       {StatusDiagnosed, StatusWaitingParts, StatusInProgress, StatusCancelled, StatusUnrepairable},
	StatusDiagnosed:    {StatusWaitingParts, StatusInProgress, StatusCancelled, StatusUnrepairable},
	StatusWaitingParts: {StatusInProgress, StatusCancelled, StatusUnrepairable},
	StatusInProgress:   {StatusReadyPickup, StatusWaitingParts, StatusCancelled, StatusUnrepairable},
	StatusReadyPickup:  {StatusCompleted, StatusInProgress, StatusCancelled, StatusUnrepairable},
	StatusCompleted:    {StatusCollected},
	StatusCancelled:    {StatusCollected},
	StatusUnrepairable: {StatusCollected},
	StatusCollected:    {},
}

// closureReasons are the structured reason codes accepted when closing a job
// without a repair. Free text goes in the status event note; this code exists
// so "why do we lose jobs?" is a groupable report rather than a text search.
var closureReasons = map[string][]string{
	StatusCancelled: {
		"customer_declined_quote",
		"customer_withdrew",
		"no_response",
		"duplicate_job",
		"other",
	},
	StatusUnrepairable: {
		"beyond_economical_repair",
		"parts_unavailable",
		"severe_liquid_damage",
		"further_damage_found",
		"other",
	},
}

func ValidateStatusTransition(from, to string) error {
	if from == to {
		return nil
	}
	next, ok := allowedTransitions[from]
	if !ok {
		return fmt.Errorf("unknown status %q", from)
	}
	for _, s := range next {
		if s == to {
			return nil
		}
	}
	return fmt.Errorf("invalid transition from %q to %q", from, to)
}

func NextStatuses(from string) []string {
	return append([]string{}, allowedTransitions[from]...)
}

// IsClosure reports whether status ends the job without a completed repair.
func IsClosure(status string) bool {
	return status == StatusCancelled || status == StatusUnrepairable
}

// IsOpen reports whether the job is still active work: not handed back and
// not closed out. Queues, dashboards and aging all key off this.
func IsOpen(status string) bool {
	return status != StatusCollected && !IsClosure(status)
}

// ClosureReasonsFor returns the accepted reason codes for a closure status.
func ClosureReasonsFor(status string) []string {
	return append([]string{}, closureReasons[status]...)
}

// ValidateClosureReason checks that a closure status carries a known reason
// code. Closing a job is the one transition that destroys pipeline value, so
// it is never allowed to happen silently.
func ValidateClosureReason(status, reason string) error {
	allowed, ok := closureReasons[status]
	if !ok {
		return nil
	}
	reason = strings.TrimSpace(reason)
	if reason == "" {
		return fmt.Errorf("closure_reason is required when marking a job %s", status)
	}
	for _, r := range allowed {
		if r == reason {
			return nil
		}
	}
	return fmt.Errorf("unknown closure_reason %q for status %s", reason, status)
}

func CanDelete(status string) bool {
	return status != StatusCompleted && status != StatusCollected && !IsClosure(status)
}

// CanEditDetails is false once the device has left the shop. Collected jobs are
// a sealed handoff record; earlier stages (including completed) may still need
// intake corrections when a typo surfaces later.
func CanEditDetails(status string) bool {
	return status != StatusCollected
}

// CanRequestParts is true while the job is still on the bench. Once it reaches
// QC / complete / closure, new supplier requests would orphan money and status.
func CanRequestParts(status string) bool {
	switch status {
	case StatusIntake, StatusDiagnosed, StatusWaitingParts, StatusInProgress:
		return true
	default:
		return false
	}
}

// RequiresClearPartsAndEstimates is true for transitions that mean "work is done
// enough to leave the bench" — outstanding supplier parts or a pending customer
// estimate must be cleared first.
func RequiresClearPartsAndEstimates(to string) bool {
	return to == StatusReadyPickup || to == StatusCompleted
}

// LeavingWaitingPartsForBench is waiting_parts → in_progress: staff must not
// skip an open supplier request by manually advancing.
func LeavingWaitingPartsForBench(from, to string) bool {
	return from == StatusWaitingParts && to == StatusInProgress
}
