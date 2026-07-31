import { useCallback, useEffect, useMemo, useState, type FormEvent } from "react";
import { createPortal } from "react-dom";
import { Link, useSearchParams } from "react-router-dom";
import { useAuth } from "../auth/AuthContext";
import { useBranch } from "../branch/BranchContext";
import { useRealtimeEvents } from "../lib/realtime";
import { Badge, Button, EmptyState, ICONS, Input, PageHeader, PhotoCaptureField, SearchInput } from "../components/ui";
import { SearchableCombobox, type ComboOption } from "../components/SearchableCombobox";
import {
  assignRepair,
  createCustomer,
  intakeRepair,
  listCustomers,
  listRepairs,
  openIntakeSlip,
  uploadRepairAttachment,
  type Customer,
  type IntakeResult,
  type RepairJob,
} from "../lib/api";
import { statusLabel, statusTone } from "../lib/repairStatus";
import { useCurrency } from "../lib/currency";

const STAGES: { value: string; label: string; hint: string }[] = [
  { value: "open", label: "Open", hint: "Still in the shop" },
  { value: "waiting_parts", label: "Waiting parts", hint: "Blocked on parts" },
  { value: "in_progress", label: "On bench", hint: "Being worked" },
  { value: "ready", label: "Ready", hint: "QC / complete" },
  { value: "overdue", label: "Overdue", hint: "Past promise" },
  { value: "collected", label: "Collected", hint: "Handed back" },
  { value: "", label: "All", hint: "Every job" },
]

const WALK_IN_VALUE = "__walk_in__";

const COMMON_ISSUES: ComboOption[] = [
  { value: "Screen cracked", label: "Screen cracked" },
  { value: "Won't charge", label: "Won't charge" },
  { value: "No power", label: "No power" },
  { value: "Water damage", label: "Water damage" },
  { value: "Battery draining fast", label: "Battery draining fast" },
  { value: "Speaker / mic issue", label: "Speaker / mic issue" },
  { value: "Camera not working", label: "Camera not working" },
  { value: "Software / boot loop", label: "Software / boot loop" },
  { value: "Charging port damaged", label: "Charging port damaged" },
];

// TODO: back with real recent-devices query
const RECENT_DEVICES: ComboOption[] = [
  { value: "Samsung|A14", label: "Samsung A14", sublabel: "phone" },
  { value: "Samsung|A15", label: "Samsung A15", sublabel: "phone" },
  { value: "Apple|iPhone 11", label: "Apple iPhone 11", sublabel: "phone" },
  { value: "Apple|iPhone 12", label: "Apple iPhone 12", sublabel: "phone" },
  { value: "Apple|iPhone 13", label: "Apple iPhone 13", sublabel: "phone" },
  { value: "Xiaomi|Redmi Note", label: "Xiaomi Redmi Note", sublabel: "phone" },
  { value: "Tecno|Spark", label: "Tecno Spark", sublabel: "phone" },
  { value: "Infinix|Hot", label: "Infinix Hot", sublabel: "phone" },
  { value: "HP|Laptop", label: "HP Laptop", sublabel: "laptop" },
  { value: "Dell|Laptop", label: "Dell Laptop", sublabel: "laptop" },
  { value: "Lenovo|ThinkPad", label: "Lenovo ThinkPad", sublabel: "laptop" },
];

const CONDITION_TAGS = [
  "Back cover missing",
  "Screen scratches",
  "Powers on",
  "Does not power on",
  "Liquid marks",
  "Bent frame",
  "Missing screws",
];

type PendingPhoto = { file: File; preview: string };

function jobAge(createdAt?: string) {
  if (!createdAt) return "—";
  const days = Math.max(0, Math.floor((Date.now() - new Date(createdAt).getTime()) / 86_400_000));
  return days === 0 ? "Today" : `${days}d`;
}

function isOverdue(job: RepairJob) {
  return Boolean(
    job.promised_by &&
      new Date(job.promised_by).getTime() < Date.now() &&
      !["completed", "collected", "cancelled", "unrepairable", "ready_for_pickup"].includes(job.status),
  );
}

function deviceLabel(job: RepairJob) {
  const brand = job.device?.brand?.trim();
  const model = job.device?.model?.trim();
  if (brand && model) return `${brand} ${model}`;
  if (brand) return brand;
  if (model) return model;
  return "Device";
}

