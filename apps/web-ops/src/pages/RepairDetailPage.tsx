import { useCallback, useEffect, useState, type FormEvent } from "react";
import { Link, useParams } from "react-router-dom";
import { useAuth } from "../auth/AuthContext";
import { Badge, Button, EmptyState, ICONS, Input, PageHeader, PhotoCaptureField, Stat, StatStrip } from "../components/ui";
import {
  addRepairNote,
  assignPartRequest,
  assignRepair,
  changeRepairStatus,
  collectSupplierIssue,
  createRepairEstimate,
  createPartRequest,
  createPayment,
  createRefund,
  claimRepairWarranty,
  createRepairWarranty,
  deleteRepairAttachment,
  downloadRepairReceiptPDF,
  downloadRepairTaxInvoicePDF,
  fetchRepairAttachmentBlob,
  getPaymentSettings,
  getRepair,
  getRepairWarranty,
  listPartRequests,
  listRepairAttachments,
  listRepairEstimates,
  listRepairNotes,
  listRepairPayments,
  listSuppliers,
  listTechnicians,
  openRepairReceipt,
  reconcileMpesaPayment,
  uploadRepairAttachment,
  type PartRequest,
  type Payment,
  type PaymentProviderSettings,
  type RepairAttachment,
  type RepairEstimate,
  type RepairJob,
  type RepairNote,
  type StaffUser,
  type Supplier,
  type Warranty,
} from "../lib/api";

function dedupeById<T extends { id: string }>(items: T[]): T[] {
  const seen = new Set<string>();
  return items.filter((item) => {
    if (seen.has(item.id)) return false;
    seen.add(item.id);
    return true;
  });
}

function can(permissions: string[] | undefined, permission: string) {
  return permissions?.includes("*") || permissions?.includes(permission);
}

function statusTone(status: string): "success" | "warning" | "danger" | "info" | "pending" {
  if (status === "completed" || status === "collected" || status === "approved") return "success";
  if (status === "waiting_parts" || status === "pending" || status === "provisional" || status === "pending_handover")
    return "pending";
  if (status.includes("fail") || status === "orphan") return "danger";
  return "info";
}

