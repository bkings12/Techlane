package payments

import "testing"

func TestJoinPersonName(t *testing.T) {
	got := joinPersonName(" Jane ", "", "Wanjiku", " ")
	if got != "Jane Wanjiku" {
		t.Fatalf("got %q", got)
	}
}

func TestNormalizeMSISDNLoose(t *testing.T) {
	cases := map[string]string{
		"0712345678":    "254712345678",
		"254712345678":  "254712345678",
		"+254 712 345678": "254712345678",
		"712345678":     "254712345678",
		"":              "",
	}
	for in, want := range cases {
		if got := normalizeMSISDNLoose(in); got != want {
			t.Errorf("normalizeMSISDNLoose(%q)=%q want %q", in, got, want)
		}
	}
}

func TestMetadataItemString(t *testing.T) {
	if got := metadataItemString(float64(254723925902)); got != "254723925902" {
		t.Fatalf("float phone: %q", got)
	}
	if got := metadataItemString("UH14T1NDC4"); got != "UH14T1NDC4" {
		t.Fatalf("string: %q", got)
	}
}
