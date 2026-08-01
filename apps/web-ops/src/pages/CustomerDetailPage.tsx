import { useEffect, useState, type FormEvent } from "react";
import { Link, useParams } from "react-router-dom";
import { useAuth } from "../auth/AuthContext";
import { Badge, Button, EmptyState, ICONS, Input, PageHeader } from "../components/ui";
import { SendSmsModal } from "../components/SendSmsModal";
import {
  getCustomer,
  getCustomerLoyalty,
  updateCustomer,
  type Customer,
  type Device,
  type LoyaltyAccount,
  type LoyaltyLedgerEntry,
  type RepairJob,
} from "../lib/api";
import { statusTone } from "../lib/repairStatus";

function can(permissions: string[] | undefined, permission: string) {
  return permissions?.includes("*") || permissions?.includes(permission);
}

export function CustomerDetailPage() {
  const { id } = useParams<{ id: string }>();
  const { user } = useAuth();
  const canEdit = can(user?.permissions, "customers.write");
  const canSms =
    can(user?.permissions, "customers.write") || can(user?.permissions, "repairs.create");

  const [customer, setCustomer] = useState<Customer | null>(null);
  const [devices, setDevices] = useState<Device[]>([]);
  const [repairs, setRepairs] = useState<RepairJob[]>([]);
  const [loyalty, setLoyalty] = useState<LoyaltyAccount | null>(null);
  const [ledger, setLedger] = useState<LoyaltyLedgerEntry[]>([]);
  const [showSms, setShowSms] = useState(false);
  const [error, setError] = useState("");

  const [editing, setEditing] = useState(false);
  const [fullName, setFullName] = useState("");
  const [phone, setPhone] = useState("");
  const [email, setEmail] = useState("");
  const [saving, setSaving] = useState(false);
  const [saveError, setSaveError] = useState("");
  const [saved, setSaved] = useState("");

  useEffect(() => {
    if (!id) return;
    void (async () => {
      try {
        const res = await getCustomer(id);
        setCustomer(res.customer);
        setDevices(res.devices ?? []);
        setRepairs(res.repairs ?? []);
        setFullName(res.customer.full_name);
        setPhone(res.customer.phone ?? "");
        setEmail(res.customer.email ?? "");
      } catch (e) {
        setError(e instanceof Error ? e.message : "Failed to load customer");
      }
    })();
    getCustomerLoyalty(id)
      .then((r) => {
        setLoyalty(r.account);
        setLedger(r.ledger ?? []);
      })
      .catch(() => {
        /* loyalty program may not be enabled — fail silently */
      });
  }, [id]);

  async function onSave(e: FormEvent) {
    e.preventDefault();
    if (!id || !canEdit) return;
    setSaving(true);
    setSaveError("");
    setSaved("");
    try {
      const updated = await updateCustomer(id, {
        full_name: fullName.trim(),
        phone: phone.trim() || null,
        email: email.trim() || null,
      });
      setCustomer(updated);
      setFullName(updated.full_name);
      setPhone(updated.phone ?? "");
      setEmail(updated.email ?? "");
      setEditing(false);
      setSaved("Customer details updated");
    } catch (err) {
      setSaveError(err instanceof Error ? err.message : "Could not save");
    } finally {
      setSaving(false);
    }
  }

  function cancelEdit() {
    if (!customer) return;
    setFullName(customer.full_name);
    setPhone(customer.phone ?? "");
    setEmail(customer.email ?? "");
    setSaveError("");
    setEditing(false);
  }

  if (error) {
    return (
      <div>
        <PageHeader title="Customer" subtitle="Detail" />
        <p className="form-error">{error}</p>
      </div>
    );
  }
  if (!customer) {
    return <div className="boot">Loading…</div>;
  }

  const openJobs = repairs.filter(
    (r) => !["collected", "cancelled", "unrepairable"].includes(r.status),
  ).length;

  return (
    <div className="customer-dossier">
      <PageHeader
        title={customer.full_name}
        subtitle="Customer dossier — devices, jobs, and loyalty in one place."
        actions={
          <>
            {canEdit && !editing ? (
              <Button type="button" variant="secondary" onClick={() => setEditing(true)}>
                Edit details
              </Button>
            ) : null}
            <Link to="/customers" className="btn btn-ghost">
              ← All customers
            </Link>
          </>
        }
      />

      {saved ? (
        <p className="hint" role="status">
          {saved}
        </p>
      ) : null}

      {editing ? (
        <form className="panel form-grid" onSubmit={onSave} style={{ marginBottom: "1rem" }}>
          <h2>Edit customer</h2>
          <label>
            Full name
            <Input value={fullName} onChange={(e) => setFullName(e.target.value)} required autoFocus />
          </label>
          <label>
            Phone
            <Input
              value={phone}
              onChange={(e) => setPhone(e.target.value)}
              inputMode="tel"
              placeholder="07… or 254…"
            />
            <span className="hint">Used for SMS and customer portal login. Leave blank to clear.</span>
          </label>
          <label>
            Email
            <Input
              type="email"
              value={email}
              onChange={(e) => setEmail(e.target.value)}
              placeholder="Optional"
            />
          </label>
          {saveError ? (
            <p className="form-error" role="alert">
              {saveError}
            </p>
          ) : null}
          <div className="chip-row">
            <Button type="submit" disabled={saving || !fullName.trim()}>
              {saving ? "Saving…" : "Save changes"}
            </Button>
            <Button type="button" variant="secondary" onClick={cancelEdit} disabled={saving}>
              Cancel
            </Button>
          </div>
        </form>
      ) : (
        <section className="dossier-hero" aria-label="Customer snapshot">
          <div>
            <span>Contact</span>
            <strong>{customer.phone ?? "No phone"}</strong>
            <div className="muted">{customer.email ?? "No email"}</div>
            {canSms ? (
              <Button
                type="button"
                variant="secondary"
                style={{ marginTop: "0.45rem" }}
                onClick={() => setShowSms(true)}
              >
                Send SMS
              </Button>
            ) : null}
          </div>
          <div>
            <span>Workshop</span>
            <strong>
              {devices.length} device{devices.length === 1 ? "" : "s"}
            </strong>
            <div className="muted">
              {openJobs} open job{openJobs === 1 ? "" : "s"} · {repairs.length} total
            </div>
          </div>
          <div>
            <span>Loyalty</span>
            <strong>{loyalty ? `${loyalty.points_balance} pts` : "Not enrolled"}</strong>
            <div className="muted">{loyalty ? `${ledger.length} recent entries` : "Program off or empty"}</div>
          </div>
        </section>
      )}

      <div className="dossier-grid">
        <section className="panel" style={{ padding: "0.85rem" }}>
          <div className="panel-head">
            <h2>Devices</h2>
            <span className="muted">{devices.length}</span>
          </div>
          {devices.length === 0 ? (
            <EmptyState title="No devices" body="Devices appear when registered on intake." icon={ICONS.repairs} />
          ) : (
            <ul className="device-board">
              {devices.map((d) => (
                <li key={d.id} className="device-tile">
                  <strong>{[d.brand, d.model].filter(Boolean).join(" ") || d.kind}</strong>
                  <span className="muted">
                    {d.kind}
                    {d.anonymous ? " · anonymous" : ""}
                  </span>
                  <span className="mono muted">{d.imei ? `IMEI ${d.imei}` : d.serial_number ? `S/N ${d.serial_number}` : "No IMEI"}</span>
                </li>
              ))}
            </ul>
          )}
        </section>

        <section className="panel" style={{ padding: "0.85rem" }}>
          <div className="panel-head">
            <h2>Repairs</h2>
            <span className="muted">{repairs.length}</span>
          </div>
          {repairs.length === 0 ? (
            <EmptyState title="No repairs" body="No jobs for this customer yet." icon={ICONS.repairs} />
          ) : (
            <ul className="job-board">
              {repairs.map((j) => (
                <li key={j.id}>
                  <Link className="job-board-row" to={`/repairs/${j.id}`}>
                    <div className="job-board-id">
                      <strong className="mono">{j.job_code ?? j.id.slice(0, 8)}</strong>
                      <Badge tone={statusTone(j.status)}>{j.status.replaceAll("_", " ")}</Badge>
                    </div>
                    <div className="job-board-body">
                      <p>{j.problem_summary}</p>
                    </div>
                  </Link>
                </li>
              ))}
            </ul>
          )}
        </section>
      </div>

      {loyalty || ledger.length > 0 ? (
        <section className="panel" style={{ padding: "0.85rem", marginTop: "1rem" }}>
          <div className="panel-head">
            <h2>Loyalty</h2>
            <span className="muted">{loyalty ? `${loyalty.points_balance} pts` : "—"}</span>
          </div>
          {ledger.length === 0 ? (
            <p className="muted">No loyalty activity yet.</p>
          ) : (
            <table className="table">
              <thead>
                <tr>
                  <th>When</th>
                  <th>Entry</th>
                  <th>Points</th>
                </tr>
              </thead>
              <tbody>
                {ledger.map((e) => (
                  <tr key={e.id}>
                    <td className="muted">{e.created_at ? new Date(e.created_at).toLocaleString() : "—"}</td>
                    <td>{e.reason || e.reference_type || "—"}</td>
                    <td className="mono">{e.delta > 0 ? `+${e.delta}` : e.delta}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          )}
        </section>
      ) : null}

      <SendSmsModal
        open={showSms}
        onClose={() => setShowSms(false)}
        initialPhone={customer.phone ?? ""}
        customerId={customer.id}
        title={`SMS to ${customer.full_name}`}
      />
    </div>
  );
}
