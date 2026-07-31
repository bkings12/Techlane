package payments

import "testing"

func TestCustomerFacingPaymentRefPrefersMpesaReceipt(t *testing.T) {
	got := CustomerFacingPaymentRef("ws_CO_123456789", "SJ42KD9QM1")
	if got != "SJ42KD9QM1" {
		t.Fatalf("got %q", got)
	}
	if CustomerFacingPaymentRef("ws_CO_123456789") != "" {
		t.Fatalf("checkout id alone should be hidden on receipts")
	}
	if CustomerFacingPaymentRef("", "QH7X2ABCDE") != "QH7X2ABCDE" {
		t.Fatalf("expected mpesa receipt")
	}
}

func TestExtractMpesaReceiptFromCallback(t *testing.T) {
	raw := []byte(`{
	  "Body": {
	    "stkCallback": {
	      "CheckoutRequestID": "ws_CO_ABC",
	      "ResultCode": 0,
	      "CallbackMetadata": {
	        "Item": [
	          {"Name": "Amount", "Value": 100},
	          {"Name": "MpesaReceiptNumber", "Value": "SJ42KD9QM1"},
	          {"Name": "PhoneNumber", "Value": 254712345678}
	        ]
	      }
	    }
	  }
	}`)
	if got := ExtractMpesaReceiptFromCallback(raw); got != "SJ42KD9QM1" {
		t.Fatalf("got %q", got)
	}
}
