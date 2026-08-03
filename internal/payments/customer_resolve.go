package payments

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"unicode"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// normalizeMSISDNLoose turns Kenyan till/phone strings into 2547… digits when possible.
// Unlike normalizeMSISDN it never errors — empty/invalid input returns "".
func normalizeMSISDNLoose(phone string) string {
	digits := digitsOnlyPhone(phone)
	if digits == "" {
		return ""
	}
	switch {
	case strings.HasPrefix(digits, "254") && len(digits) == 12:
		return digits
	case strings.HasPrefix(digits, "0") && len(digits) == 10:
		return "254" + digits[1:]
	case strings.HasPrefix(digits, "7") && len(digits) == 9:
		return "254" + digits
	default:
		return digits
	}
}

func digitsOnlyPhone(raw string) string {
	var b strings.Builder
	b.Grow(len(raw))
	for _, r := range raw {
		if unicode.IsDigit(r) {
			b.WriteRune(r)
		}
	}
	return b.String()
}

func phoneMatchVariants(raw string) []string {
	digits := digitsOnlyPhone(raw)
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
	canon := normalizeMSISDNLoose(digits)
	add(canon)
	if strings.HasPrefix(canon, "254") && len(canon) == 12 {
		add("0" + canon[3:])
		add(canon[3:])
	}
	return out
}

func joinPersonName(parts ...string) string {
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return strings.Join(out, " ")
}

// resolveOrCreateCustomerByPhoneAndName finds a repair.customers row by MSISDN.
// If none exists and name is non-empty, creates one. Never creates a nameless customer.
func (s *Service) resolveOrCreateCustomerByPhoneAndName(ctx context.Context, tenantID uuid.UUID, msisdn, name string) (*uuid.UUID, error) {
	phone := normalizeMSISDNLoose(msisdn)
	if phone == "" {
		return nil, nil
	}
	variants := phoneMatchVariants(phone)
	var existingID uuid.UUID
	err := s.pool.QueryRow(ctx, `
		SELECT id FROM repair.customers
		WHERE tenant_id = $1
		  AND regexp_replace(COALESCE(phone, ''), '[^0-9]', '', 'g') = ANY($2::text[])
		ORDER BY created_at ASC
		LIMIT 1`, tenantID, variants).Scan(&existingID)
	if err == nil {
		return &existingID, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return nil, err
	}

	name = strings.TrimSpace(name)
	if name == "" {
		return nil, nil
	}
	id := uuid.New()
	_, err = s.pool.Exec(ctx, `
		INSERT INTO repair.customers (id, tenant_id, full_name, phone)
		VALUES ($1, $2, $3, $4)`, id, tenantID, name, phone)
	if err != nil {
		// Race on unique phone — return the winner.
		if strings.Contains(err.Error(), "idx_customers_tenant_phone_normalized") ||
			strings.Contains(err.Error(), "duplicate key") {
			if qErr := s.pool.QueryRow(ctx, `
				SELECT id FROM repair.customers
				WHERE tenant_id = $1
				  AND regexp_replace(COALESCE(phone, ''), '[^0-9]', '', 'g') = ANY($2::text[])
				LIMIT 1`, tenantID, variants).Scan(&existingID); qErr == nil {
				return &existingID, nil
			}
		}
		return nil, err
	}
	return &id, nil
}

// attachMpesaCustomerToSale resolves a customer from M-Pesa payer details and sets
// sales.sales.customer_id when the payment is allocated to a sale that has none yet.
func (s *Service) attachMpesaCustomerToSale(ctx context.Context, tenantID, paymentID uuid.UUID, msisdn, name string) error {
	custID, err := s.resolveOrCreateCustomerByPhoneAndName(ctx, tenantID, msisdn, name)
	if err != nil {
		return err
	}
	if custID == nil {
		return nil
	}
	var saleID uuid.UUID
	err = s.pool.QueryRow(ctx, `
		SELECT payable_id FROM payments.payment_allocations
		WHERE tenant_id = $1 AND payment_id = $2 AND payable_type = 'sale'
		ORDER BY created_at ASC
		LIMIT 1`, tenantID, paymentID).Scan(&saleID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil
		}
		return err
	}
	_, err = s.pool.Exec(ctx, `
		UPDATE sales.sales
		SET customer_id = $1, updated_at = now()
		WHERE tenant_id = $2 AND id = $3 AND customer_id IS NULL`,
		*custID, tenantID, saleID)
	return err
}

func metadataItemString(value any) string {
	if value == nil {
		return ""
	}
	switch v := value.(type) {
	case string:
		return strings.TrimSpace(v)
	case float64:
		// JSON numbers for PhoneNumber arrive without decimals (e.g. 2547…).
		return strings.TrimSpace(fmt.Sprintf("%.0f", v))
	default:
		return strings.TrimSpace(fmt.Sprint(v))
	}
}

// attachCustomerForDigitalPayment pulls payer phone/name from the STK or C2B row
// linked to this payment and attaches a customer onto the sale when possible.
func (s *Service) attachCustomerForDigitalPayment(ctx context.Context, tenantID, paymentID uuid.UUID, method string) error {
	switch method {
	case "mpesa_stk":
		var phone, raw string
		_ = s.pool.QueryRow(ctx, `
			SELECT COALESCE(phone, ''), COALESCE(raw_callback::text, '')
			FROM payments.mpesa_stk_transactions WHERE payment_id = $1`, paymentID).Scan(&phone, &raw)
		if phone == "" {
			phone = extractSTKPhoneFromRaw(raw)
		}
		// STK metadata has no payer name on this tenant — match existing by phone only.
		return s.attachMpesaCustomerToSale(ctx, tenantID, paymentID, phone, "")
	case "mpesa_c2b":
		var msisdn, firstName, middleName, lastName string
		_ = s.pool.QueryRow(ctx, `
			SELECT COALESCE(msisdn, ''), COALESCE(first_name, ''), COALESCE(middle_name, ''), COALESCE(last_name, '')
			FROM payments.mpesa_c2b_transactions
			WHERE payment_id = $1 AND status IS DISTINCT FROM 'superseded'
			ORDER BY updated_at DESC NULLS LAST, created_at DESC
			LIMIT 1`, paymentID).Scan(&msisdn, &firstName, &middleName, &lastName)
		return s.attachMpesaCustomerToSale(ctx, tenantID, paymentID, msisdn, joinPersonName(firstName, middleName, lastName))
	default:
		return nil
	}
}

func extractSTKPhoneFromRaw(raw string) string {
	if strings.TrimSpace(raw) == "" {
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
	if err := json.Unmarshal([]byte(raw), &body); err != nil || body.Body.StkCallback.CallbackMetadata == nil {
		return ""
	}
	for _, it := range body.Body.StkCallback.CallbackMetadata.Item {
		if it.Name == "PhoneNumber" {
			return metadataItemString(it.Value)
		}
	}
	return ""
}
