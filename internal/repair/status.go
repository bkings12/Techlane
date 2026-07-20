package repair

import "fmt"

const (
	StatusIntake        = "intake"
	StatusDiagnosed     = "diagnosed"
	StatusWaitingParts  = "waiting_parts"
	StatusInProgress    = "in_progress"
	StatusCompleted     = "completed"
	StatusCollected     = "collected"
)

var allowedTransitions = map[string][]string{
	StatusIntake:       {StatusDiagnosed, StatusWaitingParts, StatusInProgress},
	StatusDiagnosed:    {StatusWaitingParts, StatusInProgress},
	StatusWaitingParts: {StatusInProgress, StatusDiagnosed},
	StatusInProgress:   {StatusWaitingParts, StatusCompleted},
	StatusCompleted:    {StatusCollected},
	StatusCollected:    {},
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

func CanDelete(status string) bool {
	return status != StatusCompleted && status != StatusCollected
}
