package payments

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/google/uuid"
)

type darajaTokenResp struct {
	AccessToken string `json:"access_token"`
	ExpiresIn   string `json:"expires_in"`
}

type stkPushReq struct {
	BusinessShortCode string `json:"BusinessShortCode"`
	Password          string `json:"Password"`
	Timestamp         string `json:"Timestamp"`
	TransactionType   string `json:"TransactionType"`
	Amount            string `json:"Amount"`
	PartyA            string `json:"PartyA"`
	PartyB            string `json:"PartyB"`
	PhoneNumber       string `json:"PhoneNumber"`
	CallBackURL       string `json:"CallBackURL"`
	AccountReference  string `json:"AccountReference"`
	TransactionDesc   string `json:"TransactionDesc"`
}

type stkPushResp struct {
	MerchantRequestID   string `json:"MerchantRequestID"`
	CheckoutRequestID   string `json:"CheckoutRequestID"`
	ResponseCode        string `json:"ResponseCode"`
	ResponseDescription string `json:"ResponseDescription"`
	CustomerMessage     string `json:"CustomerMessage"`
	ErrorCode           string `json:"errorCode"`
	ErrorMessage        string `json:"errorMessage"`
}

func darajaBaseURL(env string) string {
	if env == "production" {
		return "https://api.safaricom.co.ke"
	}
	return "https://sandbox.safaricom.co.ke"
}

func normalizeMSISDN(phone string) (string, error) {
	p := strings.TrimSpace(phone)
	p = strings.ReplaceAll(p, " ", "")
	p = strings.ReplaceAll(p, "-", "")
	p = strings.ReplaceAll(p, "+", "")
	if strings.HasPrefix(p, "0") && len(p) == 10 {
		p = "254" + p[1:]
	}
	if strings.HasPrefix(p, "254") && len(p) == 12 {
		return p, nil
	}
	return "", fmt.Errorf("phone must be 07… or 2547… format")
}

func (s *Service) shouldMockDaraja(consumerKey, consumerSecret string) bool {
	// Never mock in production/staging: a leftover MPESA_MOCK=1 or test creds
	// would silently auto-confirm real payments.
	env := strings.ToLower(strings.TrimSpace(os.Getenv("APP_ENV")))
	if env == "production" || env == "staging" {
		return false
	}
	if os.Getenv("MPESA_MOCK") == "1" {
		return true
	}
	key := strings.TrimSpace(consumerKey)
	sec := strings.TrimSpace(consumerSecret)
	return key == "test-key" || sec == "test-secret" || key == "mock"
}

func (s *Service) fetchDarajaToken(ctx context.Context, env, consumerKey, consumerSecret string) (string, error) {
	url := darajaBaseURL(env) + "/oauth/v1/generate?grant_type=client_credentials"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", err
	}
	cred := base64.StdEncoding.EncodeToString([]byte(consumerKey + ":" + consumerSecret))
	req.Header.Set("Authorization", "Basic "+cred)
	client := &http.Client{Timeout: 20 * time.Second}
	res, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer res.Body.Close()
	body, _ := io.ReadAll(res.Body)
	if res.StatusCode >= 300 {
		return "", fmt.Errorf("daraja oauth %d: %s", res.StatusCode, truncate(string(body), 200))
	}
	var tok darajaTokenResp
	if err := json.Unmarshal(body, &tok); err != nil || tok.AccessToken == "" {
		return "", fmt.Errorf("daraja oauth parse failed")
	}
	return tok.AccessToken, nil
}

func stkPassword(shortcode, passkey, timestamp string) string {
	return base64.StdEncoding.EncodeToString([]byte(shortcode + passkey + timestamp))
}

