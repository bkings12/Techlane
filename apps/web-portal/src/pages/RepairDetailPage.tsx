import { useCallback, useEffect, useState } from "react";
import { useNavigate, useParams } from "react-router-dom";
import {
  approveEstimate,
  claimRepairWarranty,
  downloadRepairReceiptPDF,
  getRepair,
  getRepairWarranty,
  openRepairReceipt,
  payRepair,
  paymentStatus,
  rejectEstimate,
  savedPhone,
  statusLabels,
  type Repair,
  type Warranty,
} from "../api";
import { deviceLabel, estimateTotal, pendingEstimate, pillTone } from "../format";
import { EmptyState } from "../components/EmptyState";
import { useSession } from "../session";

type PayState = "idle" | "sending" | "waiting" | "done";

export function RepairDetailPage() {
  const { id } = useParams<{ id: string }>();
  const navigate = useNavigate();
  const { refreshRepairs } = useSession();

  const [detail, setDetail] = useState<Repair | null>(null);
  const [warranty, setWarranty] = useState<Warranty | null>(null);
  const [warrantyNote, setWarrantyNote] = useState("");
  const [warrantyBusy, setWarrantyBusy] = useState(false);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");

  const [payState, setPayState] = useState<PayState>("idle");
  const [payMsg, setPayMsg] = useState("");
  const [lastPaymentId, setLastPaymentId] = useState<string | null>(null);

  const refreshDetail = useCallback(async (repairId: string) => {
    const r = await getRepair(repairId);
    setDetail(r);
    setWarranty(null);
    void getRepairWarranty(r.id)
      .then(setWarranty)
      .catch(() => setWarranty(null));
  }, []);

  useEffect(() => {
    if (!id) return;
    setBusy(true);
    setError("");
    setPayState("idle");
    setPayMsg("");
    setLastPaymentId(null);
    refreshDetail(id)
      .catch((e) => setError(e instanceof Error ? e.message : "Failed to load"))
      .finally(() => setBusy(false));
  }, [id, refreshDetail]);

  async function onApprove() {
    if (!detail) return;
    const est = pendingEstimate(detail);
    if (!est) return;
    setBusy(true);
    setError("");
    try {
      await approveEstimate(detail.id, est.id);
      await refreshDetail(detail.id);
      void refreshRepairs();
    } catch (err) {
      setError(err instanceof Error ? err.message : "Approve failed");
    } finally {
      setBusy(false);
    }
  }

  async function onReject() {
    if (!detail) return;
    const est = pendingEstimate(detail);
    if (!est) return;
    if (!window.confirm("Reject this estimate? The shop will be notified — you can discuss next steps with them.")) {
      return;
    }
    setBusy(true);
    setError("");
    try {
      await rejectEstimate(detail.id, est.id);
      await refreshDetail(detail.id);
      void refreshRepairs();
    } catch (err) {
      setError(err instanceof Error ? err.message : "Reject failed");
    } finally {
      setBusy(false);
    }
  }

  async function pollPayment(repairId: string, paymentId: string) {
    for (let i = 0; i < 12; i++) {
      await new Promise((r) => setTimeout(r, 2500));
      const st = await paymentStatus(repairId, paymentId);
      if (["confirmed", "allocated"].includes(st.status)) {
        setPayMsg("Payment received — thank you!");
        setPayState("done");
        await refreshDetail(repairId);
        void refreshRepairs();
        return;
      }
      if (["failed", "cancelled"].includes(st.status)) {
        setPayMsg(`Payment ${st.status}. You can try again.`);
        setPayState("idle");
        await refreshDetail(repairId);
        return;
      }
      setPayMsg("Waiting for you to confirm on your phone…");
    }
    setPayState("idle");
    setPayMsg("Still not confirmed after 30s. If you completed the prompt, check status again — otherwise try paying again.");
  }

  async function onPay() {
    if (!detail) return;
    setPayState("sending");
    setError("");
    setPayMsg("");
    try {
      const res = await payRepair(detail.id, savedPhone());
      const paymentId = res.payment_id || res.id || null;
      setLastPaymentId(paymentId);
      setPayMsg(res.message || "Prompt sent — check your phone to complete payment.");
      if (paymentId) {
        setPayState("waiting");
        await pollPayment(detail.id, paymentId);
      } else {
        setPayState("idle");
      }
    } catch (err) {
      setError(err instanceof Error ? err.message : "Payment failed");
      setPayState("idle");
    }
  }

  async function onCheckAgain() {
    if (!detail || !lastPaymentId) return;
    setPayState("waiting");
    setError("");
    try {
      const st = await paymentStatus(detail.id, lastPaymentId);
      if (["confirmed", "allocated"].includes(st.status)) {
        setPayMsg("Payment received — thank you!");
        setPayState("done");
        await refreshDetail(detail.id);
        void refreshRepairs();
      } else if (["failed", "cancelled"].includes(st.status)) {
        setPayMsg(`Payment ${st.status}. You can try again.`);
        setPayState("idle");
        await refreshDetail(detail.id);
      } else {
        setPayMsg(`Still ${st.status}. Give it a bit more time, or try paying again.`);
        setPayState("idle");
      }
    } catch (err) {
      setError(err instanceof Error ? err.message : "Could not check payment status");
      setPayState("idle");
    }
  }

  if (busy && !detail) {
    return (
      <div className="app">
        <main className="shell">
          <div className="skeleton" />
        </main>
      </div>
    );
  }

  if (!detail) {
    return (
      <div className="app">
        <main className="shell">
          <div className="topbar">
            <h1>Repair not found</h1>
            <button className="btn-ghost" onClick={() => navigate("/")}>
              Back
            </button>
          </div>
          {error ? <p className="error">{error}</p> : null}
        </main>
      </div>
    );
  }

  const est = pendingEstimate(detail);
  const balance = detail.balance_due ?? detail.amount_due ?? 0;
  const canRetryCheck = payState === "idle" && lastPaymentId && payMsg.toLowerCase().includes("still");

  return (
    <div className="app">
      <main className="shell">
        <div className="topbar">
          <div>
            <h1>{detail.job_code}</h1>
            <p className="muted">{deviceLabel(detail) || "Device under repair"}</p>
          </div>
          <button className="btn-ghost" onClick={() => navigate("/")}>
            Back
          </button>
        </div>
        {error ? <p className="error">{error}</p> : null}
        <section className="section">
          <span className={`pill ${pillTone(detail.status)}`}>
            {statusLabels[detail.status] ?? detail.status}
          </span>
          <p className="muted" style={{ marginTop: "0.85rem" }}>
            {detail.problem_summary}
          </p>
        </section>

        {est && est.status === "pending" ? (
          <section className="section estimate">
            <h2>Estimate for approval</h2>
            <p className="money">
              {(est.currency || "KES")} {estimateTotal(est).toLocaleString()}
            </p>
            {est.notes ? <p className="muted">{est.notes}</p> : null}
            <div className="btn-row">
              <button className="btn" disabled={busy} onClick={() => void onApprove()}>
                Approve
              </button>
              <button className="btn btn-secondary" disabled={busy} onClick={() => void onReject()}>
                Reject
              </button>
            </div>
          </section>
        ) : null}

        {balance > 0 ? (
          <section className="section">
            <h2>Pay balance</h2>
            <p className="money">KES {balance.toLocaleString()}</p>
            {payMsg ? (
              <div className="pay-status">
                {payState === "waiting" ? <span className="spinner" aria-hidden="true" /> : null}
                <p className="hint" role="status">
                  {payMsg}
                </p>
              </div>
            ) : null}
            <div className="btn-row">
              <button className="btn" disabled={payState === "sending" || payState === "waiting"} onClick={() => void onPay()}>
                {payState === "sending"
                  ? "Sending prompt…"
                  : payState === "waiting"
                    ? "Waiting for confirmation…"
                    : "Pay with M-Pesa"}
              </button>
              {canRetryCheck ? (
                <button className="btn btn-secondary" onClick={() => void onCheckAgain()}>
                  Check status again
                </button>
              ) : null}
            </div>
          </section>
        ) : null}

        <section className="section">
          <h3>Timeline</h3>
          {(detail.timeline ?? []).length === 0 ? (
            <EmptyState title="No updates yet" body="You'll see progress here as work happens." />
          ) : (
            <ol className="timeline">
              {(detail.timeline ?? []).map((ev, i) => (
                <li key={`${ev.status}-${ev.created_at ?? ev.at}-${i}`}>
                  <strong>{statusLabels[ev.status ?? ""] ?? ev.status ?? ev.event_type}</strong>
                  <time>
                    {ev.created_at || ev.at ? new Date(ev.created_at || ev.at || "").toLocaleString() : ""}
                  </time>
                  {ev.note ? <p>{ev.note}</p> : null}
                </li>
              ))}
            </ol>
          )}
        </section>

        <section className="section">
          <h3>Receipts</h3>
          <div className="row" style={{ marginBottom: 12 }}>
            <button
              className="btn btn-secondary"
              type="button"
              onClick={() => {
                void openRepairReceipt(detail.id).catch((err) =>
                  setError(err instanceof Error ? err.message : "Could not open receipt"),
                );
              }}
            >
              Print receipt
            </button>
            <button
              className="btn btn-secondary"
              type="button"
              onClick={() => {
                void downloadRepairReceiptPDF(detail.id).catch((err) =>
                  setError(err instanceof Error ? err.message : "Could not download PDF"),
                );
              }}
            >
              Receipt PDF
            </button>
          </div>
          {warranty ? (
            <div className="card" style={{ marginBottom: 12 }}>
              <strong>Warranty</strong>
              <p className="muted">
                {warranty.status} · {warranty.duration_days} days · ends{" "}
                {new Date(warranty.ends_at).toLocaleDateString()}
              </p>
              {warranty.status === "active" ? (
                <form
                  onSubmit={(e) => {
                    e.preventDefault();
                    setWarrantyBusy(true);
                    void claimRepairWarranty(detail.id, warrantyNote)
                      .then((w) => {
                        setWarranty(w);
                        setWarrantyNote("");
                      })
                      .catch((err) => setError(err instanceof Error ? err.message : "Claim failed"))
                      .finally(() => setWarrantyBusy(false));
                  }}
                >
                  <input
                    value={warrantyNote}
                    onChange={(e) => setWarrantyNote(e.target.value)}
                    placeholder="Claim note"
                  />
                  <button className="btn" type="submit" disabled={warrantyBusy}>
                    {warrantyBusy ? "Claiming…" : "Claim warranty"}
                  </button>
                </form>
              ) : null}
            </div>
          ) : null}
          {(detail.receipts ?? []).length === 0 ? (
            <EmptyState title="No payments yet" body="Receipts appear here after you pay." />
          ) : (
            <ul className="receipts">
              {(detail.receipts ?? []).map((r) => (
                <li key={r.id}>
                  <div>
                    <strong>
                      {r.currency || "KES"} {r.amount.toLocaleString()}
                    </strong>
                    <div className="muted">
                      {r.method.replaceAll("_", " ")} · {r.status}
                    </div>
                  </div>
                  <time>{new Date(r.created_at).toLocaleString()}</time>
                </li>
              ))}
            </ul>
          )}
        </section>
      </main>
    </div>
  );
}
