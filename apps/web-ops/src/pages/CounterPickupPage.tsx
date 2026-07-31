import { useState, type FormEvent } from "react";
import { Link, useNavigate } from "react-router-dom";
import { Badge, Button, Input } from "../components/ui";
import {
  collectOnlineOrder,
  collectRepairByPickupCode,
  lookupRepairByPickupCode,
  type PickupLookup,
} from "../lib/api";

/**
 * Counter pickup workspace — parity with Android Pickup / Scan tabs.
 * Accepts repair PK- codes or online order collection codes.
 */
export function CounterPickupPage() {
  const navigate = useNavigate();
  const [code, setCode] = useState("");
  const [name, setName] = useState("Customer");
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");
  const [message, setMessage] = useState("");
  const [lookup, setLookup] = useState<PickupLookup | null>(null);
  const [mode, setMode] = useState<"repair" | "order">("repair");

  async function onLookup(e: FormEvent) {
    e.preventDefault();
    const trimmed = code.trim();
    if (!trimmed) {
      setError("Enter or scan a pickup code");
      return;
    }
    setBusy(true);
    setError("");
    setMessage("");
    setLookup(null);
    try {
      if (mode === "order") {
        const order = await collectOnlineOrder(trimmed);
        setMessage(`Order ${order.id.slice(0, 8)} marked collected (${order.status})`);
        setCode("");
      } else {
        const hit = await lookupRepairByPickupCode(trimmed);
        setLookup(hit);
      }
    } catch (err) {
      setError(err instanceof Error ? err.message : "Code not found");
    } finally {
      setBusy(false);
    }
  }

  async function releaseRepair() {
    if (!lookup) return;
    setBusy(true);
    setError("");
    try {
      await collectRepairByPickupCode({
        pickup_code: lookup.pickup_code || code.trim(),
        collected_by_name: name.trim() || "Customer",
        relationship: "self",
      });
      setMessage(`Released ${lookup.job_code || lookup.id.slice(0, 8)}`);
      setLookup(null);
      setCode("");
    } catch (err) {
      setError(err instanceof Error ? err.message : "Could not release device");
    } finally {
      setBusy(false);
    }
  }

  return (
    <div className="pos-fullscreen">
      <header className="pos-fullscreen-bar">
        <div className="pos-fullscreen-brand">
          <strong>Pickup</strong>
          <span>Counter · full screen</span>
        </div>
        <nav className="pos-fullscreen-actions" aria-label="Counter tools">
          <Link to="/pos" className="btn btn-ghost">
            Sell
          </Link>
          <Link to="/counter/fix" className="btn btn-ghost">
            Same-day fix
          </Link>
          <Link to="/" className="btn btn-secondary">
            Exit to ops
          </Link>
        </nav>
      </header>

      <div className="pos-counter-immersive" style={{ maxWidth: 520 }}>
        <div className="payment-method-grid" style={{ marginBottom: 14 }}>
          <button type="button" className={mode === "repair" ? "active" : ""} onClick={() => setMode("repair")}>
            Repair job
          </button>
          <button type="button" className={mode === "order" ? "active" : ""} onClick={() => setMode("order")}>
            Online order
          </button>
        </div>

        {error ? <p className="form-error">{error}</p> : null}
        {message ? <p className="hint">{message}</p> : null}

        <form className="panel form-grid" onSubmit={(e) => void onLookup(e)}>
          <h2>{mode === "repair" ? "Repair pickup code" : "Order collection code"}</h2>
          <p className="muted" style={{ margin: 0 }}>
            {mode === "repair"
              ? "Scan or type the PK- code from the intake slip."
              : "Scan or type the online order collection code — releases in one step."}
          </p>
          <label>
            Code
            <Input
              value={code}
              onChange={(e) => setCode(e.target.value)}
              placeholder={mode === "repair" ? "PK-…" : "Collection code"}
              autoFocus
              required
            />
          </label>
          {mode === "repair" ? (
            <label>
              Collected by
              <Input value={name} onChange={(e) => setName(e.target.value)} />
            </label>
          ) : null}
          <Button type="submit" disabled={busy}>
            {busy ? "Looking up…" : mode === "order" ? "Collect order" : "Look up job"}
          </Button>
        </form>

        {lookup ? (
          <div className="panel" style={{ marginTop: 14, display: "grid", gap: 10 }}>
            <div className="panel-head">
              <h2>{lookup.job_code || lookup.id.slice(0, 8)}</h2>
              <Badge tone={lookup.can_release ? "success" : "warning"}>{lookup.status}</Badge>
            </div>
            <p className="muted" style={{ margin: 0 }}>
              {lookup.problem_summary || "Repair job"}
            </p>
            {(lookup.balance_due ?? 0) > 0.01 ? (
              <p className="form-error">Balance due: KES {Number(lookup.balance_due).toLocaleString()} — take payment first.</p>
            ) : null}
            <div className="btn-row">
              <Button type="button" disabled={busy || !lookup.can_release} onClick={() => void releaseRepair()}>
                {busy ? "Releasing…" : "Hand over device"}
              </Button>
              <Button type="button" variant="ghost" onClick={() => navigate(`/repairs/${lookup.id}`)}>
                Open job
              </Button>
            </div>
          </div>
        ) : null}
      </div>
    </div>
  );
}
