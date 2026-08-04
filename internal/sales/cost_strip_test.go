package sales

import (
	"testing"

	"github.com/google/uuid"
)

// stripItemCosts is unexported, so this lives in-package (unlike the rest of
// this package's tests) — it's the one function the cashier-role permission
// gate in handler.go actually depends on.
func TestStripItemCosts(t *testing.T) {
	cost := 1800.0
	margin := 1000.0
	supplier := uuid.New()
	items := []SaleItem{
		{Description: "Oraimo Charger", UnitPrice: 2800, LineTotal: 2800, UnitCost: &cost, Margin: &margin, SupplierID: &supplier},
	}
	stripped := stripItemCosts(items)
	if stripped[0].UnitCost != nil {
		t.Errorf("unit_cost must be nil after stripping, got %v", *stripped[0].UnitCost)
	}
	if stripped[0].Margin != nil {
		t.Errorf("margin must be nil after stripping, got %v", *stripped[0].Margin)
	}
	if stripped[0].SupplierID != nil {
		t.Errorf("supplier_id must be nil after stripping, got %v", *stripped[0].SupplierID)
	}
	if stripped[0].UnitPrice != 2800 || stripped[0].LineTotal != 2800 {
		t.Errorf("customer-facing price fields must survive stripping, got %+v", stripped[0])
	}
	// Original slice must be untouched — a reports.read caller needs the real data.
	if items[0].UnitCost == nil || *items[0].UnitCost != 1800 {
		t.Errorf("stripItemCosts must not mutate its input")
	}
}
