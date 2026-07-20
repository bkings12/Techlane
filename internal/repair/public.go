package repair

import (
	"strings"
	"unicode/utf8"
)

const publicNoteMaxLen = 80

// PublicDeviceView strips identifiers that should not appear on public status pages.
func PublicDeviceView(d *Device) map[string]any {
	if d == nil {
		return nil
	}
	out := map[string]any{
		"id":        d.ID,
		"kind":      d.Kind,
		"anonymous": d.Anonymous,
	}
	if d.Brand != nil {
		out["brand"] = *d.Brand
	}
	if d.Model != nil {
		out["model"] = *d.Model
	}
	return out
}

// IsPublicStatusNote returns true for empty notes or short customer-safe notes.
func IsPublicStatusNote(note *string) bool {
	if note == nil {
		return true
	}
	trimmed := strings.TrimSpace(*note)
	if trimmed == "" {
		return true
	}
	if utf8.RuneCountInString(trimmed) > publicNoteMaxLen {
		return false
	}
	lower := strings.ToLower(trimmed)
	for _, bad := range []string{"internal", "supplier", "auth code", "cost", "margin", "imei", "serial"} {
		if strings.Contains(lower, bad) {
			return false
		}
	}
	return true
}

// PublicTimelineView returns status events without staff actor IDs and with filtered notes.
func PublicTimelineView(events []StatusEvent) []map[string]any {
	out := make([]map[string]any, 0, len(events))
	for _, e := range events {
		item := map[string]any{
			"status": e.Status,
			"at":     e.At,
		}
		if IsPublicStatusNote(e.Note) && e.Note != nil && strings.TrimSpace(*e.Note) != "" {
			item["note"] = strings.TrimSpace(*e.Note)
		}
		out = append(out, item)
	}
	return out
}
