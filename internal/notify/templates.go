package notify

import (
	"fmt"
	"regexp"
	"strings"
)

// Template meta for the owner settings UI (helpers + defaults).
type TemplateDef struct {
	Key         string   `json:"key"`
	Label       string   `json:"label"`
	Description string   `json:"description"`
	Audience    string   `json:"audience"` // customer | supplier
	Helpers     []string `json:"helpers"`
	DefaultBody string   `json:"default_body"`
}

var placeholderRe = regexp.MustCompile(`\{\{\s*([a-zA-Z0-9_]+)\s*\}\}`)

// DefaultTemplateDefs is the catalog of editable SMS templates.
// Keep bodies short enough for 1–2 SMS segments while still actionable.
func DefaultTemplateDefs() []TemplateDef {
	return []TemplateDef{
		{
			Key:         "repair.created",
			Label:       "Job recorded",
			Description: "Sent to the customer when a repair job is created at intake (customer leaving the device).",
			Audience:    "customer",
			Helpers: []string{
				"shop_name", "customer_name", "job_code", "pickup_code",
				"device_label", "problem_summary", "pricing_line",
				"labor_amount", "currency", "branch_name", "promised_by",
			},
			DefaultBody: "{{shop_name}}: Hi {{customer_name}}. We checked in your {{device_label}} as {{job_code}}. Issue: {{problem_summary}}. {{pricing_line}} Your collection code is {{pickup_code}} — keep your intake slip (QR) for pickup.",
		},
		{
			Key:         "repair.wait_bench",
			Label:       "Wait bench",
			Description: "Sent when the customer stays at the shop wait bench instead of leaving the device.",
			Audience:    "customer",
			Helpers: []string{
				"shop_name", "customer_name", "job_code", "device_label",
				"problem_summary", "pricing_line", "wait_minutes", "wait_line",
				"labor_amount", "currency", "branch_name", "promised_by",
			},
			DefaultBody: "{{shop_name}}: Hi {{customer_name}}. Your {{device_label}} ({{job_code}}) is being looked at now. {{wait_line}} Issue: {{problem_summary}}. {{pricing_line}} We'll call you when ready.",
		},
		{
			Key:         "part_request.created",
			Label:       "Part requested",
			Description: "Sent to the assigned supplier when a part is requested on a job.",
			Audience:    "supplier",
			Helpers: []string{
				"shop_name", "job_code", "description", "quantity",
				"supplier_name", "device_label", "branch_name",
			},
			DefaultBody: "{{shop_name}}: part needed for {{job_code}} ({{device_label}}) — {{description}} x{{quantity}}. Please quote in the supplier portal.",
		},
		{
			Key:         "repair.status_changed",
			Label:       "Status update",
			Description: "Sent sparingly for customer-relevant status changes (e.g. waiting for parts). Bench hops are staff-only.",
			Audience:    "customer",
			Helpers: []string{
				"shop_name", "customer_name", "job_code", "status", "status_label",
				"device_label", "pickup_code",
			},
			DefaultBody: "{{shop_name}}: Hi {{customer_name}}. {{job_code}} ({{device_label}}) is {{status_label}}. We'll text you when it's ready for collection.",
		},
		{
			Key:         "repair.ready",
			Label:       "Ready for collection",
			Description: "Sent to the customer when the repair is completed / ready.",
			Audience:    "customer",
			Helpers: []string{
				"shop_name", "customer_name", "job_code", "pickup_code",
				"device_label", "balance", "currency", "branch_name",
				"branch_location", "pickup_place",
			},
			DefaultBody: "{{shop_name}}: Hi {{customer_name}}. Your {{device_label}} ({{job_code}}) is ready. Bring your receipt to collect your device. Balance due: {{currency}} {{balance}}. See you at {{pickup_place}}.",
		},
		{
			Key:         "repair.quick_thanks",
			Label:       "Same-day fix thanks",
			Description: "Sent after a quick / same-day counter fix — short thanks, no collection/receipt wording.",
			Audience:    "customer",
			Helpers: []string{
				"shop_name", "customer_name", "job_code", "device_label",
			},
			DefaultBody: "{{shop_name}}: Asante {{customer_name}}! Thank you for choosing {{shop_name}}. Karibu tena.",
		},
		{
			Key:         "estimate.pending",
			Label:       "Estimate ready",
			Description: "Sent to the customer when a repair estimate is created.",
			Audience:    "customer",
			Helpers: []string{
				"shop_name", "customer_name", "job_code", "device_label",
				"total_amount", "currency", "recommendation_line",
			},
			DefaultBody: "{{shop_name}}: Estimate for {{job_code}} ({{device_label}}): {{currency}} {{total_amount}}.{{recommendation_line}} Approve in the customer portal/app to start work.",
		},
		{
			Key:         "payment.confirmed",
			Label:       "Payment confirmed",
			Description: "Sent to the customer when a payment is confirmed.",
			Audience:    "customer",
			Helpers: []string{
				"shop_name", "customer_name", "job_code", "device_label",
				"amount", "balance", "currency", "method",
			},
			DefaultBody: "{{shop_name}}: We received {{currency}} {{amount}} for {{job_code}} ({{device_label}}). Remaining balance: {{currency}} {{balance}}. Asante!",
		},
		{
			Key:         "repair.collected",
			Label:       "Device collected",
			Description: "Sent to the customer after the device is handed over at the counter.",
			Audience:    "customer",
			Helpers: []string{
				"shop_name", "customer_name", "job_code", "device_label",
				"collected_by", "branch_name", "branch_location", "pickup_place",
			},
			DefaultBody: "{{shop_name}}: {{job_code}} — your {{device_label}} was collected by {{collected_by}} at {{pickup_place}}. Thank you for choosing us.",
		},
		{
			Key:         "order.placed",
			Label:       "Online order placed",
			Description: "Sent to the shop contact phone when a customer places an online storefront order.",
			Audience:    "owner",
			Helpers: []string{
				"shop_name", "customer_name", "customer_phone", "order_ref",
				"total", "currency", "fulfilment", "item_count",
				"collection_code", "delivery_address", "delivery_line",
			},
			DefaultBody: "{{shop_name}}: New online order {{order_ref}} — {{currency}} {{total}} ({{fulfilment}}, {{item_count}} item(s)) from {{customer_name}} {{customer_phone}}. {{delivery_line}}",
		},
	}
}

