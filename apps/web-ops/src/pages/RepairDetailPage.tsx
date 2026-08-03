import { useCallback, useEffect, useRef, useState, type FormEvent } from "react";
import { Link, useNavigate, useParams } from "react-router-dom";
import { useRealtimeEvents } from "../lib/realtime";
import { useAuth } from "../auth/AuthContext";
import { Badge, Button, EmptyState, Input, PageHeader, PhotoCaptureField, StkWaitOverlay, Textarea, isTerminalStkError, sleep } from "../components/ui";
import { SearchableCombobox, type ComboOption } from "../components/SearchableCombobox";
import { SendSmsModal } from "../components/SendSmsModal";
import {
  addRepairNote,
  assignPartRequest,
  assignRepair,
  authorizeRepairWork,
  acceptPaidCharge,
  addRepairSaleLine,
  changeRepairStatus,
  collectSupplierIssue,
  ConflictError,
  createRepairEstimate,
  createRepairRework,
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
  getRepairMargin,
  getRepairWarranty,
  issuePartFromStock,
  listCustomers,
  listPartRequests,
  listStockBalances,
  recordHandover,
  removeRepairSaleLine,
  revealRepairPasscode,
  sendHandoverCode,
  listRepairAttachments,
  listRepairEstimates,
  listRepairNotes,
  listRepairPayments,
  listSuppliers,
  listTechnicians,
  openRepairReceipt,
  reconcileMpesaPayment,
  resendRepairIntakeSMS,
  uploadRepairAttachment,
  updateRepairDetails,
  updateRepairSchedule,
  trashRepair,
  type JobMargin,
  type JobSaleLine,
  type PartRequest,
  type Payment,
  type PaymentProviderSettings,
  type RepairAttachment,
  type RepairEstimate,
  type RepairJob,
  type RepairNote,
  type StaffUser,
  type StockBalance,
  type Supplier,
  type Warranty,
} from "../lib/api";
import {
  CLOSURE_REASON_LABELS,
  CLOSURE_STATUS_LABELS,
  compareWorkflowStatus,
  isClosureStatus,
  PART_STATUS_LABELS,
  statusLabel,
  statusTone,
} from "../lib/repairStatus";
import { useCurrency } from "../lib/currency";

function dedupeById<T extends { id: string }>(items: T[]): T[] {
  const seen = new Set<string>();
  return items.filter((item) => {
    if (seen.has(item.id)) return false;
    seen.add(item.id);
    return true;
  });
}

function estimateTotalAmount(e: RepairEstimate): number {
  if (typeof e.total_amount === "number" && Number.isFinite(e.total_amount)) {
    return e.total_amount;
  }
  return (e.labor_amount ?? 0) + (e.parts_amount ?? 0);
}

/** Stages a customer can take the device home from: QC-passed, fixed, or given up on. */
function isCollectableStatus(status: string) {
  return status === "ready_for_pickup" || status === "completed" || isClosureStatus(status);
}

type DetailTab = "now" | "payment" | "parts" | "device" | "history";

function isPickupOrHandoverCode(raw: string) {
  const v = raw.trim();
  return /^PK-/i.test(v) || /REPAIR-PICKUP/i.test(v) || /techlane:\/\/repair-pickup\//i.test(v);
}

function parseHandoverCodeInput(raw: string): { otp_code?: string; pickup_code?: string } {
  const v = raw.trim();
  if (!v) return {};
  if (isPickupOrHandoverCode(v)) {
    const m = v.match(/PK-[A-Z0-9]+/i);
    return { pickup_code: (m?.[0] ?? v).toUpperCase() };
  }
  return { otp_code: v };
}

const REPAIR_STAGES = [
  { key: "intake", label: "Intake" },
  { key: "diagnosed", label: "Diagnosed" },
  { key: "waiting_parts", label: "Waiting parts" },
  { key: "in_progress", label: "On bench" },
  { key: "ready_for_pickup", label: "Ready" },
  { key: "completed", label: "Complete" },
  { key: "collected", label: "Collected" },
] as const;

function StageRail({ status }: { status: string }) {
  if (isClosureStatus(status)) {
    return (
      <nav className="stage-rail stage-rail-closed" aria-label="Repair progress">
        <span className="stage-node stage-done"><i>1</i>Intake</span>
        <span className="stage-line" />
        <span className="stage-node stage-current"><i>×</i>{statusLabel(status)}</span>
        <span className="stage-line" />
        <span className="stage-node"><i>3</i>Return device</span>
      </nav>
    );
  }
  const current = Math.max(0, REPAIR_STAGES.findIndex((stage) => stage.key === status));
  return (
    <nav className="stage-rail" aria-label="Repair progress">
      {REPAIR_STAGES.map((stage, index) => (
        <span className="stage-segment" key={stage.key}>
          <span
            className={`stage-node ${index < current ? "stage-done" : ""} ${index === current ? "stage-current" : ""}`}
            aria-current={index === current ? "step" : undefined}
          >
            <i>{index < current ? "✓" : index + 1}</i>
            {stage.label}
          </span>
          {index < REPAIR_STAGES.length - 1 ? <span className={index < current ? "stage-line stage-line-done" : "stage-line"} /> : null}
        </span>
      ))}
    </nav>
  );
}

const RELATIONSHIPS = ["self", "spouse", "parent", "sibling", "child", "friend", "colleague", "courier", "other"];

/**
 * Releasing a device is the one step in the workflow with no undo, so it asks who
 * is taking it and how we know they are entitled to it before letting go.
 */
function HandoverPanel({
  job,
  balanceDue,
  cashHeldHint,
  busy,
  canVouch,
  promptCode,
  onSendCode,
  onRecord,
  onTakePayment,
}: {
  job: RepairJob;
  balanceDue: number;
  cashHeldHint?: number;
  busy: boolean;
  canVouch: boolean;
  promptCode?: boolean;
  onSendCode: () => Promise<void>;
  onRecord: (body: {
    collected_by_name: string;
    relationship?: string;
    id_number?: string;
    note?: string;
    otp_code?: string;
    pickup_code?: string;
  }) => Promise<void>;
  onTakePayment: () => void;
}) {
  const [name, setName] = useState(job.customer?.full_name ?? "");
  const [relationship, setRelationship] = useState("self");
  const [idNumber, setIdNumber] = useState("");
  const [code, setCode] = useState("");
  const [note, setNote] = useState("");
  const [codeSent, setCodeSent] = useState(false);
  const codeRef = useRef<HTMLInputElement>(null);
  const { formatMoney } = useCurrency();

  const isThirdParty = relationship !== "self";
  const vouching = code.trim() === "";
  const blocked = balanceDue > 0;

  useEffect(() => {
    if (blocked || !promptCode) return;
    const node = document.getElementById("repair-handover");
    node?.scrollIntoView({ behavior: "smooth", block: "start" });
    const t = window.setTimeout(() => codeRef.current?.focus(), 280);
    return () => window.clearTimeout(t);
  }, [blocked, promptCode, job.id]);

  return (
    <div className="action-block handover-block" id="repair-handover">
      <p className="muted">Hand the device over</p>
      {blocked ? (
        <div className="handover-paywall">
          <p className="form-error">
            {formatMoney(balanceDue)} still owed — collect it (or accept paid as full on the Money tab) before release.
          </p>
          <Button type="button" onClick={onTakePayment}>
            Collect {formatMoney(balanceDue)}
          </Button>
        </div>
      ) : null}
      {!blocked && cashHeldHint ? (
        <p className="hint">
          {formatMoney(cashHeldHint)} cash is on the counter. Enter the SMS code from the owner's phone, or scan /
          type the PK- code from the intake slip.
        </p>
      ) : null}
      {!blocked && promptCode && !cashHeldHint ? (
        <p className="hint">Payment received — enter the SMS code or scan the intake QR to release the device.</p>
      ) : null}
      <form
        className="form-grid"
        onSubmit={(e) => {
          e.preventDefault();
          void onRecord({
            collected_by_name: name.trim(),
            relationship,
            id_number: idNumber.trim() || undefined,
            note: note.trim() || undefined,
            ...parseHandoverCodeInput(code),
          });
        }}
      >
        <label>
          Who is collecting it?
          <Input value={name} onChange={(e) => setName(e.target.value)} required disabled={blocked} />
        </label>
        <label>
          Relationship to the owner
          <select
            className="input"
            value={relationship}
            onChange={(e) => setRelationship(e.target.value)}
            disabled={blocked}
          >
            {RELATIONSHIPS.map((r) => (
              <option key={r} value={r}>
                {r === "self" ? "The owner themselves" : r}
              </option>
            ))}
          </select>
        </label>
        {isThirdParty ? (
          <label>
            ID number shown
            <Input
              value={idNumber}
              onChange={(e) => setIdNumber(e.target.value)}
              placeholder="Recommended when someone else collects"
              disabled={blocked}
            />
          </label>
        ) : null}
        <div className="handover-code">
          <label className="tight">
            SMS code or intake slip / QR (PK-…)
            <Input
              ref={codeRef}
              value={code}
              onChange={(e) => setCode(e.target.value)}
              placeholder="6 digits or PK-…"
              inputMode="text"
              autoComplete="one-time-code"
              disabled={blocked}
            />
          </label>
          <Button
            type="button"
            variant="secondary"
            disabled={busy || blocked}
            onClick={() => {
              void onSendCode().then(() => setCodeSent(true));
            }}
          >
            {codeSent ? "Resend code" : "Text a code"}
          </Button>
        </div>
        {codeSent ? <p className="hint">Code sent. It expires in 10 minutes.</p> : null}
        {vouching ? (
          canVouch ? (
            <>
              <p className="hint warn-text">
                Without a code you are personally vouching that this is the right person. That is recorded against
                your name.
              </p>
              <label>
                Why are you releasing it without a code?
                <Input
                  value={note}
                  onChange={(e) => setNote(e.target.value)}
                  placeholder="e.g. owner known to us, ID checked at the counter"
                  disabled={blocked}
                />
              </label>
            </>
          ) : (
            <p className="hint">
              Enter the SMS code or scan the intake QR. Only a manager or owner can release a device without one.
            </p>
          )
        ) : null}
        <Button type="submit" disabled={busy || blocked || !name.trim() || (vouching && !canVouch)}>
          Release device
        </Button>
      </form>
    </div>
  );
}

