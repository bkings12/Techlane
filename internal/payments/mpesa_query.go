package payments

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
)

type stkQueryReq struct {
	BusinessShortCode string `json:"BusinessShortCode"`
	Password          string `json:"Password"`
	Timestamp         string `json:"Timestamp"`
	CheckoutRequestID string `json:"CheckoutRequestID"`
}

type stkQueryResp struct {
	ResponseCode        string `json:"ResponseCode"`
	ResponseDescription string `json:"ResponseDescription"`
	MerchantRequestID   string `json:"MerchantRequestID"`
	CheckoutRequestID   string `json:"CheckoutRequestID"`
	ResultCode          string `json:"ResultCode"`
	ResultDesc          string `json:"ResultDesc"`
	ErrorCode           string `json:"errorCode"`
	ErrorMessage        string `json:"errorMessage"`
}

// QuerySTKStatus calls Safaricom STK Query for a checkout request.
func (s *Service) QuerySTKStatus(ctx context.Context, tenantID uuid.UUID, checkoutRequestID string) (*stkQueryResp, error) {
	raw, err := s.loadRawSettings(ctx, tenantID)
	if err != nil {
		return nil, err
	}
	if raw.Shortcode == "" || raw.ConsumerKey == "" || raw.ConsumerSecret == "" || raw.Passkey == "" {
		return nil, fmt.Errorf("M-Pesa credentials not configured")
	}
	checkoutRequestID = strings.TrimSpace(checkoutRequestID)
	if checkoutRequestID == "" {
		return nil, fmt.Errorf("checkout_request_id required")
	}

	if s.shouldMockDaraja(raw.ConsumerKey, raw.ConsumerSecret) {
		return &stkQueryResp{
			ResponseCode:      "0",
			ResultCode:        "0",
			ResultDesc:        "The service request is processed successfully.",
			CheckoutRequestID: checkoutRequestID,
		}, nil
	}

	token, err := s.fetchDarajaToken(ctx, raw.Environment, raw.ConsumerKey, raw.ConsumerSecret)
	if err != nil {
		return nil, err
	}
	ts := time.Now().UTC().Format("20060102150405")
	payload := stkQueryReq{
		BusinessShortCode: raw.Shortcode,
		Password:          stkPassword(raw.Shortcode, raw.Passkey, ts),
		Timestamp:         ts,
		CheckoutRequestID: checkoutRequestID,
	}
	b, _ := json.Marshal(payload)
	url := darajaBaseURL(raw.Environment) + "/mpesa/stkpushquery/v1/query"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(b))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	client := &http.Client{Timeout: 30 * time.Second}
	res, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	body, _ := io.ReadAll(res.Body)
	var out stkQueryResp
	if err := json.Unmarshal(body, &out); err != nil {
		return nil, fmt.Errorf("stk query parse failed: %s", truncate(string(body), 200))
	}
	if res.StatusCode >= 300 && out.ResponseCode == "" && out.ResultCode == "" {
		msg := out.ErrorMessage
		if msg == "" {
			msg = truncate(string(body), 200)
		}
		return nil, fmt.Errorf("stk query http %d: %s", res.StatusCode, msg)
	}
	return &out, nil
}

// ReconcileSTKPayment queries Daraja and confirms only when ResultCode is success.
// Typed provider refs from staff are not accepted as proof of payment.
func (s *Service) ReconcileSTKPayment(ctx context.Context, tenantID, paymentID uuid.UUID) (*Payment, error) {
	var checkoutID, status string
	err := s.pool.QueryRow(ctx, `
		SELECT COALESCE(checkout_request_id, ''), status
		FROM payments.mpesa_stk_transactions
		WHERE tenant_id = $1 AND payment_id = $2
		ORDER BY created_at DESC LIMIT 1`, tenantID, paymentID).Scan(&checkoutID, &status)
	if err != nil {
		return nil, fmt.Errorf("stk transaction not found")
	}
	if status == "confirmed" {
		return s.GetPayment(ctx, tenantID, paymentID)
	}
	if checkoutID == "" {
		return nil, fmt.Errorf("checkout_request_id missing; cannot reconcile")
	}

	q, err := s.QuerySTKStatus(ctx, tenantID, checkoutID)
	if err != nil {
		return nil, err
	}
	_, _ = s.pool.Exec(ctx, `
		UPDATE payments.mpesa_stk_transactions
		SET last_queried_at = now(), query_result_code = $1, query_result_desc = $2, updated_at = now()
		WHERE payment_id = $3`, q.ResultCode, q.ResultDesc, paymentID)

	if q.ResultCode != "0" {
		if q.ResultCode != "" && q.ResultCode != "500.001.1001" {
			// Still processing codes vary; treat non-zero as not paid unless "request timeout in progress"
			if !strings.Contains(strings.ToLower(q.ResultDesc), "being processed") {
				_, _ = s.pool.Exec(ctx, `
					UPDATE payments.mpesa_stk_transactions
					SET status = 'failed', result_code = $1, result_desc = $2, updated_at = now()
					WHERE payment_id = $3 AND status NOT IN ('confirmed')`, q.ResultCode, q.ResultDesc, paymentID)
			}
		}
		return nil, fmt.Errorf("STK not paid: %s %s", q.ResultCode, q.ResultDesc)
	}

	ref := checkoutID
	if q.MerchantRequestID != "" {
		ref = q.CheckoutRequestID
	}
	return s.ConfirmMpesaWebhook(ctx, tenantID, paymentID, ref)
}

// ReconcilePendingSTK queries stale STK transactions and confirms successes.
func (s *Service) ReconcilePendingSTK(ctx context.Context, tenantID uuid.UUID) (int, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT payment_id, COALESCE(checkout_request_id, '')
		FROM payments.mpesa_stk_transactions
		WHERE tenant_id = $1
		  AND status IN ('pending', 'stk_sent')
		  AND checkout_request_id IS NOT NULL
		  AND created_at < now() - interval '90 seconds'
		  AND (last_queried_at IS NULL OR last_queried_at < now() - interval '60 seconds')
		ORDER BY created_at ASC
		LIMIT 25`, tenantID)
	if err != nil {
		return 0, err
	}
	defer rows.Close()

	confirmed := 0
	for rows.Next() {
		var paymentID uuid.UUID
		var checkout string
		if err := rows.Scan(&paymentID, &checkout); err != nil {
			return confirmed, err
		}
		if checkout == "" {
			continue
		}
		if _, err := s.ReconcileSTKPayment(ctx, tenantID, paymentID); err == nil {
			confirmed++
		}
	}
	return confirmed, rows.Err()
}
