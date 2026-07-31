package repair

import (
	"fmt"
	"strings"
	"unicode"
)

// NormalizePhone strips non-digits and normalizes Kenya MSISDNs to E.164 without '+'.
// Examples: "0712 345 678" -> "254712345678", "+254712345678" -> "254712345678".
func NormalizePhone(raw string) (string, error) {
	digits := digitsOnly(raw)
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

// PhoneMatchVariants returns digit forms that should match the same subscriber.
// Staff intake often stores "0712…"; OTP stores "254712…". Both must resolve to one customer.
func PhoneMatchVariants(rawOrE164 string) []string {
	digits := digitsOnly(rawOrE164)
	if digits == "" {
		return nil
	}
	seen := map[string]struct{}{digits: {}}
	out := []string{digits}

	add := func(v string) {
		if v == "" {
			return
		}
		if _, ok := seen[v]; ok {
			return
		}
		seen[v] = struct{}{}
		out = append(out, v)
	}

	e164, err := NormalizePhone(digits)
	if err == nil {
		add(e164)
		if strings.HasPrefix(e164, "254") && len(e164) == 12 {
			add("0" + e164[3:]) // 0712345678
			add(e164[3:])       // 712345678
		}
	}
	return out
}

func digitsOnly(raw string) string {
	var b strings.Builder
	b.Grow(len(raw))
	for _, r := range raw {
		if unicode.IsDigit(r) {
			b.WriteRune(r)
		}
	}
	return b.String()
}