function moneyLabel(job: RepairJob, formatMoney: (n: number) => string) {
  if ((job.balance_due ?? 0) > 0) return `${formatMoney(job.balance_due ?? 0)} due`;
  if ((job.quoted_value ?? 0) > 0) return formatMoney(job.quoted_value ?? 0);
  return "Unpriced";
}

function promiseLabel(job: RepairJob, overdue: boolean) {
  if (job.customer_waiting && job.estimated_wait_minutes) {
    return `~${job.estimated_wait_minutes} min wait`;
  }
  if (job.promised_by) {
    if (overdue) return "Overdue";
    return `Due ${new Date(job.promised_by).toLocaleDateString("en-KE", { day: "numeric", month: "short" })}`;
  }
  return "No promise";
}

async function fileToBase64(file: File): Promise<string> {
  const dataUrl = await new Promise<string>((resolve, reject) => {
    const reader = new FileReader();
    reader.onload = () => resolve(String(reader.result ?? ""));
    reader.onerror = () => reject(new Error("Could not read photo"));
    reader.readAsDataURL(file);
  });
  return dataUrl.includes(",") ? dataUrl.split(",")[1]! : dataUrl;
}

/** Try to read an IMEI / serial from a barcode photo when the browser supports it. */
async function tryReadImeiFromPhoto(file: File): Promise<string | null> {
  const Detector = (window as unknown as { BarcodeDetector?: new (opts?: { formats?: string[] }) => {
    detect: (source: ImageBitmap) => Promise<Array<{ rawValue: string }>>;
  } }).BarcodeDetector;
  if (!Detector) return null;
  try {
    const bitmap = await createImageBitmap(file);
    const detector = new Detector({
      formats: ["code_128", "code_39", "ean_13", "qr_code", "data_matrix"],
    });
    const codes = await detector.detect(bitmap);
    bitmap.close();
    for (const code of codes) {
      const digits = code.rawValue.replace(/\D/g, "");
      if (digits.length >= 14 && digits.length <= 17) return digits;
      if (/^[A-Za-z0-9-]{8,}$/.test(code.rawValue.trim())) return code.rawValue.trim();
    }
  } catch {
    /* unsupported / decode failed — photo still stored as attachment */
  }
  return null;
}

