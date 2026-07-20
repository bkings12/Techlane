package repair

import (
	"context"
	"fmt"
	"sync"
)

// SMSSender delivers OTP codes and async repair notifications to a phone number.
type SMSSender interface {
	SendOTP(ctx context.Context, phoneE164, code string) error
	SendMessage(ctx context.Context, phoneE164, message string) error
}

// DevSMSSender retains the last code in memory for tests only — it never logs the OTP.
type DevSMSSender struct {
	mu        sync.Mutex
	LastPhone string
	LastCode  string
}

func NewDevSMSSender() *DevSMSSender {
	return &DevSMSSender{}
}

func (d *DevSMSSender) SendOTP(_ context.Context, phoneE164, code string) error {
	return d.SendMessage(nil, phoneE164, code)
}

func (d *DevSMSSender) SendMessage(_ context.Context, phoneE164, message string) error {
	d.mu.Lock()
	d.LastPhone = phoneE164
	d.LastCode = message
	d.mu.Unlock()
	return nil
}

func (d *DevSMSSender) Last() (phone, code string) {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.LastPhone, d.LastCode
}

// NoopSMSSender rejects sends when no provider is configured.
type NoopSMSSender struct{}

func (NoopSMSSender) SendOTP(_ context.Context, _, _ string) error {
	return fmt.Errorf("SMS provider not configured — set BlessedTexts under Settings → SMS (OTP)")
}

func (NoopSMSSender) SendMessage(_ context.Context, _, _ string) error {
	return fmt.Errorf("SMS provider not configured — set BlessedTexts under Settings → SMS")
}
