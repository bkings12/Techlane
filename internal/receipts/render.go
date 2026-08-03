package receipts

import (
	"context"

	"github.com/google/uuid"
)

// Render loads the tenant's branding and settings, stamps a receipt number, and
// returns printable HTML. docID keys the receipt number series so reprints of
// the same job or sale keep their original serial.
func (s *Service) Render(ctx context.Context, tenantID uuid.UUID, doc Document, docID uuid.UUID, paper string) (string, error) {
	set, err := s.GetSettings(ctx, tenantID)
	if err != nil {
		return "", err
	}
	shop := s.LoadShop(ctx, tenantID, set)
	if doc.Currency == "" {
		doc.Currency = s.Currency(ctx, tenantID)
	}
	if doc.Number == "" && docID != uuid.Nil {
		doc.Number = s.ReceiptNumber(ctx, tenantID, doc.Kind, docID)
	}
	return RenderHTML(shop, doc, set, paper), nil
}

// RenderESCPOS returns a raw ESC/POS byte stream for USB thermal printers.
func (s *Service) RenderESCPOS(ctx context.Context, tenantID uuid.UUID, doc Document, docID uuid.UUID) ([]byte, error) {
	set, err := s.GetSettings(ctx, tenantID)
	if err != nil {
		return nil, err
	}
	shop := s.LoadShop(ctx, tenantID, set)
	if doc.Currency == "" {
		doc.Currency = s.Currency(ctx, tenantID)
	}
	if doc.Number == "" && docID != uuid.Nil {
		doc.Number = s.ReceiptNumber(ctx, tenantID, doc.Kind, docID)
	}
	return BuildESCPOS(shop, doc, set), nil
}

// RenderPDF is the A4 PDF companion to Render.
func (s *Service) RenderPDF(ctx context.Context, tenantID uuid.UUID, doc Document, docID uuid.UUID) ([]byte, error) {
	set, err := s.GetSettings(ctx, tenantID)
	if err != nil {
		return nil, err
	}
	shop := s.LoadShop(ctx, tenantID, set)
	if doc.Currency == "" {
		doc.Currency = s.Currency(ctx, tenantID)
	}
	if doc.Number == "" && docID != uuid.Nil {
		doc.Number = s.ReceiptNumber(ctx, tenantID, doc.Kind, docID)
	}
	doc.applySettings(set)
	return RenderPDF(shop, doc, set)
}
