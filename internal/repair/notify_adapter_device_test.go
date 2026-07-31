package repair

import "testing"

func TestDeviceLabelSkipsKindDot(t *testing.T) {
	brandDot := "."
	model := "iPhone 13 Pro"
	got := deviceLabel(&Device{Kind: "phone", Brand: &brandDot, Model: &model})
	if got != "iPhone 13 Pro" {
		t.Fatalf("got %q", got)
	}
	brand := "Apple"
	got = deviceLabel(&Device{Kind: "phone", Brand: &brand, Model: &model})
	if got != "Apple iPhone 13 Pro" {
		t.Fatalf("got %q", got)
	}
}