// DefaultWhatsAppBody returns interactive copy for WhatsApp when the shop has
// not customized the SMS template body.
func DefaultWhatsAppBody(key string) (string, bool) {
	switch key {
	case "estimate.pending":
		return "{{shop_name}}: Hi {{customer_name}}. Estimate for {{job_code}} ({{device_label}}): {{currency}} {{total_amount}}.{{recommendation_line}} Reply YES to approve or NO to decline.", true
	case "part_request.created":
		return "{{shop_name}}: Part needed for {{job_code}} ({{device_label}}) — {{description}} x{{quantity}}. Reply QUOTE 2500 (your price) or DECLINE.", true
	case "repair.created":
		return "{{shop_name}}: Hi {{customer_name}}. We checked in your {{device_label}} as {{job_code}}. Issue: {{problem_summary}}. {{pricing_line}} Collection code: {{pickup_code}}.", true
	case "repair.wait_bench":
		return "{{shop_name}}: Hi {{customer_name}}. Your {{device_label}} ({{job_code}}) is being looked at now. {{wait_line}} Issue: {{problem_summary}}. {{pricing_line}} We'll call you when ready.", true
	case "repair.ready":
		return "{{shop_name}}: Hi {{customer_name}}. Your {{device_label}} ({{job_code}}) is ready. Bring your receipt to collect. Balance due: {{currency}} {{balance}}. See you at {{pickup_place}}.", true
	case "repair.quick_thanks":
		return "{{shop_name}}: Asante {{customer_name}}! Thank you for choosing {{shop_name}}. Karibu tena.", true
	case "payment.confirmed":
		return "{{shop_name}}: We received {{currency}} {{amount}} for {{job_code}}. Remaining balance: {{currency}} {{balance}}. Asante!", true
	case "repair.collected":
		return "{{shop_name}}: {{job_code}} — your {{device_label}} was collected by {{collected_by}} at {{pickup_place}}. Thank you!", true
	case "repair.status_changed":
		return "{{shop_name}}: Hi {{customer_name}}. {{job_code}} ({{device_label}}) is {{status_label}}. We'll message you when it's ready.", true
	case "order.placed":
		return "{{shop_name}}: New online order {{order_ref}} — {{currency}} {{total}} ({{fulfilment}}, {{item_count}} item(s)) from {{customer_name}} {{customer_phone}}. {{delivery_line}}", true
	default:
		return "", false
	}
}

