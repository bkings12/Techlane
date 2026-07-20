package commerce

import "testing"

func TestRandomCodeLength(t *testing.T) {
	code, err := randomCode(6)
	if err != nil {
		t.Fatal(err)
	}
	if len(code) != 6 {
		t.Fatalf("got %q", code)
	}
}
