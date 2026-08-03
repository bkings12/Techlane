package notify

import "testing"

func TestSplitPhoneList(t *testing.T) {
	got := SplitPhoneList("0723433660 / 0723239995 / 0726676628")
	want := []string{"254723433660", "254723239995", "254726676628"}
	if len(got) != len(want) {
		t.Fatalf("got %v want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("got %v want %v", got, want)
		}
	}
}

func TestNormalizeRecipientPhone(t *testing.T) {
	got, err := NormalizeRecipientPhone("0723 433 660")
	if err != nil || got != "254723433660" {
		t.Fatalf("got %q err %v", got, err)
	}
}
