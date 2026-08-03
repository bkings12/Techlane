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
const WALK_IN_OPTION: ComboOption = {
  value: WALK_IN_VALUE,
  label: "Walk-in (no record)",
  sublabel: "Anonymous check-in",
};

function phoneDigits(s: string): string {
  return s.replace(/\D/g, "");
}

function phonesMatch(a: string, b: string): boolean {
  const da = phoneDigits(a);
  const db = phoneDigits(b);
  if (da.length < 9 || db.length < 9) return false;
  return da === db || da.endsWith(db) || db.endsWith(da);
}

/** Turn free-typed device text into brand/model (and prefer an exact catalog hit). */
function parseFreeDevice(query: string, options: ComboOption[]): { brand: string; model: string; value: string } {
  const t = query.trim();
  if (!t) return { brand: "", model: "", value: "" };
  const lower = t.toLowerCase();
  const exact = options.find((o) => o.label.toLowerCase() === lower);
  if (exact) {
    const [b, ...rest] = exact.value.split("|");
    return { brand: b || exact.label, model: rest.join("|") || "", value: exact.value };
  }
  const parts = t.split(/\s+/).filter(Boolean);
  if (parts.length === 1) return { brand: parts[0]!, model: "", value: `${parts[0]}|` };
  return {
    brand: parts[0]!,
    model: parts.slice(1).join(" "),
    value: `${parts[0]}|${parts.slice(1).join(" ")}`,
  };
}

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
        actions={
          <nav className="job-pos-quick" aria-label="Quick actions">
            <Link to="/pos" className="btn btn-secondary">
              Sell
            </Link>
            <Link to="/counter/fix" className="btn btn-ghost">
              Same-day fix
            </Link>
            <Link to="/counter/pickup" className="btn btn-ghost">
              Pickup
            </Link>
          </nav>
        }
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

  const [customerValue, setCustomerValue] = useState("");
  const [customerOptions, setCustomerOptions] = useState<ComboOption[]>([WALK_IN_OPTION]);
  const [customerSearching, setCustomerSearching] = useState(false);
  const [customerQueryDraft, setCustomerQueryDraft] = useState("");
  const [customerName, setCustomerName] = useState("");
  const [customerPhone, setCustomerPhone] = useState("");
  const [anonymous, setAnonymous] = useState(false);

  const [deviceKind, setDeviceKind] = useState<DeviceKind>("phone");
  const [deviceValue, setDeviceValue] = useState("");
  const [devicePickKey, setDevicePickKey] = useState(0);
  const [brand, setBrand] = useState("");
  const [model, setModel] = useState("");
  const [imei, setImei] = useState("");

  const [issueValue, setIssueValue] = useState("");
  const [issuePickKey, setIssuePickKey] = useState(0);
  const [issueOptions, setIssueOptions] = useState<ComboOption[]>(FALLBACK_ISSUES);
  const [conditionCatalog, setConditionCatalog] = useState<string[]>(FALLBACK_CONDITION_TAGS);
  const [conditionPickKey, setConditionPickKey] = useState(0);
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

  const conditionOptions = useMemo(
    () =>
      conditionCatalog
        .filter((tag) => !conditionTags.includes(tag))
        .map((tag) => ({ value: tag, label: tag })),
    [conditionCatalog, conditionTags],
  );

  const canSubmit =
    Boolean(branchId) &&
    Boolean(problem.trim()) &&
    Boolean(deviceKind) &&
    Boolean(brand.trim()) &&
    (anonymous || Boolean(customerValue)) &&
    (!toDiagnose ? Number(amount) > 0 : true);

  const submitBlockedReason = !canSubmit
    ? !branchId
      ? "Select a branch first"
      : !anonymous && !customerValue
        ? customerQueryDraft.trim()
          ? "Tap a customer in the search list (or + to add, or Walk-in)"
          : "Select a customer (or Walk-in)"
        : !brand.trim()
          ? "Type or pick a device (brand / model)"
          : !problem.trim()
            ? "Type or pick an issue"
            : !toDiagnose && !(Number(amount) > 0)
              ? "Enter an amount, or check To be diagnosed"
              : "Fill required ticket fields"
    : "";

  const estimateLabel = toDiagnose
    ? "To diagnose"
    : Number(amount) > 0
      ? formatMoney(Number(amount))
      : "—";

  const searchCustomers = useCallback((query: string) => {
    const q = query.trim();
    if (q.length < 2) {
      setCustomerOptions([WALK_IN_OPTION]);
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
        // Walk-in last so Enter / highlight prefers a real customer match.
        setCustomerOptions([...items, WALK_IN_OPTION]);
      })
      .catch(() => setCustomerOptions([WALK_IN_OPTION]))
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

  // Typing a phone into search does not select anyone until they tap / press Enter.
  // Auto-pick when there is a single clear phone match so Create job can enable.
  useEffect(() => {
    if (customerSearching || customerValue || anonymous) return;
    const q = customerQueryDraft.trim();
    if (!q) return;
    const real = customerOptions.filter((o) => o.value && o.value !== WALK_IN_VALUE);
    if (real.length === 0) return;

    const byPhone = real.filter((o) => phonesMatch(q, o.sublabel ?? ""));
    if (byPhone.length === 1) {
      onCustomerSelect(byPhone[0]!);
      return;
    }
    if (real.length === 1 && phoneDigits(q).length >= 9) {
      onCustomerSelect(real[0]!);
    }
  }, [customerOptions, customerSearching, customerQueryDraft, customerValue, anonymous]);

  function onDeviceSelect(opt: ComboOption) {
    setDeviceValue(opt.value);
    setLast(null);
    if (!opt.value) {
      // Cleared by typing — free-text handler fills brand/model from the query.
      return;
    }
    const [b, ...rest] = opt.value.split("|");
    setBrand(b || opt.label);
    setModel(rest.join("|") || "");
  }

  function onDeviceQueryChange(query: string) {
    // Commit typed text immediately so the ticket fills without requiring a list tap.
    const parsed = parseFreeDevice(query, deviceOptions);
    setBrand(parsed.brand);
    setModel(parsed.model);
    if (parsed.value && deviceOptions.some((o) => o.value === parsed.value)) {
      setDeviceValue(parsed.value);
    } else {
      setDeviceValue("");
    }
    setLast(null);
  }

  function onIssueSelect(opt: ComboOption) {
    setIssueValue(opt.value);
    setLast(null);
    if (!opt.value) {
      // Cleared by typing — free-text handler keeps problem in sync with the query.
      return;
    }
    setProblem(opt.label || opt.value);
  }

  function onIssueQueryChange(query: string) {
    const q = query.trim();
    setProblem(q);
    if (!q) {
      setIssueValue("");
      setLast(null);
      return;
    }
    const exact = issueOptions.find((o) => o.label.toLowerCase() === q.toLowerCase());
    setIssueValue(exact?.value ?? "");
    setLast(null);
  }

  function addConditionTag(tag: string) {
    const label = tag.trim();
    if (!label) return;
    setConditionTags((cur) => (cur.includes(label) ? cur : [...cur, label]));
    setConditionCatalog((cur) => (cur.includes(label) ? cur : [...cur, label]));
    setConditionPickKey((k) => k + 1);
    setLast(null);
  }

  function removeConditionTag(tag: string) {
    setConditionTags((cur) => cur.filter((t) => t !== tag));
    setLast(null);
  }

  function clearTicket() {
    setCustomerValue("");
    setCustomerName("");
    setCustomerPhone("");
    setCustomerQueryDraft("");
    setAnonymous(false);
    setDeviceKind("phone");
    setDeviceValue("");
    setDevicePickKey((k) => k + 1);
    setBrand("");
    setModel("");
    setImei("");
    setIssueValue("");
    setIssuePickKey((k) => k + 1);
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
      setCustomerQueryDraft("");
      setAnonymous(false);
      setDeviceValue("");
      setDevicePickKey((k) => k + 1);
      setBrand("");
      setModel("");
      setImei("");
      setIssueValue("");
      setIssuePickKey((k) => k + 1);
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
    Boolean(customerValue) ||
    Boolean(customerQueryDraft.trim()) ||
    Boolean(brand) ||
    Boolean(model) ||
    Boolean(problem) ||
    conditionTags.length > 0;

  const customerTicketLabel = !customerValue
    ? customerQueryDraft.trim()
      ? `Tap to confirm · ${customerQueryDraft.trim()}`
      : ""
    : anonymous
      ? "Walk-in (no record)"
      : [customerName, customerPhone].filter(Boolean).join(" · ") || "Customer selected";

  const deviceTicketLabel =
    brand || model ? `${brand}${brand && model ? " " : ""}${model}`.trim() : "";

  const issueTicketLabel = problem.trim();

  return (
    <div className="repair-grid" style={{ marginBottom: "1.25rem" }}>
      <section className="panel" style={{ padding: "0.85rem" }}>
        <div className="panel-head">
          <div>
            <h2>Quick pick</h2>
            <span className="muted">Search to fill the ticket</span>
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
                    setBrand("");
                    setModel("");
                    setLast(null);
                  }}
                >
                  {k}
                </button>
              ))}
            </div>
          </fieldset>
        </div>

        <h3 className="intake-pick-heading">Customer</h3>
        <SearchableCombobox
          label="Search customer"
          placeholder="Search name or phone…"
          options={customerOptions}
          onSearch={searchCustomers}
          value={customerValue}
          loading={customerSearching}
          onSelect={onCustomerSelect}
          onQueryChange={setCustomerQueryDraft}
          addNewFields={{ primary: "Full name", secondary: "Phone number" }}
          onAddNew={async ({ primary, secondary }) => {
            const c = await createCustomer({
              full_name: primary,
              phone: secondary || undefined,
            });
            const opt = { value: c.id, label: c.full_name, sublabel: c.phone || "No phone" };
            setCustomerOptions((prev) => {
              const rest = prev.filter((p) => p.value !== WALK_IN_VALUE && p.value !== c.id);
              return [opt, ...rest, WALK_IN_OPTION];
            });
            setCustomerName(c.full_name);
            setCustomerPhone(c.phone ?? "");
            setAnonymous(false);
            setLast(null);
            return opt;
          }}
        />

        <h3 className="intake-pick-heading">Device</h3>
        <SearchableCombobox
          key={`device-${devicePickKey}`}
          label="Search device"
          placeholder="Brand and model…"
          options={deviceOptions}
          value={deviceValue}
          onSelect={onDeviceSelect}
          onQueryChange={onDeviceQueryChange}
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

        <h3 className="intake-pick-heading">Issue</h3>
        <SearchableCombobox
          key={`issue-${issuePickKey}`}
          label="Search issue"
          placeholder="What’s wrong?"
          options={issueOptions}
          value={issueValue}
          onSelect={onIssueSelect}
          onQueryChange={onIssueQueryChange}
          addNewFields={{ primary: "Describe the issue" }}
          onAddNew={async ({ primary }) => {
            const opt = { value: primary, label: primary };
            setIssueOptions((prev) => [opt, ...prev.filter((p) => p.value !== primary)]);
            setProblem(primary);
            setLast(null);
            return opt;
          }}
        />
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
            <div className="ticket-summary">
              <div className="ticket-line">
                <span className="ticket-line-label">Customer</span>
                <span className="ticket-line-value">{customerTicketLabel || "Not set"}</span>
                {customerValue ? (
                  <button
                    type="button"
                    className="linkish"
                    onClick={() => {
                      setCustomerValue("");
                      setCustomerName("");
                      setCustomerPhone("");
                      setAnonymous(false);
                      setLast(null);
                    }}
                  >
                    Change
                  </button>
                ) : null}
              </div>

              <div className="ticket-line">
                <span className="ticket-line-label">Device</span>
                <span className="ticket-line-value">
                  {deviceTicketLabel ? `${deviceTicketLabel} · ${deviceKind}` : "Not set"}
                </span>
                {deviceTicketLabel || deviceValue ? (
                  <button
                    type="button"
                    className="linkish"
                    onClick={() => {
                      setDeviceValue("");
                      setBrand("");
                      setModel("");
                      setDevicePickKey((k) => k + 1);
                      setLast(null);
                    }}
                  >
                    Change
                  </button>
                ) : null}
              </div>

              <div className="ticket-line">
                <span className="ticket-line-label">Issue</span>
                <span className="ticket-line-value">{issueTicketLabel || "Not set"}</span>
                {issueTicketLabel || issueValue ? (
                  <button
                    type="button"
                    className="linkish"
                    onClick={() => {
                      setIssueValue("");
                      setProblem("");
                      setIssuePickKey((k) => k + 1);
                      setLast(null);
                    }}
                  >
                    Change
                  </button>
                ) : null}
              </div>
            </div>

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

            <details className="ticket-photos">
              <summary>
                Photos
                <span className="muted">
                  {imeiPhoto || devicePhoto
                    ? ` · ${(imeiPhoto ? 1 : 0) + (devicePhoto ? 1 : 0)} attached`
                    : " · optional"}
                </span>
              </summary>
              <div className="ticket-photos-body">
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
              </div>
            </details>

            <div className="stack-form" style={{ gap: "0.45rem" }}>
              <SearchableCombobox
                key={conditionPickKey}
                label="Condition tags"
                placeholder="Search condition…"
                options={conditionOptions}
                value=""
                onSelect={(opt) => addConditionTag(opt.value || opt.label)}
                addNewFields={{ primary: "Condition note" }}
                onAddNew={async ({ primary }) => {
                  addConditionTag(primary);
                  return { value: primary.trim(), label: primary.trim() };
                }}
              />
              {conditionTags.length > 0 ? (
                <div className="intake-tag-row">
                  {conditionTags.map((tag) => (
                    <button
                      key={tag}
                      type="button"
                      className="intake-tag is-on"
                      onClick={() => removeConditionTag(tag)}
                      title="Remove"
                    >
                      {tag}
                    </button>
                  ))}
                </div>
              ) : (
                <p className="muted" style={{ margin: 0 }}>
                  None yet — search or add above. Tap a selected tag to remove it.
                </p>
              )}
            </div>

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
            {!busy && submitBlockedReason ? (
              <p className="muted" style={{ margin: 0, fontSize: "0.85rem" }}>
                {submitBlockedReason}
              </p>
            ) : null}
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
