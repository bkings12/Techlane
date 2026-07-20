package notify

import (
	"fmt"
	"strings"
)

func RenderTemplate(templateKey string, payload map[string]any) (string, error) {
	switch templateKey {
	case "repair.status_changed":
		return fmt.Sprintf("%s: repair %s is now %s.",
			str(payload, "shop_name", "TechLane"),
			str(payload, "job_code", "job"),
			strings.ReplaceAll(str(payload, "status", "updated"), "_", " "),
		), nil
	case "repair.ready":
		return fmt.Sprintf("%s: repair %s is ready for collection. Thank you.",
			str(payload, "shop_name", "TechLane"),
			str(payload, "job_code", "job"),
		), nil
	case "payment.confirmed":
		return fmt.Sprintf("%s: we received %s %s for repair %s. Thank you.",
			str(payload, "shop_name", "TechLane"),
			str(payload, "currency", "KES"),
			str(payload, "amount", "0"),
			str(payload, "job_code", "job"),
		), nil
	case "estimate.pending":
		labor := str(payload, "labor_amount", "0")
		parts := str(payload, "parts_amount", "0")
		return fmt.Sprintf("%s: estimate ready for repair %s (labor %s, parts %s). Open the customer portal to approve.",
			str(payload, "shop_name", "TechLane"),
			str(payload, "job_code", "job"),
			labor, parts,
		), nil
	default:
		return "", fmt.Errorf("unknown template %q", templateKey)
	}
}

func str(payload map[string]any, key, fallback string) string {
	if payload == nil {
		return fallback
	}
	v, ok := payload[key]
	if !ok || v == nil {
		return fallback
	}
	switch t := v.(type) {
	case string:
		if strings.TrimSpace(t) == "" {
			return fallback
		}
		return t
	case float64:
		return fmt.Sprintf("%.0f", t)
	case int:
		return fmt.Sprintf("%d", t)
	default:
		return fmt.Sprint(v)
	}
}
