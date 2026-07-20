package repair

import (
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestWarrantyEndsAtDefaultDays(t *testing.T) {
	start := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	ends := warrantyEndsAt(start, DefaultWarrantyDays)
	if ends.Sub(start) != 90*24*time.Hour {
		t.Fatalf("expected 90 days, got %v", ends.Sub(start))
	}
}

func TestEffectiveWarrantyStatus(t *testing.T) {
	now := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)
	ends := now.Add(24 * time.Hour)
	if effectiveWarrantyStatus(WarrantyActive, ends, now) != WarrantyActive {
		t.Fatal("expected active")
	}
	if effectiveWarrantyStatus(WarrantyActive, now.Add(-time.Hour), now) != WarrantyExpired {
		t.Fatal("expected expired")
	}
	if effectiveWarrantyStatus(WarrantyClaimed, ends, now) != WarrantyClaimed {
		t.Fatal("claimed should remain claimed")
	}
}

func TestDefaultWarrantyDaysConstant(t *testing.T) {
	if DefaultWarrantyDays != 90 {
		t.Fatalf("expected 90 day default, got %d", DefaultWarrantyDays)
	}
}

func TestWarrantyClaimRequiresOwnership(t *testing.T) {
	owner := uuid.New()
	other := uuid.New()
	if !CustomerOwnsRepair(&owner, owner) {
		t.Fatal("owner should own repair for warranty claim")
	}
	if CustomerOwnsRepair(&owner, other) {
		t.Fatal("non-owner must not claim warranty")
	}
}
