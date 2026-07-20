package repair

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestBlessedTextsSMSSenderSuccess(t *testing.T) {
	var got map[string]string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/sendsms" {
			t.Fatalf("path %s", r.URL.Path)
		}
		if err := json.NewDecoder(r.Body).Decode(&got); err != nil {
			t.Fatal(err)
		}
		_ = json.NewEncoder(w).Encode([]map[string]string{{
			"status_code": "1000",
			"status_desc": "Success",
			"message_id":  "abc",
			"phone":       "254721000000",
		}})
	}))
	defer srv.Close()

	s := NewBlessedTextsSMSSender("key", "23107", srv.URL)
	if err := s.SendOTP(context.Background(), "254721000000", "123456"); err != nil {
		t.Fatal(err)
	}
	if got["api_key"] != "key" || got["sender_id"] != "23107" || got["phone"] != "254721000000" {
		t.Fatalf("unexpected body %#v", got)
	}
	if !strings.Contains(got["message"], "123456") {
		t.Fatalf("message missing code: %q", got["message"])
	}
}

func TestBlessedTextsSMSSenderErrorCode(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode([]map[string]string{{
			"status_code": "1009",
			"status_desc": "Low bulk credits",
			"phone":       "254721000000",
		}})
	}))
	defer srv.Close()

	s := NewBlessedTextsSMSSender("key", "23107", srv.URL)
	err := s.SendOTP(context.Background(), "254721000000", "999999")
	if err == nil || !strings.Contains(err.Error(), "1009") {
		t.Fatalf("expected low-credit error, got %v", err)
	}
}

func TestInterpretBlessedTextsSendResponseObjectError(t *testing.T) {
	err := interpretBlessedTextsSendResponse([]byte(`{"status_code":"1002","status_desc":"Invalid API Key"}`))
	if err == nil || !strings.Contains(err.Error(), "1002") {
		t.Fatalf("got %v", err)
	}
}
