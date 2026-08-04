import { useEffect, useMemo, useState, type FormEvent } from "react";
import { Button, Input } from "./ui";
import { SearchableCombobox, type ComboOption } from "./SearchableCombobox";
import type { StockBalance } from "../lib/api";

export type AddPartModalProps = {
  open: boolean;
  onClose: () => void;
  stock: StockBalance[];
  canSeeCost: boolean;
  currencyCode: string;
  onAddInventory: (input: { variant_id: string; location_id: string; quantity: number; unit_price?: number }) => Promise<void>;
  onAddSourced: (input: {
    description: string;
    unit_cost: number;
    unit_price: number;
    quantity: number;
    supplier_name?: string;
    supplier_ref?: string;
    expected_arrival?: string;
  }) => Promise<void>;
};

export function AddPartModal({ open, onClose, stock, canSeeCost, currencyCode, onAddInventory, onAddSourced }: AddPartModalProps) {
  const [mode, setMode] = useState<"inventory" | "sourced">("inventory");
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");

  // From-inventory fields
  const [stockKey, setStockKey] = useState("");
  const [invQty, setInvQty] = useState("1");

  // Sourced-part fields
  const [description, setDescription] = useState("");
  const [supplier, setSupplier] = useState("");
  const [supplierRef, setSupplierRef] = useState("");
  const [cost, setCost] = useState("");
  const [price, setPrice] = useState("");
  const [qty, setQty] = useState("1");
  const [arrival, setArrival] = useState("");

  useEffect(() => {
    if (!open) return;
    setMode("inventory");
    setStockKey("");
    setInvQty("1");
    setDescription("");
    setSupplier("");
    setSupplierRef("");
    setCost("");
    setPrice("");
    setQty("1");
    setArrival("");
    setError("");
    setBusy(false);
  }, [open]);

  const options: ComboOption[] = useMemo(
    () =>
      stock
        .filter((b) => b.available_qty > 0 && b.sell_price > 0)
        .map((b) => ({
          value: `${b.variant_id}:${b.location_id}`,
          label: `${b.product_name} (${b.sku})`,
          sublabel: `${currencyCode} ${b.sell_price.toFixed(0)} · ${b.available_qty} at ${b.location_name}`,
        })),
    [stock, currencyCode],
  );
  const selectedLabel = options.find((o) => o.value === stockKey)?.label ?? "";
  const selectedBalance = stock.find((b) => `${b.variant_id}:${b.location_id}` === stockKey);

  const sourcedCost = Number(cost);
  const sourcedPrice = Number(price);
  const sourcedProfit = Number.isFinite(sourcedCost) && Number.isFinite(sourcedPrice) ? sourcedPrice - sourcedCost : null;
  const sourcedMarginPct = sourcedProfit != null && sourcedPrice > 0 ? (sourcedProfit / sourcedPrice) * 100 : null;

  if (!open) return null;

  async function onSubmit(e: FormEvent) {
    e.preventDefault();
    setError("");
    if (mode === "inventory") {
      if (!selectedBalance) {
        setError("Search and select a part from inventory");
        return;
      }
      setBusy(true);
      try {
        await onAddInventory({
          variant_id: selectedBalance.variant_id,
          location_id: selectedBalance.location_id,
          quantity: Math.max(1, Number(invQty) || 1),
        });
        onClose();
      } catch (err) {
        setError(err instanceof Error ? err.message : "Could not add part");
      } finally {
        setBusy(false);
      }
      return;
    }
    const desc = description.trim();
    if (!desc) {
      setError("Enter what the part is (e.g. MacBook A2337 Display)");
      return;
    }
    if (!Number.isFinite(sourcedCost) || sourcedCost < 0 || !Number.isFinite(sourcedPrice) || sourcedPrice < 0) {
      setError("Enter a valid cost and selling price");
      return;
    }
    setBusy(true);
    try {
      await onAddSourced({
        description: desc,
        unit_cost: sourcedCost,
        unit_price: sourcedPrice,
        quantity: Math.max(1, Number(qty) || 1),
        supplier_name: supplier.trim() || undefined,
        supplier_ref: supplierRef.trim() || undefined,
        expected_arrival: arrival || undefined,
      });
      onClose();
    } catch (err) {
      setError(err instanceof Error ? err.message : "Could not add part");
    } finally {
      setBusy(false);
    }
  }

  return (
    <div className="cmdk-backdrop sheet-backdrop" role="presentation" onClick={() => !busy && onClose()}>
      <div className="cmdk-panel sheet-panel" role="dialog" aria-modal="true" aria-label="Add part" onClick={(e) => e.stopPropagation()}>
        <div className="panel-head" style={{ padding: "1rem 1.25rem 0" }}>
          <h2 style={{ margin: 0 }}>Add part</h2>
        </div>
        <div className="chip-row" style={{ padding: "0.75rem 1.25rem 0" }}>
          <button type="button" className={mode === "inventory" ? "chip chip-active" : "chip"} onClick={() => setMode("inventory")}>
            From inventory
          </button>
          <button type="button" className={mode === "sourced" ? "chip chip-active" : "chip"} onClick={() => setMode("sourced")}>
            Source new part
          </button>
        </div>
        <form className="form-grid" style={{ padding: "1rem 1.25rem 1.25rem" }} onSubmit={onSubmit}>
          {mode === "inventory" ? (
            <>
              <SearchableCombobox
                label="Search inventory"
                placeholder="Part name or SKU…"
                options={options}
                value={selectedLabel}
                onSelect={(opt) => setStockKey(opt.value)}
              />
              <label>
                Qty
                <Input type="number" min={1} value={invQty} onChange={(e) => setInvQty(e.target.value)} />
              </label>
              {selectedBalance ? (
                <p className="hint" style={{ margin: 0 }}>
                  Sell {currencyCode} {selectedBalance.sell_price.toFixed(0)}
                  {canSeeCost ? ` · cost ${currencyCode} ${selectedBalance.cost_price.toFixed(0)}` : ""}
                </p>
              ) : null}
            </>
          ) : (
            <>
              <label>
                Part description
                <Input value={description} onChange={(e) => setDescription(e.target.value)} placeholder="e.g. MacBook A2337 Display" autoFocus required />
              </label>
              <div className="row gap">
                <label style={{ flex: 1 }}>
                  Supplier
                  <Input value={supplier} onChange={(e) => setSupplier(e.target.value)} placeholder="e.g. XYZ Spares" />
                </label>
                <label style={{ flex: 1 }}>
                  Supplier ref (optional)
                  <Input value={supplierRef} onChange={(e) => setSupplierRef(e.target.value)} />
                </label>
              </div>
              <div className="row gap">
                <label style={{ flex: 1 }}>
                  Cost ({currencyCode})
                  <Input type="number" min="0" step="0.01" value={cost} onChange={(e) => setCost(e.target.value)} required />
                </label>
                <label style={{ flex: 1 }}>
                  Selling price ({currencyCode})
                  <Input type="number" min="0" step="0.01" value={price} onChange={(e) => setPrice(e.target.value)} required />
                </label>
                <label style={{ width: 90 }}>
                  Qty
                  <Input type="number" min={1} value={qty} onChange={(e) => setQty(e.target.value)} />
                </label>
              </div>
              <label>
                Expected arrival (optional)
                <Input type="date" value={arrival} onChange={(e) => setArrival(e.target.value)} />
              </label>
              {canSeeCost && sourcedProfit != null ? (
                <p className="hint" style={{ margin: 0 }}>
                  Profit {currencyCode} {sourcedProfit.toFixed(0)}
                  {sourcedMarginPct != null ? ` · ${sourcedMarginPct.toFixed(0)}% margin` : ""}
                </p>
              ) : null}
              <p className="hint" style={{ margin: 0 }}>
                Not added to shop inventory. Once it arrives, add any leftover stock from the part's row.
              </p>
            </>
          )}
          {error ? <p className="form-error" role="alert">{error}</p> : null}
          <div className="btn-row">
            <Button type="submit" disabled={busy}>{busy ? "Adding…" : "Add part"}</Button>
            <Button type="button" variant="secondary" disabled={busy} onClick={onClose}>Cancel</Button>
          </div>
        </form>
      </div>
    </div>
  );
}
