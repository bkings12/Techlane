import { useEffect, useState, type FormEvent } from "react";
import { Button, Input } from "./ui";

export type AddServiceModalProps = {
  open: boolean;
  onClose: () => void;
  onAdd: (input: { description: string; unit_price: number; quantity: number }) => Promise<void>;
  currencyCode: string;
};

export function AddServiceModal({ open, onClose, onAdd, currencyCode }: AddServiceModalProps) {
  const [description, setDescription] = useState("");
  const [price, setPrice] = useState("");
  const [qty, setQty] = useState("1");
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");

  useEffect(() => {
    if (!open) return;
    setDescription("");
    setPrice("");
    setQty("1");
    setError("");
    setBusy(false);
  }, [open]);

  if (!open) return null;

  async function onSubmit(e: FormEvent) {
    e.preventDefault();
    const desc = description.trim();
    const unitPrice = Number(price);
    if (!desc) {
      setError("Enter what the service is (e.g. Board repair)");
      return;
    }
    if (!Number.isFinite(unitPrice) || unitPrice < 0) {
      setError("Enter a valid price");
      return;
    }
    setBusy(true);
    setError("");
    try {
      await onAdd({ description: desc, unit_price: unitPrice, quantity: Math.max(1, Number(qty) || 1) });
      onClose();
    } catch (err) {
      setError(err instanceof Error ? err.message : "Could not add service");
    } finally {
      setBusy(false);
    }
  }

  return (
    <div className="cmdk-backdrop sheet-backdrop" role="presentation" onClick={() => !busy && onClose()}>
      <div className="cmdk-panel sheet-panel" role="dialog" aria-modal="true" aria-label="Add service" onClick={(e) => e.stopPropagation()}>
        <div className="panel-head" style={{ padding: "1rem 1.25rem 0" }}>
          <h2 style={{ margin: 0 }}>Add service</h2>
          <p className="hint" style={{ margin: "0.25rem 0 0" }}>
            Diagnosis, board repair, software install, cleaning — anything billed as labour.
          </p>
        </div>
        <form className="form-grid" style={{ padding: "1rem 1.25rem 1.25rem" }} onSubmit={onSubmit}>
          <label>
            Description
            <Input
              value={description}
              onChange={(e) => setDescription(e.target.value)}
              placeholder="e.g. Logic board repair"
              autoFocus
              required
            />
          </label>
          <div className="row gap">
            <label style={{ flex: 1 }}>
              Price ({currencyCode})
              <Input type="number" min="0" step="0.01" value={price} onChange={(e) => setPrice(e.target.value)} required />
            </label>
            <label style={{ width: 90 }}>
              Qty
              <Input type="number" min={1} value={qty} onChange={(e) => setQty(e.target.value)} />
            </label>
          </div>
          {error ? <p className="form-error" role="alert">{error}</p> : null}
          <div className="btn-row">
            <Button type="submit" disabled={busy}>{busy ? "Adding…" : "Add service"}</Button>
            <Button type="button" variant="secondary" disabled={busy} onClick={onClose}>Cancel</Button>
          </div>
        </form>
      </div>
    </div>
  );
}
