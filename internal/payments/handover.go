package payments

import (
	"errors"
	"fmt"
)

var ErrSelfApproveHandover = errors.New("confirmer cannot be the same as from_user")

func ValidateHandoverConfirm(fromUserID, confirmerID string) error {
	if fromUserID == "" || confirmerID == "" {
		return fmt.Errorf("from_user and confirmer required")
	}
	if fromUserID == confirmerID {
		return ErrSelfApproveHandover
	}
	return nil
}

func IsCashMethod(method string) bool {
	return method == "cash"
}

// IsCashOnPickup is storefront branch pickup paid in cash at the counter
// (not cash-on-delivery and not till cash that needs dual-control handover).
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
		return "pending_handover"
	}
	if method == "store_credit" {
		return "allocated"
	}
	if IsDigitalMethod(method) {
		return "initiated"
	}
	return "pending"
}