export function RepairsPage() {
  const [searchParams] = useSearchParams();
  const { user } = useAuth();
  const { branchId } = useBranch();
  const { formatMoney } = useCurrency();
  const [items, setItems] = useState<RepairJob[]>([]);
  const [allOpen, setAllOpen] = useState<RepairJob[]>([]);
  const [collectedCount, setCollectedCount] = useState(0);
  const [error, setError] = useState("");
  const [showForm, setShowForm] = useState(false);
  const [q, setQ] = useState("");
  const [status, setStatus] = useState(() => {
    const raw = searchParams.get("status") ?? "open";
    // Home / old links used completed; shop-floor stage is "ready".
    if (raw === "completed" || raw === "ready_for_pickup") return "ready";
    return raw;
  });
  const [myJobs, setMyJobs] = useState(false);

  const refresh = useCallback(async () => {
    try {
      const [filtered, openBoard, collectedBoard] = await Promise.all([
        listRepairs({
          q: q.trim() || undefined,
          status: status || undefined,
          branch_id: branchId || undefined,
          technician_id: myJobs ? "me" : undefined,
        }),
        listRepairs({
          status: "open",
          branch_id: branchId || undefined,
          technician_id: myJobs ? "me" : undefined,
        }),
        listRepairs({
          status: "collected",
          branch_id: branchId || undefined,
          technician_id: myJobs ? "me" : undefined,
        }),
      ]);
      setItems(filtered.items ?? []);
      setAllOpen(openBoard.items ?? []);
      setCollectedCount((collectedBoard.items ?? []).length);
      setError("");
    } catch (e) {
      setError(e instanceof Error ? e.message : "Failed to load");
    }
  }, [q, status, branchId, myJobs]);

  useEffect(() => {
    void refresh();
  }, [refresh]);

  useRealtimeEvents(["repair.*"], () => {
    void refresh();
  });

  useEffect(() => {
    if (!showForm) return;
    const onKey = (e: KeyboardEvent) => {
      if (e.key === "Escape") setShowForm(false);
    };
    const prev = document.body.style.overflow;
    document.body.style.overflow = "hidden";
    window.addEventListener("keydown", onKey);
    return () => {
      document.body.style.overflow = prev;
      window.removeEventListener("keydown", onKey);
    };
  }, [showForm]);

  const pulse: Record<string, number> = {
    open: allOpen.length,
    waiting_parts: allOpen.filter((j) => j.status === "waiting_parts").length,
    in_progress: allOpen.filter((j) => j.status === "in_progress").length,
    ready: allOpen.filter((j) => j.status === "ready_for_pickup" || j.status === "completed").length,
    overdue: allOpen.filter((j) => isOverdue(j)).length,
    collected: collectedCount,
    "": 0,
  };

  const stageLabel = STAGES.find((s) => s.value === status)?.label ?? "Jobs";

  return (
    <div className="jobs-desk">
      <PageHeader
        title="Jobs"
        subtitle="Find a job on the board, or take a new device in."
        actions={
          <Button type="button" onClick={() => setShowForm(true)}>
            New job
          </Button>
        }
      />
      {error ? <p className="form-error">{error}</p> : null}

      <section className="jobs-stages" aria-label="Job stages">
        {STAGES.map((stage) => {
          const count = stage.value === "" ? null : pulse[stage.value] ?? 0;
          const active = status === stage.value;
          const warn = stage.value === "overdue" && (pulse.overdue ?? 0) > 0;
          return (
            <button
              key={stage.value || "all"}
              type="button"
              className={`jobs-stage ${active ? "active" : ""} ${warn ? "warn" : ""}`}
              onClick={() => setStatus(stage.value)}
              aria-pressed={active}
            >
              <span className="jobs-stage-label">{stage.label}</span>
              {count !== null ? <strong>{count}</strong> : <strong className="jobs-stage-all">···</strong>}
              <em>{stage.hint}</em>
            </button>
          );
        })}
      </section>

      <section className="jobs-ledger desk-ledger">
        <div className="panel-head jobs-ledger-head">
          <div>
            <h2>{stageLabel}</h2>
            <p className="muted">
              {items.length === 0
                ? "No jobs in this view"
                : `${items.length} job${items.length === 1 ? "" : "s"} · overdue & wait-bench first, then oldest promise`}
            </p>
          </div>
          <div className="jobs-toolbar">
            <SearchInput
              value={q}
              onChange={(e) => setQ(e.target.value)}
              placeholder="Search job, client, device, problem…"
              aria-label="Search jobs"
            />
            <label className="checkbox-row jobs-mine">
              <input type="checkbox" checked={myJobs} onChange={(e) => setMyJobs(e.target.checked)} />
              My jobs
            </label>
          </div>
        </div>

        {items.length === 0 ? (
          <div className="jobs-empty">
            <EmptyState
              title="No jobs here"
              body={
                status === "open"
                  ? "Nothing open on this branch. Take a device in to get started."
                  : "Try another stage, clear search, or take a new job in."
              }
              icon={ICONS.repairs}
            />
            <div className="btn-row jobs-empty-actions">
              {status !== "open" || q ? (
                <Button
                  type="button"
                  variant="secondary"
                  onClick={() => {
                    setStatus("open");
                    setQ("");
                  }}
                >
                  Show open jobs
                </Button>
              ) : null}
              <Button type="button" onClick={() => setShowForm(true)}>
                New job
              </Button>
            </div>
          </div>
        ) : (
          <ul className="jobs-list" aria-label="Repair jobs">
            {items.map((j) => {
              const overdue = isOverdue(j);
              return (
                <li key={j.id}>
                  <Link className={`jobs-row ${overdue ? "is-overdue" : ""}`} to={`/repairs/${j.id}`}>
                    <div className="jobs-row-id">
                      <strong className="mono">{j.job_code ?? j.id.slice(0, 8)}</strong>
                      <Badge tone={statusTone(j.status)}>{statusLabel(j.status)}</Badge>
                    </div>
                    <div className="jobs-row-body">
                      <div className="jobs-row-customer">
                        <span>{j.customer_name ?? j.customer?.full_name ?? "Walk-in"}</span>
                        {j.customer_waiting ? <Badge tone="pending">wait bench</Badge> : null}
                        {j.parent_job_id ? <Badge tone="warning">return</Badge> : null}
                      </div>
                      <div className="jobs-row-device">{deviceLabel(j)}</div>
                      <p>{j.problem_summary}</p>
                    </div>
                    <div className="jobs-row-meta">
                      <span className={(j.quoted_value ?? 0) > 0 || (j.balance_due ?? 0) > 0 ? "jobs-row-money" : "muted"}>
                        {moneyLabel(j, formatMoney)}
                      </span>
                      <span>{jobAge(j.created_at)}</span>
                      <span className={overdue ? "repair-overdue" : "muted"}>{promiseLabel(j, overdue)}</span>
                    </div>
                  </Link>
                </li>
              );
            })}
          </ul>
        )}
      </section>

      {showForm
        ? createPortal(
            <div className="jobs-intake-portal" role="presentation">
              <button
                type="button"
                className="jobs-intake-backdrop"
                aria-label="Close intake"
                onClick={() => setShowForm(false)}
              />
              <aside className="jobs-intake" aria-label="New job" role="dialog" aria-modal="true">
                <QuickIntake
                  branchId={branchId}
                  userId={user?.id}
                  onClose={() => setShowForm(false)}
                  onCreated={() => {
                    void refresh();
                  }}
                />
              </aside>
            </div>,
            document.body,
          )
        : null}
    </div>
  );
}

