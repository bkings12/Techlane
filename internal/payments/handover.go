package payments

// IsCashMethod is till cash taken at the counter (immediate custody).
func IsCashMethod(method string) bool {
	return method == "cash"
}

// IsCashOnPickup is storefront branch pickup paid in cash at the counter
// (not cash-on-delivery).
func IsCashOnPickup(method string) bool {
	return method == "cash_on_pickup"
}

func IsDigitalMethod(method string) bool {
	switch method {
	case "mpesa_stk", "mpesa_c2b", "bank_paybill", "bank_transfer", "card", "store_credit":
		return true
	default:
		return false
	}
}

func InitialPaymentStatus(method string) string {
	if IsCashOnPickup(method) {
		return "pending"
	}
	if IsCashMethod(method) {
		// Cash is physically in hand at the point of sale — no async confirmation.
		return "confirmed"
	}
	if method == "store_credit" {
		return "allocated"
	}
	if IsDigitalMethod(method) {
		return "initiated"
	}
	return "pending"
}
