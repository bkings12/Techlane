import { useEffect, useMemo, useState, type FormEvent } from "react";
import { Button, Input } from "./ui";
import { SearchableCombobox, type ComboOption } from "./SearchableCombobox";
import type { StockBalance } from "../lib/api";

export type AddProductModalProps = {
  open: boolean;
  onClose: () => void;
  stock: StockBalance[];
  currencyCode: string;
  onAdd: (input: { variant_id: string; location_id: string; quantity: number }) => Promise<void>;
};

export function AddProductModal({ open, onClose, stock, currencyCode, onAdd }: AddProductModalProps) {
  const [stockKey, setStockKey] = useState("");
  const [qty, setQty] = useState("1");
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");

  useEffect(() => {
    if (!open) return;
    setStockKey("");
    setQty("1");
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
          sublabel: `${currencyCode} ${b.sell_price.toFixed(0)} · ${b.available_qty} in stock at ${b.location_name}`,
        })),
    [stock, currencyCode],
  );
  const selectedLabel = options.find((o) => o.value === stockKey)?.label ?? "";
  const selectedBalance = stock.find((b) => `${b.variant_id}:${b.location_id}` === stockKey);

  if (!open) return null;

  async function onSubmit(e: FormEvent) {
    e.preventDefault();
    if (!selectedBalance) {
      setError("Search and select a product");
      return;
    }
    setBusy(true);
    setError("");
    try {
      await onAdd({
        variant_id: selectedBalance.variant_id,
        location_id: selectedBalance.location_id,
        quantity: Math.max(1, Number(qty) || 1),
      });
      onClose();
    } catch (err) {
      setError(err instanceof Error ? err.message : "Could not add product");
    } finally {
      setBusy(false);
    }
  }

  return (
    <div className="cmdk-backdrop sheet-backdrop" role="presentation" onClick={() => !busy && onClose()}>
      <div className="cmdk-panel sheet-panel" role="dialog" aria-modal="true" aria-label="Add product" onClick={(e) => e.stopPropagation()}>
        <div className="panel-head" style={{ padding: "1rem 1.25rem 0" }}>
          <h2 style={{ margin: 0 }}>Add product</h2>
          <p className="hint" style={{ margin: "0.25rem 0 0" }}>
            A retail accessory the customer decided to buy alongside the repair.
          </p>
        </div>
        <form className="form-grid" style={{ padding: "1rem 1.25rem 1.25rem" }} onSubmit={onSubmit}>
          <SearchableCombobox
            label="Search products"
            placeholder="Name or SKU…"
            options={options}
            value={selectedLabel}
            onSelect={(opt) => setStockKey(opt.value)}
          />
          <label>
            Qty
            <Input type="number" min={1} value={qty} onChange={(e) => setQty(e.target.value)} />
          </label>
          {selectedBalance ? (
            <p className="hint" style={{ margin: 0 }}>
              {currencyCode} {selectedBalance.sell_price.toFixed(0)} · {selectedBalance.available_qty} in stock
            </p>
          ) : null}
          {error ? <p className="form-error" role="alert">{error}</p> : null}
          <div className="btn-row">
            <Button type="submit" disabled={busy}>{busy ? "Adding…" : "Add to job"}</Button>
            <Button type="button" variant="secondary" disabled={busy} onClick={onClose}>Cancel</Button>
          </div>
        </form>
      </div>
    </div>
  );
}
