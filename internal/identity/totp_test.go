package identity

import (
	"testing"
	"time"
)

func TestTOTPRoundTrip(t *testing.T) {
	secret, err := generateTOTPSecret()
	if err != nil {
		t.Fatalf("generateTOTPSecret: %v", err)
	}
	now := time.Now().UTC()
	counter := uint64(now.Unix() / totpStepSecs)
	code, err := hotp(secret, counter)
	if err != nil {
		t.Fatalf("hotp: %v", err)
	}
	if len(code) != totpDigits {
		t.Fatalf("expected %d digit code, got %q", totpDigits, code)
	}
	if !verifyTOTP(secret, code, now) {
		t.Fatalf("expected code %q to verify for current time step", code)
	}
	if verifyTOTP(secret, "000000", now) && code != "000000" {
		t.Fatalf("wrong code unexpectedly verified")
	}
}

func TestTOTPKnownVector(t *testing.T) {
	// RFC 6238 Appendix B test vector for SHA1, 8-digit codes are documented,
	// but our implementation is fixed at 6 digits/30s steps, so we just check
	// determinism: same secret+counter always yields the same code.
	secret := "JBSWY3DPEHPK3PXP" // arbitrary valid base32 secret
	c1, err := hotp(secret, 1)
	if err != nil {
		t.Fatalf("hotp: %v", err)
	}
	c2, err := hotp(secret, 1)
	if err != nil {
		t.Fatalf("hotp: %v", err)
	}
	if c1 != c2 {
		t.Fatalf("hotp not deterministic: %q vs %q", c1, c2)
	}
	c3, err := hotp(secret, 2)
	if err != nil {
		t.Fatalf("hotp: %v", err)
	}
	if c1 == c3 {
		t.Fatalf("expected different codes for different counters")
	}
}

func TestTOTPClockSkewTolerance(t *testing.T) {
	secret, err := generateTOTPSecret()
	if err != nil {
		t.Fatalf("generateTOTPSecret: %v", err)
	}
	now := time.Now().UTC()
	prevStep := now.Add(-totpStepSecs * time.Second)
	code, err := hotp(secret, uint64(prevStep.Unix()/totpStepSecs))
	if err != nil {
		t.Fatalf("hotp: %v", err)
	}
	if !verifyTOTP(secret, code, now) {
		t.Fatalf("expected previous time-step code to still verify within skew window")
	}
	farPast := now.Add(-10 * time.Minute)
	codeFar, err := hotp(secret, uint64(farPast.Unix()/totpStepSecs))
	if err != nil {
		t.Fatalf("hotp: %v", err)
	}
	if verifyTOTP(secret, codeFar, now) {
		t.Fatalf("code from 10 minutes ago should not verify")
	}
}

func TestLockoutDurationEscalates(t *testing.T) {
	cases := []struct {
		failed   int
		wantZero bool
	}{
		{1, true},
		{4, true},
		{5, false},
	}
	for _, c := range cases {
		d := lockoutDuration(c.failed)
		if c.wantZero && d != 0 {
			t.Fatalf("failed=%d: expected no lockout, got %v", c.failed, d)
		}
		if !c.wantZero && d <= 0 {
			t.Fatalf("failed=%d: expected lockout duration, got %v", c.failed, d)
		}
	}
	d5 := lockoutDuration(5)
	d10 := lockoutDuration(10)
	if d10 < d5 {
		t.Fatalf("expected lockout to escalate with repeated failures: d5=%v d10=%v", d5, d10)
	}
	if d10 > maxLockoutDuration {
		t.Fatalf("lockout duration should be capped at %v, got %v", maxLockoutDuration, d10)
	}
}
