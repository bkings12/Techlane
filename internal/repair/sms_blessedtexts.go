package repair

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

const defaultBlessedTextsBaseURL = "https://sms.blessedtexts.com/api/sms/v1"

// BlessedTextsSMSSender sends OTPs via the BlessedTexts bulk SMS API.
// Docs: POST {base}/sendsms with api_key, sender_id, message, phone.
type BlessedTextsSMSSender struct {
	APIKey   string
	SenderID string
	BaseURL  string
	Client   *http.Client
}

func NewBlessedTextsSMSSender(apiKey, senderID, baseURL string) *BlessedTextsSMSSender {
	baseURL = strings.TrimRight(strings.TrimSpace(baseURL), "/")
	if baseURL == "" {
		baseURL = defaultBlessedTextsBaseURL
	}
	return &BlessedTextsSMSSender{
		APIKey:   strings.TrimSpace(apiKey),
		SenderID: strings.TrimSpace(senderID),
		BaseURL:  baseURL,
		Client:   &http.Client{Timeout: 20 * time.Second},
	}
}

func (b *BlessedTextsSMSSender) SendOTP(ctx context.Context, phoneE164, code string) error {
	message := fmt.Sprintf(
		"Your verification code is %s. It expires in 10 minutes. Do not share this code with anyone.",
		code,
	)
	return b.SendMessage(ctx, phoneE164, message)
}

func (b *BlessedTextsSMSSender) SendMessage(ctx context.Context, phoneE164, message string) error {
	if b.APIKey == "" {
		return fmt.Errorf("blessedtexts api key not configured")
	}
	if b.SenderID == "" {
		return fmt.Errorf("blessedtexts sender id not configured")
	}
	phone := strings.TrimSpace(phoneE164)
	if phone == "" {
		return fmt.Errorf("phone required")
	}
	if strings.TrimSpace(message) == "" {
		return fmt.Errorf("message required")
	}
	body := map[string]string{
		"api_key":   b.APIKey,
		"sender_id": b.SenderID,
		"message":   message,
		"phone":     phone,
	}
	payload, err := json.Marshal(body)
	if err != nil {
		return err
	}
	return b.postSendSMS(ctx, payload)
}

func (b *BlessedTextsSMSSender) postSendSMS(ctx context.Context, payload []byte) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, b.BaseURL+"/sendsms", bytes.NewReader(payload))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	client := b.Client
	if client == nil {
		client = http.DefaultClient
	}
	var lastErr error
	for attempt := 0; attempt < 3; attempt++ {
		if attempt > 0 {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(time.Duration(attempt) * 400 * time.Millisecond):
			}
			req, err = http.NewRequestWithContext(ctx, http.MethodPost, b.BaseURL+"/sendsms", bytes.NewReader(payload))
			if err != nil {
				return err
			}
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("Accept", "application/json")
		}
		res, err := client.Do(req)
		if err != nil {
			lastErr = fmt.Errorf("blessedtexts request failed: %w", err)
			continue
		}
		raw, _ := io.ReadAll(io.LimitReader(res.Body, 1<<20))
		res.Body.Close()
		if res.StatusCode >= 500 {
			lastErr = fmt.Errorf("blessedtexts http %d: %s", res.StatusCode, strings.TrimSpace(string(raw)))
			continue
		}
		if res.StatusCode < 200 || res.StatusCode >= 300 {
			return fmt.Errorf("blessedtexts http %d: %s", res.StatusCode, strings.TrimSpace(string(raw)))
		}
		if err := interpretBlessedTextsSendResponse(raw); err != nil {
			return err
		}
		return nil
	}
	if lastErr != nil {
		return lastErr
	}
	return fmt.Errorf("blessedtexts send failed")
}

type blessedTextsSendItem struct {
	StatusCode string `json:"status_code"`
	StatusDesc string `json:"status_desc"`
	Phone      string `json:"phone"`
	MessageID  string `json:"message_id"`
}

func interpretBlessedTextsSendResponse(raw []byte) error {
	raw = bytes.TrimSpace(raw)
	if len(raw) == 0 {
		return fmt.Errorf("blessedtexts empty response")
	}
	// API returns an array of per-phone results on success.
	var items []blessedTextsSendItem
	if err := json.Unmarshal(raw, &items); err == nil {
		if len(items) == 0 {
			return fmt.Errorf("blessedtexts empty result set")
		}
		for _, item := range items {
			if item.StatusCode != "1000" {
				desc := item.StatusDesc
				if desc == "" {
					desc = "send failed"
				}
				return fmt.Errorf("blessedtexts %s: %s", item.StatusCode, desc)
			}
		}
		return nil
	}
	// Some error paths may return a single object.
	var one blessedTextsSendItem
	if err := json.Unmarshal(raw, &one); err == nil && one.StatusCode != "" {
		if one.StatusCode == "1000" {
			return nil
		}
		desc := one.StatusDesc
		if desc == "" {
			desc = "send failed"
		}
		return fmt.Errorf("blessedtexts %s: %s", one.StatusCode, desc)
	}
	return fmt.Errorf("blessedtexts unexpected response: %s", string(raw))
}
