package repair

import (
	"strings"
	"testing"
)

func TestGeneratePickupCode(t *testing.T) {
	code, err := generatePickupCode()
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(code, "PK-") || len(code) != 9 {
		t.Fatalf("unexpected code shape: %q", code)
	}
	payload := PickupQRPayload(code)
	if !strings.HasPrefix(payload, "techlane://repair-pickup/PK-") {
		t.Fatalf("unexpected qr payload: %q", payload)
	}
}

func TestNormalizePickupCode(t *testing.T) {
	cases := map[string]string{
		"pk-ab12cd":                           "PK-AB12CD",
		"techlane://repair-pickup/pk-ab12cd":  "PK-AB12CD",
		"TECHLANE:REPAIR-PICKUP:PK-ZZ99YY":    "PK-ZZ99YY",
		"  PK-GRAXNF  ":                       "PK-GRAXNF",
		"noise around PK-9D2XFH trailing":     "PK-9D2XFH",
		"":                                    "",
	}
	for in, want := range cases {
		if got := NormalizePickupCode(in); got != want {
			t.Fatalf("NormalizePickupCode(%q)=%q want %q", in, got, want)
		}
	}
}
