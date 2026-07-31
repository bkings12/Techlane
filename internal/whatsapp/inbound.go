package whatsapp

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/uuid"
)

type InboundMessage struct {
	TenantID string `json:"tenantId"`
	Phone    string `json:"phone"`
	Text     string `json:"text"`
	PushName string `json:"pushName"`
}

// HandleInbound processes a customer/supplier WhatsApp reply and returns an
// acknowledgement message to send back (empty = no reply).
// Unrecognised chat is left alone so the shop phone can still be used for normal conversation.
func (s *Service) HandleInbound(ctx context.Context, msg InboundMessage) (reply string, err error) {
	tenantID, err := uuid.Parse(strings.TrimSpace(msg.TenantID))
	if err != nil {
		return "", fmt.Errorf("invalid tenantId")
	}
	phone := strings.TrimSpace(msg.Phone)
	text := strings.TrimSpace(msg.Text)
	if phone == "" || text == "" {
		return "", nil
	}

	parsed := ParseInbound(text)
	switch parsed.Intent {
	case IntentHelp:
		return "TechLane commands: YES / NO for estimates, QUOTE 2500 or DECLINE for parts. Other messages are ignored so you can chat normally.", nil

	case IntentApprove:
		if s.repair == nil {
			return "", nil
		}
		var estimateID *uuid.UUID
		pend, pErr := s.LatestPending(ctx, tenantID, phone, ActionEstimateDecide)
		if pErr != nil || pend == nil {
			// No open estimate ask — treat as normal chat (e.g. "ok", "sawa").
			return "", nil
		}
		estimateID = &pend.RefID
		jobCode, dErr := s.repair.DecideEstimateByPhone(ctx, tenantID, phone, true, estimateID)
		if dErr != nil {
			return fmt.Sprintf("Could not approve: %s", dErr.Error()), nil
		}
		s.ClearPending(ctx, tenantID, *estimateID)
		if jobCode == "" {
			jobCode = "your repair"
		}
		return fmt.Sprintf("Asante. Estimate for %s is approved — we'll start the work.", jobCode), nil

	case IntentReject:
		// If latest pending is a part quote, treat NO as supplier decline.
		if pend, pErr := s.LatestPending(ctx, tenantID, phone, ActionPartQuote); pErr == nil && pend != nil && s.inv != nil {
			jobCode, dErr := s.inv.DeclineSupplierRequestByPhone(ctx, tenantID, phone, &pend.RefID)
			if dErr != nil {
				return fmt.Sprintf("Could not decline: %s", dErr.Error()), nil
			}
			s.ClearPending(ctx, tenantID, pend.RefID)
			if jobCode == "" {
				jobCode = "the part request"
			}
			return fmt.Sprintf("Noted — declined %s.", jobCode), nil
		}
		if s.repair == nil {
			return "", nil
		}
		pend, pErr := s.LatestPending(ctx, tenantID, phone, ActionEstimateDecide)
		if pErr != nil || pend == nil {
			return "", nil
		}
		estimateID := &pend.RefID
		jobCode, dErr := s.repair.DecideEstimateByPhone(ctx, tenantID, phone, false, estimateID)
		if dErr != nil {
			return fmt.Sprintf("Could not decline: %s", dErr.Error()), nil
		}
		s.ClearPending(ctx, tenantID, *estimateID)
		if jobCode == "" {
			jobCode = "your repair"
		}
		return fmt.Sprintf("Okay — estimate for %s declined. Contact the shop if you want a new quote.", jobCode), nil

	case IntentQuote:
		if s.inv == nil {
			return "", nil
		}
		pend, pErr := s.LatestPending(ctx, tenantID, phone, ActionPartQuote)
		if pErr != nil || pend == nil {
			// Bare numbers / prices in normal chat — don't hijack the conversation.
			return "", nil
		}
		requestID := &pend.RefID
		jobCode, qErr := s.inv.SubmitSupplierQuoteByPhone(ctx, tenantID, phone, parsed.Amount, requestID)
		if qErr != nil {
			return fmt.Sprintf("Could not save quote: %s", qErr.Error()), nil
		}
		s.ClearPending(ctx, tenantID, *requestID)
		if jobCode == "" {
			jobCode = "the job"
		}
		return fmt.Sprintf("Quote of %.0f received for %s. Asante.", parsed.Amount, jobCode), nil

	case IntentDeclinePart:
		if s.inv == nil {
			return "", nil
		}
		pend, pErr := s.LatestPending(ctx, tenantID, phone, ActionPartQuote)
		if pErr != nil || pend == nil {
			return "", nil
		}
		requestID := &pend.RefID
		jobCode, dErr := s.inv.DeclineSupplierRequestByPhone(ctx, tenantID, phone, requestID)
		if dErr != nil {
			return fmt.Sprintf("Could not decline: %s", dErr.Error()), nil
		}
		s.ClearPending(ctx, tenantID, *requestID)
		if jobCode == "" {
			jobCode = "the part request"
		}
		return fmt.Sprintf("Noted — declined %s.", jobCode), nil

	default:
		// Free-form chat — no bot reply.
		return "", nil
	}
}