/** True when the picked "variantId:locationId" line has no buying price recorded. */
function stockNeedsCostPrice(selection: string | undefined, stock: StockBalance[]) {
  if (!selection) return false;
  const [variantId, locationId] = selection.split(":");
  const row = stock.find((b) => b.variant_id === variantId && b.location_id === locationId);
  return row != null && row.cost_price <= 0;
}

function can(permissions: string[] | undefined, permission: string) {
  return permissions?.includes("*") || permissions?.includes(permission);
}


export function RepairDetailPage() {
  const { id } = useParams();
  const navigate = useNavigate();
  const { user } = useAuth();
  const { currencyCode, formatMoney } = useCurrency();
  const [job, setJob] = useState<RepairJob | null>(null);
  const [parts, setParts] = useState<PartRequest[]>([]);
  const [payments, setPayments] = useState<Payment[]>([]);
  const [notes, setNotes] = useState<RepairNote[]>([]);
  const [techs, setTechs] = useState<StaffUser[]>([]);
  const [error, setError] = useState("");
  const [busy, setBusy] = useState(false);
  const [stkPolling, setStkPolling] = useState(false);
  const [stkSuccess, setStkSuccess] = useState("");
  const stkPollRef = useRef(false);
  const [techId, setTechId] = useState("");
  const [partDesc, setPartDesc] = useState("OEM screen");
  const [partSupplierId, setPartSupplierId] = useState("");
  const [suppliers, setSuppliers] = useState<Supplier[]>([]);
  const [supplierByRequest, setSupplierByRequest] = useState<Record<string, string>>({});
  const [stockByRequest, setStockByRequest] = useState<Record<string, string>>({});
  const [stock, setStock] = useState<StockBalance[]>([]);
  const [margin, setMargin] = useState<JobMargin | null>(null);
  const [labor, setLabor] = useState("");
  const [overrideCharge, setOverrideCharge] = useState(false);
  const [payAmount, setPayAmount] = useState("");
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
  const [estimateTotal, setEstimateTotal] = useState("");
  const [estimateNotes, setEstimateNotes] = useState("");
  const [showEstimateModal, setShowEstimateModal] = useState(false);
  const [showSmsModal, setShowSmsModal] = useState(false);
  const [actionMsg, setActionMsg] = useState("");
  const [warranty, setWarranty] = useState<Warranty | null>(null);
  const [warrantyNote, setWarrantyNote] = useState("");
  const [returnReason, setReturnReason] = useState("");
  const [authNote, setAuthNote] = useState("");
  const [authAmount, setAuthAmount] = useState("");
  const [varianceReason, setVarianceReason] = useState("");
  const [promisedBy, setPromisedBy] = useState("");
  const [revealedPasscode, setRevealedPasscode] = useState("");
  const [closing, setClosing] = useState(false);
  const [closureStatus, setClosureStatus] = useState("cancelled");
  const [closureReason, setClosureReason] = useState("");
  const [closureNote, setClosureNote] = useState("");
  const [splitPay, setSplitPay] = useState(false);
  const [tenders, setTenders] = useState([{ method: "cash", amount: "", phone: "" }]);
  const [detailLane, setDetailLane] = useState<DetailTab>("now");
  const [actionsMenuOpen, setActionsMenuOpen] = useState(false);
  const actionsMenuRef = useRef<HTMLDivElement>(null);
  const [handoverPrompt, setHandoverPrompt] = useState(false);
  const [saleStockId, setSaleStockId] = useState("");
  const [saleQty, setSaleQty] = useState("1");
  const [saleAddMode, setSaleAddMode] = useState<"stock" | "custom">("stock");
  const [customSaleDesc, setCustomSaleDesc] = useState("");
  const [customSalePrice, setCustomSalePrice] = useState("");
  const [customSaleQty, setCustomSaleQty] = useState("1");
  const [diagnosisFee, setDiagnosisFee] = useState("");
  const [showFailedPays, setShowFailedPays] = useState(false);
  const [showAccessories, setShowAccessories] = useState(false);
  const [showRefund, setShowRefund] = useState(false);
  const [showEconomics, setShowEconomics] = useState(false);
  const [editingDetails, setEditingDetails] = useState(false);
  const [editConflict, setEditConflict] = useState(false);
  const [editReason, setEditReason] = useState("");
  const [editProblem, setEditProblem] = useState("");
  const [editKind, setEditKind] = useState("phone");
  const [editBrand, setEditBrand] = useState("");
  const [editModel, setEditModel] = useState("");
  const [editImei, setEditImei] = useState("");
  const [editSerial, setEditSerial] = useState("");
  const [editCustomerId, setEditCustomerId] = useState("");
  const [editCustomerLabel, setEditCustomerLabel] = useState("");
  const [editAnonymous, setEditAnonymous] = useState(false);
  const [editCustomerOptions, setEditCustomerOptions] = useState<ComboOption[]>([]);
  const [editCustomerLoading, setEditCustomerLoading] = useState(false);

  const refresh = useCallback(async () => {
    if (!id) return;
    const [j, p, pay, t, cfg, n, att, estimateResult, supplierRes, warrantyRes, stockRes, marginRes] =
      await Promise.all([
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
        listStockBalances().catch(() => ({ items: [] as StockBalance[] })),
        // Cost and margin need reports.read; a technician simply won't see the panel.
        getRepairMargin(id).catch(() => null),
      ]);
    setJob(j);
    setPromisedBy(j.promised_by ? new Date(j.promised_by).toISOString().slice(0, 16) : "");
    setParts(
      [...(p.items ?? [])].sort((a, b) => {
        const rank = (status: string) => {
          if (status === "pending") return 0;
          if (status === "approved") return 1;
          if (status === "issued_from_stock") return 2;
          if (status === "collected") return 3;
          return 4;
        };
        return rank(a.issue?.status || a.status) - rank(b.issue?.status || b.status);
      }),
    );
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
    setStock(stockRes.items ?? []);
    setMargin(marginRes);
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
    } else if (j.authorization?.authorized_amount != null) {
      setLabor(String(j.authorization.authorized_amount));
    }
    setOverrideCharge(false);
    if (j.customer?.phone) {
      setPayPhone((prev) => prev || j.customer!.phone || "");
    }
    {
      const approved = (estimateResult.items ?? []).find((e) => e.status === "approved");
      const due =
        approved != null
          ? estimateTotalAmount(approved)
          : (j.labor_amount ?? 0);
      const paid = dedupeById(pay.items ?? [])
        .filter((p) => ["allocated", "confirmed", "provisional"].includes(p.status))
        .reduce((sum, p) => sum + p.amount, 0);
      const remaining = Math.max(0, due - paid);
      if (remaining > 0 || due > 0) setPayAmount(String(remaining > 0 ? remaining : due));
    }
  }, [id]);

  useEffect(() => {
    refresh().catch((e) => setError(e instanceof Error ? e.message : "Failed to load"));
  }, [refresh]);

  useRealtimeEvents(["repair.*", "part_request.*", "payment.confirmed", "estimate.*"], (event) => {
    const payload = event.payload as { repair_job_id?: string };
    if (payload.repair_job_id && payload.repair_job_id !== id) return;
    refresh().catch(() => undefined);
  });

  // Must stay above early returns — React requires a stable hooks order.
  // Auto-open the Money tab when the counter needs to take payment.
  useEffect(() => {
    if (!job) return;
    const approved = estimates.find((e) => e.status === "approved");
    const lines: JobSaleLine[] = job.sale_lines ?? [];
    const linesTotal =
      typeof job.sale_lines_total === "number"
        ? job.sale_lines_total
        : lines.reduce((s, line) => s + (line.line_total || 0), 0);
    const due =
      (approved != null ? estimateTotalAmount(approved) : (job.labor_amount ?? 0)) + linesTotal;
    const paid = payments
      .filter((p) => ["allocated", "confirmed", "provisional"].includes(p.status))
      .reduce((sum, p) => sum + p.amount, 0);
    const balance = Math.max(0, due - paid);
    if (isCollectableStatus(job.status) && balance > 0) {
      setDetailLane("payment");
    }
  }, [job, estimates, payments]);

  useEffect(() => {
    if (!actionsMenuOpen) return;
    function onPointerDown(e: MouseEvent) {
      if (actionsMenuRef.current && !actionsMenuRef.current.contains(e.target as Node)) {
        setActionsMenuOpen(false);
      }
    }
    function onKeyDown(e: KeyboardEvent) {
      if (e.key === "Escape") setActionsMenuOpen(false);
    }
    document.addEventListener("mousedown", onPointerDown);
    document.addEventListener("keydown", onKeyDown);
    return () => {
      document.removeEventListener("mousedown", onPointerDown);
      document.removeEventListener("keydown", onKeyDown);
    };
  }, [actionsMenuOpen]);

  async function run(action: () => Promise<unknown>) {
    setBusy(true);
    setError("");
    setActionMsg("");
    try {
      await action();
      await refresh();
    } catch (e) {
      setError(e instanceof Error ? e.message : "Action failed");
    } finally {
      setBusy(false);
    }
  }

  function paymentIdFromCreate(res: Payment | { items: Payment[] } | null | undefined): string | null {
    if (!res || typeof res !== "object") return null;
    if ("items" in res && Array.isArray(res.items)) {
      const stk = res.items.find((p) => p.method === "mpesa_stk") ?? res.items[0];
      return stk?.id ?? null;
    }
    const id = (res as Payment).id;
    return typeof id === "string" && id ? id : null;
  }

  function pendingStkPaymentId(list: Payment[] = payments): string | null {
    const pending = list.find(
      (p) => p.method === "mpesa_stk" && (p.status === "initiated" || p.status === "pending"),
    );
    return pending?.id ?? null;
  }

  async function pollStkPayment(paymentId: string) {
    if (!paymentId || stkPollRef.current) return;
    stkPollRef.current = true;
    setStkPolling(true);
    setStkSuccess("");
    setError("");
    try {
      for (let i = 0; i < 48; i++) {
        await sleep(i === 0 ? 1200 : 2500);
        try {
          const p = await reconcileMpesaPayment(paymentId);
          if (p.status === "allocated" || p.status === "confirmed") {
            setStkSuccess("Payment successful");
            await refresh();
            if (job?.id) {
              void openRepairReceipt(job.id).catch(() => undefined);
            }
            const due =
              job && typeof job.balance_due === "number"
                ? job.balance_due
                : null;
            // After refresh, balance may still be stale in closure — treat full STK as paid at counter.
            if (due === null || due - p.amount <= 0.009 || Number(payAmount) + 0.009 >= (balanceDue || 0)) {
              await advanceToHandoverAfterPayment("mpesa_stk");
            }
            window.setTimeout(() => setStkSuccess(""), 2800);
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
      setError("STK timed out — tap Check payment to try again");
    } finally {
      setStkPolling(false);
      stkPollRef.current = false;
    }
  }

  // Auto-poll any open repair STK (same pattern as POS) — covers Send STK, reload mid-wait, and Check payment.
  useEffect(() => {
    const id = pendingStkPaymentId();
    if (!id || stkPollRef.current) return;
    void pollStkPayment(id);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [payments.map((p) => `${p.id}:${p.status}:${p.method}`).join("|")]);

  const canApprove = can(user?.permissions, "parts.approve");
  const canCollectRepair = can(user?.permissions, "repairs.collect");
  const canCloseRepair = can(user?.permissions, "repairs.close");
  const canAuthorizeWork = can(user?.permissions, "repairs.authorize_work");
  const canEditDetails = can(user?.permissions, "repairs.edit");
  const canAcceptPaid =
    canAuthorizeWork || can(user?.permissions, "payments.initiate") || can(user?.permissions, "*");
  const canReleaseUnverified = can(user?.permissions, "repairs.release_unverified") ?? false;

  function openEditDetails() {
    if (!job) return;
    setEditConflict(false);
    setEditReason("");
    setEditProblem(job.problem_summary ?? "");
    setEditKind(job.device?.kind || "phone");
    setEditBrand(job.device?.brand ?? "");
    setEditModel(job.device?.model ?? "");
    setEditImei(job.device?.imei ?? "");
    setEditSerial(job.device?.serial_number ?? "");
    setEditAnonymous(Boolean(job.device?.anonymous) || !job.customer_id);
    setEditCustomerId(job.customer_id ?? "");
    setEditCustomerLabel(
      job.customer
        ? `${job.customer.full_name}${job.customer.phone ? ` · ${job.customer.phone}` : ""}`
        : "",
    );
    setEditCustomerOptions(
      job.customer
        ? [
            {
              value: job.customer.id,
              label: job.customer.full_name,
              sublabel: job.customer.phone,
            },
          ]
        : [],
    );
    setEditingDetails(true);
  }

  function searchEditCustomers(q: string) {
    setEditCustomerLoading(true);
    listCustomers(q)
      .then((res) => {
        setEditCustomerOptions(
          (res.items ?? []).map((c) => ({
            value: c.id,
            label: c.full_name,
            sublabel: c.phone,
          })),
        );
      })
      .catch(() => setEditCustomerOptions([]))
      .finally(() => setEditCustomerLoading(false));
  }

  async function saveEditDetails() {
    if (!job || !editReason.trim()) return;
    setBusy(true);
    setError("");
    setEditConflict(false);
    try {
      const body: Parameters<typeof updateRepairDetails>[1] = {
        expected_version: job.version ?? 1,
        problem_summary: editProblem.trim(),
        device_kind: editKind.trim(),
        device_brand: editBrand.trim(),
        device_model: editModel.trim(),
        device_imei: editImei.trim(),
        device_serial: editSerial.trim(),
        reason: editReason.trim(),
      };
      if (editAnonymous) {
        body.anonymous = true;
      } else if (editCustomerId) {
        body.customer_id = editCustomerId;
        body.anonymous = false;
      }
      await updateRepairDetails(job.id, body);
      setEditingDetails(false);
      await refresh();
    } catch (e) {
      if (e instanceof ConflictError) {
        setEditConflict(true);
        setError("This job changed since you opened it — refresh and try again");
      } else {
        setError(e instanceof Error ? e.message : "Could not save correction");
      }
    } finally {
      setBusy(false);
    }
  }

  if (!job && !error) return <div className="boot">Loading job…</div>;
  if (!job) {
    return (
      <div>
        <PageHeader title="Job" subtitle="Not found" />
        <p className="form-error">{error}</p>
        <Link to="/repairs">Back to jobs</Link>
      </div>
    );
  }

  const next = job.next_statuses ?? [];
  // Closing a job is a different kind of decision from moving it forward, so it
  // gets its own deliberate flow rather than sitting in the same row of chips.
  const closureStatuses = next.filter(isClosureStatus);
  // Collected is not a chip: it means the customer physically has the device, so
  // it is reached by recording the handover in the panel above.
  const forwardStatuses = next
    .filter((s) => !isClosureStatus(s) && s !== "collected")
    .sort(compareWorkflowStatus);
  const reasonOptions = job.closure_reasons?.[closureStatus] ?? [];
  const outstandingParts = parts.filter((pr) => pr.status === "pending" || pr.status === "approved");
  const pendingEstimates = estimates.filter((e) => e.status === "pending");
  const currentStatus = job.status;
  const canRequestParts = ["intake", "diagnosed", "waiting_parts", "in_progress"].includes(currentStatus);
  const finishBlockedByParts = outstandingParts.length > 0;
  const finishBlockedByEstimate = pendingEstimates.length > 0;
  // The gate only bites on the way onto the bench for the first time; a job coming
  // back from waiting_parts was authorized before the parts detour.
  const needsAuthorization =
    !job.authorization?.authorized_at && (currentStatus === "intake" || currentStatus === "diagnosed");
  function statusBlocked(s: string): string | null {
    if (s === "in_progress" && needsAuthorization) return "Authorize the price first";
    const needsClear =
      s === "ready_for_pickup" ||
      s === "completed" ||
      (s === "in_progress" && currentStatus === "waiting_parts");
    if (needsClear && finishBlockedByParts) {
      return "Clear outstanding supplier parts first";
    }
    if ((s === "ready_for_pickup" || s === "completed") && finishBlockedByEstimate) {
      return "Customer estimate is still pending";
    }
    return null;
  }
  const authorizedAmount = job.authorization?.authorized_amount;
  const agreedCharge =
    authorizedAmount ??
    (job.labor_amount && job.labor_amount > 0 ? job.labor_amount : undefined);
  const overrunAmount =
    !overrideCharge || authorizedAmount === undefined
      ? 0
      : Math.max(0, (Number(labor) || 0) - authorizedAmount);
  const customer = job.customer;
  const device = job.device;
  const refundable = payments.filter(
    (p) =>
      p.status === "allocated" ||
      p.status === "confirmed" ||
      p.status === "provisional",
  );
  const paidTotal = payments
    .filter((p) => ["allocated", "confirmed", "provisional"].includes(p.status))
    .reduce((sum, p) => sum + p.amount, 0);
  const livePayments = payments.filter((p) => p.status !== "failed" && p.status !== "cancelled");
  const failedPayments = payments.filter((p) => p.status === "failed" || p.status === "cancelled");
  const cashHeld = payments
    .filter((p) => p.method === "cash" && p.status === "provisional")
    .reduce((sum, p) => sum + p.amount, 0);
  // Prefer server rollups (same formula as collection / receipts) when present.
  const approvedEstimate = estimates.find((e) => e.status === "approved");
  const saleLines: JobSaleLine[] = job.sale_lines ?? [];
  const saleLinesTotal =
    typeof job.sale_lines_total === "number"
      ? job.sale_lines_total
      : saleLines.reduce((s, line) => s + (line.line_total || 0), 0);
  const amountDue =
    typeof job.amount_due === "number"
      ? job.amount_due
      : (approvedEstimate != null ? estimateTotalAmount(approvedEstimate) : (job.labor_amount ?? 0)) +
        saleLinesTotal;
  const balanceDue =
    typeof job.balance_due === "number" ? job.balance_due : Math.max(0, amountDue - paidTotal);
  const paymentLocked = amountDue > 0 && balanceDue <= 0;
  const promiseOverdue =
    Boolean(job.promised_by) &&
    new Date(job.promised_by!).getTime() < Date.now() &&
    !isCollectableStatus(job.status);

  function goTakePayment() {
    setPayAmount(String(balanceDue > 0 ? balanceDue : amountDue || ""));
    if (customer?.phone) setPayPhone(customer.phone);
    if (payCfg?.configured) setPayMethod("mpesa_stk");
    goToSection("repair-payment");
  }

  function goToSection(sectionId: string) {
    const tab: DetailTab =
      sectionId === "repair-payment"
        ? "payment"
        : sectionId === "job-parts"
          ? "parts"
          : sectionId === "job-customer"
            ? "device"
            : sectionId === "job-history"
              ? "history"
              : "now";
    setDetailLane(tab);
    window.setTimeout(() => {
      document.getElementById(sectionId)?.scrollIntoView({ behavior: "smooth", block: "start" });
    }, 40);
  }

  async function advanceToHandoverAfterPayment(method: string) {
    if (!job || !isCollectableStatus(job.status) || job.handover) return;
    setDetailLane("now");
    setHandoverPrompt(true);
    // Cash at the counter means the customer is here — text the release code now.
    if (method === "cash" || method === "mpesa_c2b" || method === "bank_paybill") {
      try {
        await sendHandoverCode(job.id);
      } catch {
        // Code send is best-effort; staff can still use the intake QR / PK- code.
      }
    }
    window.requestAnimationFrame(() => {
      document.getElementById("repair-handover")?.scrollIntoView({ behavior: "smooth", block: "start" });
    });
  }

  return (
    <div className="repair-detail">
      <StkWaitOverlay
        visible={stkPolling}
        message="Waiting for M-Pesa"
        detail="Approve the STK prompt on the phone. We’ll confirm payment automatically."
        success={stkSuccess}
      />
      <PageHeader
        title={job.job_code ?? `Job ${job.id.slice(0, 8)}`}
        subtitle={`${statusLabel(job.status)} · ${job.problem_summary}`}
        actions={
          <div className="btn-row">
            <Link to="/repairs" className="muted">
              All repairs
            </Link>
            <div className="actions-menu" ref={actionsMenuRef} style={{ position: "relative" }}>
              <Button
                type="button"
                variant="secondary"
                aria-haspopup="menu"
                aria-expanded={actionsMenuOpen}
                onClick={() => setActionsMenuOpen((open) => !open)}
              >
                Actions
              </Button>
              {actionsMenuOpen ? (
                <div
                  className="panel"
                  role="menu"
                  style={{
                    position: "absolute",
                    right: 0,
                    top: "calc(100% + 4px)",
                    minWidth: "12rem",
                    padding: "0.35rem 0",
                    zIndex: 30,
                  }}
                >
                  {canEditDetails && job.status !== "collected" ? (
                    <button
                      type="button"
                      role="menuitem"
                      className="btn btn-ghost"
                      style={{ display: "block", width: "100%", textAlign: "left" }}
                      disabled={busy}
                      onClick={() => {
                        setActionsMenuOpen(false);
                        if (editingDetails) setEditingDetails(false);
                        else openEditDetails();
                      }}
                    >
                      {editingDetails ? "Cancel edit" : "Edit details"}
                    </button>
                  ) : null}
                  <button
                    type="button"
                    role="menuitem"
                    className="btn btn-ghost"
                    style={{ display: "block", width: "100%", textAlign: "left" }}
                    disabled={busy}
                    onClick={() => {
                      setActionsMenuOpen(false);
                      void run(async () => {
                        await openRepairReceipt(job.id);
                      });
                    }}
                  >
                    Print receipt
                  </button>
                  <button
                    type="button"
                    role="menuitem"
                    className="btn btn-ghost"
                    style={{ display: "block", width: "100%", textAlign: "left" }}
                    disabled={busy}
                    onClick={() => {
                      setActionsMenuOpen(false);
                      void run(() => downloadRepairReceiptPDF(job.id));
                    }}
                  >
                    Receipt PDF
                  </button>
                  <button
                    type="button"
                    role="menuitem"
                    className="btn btn-ghost"
                    style={{ display: "block", width: "100%", textAlign: "left" }}
                    disabled={busy}
                    onClick={() => {
                      setActionsMenuOpen(false);
                      void run(() => downloadRepairTaxInvoicePDF(job.id));
                    }}
                  >
                    Tax invoice PDF
                  </button>
                  {canCloseRepair && !["completed", "collected", "cancelled", "unrepairable"].includes(job.status) ? (
                    <button
                      type="button"
                      role="menuitem"
                      className="btn btn-ghost"
                      style={{ display: "block", width: "100%", textAlign: "left", color: "var(--danger, #c0392b)" }}
                      disabled={busy}
                      onClick={() => {
                        setActionsMenuOpen(false);
                        if (!window.confirm(`Move ${job.job_code ?? "this job"} to Trashed?`)) return;
                        void run(async () => {
                          await trashRepair(job.id);
                          navigate("/trash");
                        });
                      }}
                    >
                      Move to trash
                    </button>
                  ) : null}
                </div>
              ) : null}
            </div>
          </div>
        }
      />
      {error ? <p className="form-error">{error}</p> : null}
      {actionMsg ? <p className="form-success">{actionMsg}</p> : null}
      {editingDetails ? (
        <section className="panel" aria-label="Correct job details">
          <h2>Correct intake details</h2>
          <p className="muted">
            Every change is audited with your reason. Collected jobs cannot be rewritten.
          </p>
          {editConflict ? (
            <div className="btn-row" style={{ marginBottom: "0.75rem" }}>
              <p className="form-error" style={{ margin: 0 }}>
                This job changed since you opened it — refresh and try again
              </p>
              <Button
                type="button"
                variant="secondary"
                disabled={busy}
                onClick={() =>
                  void (async () => {
                    await refresh();
                    openEditDetails();
                  })()
                }
              >
                Reload
              </Button>
            </div>
          ) : null}
          <div className="form-grid">
            <label>
              Problem summary
              <Input value={editProblem} onChange={(e) => setEditProblem(e.target.value)} />
            </label>
            <label>
              Device kind
              <Input value={editKind} onChange={(e) => setEditKind(e.target.value)} />
            </label>
            <label>
              Brand
              <Input value={editBrand} onChange={(e) => setEditBrand(e.target.value)} />
            </label>
            <label>
              Model
              <Input value={editModel} onChange={(e) => setEditModel(e.target.value)} />
            </label>
            <label>
              IMEI
              <Input value={editImei} onChange={(e) => setEditImei(e.target.value)} />
            </label>
            <label>
              Serial
              <Input value={editSerial} onChange={(e) => setEditSerial(e.target.value)} />
            </label>
          </div>
          <div style={{ marginTop: "0.75rem" }}>
            <label className="check-row">
              <input
                type="checkbox"
                checked={editAnonymous}
                onChange={(e) => {
                  setEditAnonymous(e.target.checked);
                  if (e.target.checked) {
                    setEditCustomerId("");
                    setEditCustomerLabel("");
                  }
                }}
              />
              Anonymous / walk-in (no customer record)
            </label>
            {!editAnonymous ? (
              <div style={{ marginTop: "0.5rem" }}>
                <SearchableCombobox
                  label="Customer"
                  placeholder="Search by name or phone"
                  options={editCustomerOptions}
                  onSearch={searchEditCustomers}
                  value={editCustomerLabel}
                  loading={editCustomerLoading}
                  onSelect={(opt) => {
                    setEditCustomerId(opt.value);
                    setEditCustomerLabel(opt.sublabel ? `${opt.label} · ${opt.sublabel}` : opt.label);
                  }}
                />
              </div>
            ) : null}
          </div>
          <label style={{ display: "block", marginTop: "0.85rem" }}>
            Reason for this correction
            <Input
              value={editReason}
              onChange={(e) => setEditReason(e.target.value)}
              placeholder="e.g. IMEI mistyped at intake"
              required
            />
          </label>
          <div className="btn-row" style={{ marginTop: "0.85rem" }}>
            <Button
              type="button"
              disabled={busy || !editReason.trim() || !editProblem.trim() || !editKind.trim()}
              onClick={() => void saveEditDetails()}
            >
              Save correction
            </Button>
            <Button type="button" variant="ghost" disabled={busy} onClick={() => setEditingDetails(false)}>
              Cancel
            </Button>
          </div>
        </section>
      ) : null}
      {finishBlockedByEstimate || promiseOverdue ? (
        <div className="blocked-banner" role="status" style={{ marginBottom: "1rem", flexDirection: "column", alignItems: "flex-start", gap: "0.35rem" }}>
          {finishBlockedByEstimate ? (
            <span>A customer estimate is still pending approval.</span>
          ) : null}
          {promiseOverdue ? (
            <span>
              Promise overdue — was due {new Date(job.promised_by!).toLocaleString()}.
            </span>
          ) : null}
        </div>
      ) : null}
      <StageRail status={job.status} />
      {job.parent_job_id ? (
        <div className="rework-banner">
          <Badge tone="warning">customer return</Badge>
          <span>
            Linked to{" "}
            <Link to={`/repairs/${job.parent_job_id}`}>{job.parent_job_code ?? job.parent_job_id.slice(0, 8)}</Link>
            {job.rework_reason ? ` · ${job.rework_reason}` : ""}
          </span>
        </div>
      ) : null}

      {job.closure_reason ? (
        <div className="job-closed-banner" role="status">
          <Badge tone="danger">{job.status === "unrepairable" ? "unrepairable" : "cancelled"}</Badge>
          <span>
            {CLOSURE_REASON_LABELS[job.closure_reason] ?? job.closure_reason.replaceAll("_", " ")}
            {job.closed_at ? ` · closed ${new Date(job.closed_at).toLocaleString()}` : ""}
          </span>
          {job.status !== "collected" ? <strong>Device still needs to be returned to the customer.</strong> : null}
        </div>
      ) : null}

      <section className="job-hero" aria-label="Job snapshot">
        <div>
          <span>Customer</span>
          <strong>{customer?.full_name ?? "Walk-in"}</strong>
          <div className="muted">{customer?.phone ?? "No phone"}</div>
          {customer?.phone ? (
            <div className="btn-row" style={{ marginTop: "0.45rem", flexWrap: "wrap" }}>
              <Button type="button" variant="secondary" disabled={busy} onClick={() => setShowSmsModal(true)}>
                Send SMS
              </Button>
              <Button
                type="button"
                variant="ghost"
                disabled={busy}
                onClick={() =>
                  void run(async () => {
                    const res = await resendRepairIntakeSMS(job.id);
                    window.alert(
                      `Intake SMS queued to ${res.phone} (${res.template_key === "repair.wait_bench" ? "wait bench" : "intake"}).`,
                    );
                  })
                }
              >
                Resend intake SMS
              </Button>
            </div>
          ) : null}
        </div>
        <div>
          <span>Device</span>
          <strong>
            {device ? [device.brand, device.model].filter(Boolean).join(" ") || device.kind : "Device"}
          </strong>
          <div className="mono muted">{device?.imei ? `IMEI ${device.imei}` : "No IMEI on file"}</div>
        </div>
        <div>
          <span>Money</span>
          <strong>{formatMoney(amountDue)}</strong>
          <div className={balanceDue > 0.009 ? "warn-text" : "muted"}>
            {amountDue <= 0
              ? "No charge set"
              : balanceDue <= 0.009
                ? `Paid in full · ${formatMoney(paidTotal)}`
                : `Paid ${formatMoney(paidTotal)} · ${formatMoney(balanceDue)} left`}
          </div>
          {balanceDue > 0.009 ? (
            <div className="btn-row" style={{ marginTop: "0.45rem", flexWrap: "wrap" }}>
              <Button type="button" variant="secondary" onClick={goTakePayment}>
                Collect {formatMoney(balanceDue)}
              </Button>
              {canAcceptPaid && paidTotal > 0 ? (
                <Button
                  type="button"
                  variant="ghost"
                  disabled={busy}
                  title="Set the job charge to what was already paid and clear the remainder"
                  onClick={() =>
                    void run(async () => {
                      if (
                        !window.confirm(
                          `Accept ${formatMoney(paidTotal)} as the full charge and clear the ${formatMoney(balanceDue)} remainder?`,
                        )
                      ) {
                        return;
                      }
                      await acceptPaidCharge(job.id);
                    })
                  }
                >
                  Accept paid as full
                </Button>
              ) : null}
            </div>
          ) : null}
        </div>
        <div>
          <span>Bench</span>
          <strong>
            {job.technician_id
              ? techs.find((t) => t.id === job.technician_id)?.display_name ?? "Assigned"
              : "Unassigned"}
          </strong>
          <div className={job.promised_by && new Date(job.promised_by).getTime() < Date.now() && !isCollectableStatus(job.status) ? "warn-text" : "muted"}>
            {job.customer_waiting
              ? `Wait bench${job.estimated_wait_minutes ? ` · ~${job.estimated_wait_minutes} min` : ""}${
                  job.promised_by ? ` · ready ~${new Date(job.promised_by).toLocaleTimeString([], { hour: "2-digit", minute: "2-digit" })}` : ""
                }`
              : job.promised_by
                ? `Promise ${new Date(job.promised_by).toLocaleString()}`
                : "No promise set"}
          </div>
        </div>
      </section>

      <div className="detail-workspace">
        <nav className="job-section-nav" role="tablist" aria-label="Job sections">
          <button
            type="button"
            role="tab"
            aria-selected={detailLane === "now"}
            className={detailLane === "now" ? "active" : undefined}
            onClick={() => setDetailLane("now")}
          >
            Now
          </button>
          <button
            type="button"
            role="tab"
            aria-selected={detailLane === "payment"}
            className={detailLane === "payment" ? "active" : undefined}
            onClick={() => setDetailLane("payment")}
          >
            Payment
          </button>
          <button
            type="button"
            role="tab"
            aria-selected={detailLane === "parts"}
            className={detailLane === "parts" ? "active" : undefined}
            onClick={() => setDetailLane("parts")}
          >
            Parts
          </button>
          <button
            type="button"
            role="tab"
            aria-selected={detailLane === "device"}
            className={detailLane === "device" ? "active" : undefined}
            onClick={() => setDetailLane("device")}
          >
            Customer & device
          </button>
          <button
            type="button"
            role="tab"
            aria-selected={detailLane === "history"}
            className={detailLane === "history" ? "active" : undefined}
            onClick={() => setDetailLane("history")}
          >
            History
          </button>
        </nav>

        {detailLane === "now" ? (
          <div className="lane-stack">
          <section className="panel" id="job-now">
            <h2>Now</h2>
            {paymentLocked && job.status !== "collected" && !job.handover ? (
              <div className="job-paid-banner" role="status">
                <Badge tone="success">Paid in full</Badge>
                <span>
                  Money is settled — that does not finish the repair. Keep moving the job through the workshop until
                  it is <strong>Ready</strong>, then hand the device back.
                </span>
              </div>
            ) : null}
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

            {needsAuthorization ? (
              <div className="action-block authorize-block">
                <p className="form-error">
                  Work is not authorized yet — nobody has agreed to a price, so this job cannot go on the bench.
                </p>
                <p className="hint">
                  Send the customer an estimate to approve, or record their verbal go-ahead here if they are at the
                  counter.
                </p>
                <div className="btn-row" style={{ marginBottom: "0.75rem" }}>
                  <Button
                    type="button"
                    disabled={busy || !canRequestParts}
                    onClick={() => setShowEstimateModal(true)}
                  >
                    Send customer estimate
                  </Button>
                </div>
                {canAuthorizeWork ? (
                  <form
                    className="form-grid"
                    onSubmit={(e: FormEvent) => {
                      e.preventDefault();
                      if (!authNote.trim()) {
                        setError("Record how the customer agreed — this is the audit trail for the price");
                        return;
                      }
                      void run(async () => {
                        await authorizeRepairWork(job.id, {
                          amount: authAmount === "" ? undefined : Number(authAmount),
                          note: authNote.trim(),
                        });
                        setAuthNote("");
                        setAuthAmount("");
                      });
                    }}
                  >
                    <label>
                      Agreed price ({currencyCode})
                      <Input
                        type="number"
                        min="0"
                        value={authAmount}
                        onChange={(e) => setAuthAmount(e.target.value)}
                        placeholder={String(job.labor_amount ?? 0)}
                      />
                    </label>
                    <label>
                      How did they agree?
                      <Input
                        value={authNote}
                        onChange={(e) => setAuthNote(e.target.value)}
                        placeholder="e.g. verbal go-ahead at the counter"
                        required
                      />
                    </label>
                    <div className="btn-row">
                      <Button type="submit" disabled={busy || !authNote.trim()}>
                        Authorize work
                      </Button>
                    </div>
                  </form>
                ) : (
                  <p className="hint">Only a manager or owner can record a go-ahead without a written estimate.</p>
                )}
              </div>
            ) : null}

            {job.handover ? (
              <div className="action-block handover-done">
                <p className="muted">Handed over</p>
                <p>
                  <strong>{job.handover.collected_by_name}</strong>
                  {job.handover.relationship !== "self" ? ` (${job.handover.relationship})` : ""} collected this
                  device
                  {job.handover.verified_at ? ` on ${new Date(job.handover.verified_at).toLocaleString()}` : ""}.
                </p>
                <Badge tone={job.handover.verification_method === "otp" ? "success" : "warning"}>
                  {job.handover.verification_method === "otp"
                    ? "Confirmed by code on the owner's phone"
                    : "Released by staff without a code"}
                </Badge>
                {job.handover.id_number ? (
                  <p className="muted">ID recorded: {job.handover.id_number}</p>
                ) : null}
                {job.handover.note ? <p className="muted">{job.handover.note}</p> : null}
              </div>
            ) : canCollectRepair && isCollectableStatus(job.status) ? (
              <HandoverPanel
                job={job}
                balanceDue={balanceDue}
                cashHeldHint={cashHeld}
                busy={busy}
                canVouch={canReleaseUnverified}
                promptCode={handoverPrompt}
                onSendCode={() => run(() => sendHandoverCode(job.id))}
                onRecord={(body) => run(() => recordHandover(job.id, body))}
                onTakePayment={goTakePayment}
              />
            ) : null}

            {forwardStatuses.length > 0 ? (
              <div className="action-block">
                <p className="muted">
                  {paymentLocked ? "Next workshop step" : "Update status"}
                </p>
                <div className="chip-row">
                  {forwardStatuses
                    .map((s) => {
                      const blockReason = statusBlocked(s);
                      return (
                        <Button
                          key={s}
                          type="button"
                          variant={s === "completed" || s === "ready_for_pickup" || s === "in_progress" ? "primary" : "secondary"}
                          disabled={busy || Boolean(blockReason)}
                          title={blockReason ?? undefined}
                          onClick={() =>
                            void run(async () => {
                              const body: {
                                status: string;
                                labor_amount?: number;
                                variance_reason?: string;
                              } = { status: s };
                              if (s === "completed") {
                                const hasAgreed = agreedCharge != null && agreedCharge > 0;
                                if (!hasAgreed || overrideCharge) {
                                  const amount = Number(labor);
                                  if (!Number.isFinite(amount) || amount < 0) {
                                    throw new Error("Enter a valid final charge");
                                  }
                                  body.labor_amount = amount;
                                  if (varianceReason.trim()) {
                                    body.variance_reason = varianceReason.trim();
                                  }
                                }
                                // else: leave labor_amount unset — API keeps the agreed price on the job
                              }
                              await changeRepairStatus(job.id, body);
                              setOverrideCharge(false);
                            })
                          }
                        >
                          → {statusLabel(s)}
                        </Button>
                      );
                    })}
                </div>
                {needsAuthorization ? (
                  <p className="form-error" role="status">
                    Blocked until a price is agreed.{" "}
                    <Button
                      type="button"
                      variant="secondary"
                      disabled={busy || !canRequestParts}
                      onClick={() => setShowEstimateModal(true)}
                    >
                      Send customer estimate
                    </Button>
                  </p>
                ) : null}
                {finishBlockedByParts || finishBlockedByEstimate ? (
                  <p className="form-error" role="status">
                    {finishBlockedByParts
                      ? `Waiting on ${outstandingParts.length} supplier part(s) before ready/complete.`
                      : null}
                    {finishBlockedByParts && finishBlockedByEstimate ? " " : null}
                    {finishBlockedByEstimate
                      ? "A customer estimate is still pending approval."
                      : null}
                  </p>
                ) : null}
                {next.includes("completed") ? (
                  <>
                    {agreedCharge != null && agreedCharge > 0 && !overrideCharge ? (
                      <div className="hint">
                        <p>
                          Customer will be charged the agreed price:{" "}
                          <strong>{formatMoney(agreedCharge)}</strong>
                        </p>
                        <Button
                          type="button"
                          variant="ghost"
                          disabled={busy}
                          onClick={() => {
                            setOverrideCharge(true);
                            setLabor(String(agreedCharge));
                          }}
                        >
                          Charge a different amount
                        </Button>
                      </div>
                    ) : (
                      <>
                        <label className="tight">
                          Final charge ({currencyCode})
                          <Input type="number" value={labor} onChange={(e) => setLabor(e.target.value)} />
                        </label>
                        {overrunAmount > 0 ? (
                          <label className="tight">
                            <span className="warn-text">
                              This is {formatMoney(overrunAmount)} more than the {formatMoney(authorizedAmount!)}{" "}
                              the customer agreed to — explain why before completing.
                            </span>
                            <Input
                              value={varianceReason}
                              onChange={(e) => setVarianceReason(e.target.value)}
                              placeholder="e.g. board-level fault found, customer approved by phone"
                              required
                            />
                          </label>
                        ) : null}
                        {agreedCharge != null && agreedCharge > 0 ? (
                          <Button
                            type="button"
                            variant="ghost"
                            disabled={busy}
                            onClick={() => {
                              setOverrideCharge(false);
                              setVarianceReason("");
                            }}
                          >
                            Use agreed price instead
                          </Button>
                        ) : null}
                      </>
                    )}
                  </>
                ) : null}
              </div>
            ) : (
              <p className="hint">No further status changes available.</p>
            )}

            {closureStatuses.length > 0 && canCloseRepair ? (
              <div className="action-block close-out">
                {!closing ? (
                  <button type="button" className="linkish danger-link" onClick={() => setClosing(true)}>
                    Close this job without repairing it
                  </button>
                ) : (
                  <form
                    className="form-grid"
                    onSubmit={(e: FormEvent) => {
                      e.preventDefault();
                      if (!closureReason) {
                        setError("Choose a reason before closing the job");
                        return;
                      }
                      void (async () => {
                        setBusy(true);
                        setError("");
                        setActionMsg("");
                        try {
                          await changeRepairStatus(job.id, {
                            status: closureStatus,
                            closure_reason: closureReason,
                            note: closureNote.trim() || undefined,
                          });
                          const fee = Number(diagnosisFee);
                          if (Number.isFinite(fee) && fee > 0) {
                            try {
                              await addRepairSaleLine(job.id, {
                                description: "Diagnosis fee",
                                unit_price: fee,
                                quantity: 1,
                              });
                            } catch (feeErr) {
                              setError(
                                feeErr instanceof Error
                                  ? `Job closed, but diagnosis fee was not recorded: ${feeErr.message}. Add it under accessories.`
                                  : "Job closed, but diagnosis fee was not recorded. Add it under accessories.",
                              );
                              await refresh();
                              setClosing(false);
                              setClosureReason("");
                              setClosureNote("");
                              setDiagnosisFee("");
                              return;
                            }
                          }
                          setClosing(false);
                          setClosureReason("");
                          setClosureNote("");
                          setDiagnosisFee("");
                          setActionMsg(
                            Number.isFinite(fee) && fee > 0
                              ? `Job closed — diagnosis fee ${formatMoney(fee)} added to the bill.`
                              : "Job closed.",
                          );
                          await refresh();
                        } catch (closeErr) {
                          setError(closeErr instanceof Error ? closeErr.message : "Could not close job");
                        } finally {
                          setBusy(false);
                        }
                      })();
                    }}
                  >
                    <h3>Close without repair</h3>
                    <p className="hint">
                      No commission, warranty or loyalty points are issued. Any price quoted at intake is cleared so
                      the device can be handed back. Add an optional diagnosis fee below to put it on the same bill.
                    </p>
                    <label>
                      Outcome
                      <select
                        className="input"
                        value={closureStatus}
                        onChange={(e) => {
                          setClosureStatus(e.target.value);
                          setClosureReason("");
                        }}
                      >
                        {closureStatuses.map((s) => (
                          <option key={s} value={s}>
                            {CLOSURE_STATUS_LABELS[s] ?? s}
                          </option>
                        ))}
                      </select>
                    </label>
                    <label>
                      Reason
                      <select
                        className="input"
                        value={closureReason}
                        onChange={(e) => setClosureReason(e.target.value)}
                        required
                      >
                        <option value="">Select a reason…</option>
                        {reasonOptions.map((code) => (
                          <option key={code} value={code}>
                            {CLOSURE_REASON_LABELS[code] ?? code.replaceAll("_", " ")}
                          </option>
                        ))}
                      </select>
                    </label>
                    <label>
                      Note (optional)
                      <Input
                        value={closureNote}
                        onChange={(e) => setClosureNote(e.target.value)}
                        placeholder="What you told the customer"
                      />
                    </label>
                    <label>
                      Diagnosis fee ({currencyCode}, optional)
                      <Input
                        type="number"
                        min="0"
                        step="0.01"
                        value={diagnosisFee}
                        onChange={(e) => setDiagnosisFee(e.target.value)}
                        placeholder="0"
                      />
                    </label>
                    <div className="btn-row">
                      <Button type="submit" variant="danger" disabled={busy || !closureReason}>
                        Close job
                      </Button>
                      <Button type="button" variant="ghost" onClick={() => setClosing(false)}>
                        Cancel
                      </Button>
                    </div>
                  </form>
                )}
              </div>
            ) : null}
          </section>
          </div>
        ) : null}

          {detailLane === "parts" ? (
            <div className="lane-stack">
          <section className="panel" id="job-parts">
            <h2>Parts (anti-leakage)</h2>
            {outstandingParts.length > 0 ? (
              <div className="blocked-banner" role="status">
                <strong>
                  Blocked on {outstandingParts.length} part{outstandingParts.length === 1 ? "" : "s"}
                </strong>
                <span>
                  {outstandingParts.map((pr) => pr.description).join(", ")} — the job returns to the bench
                  automatically once the last one is collected.
                </span>
              </div>
            ) : null}
            {canRequestParts ? (
            <form
              className="inline-form"
              onSubmit={(e: FormEvent) => {
                e.preventDefault();
                if (busy) return;
                if (!partSupplierId) {
                  setError("Select a supplier for this part request");
                  return;
                }
                if (!partDesc.trim()) {
                  setError("Describe the part");
                  return;
                }
                void run(async () => {
                  await createPartRequest({
                    repair_job_id: job.id,
                    branch_id: job.branch_id,
                    description: partDesc.trim(),
                    quantity: 1,
                    supplier_id: partSupplierId,
                  });
                  setPartDesc("");
                  setError("");
                });
              }}
            >
              <label>
                Request part
                <Input
                  value={partDesc}
                  onChange={(e) => setPartDesc(e.target.value)}
                  required
                  disabled={busy}
                />
              </label>
              <label>
                Supplier
                <select
                  className="input"
                  value={partSupplierId}
                  onChange={(e) => setPartSupplierId(e.target.value)}
                  required
                  disabled={busy}
                >
                  <option value="">Select supplier…</option>
                  {suppliers.map((supplier) => (
                    <option key={supplier.id} value={supplier.id}>
                      {supplier.name}
                      {supplier.phone ? ` · ${supplier.phone}` : ""}
                    </option>
                  ))}
                </select>
              </label>
              <Button type="submit" disabled={busy || !partSupplierId || !partDesc.trim()}>
                {busy ? "Requesting…" : "Request"}
              </Button>
            </form>
            ) : (
              <p className="muted">Parts can only be requested while the job is on the bench.</p>
            )}

            {parts.length === 0 ? (
              <EmptyState title="No parts yet" body="Request a screen or part to create a supplier authorization trail." />
            ) : (
              <ul className="part-list">
                {parts.map((pr) => (
                  <li key={pr.id} className="part-card">
                    <div className="part-head">
                      <strong>{pr.description}</strong>
                      <Badge tone={statusTone(pr.issue?.status || pr.status)}>
                        {PART_STATUS_LABELS[pr.issue?.status || pr.status] ??
                          statusLabel(pr.issue?.status || pr.status)}
                      </Badge>
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
                    {(pr.status === "pending" || pr.status === "approved") && canApprove ? (
                      <div className="assign-block">
                        <span className="muted">Or take it off our own shelf</span>
                        <div className="inline-form">
                          <select
                            className="input"
                            aria-label="Stock item to issue"
                            value={stockByRequest[pr.id] ?? ""}
                            onChange={(e) =>
                              setStockByRequest((current) => ({ ...current, [pr.id]: e.target.value }))
                            }
                          >
                            <option value="">Select a stock item…</option>
                            {stock
                              .filter((b) => b.available_qty > 0)
                              .map((b) => (
                                <option key={`${b.variant_id}:${b.location_id}`} value={`${b.variant_id}:${b.location_id}`}>
                                  {b.product_name} · {b.sku} — {b.available_qty} at {b.location_name}
                                  {b.cost_price > 0 ? ` · cost ${b.cost_price.toLocaleString()}` : " · no cost price"}
                                </option>
                              ))}
                          </select>
                          <Button
                            type="button"
                            variant="secondary"
                            disabled={busy || !stockByRequest[pr.id]}
                            onClick={() => {
                              const [variantId, locationId] = (stockByRequest[pr.id] ?? "").split(":");
                              if (!variantId || !locationId) return;
                              void run(async () => {
                                await issuePartFromStock(pr.id, {
                                  variant_id: variantId,
                                  location_id: locationId,
                                  quantity: pr.quantity || 1,
                                });
                                setStockByRequest((current) => ({ ...current, [pr.id]: "" }));
                              });
                            }}
                          >
                            Issue from stock
                          </Button>
                        </div>
                        <p className="hint">
                          Stock is decremented and the part's cost is booked to this job, so margin stays honest.
                        </p>
                        {stockNeedsCostPrice(stockByRequest[pr.id], stock) ? (
                          <p className="hint warn-text">
                            That item has no cost price, so this job's margin will read higher than it really is. Set
                            one under Inventory first.
                          </p>
                        ) : null}
                      </div>
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
            </div>
          ) : null}

          {detailLane === "payment" ? (
            <div className="lane-stack">
            {job.authorization?.authorized_at ? (
              <div className="auth-note" role="status">
                <Badge tone="success">price agreed</Badge>
                <span>
                  {formatMoney(job.authorization.authorized_amount ?? 0)}
                  {job.authorization.source === "customer_estimate"
                    ? " — customer approved the estimate"
                    : job.authorization.source === "manager_override"
                      ? " — go-ahead recorded by staff"
                      : job.authorization.source === "return_rework"
                        ? " — return / follow-up job"
                      : ""}
                  {` on ${new Date(job.authorization.authorized_at).toLocaleString()}`}
                </span>
                {job.authorization.variance_reason ? (
                  <span className="warn-text">Final charge varied: {job.authorization.variance_reason}</span>
                ) : null}
              </div>
            ) : null}
          <section className="panel" id="repair-payment">
            <h2>Payment</h2>
            {balanceDue > 0.009 ? (
              <div className="job-money-summary">
                <p>
                  <strong>{formatMoney(balanceDue)}</strong> still owed
                  <span className="muted">
                    {" "}
                    · charged {formatMoney(amountDue)} · paid {formatMoney(paidTotal)}
                  </span>
                </p>
                {canAcceptPaid && paidTotal > 0 ? (
                  <Button
                    type="button"
                    variant="secondary"
                    disabled={busy}
                    onClick={() =>
                      void run(async () => {
                        if (
                          !window.confirm(
                            `Customer already paid ${formatMoney(paidTotal)}. Set that as the full charge and clear the ${formatMoney(balanceDue)} remainder?`,
                          )
                        ) {
                          return;
                        }
                        await acceptPaidCharge(job.id);
                      })
                    }
                  >
                    Accept {formatMoney(paidTotal)} as full charge
                  </Button>
                ) : null}
              </div>
            ) : amountDue > 0 ? (
              <p className="hint">
                Paid in full
                {cashHeld > 0
                  ? ` (${formatMoney(cashHeld)} cash on the counter — device can be released)`
                  : ""}
                .
              </p>
            ) : (
              <p className="hint">No charge on this job yet.</p>
            )}
            {paymentLocked ? null : (
            <>
            <label style={{ display: "block", marginBottom: "0.75rem" }}>
              <input type="checkbox" checked={splitPay} onChange={(e) => setSplitPay(e.target.checked)} /> Split tender
            </label>
            {!splitPay ? (
            <form
              className="form-grid"
              onSubmit={(e: FormEvent) => {
                e.preventDefault();
                void run(async () => {
                  const amount = Number(payAmount) || 0;
                  const willClear = amountDue > 0 && amount + 0.009 >= balanceDue;
                  const created = await createPayment({
                    method: payMethod,
                    amount,
                    payable_type: "repair",
                    payable_id: job.id,
                    branch_id: job.branch_id,
                    currency: "KES",
                    phone: payMethod === "mpesa_stk" ? payPhone : undefined,
                    account_reference: job.job_code ?? job.id.slice(0, 8),
                  });
                  setPayAmount("");
                  if (payMethod === "mpesa_stk") {
                    const id = paymentIdFromCreate(created);
                    if (id) {
                      void pollStkPayment(id);
                    } else {
                      setError(
                        "STK push sent, but we couldn't track its status here — check Payments for confirmation.",
                      );
                    }
                  } else {
                    void openRepairReceipt(job.id).catch(() => undefined);
                  }
                  if (willClear && payMethod === "cash") {
                    await advanceToHandoverAfterPayment("cash");
                  }
                });
              }}
            >
              <label>
                Method
                <select className="input" value={payMethod} onChange={(e) => setPayMethod(e.target.value)}>
                  <option value="cash">Cash</option>
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
              <Button type="submit" disabled={busy || stkPolling || !(Number(payAmount) > 0)}>
                {busy || stkPolling
                  ? "Processing…"
                  : payMethod === "mpesa_stk"
                    ? "Send STK push"
                    : payMethod === "mpesa_c2b"
                      ? "Await paybill"
                      : "Record payment"}
              </Button>
            </form>
            ) : (
              <form
                className="form-grid"
                onSubmit={(e: FormEvent) => {
                  e.preventDefault();
                  void run(async () => {
                    const lines = tenders
                      .filter((t) => Number(t.amount) > 0)
                      .map((t) => ({
                        method: t.method,
                        amount: Number(t.amount) || 0,
                        phone: t.method === "mpesa_stk" ? t.phone || undefined : undefined,
                      }));
                    const cashTotal = lines.filter((t) => t.method === "cash").reduce((s, t) => s + t.amount, 0);
                    const sum = lines.reduce((s, t) => s + t.amount, 0);
                    const willClear = amountDue > 0 && sum + 0.009 >= balanceDue;
                    const created = await createPayment({
                      payable_type: "repair",
                      payable_id: job.id,
                      branch_id: job.branch_id,
                      currency: "KES",
                      account_reference: job.job_code ?? job.id.slice(0, 8),
                      tenders: lines,
                    });
                    setTenders([{ method: "cash", amount: "", phone: "" }]);
                    setSplitPay(false);
                    if (willClear && cashTotal > 0) {
                      await advanceToHandoverAfterPayment("cash");
                    }
                    if (lines.some((t) => t.method === "mpesa_stk")) {
                      const id = paymentIdFromCreate(created);
                      if (id) {
                        void pollStkPayment(id);
                      } else {
                        setError(
                          "STK push sent, but we couldn't track its status here — check Payments for confirmation.",
                        );
                      }
                    } else {
                      void openRepairReceipt(job.id).catch(() => undefined);
                    }
                  });
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
                  <Button type="submit" disabled={busy || stkPolling}>
                    {busy || stkPolling ? "Processing…" : "Record split payment"}
                  </Button>
                </div>
              </form>
            )}
            {payMethod === "mpesa_c2b" && payCfg?.configured ? (
              <p className="hint">
                Customer pays Till/Paybill <span className="mono">{payCfg.mpesa_shortcode}</span> · account{" "}
                <span className="mono">{job.job_code ?? job.id.slice(0, 8)}</span>.
              </p>
            ) : null}
            {payCfg?.bank_configured ? (
              <p className="hint">
                Bank paybill <span className="mono">{payCfg.bank_paybill}</span> · account{" "}
                <span className="mono">{payCfg.bank_account}</span>
              </p>
            ) : null}
            </>
            )}

            {livePayments.length === 0 && failedPayments.length === 0 ? (
              <EmptyState title="No payments yet" body="Record cash or send an STK when the customer pays." />
            ) : (
              <ul className="list">
                {livePayments.map((p) => (
                  <li key={p.id}>
                    <Badge tone={statusTone(p.status)}>{p.status}</Badge>
                    <span>
                      {p.method.replaceAll("_", " ")} · {formatMoney(p.amount)}
                      {p.checkout_request_id ? ` · ${p.checkout_request_id.slice(0, 16)}…` : ""}
                    </span>
                    {p.method === "mpesa_stk" && (p.status === "initiated" || p.status === "pending") ? (
                      <Button
                        type="button"
                        variant="secondary"
                        disabled={busy || stkPolling}
                        onClick={() => void pollStkPayment(p.id)}
                      >
                        Check payment
                      </Button>
                    ) : null}
                  </li>
                ))}
              </ul>
            )}
            {failedPayments.length > 0 ? (
              <div style={{ marginTop: "0.75rem" }}>
                <button
                  type="button"
                  className="jobs-more-toggle"
                  onClick={() => setShowFailedPays((v) => !v)}
                >
                  {showFailedPays ? "Hide" : "Show"} {failedPayments.length} failed attempt
                  {failedPayments.length === 1 ? "" : "s"}
                </button>
                {showFailedPays ? (
                  <ul className="list" style={{ marginTop: "0.5rem" }}>
                    {failedPayments.map((p) => (
                      <li key={p.id}>
                        <Badge tone="danger">{p.status}</Badge>
                        <span className="muted">
                          {p.method.replaceAll("_", " ")} · {formatMoney(p.amount)}
                        </span>
                      </li>
                    ))}
                  </ul>
                ) : null}
              </div>
            ) : null}

            {refundable.length > 0 ? (
              <div style={{ marginTop: "1rem" }}>
                <button type="button" className="jobs-more-toggle" onClick={() => setShowRefund((v) => !v)}>
                  {showRefund ? "Hide refund" : "Request a refund"}
                </button>
                {showRefund ? (
                  <form
                    className="form-grid"
                    style={{ marginTop: "0.75rem" }}
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
              </div>
            ) : null}
          </section>

          <section className="panel">
            <button
              type="button"
              className="jobs-more-toggle"
              style={{ width: "100%", textAlign: "left" }}
              onClick={() => setShowAccessories((v) => !v)}
            >
              {showAccessories ? "Hide accessories" : "Add accessories / extras to this bill"}
              {saleLinesTotal > 0 ? ` · ${formatMoney(saleLinesTotal)}` : ""}
            </button>
            {showAccessories || saleLines.length > 0 ? (
              <div style={{ marginTop: "0.75rem" }}>
                {saleLines.length === 0 ? (
                  <p className="hint">
                    {stock.filter((b) => b.available_qty > 0 && b.sell_price > 0).length === 0
                      ? "No sellable stock yet — use Custom for one-off charges, or add items under Inventory."
                      : "Cases, chargers, glass, or a custom charge — added here so one payment covers the job."}
                  </p>
                ) : (
                  <ul className="list">
                    {saleLines.map((line) => (
                      <li key={line.id}>
                        <span>
                          {line.description} · ×{line.quantity} · {formatMoney(line.line_total)}
                          {line.is_custom ? (
                            <span className="muted"> · custom</span>
                          ) : null}
                        </span>
                        {!paymentLocked && job.status !== "collected" ? (
                          <Button
                            type="button"
                            variant="ghost"
                            disabled={busy}
                            onClick={() => void run(() => removeRepairSaleLine(job.id, line.id))}
                          >
                            Remove
                          </Button>
                        ) : null}
                      </li>
                    ))}
                  </ul>
                )}
                {!paymentLocked && job.status !== "collected" && showAccessories ? (
                  <div style={{ marginTop: "0.75rem" }}>
                    <div className="chip-row" style={{ marginBottom: "0.55rem" }}>
                      <button
                        type="button"
                        className={saleAddMode === "stock" ? "chip chip-active" : "chip"}
                        onClick={() => setSaleAddMode("stock")}
                      >
                        From stock
                      </button>
                      <button
                        type="button"
                        className={saleAddMode === "custom" ? "chip chip-active" : "chip"}
                        onClick={() => setSaleAddMode("custom")}
                      >
                        Custom item
                      </button>
                    </div>
                    {saleAddMode === "stock" ? (
                      <form
                        className="inline-form"
                        onSubmit={(e: FormEvent) => {
                          e.preventDefault();
                          if (!saleStockId) return;
                          const [variantId, locationId] = saleStockId.split(":");
                          if (!variantId || !locationId) return;
                          void run(async () => {
                            await addRepairSaleLine(job.id, {
                              variant_id: variantId,
                              location_id: locationId,
                              quantity: Math.max(1, Number(saleQty) || 1),
                            });
                            setSaleQty("1");
                          });
                        }}
                      >
                        <label>
                          Stock item
                          <select className="input" value={saleStockId} onChange={(e) => setSaleStockId(e.target.value)}>
                            <option value="">Select…</option>
                            {stock
                              .filter((b) => b.available_qty > 0 && b.sell_price > 0)
                              .map((b) => (
                                <option
                                  key={`${b.variant_id}:${b.location_id}`}
                                  value={`${b.variant_id}:${b.location_id}`}
                                >
                                  {b.product_name} ({b.sku}) · {formatMoney(b.sell_price)} · {b.available_qty} at{" "}
                                  {b.location_name}
                                </option>
                              ))}
                          </select>
                        </label>
                        <label>
                          Qty
                          <Input type="number" min={1} value={saleQty} onChange={(e) => setSaleQty(e.target.value)} />
                        </label>
                        <Button type="submit" disabled={busy || !saleStockId}>
                          Add to bill
                        </Button>
                      </form>
                    ) : (
                      <form
                        className="inline-form"
                        onSubmit={(e: FormEvent) => {
                          e.preventDefault();
                          const desc = customSaleDesc.trim();
                          const price = Number(customSalePrice);
                          if (!desc || !Number.isFinite(price) || price < 0) {
                            setError("Enter a description and a price for the custom item");
                            return;
                          }
                          void run(async () => {
                            await addRepairSaleLine(job.id, {
                              description: desc,
                              unit_price: price,
                              quantity: Math.max(1, Number(customSaleQty) || 1),
                            });
                            setCustomSaleDesc("");
                            setCustomSalePrice("");
                            setCustomSaleQty("1");
                          });
                        }}
                      >
                        <label>
                          Description
                          <Input
                            value={customSaleDesc}
                            onChange={(e) => setCustomSaleDesc(e.target.value)}
                            placeholder="e.g. Screen protector fit"
                            required
                          />
                        </label>
                        <label>
                          Unit price ({currencyCode})
                          <Input
                            type="number"
                            min="0"
                            step="0.01"
                            value={customSalePrice}
                            onChange={(e) => setCustomSalePrice(e.target.value)}
                            required
                          />
                        </label>
                        <label>
                          Qty
                          <Input
                            type="number"
                            min={1}
                            value={customSaleQty}
                            onChange={(e) => setCustomSaleQty(e.target.value)}
                          />
                        </label>
                        <Button
                          type="submit"
                          disabled={busy || !customSaleDesc.trim() || customSalePrice === ""}
                        >
                          Add to bill
                        </Button>
                        <p className="hint" style={{ margin: 0, flexBasis: "100%" }}>
                          For items not tracked in inventory — won’t affect stock counts.
                        </p>
                      </form>
                    )}
                  </div>
                ) : null}
              </div>
            ) : null}
          </section>

          {margin ? (
            <section className="panel">
              <button
                type="button"
                className="jobs-more-toggle"
                style={{ width: "100%", textAlign: "left" }}
                onClick={() => setShowEconomics((v) => !v)}
              >
                {showEconomics ? "Hide job economics" : "Job economics (margin)"}
              </button>
              {showEconomics ? (
                <div style={{ marginTop: "0.75rem" }}>
              <dl className="meta-dl">
                <dt>Charged for labor</dt>
                <dd>{formatMoney(margin.labor_amount)}</dd>
                <dt>Parts cost</dt>
                <dd>{margin.parts_cost > 0 ? formatMoney(margin.parts_cost) : <span className="muted">None</span>}</dd>
                <dt>Margin</dt>
                <dd>
                  <strong className={margin.margin < 0 ? "warn-text" : undefined}>
                    {margin.unpriced_parts > 0 ? "at most " : ""}
                    {formatMoney(margin.margin)}
                  </strong>
                  {margin.margin_pct != null ? (
                    <div className="muted">{margin.margin_pct.toFixed(1)}% of what we charged</div>
                  ) : null}
                </dd>
              </dl>
              {margin.costs.length > 0 ? (
                <ul className="cost-list">
                  {margin.costs.map((c) => (
                    <li key={c.id}>
                      <span>
                        {c.description}
                        {c.quantity > 1 ? <em className="muted"> × {c.quantity}</em> : null}
                      </span>
                      <strong>{formatMoney(c.amount)}</strong>
                    </li>
                  ))}
                </ul>
              ) : (
                <p className="hint">
                  No part costs booked yet. Costs land here when a supplier part is collected or stock is issued.
                </p>
              )}
              {margin.unpriced_parts > 0 ? (
                <p className="hint warn-text">
                  {margin.unpriced_parts} part{margin.unpriced_parts === 1 ? "" : "s"} on this job have no buying
                  price on record, so the real margin is lower than shown. Set cost prices under Inventory to fix
                  this.
                </p>
              ) : null}
              {margin.margin < 0 ? (
                <p className="form-error">
                  This job is losing money — the parts cost more than what we are charging.
                </p>
              ) : null}
                </div>
              ) : null}
            </section>
          ) : null}

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
              Customer only sees the total amount in the portal / app — not a labor/parts split.
            </p>
            {canRequestParts ? (
              <div className="btn-row" style={{ marginBottom: "1rem" }}>
                <Button type="button" disabled={busy} onClick={() => setShowEstimateModal(true)}>
                  Send customer estimate
                </Button>
              </div>
            ) : null}
            {estimates.length === 0 ? (
              <EmptyState title="No estimates yet" body="Send an estimate for the customer to approve or reject." />
            ) : (
              <ul className="part-list">
                {estimates.map((estimate) => (
                  <li key={estimate.id} className="part-card">
                    <div className="part-head">
                      <div>
                        <strong className="estimate-total">
                          {estimate.currency} {estimateTotalAmount(estimate).toLocaleString()}
                        </strong>
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
                        const rework = await createRepairRework(job.id, warrantyNote);
                        setWarrantyNote("");
                        navigate(`/repairs/${rework.id}`);
                      });
                    }}
                  >
                    <Input
                      value={warrantyNote}
                      onChange={(e) => setWarrantyNote(e.target.value)}
                      placeholder="What failed again?"
                      required
                    />
                    <Button type="submit" disabled={busy || !warrantyNote.trim()}>
                      Claim warranty & open return
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
            </div>
          ) : null}

          {detailLane === "device" ? (
            <div className="lane-stack">
          {(job.status === "collected" || job.status === "completed") && !job.parent_job_id ? (
            <section className="panel">
              <h2>Customer returned this device?</h2>
              <p className="hint">
                Opens a linked follow-up job on the same customer and device (warranty return or come-back fix). The original job stays the payment record.
              </p>
              <form
                className="inline-form"
                onSubmit={(e: FormEvent) => {
                  e.preventDefault();
                  void run(async () => {
                    const returned = await createRepairRework(job.id, returnReason);
                    setReturnReason("");
                    navigate(`/repairs/${returned.id}`);
                  });
                }}
              >
                <Input
                  value={returnReason}
                  onChange={(e) => setReturnReason(e.target.value)}
                  placeholder="What came back / what failed?"
                  required
                />
                <Button type="submit" disabled={busy || !returnReason.trim()}>
                  Open return job
                </Button>
              </form>
            </section>
          ) : null}
          <section className="panel" id="job-customer">
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

          <section className="panel intake-evidence">
            <h2>Intake evidence</h2>
            <dl className="meta-dl">
              <dt>Accessories</dt>
              <dd>
                {job.intake_accessories?.length ? job.intake_accessories.join(", ") : <span className="muted">None recorded</span>}
              </dd>
              <dt>Condition</dt>
              <dd>{job.intake_condition || <span className="muted">No condition note</span>}</dd>
              <dt>Device access</dt>
              <dd>
                {job.has_device_passcode ? (
                  <>
                    {revealedPasscode ? <code className="passcode-reveal">{revealedPasscode}</code> : <span>Passcode captured</span>}
                    <Button
                      type="button"
                      variant="ghost"
                      disabled={busy}
                      onClick={() =>
                        void run(async () => {
                          const result = await revealRepairPasscode(job.id);
                          setRevealedPasscode(result.passcode);
                        })
                      }
                    >
                      {revealedPasscode ? "Reveal again" : "Reveal (audited)"}
                    </Button>
                  </>
                ) : (
                  <span className="muted">No passcode</span>
                )}
              </dd>
            </dl>
          </section>

          <section className="panel schedule-panel">
            <h2>Customer promise</h2>
            <p className="hint">Keep the commitment separate from status so overdue work cannot hide.</p>
            <label>
              Promised by
              <Input
                type="datetime-local"
                value={promisedBy}
                onChange={(e) => setPromisedBy(e.target.value)}
                disabled={busy}
              />
            </label>
            <div className="btn-row">
              <Button
                type="button"
                variant="secondary"
                disabled={busy}
                onClick={() =>
                  void run(() =>
                    updateRepairSchedule(job.id, promisedBy ? new Date(promisedBy).toISOString() : undefined),
                  )
                }
              >
                Save promise
              </Button>
              {promisedBy ? (
                <Button
                  type="button"
                  variant="ghost"
                  disabled={busy}
                  onClick={() =>
                    void run(async () => {
                      await updateRepairSchedule(job.id);
                      setPromisedBy("");
                    })
                  }
                >
                  Clear
                </Button>
              ) : null}
            </div>
          </section>

            </div>
          ) : null}

          {detailLane === "history" ? (
            <div className="lane-stack">
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

          <section className="panel" id="job-history">
            <h2>Status timeline</h2>
            {(job.timeline?.length ?? 0) === 0 ? (
              <EmptyState title="No events" body="Status changes appear here." />
            ) : (
              <ol className="timeline">
                {job.timeline!.map((ev, i) => (
                  <li key={`${ev.at}-${i}`}>
                    <Badge tone={statusTone(ev.status)}>{statusLabel(ev.status)}</Badge>
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
            </div>
          ) : null}
      </div>

      {showEstimateModal ? (
        <div
          className="cmdk-backdrop"
          role="presentation"
          onClick={() => {
            if (!busy) setShowEstimateModal(false);
          }}
        >
          <div
            className="cmdk-panel estimate-modal"
            role="dialog"
            aria-modal="true"
            aria-label="Send customer estimate"
            onClick={(e) => e.stopPropagation()}
          >
            <div className="panel-head" style={{ padding: "1rem 1.25rem 0" }}>
              <h2 style={{ margin: 0 }}>Customer estimate</h2>
            </div>
            <form
              className="form-grid"
              style={{ padding: "1rem 1.25rem 1.25rem" }}
              onSubmit={(event: FormEvent) => {
                event.preventDefault();
                const total = Number(estimateTotal);
                if (!Number.isFinite(total) || total <= 0) {
                  setError("Enter a total greater than zero");
                  return;
                }
                void run(async () => {
                  await createRepairEstimate(job.id, {
                    total_amount: total,
                    notes: estimateNotes.trim() || undefined,
                  });
                  const phone = customer?.phone?.trim();
                  setActionMsg(
                    phone
                      ? `Estimate sent — SMS sent to ${phone}.`
                      : "Estimate saved — no phone on file to text it to.",
                  );
                  setEstimateTotal("");
                  setEstimateNotes("");
                  setShowEstimateModal(false);
                });
              }}
            >
              <p className="hint">
                Customer only sees this total — labor and parts stay internal. Diagnosis text below is included in the SMS.
              </p>
              <label>
                Total amount ({currencyCode})
                <Input
                  type="number"
                  min="0.01"
                  step="0.01"
                  value={estimateTotal}
                  onChange={(e) => setEstimateTotal(e.target.value)}
                  required
                  autoFocus
                />
              </label>
              <label>
                Diagnosis / recommendation
                <Textarea
                  rows={3}
                  value={estimateNotes}
                  onChange={(e) => setEstimateNotes(e.target.value)}
                  placeholder="What you found and why this is the price — this gets included in the SMS sent to the customer."
                />
              </label>
              <p className="hint">Keep it brief — long text may split into multiple SMS segments.</p>
              <div className="btn-row">
                <Button type="submit" disabled={busy || !estimateTotal}>
                  Send estimate
                </Button>
                <Button
                  type="button"
                  variant="secondary"
                  disabled={busy}
                  onClick={() => setShowEstimateModal(false)}
                >
                  Cancel
                </Button>
              </div>
            </form>
          </div>
        </div>
      ) : null}

      <SendSmsModal
        open={showSmsModal}
        onClose={() => setShowSmsModal(false)}
        initialPhone={customer?.phone ?? ""}
        customerId={customer?.id}
        repairJobId={job.id}
        title={customer?.full_name ? `SMS to ${customer.full_name}` : "Send SMS"}
      />
    </div>
  );
}
