package repair

import "testing"

func TestNormalizePhone(t *testing.T) {
	cases := []struct {
		in      string
		want    string
		wantErr bool
	}{
		{"0712 345 678", "254712345678", false},
		{"+254712345678", "254712345678", false},
		{"254712345678", "254712345678", false},
		{"712345678", "254712345678", false},
		{"", "", true},
		{"abc", "", true},
		{"123", "", true},
	}
	for _, tc := range cases {
		got, err := NormalizePhone(tc.in)
		if tc.wantErr {
			if err == nil {
				t.Errorf("NormalizePhone(%q) expected error", tc.in)
			}
			continue
		}
		if err != nil {
			t.Errorf("NormalizePhone(%q): %v", tc.in, err)
			continue
		}
		if got != tc.want {
			t.Errorf("NormalizePhone(%q)=%q want %q", tc.in, got, tc.want)
		}
	}
}

func TestPhoneMatchVariants(t *testing.T) {
	got := PhoneMatchVariants("0712345678")
	want := map[string]bool{"0712345678": true, "254712345678": true, "712345678": true}
	if len(got) != len(want) {
		t.Fatalf("got %v", got)
	}
	for _, v := range got {
		if !want[v] {
			t.Fatalf("unexpected variant %q in %v", v, got)
		}
	}
	got2 := PhoneMatchVariants("254712345678")
	for _, need := range []string{"254712345678", "0712345678", "712345678"} {
		found := false
		for _, v := range got2 {
			if v == need {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("missing %q in %v", need, got2)
		}
	}
}

func TestDevSMSSenderStoresLast(t *testing.T) {
	s := NewDevSMSSender()
	if err := s.SendOTP(nil, "254712345678", "123456"); err != nil {
		t.Fatal(err)
	}
	phone, code := s.Last()
	if phone != "254712345678" || code != "123456" {
		t.Fatalf("got %s %s", phone, code)
	}
}
