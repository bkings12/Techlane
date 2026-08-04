package whatsapp

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

// Client talks to the Baileys WhatsApp sidecar (Netpay-compatible API).
type Client struct {
	BaseURL    string
	ServiceKey string
	HTTP       *http.Client
}

func NewClient(baseURL, serviceKey string) *Client {
	baseURL = strings.TrimRight(strings.TrimSpace(baseURL), "/")
	return &Client{
		BaseURL:    baseURL,
		ServiceKey: strings.TrimSpace(serviceKey),
		HTTP:       &http.Client{Timeout: 45 * time.Second},
	}
}

func (c *Client) Configured() bool {
	return c != nil && c.BaseURL != "" && c.ServiceKey != ""
}

type StatusResponse struct {
	Success   bool   `json:"success"`
	Status    string `json:"status"`
	Connected bool   `json:"connected"`
	HasQR     bool   `json:"hasQr"`
	User      any    `json:"user"`
	Error     string `json:"error"`
	// The reason the sidecar's circuit breaker tripped, when Status is
	// "reconnect_failed". Empty otherwise.
	LastError string `json:"lastError"`
	// The active pairing code, when Status is "waiting_pairing_code". Empty
	// otherwise.
	PairingCode string `json:"pairingCode"`
}

type QRResponse struct {
	Success bool   `json:"success"`
	Status  string `json:"status"`
	QR      string `json:"qr"`
	Message string `json:"message"`
	User    any    `json:"user"`
	Error   string `json:"error"`
}

// PairingCodeResponse is returned when requesting phone-number pairing-code
// linking — an alternative to scanning a QR that WhatsApp Business often
// accepts where its QR scan is refused.
type PairingCodeResponse struct {
	Success bool   `json:"success"`
	Status  string `json:"status"`
	Code    string `json:"code"`
	Phone   string `json:"phone"`
	Message string `json:"message"`
	Error   string `json:"error"`
}

type SendResponse struct {
	Success   bool   `json:"success"`
	MessageID string `json:"messageId"`
	Phone     string `json:"phone"`
	Error     string `json:"error"`
}

func (c *Client) Status(ctx context.Context, sessionID string) (*StatusResponse, error) {
	var out StatusResponse
	if err := c.get(ctx, "/status/"+sessionID, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *Client) QR(ctx context.Context, sessionID string) (*QRResponse, error) {
	var out QRResponse
	if err := c.get(ctx, "/qr/"+sessionID, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *Client) Disconnect(ctx context.Context, sessionID string) error {
	var out map[string]any
	return c.post(ctx, "/disconnect/"+sessionID, nil, &out)
}

func (c *Client) Reconnect(ctx context.Context, sessionID string) error {
	var out map[string]any
	return c.post(ctx, "/reconnect/"+sessionID, nil, &out)
}

// RequestPairingCode asks the sidecar to link via a phone-number pairing
// code instead of a QR scan. phone should include the country code (digits
// only or with punctuation — the sidecar strips non-digits).
func (c *Client) RequestPairingCode(ctx context.Context, sessionID, phone string) (*PairingCodeResponse, error) {
	body := map[string]any{"phone": phone}
	var out PairingCodeResponse
	if err := c.post(ctx, "/pairing-code/"+sessionID, body, &out); err != nil {
		return nil, err
	}
	if !out.Success {
		errMsg := strings.TrimSpace(out.Error)
		if errMsg == "" {
			errMsg = "pairing code request failed"
		}
		return &out, fmt.Errorf("%s", errMsg)
	}
	return &out, nil
}

func (c *Client) Send(ctx context.Context, sessionID, phone, message string) (*SendResponse, error) {
	body := map[string]any{
		"tenantId": sessionID,
		"phone":    phone,
		"message":  message,
		"type":     "text",
	}
	var out SendResponse
	if err := c.post(ctx, "/send", body, &out); err != nil {
		return nil, err
	}
	if !out.Success {
		errMsg := strings.TrimSpace(out.Error)
		if errMsg == "" {
			errMsg = "whatsapp send failed"
		}
		return &out, fmt.Errorf("%s", errMsg)
	}
	return &out, nil
}

func (c *Client) get(ctx context.Context, path string, dest any) error {
	if !c.Configured() {
		return fmt.Errorf("whatsapp service not configured")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.BaseURL+path, nil)
	if err != nil {
		return err
	}
	req.Header.Set("X-WhatsApp-Service-Key", c.ServiceKey)
	res, err := c.HTTP.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(res.Body, 1<<20))
	if res.StatusCode >= 400 {
		return fmt.Errorf("whatsapp %s: http %d: %s", path, res.StatusCode, strings.TrimSpace(string(raw)))
	}
	if dest == nil {
		return nil
	}
	return json.Unmarshal(raw, dest)
}

func (c *Client) post(ctx context.Context, path string, body any, dest any) error {
	if !c.Configured() {
		return fmt.Errorf("whatsapp service not configured")
	}
	var rdr io.Reader
	if body != nil {
		raw, err := json.Marshal(body)
		if err != nil {
			return err
		}
		rdr = bytes.NewReader(raw)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.BaseURL+path, rdr)
	if err != nil {
		return err
	}
	req.Header.Set("X-WhatsApp-Service-Key", c.ServiceKey)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	res, err := c.HTTP.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(res.Body, 1<<20))
	if res.StatusCode >= 400 {
		return fmt.Errorf("whatsapp %s: http %d: %s", path, res.StatusCode, strings.TrimSpace(string(raw)))
	}
	if dest == nil || len(raw) == 0 {
		return nil
	}
	return json.Unmarshal(raw, dest)
}
