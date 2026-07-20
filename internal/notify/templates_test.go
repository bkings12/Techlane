package notify

import "testing"

func TestRenderTemplateRepairStatusChanged(t *testing.T) {
	msg, err := RenderTemplate("repair.status_changed", map[string]any{
		"shop_name": "Acme Repairs",
		"job_code":  "TL-42",
		"status":    "in_progress",
	})
	if err != nil {
		t.Fatal(err)
	}
	if msg == "" || !contains(msg, "Acme Repairs") || !contains(msg, "TL-42") {
		t.Fatalf("unexpected message: %q", msg)
	}
}

func TestRenderTemplateUnknown(t *testing.T) {
	_, err := RenderTemplate("unknown.template", nil)
	if err == nil {
		t.Fatal("expected error")
	}
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || len(sub) == 0 || indexOf(s, sub) >= 0)
}

func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}
