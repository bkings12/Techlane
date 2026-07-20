package inventory

import (
	"strings"
	"testing"
)

func TestGenerateAuthCode(t *testing.T) {
	code, err := GenerateAuthCode()
	if err != nil {
		t.Fatal(err)
	}
	if len(code) != authCodeLength {
		t.Fatalf("expected length %d, got %d", authCodeLength, len(code))
	}
	for _, c := range code {
		if !strings.ContainsRune(authCodeChars, c) {
			t.Fatalf("invalid char %q in code %s", c, code)
		}
	}
}

func TestGenerateAuthCodeUnique(t *testing.T) {
	seen := map[string]struct{}{}
	for i := 0; i < 50; i++ {
		code, err := GenerateAuthCode()
		if err != nil {
			t.Fatal(err)
		}
		if _, ok := seen[code]; ok {
			t.Fatalf("duplicate auth code: %s", code)
		}
		seen[code] = struct{}{}
	}
}
