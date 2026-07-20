import { useCallback, useEffect, useState, type FormEvent } from "react";
import { Link } from "react-router-dom";
import { useAuth } from "../auth/AuthContext";
import { Badge, Button, EmptyState, ICONS, Input, PageHeader, Stat, StatStrip } from "../components/ui";
import {
  approveRefund,
  confirmCashHandover,
  createRefund,
  getPaymentSettings,
  listAllPayments,
  listC2BTransactions,
  listCashHandovers,
  listRefunds,
  listUsers,
  matchC2BTransaction,
  pendingCashTotal,
  requestCashHandover,
  type C2BTransaction,
  type CashHandover,
  type Payment,
  type PaymentProviderSettings,
  type Refund,
  type StaffUser,
} from "../lib/api";

function can(perms: string[] | undefined, code: string) {
  return !!perms?.includes("*") || !!perms?.includes(code);
}

export function PaymentsPage() {
  const { user } = useAuth();
  const [cfg, setCfg] = useState<PaymentProviderSettings | null>(null);
  const [items, setItems] = useState<Payment[]>([]);
  const [handovers, setHandovers] = useState<CashHandover[]>([]);
  const [refunds, setRefunds] = useState<Refund[]>([]);
  const [c2bOpen, setC2bOpen] = useState<C2BTransaction[]>([]);
  const [matchByC2b, setMatchByC2b] = useState<Record<string, string>>({});
  const [staff, setStaff] = useState<StaffUser[]>([]);
  const [pendingCash, setPendingCash] = useState(0);
  const [toUser, setToUser] = useState("");
  const [amount, setAmount] = useState("");
  const [countByHandover, setCountByHandover] = useState<Record<string, string>>({});
  const [refundPaymentId, setRefundPaymentId] = useState("");
  const [refundAmount, setRefundAmount] = useState("");
  const [refundReason, setRefundReason] = useState("");
  const [error, setError] = useState("");
  const [busy, setBusy] = useState(false);

  const canCreateRefund = can(user?.permissions, "refunds.create") || can(user?.permissions, "payments.initiate");
  const canApproveRefund = can(user?.permissions, "refunds.approve");
  const canMatchC2B =
    can(user?.permissions, "cash.handover.confirm") ||
    can(user?.permissions, "refunds.approve") ||
    can(user?.permissions, "payments.initiate");

  const refresh = useCallback(async () => {
    const [c, p, h, cash, users, refs, unmatched, mismatch] = await Promise.all([
      getPaymentSettings(),
      listAllPayments(),
      listCashHandovers(),
      pendingCashTotal(),
      listUsers().catch(() => ({ items: [] as StaffUser[] })),
      listRefunds().catch(() => ({ items: [] as Refund[] })),
      listC2BTransactions("unmatched").catch(() => ({ items: [] as C2BTransaction[] })),
      listC2BTransactions("amount_mismatch").catch(() => ({ items: [] as C2BTransaction[] })),
    ]);
    setCfg(c);
    setItems(p.items ?? []);
    setHandovers(h.items ?? []);
    setPendingCash(cash.amount ?? 0);
    setStaff((users.items ?? []).filter((u) => u.id !== user?.id));
    setRefunds(refs.items ?? []);
    setC2bOpen([...(unmatched.items ?? []), ...(mismatch.items ?? [])]);
    setAmount((prev) => (prev === "" && cash.amount > 0 ? String(cash.amount) : prev));
    setCountByHandover((prev) => {
      const next = { ...prev };
      for (const ho of h.items ?? []) {
        if (ho.status === "requested" && next[ho.id] === undefined) {
          next[ho.id] = String(ho.amount);
        }
      }
      return next;
    });
  }, [user?.id]);

  useEffect(() => {
    refresh().catch((e) => setError(e instanceof Error ? e.message : "Failed to load"));
  }, [refresh]);

  async function submitHandover(e: FormEvent) {
    e.preventDefault();
    setBusy(true);
    setError("");
    try {
      await requestCashHandover({
        amount: Number(amount) || 0,
        to_user_id: toUser || undefined,
        branch_id: user?.branch_ids?.[0],
      });
      await refresh();
    } catch (err) {
      setError(err instanceof Error ? err.message : "Handover failed");
    } finally {
      setBusy(false);
    }
  }

  async function confirm(id: string) {
    setBusy(true);
    setError("");
    try {
      const raw = countByHandover[id];
      const counted = raw === undefined || raw === "" ? undefined : Number(raw);
      if (counted !== undefined && (!Number.isFinite(counted) || counted < 0)) {
        throw new Error("Counted amount must be zero or more");
      }
      await confirmCashHandover(id, counted);
      await refresh();
    } catch (err) {
      setError(err instanceof Error ? err.message : "Confirm failed");
    } finally {
      setBusy(false);
    }
  }

  async function submitRefund(e: FormEvent) {
    e.preventDefault();
    setBusy(true);
    setError("");
    try {
      await createRefund({
        payment_id: refundPaymentId,
        amount: Number(refundAmount) || 0,
        reason: refundReason.trim() || undefined,
      });
      setRefundPaymentId("");
      setRefundAmount("");
      setRefundReason("");
      await refresh();
    } catch (err) {
      setError(err instanceof Error ? err.message : "Refund request failed");
    } finally {
      setBusy(false);
    }
  }

  async function approve(id: string) {
    setBusy(true);
    setError("");
    try {
      await approveRefund(id);
      await refresh();
    } catch (err) {
      setError(err instanceof Error ? err.message : "Approve failed");
    } finally {
      setBusy(false);
    }
  }

  async function matchC2B(id: string) {
    const paymentId = matchByC2b[id];
    if (!paymentId) {
      setError("Select a C2B payment to match against");
      return;
    }
    setBusy(true);
    setError("");
    try {
      await matchC2BTransaction(id, paymentId);
      setMatchByC2b((prev) => {
        const next = { ...prev };
        delete next[id];
        return next;
      });
      await refresh();
    } catch (err) {
      setError(err instanceof Error ? err.message : "Match failed");
    } finally {
      setBusy(false);
    }
  }

  const openHandovers = handovers.filter((h) => h.status === "requested");
  const shortageTotal = handovers.reduce((s, h) => s + (h.shortage_amount || 0), 0);
  const pendingRefunds = refunds.filter((r) => r.status === "pending");
  const refundable = items.filter((p) => p.status === "allocated" || p.status === "confirmed");
  const matchablePayments = items.filter((p) => p.method === "mpesa_c2b" && (p.status === "initiated" || p.status === "pending"));

  return (
    <div>
      <PageHeader
        title="Payments & cash"
        subtitle="STK readiness, provisional cash, handovers, and refunds"
        actions={
          <Link to="/settings/payments" className="muted">
            Configure credentials
          </Link>
        }
      />
      {error ? <p className="form-error">{error}</p> : null}

      <StatStrip>
        <Stat
          icon={ICONS.stk}
          label="M-Pesa"
          value={<Badge tone={cfg?.configured ? "success" : "warning"}>{cfg?.configured ? "ready" : "needs setup"}</Badge>}
        />
        <Stat icon={ICONS.cash} label="Your provisional cash" value={`KES ${pendingCash.toLocaleString()}`} />
        <Stat icon={ICONS.clock} label="Open handovers" value={openHandovers.length} tone={openHandovers.length ? "warn" : undefined} />
        <Stat icon={ICONS.reports} label="Pending refunds" value={pendingRefunds.length} tone={pendingRefunds.length ? "warn" : undefined} />
        <Stat icon={ICONS.risk} label="Unmatched C2B" value={c2bOpen.length} tone={c2bOpen.length ? "danger" : undefined} />
        <Stat icon={ICONS.shortage} label="Recorded shortages" value={`KES ${shortageTotal.toLocaleString()}`} tone={shortageTotal ? "warn" : undefined} />
      </StatStrip>

      <div className="repair-grid">
        <section className="panel">
          <h2>Cash handover</h2>
          <p className="hint">
            Cash stays provisional until a different manager/cashier confirms. Confirmer enters the physical count — short
            counts raise a risk alert. You cannot confirm your own request.
          </p>
          <form className="form-grid" onSubmit={submitHandover}>
            <label>
              Amount (KES)
              <Input type="number" value={amount} onChange={(e) => setAmount(e.target.value)} required />
            </label>
            <label>
              Hand to (optional)
              <select className="input" value={toUser} onChange={(e) => setToUser(e.target.value)}>
                <option value="">Any manager / cashier</option>
                {staff.map((s) => (
                  <option key={s.id} value={s.id}>
                    {s.display_name}
                  </option>
                ))}
              </select>
            </label>
            <Button type="submit" disabled={busy || !(Number(amount) > 0)}>
              Request handover
            </Button>
          </form>

          <h3 style={{ marginTop: "1.5rem" }}>Queue</h3>
          {handovers.length === 0 ? (
            <EmptyState title="No handovers" body="Request a handover after collecting provisional cash." />
          ) : (
            <ul className="part-list">
              {handovers.slice(0, 12).map((h) => {
                const mine = h.from_user_id === user?.id;
                const counted = countByHandover[h.id] ?? String(h.amount);
                const short =
                  h.status === "requested"
                    ? Math.max(0, h.amount - (Number(counted) || 0))
                    : h.shortage_amount || 0;
                return (
                  <li key={h.id} className="part-card">
                    <div className="part-head">
                      <div>
                        <strong>KES {h.amount.toLocaleString()}</strong>
                        <div className="muted">{new Date(h.created_at).toLocaleString()}</div>
                      </div>
                      <Badge tone={h.status === "confirmed" ? (short > 0 ? "warning" : "success") : "pending"}>
                        {h.status}
                      </Badge>
                    </div>
                    {h.status === "confirmed" && (h.shortage_amount || 0) > 0 ? (
                      <p className="hint">
                        Shortage KES {h.shortage_amount!.toLocaleString()} — see{" "}
                        <Link to="/risk">Risk</Link>
                      </p>
                    ) : null}
                    {h.status === "requested" && !mine ? (
                      <div className="stack-form" style={{ marginTop: "0.75rem" }}>
                        <label>
                          Counted cash (KES)
                          <Input
                            type="number"
                            value={counted}
                            onChange={(e) =>
                              setCountByHandover((prev) => ({ ...prev, [h.id]: e.target.value }))
                            }
                          />
                        </label>
                        {short > 0 ? (
                          <p className="hint">Will record shortage of KES {short.toLocaleString()}</p>
                        ) : null}
                        <Button type="button" disabled={busy} onClick={() => void confirm(h.id)}>
                          Confirm received
                        </Button>
                      </div>
                    ) : null}
                    {h.status === "requested" && mine ? (
                      <p className="hint">Waiting for another staff member to confirm.</p>
                    ) : null}
                  </li>
                );
              })}
            </ul>
          )}
        </section>

        <aside className="stack">
          <section className="panel context-panel">
            <h2>Digital rails</h2>
            {!cfg?.configured ? (
              <EmptyState
                title="Connect M-Pesa"
                body="Daraja credentials unlock STK on repair jobs. Bank paybill reuses the same keys."
              />
            ) : (
              <dl className="meta-dl">
                <dt>Shortcode</dt>
                <dd className="mono">{cfg.mpesa_shortcode}</dd>
                <dt>Bank</dt>
                <dd className="mono">
                  {cfg.bank_configured ? `${cfg.bank_paybill} / ${cfg.bank_account}` : "off"}
                </dd>
              </dl>
            )}
            <Link to="/settings/payments">Payment settings →</Link>
          </section>

          <section className="panel">
            <div className="panel-head">
              <h2>Unmatched C2B</h2>
            </div>
            <p className="hint">
              Paybill deposits that did not match an awaiting account ref (or amount differed). Record an{" "}
              <span className="mono">mpesa_c2b</span> payment on the job first, then match here. Clears the risk alert.
            </p>
            {c2bOpen.length === 0 ? (
              <EmptyState title="No open C2B exceptions" body="Matched paybill confirmations settle automatically." />
            ) : (
              <ul className="part-list">
                {c2bOpen.map((t) => (
                  <li key={t.id} className="part-card">
                    <div className="part-head">
                      <div>
                        <strong>KES {t.amount.toLocaleString()}</strong>
                        <div className="muted">
                          Ref <span className="mono">{t.bill_ref_number || "—"}</span>
                          {t.trans_id ? (
                            <>
                              {" "}
                              · <span className="mono">{t.trans_id}</span>
                            </>
                          ) : null}
                        </div>
                        {t.msisdn ? <div className="hint">{t.msisdn}</div> : null}
                      </div>
                      <Badge tone="warning">{t.status.replaceAll("_", " ")}</Badge>
                    </div>
                    {canMatchC2B ? (
                      <div className="stack-form" style={{ marginTop: "0.75rem" }}>
                        <label>
                          Match to awaiting C2B payment
                          <select
                            className="input"
                            value={matchByC2b[t.id] ?? ""}
                            onChange={(e) => setMatchByC2b((prev) => ({ ...prev, [t.id]: e.target.value }))}
                          >
                            <option value="">Select payment</option>
                            {matchablePayments.map((p) => (
                              <option key={p.id} value={p.id}>
                                KES {p.amount.toLocaleString()} · {p.status} · {p.id.slice(0, 8)}
                              </option>
                            ))}
                          </select>
                        </label>
                        <Button type="button" disabled={busy || !matchByC2b[t.id]} onClick={() => void matchC2B(t.id)}>
                          Match &amp; allocate
                        </Button>
                      </div>
                    ) : null}
                  </li>
                ))}
              </ul>
            )}
          </section>

          <section className="panel">
            <div className="panel-head">
              <h2>Refunds</h2>
            </div>
            <p className="hint">
              Request against allocated/confirmed payments. A different manager or accountant must approve — you cannot
              approve your own request.
            </p>
            {canCreateRefund ? (
              <form className="form-grid" onSubmit={submitRefund}>
                <label>
                  Payment
                  <select
                    className="input"
                    value={refundPaymentId}
                    onChange={(e) => {
                      setRefundPaymentId(e.target.value);
                      const p = refundable.find((x) => x.id === e.target.value);
                      if (p) setRefundAmount(String(p.amount));
                    }}
                    required
                  >
                    <option value="">Select payment</option>
                    {refundable.map((p) => (
                      <option key={p.id} value={p.id}>
                        {p.method.replaceAll("_", " ")} · KES {p.amount.toLocaleString()} · {p.status}
                      </option>
                    ))}
                  </select>
                </label>
                <label>
                  Amount (KES)
                  <Input
                    type="number"
                    value={refundAmount}
                    onChange={(e) => setRefundAmount(e.target.value)}
                    required
                  />
                </label>
                <label>
                  Reason
                  <Input value={refundReason} onChange={(e) => setRefundReason(e.target.value)} placeholder="Optional" />
                </label>
                <Button type="submit" disabled={busy || !refundPaymentId || !(Number(refundAmount) > 0)}>
                  Request refund
                </Button>
              </form>
            ) : null}

            {refunds.length === 0 ? (
              <EmptyState title="No refunds" body="Refund requests will show here for approval." />
            ) : (
              <ul className="part-list" style={{ marginTop: "1rem" }}>
                {refunds.slice(0, 10).map((r) => {
                  const mine = r.created_by === user?.id;
                  return (
                    <li key={r.id} className="part-card">
                      <div className="part-head">
                        <div>
                          <strong>KES {r.amount.toLocaleString()}</strong>
                          <div className="muted mono">{r.payment_id.slice(0, 8)}…</div>
                          {r.reason ? <div className="hint">{r.reason}</div> : null}
                        </div>
                        <Badge tone={r.status === "approved" ? "success" : "pending"}>{r.status}</Badge>
                      </div>
                      {r.status === "pending" && canApproveRefund && !mine ? (
                        <Button
                          type="button"
                          disabled={busy}
                          onClick={() => void approve(r.id)}
                          style={{ marginTop: "0.75rem" }}
                        >
                          Approve refund
                        </Button>
                      ) : null}
                      {r.status === "pending" && mine ? (
                        <p className="hint">Waiting for another approver.</p>
                      ) : null}
                    </li>
                  );
                })}
              </ul>
            )}
          </section>

          <section className="panel">
            <div className="panel-head">
              <h2>Recent payments</h2>
            </div>
            {items.length === 0 ? (
              <EmptyState title="No payments yet" body="Cash and STK from repairs appear here." />
            ) : (
              <ul className="list">
                {items.slice(0, 8).map((p) => (
                  <li key={p.id}>
                    <Badge tone={p.status === "allocated" || p.status.includes("confirm") ? "success" : "pending"}>
                      {p.status}
                    </Badge>
                    <span>
                      {p.method.replaceAll("_", " ")} · KES {p.amount.toLocaleString()}
                    </span>
                  </li>
                ))}
              </ul>
            )}
          </section>
        </aside>
      </div>
    </div>
  );
}
