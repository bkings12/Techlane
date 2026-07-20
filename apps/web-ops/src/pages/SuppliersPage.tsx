import { useCallback, useEffect, useState, type FormEvent } from "react";
import { Link } from "react-router-dom";
import { useAuth } from "../auth/AuthContext";
import { Avatar, Badge, Button, EmptyState, ICONS, Input, PageHeader, Stat, StatStrip } from "../components/ui";
import {
  createSupplier,
  inviteSupplierContact,
  listOrphanIssues,
  listPendingReconciliation,
  listSupplierCredit,
  listSuppliers,
  reconcileSupplierIssue,
  type CreditEntry,
  type PendingSupplierIssue,
  type Supplier,
  type SupplierInviteResult,
  type SupplierIssue,
} from "../lib/api";

function can(permissions: string[] | undefined, permission: string) {
  return permissions?.includes("*") || permissions?.includes(permission);
}

export function SuppliersPage() {
  const { user } = useAuth();
  const [suppliers, setSuppliers] = useState<Supplier[]>([]);
  const [pending, setPending] = useState<PendingSupplierIssue[]>([]);
  const [orphans, setOrphans] = useState<SupplierIssue[]>([]);
  const [selectedId, setSelectedId] = useState("");
  const [credit, setCredit] = useState<CreditEntry[]>([]);
  const [error, setError] = useState("");
  const [busy, setBusy] = useState<string | null>(null);
  const [supplierName, setSupplierName] = useState("");
  const [supplierPhone, setSupplierPhone] = useState("");
  const [contactEmail, setContactEmail] = useState("");
  const [contactName, setContactName] = useState("");
  const [contactPhone, setContactPhone] = useState("");
  const [invite, setInvite] = useState<SupplierInviteResult | null>(null);
  const [copied, setCopied] = useState(false);

  const refresh = useCallback(async () => {
    const [s, p, o] = await Promise.all([
      listSuppliers(),
      listPendingReconciliation(),
      listOrphanIssues().catch(() => ({ items: [] as SupplierIssue[] })),
    ]);
    setSuppliers(s.items ?? []);
    setPending(p.items ?? []);
    setOrphans(o.items ?? []);
    setSelectedId((prev) => prev || s.items?.[0]?.id || "");
  }, []);

  useEffect(() => {
    refresh().catch((e) => setError(e instanceof Error ? e.message : "Failed to load"));
  }, [refresh]);

  useEffect(() => {
    if (!selectedId) {
      setCredit([]);
      return;
    }
    listSupplierCredit(selectedId)
      .then((r) => setCredit(r.items ?? []))
      .catch((e) => setError(e instanceof Error ? e.message : "Credit failed"));
  }, [selectedId]);

  async function reconcile(id: string) {
    setBusy(id);
    setError("");
    try {
      await reconcileSupplierIssue(id);
      await refresh();
      if (selectedId) {
        const r = await listSupplierCredit(selectedId);
        setCredit(r.items ?? []);
      }
    } catch (e) {
      setError(e instanceof Error ? e.message : "Reconcile failed");
    } finally {
      setBusy(null);
    }
  }

  async function addSupplier(event: FormEvent) {
    event.preventDefault();
    setBusy("create-supplier");
    setError("");
    try {
      const created = await createSupplier({
        name: supplierName.trim(),
        phone: supplierPhone.trim() || undefined,
      });
      setSupplierName("");
      setSupplierPhone("");
      await refresh();
      setSelectedId(created.id);
    } catch (e) {
      setError(e instanceof Error ? e.message : "Create supplier failed");
    } finally {
      setBusy(null);
    }
  }

  async function inviteContact(event: FormEvent) {
    event.preventDefault();
    if (!selectedId) return;
    setBusy("invite-contact");
    setError("");
    setInvite(null);
    setCopied(false);
    try {
      const result = await inviteSupplierContact(selectedId, {
        email: contactEmail.trim(),
        display_name: contactName.trim(),
        phone: contactPhone.trim() || undefined,
      });
      setInvite(result);
      setContactEmail("");
      setContactName("");
      setContactPhone("");
    } catch (e) {
      setError(e instanceof Error ? e.message : "Invite failed");
    } finally {
      setBusy(null);
    }
  }

  async function copyInviteToken() {
    if (!invite) return;
    try {
      await navigator.clipboard.writeText(invite.invite_token);
      setCopied(true);
    } catch {
      setError("Could not copy automatically. Select the token and copy it.");
    }
  }

  const outstanding = suppliers.reduce((sum, s) => sum + (s.outstanding_credit || 0), 0);
  const selected = suppliers.find((s) => s.id === selectedId);
  const canWrite = can(user?.permissions, "suppliers.write");

  return (
    <div className="suppliers-page">
      <PageHeader title="Suppliers" subtitle="Credit ledger, orphans, and part issue reconciliation" />
      {error ? <p className="form-error">{error}</p> : null}

      <StatStrip>
        <Stat icon={ICONS.suppliers} label="Suppliers" value={suppliers.length} />
        <Stat icon={ICONS.cash} label="Outstanding credit" value={`KES ${outstanding.toLocaleString()}`} />
        <Stat icon={ICONS.clock} label="Pending reconcile" value={pending.length} tone={pending.length ? "warn" : undefined} />
        <Stat icon={ICONS.risk} label="Orphans" value={orphans.length} tone={orphans.length ? "danger" : undefined} />
      </StatStrip>

      {canWrite ? (
        <div className="repair-grid">
          <section className="panel">
            <h2>Add supplier</h2>
            <form className="form-grid" onSubmit={(event) => void addSupplier(event)}>
              <label>
                Supplier name
                <Input value={supplierName} onChange={(e) => setSupplierName(e.target.value)} required />
              </label>
              <label>
                Phone (optional)
                <Input value={supplierPhone} onChange={(e) => setSupplierPhone(e.target.value)} />
              </label>
              <Button type="submit" disabled={busy !== null || !supplierName.trim()}>
                Create supplier
              </Button>
            </form>
          </section>

          <section className="panel">
            <h2>Invite supplier contact{selected ? ` · ${selected.name}` : ""}</h2>
            <form className="form-grid" onSubmit={(event) => void inviteContact(event)}>
              <label>
                Supplier
                <select className="input" value={selectedId} onChange={(e) => setSelectedId(e.target.value)} required>
                  <option value="">Select…</option>
                  {suppliers.map((supplier) => (
                    <option key={supplier.id} value={supplier.id}>
                      {supplier.name}
                    </option>
                  ))}
                </select>
              </label>
              <label>
                Contact name
                <Input value={contactName} onChange={(e) => setContactName(e.target.value)} required />
              </label>
              <label>
                Email
                <Input type="email" value={contactEmail} onChange={(e) => setContactEmail(e.target.value)} required />
              </label>
              <label>
                Phone (optional)
                <Input value={contactPhone} onChange={(e) => setContactPhone(e.target.value)} />
              </label>
              <Button
                type="submit"
                disabled={busy !== null || !selectedId || !contactName.trim() || !contactEmail.trim()}
              >
                Send invite
              </Button>
            </form>
            {invite ? (
              <div className="auth-block invite-result" style={{ marginTop: "1rem" }}>
                <div className="part-head">
                  <div>
                    <strong>{invite.contact.display_name}</strong>
                    <div className="muted">{invite.contact.email}</div>
                  </div>
                  <Badge tone="pending">{invite.contact.status}</Badge>
                </div>
                <p className="form-success">
                  <Badge tone="success">invite created</Badge> Token is shown once — paste it into the supplier
                  portal Accept invite flow (http://localhost:5176).
                </p>
                <label>
                  Invite token
                  <input
                    className="input invite-token-input"
                    readOnly
                    value={invite.invite_token}
                    onFocus={(e) => e.target.select()}
                  />
                </label>
                <div className="chip-row">
                  <Button type="button" onClick={() => void copyInviteToken()}>
                    {copied ? "Copied" : "Copy invite token"}
                  </Button>
                </div>
                <p className="muted">Expires {new Date(invite.expires_at).toLocaleString()}</p>
              </div>
            ) : null}
          </section>
        </div>
      ) : null}

      <div className="repair-grid">
        <section className="panel">
          <h2>Pending reconciliation</h2>
          {pending.length === 0 ? (
            <EmptyState
              title="Queue clear"
              body="Approved or collected parts awaiting credit settlement will show here."
            />
          ) : (
            <ul className="part-list">
              {pending.map((issue) => (
                <li key={issue.id} className="part-card">
                  <div className="part-head">
                    <div>
                      <strong>{issue.description}</strong>
                      <div className="muted">
                        {issue.supplier_name} ·{" "}
                        <Link className="mono" to={`/repairs/${issue.repair_job_id}`}>
                          {issue.job_code || issue.repair_job_id.slice(0, 8)}
                        </Link>
                      </div>
                    </div>
                    <Badge tone={issue.status === "collected" ? "success" : "pending"}>{issue.status}</Badge>
                  </div>
                  <div className="inline-form">
                    <div>
                      <span className="muted">Unit cost</span>
                      <div>
                        <strong>KES {issue.unit_cost.toLocaleString()}</strong>
                      </div>
                      <code className="auth-code" style={{ fontSize: "1rem" }}>
                        {issue.auth_code}
                      </code>
                    </div>
                    <Button type="button" disabled={busy === issue.id} onClick={() => void reconcile(issue.id)}>
                      Mark reconciled
                    </Button>
                  </div>
                </li>
              ))}
            </ul>
          )}
        </section>

        <aside className="stack">
          <section className="panel">
            <h2>Orphan issues</h2>
            {orphans.length === 0 ? (
              <EmptyState title="No orphans" body="Collected parts without a completed job appear here." />
            ) : (
              <ul className="part-list">
                {orphans.map((issue) => (
                  <li key={issue.id} className="part-card">
                    <div className="part-head">
                      <div>
                        <strong className="mono">{issue.auth_code}</strong>
                        <div className="muted">
                          <Link to={`/repairs/${issue.repair_job_id}`}>
                            Job {issue.repair_job_id.slice(0, 8)}…
                          </Link>
                          {" · "}KES {issue.unit_cost.toLocaleString()}
                        </div>
                      </div>
                      <Badge tone="danger">{issue.status}</Badge>
                    </div>
                  </li>
                ))}
              </ul>
            )}
          </section>

          <section className="panel context-panel">
            <h2>Supplier balances</h2>
            {suppliers.length === 0 ? (
              <EmptyState title="No suppliers" body="A default supplier is seeded on first API start." />
            ) : (
              <ul className="list">
                {suppliers.map((s) => (
                  <li key={s.id}>
                    <button
                      type="button"
                      className="linkish name-cell"
                      onClick={() => setSelectedId(s.id)}
                      style={{ textAlign: "left" }}
                    >
                      <Avatar name={s.name} size={30} />
                      <span className="name-cell-text">
                        <strong>{s.name}</strong>
                        <span className="muted">
                          {s.pending_issue_count} pending · KES {s.outstanding_credit.toLocaleString()} open
                          {selectedId === s.id ? " · selected" : ""}
                        </span>
                      </span>
                    </button>
                  </li>
                ))}
              </ul>
            )}
          </section>

          <section className="panel">
            <h2>Credit detail{selected ? ` · ${selected.name}` : ""}</h2>
            {credit.length === 0 ? (
              <EmptyState title="No credit entries" body="Approve a part to post supplier credit." />
            ) : (
              <ul className="list">
                {credit.map((c) => (
                  <li key={c.id}>
                    <Badge tone={c.entry_type === "settlement" ? "success" : "pending"}>{c.entry_type}</Badge>
                    <span>
                      KES {c.amount.toLocaleString()} · {new Date(c.created_at).toLocaleString()}
                    </span>
                  </li>
                ))}
              </ul>
            )}
            <p className="hint">
              Approving a part posts supplier credit. Reconcile after the invoice matches — that posts a settlement
              and clears related orphan alerts.
            </p>
          </section>
        </aside>
      </div>
    </div>
  );
}
