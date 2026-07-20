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

func IsDigitalMethod(method string) bool {
	switch method {
	case "mpesa_stk", "mpesa_c2b", "bank_paybill", "bank_transfer", "card", "store_credit":
		return true
	default:
		return false
	}
}

func InitialPaymentStatus(method string) string {
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
