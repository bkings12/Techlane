package notify

import (
	"strings"
	"testing"
)

func TestRenderTemplateRepairStatusChanged(t *testing.T) {
	msg, err := RenderTemplate("repair.status_changed", map[string]any{
		"shop_name":     "Acme Repairs",
		"customer_name": "Asha",
		"job_code":      "JOB-42",
		"device_label":  "Samsung A14",
		"status":        "waiting_parts",
	})
	if err != nil {
		t.Fatal(err)
	}
	if !contains(msg, "Acme Repairs") || !contains(msg, "JOB-42") || !contains(msg, "waiting for a part") {
		t.Fatalf("unexpected message: %q", msg)
	}
}

func TestRenderTemplateRepairCreated(t *testing.T) {
	msg, err := RenderTemplate("repair.created", map[string]any{
		"shop_name":       "Acme",
		"customer_name":   "Bryan",
		"job_code":        "JOB-1",
		"pickup_code":     "PK-AB12CD",
		"device_label":    "phone · iPhone 12",
		"labor_amount":    5000,
		"problem_summary": "cracked screen",
		"currency":        "KES",
		"pricing_line":    "Quoted KES 5000.",
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"JOB-1", "PK-AB12CD", "cracked screen", "iPhone 12", "Quoted KES 5000"} {
		if !contains(msg, want) {
			t.Fatalf("missing %q in %q", want, msg)
		}
	}
}

func TestRenderTemplateWaitBench(t *testing.T) {
	msg, err := RenderTemplate("repair.wait_bench", map[string]any{
		"shop_name":       "Acme",
		"customer_name":   "Asha",
		"job_code":        "JOB-7",
		"device_label":    "Samsung A14",
		"problem_summary": "won't charge",
		"wait_minutes":    45,
		"wait_line":       "Please wait at the wait bench — about 45 minutes.",
		"pricing_line":    "Quoted KES 2500.",
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"JOB-7", "wait bench", "45 minutes", "won't charge", "Quoted KES 2500"} {
		if !contains(msg, want) {
			t.Fatalf("missing %q in %q", want, msg)
		}
	}
}

func TestRenderTemplateRepairReadyIncludesBalance(t *testing.T) {
	msg, err := RenderTemplate("repair.ready", map[string]any{
		"shop_name":     "Acme",
		"customer_name": "Asha",
		"job_code":      "JOB-9",
		"pickup_code":   "PK-ZZ99YY",
		"device_label":  "laptop · HP",
		"balance":       1500,
		"currency":      "KES",
		"branch_name":   "Westlands",
		"pickup_place":  "TechLane & Westlands",
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"JOB-9", "receipt", "1500", "TechLane & Westlands"} {
		if !contains(msg, want) {
			t.Fatalf("missing %q in %q", want, msg)
		}
	}
}

func TestRenderTemplateWithOverride(t *testing.T) {
	msg, err := RenderTemplateWithOverride("repair.created", "Hi {{job_code}} from {{shop_name}}", map[string]any{
		"job_code":  "TL-9",
		"shop_name": "Shop",
	})
	if err != nil {
		t.Fatal(err)
	}
	if msg != "Hi TL-9 from Shop" {
		t.Fatalf("got %q", msg)
	}
}

func TestRenderTemplateUnknown(t *testing.T) {
	_, err := RenderTemplate("unknown.template", nil)
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestPrettyStatus(t *testing.T) {
	if PrettyStatus("waiting_parts") != "waiting for a part" {
		t.Fatal(PrettyStatus("waiting_parts"))
	}
	if PrettyStatus("ready_for_pickup") != "passed quality check" {
		t.Fatal(PrettyStatus("ready_for_pickup"))
	}
	if PrettyStatus("completed") != "ready for collection" {
		t.Fatal(PrettyStatus("completed"))
	}
}

func TestCustomerSMSOnStatus(t *testing.T) {
	if CustomerSMSOnStatus("diagnosed") || CustomerSMSOnStatus("in_progress") || CustomerSMSOnStatus("ready_for_pickup") {
		t.Fatal("bench hops must not SMS the customer")
	}
	if !CustomerSMSOnStatus("waiting_parts") {
		t.Fatal("waiting_parts should SMS the customer")
	}
	if !CustomerSMSOnStatus("cancelled") {
		t.Fatal("cancelled should SMS the customer")
	}
}

func contains(s, sub string) bool {
	return strings.Contains(s, sub)
}