function QuickIntake({
  branchId,
  userId,
  onClose,
  onCreated,
}: {
  branchId: string;
  userId?: string;
  onClose: () => void;
  onCreated: () => void;
}) {
  const { formatMoney } = useCurrency();
  const [customerValue, setCustomerValue] = useState("");
  const [customerOptions, setCustomerOptions] = useState<ComboOption[]>([
    { value: WALK_IN_VALUE, label: "Walk-in (no record)", sublabel: "Anonymous check-in" },
  ]);
  const [customerSearching, setCustomerSearching] = useState(false);
  const [customerName, setCustomerName] = useState("");
  const [customerPhone, setCustomerPhone] = useState("");
  const [anonymous, setAnonymous] = useState(false);

  const [deviceKind, setDeviceKind] = useState<"phone" | "laptop" | "tablet" | "other">("phone");
  const [deviceValue, setDeviceValue] = useState("");
  const [brand, setBrand] = useState("");
  const [model, setModel] = useState("");
  const [imei, setImei] = useState("");

  const [issueValue, setIssueValue] = useState("");
  const [issueOptions, setIssueOptions] = useState<ComboOption[]>(COMMON_ISSUES);
  const [problem, setProblem] = useState("");

  const [toDiagnose, setToDiagnose] = useState(true);
  const [amount, setAmount] = useState("");
  const [conditionTags, setConditionTags] = useState<string[]>([]);
  const [assignMe, setAssignMe] = useState(true);
  const [imeiPhoto, setImeiPhoto] = useState<PendingPhoto | null>(null);
  const [devicePhoto, setDevicePhoto] = useState<PendingPhoto | null>(null);
  const [busy, setBusy] = useState(false);
  const [err, setErr] = useState("");
  const [created, setCreated] = useState<IntakeResult | null>(null);
  const [actionMsg, setActionMsg] = useState("");

  const deviceOptions = useMemo(() => {
    return RECENT_DEVICES.filter((d) => !d.sublabel || d.sublabel === deviceKind || deviceKind === "other");
  }, [deviceKind]);

  const canSubmit =
    Boolean(branchId) &&
    Boolean(problem.trim()) &&
    Boolean(deviceKind) &&
    (anonymous || Boolean(customerValue)) &&
    (!toDiagnose ? Number(amount) > 0 : true);

  const searchCustomers = useCallback((query: string) => {
    const q = query.trim();
    if (q.length < 2) {
      setCustomerOptions([
        { value: WALK_IN_VALUE, label: "Walk-in (no record)", sublabel: "Anonymous check-in" },
      ]);
      setCustomerSearching(false);
      return;
    }
    setCustomerSearching(true);
    listCustomers(q)
      .then((r) => {
        const items = (r.items ?? []).map((c: Customer) => ({
          value: c.id,
          label: c.full_name,
          sublabel: c.phone || "No phone",
        }));
        setCustomerOptions([
          { value: WALK_IN_VALUE, label: "Walk-in (no record)", sublabel: "Anonymous check-in" },
          ...items,
        ]);
      })
      .catch(() =>
        setCustomerOptions([
          { value: WALK_IN_VALUE, label: "Walk-in (no record)", sublabel: "Anonymous check-in" },
        ]),
      )
      .finally(() => setCustomerSearching(false));
  }, []);

  function onCustomerSelect(opt: ComboOption) {
    setCustomerValue(opt.value);
    if (!opt.value) {
      setAnonymous(false);
      return;
    }
    if (opt.value === WALK_IN_VALUE) {
      setAnonymous(true);
      setCustomerName("");
      setCustomerPhone("");
      return;
    }
    setAnonymous(false);
    setCustomerName(opt.label);
    setCustomerPhone(opt.sublabel && opt.sublabel !== "No phone" ? opt.sublabel : "");
  }

  function onDeviceSelect(opt: ComboOption) {
    setDeviceValue(opt.value);
    if (!opt.value) {
      setBrand("");
      setModel("");
      return;
    }
    const [b, ...rest] = opt.value.split("|");
    setBrand(b || opt.label);
    setModel(rest.join("|") || "");
  }

  function onIssueSelect(opt: ComboOption) {
    setIssueValue(opt.value);
    setProblem(opt.label || opt.value);
  }

  function toggleTag(tag: string) {
    setConditionTags((cur) => (cur.includes(tag) ? cur.filter((t) => t !== tag) : [...cur, tag]));
  }

  async function onImeiPhoto(file: File) {
    const preview = URL.createObjectURL(file);
    setImeiPhoto((prev) => {
      if (prev) URL.revokeObjectURL(prev.preview);
      return { file, preview };
    });
    const scanned = await tryReadImeiFromPhoto(file);
    if (scanned) setImei(scanned);
  }

  async function onDevicePhoto(file: File) {
    const preview = URL.createObjectURL(file);
    setDevicePhoto((prev) => {
      if (prev) URL.revokeObjectURL(prev.preview);
      return { file, preview };
    });
  }

  async function submit(e: FormEvent) {
    e.preventDefault();
    if (!canSubmit) {
      setErr("Customer, device kind, and issue are required");
      return;
    }
    if (!toDiagnose && !(Number(amount) > 0)) {
      setErr("Enter an amount, or mark To be diagnosed");
      return;
    }
    setBusy(true);
    setErr("");
    setActionMsg("");
    try {
      const labor = Number(amount);
      const result = await intakeRepair({
        branch_id: branchId,
        anonymous: anonymous || undefined,
        customer_id: !anonymous && customerValue && customerValue !== WALK_IN_VALUE ? customerValue : undefined,
        customer_name: !anonymous ? customerName.trim() || undefined : undefined,
        customer_phone: !anonymous ? customerPhone.trim() || undefined : undefined,
        device_kind: deviceKind,
        brand: brand.trim() || "Unknown",
        model: model.trim() || "Unknown",
        imei: deviceKind === "phone" ? imei.trim() || undefined : undefined,
        serial_number: deviceKind !== "phone" ? imei.trim() || undefined : undefined,
        problem_summary: problem.trim(),
        condition_tags: conditionTags,
        estimate_labor_amount: !toDiagnose && Number.isFinite(labor) && labor > 0 ? labor : undefined,
        technician_id: assignMe && userId ? userId : undefined,
      });

      const uploads: Promise<unknown>[] = [];
      if (imeiPhoto) {
        uploads.push(
          fileToBase64(imeiPhoto.file).then((data_base64) =>
            uploadRepairAttachment(result.repair.id, {
              file_name: imeiPhoto.file.name || "imei-photo.jpg",
              content_type: imeiPhoto.file.type || "image/jpeg",
              data_base64,
            }),
          ),
        );
      }
      if (devicePhoto) {
        uploads.push(
          fileToBase64(devicePhoto.file).then((data_base64) =>
            uploadRepairAttachment(result.repair.id, {
              file_name: devicePhoto.file.name || "device-condition.jpg",
              content_type: devicePhoto.file.type || "image/jpeg",
              data_base64,
            }),
          ),
        );
      }
      if (uploads.length) await Promise.all(uploads);
      setCreated(result);
      onCreated();
    } catch (error) {
      setErr(error instanceof Error ? error.message : "Failed");
    } finally {
      setBusy(false);
    }
  }

  if (created) {
    const job = created.repair;
    return (
      <div className="jobs-intake-form jobs-intake-success">
        <header className="jobs-intake-head">
          <div>
            <p className="jobs-intake-kicker">Job created</p>
            <h2>{job.job_code}</h2>
          </div>
          <button type="button" className="jobs-intake-close" onClick={onClose} aria-label="Close">
            ×
          </button>
        </header>
        <div className="jobs-intake-body">
          <div className="action-block" style={{ margin: 0 }}>
            <p className="muted">Last intake</p>
            <dl className="meta-dl">
              <dt>Status</dt>
              <dd>
                <Badge tone="success">{statusLabel(job.status)}</Badge>
              </dd>
              <dt>Device</dt>
              <dd>{deviceLabel(job)}</dd>
              <dt>Issue</dt>
              <dd>{job.problem_summary}</dd>
              <dt>Price</dt>
              <dd>
                {(job.labor_amount ?? 0) > 0 ? formatMoney(job.labor_amount ?? 0) : "To be diagnosed"}
              </dd>
              {job.pickup_code ? (
                <>
                  <dt>Pickup code</dt>
                  <dd className="mono">{job.pickup_code}</dd>
                </>
              ) : null}
            </dl>
            <div className="chip-row" style={{ marginTop: "0.6rem" }}>
              <Button
                type="button"
                variant="secondary"
                onClick={() =>
                  void openIntakeSlip(job.id).catch((e) =>
                    setActionMsg(e instanceof Error ? e.message : "Print failed"),
                  )
                }
              >
                Print slip
              </Button>
              {userId ? (
                <Button
                  type="button"
                  variant="ghost"
                  onClick={() =>
                    void assignRepair(job.id, userId)
                      .then(() => setActionMsg("Assigned to you"))
                      .catch((e) => setActionMsg(e instanceof Error ? e.message : "Assign failed"))
                  }
                >
                  Assign to me
                </Button>
              ) : null}
              <Link className="btn btn-ghost" to={`/repairs/${job.id}`} onClick={onClose}>
                Open job
              </Link>
            </div>
            {actionMsg ? <p className="hint">{actionMsg}</p> : null}
          </div>
        </div>
        <footer className="jobs-intake-foot">
          <Button type="button" onClick={onClose}>
            Done
          </Button>
        </footer>
      </div>
    );
  }

  return (
    <form className="jobs-intake-form jobs-intake-compact" onSubmit={(e) => void submit(e)}>
      <header className="jobs-intake-head">
        <div>
          <p className="jobs-intake-kicker">New job</p>
          <h2>Quick intake</h2>
        </div>
        <button type="button" className="jobs-intake-close" onClick={onClose} aria-label="Close">
          ×
        </button>
      </header>

      <div className="jobs-intake-body">
        <p className="hint jobs-intake-lead">One form — search or add as you go, then create the job.</p>

        <SearchableCombobox
          label="Customer"
          placeholder="Search name or phone…"
          options={customerOptions}
          onSearch={searchCustomers}
          value={customerValue}
          loading={customerSearching}
          onSelect={onCustomerSelect}
          addNewFields={{ primary: "Full name", secondary: "Phone number" }}
          onAddNew={async ({ primary, secondary }) => {
            const c = await createCustomer({
              full_name: primary,
              phone: secondary || undefined,
            });
            const opt = { value: c.id, label: c.full_name, sublabel: c.phone || "No phone" };
            setCustomerOptions((prev) => [prev[0]!, opt, ...prev.slice(1).filter((p) => p.value !== c.id)]);
            setCustomerName(c.full_name);
            setCustomerPhone(c.phone ?? "");
            setAnonymous(false);
            return opt;
          }}
        />

        <fieldset className="intake-kind">
          <legend>Device kind</legend>
          <div className="intake-kind-grid">
            {(["phone", "laptop", "tablet", "other"] as const).map((k) => (
              <button
                key={k}
                type="button"
                className={deviceKind === k ? "active" : ""}
                onClick={() => {
                  setDeviceKind(k);
                  setDeviceValue("");
                }}
              >
                {k}
              </button>
            ))}
          </div>
        </fieldset>

        <SearchableCombobox
          label="Device"
          placeholder="Brand and model…"
          options={deviceOptions}
          value={deviceValue}
          onSelect={onDeviceSelect}
          addNewFields={{ primary: "Brand", secondary: "Model" }}
          onAddNew={async ({ primary, secondary }) => {
            const brandName = primary.trim();
            const modelName = (secondary ?? "").trim() || "Unknown";
            const opt = {
              value: `${brandName}|${modelName}`,
              label: `${brandName} ${modelName}`.trim(),
              sublabel: deviceKind,
            };
            setBrand(brandName);
            setModel(modelName);
            return opt;
          }}
        />

        <label>
          {deviceKind === "phone" ? "IMEI" : "Serial number"}
          <Input
            value={imei}
            onChange={(e) => setImei(e.target.value)}
            className="mono"
            placeholder={deviceKind === "phone" ? "Type or photograph the sticker" : "Serial / service tag"}
          />
        </label>

        <SearchableCombobox
          label="Issue"
          placeholder="What’s wrong?"
          options={issueOptions}
          value={issueValue}
          onSelect={onIssueSelect}
          addNewFields={{ primary: "Describe the issue" }}
          onAddNew={async ({ primary }) => {
            const opt = { value: primary, label: primary };
            setIssueOptions((prev) => [opt, ...prev.filter((p) => p.value !== primary)]);
            setProblem(primary);
            return opt;
          }}
        />

        <PhotoCaptureField
          label={deviceKind === "phone" ? "IMEI / serial photo" : "Serial photo"}
          hint="Photo of the sticker or barcode — scans when the browser can."
          previewUrl={imeiPhoto?.preview}
          onFile={(file) => void onImeiPhoto(file)}
          onClear={() =>
            setImeiPhoto((prev) => {
              if (prev) URL.revokeObjectURL(prev.preview);
              return null;
            })
          }
        />
        <PhotoCaptureField
          label="Device condition photo"
          hint="Cracks, missing parts, water marks."
          previewUrl={devicePhoto?.preview}
          onFile={(file) => void onDevicePhoto(file)}
          onClear={() =>
            setDevicePhoto((prev) => {
              if (prev) URL.revokeObjectURL(prev.preview);
              return null;
            })
          }
        />

        <fieldset className="intake-checklist">
          <legend>Condition tags</legend>
          <div className="intake-tag-row">
            {CONDITION_TAGS.map((tag) => {
              const on = conditionTags.includes(tag);
              return (
                <button
                  key={tag}
                  type="button"
                  className={`intake-tag${on ? " is-on" : ""}`}
                  aria-pressed={on}
                  onClick={() => toggleTag(tag)}
                >
                  {tag}
                </button>
              );
            })}
          </div>
        </fieldset>

        <div className="intake-amount-block">
          <label className="checkbox-row">
            <input
              type="checkbox"
              checked={toDiagnose}
              onChange={(e) => {
                setToDiagnose(e.target.checked);
                if (e.target.checked) setAmount("");
              }}
            />
            To be diagnosed
          </label>
          <label>
            Amount (KES)
            <Input
              type="number"
              min={1}
              step="1"
              value={amount}
              disabled={toDiagnose}
              onChange={(e) => setAmount(e.target.value)}
              placeholder={toDiagnose ? "Diagnose first" : "Agreed price"}
            />
          </label>
        </div>

        <label className="checkbox-row">
          <input type="checkbox" checked={assignMe} onChange={(e) => setAssignMe(e.target.checked)} />
          Assign to me
        </label>

        {err ? <p className="form-error">{err}</p> : null}
      </div>

      <footer className="jobs-intake-foot">
        <Button type="button" variant="secondary" onClick={onClose} disabled={busy}>
          Cancel
        </Button>
        <Button type="submit" disabled={busy || !canSubmit}>
          {busy ? "Creating…" : "Create job"}
        </Button>
      </footer>
    </form>
  );
}

