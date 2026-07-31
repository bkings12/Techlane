import { useCallback, useEffect, useMemo, useState, type FormEvent } from "react";
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
  listIntakePresets,
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
];

const WALK_IN_VALUE = "__walk_in__";

/** Fallback only if the presets API is unreachable — primary source is listIntakePresets. */
const FALLBACK_ISSUES: ComboOption[] = [
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

const FALLBACK_CONDITION_TAGS = [
  "Back cover missing",
  "Screen scratches",
  "Powers on",
  "Does not power on",
  "Liquid marks",
  "Bent frame",
  "Missing screws",
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

const DEVICE_KINDS = ["phone", "laptop", "tablet", "other"] as const;

type DeviceKind = (typeof DEVICE_KINDS)[number];
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
      <PageHeader title="Jobs" subtitle="Take a device in above, then find work on the board below." />
      {error ? <p className="form-error">{error}</p> : null}

      <QuickIntake
        branchId={branchId}
        userId={user?.id}
        onCreated={() => {
          void refresh();
        }}
      />

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
                  ? "Nothing open on this branch. Use intake above to take a device in."
                  : "Try another stage or clear search."
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
    </div>
  );
}

function QuickIntake({
  branchId,
  userId,
  onCreated,
}: {
  branchId: string;
  userId?: string;
  onCreated: () => void;
}) {
  const { formatMoney } = useCurrency();
  const [pickQuery, setPickQuery] = useState("");
  const [recentCustomers, setRecentCustomers] = useState<Customer[]>([]);

  const [customerValue, setCustomerValue] = useState("");
  const [customerOptions, setCustomerOptions] = useState<ComboOption[]>([
    { value: WALK_IN_VALUE, label: "Walk-in (no record)", sublabel: "Anonymous check-in" },
  ]);
  const [customerSearching, setCustomerSearching] = useState(false);
  const [customerName, setCustomerName] = useState("");
  const [customerPhone, setCustomerPhone] = useState("");
  const [anonymous, setAnonymous] = useState(false);

  const [deviceKind, setDeviceKind] = useState<DeviceKind>("phone");
  const [deviceValue, setDeviceValue] = useState("");
  const [brand, setBrand] = useState("");
  const [model, setModel] = useState("");
  const [imei, setImei] = useState("");

  const [issueValue, setIssueValue] = useState("");
  const [issueOptions, setIssueOptions] = useState<ComboOption[]>(FALLBACK_ISSUES);
  const [conditionCatalog, setConditionCatalog] = useState<string[]>(FALLBACK_CONDITION_TAGS);
  const [problem, setProblem] = useState("");

  const [toDiagnose, setToDiagnose] = useState(true);
  const [amount, setAmount] = useState("");
  const [conditionTags, setConditionTags] = useState<string[]>([]);
  const [assignMe, setAssignMe] = useState(true);
  const [imeiPhoto, setImeiPhoto] = useState<PendingPhoto | null>(null);
  const [devicePhoto, setDevicePhoto] = useState<PendingPhoto | null>(null);
  const [busy, setBusy] = useState(false);
  const [err, setErr] = useState("");
  const [last, setLast] = useState<IntakeResult | null>(null);
  const [actionMsg, setActionMsg] = useState("");

  useEffect(() => {
    listCustomers()
      .then((r) => setRecentCustomers((r.items ?? []).slice(0, 12)))
      .catch(() => setRecentCustomers([]));
  }, []);

  useEffect(() => {
    let cancelled = false;
    Promise.all([
      listIntakePresets("issue"),
      listIntakePresets("condition_tag"),
    ])
      .then(([issues, tags]) => {
        if (cancelled) return;
        const issueItems = (issues.items ?? []).map((p) => ({ value: p.label, label: p.label }));
        const tagItems = (tags.items ?? []).map((p) => p.label);
        if (issueItems.length) setIssueOptions(issueItems);
        if (tagItems.length) setConditionCatalog(tagItems);
      })
      .catch(() => {
        /* keep FALLBACK_* so intake still works if presets are briefly down */
      });
    return () => {
      cancelled = true;
    };
  }, []);

  const deviceOptions = useMemo(() => {
    return RECENT_DEVICES.filter((d) => !d.sublabel || d.sublabel === deviceKind || deviceKind === "other");
  }, [deviceKind]);

  const needle = pickQuery.trim().toLowerCase();

  const visibleCustomers = useMemo(() => {
    const walkIn = { id: WALK_IN_VALUE, full_name: "Walk-in", phone: "No record" };
    const list = [walkIn, ...recentCustomers.map((c) => ({ id: c.id, full_name: c.full_name, phone: c.phone || "No phone" }))];
    if (!needle) return list.slice(0, 8);
    return list
      .filter((c) => `${c.full_name} ${c.phone}`.toLowerCase().includes(needle))
      .slice(0, 12);
  }, [recentCustomers, needle]);

  const visibleDevices = useMemo(() => {
    const list = deviceOptions;
    if (!needle) return list.slice(0, 8);
    return list.filter((d) => d.label.toLowerCase().includes(needle)).slice(0, 12);
  }, [deviceOptions, needle]);

  const visibleIssues = useMemo(() => {
    if (!needle) return issueOptions;
    return issueOptions.filter((i) => i.label.toLowerCase().includes(needle));
  }, [needle, issueOptions]);

  const canSubmit =
    Boolean(branchId) &&
    Boolean(problem.trim()) &&
    Boolean(deviceKind) &&
    (anonymous || Boolean(customerValue)) &&
    (!toDiagnose ? Number(amount) > 0 : true);

  const estimateLabel = toDiagnose
    ? "To diagnose"
    : Number(amount) > 0
      ? formatMoney(Number(amount))
      : "—";

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
    setLast(null);
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

  function pickCustomer(id: string, name: string, phone: string) {
    onCustomerSelect({
      value: id,
      label: name,
      sublabel: phone,
    });
    if (id !== WALK_IN_VALUE) {
      setCustomerOptions((prev) => {
        const walk = prev[0] ?? { value: WALK_IN_VALUE, label: "Walk-in (no record)", sublabel: "Anonymous check-in" };
        const opt = { value: id, label: name, sublabel: phone };
        return [walk, opt, ...prev.slice(1).filter((p) => p.value !== id)];
      });
    }
  }

  function onDeviceSelect(opt: ComboOption) {
    setDeviceValue(opt.value);
    setLast(null);
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
    setLast(null);
  }

  function toggleTag(tag: string) {
    setConditionTags((cur) => (cur.includes(tag) ? cur.filter((t) => t !== tag) : [...cur, tag]));
    setLast(null);
  }

  function clearTicket() {
    setCustomerValue("");
    setCustomerName("");
    setCustomerPhone("");
    setAnonymous(false);
    setDeviceKind("phone");
    setDeviceValue("");
    setBrand("");
    setModel("");
    setImei("");
    setIssueValue("");
    setProblem("");
    setToDiagnose(true);
    setAmount("");
    setConditionTags([]);
    setAssignMe(true);
    setImeiPhoto((prev) => {
      if (prev) URL.revokeObjectURL(prev.preview);
      return null;
    });
    setDevicePhoto((prev) => {
      if (prev) URL.revokeObjectURL(prev.preview);
      return null;
    });
    setErr("");
    setActionMsg("");
    setLast(null);
  }

  async function onImeiPhoto(file: File) {
    const preview = URL.createObjectURL(file);
    setImeiPhoto((prev) => {
      if (prev) URL.revokeObjectURL(prev.preview);
      return { file, preview };
    });
    setLast(null);
    const scanned = await tryReadImeiFromPhoto(file);
    if (scanned) setImei(scanned);
  }

  async function onDevicePhoto(file: File) {
    const preview = URL.createObjectURL(file);
    setDevicePhoto((prev) => {
      if (prev) URL.revokeObjectURL(prev.preview);
      return { file, preview };
    });
    setLast(null);
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

      // Keep the page open for the next walk-in — mirror POS clearing the cart after checkout.
      setLast(result);
      setCustomerValue("");
      setCustomerName("");
      setCustomerPhone("");
      setAnonymous(false);
      setDeviceValue("");
      setBrand("");
      setModel("");
      setImei("");
      setIssueValue("");
      setProblem("");
      setToDiagnose(true);
      setAmount("");
      setConditionTags([]);
      setImeiPhoto((prev) => {
        if (prev) URL.revokeObjectURL(prev.preview);
        return null;
      });
      setDevicePhoto((prev) => {
        if (prev) URL.revokeObjectURL(prev.preview);
        return null;
      });
      onCreated();
    } catch (error) {
      setErr(error instanceof Error ? error.message : "Failed");
    } finally {
      setBusy(false);
    }
  }

  const ticketFilled =
    Boolean(customerValue) || Boolean(brand) || Boolean(model) || Boolean(problem) || conditionTags.length > 0;

  return (
    <div className="repair-grid" style={{ marginBottom: "1.25rem" }}>
      <section className="panel" style={{ padding: "0.85rem" }}>
        <div className="panel-head">
          <div>
            <h2>Quick pick</h2>
            <span className="muted">Tap a tile to fill the ticket — same as adding to a POS cart</span>
          </div>
          <SearchInput
            value={pickQuery}
            onChange={(e) => setPickQuery(e.target.value)}
            placeholder="Filter customers, devices, issues…"
            aria-label="Filter quick picks"
          />
        </div>

        <div className="pos-toolbar" style={{ marginTop: "0.75rem" }}>
          <span className="muted">Device kind</span>
          <fieldset className="intake-kind" style={{ margin: 0, flex: 1 }} aria-label="Device kind">
            <div className="intake-kind-grid">
              {DEVICE_KINDS.map((k) => (
                <button
                  key={k}
                  type="button"
                  className={deviceKind === k ? "active" : ""}
                  onClick={() => {
                    setDeviceKind(k);
                    setDeviceValue("");
                    setLast(null);
                  }}
                >
                  {k}
                </button>
              ))}
            </div>
          </fieldset>
        </div>

        <h3 className="intake-pick-heading">Customers</h3>
        <ul className="pos-catalog-grid">
          {visibleCustomers.map((c) => (
            <li key={c.id}>
              <button
                type="button"
                className={`pos-item${customerValue === c.id ? " is-selected" : ""}`}
                onClick={() => pickCustomer(c.id, c.full_name, c.phone)}
              >
                <strong>{c.full_name}</strong>
                <span className="muted">{c.phone}</span>
                <span className="pos-add-label">{customerValue === c.id ? "On ticket" : "Use customer"}</span>
              </button>
            </li>
          ))}
        </ul>

        <h3 className="intake-pick-heading">Devices</h3>
        {visibleDevices.length === 0 ? (
          <p className="muted">No device presets for this kind — type brand/model on the ticket.</p>
        ) : (
          <ul className="pos-catalog-grid">
            {visibleDevices.map((d) => (
              <li key={d.value}>
                <button
                  type="button"
                  className={`pos-item${deviceValue === d.value ? " is-selected" : ""}`}
                  onClick={() => onDeviceSelect(d)}
                >
                  <strong>{d.label}</strong>
                  <span className="muted">{d.sublabel ?? deviceKind}</span>
                  <span className="pos-add-label">{deviceValue === d.value ? "On ticket" : "Use device"}</span>
                </button>
              </li>
            ))}
          </ul>
        )}

        <h3 className="intake-pick-heading">Common issues</h3>
        <ul className="pos-catalog-grid">
          {visibleIssues.map((issue) => (
            <li key={issue.value}>
              <button
                type="button"
                className={`pos-item${issueValue === issue.value ? " is-selected" : ""}`}
                onClick={() => onIssueSelect(issue)}
              >
                <strong>{issue.label}</strong>
                <span className="pos-add-label">{issueValue === issue.value ? "On ticket" : "Use issue"}</span>
              </button>
            </li>
          ))}
        </ul>
      </section>

      <aside className="stack">
        <section className="panel" style={{ padding: "0.85rem" }}>
          <div className="panel-head">
            <div>
              <h2>
                Ticket{" "}
                <small className="muted">{ticketFilled ? "in progress" : "empty"}</small>
              </h2>
            </div>
            {ticketFilled ? (
              <button type="button" className="cart-clear" onClick={clearTicket}>
                Clear
              </button>
            ) : null}
          </div>

          <form className="stack-form" onSubmit={(e) => void submit(e)}>
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
                setRecentCustomers((prev) => [c, ...prev.filter((x) => x.id !== c.id)].slice(0, 12));
                setLast(null);
                return opt;
              }}
            />

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
                setLast(null);
                return opt;
              }}
            />

            <label>
              {deviceKind === "phone" ? "IMEI" : "Serial number"}
              <Input
                value={imei}
                onChange={(e) => {
                  setImei(e.target.value);
                  setLast(null);
                }}
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
                setLast(null);
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
                {conditionCatalog.map((tag) => {
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
                    setLast(null);
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
                  onChange={(e) => {
                    setAmount(e.target.value);
                    setLast(null);
                  }}
                  placeholder={toDiagnose ? "Diagnose first" : "Agreed price"}
                />
              </label>
            </div>

            <label className="checkbox-row">
              <input type="checkbox" checked={assignMe} onChange={(e) => setAssignMe(e.target.checked)} />
              Assign to me
            </label>

            {err ? <p className="form-error">{err}</p> : null}

            <div className="pos-total">
              <span className="muted">Estimate</span>
              <strong>{estimateLabel}</strong>
            </div>
            <Button type="submit" disabled={busy || !canSubmit}>
              {busy ? "Creating…" : "Create job"}
            </Button>
          </form>
        </section>

        {last ? (
          <section className="panel" style={{ padding: "0.85rem" }}>
            <div className="action-block" style={{ margin: 0, paddingTop: 0, borderTop: 0 }}>
              <p className="muted">Last intake</p>
              <dl className="meta-dl">
                <dt>Job</dt>
                <dd className="mono">{last.repair.job_code}</dd>
                <dt>Status</dt>
                <dd>
                  <Badge tone="success">{statusLabel(last.repair.status)}</Badge>
                </dd>
                <dt>Device</dt>
                <dd>{deviceLabel(last.repair)}</dd>
                <dt>Issue</dt>
                <dd>{last.repair.problem_summary}</dd>
                <dt>Price</dt>
                <dd>
                  {(last.repair.labor_amount ?? 0) > 0
                    ? formatMoney(last.repair.labor_amount ?? 0)
                    : "To be diagnosed"}
                </dd>
                {last.repair.pickup_code ? (
                  <>
                    <dt>Pickup code</dt>
                    <dd className="mono">{last.repair.pickup_code}</dd>
                  </>
                ) : null}
              </dl>
              <div className="chip-row" style={{ marginTop: "0.6rem" }}>
                <Button
                  type="button"
                  variant="secondary"
                  onClick={() =>
                    void openIntakeSlip(last.repair.id).catch((e) =>
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
                      void assignRepair(last.repair.id, userId)
                        .then(() => setActionMsg("Assigned to you"))
                        .catch((e) => setActionMsg(e instanceof Error ? e.message : "Assign failed"))
                    }
                  >
                    Assign to me
                  </Button>
                ) : null}
                <Link className="btn btn-ghost" to={`/repairs/${last.repair.id}`}>
                  Open job
                </Link>
              </div>
              {actionMsg ? <p className="hint">{actionMsg}</p> : null}
            </div>
          </section>
        ) : null}
      </aside>
    </div>
  );
}
