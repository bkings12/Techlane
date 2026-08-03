package notify

import (
	"fmt"
	"strings"
	"unicode"
)

// NormalizeRecipientPhone strips separators and normalizes Kenya numbers to
// 2547… (E.164 without '+'), matching what BlessedTexts / WhatsApp expect.
func NormalizeRecipientPhone(raw string) (string, error) {
	digits := recipientDigits(raw)
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

// SplitPhoneList pulls distinct numbers out of owner-entered blobs like
// "0723433660 / 0723239995 / 0726676628".
func SplitPhoneList(raw string) []string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	fields := strings.FieldsFunc(raw, func(r rune) bool {
		switch r {
		case '/', ',', ';', '|', '\n', '\t':
			return true
		default:
			return false
		}
	})
	seen := map[string]struct{}{}
	var out []string
	for _, f := range fields {
		f = strings.TrimSpace(f)
		if f == "" {
			continue
		}
		norm, err := NormalizeRecipientPhone(f)
		if err != nil {
			continue
		}
		if _, ok := seen[norm]; ok {
			continue
		}
		seen[norm] = struct{}{}
		out = append(out, norm)
	}
	return out
}

func recipientDigits(raw string) string {
	var b strings.Builder
	for _, r := range raw {
		if unicode.IsDigit(r) {
			b.WriteRune(r)
		}
	}
	return b.String()
}
