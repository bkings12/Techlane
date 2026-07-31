package receipts

import (
	"encoding/json"
	"fmt"
	"strings"
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
		if !strings.EqualFold(it.Name, "MpesaReceiptNumber") {
			continue
		}
		ref := strings.TrimSpace(fmt.Sprint(it.Value))
		if ref != "" && ref != "<nil>" {
			return CustomerPaymentRef(ref)
		}
	}
	return ""
}