export function RepairDetailPage() {
  const { id } = useParams();
  const { user } = useAuth();
  const [job, setJob] = useState<RepairJob | null>(null);
  const [parts, setParts] = useState<PartRequest[]>([]);
  const [payments, setPayments] = useState<Payment[]>([]);
  const [notes, setNotes] = useState<RepairNote[]>([]);
  const [techs, setTechs] = useState<StaffUser[]>([]);
  const [error, setError] = useState("");
  const [busy, setBusy] = useState(false);
  const [techId, setTechId] = useState("");
  const [partDesc, setPartDesc] = useState("OEM screen");
  const [suppliers, setSuppliers] = useState<Supplier[]>([]);
  const [supplierByRequest, setSupplierByRequest] = useState<Record<string, string>>({});
  const [labor, setLabor] = useState("5000");
  const [payAmount, setPayAmount] = useState("5000");
  const [payMethod, setPayMethod] = useState("cash");
  const [payPhone, setPayPhone] = useState("");
  const [payCfg, setPayCfg] = useState<PaymentProviderSettings | null>(null);
  const [collectCode, setCollectCode] = useState<Record<string, string>>({});
  const [noteText, setNoteText] = useState("");
  const [refundPaymentId, setRefundPaymentId] = useState("");
  const [refundAmount, setRefundAmount] = useState("");
  const [refundReason, setRefundReason] = useState("");
  const [attachments, setAttachments] = useState<RepairAttachment[]>([]);
  const [estimates, setEstimates] = useState<RepairEstimate[]>([]);
  const [estimateLabor, setEstimateLabor] = useState("");
  const [estimateParts, setEstimateParts] = useState("");
  const [estimateNotes, setEstimateNotes] = useState("");
  const [warranty, setWarranty] = useState<Warranty | null>(null);
  const [warrantyNote, setWarrantyNote] = useState("");
  const [splitPay, setSplitPay] = useState(false);
  const [tenders, setTenders] = useState([
    { method: "cash", amount: "2500", phone: "" },
    { method: "mpesa_stk", amount: "2500", phone: "" },
  ]);

  const refresh = useCallback(async () => {
    if (!id) return;
    const [j, p, pay, t, cfg, n, att, estimateResult, supplierRes, warrantyRes] = await Promise.all([
      getRepair(id),
      listPartRequests(id),
      listRepairPayments(id),
      listTechnicians(),
      getPaymentSettings().catch(() => null),
      listRepairNotes(id),
      listRepairAttachments(id).catch(() => ({ items: [] as RepairAttachment[] })),
      listRepairEstimates(id),
      listSuppliers().catch(() => ({ items: [] as Supplier[] })),
      getRepairWarranty(id).catch(() => null),
    ]);
    setJob(j);
    setParts(p.items ?? []);
    // Guard against fanned-out duplicate rows from the payments API (e.g. a payment
    // linked to more than one reconciliation record) so list keys stay unique.
    setPayments(dedupeById(pay.items ?? []));
    setTechs(t.items ?? []);
    setPayCfg(cfg);
    setNotes(n.items ?? []);
    setAttachments(att.items ?? []);
    setEstimates(estimateResult.items ?? []);
    setSuppliers(supplierRes.items ?? []);
    setWarranty(warrantyRes);
    setSupplierByRequest((current) => {
      const next = { ...current };
      for (const row of p.items ?? []) {
        if (!next[row.id] && row.assigned_supplier_id) next[row.id] = row.assigned_supplier_id;
      }
      return next;
    });
    if (j.technician_id) setTechId(j.technician_id);
    if (j.labor_amount) {
      setLabor(String(j.labor_amount));
      setPayAmount(String(j.labor_amount));
    }
  }, [id]);

  useEffect(() => {
    refresh().catch((e) => setError(e instanceof Error ? e.message : "Failed to load"));
  }, [refresh]);

  async function run(action: () => Promise<unknown>) {
    setBusy(true);
    setError("");
    try {
      await action();
      await refresh();
    } catch (e) {
      setError(e instanceof Error ? e.message : "Action failed");
    } finally {
      setBusy(false);
    }
  }

  const canApprove = can(user?.permissions, "parts.approve");

  if (!job && !error) return <div className="boot">Loading job…</div>;
  if (!job) {
    return (
      <div>
        <PageHeader title="Repair" subtitle="Not found" />
        <p className="form-error">{error}</p>
        <Link to="/repairs">Back to repairs</Link>
      </div>
    );
  }

  const next = job.next_statuses ?? [];
  const customer = job.customer;
  const device = job.device;
  const refundable = payments.filter(
    (p) => p.status === "allocated" || p.status === "confirmed" || p.status === "provisional",
  );
  const paidTotal = payments
    .filter((p) => ["allocated", "confirmed", "provisional"].includes(p.status))
    .reduce((sum, p) => sum + p.amount, 0);
  const balanceDue = Math.max(0, (job.labor_amount ?? 0) - paidTotal);

  return (
    <div className="repair-detail">
      <PageHeader
        title={job.job_code ?? `Job ${job.id.slice(0, 8)}`}
        subtitle={`${job.problem_summary} · Branch ${job.branch_id.slice(0, 8)}…`}
        actions={
          <div className="btn-row">
            <Button
              type="button"
              variant="secondary"
              disabled={busy}
              onClick={() =>
                void run(async () => {
                  await openRepairReceipt(job.id);
                })
              }
            >
              Print receipt
            </Button>
            <Button
              type="button"
              variant="secondary"
              disabled={busy}
              onClick={() => void run(() => downloadRepairReceiptPDF(job.id))}
            >
              Receipt PDF
            </Button>
            <Button
              type="button"
              variant="secondary"
              disabled={busy}
              onClick={() => void run(() => downloadRepairTaxInvoicePDF(job.id))}
            >
              Tax invoice PDF
            </Button>
            <Link to="/repairs" className="muted">
              All repairs
            </Link>
          </div>
        }
      />
      {error ? <p className="form-error">{error}</p> : null}

      <StatStrip>
        <Stat icon={ICONS.repairs} label="Status" value={<Badge tone={statusTone(job.status)}>{job.status.replaceAll("_", " ")}</Badge>} />
        <Stat icon={ICONS.cash} label="Labor" value={`KES ${(job.labor_amount ?? 0).toLocaleString()}`} />
        <Stat
          icon={ICONS.customers}
          label="Technician"
          value={
            job.technician_id
              ? techs.find((t) => t.id === job.technician_id)?.display_name ?? job.technician_id.slice(0, 8)
              : "Unassigned"
          }
        />
        <Stat icon={ICONS.clock} label="Opened" value={job.created_at ? new Date(job.created_at).toLocaleString() : "—"} />
      </StatStrip>

      <section className="panel" style={{ marginBottom: "1rem" }}>
        <h2>Warranty</h2>
        {warranty ? (
          <p>
            <Badge tone={statusTone(warranty.status)}>{warranty.status}</Badge>{" "}
            {warranty.duration_days} days · ends {new Date(warranty.ends_at).toLocaleDateString()}
            {warranty.status === "active" ? (
              <form
                className="inline-form"
                style={{ marginTop: "0.75rem" }}
                onSubmit={(e: FormEvent) => {
                  e.preventDefault();
                  void run(async () => {
                    await claimRepairWarranty(job.id, warrantyNote);
                    setWarrantyNote("");
                  });
                }}
              >
                <Input
                  value={warrantyNote}
                  onChange={(e) => setWarrantyNote(e.target.value)}
                  placeholder="Claim note"
                />
                <Button type="submit" disabled={busy}>
                  Claim warranty
                </Button>
              </form>
            ) : null}
          </p>
        ) : (
          <p className="muted">
            No warranty yet.{" "}
            {job.status === "completed" || job.status === "collected" ? (
              <Button type="button" variant="secondary" disabled={busy} onClick={() => void run(() => createRepairWarranty(job.id))}>
                Create 90-day warranty
              </Button>
            ) : (
              "Created automatically when the job is completed or collected."
            )}
          </p>
        )}
      </section>

      <div className="repair-grid">
        <div className="stack">
          <section className="panel">
            <h2>Immediate actions</h2>
            <form
              className="inline-form"
              onSubmit={(e: FormEvent) => {
                e.preventDefault();
                if (!techId) return;
                void run(() => assignRepair(job.id, techId));
              }}
            >
              <label>
                Assign technician
                <select className="input" value={techId} onChange={(e) => setTechId(e.target.value)}>
                  <option value="">Select…</option>
                  {techs.map((t) => (
                    <option key={t.id} value={t.id}>
                      {t.display_name}
                    </option>
                  ))}
                </select>
              </label>
              <Button type="submit" disabled={busy || !techId}>
                Assign
              </Button>
              {user?.id ? (
                <Button
                  type="button"
                  variant="secondary"
                  disabled={busy}
                  onClick={() => void run(() => assignRepair(job.id, user.id))}
                >
                  Assign me
                </Button>
              ) : null}
            </form>

            {next.length > 0 ? (
              <div className="action-block">
                <p className="muted">Update status</p>
                <div className="chip-row">
                  {next.map((s) => (
                    <Button
                      key={s}
                      type="button"
                      variant={s === "completed" ? "primary" : "secondary"}
                      disabled={busy}
                      onClick={() =>
                        void run(() =>
                          changeRepairStatus(job.id, {
                            status: s,
                            labor_amount: s === "completed" ? Number(labor) || 0 : undefined,
                          }),
                        )
                      }
                    >
                      → {s.replaceAll("_", " ")}
                    </Button>
                  ))}
                </div>
                {next.includes("completed") ? (
                  <label className="tight">
                    Labor amount (KES) for commission & receipt
                    <Input type="number" value={labor} onChange={(e) => setLabor(e.target.value)} />
                  </label>
                ) : null}
              </div>
            ) : (
              <p className="hint">No further status changes available.</p>
            )}
          </section>

          <section className="panel">
            <div className="panel-head">
              <h2>Customer estimates</h2>
              {estimates.some((e) => e.status === "pending") ? (
                <Badge tone="pending">awaiting customer</Badge>
              ) : estimates.some((e) => e.status === "approved") ? (
                <Badge tone="success">approved</Badge>
              ) : null}
            </div>
            <p className="hint" style={{ marginBottom: "1rem" }}>
              Customer sees this in the portal / app and can approve or reject before payment.
            </p>
            <form
              className="form-grid estimate-form"
              onSubmit={(event: FormEvent) => {
                event.preventDefault();
                const laborAmount = Number(estimateLabor);
                const partsAmount = Number(estimateParts);
                if (
                  !Number.isFinite(laborAmount) ||
                  !Number.isFinite(partsAmount) ||
                  laborAmount < 0 ||
                  partsAmount < 0
                ) {
                  setError("Estimate amounts must be zero or greater");
                  return;
                }
                void run(async () => {
                  await createRepairEstimate(job.id, {
                    labor_amount: laborAmount,
                    parts_amount: partsAmount,
                    notes: estimateNotes.trim() || undefined,
                  });
                  setEstimateLabor("");
                  setEstimateParts("");
                  setEstimateNotes("");
                });
              }}
            >
              <label>
                Labor (KES)
                <Input
                  type="number"
                  min="0"
                  step="0.01"
                  value={estimateLabor}
                  onChange={(e) => setEstimateLabor(e.target.value)}
                  required
                />
              </label>
              <label>
                Parts (KES)
                <Input
                  type="number"
                  min="0"
                  step="0.01"
                  value={estimateParts}
                  onChange={(e) => setEstimateParts(e.target.value)}
                  required
                />
              </label>
              <label>
                Notes (optional)
                <Input value={estimateNotes} onChange={(e) => setEstimateNotes(e.target.value)} />
              </label>
              <div className="estimate-preview">
                <span className="muted">Total</span>
                <strong className="estimate-total">
                  KES{" "}
                  {(
                    (Number(estimateLabor) || 0) + (Number(estimateParts) || 0)
                  ).toLocaleString()}
                </strong>
              </div>
              <Button type="submit" disabled={busy || !estimateLabor || !estimateParts}>
                Create estimate
              </Button>
            </form>
            {estimates.length === 0 ? (
              <EmptyState title="No estimates yet" body="Create an estimate for the customer to approve or reject." />
            ) : (
              <ul className="part-list" style={{ marginTop: "1rem" }}>
                {estimates.map((estimate) => (
                  <li key={estimate.id} className="part-card">
                    <div className="part-head">
                      <div>
                        <strong className="estimate-total">
                          {estimate.currency}{" "}
                          {(estimate.labor_amount + estimate.parts_amount).toLocaleString()}
                        </strong>
                        <div className="muted">
                          Labor {estimate.labor_amount.toLocaleString()} · Parts{" "}
                          {estimate.parts_amount.toLocaleString()}
                        </div>
                        <div className="muted">{new Date(estimate.created_at).toLocaleString()}</div>
                      </div>
                      <Badge tone={statusTone(estimate.status)}>{estimate.status}</Badge>
                    </div>
                    {estimate.notes ? <p>{estimate.notes}</p> : null}
                    {estimate.expires_at && estimate.status === "pending" ? (
                      <p className="hint">Expires {new Date(estimate.expires_at).toLocaleString()}</p>
                    ) : null}
                  </li>
                ))}
              </ul>
            )}
          </section>

          <section className="panel">
            <h2>Parts (anti-leakage)</h2>
            <form
              className="inline-form"
              onSubmit={(e: FormEvent) => {
                e.preventDefault();
                void run(() =>
                  createPartRequest({
                    repair_job_id: job.id,
                    branch_id: job.branch_id,
                    description: partDesc,
                    quantity: 1,
                  }),
                );
              }}
            >
              <label>
                Request part
                <Input value={partDesc} onChange={(e) => setPartDesc(e.target.value)} required />
              </label>
              <Button type="submit" disabled={busy}>
                Request
              </Button>
            </form>

            {parts.length === 0 ? (
              <EmptyState title="No parts yet" body="Request a screen or part to create a supplier authorization trail." />
            ) : (
              <ul className="part-list">
                {parts.map((pr) => (
                  <li key={pr.id} className="part-card">
                    <div className="part-head">
                      <strong>{pr.description}</strong>
                      <Badge tone={statusTone(pr.issue?.status || pr.status)}>{pr.issue?.status || pr.status}</Badge>
                    </div>
                    {pr.status === "pending" && canApprove ? (
                      <div className="assign-block">
                        <span className="muted">Assign to supplier portal</span>
                        <div className="inline-form">
                          <select
                            className="input"
                            value={supplierByRequest[pr.id] ?? pr.assigned_supplier_id ?? ""}
                            onChange={(e) =>
                              setSupplierByRequest((current) => ({ ...current, [pr.id]: e.target.value }))
                            }
                          >
                            <option value="">Select…</option>
                            {suppliers.map((supplier) => (
                              <option key={supplier.id} value={supplier.id}>
                                {supplier.name}
                              </option>
                            ))}
                          </select>
                          <Button
                            type="button"
                            variant="secondary"
                            disabled={busy || !supplierByRequest[pr.id]}
                            onClick={() => void run(() => assignPartRequest(pr.id, supplierByRequest[pr.id]!))}
                          >
                            {pr.assigned_supplier_id ? "Reassign" : "Assign supplier"}
                          </Button>
                        </div>
                      </div>
                    ) : null}
                    {(pr.quotes ?? []).filter((q) => q.status !== "superseded" && q.status !== "declined").length > 0 ? (
                      <ul className="quote-list">
                        {(pr.quotes ?? [])
                          .filter((q) => q.status !== "superseded" && q.status !== "declined")
                          .map((quote) => {
                          const supplier = suppliers.find((item) => item.id === quote.supplier_id);
                          return (
                            <li key={quote.id} className={`quote-row quote-${quote.status}`}>
                              <div>
                                <strong>{supplier?.name ?? quote.supplier_id.slice(0, 8)}</strong>
                                {quote.notes ? <div className="muted">{quote.notes}</div> : null}
                              </div>
                              <strong className="quote-money">KES {quote.unit_cost.toLocaleString()}</strong>
                              <Badge tone={statusTone(quote.status)}>{quote.status}</Badge>
                            </li>
                          );
                        })}
                      </ul>
                    ) : null}
                    {pr.status === "pending" &&
                    (pr.assigned_supplier_id || supplierByRequest[pr.id]) &&
                    !(pr.quotes ?? []).some((q) => q.status === "pending" || q.status === "accepted") ? (
                      <p className="hint">Waiting for the supplier — their price is authorized automatically.</p>
                    ) : null}
                    {pr.issue ? (
                      <div className="auth-block">
                        <span className="muted">Authorization</span>
                        <code className="auth-code">{pr.issue.auth_code}</code>
                        {pr.issue.status === "approved" ? (
                          <div className="inline-form">
                            <Input
                              placeholder="Confirm auth code"
                              value={collectCode[pr.issue.id] ?? pr.issue.auth_code}
                              onChange={(e) =>
                                setCollectCode((m) => ({ ...m, [pr.issue!.id]: e.target.value }))
                              }
                            />
                            <Button
                              type="button"
                              disabled={busy}
                              onClick={() =>
                                void run(() =>
                                  collectSupplierIssue(
                                    pr.issue!.id,
                                    collectCode[pr.issue!.id] ?? pr.issue!.auth_code,
                                  ),
                                )
                              }
                            >
                              Mark collected
                            </Button>
                          </div>
                        ) : null}
                        {pr.issue.status === "collected" && pr.issue.reconciliation_status === "pending" ? (
                          <p className="hint">Collected · credit pending on Suppliers</p>
                        ) : null}
                        {pr.issue.status === "collected" && pr.issue.reconciliation_status === "reconciled" ? (
                          <p className="hint">Collected · credit reconciled</p>
                        ) : null}
                        {pr.issue.status !== "collected" ? (
                          <p className="hint">Reconciliation {pr.issue.reconciliation_status}</p>
                        ) : null}
                      </div>
                    ) : null}
                  </li>
                ))}
              </ul>
            )}
          </section>

          <section className="panel">
            <h2>Payment</h2>
            {balanceDue > 0 ? (
              <p className="hint">
                Balance due: <strong>KES {balanceDue.toLocaleString()}</strong>
                {paidTotal > 0 ? ` (${paidTotal.toLocaleString()} already recorded)` : ""}
              </p>
            ) : job.labor_amount ? (
              <p className="hint">Labor fully covered by recorded payments.</p>
            ) : null}
            <label style={{ display: "block", marginBottom: "0.75rem" }}>
              <input type="checkbox" checked={splitPay} onChange={(e) => setSplitPay(e.target.checked)} /> Split tender
              (multiple methods in one POST)
            </label>
            {!splitPay ? (
            <form
              className="form-grid"
              onSubmit={(e: FormEvent) => {
                e.preventDefault();
                void run(() =>
                  createPayment({
                    method: payMethod,
                    amount: Number(payAmount) || 0,
                    payable_type: "repair",
                    payable_id: job.id,
                    branch_id: job.branch_id,
                    currency: "KES",
                    phone: payMethod === "mpesa_stk" ? payPhone : undefined,
                    account_reference: job.job_code ?? job.id.slice(0, 8),
                  }),
                );
              }}
            >
              <label>
                Method
                <select className="input" value={payMethod} onChange={(e) => setPayMethod(e.target.value)}>
                  <option value="cash">Cash (provisional)</option>
                  <option value="mpesa_stk" disabled={!payCfg?.configured}>
                    M-Pesa STK{!payCfg?.configured ? " — configure in Settings" : ""}
                  </option>
                  <option value="mpesa_c2b" disabled={!payCfg?.configured}>
                    M-Pesa paybill (C2B){!payCfg?.configured ? " — configure in Settings" : ""}
                  </option>
                  <option value="bank_paybill" disabled={!payCfg?.bank_configured}>
                    Bank paybill{!payCfg?.bank_configured ? " — set paybill + account" : ""}
                  </option>
                </select>
              </label>
              <label>
                Amount (KES)
                <Input type="number" value={payAmount} onChange={(e) => setPayAmount(e.target.value)} />
              </label>
              {payMethod === "mpesa_stk" ? (
                <label>
                  Customer phone
                  <Input
                    value={payPhone}
                    onChange={(e) => setPayPhone(e.target.value)}
                    placeholder="07XXXXXXXX"
                    required
                  />
                </label>
              ) : null}
              <Button type="submit" disabled={busy}>
                {payMethod === "mpesa_stk" ? "Send STK push" : payMethod === "mpesa_c2b" ? "Await paybill" : "Record payment"}
              </Button>
            </form>
            ) : (
              <form
                className="form-grid"
                onSubmit={(e: FormEvent) => {
                  e.preventDefault();
                  void run(() =>
                    createPayment({
                      payable_type: "repair",
                      payable_id: job.id,
                      branch_id: job.branch_id,
                      currency: "KES",
                      account_reference: job.job_code ?? job.id.slice(0, 8),
                      tenders: tenders
                        .filter((t) => Number(t.amount) > 0)
                        .map((t) => ({
                          method: t.method,
                          amount: Number(t.amount) || 0,
                          phone: t.method === "mpesa_stk" ? t.phone || undefined : undefined,
                        })),
                    }),
                  );
                }}
              >
                {tenders.map((t, idx) => (
                  <div key={idx} className="inline-form" style={{ gridColumn: "1 / -1" }}>
                    <select
                      className="input"
                      value={t.method}
                      onChange={(e) =>
                        setTenders((rows) =>
                          rows.map((row, i) => (i === idx ? { ...row, method: e.target.value } : row)),
                        )
                      }
                    >
                      <option value="cash">Cash</option>
                      <option value="mpesa_stk">M-Pesa STK</option>
                      <option value="mpesa_c2b">M-Pesa C2B</option>
                      <option value="bank_paybill">Bank paybill</option>
                    </select>
                    <Input
                      type="number"
                      value={t.amount}
                      onChange={(e) =>
                        setTenders((rows) =>
                          rows.map((row, i) => (i === idx ? { ...row, amount: e.target.value } : row)),
                        )
                      }
                      placeholder="Amount"
                    />
                    {t.method === "mpesa_stk" ? (
                      <Input
                        value={t.phone}
                        onChange={(e) =>
                          setTenders((rows) =>
                            rows.map((row, i) => (i === idx ? { ...row, phone: e.target.value } : row)),
                          )
                        }
                        placeholder="Phone"
                      />
                    ) : null}
                    <Button
                      type="button"
                      variant="ghost"
                      disabled={tenders.length <= 1}
                      onClick={() => setTenders((rows) => rows.filter((_, i) => i !== idx))}
                    >
                      Remove
                    </Button>
                  </div>
                ))}
                <div className="btn-row">
                  <Button
                    type="button"
                    variant="secondary"
                    onClick={() => setTenders((rows) => [...rows, { method: "cash", amount: "", phone: "" }])}
                  >
                    Add tender line
                  </Button>
                  <Button type="submit" disabled={busy}>
                    Record split payment
                  </Button>
                </div>
              </form>
            )}
            {payMethod === "mpesa_c2b" && payCfg?.configured ? (
              <p className="hint">
                Customer pays Till/Paybill <span className="mono">{payCfg.mpesa_shortcode}</span> · account{" "}
                <span className="mono">{job.job_code ?? job.id.slice(0, 8)}</span>. Confirmation webhook matches this
                account ref.
              </p>
            ) : null}
            {payCfg?.bank_configured ? (
              <p className="hint">
                Bank paybill <span className="mono">{payCfg.bank_paybill}</span> · account{" "}
                <span className="mono">{payCfg.bank_account}</span>
              </p>
            ) : null}
            {payments.length === 0 ? (
              <EmptyState title="No payments" body="Cash stays provisional until handover is confirmed." />
            ) : (
              <ul className="list">
                {payments.map((p) => (
                  <li key={p.id}>
                    <Badge tone={statusTone(p.status)}>{p.status}</Badge>
                    <span>
                      {p.method.replaceAll("_", " ")} · KES {p.amount.toLocaleString()}
                      {p.checkout_request_id ? ` · ${p.checkout_request_id.slice(0, 16)}…` : ""}
                    </span>
                    {p.method === "mpesa_stk" && (p.status === "initiated" || p.status === "pending") ? (
                      <Button
                        type="button"
                        variant="secondary"
                        disabled={busy}
                        onClick={() => void run(() => reconcileMpesaPayment(p.id))}
                      >
                        Reconcile
                      </Button>
                    ) : null}
                  </li>
                ))}
              </ul>
            )}

            {refundable.length > 0 ? (
              <form
                className="form-grid"
                style={{ marginTop: "1rem" }}
                onSubmit={(e: FormEvent) => {
                  e.preventDefault();
                  if (!refundPaymentId) return;
                  void run(async () => {
                    await createRefund({
                      payment_id: refundPaymentId,
                      amount: Number(refundAmount) || 0,
                      reason: refundReason || undefined,
                    });
                    setRefundReason("");
                  });
                }}
              >
                <h3>Request refund</h3>
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
                  >
                    <option value="">Select…</option>
                    {refundable.map((p) => (
                      <option key={p.id} value={p.id}>
                        {p.method} · KES {p.amount} · {p.status}
                      </option>
                    ))}
                  </select>
                </label>
                <label>
                  Amount
                  <Input type="number" value={refundAmount} onChange={(e) => setRefundAmount(e.target.value)} />
                </label>
                <label>
                  Reason
                  <Input value={refundReason} onChange={(e) => setRefundReason(e.target.value)} />
                </label>
                <Button type="submit" variant="secondary" disabled={busy || !refundPaymentId}>
                  Submit refund
                </Button>
              </form>
            ) : null}
          </section>
        </div>

        <aside className="stack">
          <section className="panel">
            <h2>Customer & device</h2>
            <dl className="meta-dl">
              <dt>Customer</dt>
              <dd>
                {customer ? (
                  <>
                    <strong>{customer.full_name}</strong>
                    {customer.phone ? <div className="muted">{customer.phone}</div> : null}
                    {customer.email ? <div className="muted">{customer.email}</div> : null}
                  </>
                ) : (
                  <span className="muted">Walk-in / anonymous</span>
                )}
              </dd>
              <dt>Device</dt>
              <dd>
                {device ? (
                  <>
                    <strong>
                      {[device.brand, device.model].filter(Boolean).join(" ") || device.kind}
                    </strong>
                    <div className="muted">
                      {device.kind}
                      {device.anonymous ? " · anonymous" : ""}
                    </div>
                    {device.imei ? (
                      <div className="mono muted">IMEI {device.imei}</div>
                    ) : null}
                    {device.serial_number ? (
                      <div className="mono muted">S/N {device.serial_number}</div>
                    ) : null}
                  </>
                ) : (
                  <span className="mono muted">{job.device_id.slice(0, 8)}…</span>
                )}
              </dd>
            </dl>
          </section>

          <section className="panel">
            <h2>Notes</h2>
            <form
              className="inline-form"
              onSubmit={(e: FormEvent) => {
                e.preventDefault();
                if (!noteText.trim()) return;
                void run(async () => {
                  await addRepairNote(job.id, noteText.trim());
                  setNoteText("");
                });
              }}
            >
              <label>
                Add note
                <Input value={noteText} onChange={(e) => setNoteText(e.target.value)} required />
              </label>
              <Button type="submit" disabled={busy || !noteText.trim()}>
                Save
              </Button>
            </form>
            {notes.length === 0 ? (
              <EmptyState title="No notes" body="Internal notes appear here." />
            ) : (
              <ul className="list">
                {notes.map((n) => (
                  <li key={n.id}>
                    <div>
                      <time className="muted">{new Date(n.created_at).toLocaleString()}</time>
                      {n.author_name ? <span className="muted"> · {n.author_name}</span> : null}
                      <p>{n.note}</p>
                    </div>
                  </li>
                ))}
              </ul>
            )}
          </section>

          <section className="panel">
            <h2>Attachments</h2>
            <PhotoCaptureField
              label="Job photos"
              hint="Take a photo or upload — IMEI sticker, damage, after-repair proof."
              onFile={(file) => {
                if (!id) return;
                const reader = new FileReader();
                reader.onload = () => {
                  const result = String(reader.result ?? "");
                  const base64 = result.includes(",") ? result.split(",")[1]! : result;
                  void run(() =>
                    uploadRepairAttachment(id, {
                      file_name: file.name || "job-photo.jpg",
                      content_type: file.type || "image/jpeg",
                      data_base64: base64,
                    }),
                  );
                };
                reader.readAsDataURL(file);
              }}
            />
            <form
              className="inline-form"
              onSubmit={(e: FormEvent<HTMLFormElement>) => {
                e.preventDefault();
                const input = e.currentTarget.elements.namedItem("file") as HTMLInputElement;
                const file = input?.files?.[0];
                if (!file || !id) return;
                const reader = new FileReader();
                reader.onload = () => {
                  const result = String(reader.result ?? "");
                  const base64 = result.includes(",") ? result.split(",")[1]! : result;
                  void run(() =>
                    uploadRepairAttachment(id, {
                      file_name: file.name,
                      content_type: file.type || "application/octet-stream",
                      data_base64: base64,
                    }),
                  );
                };
                reader.readAsDataURL(file);
                e.currentTarget.reset();
              }}
            >
              <label>
                Or upload any file (PDF, image)
                <input className="input" name="file" type="file" accept="image/*,.pdf" />
              </label>
              <Button type="submit" disabled={busy}>
                Upload
              </Button>
            </form>
            {attachments.length === 0 ? (
              <EmptyState title="No attachments" body="Upload photos or documents for this job." />
            ) : (
              <ul className="part-list">
                {attachments.map((a) => (
                  <li key={a.id} className="part-card">
                    <div className="part-head">
                      <div>
                        <strong>{a.file_name}</strong>
                        <div className="muted">
                          {a.content_type} · {(a.size_bytes / 1024).toFixed(1)} KB
                        </div>
                      </div>
                      <div className="btn-row">
                        <Button
                          type="button"
                          variant="secondary"
                          disabled={busy}
                          onClick={() => {
                            if (!id) return;
                            void fetchRepairAttachmentBlob(id, a.id)
                              .then((blob) => {
                                const url = URL.createObjectURL(blob);
                                window.open(url, "_blank");
                                window.setTimeout(() => URL.revokeObjectURL(url), 60_000);
                              })
                              .catch((err) =>
                                setError(err instanceof Error ? err.message : "Download failed"),
                              );
                          }}
                        >
                          View
                        </Button>
                        <Button
                          type="button"
                          variant="ghost"
                          disabled={busy}
                          onClick={() => {
                            if (!id || !confirm(`Delete ${a.file_name}?`)) return;
                            void run(() => deleteRepairAttachment(id, a.id));
                          }}
                        >
                          Delete
                        </Button>
                      </div>
                    </div>
                  </li>
                ))}
              </ul>
            )}
          </section>

          <section className="panel">
            <h2>Status timeline</h2>
            {(job.timeline?.length ?? 0) === 0 ? (
              <EmptyState title="No events" body="Status changes appear here." />
            ) : (
              <ol className="timeline">
                {job.timeline!.map((ev, i) => (
                  <li key={`${ev.at}-${i}`}>
                    <Badge tone={statusTone(ev.status)}>{ev.status.replaceAll("_", " ")}</Badge>
                    <div>
                      <time className="muted">{new Date(ev.at).toLocaleString()}</time>
                      {ev.note ? <p>{ev.note}</p> : null}
                      {ev.by ? <p className="muted mono">by {ev.by.slice(0, 8)}…</p> : null}
                    </div>
                  </li>
                ))}
              </ol>
            )}
          </section>

          <section className="panel context-panel">
            <h2>Trace</h2>
            <dl className="meta-dl">
              <dt>Job</dt>
              <dd className="mono">{job.job_code ?? job.id}</dd>
              <dt>Parts</dt>
              <dd>{parts.length}</dd>
              <dt>Job ID</dt>
              <dd className="mono">{job.id}</dd>
            </dl>
            <p className="hint">
              Every supplier part must stay linked to this job. Orphan parts surface on Risk and Suppliers after
              collection without completion.
            </p>
          </section>
        </aside>
      </div>
    </div>
  );
}
