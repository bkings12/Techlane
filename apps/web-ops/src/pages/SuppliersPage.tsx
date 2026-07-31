import { useCallback, useEffect, useState, type FormEvent } from "react";
import { Link } from "react-router-dom";
import { useAuth } from "../auth/AuthContext";
import { Avatar, Badge, Button, EmptyState, Input, PageHeader } from "../components/ui";
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
  const [lane, setLane] = useState<"attention" | "roster">("attention");

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
      setLane("roster");
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
    <div className="supplier-desk">
      <PageHeader
        title="Suppliers"
        subtitle="Reconcile first — orphans and credit settle before roster admin."
      />
      {error ? <p className="form-error">{error}</p> : null}

      <section className="board-pulse" aria-label="Supplier pulse">
        <div>
          <strong>{suppliers.length}</strong>
          <span>Suppliers</span>
        </div>
        <div>
          <strong>KES {outstanding.toLocaleString()}</strong>
          <span>Outstanding</span>
        </div>
        <div className={pending.length ? "warn" : ""}>
          <strong>{pending.length}</strong>
          <span>Pending reconcile</span>
        </div>
        <div className={orphans.length ? "warn" : ""}>
          <strong>{orphans.length}</strong>
          <span>Orphans</span>
        </div>
        <button type="button" className={lane === "roster" ? "active" : ""} onClick={() => setLane("roster")}>
          <strong>{selected ? "1" : "0"}</strong>
          <span>Selected</span>
        </button>
      </section>

      <div className="lane-tabs" role="tablist" aria-label="Supplier sections">
        <button type="button" role="tab" aria-selected={lane === "attention"} className={lane === "attention" ? "active" : ""} onClick={() => setLane("attention")}>
          Attention
        </button>
        <button type="button" role="tab" aria-selected={lane === "roster"} className={lane === "roster" ? "active" : ""} onClick={() => setLane("roster")}>
          Roster & credit
        </button>
      </div>

      {lane === "attention" ? (
        <div className="desk-attention">
          <section className={`attention-card ${pending.length ? "warn" : ""}`}>
            <h2>Pending reconciliation</h2>
            {pending.length === 0 ? (
              <EmptyState title="Queue clear" body="Approved or collected parts awaiting credit settlement show here." />
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

          <section className={`attention-card ${orphans.length ? "danger" : ""}`}>
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
                          <Link to={`/repairs/${issue.repair_job_id}`}>Job {issue.repair_job_id.slice(0, 8)}…</Link>
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

          {canWrite ? (
            <section className="attention-card">
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
          ) : (
            <section className="attention-card">
              <h2>Write access</h2>
              <EmptyState title="View only" body="You can reconcile if permitted, but creating suppliers needs suppliers.write." />
            </section>
          )}
        </div>
      ) : (
        <div className="directory-layout">
          <section className="panel" style={{ padding: "0.85rem" }}>
            <div className="panel-head">
              <h2>Supplier roster</h2>
            </div>
            {suppliers.length === 0 ? (
              <EmptyState title="No suppliers" body="A default supplier is seeded on first API start." />
            ) : (
              <ul className="supplier-roster">
                {suppliers.map((s) => (
                  <li key={s.id}>
                    <button
                      type="button"
                      className={`supplier-row ${selectedId === s.id ? "active" : ""}`}
                      onClick={() => setSelectedId(s.id)}
                    >
                      <span className="name-cell">
                        <Avatar name={s.name} size={32} />
                        <span>
                          <strong>{s.name}</strong>
                          <div className="muted">
                            {s.pending_issue_count} pending · KES {s.outstanding_credit.toLocaleString()} open
                          </div>
                        </span>
                      </span>
                      {s.outstanding_credit > 0 ? <Badge tone="warning">credit</Badge> : <Badge tone="success">clear</Badge>}
                    </button>
                  </li>
                ))}
              </ul>
            )}
          </section>

          <aside className="preview-rail">
            <div>
              <h2 style={{ margin: 0 }}>{selected?.name ?? "Select a supplier"}</h2>
              <p className="muted" style={{ margin: "0.35rem 0 0" }}>
                Credit ledger and portal invite
              </p>
            </div>

            <section>
              <h3>Credit detail</h3>
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
                Approving a part posts credit. Reconcile after the invoice matches — that posts a settlement.
              </p>
            </section>

            {canWrite && selected ? (
              <section>
                <h3>Invite contact</h3>
                <form className="form-grid" onSubmit={(event) => void inviteContact(event)}>
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
                    disabled={busy !== null || !contactName.trim() || !contactEmail.trim()}
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
                      portal Accept invite flow.
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
            ) : null}
          </aside>
        </div>
      )}
    </div>
  );
}
