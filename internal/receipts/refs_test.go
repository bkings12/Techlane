package receipts

import "testing"

func TestParseMpesaTransactionDate(t *testing.T) {
	got := ParseMpesaTransactionDate("20240501143022")
	if got == nil {
		t.Fatal("expected parsed time")
	}
	if got.Year() != 2024 || got.Month() != 5 || got.Day() != 1 {
		t.Fatalf("date=%v", got)
	}
	if got.Hour() != 14 || got.Minute() != 30 {
		t.Fatalf("time=%v", got)
	}
}

func TestMpesaPhoneAndTimeFromSTKCallback(t *testing.T) {
	raw := `{
	  "Body": {
	    "stkCallback": {
	      "CallbackMetadata": {
	        "Item": [
	          {"Name": "PhoneNumber", "Value": 254712345678},
	          {"Name": "TransactionDate", "Value": 20240501143022},
	          {"Name": "MpesaReceiptNumber", "Value": "ABC123XYZ"}
	        ]
	      }
	    }
	  }
	}`
	if got := MpesaPhoneFromSTKCallback(raw); got != "254712345678" {
		t.Fatalf("phone=%q", got)
	}
	if got := MpesaReceiptFromSTKCallback(raw); got != "ABC123XYZ" {
		t.Fatalf("receipt=%q", got)
	}
	if got := MpesaTransactionTimeFromSTKCallback(raw); got == nil || got.Day() != 1 {
		t.Fatalf("tx time=%v", got)
	}
}
