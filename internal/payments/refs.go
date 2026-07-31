package payments

import (
	"encoding/json"
	"fmt"
	"strings"
)

// IsDarajaCheckoutRef reports Safaricom checkout/request ids that should never
// be shown to customers on receipts (e.g. ws_CO_…).
func IsDarajaCheckoutRef(ref string) bool {
	r := strings.TrimSpace(ref)
	if r == "" {
		return false
	}
	upper := strings.ToUpper(r)
	return strings.HasPrefix(upper, "WS_CO_") ||
		strings.HasPrefix(upper, "WS_") ||
		strings.HasPrefix(upper, "MOCK-")
}

// CustomerFacingPaymentRef prefers an M-Pesa receipt number over a Daraja
// checkout/request id. Returns empty when only an internal checkout id exists.
func CustomerFacingPaymentRef(candidates ...string) string {
	for _, c := range candidates {
		c = strings.TrimSpace(c)
		if c == "" || IsDarajaCheckoutRef(c) {
			continue
		}
		return c
	}
	return ""
}

// ExtractMpesaReceiptFromCallback pulls MpesaReceiptNumber from a Daraja STK
// callback body when present.
func ExtractMpesaReceiptFromCallback(raw []byte) string {
	if len(raw) == 0 {
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
	if json.Unmarshal(raw, &body) != nil || body.Body.StkCallback.CallbackMetadata == nil {
		return ""
	}
	for _, it := range body.Body.StkCallback.CallbackMetadata.Item {
		if !strings.EqualFold(it.Name, "MpesaReceiptNumber") {
			continue
		}
		ref := strings.TrimSpace(fmt.Sprint(it.Value))
		if ref != "" && ref != "<nil>" && !IsDarajaCheckoutRef(ref) {
			return ref
		}
	}
	return ""
}