func defaultBodyFor(key string) (string, bool) {
	for _, d := range DefaultTemplateDefs() {
		if d.Key == key {
			return d.DefaultBody, true
		}
	}
	return "", false
}

// ApplyTemplate replaces {{helper}} placeholders in body with payload values.
func ApplyTemplate(body string, payload map[string]any) string {
	return placeholderRe.ReplaceAllStringFunc(body, func(match string) string {
		sub := placeholderRe.FindStringSubmatch(match)
		if len(sub) < 2 {
			return match
		}
		key := sub[1]
		fallback := fallbackFor(key)
		val := str(payload, key, fallback)
		switch key {
		case "status":
			val = strings.ReplaceAll(val, "_", " ")
		case "status_label":
			if raw := str(payload, "status_label", ""); raw != "" && raw != fallback {
				val = raw
			} else {
				val = PrettyStatus(str(payload, "status", fallback))
			}
		}
		return val
	})
}

func fallbackFor(key string) string {
	switch key {
	case "currency":
		return "KES"
	case "status", "status_label":
		return "updated"
	case "shop_name":
		return "TechLane"
	case "job_code":
		return "job"
	case "pickup_code":
		return "your slip"
	case "customer_name":
		return "there"
	case "device_label":
		return "device"
	case "problem_summary":
		return "repair"
	case "pricing_line":
		return "We'll confirm the price after diagnosis."
	case "branch_name":
		return "the shop"
	case "branch_location":
		return ""
	case "pickup_place":
		return "the shop"
	case "promised_by":
		return ""
	case "wait_minutes":
		return ""
	case "wait_line":
		return "Please wait at the wait bench."
	case "recommendation_line":
		return ""
	case "collected_by":
		return "the customer"
	case "method":
		return "payment"
	case "labor_amount", "parts_amount", "total_amount", "amount", "balance", "quantity":
		return "0"
	default:
		return ""
	}
}

// PrettyStatus turns machine statuses into customer-friendly wording.
func PrettyStatus(status string) string {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "intake":
		return "checked in"
	case "diagnosed":
		return "diagnosed"
	case "waiting_parts":
		return "waiting for a part"
	case "in_progress":
		return "being repaired"
	case "ready_for_pickup":
		return "passed quality check"
	case "completed":
		return "ready for collection"
	case "collected":
		return "collected"
	case "cancelled":
		return "cancelled"
	case "unrepairable":
		return "closed as unrepairable"
	default:
		return strings.ReplaceAll(status, "_", " ")
	}
}

// CustomerSMSOnStatus reports whether a status change should text the customer.
// Bench hops (diagnose → repair → QC) stay on the staff board only so customers
// are not flooded with near-identical "is now …" messages out of order.
func CustomerSMSOnStatus(status string) bool {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "waiting_parts":
		return true
	case "cancelled", "unrepairable":
		return true
	default:
		return false
	}
}

// RenderTemplate renders using the built-in default body for the key.
func RenderTemplate(templateKey string, payload map[string]any) (string, error) {
	body, ok := defaultBodyFor(templateKey)
	if !ok {
		return "", fmt.Errorf("unknown template %q", templateKey)
	}
	return ApplyTemplate(body, payload), nil
}

// RenderTemplateWithOverride uses customBody when non-empty, otherwise the default.
func RenderTemplateWithOverride(templateKey, customBody string, payload map[string]any) (string, error) {
	body := strings.TrimSpace(customBody)
	if body == "" {
		return RenderTemplate(templateKey, payload)
	}
	if _, ok := defaultBodyFor(templateKey); !ok {
		return "", fmt.Errorf("unknown template %q", templateKey)
	}
	return ApplyTemplate(body, payload), nil
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
		if t == float64(int64(t)) {
			return fmt.Sprintf("%.0f", t)
		}
		return fmt.Sprintf("%.2f", t)
	case int:
		return fmt.Sprintf("%d", t)
	case int64:
		return fmt.Sprintf("%d", t)
	default:
		return fmt.Sprint(v)
	}
}
