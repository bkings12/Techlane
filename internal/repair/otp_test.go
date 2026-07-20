package repair

import (
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestCanRequestOTPRateLimit(t *testing.T) {
	now := time.Date(2026, 7, 18, 12, 0, 0, 0, time.UTC)
	recent := []OTPChallengeMeta{{CreatedAt: now.Add(-30 * time.Second)}}
	if err := CanRequestOTP(recent, now); err == nil {
		t.Fatal("expected cooldown error")
	}

	var hour []OTPChallengeMeta
	for i := 0; i < otpMaxPerHour; i++ {
		hour = append(hour, OTPChallengeMeta{CreatedAt: now.Add(-time.Duration(i+2) * time.Minute)})
	}
	if err := CanRequestOTP(hour, now); err == nil {
		t.Fatal("expected hourly limit error")
	}

	ok := []OTPChallengeMeta{{CreatedAt: now.Add(-2 * time.Minute)}}
	if err := CanRequestOTP(ok, now); err != nil {
		t.Fatalf("unexpected: %v", err)
	}
}

func TestCanVerifyOTPExpiryAndAttempts(t *testing.T) {
	now := time.Now().UTC()
	expired := OTPChallengeMeta{ExpiresAt: now.Add(-time.Minute)}
	if err := CanVerifyOTP(expired, now); err == nil {
		t.Fatal("expected expiry error")
	}
	consumed := now.Add(-time.Second)
	used := OTPChallengeMeta{ExpiresAt: now.Add(time.Minute), ConsumedAt: &consumed}
	if err := CanVerifyOTP(used, now); err == nil {
		t.Fatal("expected consumed error")
	}
	tooMany := OTPChallengeMeta{ExpiresAt: now.Add(time.Minute), Attempts: otpMaxAttempts}
	if err := CanVerifyOTP(tooMany, now); err == nil {
		t.Fatal("expected attempts error")
	}
	ok := OTPChallengeMeta{ExpiresAt: now.Add(time.Minute), Attempts: 1}
	if err := CanVerifyOTP(ok, now); err != nil {
		t.Fatalf("unexpected: %v", err)
	}
}

func TestCanDecideEstimate(t *testing.T) {
	now := time.Now().UTC()
	exp := now.Add(time.Hour)
	if err := CanDecideEstimate(EstimatePending, &exp, now); err != nil {
		t.Fatal(err)
	}
	past := now.Add(-time.Minute)
	if err := CanDecideEstimate(EstimatePending, &past, now); err == nil {
		t.Fatal("expected expired")
	}
	if err := CanDecideEstimate(EstimateApproved, &exp, now); err == nil {
		t.Fatal("expected non-pending error")
	}
	if err := CanDecideEstimate(EstimateRejected, &exp, now); err == nil {
		t.Fatal("expected non-pending error")
	}
}

func TestCustomerOwnsRepair(t *testing.T) {
	owner := uuid.New()
	other := uuid.New()
	if !CustomerOwnsRepair(&owner, owner) {
		t.Fatal("owner should own")
	}
	if CustomerOwnsRepair(&owner, other) {
		t.Fatal("other must not own")
	}
	if CustomerOwnsRepair(nil, owner) {
		t.Fatal("nil customer_id must not own")
	}
}

func TestIsPublicStatusNote(t *testing.T) {
	ok := "Ready for collection"
	if !IsPublicStatusNote(&ok) {
		t.Fatal("short public note should pass")
	}
	internal := "internal cost margin note"
	if IsPublicStatusNote(&internal) {
		t.Fatal("internal-looking note should be omitted")
	}
	long := stringsRepeat("x", publicNoteMaxLen+1)
	if IsPublicStatusNote(&long) {
		t.Fatal("long note should be omitted")
	}
}

func stringsRepeat(s string, n int) string {
	out := make([]byte, 0, n*len(s))
	for i := 0; i < n; i++ {
		out = append(out, s...)
	}
	return string(out)
}

func TestPublicDeviceViewOmitsIdentifiers(t *testing.T) {
	imei := "123456789012345"
	serial := "SN-1"
	brand := "Samsung"
	d := &Device{Kind: "phone", Brand: &brand, IMEI: &imei, SerialNumber: &serial}
	view := PublicDeviceView(d)
	if _, ok := view["imei"]; ok {
		t.Fatal("imei must be omitted")
	}
	if _, ok := view["serial_number"]; ok {
		t.Fatal("serial must be omitted")
	}
	if view["brand"] != brand {
		t.Fatal("brand should remain")
	}
}
