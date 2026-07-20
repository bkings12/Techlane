import { useCallback, useEffect, useState, type FormEvent } from "react";
import { Link } from "react-router-dom";
import { useAuth } from "../auth/AuthContext";
import { useBranch } from "../branch/BranchContext";
import { Badge, Button, EmptyState, ICONS, Input, PageHeader, PhotoCaptureField, SearchInput } from "../components/ui";
import {
  createCustomer,
  createDevice,
  createRepair,
  listCustomers,
  listRepairs,
  uploadRepairAttachment,
  type Customer,
  type RepairJob,
} from "../lib/api";

const STATUSES = [
  "",
  "intake",
  "diagnosed",
  "waiting_parts",
  "in_progress",
  "completed",
  "collected",
];

type PendingPhoto = { file: File; preview: string };

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
  const { user } = useAuth();
  const { branchId } = useBranch();
  const [items, setItems] = useState<RepairJob[]>([]);
  const [error, setError] = useState("");
  const [showForm, setShowForm] = useState(false);
  const [q, setQ] = useState("");
  const [status, setStatus] = useState("");
  const [myJobs, setMyJobs] = useState(false);

  const refresh = useCallback(async () => {
    try {
      const res = await listRepairs({
        q: q.trim() || undefined,
        status: status || undefined,
        branch_id: branchId || undefined,
        technician_id: myJobs ? "me" : undefined,
      });
      setItems(res.items ?? []);
      setError("");
    } catch (e) {
      setError(e instanceof Error ? e.message : "Failed to load");
    }
  }, [q, status, branchId, myJobs]);

  useEffect(() => {
    void refresh();
  }, [refresh]);

  return (
    <div>
      <PageHeader
        title="Repairs"
        subtitle="Job cards across branches"
        actions={
          <Button type="button" onClick={() => setShowForm((v) => !v)}>
            New repair
          </Button>
        }
      />
      {error ? <p className="form-error">{error}</p> : null}

      <form
        className="pos-toolbar"
        onSubmit={(e) => {
          e.preventDefault();
          void refresh();
        }}
      >
        <label>
          Search
          <SearchInput
            value={q}
            onChange={(e) => setQ(e.target.value)}
            placeholder="Job code, problem, customer…"
          />
        </label>
        <label>
          Status
          <select className="input" value={status} onChange={(e) => setStatus(e.target.value)}>
            {STATUSES.map((s) => (
              <option key={s || "all"} value={s}>
                {s ? s.replaceAll("_", " ") : "All statuses"}
              </option>
            ))}
          </select>
        </label>
        <label className="checkbox-row">
          <input type="checkbox" checked={myJobs} onChange={(e) => setMyJobs(e.target.checked)} />
          My jobs
        </label>
        <Button type="submit" variant="secondary">
          Apply
        </Button>
      </form>

      {showForm ? (
        <QuickIntake
          branchId={branchId}
          userId={user?.id}
          onCreated={() => {
            setShowForm(false);
            void refresh();
          }}
        />
      ) : null}

      <section className="panel">
        {items.length === 0 ? (
          <EmptyState title="No repairs" body="Create a job card to start the anti-leakage trail." icon={ICONS.repairs} />
        ) : (
          <table className="table">
            <thead>
              <tr>
                <th>Job</th>
                <th>Status</th>
                <th>Customer</th>
                <th>Problem</th>
              </tr>
            </thead>
            <tbody>
              {items.map((j) => (
                <tr key={j.id}>
                  <td>
                    <Link className="mono" to={`/repairs/${j.id}`}>
                      {j.job_code ?? j.id.slice(0, 8)}
                    </Link>
                  </td>
                  <td>
                    <Badge tone="pending">{j.status.replaceAll("_", " ")}</Badge>
                  </td>
                  <td>{j.customer_name ?? j.customer?.full_name ?? "Walk-in"}</td>
                  <td>{j.problem_summary}</td>
                </tr>
              ))}
            </tbody>
          </table>
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
  const [anonymous, setAnonymous] = useState(false);
  const [assignMe, setAssignMe] = useState(true);
  const [name, setName] = useState("");
  const [phone, setPhone] = useState("");
  const [customerId, setCustomerId] = useState("");
  const [matches, setMatches] = useState<Customer[]>([]);
  const [imei, setImei] = useState("");
  const [brand, setBrand] = useState("");
  const [model, setModel] = useState("");
  const [problem, setProblem] = useState("");
  const [imeiPhoto, setImeiPhoto] = useState<PendingPhoto | null>(null);
  const [devicePhoto, setDevicePhoto] = useState<PendingPhoto | null>(null);
  const [busy, setBusy] = useState(false);
  const [err, setErr] = useState("");

  useEffect(() => {
    if (anonymous || name.trim().length < 2) {
      setMatches([]);
      return;
    }
    const t = window.setTimeout(() => {
      listCustomers(name.trim())
        .then((r) => setMatches(r.items ?? []))
        .catch(() => setMatches([]));
    }, 250);
    return () => window.clearTimeout(t);
  }, [name, anonymous]);

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
    if (!branchId) {
      setErr("Select a branch first");
      return;
    }
    setBusy(true);
    setErr("");
    try {
      let custId: string | undefined = customerId || undefined;
      if (!anonymous) {
        if (!custId) {
          if (!name.trim()) throw new Error("Customer name required");
          const customer = await createCustomer({
            full_name: name.trim(),
            phone: phone.trim() || undefined,
          });
          custId = customer.id;
        }
      }
      const device = await createDevice({
        customer_id: anonymous ? undefined : custId,
        anonymous,
        kind: "phone",
        brand: brand.trim() || "Unknown",
        model: model.trim() || "Unknown",
        imei: imei.trim() || undefined,
      });
      const job = await createRepair({
        branch_id: branchId,
        customer_id: anonymous ? undefined : custId,
        device_id: device.id,
        problem_summary: problem,
        technician_id: assignMe && userId ? userId : undefined,
      });
      const uploads: Promise<unknown>[] = [];
      if (imeiPhoto) {
        uploads.push(
          fileToBase64(imeiPhoto.file).then((data_base64) =>
            uploadRepairAttachment(job.id, {
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
            uploadRepairAttachment(job.id, {
              file_name: devicePhoto.file.name || "device-condition.jpg",
              content_type: devicePhoto.file.type || "image/jpeg",
              data_base64,
            }),
          ),
        );
      }
      if (uploads.length) await Promise.all(uploads);
      onCreated();
    } catch (error) {
      setErr(error instanceof Error ? error.message : "Failed");
    } finally {
      setBusy(false);
    }
  }

  return (
    <form className="panel form-grid" onSubmit={submit}>
      <h2>Quick intake</h2>
      <label className="checkbox-row">
        <input
          type="checkbox"
          checked={anonymous}
          onChange={(e) => {
            setAnonymous(e.target.checked);
            setCustomerId("");
          }}
        />
        Anonymous walk-in (no customer record)
      </label>
      <label className="checkbox-row">
        <input type="checkbox" checked={assignMe} onChange={(e) => setAssignMe(e.target.checked)} />
        Assign to me
      </label>
      {!anonymous ? (
        <>
          <label>
            Customer
            <Input
              value={name}
              onChange={(e) => {
                setName(e.target.value);
                setCustomerId("");
              }}
              required={!customerId}
              placeholder="Name"
            />
          </label>
          {matches.length > 0 && !customerId ? (
            <ul className="list">
              {matches.slice(0, 5).map((c) => (
                <li key={c.id}>
                  <button
                    type="button"
                    className="linkish"
                    onClick={() => {
                      setCustomerId(c.id);
                      setName(c.full_name);
                      setPhone(c.phone ?? "");
                      setMatches([]);
                    }}
                  >
                    {c.full_name}
                    {c.phone ? ` · ${c.phone}` : ""}
                  </button>
                </li>
              ))}
            </ul>
          ) : null}
          {customerId ? <p className="hint">Using existing customer {customerId.slice(0, 8)}…</p> : null}
          <label>
            Phone
            <Input value={phone} onChange={(e) => setPhone(e.target.value)} />
          </label>
        </>
      ) : null}
      <label>
        Brand
        <Input value={brand} onChange={(e) => setBrand(e.target.value)} placeholder="Unknown" />
      </label>
      <label>
        Model
        <Input value={model} onChange={(e) => setModel(e.target.value)} placeholder="Unknown" />
      </label>
      <label>
        IMEI / serial
        <Input
          value={imei}
          onChange={(e) => setImei(e.target.value)}
          className="mono"
          placeholder="Type, or photograph the sticker"
        />
      </label>
      <PhotoCaptureField
        label="IMEI / serial photo"
        hint="Instead of typing — photo of the sticker or barcode. Scans automatically when the browser can."
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
        label="Device condition"
        hint="Cracks, missing parts, water marks — proof at intake."
        previewUrl={devicePhoto?.preview}
        onFile={(file) => void onDevicePhoto(file)}
        onClear={() =>
          setDevicePhoto((prev) => {
            if (prev) URL.revokeObjectURL(prev.preview);
            return null;
          })
        }
      />
      <label>
        Problem
        <Input value={problem} onChange={(e) => setProblem(e.target.value)} required />
      </label>
      {err ? <p className="form-error">{err}</p> : null}
      <Button type="submit" disabled={busy || !branchId}>
        {busy ? "Saving…" : "Create job"}
      </Button>
    </form>
  );
}
