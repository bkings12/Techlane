import { useCallback, useEffect, useMemo, useState, type FormEvent } from "react";
import { Link, useNavigate } from "react-router-dom";
import { useAuth } from "../auth/AuthContext";
import { useBranch } from "../branch/BranchContext";
import { Badge, Button, Input, PageHeader, PhotoCaptureField } from "../components/ui";
import { SearchableCombobox, type ComboOption } from "../components/SearchableCombobox";
import {
  assignRepair,
  createCustomer,
  intakeRepair,
  listCustomers,
  listIntakePresets,
  openIntakeSlip,
  uploadRepairAttachment,
  type Customer,
  type IntakeResult,
  type RepairJob,
} from "../lib/api";
import { statusLabel } from "../lib/repairStatus";
import { useCurrency } from "../lib/currency";

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
const QUICK_PICK_CAP = 5;

type DeviceKind = (typeof DEVICE_KINDS)[number];
type PendingPhoto = { file: File; preview: string };

function deviceLabel(job: RepairJob) {
  const brand = job.device?.brand?.trim();
  const model = job.device?.model?.trim();
  if (brand && model) return `${brand} ${model}`;
  if (brand) return brand;
  if (model) return model;
  return "Device";
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


export function JobPosPage() {
  const { user } = useAuth();
  const { branchId } = useBranch();
  const navigate = useNavigate();

  return (
    <div className="jobs-desk">
      <PageHeader
        title="Job POS"
        subtitle="Take a device in — customer, device, issue, then create the job."
      />
      <QuickIntake
        branchId={branchId}
        userId={user?.id}
        onCreated={(job) => navigate(`/repairs/${job.id}`)}
      />
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
  onCreated: (job: RepairJob) => void;
}) {
  const { formatMoney } = useCurrency();
  const [recentCustomers, setRecentCustomers] = useState<Customer[]>([]);
  const [expandCustomers, setExpandCustomers] = useState(false);
  const [expandDevices, setExpandDevices] = useState(false);
  const [expandIssues, setExpandIssues] = useState(false);

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
    // TODO: order by last-visit date once available
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
        // API already orders by sort_order — keep that ranking for quick picks.
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

  useEffect(() => {
    setExpandDevices(false);
  }, [deviceKind]);

  const deviceOptions = useMemo(() => {
    return RECENT_DEVICES.filter((d) => !d.sublabel || d.sublabel === deviceKind || deviceKind === "other");
  }, [deviceKind]);

  const quickCustomers = useMemo(() => {
    const walkIn = { id: WALK_IN_VALUE, full_name: "Walk-in", phone: "No record" };
    const rest = recentCustomers.map((c) => ({
      id: c.id,
      full_name: c.full_name,
      phone: c.phone || "No phone",
    }));
    const capped = rest.slice(0, QUICK_PICK_CAP);
    return {
      items: [walkIn, ...(expandCustomers ? rest : capped)],
      more: rest.length > QUICK_PICK_CAP,
    };
  }, [recentCustomers, expandCustomers]);

  const quickDevices = useMemo(() => {
    const capped = deviceOptions.slice(0, QUICK_PICK_CAP);
    return {
      items: expandDevices ? deviceOptions : capped,
      more: deviceOptions.length > QUICK_PICK_CAP,
    };
  }, [deviceOptions, expandDevices]);

  const quickIssues = useMemo(() => {
    const capped = issueOptions.slice(0, QUICK_PICK_CAP);
    return {
      items: expandIssues ? issueOptions : capped,
      more: issueOptions.length > QUICK_PICK_CAP,
    };
  }, [issueOptions, expandIssues]);

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
      onCreated(result.repair);
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
            <span className="muted">Shortcuts for the common case — search the ticket fields for anything else</span>
          </div>
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
          {quickCustomers.items.map((c) => (
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
        {quickCustomers.more ? (
          <button type="button" className="linkish" onClick={() => setExpandCustomers((v) => !v)}>
            {expandCustomers ? "Show fewer" : "More customers"}
          </button>
        ) : null}

        <h3 className="intake-pick-heading">Devices</h3>
        {quickDevices.items.length === 0 ? (
          <p className="muted">No device presets for this kind — search or add on the ticket.</p>
        ) : (
          <ul className="pos-catalog-grid">
            {quickDevices.items.map((d) => (
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
        {quickDevices.more ? (
          <button type="button" className="linkish" onClick={() => setExpandDevices((v) => !v)}>
            {expandDevices ? "Show fewer" : "More devices"}
          </button>
        ) : null}

        <h3 className="intake-pick-heading">Common issues</h3>
        <ul className="pos-catalog-grid">
          {quickIssues.items.map((issue) => (
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
        {quickIssues.more ? (
          <button type="button" className="linkish" onClick={() => setExpandIssues((v) => !v)}>
            {expandIssues ? "Show fewer" : "More issues"}
          </button>
        ) : null}
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
