package whatsapp

import (
	"regexp"
	"strconv"
	"strings"
)

type InboundIntent int

const (
	IntentUnknown InboundIntent = iota
	IntentApprove
	IntentReject
	IntentQuote
	IntentDeclinePart
	IntentHelp
)

type ParsedInbound struct {
	Intent   InboundIntent
	Amount   float64
	JobCode  string
	Raw      string
}

var (
	reQuote   = regexp.MustCompile(`(?i)^\s*(?:quote|bei|price)\s*[:=]?\s*([\d,.]+)\s*(.*)$`)
	reJobCode = regexp.MustCompile(`(?i)\b(JOB[- ]?\d+)\b`)
	reYes     = regexp.MustCompile(`(?i)^\s*(yes|y|ok|okay|approve|confirm|ndio|sawa|proceed)\b`)
	reNo      = regexp.MustCompile(`(?i)^\s*(no|n|reject|decline|cancel|hapana)\b`)
	reHelp    = regexp.MustCompile(`(?i)^\s*(help|\?|menu)\s*$`)
)

// ParseInbound turns a free-text WhatsApp reply into an actionable intent.
func ParseInbound(text string) ParsedInbound {
	raw := strings.TrimSpace(text)
	out := ParsedInbound{Raw: raw, Intent: IntentUnknown}
	if raw == "" {
		return out
	}
	if m := reJobCode.FindStringSubmatch(raw); len(m) > 1 {
		out.JobCode = strings.ToUpper(strings.ReplaceAll(m[1], " ", ""))
		if !strings.Contains(out.JobCode, "-") && strings.HasPrefix(out.JobCode, "JOB") {
			out.JobCode = "JOB-" + strings.TrimPrefix(out.JobCode, "JOB")
		}
	}
	if reHelp.MatchString(raw) {
		out.Intent = IntentHelp
		return out
	}
	if m := reQuote.FindStringSubmatch(raw); len(m) > 1 {
		amt := strings.ReplaceAll(m[1], ",", "")
		if v, err := strconv.ParseFloat(amt, 64); err == nil {
			out.Intent = IntentQuote
			out.Amount = v
			return out
		}
	}
	// Bare number like "4500" treated as quote when not yes/no.
	if amt, err := strconv.ParseFloat(strings.ReplaceAll(raw, ",", ""), 64); err == nil && amt >= 0 {
		out.Intent = IntentQuote
		out.Amount = amt
		return out
	}
	lower := strings.ToLower(raw)
	if strings.HasPrefix(lower, "decline") || strings.HasPrefix(lower, "reject part") {
		out.Intent = IntentDeclinePart
		return out
	}
	if reYes.MatchString(raw) {
		out.Intent = IntentApprove
		return out
	}
	if reNo.MatchString(raw) {
		// Prefer estimate reject; supplier decline uses DECLINE keyword or pending part_quote + NO.
		out.Intent = IntentReject
		return out
	}
	return out
}
