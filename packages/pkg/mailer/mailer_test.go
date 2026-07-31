package mailer

import "testing"

func TestSenderAddressExtractsBareEmail(t *testing.T) {
	cases := map[string]string{
		"TechLane <no-reply@techlane.local>": "no-reply@techlane.local",
		"no-reply@techlane.local":            "no-reply@techlane.local",
	}
	for in, want := range cases {
		if got := senderAddress(in); got != want {
			t.Errorf("senderAddress(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestBuildMessageIncludesHeadersAndBody(t *testing.T) {
	msg := string(buildMessage("from@x.com", "to@y.com", "Subject line", "Hello world"))
	for _, want := range []string{"From: from@x.com", "To: to@y.com", "Subject: Subject line", "Hello world"} {
		if !contains(msg, want) {
			t.Errorf("expected message to contain %q, got:\n%s", want, msg)
		}
	}
}

func TestNoopSenderWhenNotConfigured(t *testing.T) {
	s := New(Config{})
	if err := s.Send("a@b.com", "subj", "body"); err != nil {
		t.Fatalf("noop sender should not error, got %v", err)
	}
}

func contains(haystack, needle string) bool {
	return len(haystack) >= len(needle) && indexOf(haystack, needle) >= 0
}

func indexOf(haystack, needle string) int {
	for i := 0; i+len(needle) <= len(haystack); i++ {
		if haystack[i:i+len(needle)] == needle {
			return i
		}
	}
	return -1
}
