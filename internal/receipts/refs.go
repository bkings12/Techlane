package receipts

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// CustomerPaymentRef returns a customer-facing payment reference.
// Daraja checkout ids (ws_CO_…) are hidden — M-Pesa receipt numbers are preferred.
func CustomerPaymentRef(candidates ...string) string {
	for _, c := range candidates {
		c = strings.TrimSpace(c)
		if c == "" {
			continue
		}
		upper := strings.ToUpper(c)
		if strings.HasPrefix(upper, "WS_CO_") ||
			strings.HasPrefix(upper, "WS_") ||
			strings.HasPrefix(upper, "MOCK-") {
			continue
		}
		return c
	}
	return ""
}

// MpesaReceiptFromSTKCallback pulls MpesaReceiptNumber from a stored Daraja
// STK callback body when present.
func MpesaReceiptFromSTKCallback(raw string) string {
	return stkCallbackMetadataString(raw, "MpesaReceiptNumber")
}

// MpesaPhoneFromSTKCallback pulls PhoneNumber from a stored Daraja STK callback.
func MpesaPhoneFromSTKCallback(raw string) string {
	return stkCallbackMetadataString(raw, "PhoneNumber")
}

// MpesaTransactionTimeFromSTKCallback parses TransactionDate (yyyyMMddHHmmss)
// from a stored Daraja STK callback into a time in Africa/Nairobi when possible.
func MpesaTransactionTimeFromSTKCallback(raw string) *time.Time {
	rawVal := stkCallbackMetadataString(raw, "TransactionDate")
	return ParseMpesaTransactionDate(rawVal)
}

// ParseMpesaTransactionDate accepts Safaricom's yyyyMMddHHmmss (or with separators)
// and returns a pointer to the parsed local East Africa time.
func ParseMpesaTransactionDate(raw string) *time.Time {
	raw = strings.TrimSpace(raw)
	if raw == "" || raw == "<nil>" {
		return nil
	}
	digits := make([]rune, 0, len(raw))
	for _, r := range raw {
		if r >= '0' && r <= '9' {
			digits = append(digits, r)
		}
	}
	if len(digits) < 14 {
		return nil
	}
	compact := string(digits[:14])
	loc, err := time.LoadLocation("Africa/Nairobi")
	if err != nil {
		loc = time.FixedZone("EAT", 3*60*60)
	}
	t, err := time.ParseInLocation("20060102150405", compact, loc)
	if err != nil {
		return nil
	}
	return &t
}

func stkCallbackMetadataString(raw, name string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	var body struct {
		Body struct {
			StkCallback struct {
				CallbackMetadata *struct {
					Item []struct {
						Name  string `json:"Name"`
						Value any    `json:"Value"`
					} `json:"Item"`
				} `json:"CallbackMetadata"`
			} `json:"stkCallback"`
		} `json:"Body"`
	}
	if json.Unmarshal([]byte(raw), &body) != nil || body.Body.StkCallback.CallbackMetadata == nil {
		return ""
	}
	for _, it := range body.Body.StkCallback.CallbackMetadata.Item {
		if !strings.EqualFold(it.Name, name) {
			continue
		}
		ref := metadataValueString(it.Value)
		if ref == "" {
			continue
		}
		if strings.EqualFold(name, "MpesaReceiptNumber") {
			return CustomerPaymentRef(ref)
		}
		return ref
	}
	return ""
}

func metadataValueString(value any) string {
	if value == nil {
		return ""
	}
	switch v := value.(type) {
	case string:
		return strings.TrimSpace(v)
	case float64:
		return strings.TrimSpace(fmt.Sprintf("%.0f", v))
	case json.Number:
		return strings.TrimSpace(v.String())
	default:
		s := strings.TrimSpace(fmt.Sprint(v))
		if s == "" || s == "<nil>" {
			return ""
		}
		if strings.Contains(s, ".") {
			if f, err := parseFloatish(s); err == nil {
				return fmt.Sprintf("%.0f", f)
			}
		}
		return s
	}
}

func parseFloatish(s string) (float64, error) {
	var f float64
	_, err := fmt.Sscanf(s, "%f", &f)
	return f, err
}
