import { useCallback, useEffect, useMemo, useState, type FormEvent } from "react";
import { Link } from "react-router-dom";
import { useAuth } from "../auth/AuthContext";
import { useBranch } from "../branch/BranchContext";
import { useRealtimeEvents } from "../lib/realtime";
import { Badge, Button, EmptyState, Input, PageHeader } from "../components/ui";
import {
  approveRefund,
  createRefund,
  getPaymentSettings,
  listAllPayments,
  listC2BTransactions,
  listPOSCatalog,
  listRefunds,
  listStockLocations,
  matchC2BToNewSale,
  matchC2BTransaction,
  type C2BTransaction,
  type CatalogItem,
  type Payment,
  type PaymentProviderSettings,
  type Refund,
  type StockLocation,
} from "../lib/api";

function can(perms: string[] | undefined, code: string) {
  return !!perms?.includes("*") || !!perms?.includes(code);
}

function isSuccessStatus(status: string) {
  return status === "allocated" || status === "confirmed";
}

function startOfToday() {
  const d = new Date();
  d.setHours(0, 0, 0, 0);
  return d;
}

function formatWhen(iso?: string) {
  if (!iso) return "—";
  const d = new Date(iso);
  if (Number.isNaN(d.getTime())) return "—";
  return new Intl.DateTimeFormat("en-KE", {
    day: "numeric",
    month: "short",
    hour: "2-digit",
    minute: "2-digit",
  }).format(d);
}

function methodLabel(method: string) {
  switch (method) {
    case "mpesa_stk":
      return "M-Pesa STK";
    case "mpesa_c2b":
      return "M-Pesa Paybill";
    case "cash":
      return "Cash";
    case "bank_paybill":
      return "Bank paybill";
    case "store_credit":
      return "Store credit";
    default:
      return method.replaceAll("_", " ");
  }
}

function statusTone(status: string): "success" | "pending" | "warning" | "danger" {
  if (isSuccessStatus(status)) return "success";
  if (status === "failed" || status === "cancelled") return "danger";
  if (status.includes("pending") || status === "initiated") return "pending";
  return "warning";
}

