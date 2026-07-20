package repair

import (
	"fmt"
	"strings"
	"unicode"
)

// NormalizePhone strips non-digits and normalizes Kenya MSISDNs to E.164 without '+'.
// Examples: "0712 345 678" -> "254712345678", "+254712345678" -> "254712345678".
func NormalizePhone(raw string) (string, error) {
	var b strings.Builder
	b.Grow(len(raw))
	for _, r := range raw {
		if unicode.IsDigit(r) {
			b.WriteRune(r)
		}
	}
	digits := b.String()
	if digits == "" {
		return "", fmt.Errorf("phone required")
	}
	switch {
	case strings.HasPrefix(digits, "254") && len(digits) == 12:
		return digits, nil
	case strings.HasPrefix(digits, "0") && len(digits) == 10:
		return "254" + digits[1:], nil
	case strings.HasPrefix(digits, "7") && len(digits) == 9:
		return "254" + digits, nil
	case len(digits) >= 10 && len(digits) <= 15:
		return digits, nil
	default:
		return "", fmt.Errorf("invalid phone number")
	}
}
