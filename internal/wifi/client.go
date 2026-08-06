package wifi

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

type Client struct {
	HTTP *http.Client
}

func NewClient() *Client {
	return &Client{HTTP: &http.Client{Timeout: 20 * time.Second}}
}

type IssueRequest struct {
	SiteID       string `json:"siteId"`
	PackageID    string `json:"packageId,omitempty"`
	DurationMins int    `json:"durationMins"`
	Quantity     int    `json:"quantity,omitempty"`
	Phone        string `json:"phone,omitempty"`
	SendSMS      bool   `json:"sendSms,omitempty"`
	Reference    string `json:"reference,omitempty"`
}

type IssuedVoucher struct {
	Code         string `json:"code"`
	ExpiresAt    string `json:"expiresAt"`
	PackageName  string `json:"packageName"`
	DurationMins int    `json:"durationMins"`
	RedeemURL    string `json:"redeemUrl"`
	QRPayload    string `json:"qrPayload"`
}

type IssueResponse struct {
	Vouchers  []IssuedVoucher `json:"vouchers"`
	BatchID   string          `json:"batchId"`
	Reference string          `json:"reference"`
}

type apiEnvelope struct {
	Success bool          `json:"success"`
	Data    IssueResponse `json:"data"`
	Error   *struct {
		Message string `json:"message"`
	} `json:"error"`
}

func (c *Client) IssueVoucher(ctx context.Context, apiBase, apiKey string, req IssueRequest) (*IssueResponse, error) {
	base := strings.TrimRight(strings.TrimSpace(apiBase), "/")
	if base == "" {
		base = "https://api.bytepesa.co.ke"
	}
	if req.Quantity <= 0 {
		req.Quantity = 1
	}
	body, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, base+"/partner/vouchers", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+strings.TrimSpace(apiKey))
	httpReq.Header.Set("X-Partner-Key", strings.TrimSpace(apiKey))

	res, err := c.HTTP.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("bytepesa partner request failed: %w", err)
	}
	defer res.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(res.Body, 1<<20))
	var env apiEnvelope
	if err := json.Unmarshal(raw, &env); err != nil {
		return nil, fmt.Errorf("bytepesa response decode failed (%d): %s", res.StatusCode, strings.TrimSpace(string(raw)))
	}
	if res.StatusCode >= 300 || !env.Success {
		msg := "voucher issue failed"
		if env.Error != nil && env.Error.Message != "" {
			msg = env.Error.Message
		}
		return nil, fmt.Errorf("%s", msg)
	}
	return &env.Data, nil
}