// stkPushTargets picks Daraja auth shortcode, PartyB destination, and account ref.
// Auth always uses M-Pesa shortcode + passkey. When bank paybill is enabled,
// PartyB/account route the payment to the bank while credentials stay M-Pesa.
func stkPushTargets(raw rawSettings, accountRef string) (authShortcode, partyB, acct string) {
	authShortcode = strings.TrimSpace(raw.Shortcode)
	partyB = authShortcode
	acct = strings.TrimSpace(accountRef)
	if acct == "" {
		acct = "TechLane"
	}

	bankPaybill := strings.TrimSpace(raw.BankPaybill)
	bankAccount := strings.TrimSpace(raw.BankAccount)
	if raw.BankEnabled && bankPaybill != "" && bankAccount != "" {
		partyB = bankPaybill
		acct = bankAccount
	}
	if len(acct) > 12 {
		acct = acct[:12]
	}
	return authShortcode, partyB, acct
}

func (s *Service) InitiateSTKPush(ctx context.Context, tenantID, paymentID uuid.UUID, amount float64, phone, accountRef string) (checkoutID, merchantID string, err error) {
	raw, err := s.loadRawSettings(ctx, tenantID)
	if err != nil {
		return "", "", err
	}
	if !raw.MpesaEnabled || raw.Shortcode == "" || raw.ConsumerKey == "" || raw.ConsumerSecret == "" || raw.Passkey == "" {
		return "", "", fmt.Errorf("M-Pesa credentials not configured")
	}
	authShortcode, partyB, acct := stkPushTargets(raw, accountRef)
	if authShortcode == "" {
		return "", "", fmt.Errorf("M-Pesa shortcode not configured")
	}
	msisdn, err := normalizeMSISDN(phone)
	if err != nil {
		return "", "", err
	}
	// Persist the ref customers see on their phone (bank account when bank is on).
	_, _ = s.pool.Exec(ctx, `
		UPDATE payments.mpesa_stk_transactions
		SET account_reference = $1, updated_at = now()
		WHERE payment_id = $2`, acct, paymentID)

	if s.shouldMockDaraja(raw.ConsumerKey, raw.ConsumerSecret) {
		checkoutID = fmt.Sprintf("ws_CO_MOCK_%s", paymentID.String()[:8])
		merchantID = fmt.Sprintf("mock-%s", paymentID.String()[:8])
		return checkoutID, merchantID, nil
	}

	token, err := s.fetchDarajaToken(ctx, raw.Environment, raw.ConsumerKey, raw.ConsumerSecret)
	if err != nil {
		return "", "", err
	}

	ts := time.Now().UTC().Format("20060102150405")
	callback, err := s.resolveSTKCallbackURL(raw)
	if err != nil {
		return "", "", err
	}
	amt := fmt.Sprintf("%.0f", amount)
	payload := stkPushReq{
		BusinessShortCode: authShortcode,
		Password:          stkPassword(authShortcode, raw.Passkey, ts),
		Timestamp:         ts,
		TransactionType:   "CustomerPayBillOnline",
		Amount:            amt,
		PartyA:            msisdn,
		PartyB:            partyB,
		PhoneNumber:       msisdn,
		CallBackURL:       callback,
		AccountReference:  acct,
		TransactionDesc:   "TechLane payment",
	}
	b, _ := json.Marshal(payload)
	url := darajaBaseURL(raw.Environment) + "/mpesa/stkpush/v1/processrequest"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(b))
	if err != nil {
		return "", "", err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	client := &http.Client{Timeout: 30 * time.Second}
	res, err := client.Do(req)
	if err != nil {
		return "", "", err
	}
	defer res.Body.Close()
	body, _ := io.ReadAll(res.Body)
	var out stkPushResp
	_ = json.Unmarshal(body, &out)
	if res.StatusCode >= 300 || out.ResponseCode != "0" {
		msg := out.ErrorMessage
		if msg == "" {
			msg = out.ResponseDescription
		}
		if msg == "" {
			msg = truncate(string(body), 200)
		}
		return "", "", fmt.Errorf("stk push failed: %s", msg)
	}
	return out.CheckoutRequestID, out.MerchantRequestID, nil
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}
