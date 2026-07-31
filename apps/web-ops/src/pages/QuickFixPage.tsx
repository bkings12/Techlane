import { useState, type FormEvent } from "react";
import { Link, useNavigate } from "react-router-dom";
import { useBranch } from "../branch/BranchContext";
import { Button, Input, StkWaitOverlay, isTerminalStkError, sleep } from "../components/ui";
import {
  createCustomer,
  createDevice,
  createPayment,
  createRepair,
  changeRepairStatus,
  openRepairReceipt,
  reconcileMpesaPayment,
  recordHandover,
  type Payment,
} from "../lib/api";

/**
 * Same-day counter fix — parity with Android QuickRepairScreen.
 * Create → force bench statuses → pay → hand over with pickup code → print.
 */
export function QuickFixPage() {
  const { branchId, setBranchId, branches } = useBranch();
  const navigate = useNavigate();
  const [step, setStep] = useState(0);
  const [name, setName] = useState("");
  const [phone, setPhone] = useState("");
  const [brand, setBrand] = useState("");
  const [model, setModel] = useState("");
  const [problem, setProblem] = useState("");
  const [amount, setAmount] = useState("");
  const [method, setMethod] = useState("cash");
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");
  const [message, setMessage] = useState("");
  const [jobId, setJobId] = useState("");
  const [jobCode, setJobCode] = useState("");
  const [pickupCode, setPickupCode] = useState("");
  const [stkPolling, setStkPolling] = useState(false);
  const [stkSuccess, setStkSuccess] = useState("");

  function paymentIdFromCreate(res: Payment | { items: Payment[] }): string | null {
    if ("items" in res && Array.isArray(res.items)) {
      const stk = res.items.find((p) => p.method === "mpesa_stk") ?? res.items[0];
      return stk?.id ?? null;
    }
    return (res as Payment).id ?? null;
  }

  async function finishAfterPaid() {
    if (pickupCode) {
      await recordHandover(jobId, {
        collected_by_name: name.trim() || "Customer",
        relationship: "self",
        pickup_code: pickupCode,
        note: "Same-day counter fix — handed back at the till",
      });
      setMessage("Paid and handed over");
    } else {
      setMessage("Payment successful — finish handover from the job if needed");
    }
    try {
      await openRepairReceipt(jobId);
    } catch {
      /* print is best-effort */
    }
    setStep(2);
  }

  async function pollStkPayment(paymentId: string) {
    setStkPolling(true);
    setStkSuccess("");
    setError("");
    try {
      for (let i = 0; i < 48; i++) {
        await sleep(2500);
        try {
          const p = await reconcileMpesaPayment(paymentId);
          if (p.status === "allocated" || p.status === "confirmed") {
            setStkSuccess("Payment successful");
            await finishAfterPaid();
            window.setTimeout(() => setStkSuccess(""), 2500);
            return;
          }
          if (p.status === "failed" || p.status === "cancelled") {
            setError(`STK ${p.status}`);
            return;
          }
        } catch (e) {
          const msg = e instanceof Error ? e.message : "";
          if (isTerminalStkError(msg)) {
            setError(msg || "STK failed or cancelled");
            return;
          }
        }
      }
      setError("STK timed out — send again or check from the job");
    } finally {
      setStkPolling(false);
    }
  }

  async function createJob(e: FormEvent) {
    e.preventDefault();
    if (!branchId) {
      setError("Select a branch first");
      return;
    }
    const value = Number(amount);
    if (!name.trim()) {
      setError("Enter the customer's name");
      return;
    }
    if (!problem.trim()) {
      setError("What did you fix?");
      return;
    }
    if (!Number.isFinite(value) || value <= 0) {
      setError("Enter a positive amount");
      return;
    }
    setBusy(true);
    setError("");
    try {
      const customer = await createCustomer({
        full_name: name.trim(),
        phone: phone.trim() || undefined,
      });
      const device = await createDevice({
        customer_id: customer.id,
        kind: "phone",
        brand: brand.trim() || undefined,
        model: model.trim() || undefined,
      });
      const job = await createRepair({
        branch_id: branchId,
        customer_id: customer.id,
        device_id: device.id,
        problem_summary: problem.trim(),
        labor_amount: value,
        service_type: "repair",
      });
      await changeRepairStatus(job.id, { status: "in_progress" });
      await changeRepairStatus(job.id, { status: "ready_for_pickup" });
      await changeRepairStatus(job.id, { status: "completed" });
      setJobId(job.id);
      setJobCode(job.job_code || job.id.slice(0, 8));
      setPickupCode(job.pickup_code || "");
      setStep(1);
      setMessage("Job ready — take payment");
    } catch (err) {
      setError(err instanceof Error ? err.message : "Could not create the job");
    } finally {
      setBusy(false);
    }
  }

  async function takePayment(e: FormEvent) {
    e.preventDefault();
    if (!jobId || !branchId) return;
    const value = Number(amount);
    if (!Number.isFinite(value) || value <= 0) {
      setError("Enter a positive amount");
      return;
    }
    if (method === "mpesa_stk" && !phone.trim()) {
      setError("Phone required for STK");
      return;
    }
    setBusy(true);
    setError("");
    try {
      const created = await createPayment({
        method,
        amount: value,
        payable_type: "repair",
        payable_id: jobId,
        branch_id: branchId,
        phone: phone.trim() || undefined,
        account_reference: jobCode,
      });
      if (method === "cash") {
        await finishAfterPaid();
      } else {
        setMessage("STK sent — waiting for PIN");
        const id = paymentIdFromCreate(created);
        if (id) void pollStkPayment(id);
      }
    } catch (err) {
      setError(err instanceof Error ? err.message : "Payment failed");
    } finally {
      setBusy(false);
    }
  }

  return (
    <div className="pos-fullscreen">
      <StkWaitOverlay
        visible={stkPolling}
        message="Waiting for M-Pesa"
        detail="Ask the customer to enter their PIN. Handover runs when payment succeeds."
        success={stkSuccess}
      />
      <header className="pos-fullscreen-bar">
        <div className="pos-fullscreen-brand">
          <strong>Same-day fix</strong>
          <span>Counter · full screen</span>
        </div>
        <div className="pos-fullscreen-meta">
          <label>
            Branch
            <select className="input" value={branchId} onChange={(e) => setBranchId(e.target.value)}>
              {branches.map((b) => (
                <option key={b.id} value={b.id}>
                  {b.name}
                </option>
              ))}
            </select>
          </label>
        </div>
        <nav className="pos-fullscreen-actions" aria-label="Counter tools">
          <Link to="/pos" className="btn btn-ghost">
            Sell
          </Link>
          <Link to="/counter/pickup" className="btn btn-ghost">
            Pickup
          </Link>
          <Link to="/" className="btn btn-secondary">
            Exit to ops
          </Link>
        </nav>
      </header>

      <div className="pos-counter-immersive" style={{ maxWidth: 560 }}>
        <p className="muted" style={{ marginTop: 0 }}>
          Step {step + 1} of 3 · {step === 0 ? "Customer & fix" : step === 1 ? "Payment" : "Done"}
        </p>
        {error ? <p className="form-error">{error}</p> : null}
        {message ? <p className="hint">{message}</p> : null}

        {step === 0 ? (
          <form className="panel form-grid" onSubmit={(e) => void createJob(e)}>
            <h2>Walk-in fix</h2>
            <label>
              Customer name
              <Input value={name} onChange={(e) => setName(e.target.value)} required autoFocus />
            </label>
            <label>
              Phone
              <Input value={phone} onChange={(e) => setPhone(e.target.value)} placeholder="07XXXXXXXX" />
            </label>
            <label>
              Brand
              <Input value={brand} onChange={(e) => setBrand(e.target.value)} />
            </label>
            <label>
              Model
              <Input value={model} onChange={(e) => setModel(e.target.value)} />
            </label>
            <label>
              What did you fix?
              <Input value={problem} onChange={(e) => setProblem(e.target.value)} required />
            </label>
            <label>
              Amount (KES)
              <Input type="number" min={1} value={amount} onChange={(e) => setAmount(e.target.value)} required />
            </label>
            <Button type="submit" disabled={busy}>
              {busy ? "Creating…" : "Create job & continue"}
            </Button>
          </form>
        ) : null}

        {step === 1 ? (
          <form className="panel form-grid" onSubmit={(e) => void takePayment(e)}>
            <h2>
              Pay {jobCode} · KES {Number(amount).toLocaleString()}
            </h2>
            <fieldset className="payment-methods">
              <legend>Payment method</legend>
              <div className="payment-method-grid">
                <button type="button" className={method === "cash" ? "active" : ""} onClick={() => setMethod("cash")}>
                  Cash
                </button>
                <button
                  type="button"
                  className={method === "mpesa_stk" ? "active" : ""}
                  onClick={() => setMethod("mpesa_stk")}
                >
                  M-Pesa STK
                </button>
              </div>
            </fieldset>
            {method === "mpesa_stk" ? (
              <label>
                Phone for STK
                <Input value={phone} onChange={(e) => setPhone(e.target.value)} required />
              </label>
            ) : null}
            <Button type="submit" disabled={busy || stkPolling}>
              {busy || stkPolling ? "Processing…" : method === "cash" ? "Take cash & hand over" : "Send STK"}
            </Button>
          </form>
        ) : null}

        {step === 2 ? (
          <div className="panel" style={{ display: "grid", gap: 12 }}>
            <h2>Done</h2>
            <p className="muted">{message || "Sale closed."}</p>
            <div className="btn-row">
              <Button type="button" onClick={() => navigate(`/repairs/${jobId}`)}>
                Open job
              </Button>
              <Button
                type="button"
                variant="secondary"
                onClick={() => {
                  setStep(0);
                  setName("");
                  setPhone("");
                  setBrand("");
                  setModel("");
                  setProblem("");
                  setAmount("");
                  setJobId("");
                  setJobCode("");
                  setPickupCode("");
                  setMessage("");
                  setError("");
                }}
              >
                New same-day fix
              </Button>
            </div>
          </div>
        ) : null}
      </div>
    </div>
  );
}
