package whatsapp

import "testing"

func TestParseInbound(t *testing.T) {
	cases := []struct {
		in     string
		intent InboundIntent
		amount float64
	}{
		{"YES", IntentApprove, 0},
		{"yes please", IntentApprove, 0},
		{"NO", IntentReject, 0},
		{"QUOTE 4500", IntentQuote, 4500},
		{"quote: 2,500", IntentQuote, 2500},
		{"4500", IntentQuote, 4500},
		{"DECLINE", IntentDeclinePart, 0},
		{"HELP", IntentHelp, 0},
		{"hello", IntentUnknown, 0},
	}
	for _, tc := range cases {
		got := ParseInbound(tc.in)
		if got.Intent != tc.intent {
			t.Fatalf("%q intent=%v want %v", tc.in, got.Intent, tc.intent)
		}
		if tc.intent == IntentQuote && got.Amount != tc.amount {
			t.Fatalf("%q amount=%v want %v", tc.in, got.Amount, tc.amount)
		}
	}
}