export function PaymentsPage() {
  const { user } = useAuth();
  const { branchId } = useBranch();
  const [cfg, setCfg] = useState<PaymentProviderSettings | null>(null);
  const [items, setItems] = useState<Payment[]>([]);
  const [refunds, setRefunds] = useState<Refund[]>([]);
  const [c2bOpen, setC2bOpen] = useState<C2BTransaction[]>([]);
  const [matchByC2b, setMatchByC2b] = useState<Record<string, string>>({});
  const [locations, setLocations] = useState<StockLocation[]>([]);
  const [locationId, setLocationId] = useState("");
  const [catalog, setCatalog] = useState<CatalogItem[]>([]);
  const [saleMatch, setSaleMatch] = useState<Record<string, { variantId: string; qty: string }>>({});
  const [refundPaymentId, setRefundPaymentId] = useState("");
  const [refundAmount, setRefundAmount] = useState("");
  const [refundReason, setRefundReason] = useState("");
  const [error, setError] = useState("");
  const [busy, setBusy] = useState(false);
  const [query, setQuery] = useState("");
  const [methodFilter, setMethodFilter] = useState("all");
  const [statusFilter, setStatusFilter] = useState("all");

  const canCreateRefund = can(user?.permissions, "refunds.create") || can(user?.permissions, "payments.initiate");
  const canApproveRefund = can(user?.permissions, "refunds.approve");
  const canMatchC2B =
    can(user?.permissions, "refunds.approve") || can(user?.permissions, "payments.initiate");

  const refresh = useCallback(async () => {
    const [c, p, refs, unmatched, mismatch] = await Promise.all([
      getPaymentSettings(),
      listAllPayments(),
      listRefunds().catch(() => ({ items: [] as Refund[] })),
      listC2BTransactions("unmatched").catch(() => ({ items: [] as C2BTransaction[] })),
      listC2BTransactions("amount_mismatch").catch(() => ({ items: [] as C2BTransaction[] })),
    ]);
    setCfg(c);
    setItems(p.items ?? []);
    setRefunds(refs.items ?? []);
    setC2bOpen([...(unmatched.items ?? []), ...(mismatch.items ?? [])]);
  }, []);

  useEffect(() => {
    refresh().catch((e) => setError(e instanceof Error ? e.message : "Failed to load"));
  }, [refresh]);

  useEffect(() => {
    listStockLocations(branchId || undefined)
      .then((r) => {
        const locs = r.items ?? [];
        setLocations(locs);
        setLocationId((prev) => (prev && locs.some((l) => l.id === prev) ? prev : locs[0]?.id ?? ""));
      })
      .catch(() => setLocations([]));
  }, [branchId]);

  useEffect(() => {
    if (!locationId) return;
    listPOSCatalog(locationId)
      .then((r) => setCatalog(r.items ?? []))
      .catch(() => setCatalog([]));
  }, [locationId]);

  useRealtimeEvents(["payment.*"], () => {
    refresh().catch(() => undefined);
  });

  const todayStart = startOfToday().getTime();
  const todayItems = useMemo(
    () => items.filter((p) => p.created_at && new Date(p.created_at).getTime() >= todayStart),
    [items, todayStart],
  );
  const todayReceived = todayItems.filter((p) => isSuccessStatus(p.status)).reduce((s, p) => s + p.amount, 0);
  const todaySuccessCount = todayItems.filter((p) => isSuccessStatus(p.status)).length;
  const todayPending = todayItems.filter((p) => !isSuccessStatus(p.status) && p.status !== "failed" && p.status !== "cancelled");
  const todayFailed = todayItems.filter((p) => p.status === "failed" || p.status === "cancelled").length;

  const filtered = useMemo(() => {
    const q = query.trim().toLowerCase();
    return items.filter((p) => {
      if (methodFilter !== "all" && p.method !== methodFilter) return false;
      if (statusFilter === "success" && !isSuccessStatus(p.status)) return false;
      if (statusFilter === "pending" && (isSuccessStatus(p.status) || p.status === "failed" || p.status === "cancelled")) return false;
      if (statusFilter === "failed" && p.status !== "failed" && p.status !== "cancelled") return false;
      if (!q) return true;
      const hay = [
        p.customer_name,
        p.job_code,
        p.sale_label,
        p.account_reference,
        p.phone,
        p.method,
        p.status,
        p.id,
        p.payable_type,
      ]
        .filter(Boolean)
        .join(" ")
        .toLowerCase();
      return hay.includes(q);
    });
  }, [items, query, methodFilter, statusFilter]);

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

  async function matchToNewSale(id: string) {
    const draft = saleMatch[id];
    const qty = Number(draft?.qty);
    if (!draft?.variantId) {
      setError("Select a product to match this deposit to");
      return;
    }
    if (!Number.isFinite(qty) || qty <= 0) {
      setError("Enter a positive quantity");
      return;
    }
    if (!branchId || !locationId) {
      setError("Select a branch and stock location first");
      return;
    }
    setBusy(true);
    setError("");
    try {
      await matchC2BToNewSale(id, { branch_id: branchId, location_id: locationId, variant_id: draft.variantId, quantity: qty });
      setSaleMatch((prev) => {
        const next = { ...prev };
        delete next[id];
        return next;
      });
      await refresh();
      const cat = await listPOSCatalog(locationId).catch(() => null);
      if (cat) setCatalog(cat.items ?? []);
    } catch (err) {
      setError(err instanceof Error ? err.message : "Match failed");
    } finally {
      setBusy(false);
    }
  }

  const pendingRefunds = refunds.filter((r) => r.status === "pending");
  const refundable = items.filter((p) => p.status === "allocated" || p.status === "confirmed");
  const matchablePayments = items.filter((p) => p.method === "mpesa_c2b" && (p.status === "initiated" || p.status === "pending"));

  return (
    <div className="money-desk tx-desk">
      <PageHeader
        title="Transactions"
        subtitle="Every payment tied to a job or client — live ledger with today’s collections."
        actions={
          <div className="btn-row">
            <Link to="/pos" className="btn btn-secondary">
              Sell
            </Link>
            <Link to="/settings/payments" className="muted">
              Payment settings
            </Link>
          </div>
        }
      />
      {error ? <p className="form-error">{error}</p> : null}

      <section className="tx-stats" aria-label="Today's collections">
        <article className="tx-stat tx-stat-primary">
          <span>Received today</span>
          <strong>KES {todayReceived.toLocaleString()}</strong>
          <em>{todaySuccessCount} successful payment{todaySuccessCount === 1 ? "" : "s"}</em>
        </article>
        <article className="tx-stat">
          <span>Pending today</span>
          <strong>{todayPending.length}</strong>
          <em>Awaiting PIN / allocation</em>
        </article>
        <article className="tx-stat">
          <span>Failed / cancelled</span>
          <strong>{todayFailed}</strong>
          <em>Today</em>
        </article>
        <article className={`tx-stat ${c2bOpen.length ? "warn" : ""}`}>
          <span>Unmatched C2B</span>
          <strong>{c2bOpen.length}</strong>
          <em>Need matching</em>
        </article>
        <article className="tx-stat">
          <span>M-Pesa</span>
          <strong>
            <Badge tone={cfg?.configured ? "success" : "warning"}>{cfg?.configured ? "Ready" : "Setup"}</Badge>
          </strong>
          <em>Provider status</em>
        </article>
      </section>

      <section className="desk-ledger tx-ledger">
        <div className="panel-head tx-ledger-head">
          <div>
            <h2>Transaction ledger</h2>
            <p className="muted">Linked to jobs and clients</p>
          </div>
          <div className="tx-filters">
            <Input
              value={query}
              onChange={(e) => setQuery(e.target.value)}
              placeholder="Search client, job, phone, ref…"
              aria-label="Search transactions"
            />
            <select className="input" value={methodFilter} onChange={(e) => setMethodFilter(e.target.value)} aria-label="Method">
              <option value="all">All methods</option>
              <option value="mpesa_stk">M-Pesa STK</option>
              <option value="mpesa_c2b">M-Pesa Paybill</option>
              <option value="cash">Cash</option>
              <option value="bank_paybill">Bank paybill</option>
            </select>
            <select className="input" value={statusFilter} onChange={(e) => setStatusFilter(e.target.value)} aria-label="Status">
              <option value="all">All statuses</option>
              <option value="success">Successful</option>
              <option value="pending">Pending</option>
              <option value="failed">Failed</option>
            </select>
          </div>
        </div>
        {filtered.length === 0 ? (
          <div style={{ padding: "1rem" }}>
            <EmptyState
              title={items.length === 0 ? "No transactions yet" : "No matches"}
              body={items.length === 0 ? "Cash, STK, and paybill payments from jobs and sales appear here." : "Try clearing filters."}
            />
          </div>
        ) : (
          <div className="table-wrap">
            <table className="table tx-table">
              <thead>
                <tr>
                  <th>When</th>
                  <th>Client</th>
                  <th>Job / sale</th>
                  <th>Method</th>
                  <th>Amount</th>
                  <th>Status</th>
                  <th>Ref</th>
                </tr>
              </thead>
              <tbody>
                {filtered.slice(0, 80).map((p) => {
                  const jobLink =
                    p.payable_type === "repair" && p.payable_id
                      ? `/repairs/${p.payable_id}`
                      : null;
                  const clientLink = p.customer_id ? `/customers/${p.customer_id}` : null;
                  return (
                    <tr key={p.id}>
                      <td className="muted">{formatWhen(p.created_at)}</td>
                      <td>
                        {clientLink ? (
                          <Link to={clientLink}>{p.customer_name || "Customer"}</Link>
                        ) : (
                          <span className="muted">{p.customer_name || p.phone || "—"}</span>
                        )}
                      </td>
                      <td>
                        {jobLink ? (
                          <Link to={jobLink} className="mono">
                            {p.job_code || p.payable_id?.slice(0, 8)}
                          </Link>
                        ) : p.sale_label ? (
                          <span className="mono">{p.sale_label}</span>
                        ) : p.payable_type ? (
                          <span className="muted">{p.payable_type}</span>
                        ) : (
                          <span className="muted">—</span>
                        )}
                      </td>
                      <td>{methodLabel(p.method)}</td>
                      <td className="mono tx-amount">KES {p.amount.toLocaleString()}</td>
                      <td>
                        <Badge tone={statusTone(p.status)}>{p.status.replaceAll("_", " ")}</Badge>
                      </td>
                      <td className="mono muted">{p.account_reference || p.phone || p.id.slice(0, 8)}</td>
                    </tr>
                  );
                })}
              </tbody>
            </table>
          </div>
        )}
      </section>

      <div className="desk-attention">
        <section className={`attention-card ${c2bOpen.length ? "warn" : ""}`}>
          <h2>Unmatched C2B</h2>
          <p className="hint">Paybill deposits waiting to be matched to a payment or a new sale.</p>
          {c2bOpen.length === 0 ? (
            <EmptyState title="All clear" body="No unmatched paybill deposits." />
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
                      <p className="muted" style={{ margin: "0.6rem 0 0.35rem", fontSize: "0.82rem" }}>
                        Or match to a product — creates a sale and deducts stock.
                      </p>
                      {locations.length > 1 ? (
                        <label>
                          Stock location
                          <select className="input" value={locationId} onChange={(e) => setLocationId(e.target.value)}>
                            {locations.map((l) => (
                              <option key={l.id} value={l.id}>
                                {l.name}
                              </option>
                            ))}
                          </select>
                        </label>
                      ) : null}
                      <label>
                        Product
                        <select
                          className="input"
                          value={saleMatch[t.id]?.variantId ?? ""}
                          onChange={(e) =>
                            setSaleMatch((prev) => ({
                              ...prev,
                              [t.id]: { qty: prev[t.id]?.qty ?? "1", variantId: e.target.value },
                            }))
                          }
                        >
                          <option value="">Select product</option>
                          {catalog.map((c) => (
                            <option key={c.variant_id} value={c.variant_id}>
                              {c.product_name} — {c.sku} · KES {c.sell_price.toLocaleString()} · {c.available_qty} in stock
                            </option>
                          ))}
                        </select>
                      </label>
                      <label>
                        Quantity
                        <Input
                          type="number"
                          min={1}
                          value={saleMatch[t.id]?.qty ?? "1"}
                          onChange={(e) =>
                            setSaleMatch((prev) => ({
                              ...prev,
                              [t.id]: { variantId: prev[t.id]?.variantId ?? "", qty: e.target.value },
                            }))
                          }
                        />
                      </label>
                      <Button
                        type="button"
                        variant="secondary"
                        disabled={busy || !saleMatch[t.id]?.variantId}
                        onClick={() => void matchToNewSale(t.id)}
                      >
                        Match to product &amp; deduct stock
                      </Button>
                    </div>
                  ) : null}
                </li>
              ))}
            </ul>
          )}
        </section>

        <section className={`attention-card ${pendingRefunds.length ? "warn" : ""}`}>
          <h2>Refunds</h2>
          <p className="hint">Request against allocated payments. Another manager must approve.</p>
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
                      {methodLabel(p.method)} · KES {p.amount.toLocaleString()} · {p.customer_name || p.job_code || p.status}
                    </option>
                  ))}
                </select>
              </label>
              <label>
                Amount (KES)
                <Input type="number" value={refundAmount} onChange={(e) => setRefundAmount(e.target.value)} required />
              </label>
              <label>
                Reason
                <Input value={refundReason} onChange={(e) => setRefundReason(e.target.value)} />
              </label>
              <Button type="submit" disabled={busy}>
                Request refund
              </Button>
            </form>
          ) : null}
          {refunds.length === 0 ? null : (
            <ul className="part-list" style={{ marginTop: "1rem" }}>
              {refunds.slice(0, 6).map((r) => {
                const mine = r.created_by === user?.id;
                return (
                  <li key={r.id} className="part-card">
                    <div className="part-head">
                      <div>
                        <strong>KES {r.amount.toLocaleString()}</strong>
                        <div className="muted mono">{r.payment_id.slice(0, 8)}…</div>
                      </div>
                      <Badge tone={r.status === "approved" ? "success" : "pending"}>{r.status}</Badge>
                    </div>
                    {r.status === "pending" && canApproveRefund && !mine ? (
                      <Button type="button" disabled={busy} onClick={() => void approve(r.id)}>
                        Approve refund
                      </Button>
                    ) : null}
                    {r.status === "pending" && mine ? <p className="hint">Waiting for another approver.</p> : null}
                  </li>
                );
              })}
            </ul>
          )}
        </section>
      </div>
    </div>
  );
}
