import { useCallback, useEffect, useState } from "react";
import { Link } from "react-router-dom";
import { useAuth } from "../auth/AuthContext";
import { Badge, Button, EmptyState, ICONS, Input, PageHeader, Stat, StatStrip } from "../components/ui";
import {
  assignPartRequest,
  collectSupplierIssue,
  listAllPartRequests,
  listSuppliers,
  type PartRequest,
  type Supplier,
} from "../lib/api";

function statusTone(status: string): "success" | "warning" | "danger" | "info" | "pending" {
  if (status === "collected" || status === "approved") return "success";
  if (status === "pending") return "pending";
  if (status.includes("fail") || status === "orphan") return "danger";
  return "info";
}

function can(permissions: string[] | undefined, permission: string) {
  return permissions?.includes("*") || permissions?.includes(permission);
}

export function PartRequestsPage() {
  const { user } = useAuth();
  const [rows, setRows] = useState<PartRequest[]>([]);
  const [suppliers, setSuppliers] = useState<Supplier[]>([]);
  const [error, setError] = useState("");
  const [busy, setBusy] = useState(false);
  const [collectCode, setCollectCode] = useState<Record<string, string>>({});
  const [supplierByRequest, setSupplierByRequest] = useState<Record<string, string>>({});
  const [filter, setFilter] = useState<"pending" | "all">("pending");

  const refresh = useCallback(async () => {
    setBusy(true);
    setError("");
    try {
      const [res, supplierRes] = await Promise.all([
        listAllPartRequests(filter === "pending" ? { status: "pending" } : undefined),
        listSuppliers().catch(() => ({ items: [] as Supplier[] })),
      ]);
      setRows(res.items ?? []);
      setSuppliers(supplierRes.items ?? []);
      setSupplierByRequest((current) => {
        const next = { ...current };
        for (const row of res.items ?? []) {
          if (!next[row.id] && row.assigned_supplier_id) next[row.id] = row.assigned_supplier_id;
        }
        return next;
      });
    } catch (e) {
      setError(e instanceof Error ? e.message : "Failed to load");
    } finally {
      setBusy(false);
    }
  }, [filter]);

  useEffect(() => {
    void refresh();
  }, [refresh]);

  async function run(action: () => Promise<unknown>) {
    setBusy(true);
    setError("");
    try {
      await action();
      await refresh();
    } catch (e) {
      setError(e instanceof Error ? e.message : "Action failed");
      setBusy(false);
    }
  }

  const pending = rows.filter((r) => r.status === "pending");
  const unassigned = pending.filter((r) => !r.assigned_supplier_id);
  const toCollect = rows.filter((r) => r.issue?.status === "approved");
  const canApprove = can(user?.permissions, "parts.approve");

  return (
    <div>
      <PageHeader
        title="Part requests"
        subtitle="Assign suppliers, accept quotes, approve issues, collect parts"
        actions={
          <Button type="button" variant="secondary" disabled={busy} onClick={() => void refresh()}>
            Refresh
          </Button>
        }
      />
      {error ? <p className="form-error">{error}</p> : null}

      <StatStrip>
        <Stat icon={ICONS.parts} label="Loaded" value={rows.length} />
        <Stat icon={ICONS.suppliers} label="Needs supplier" value={unassigned.length} tone={unassigned.length ? "warn" : undefined} />
        <Stat icon={ICONS.clock} label="Awaiting quote" value={pending.length - unassigned.length} />
        <Stat icon={ICONS.ready} label="Ready to collect" value={toCollect.length} tone={toCollect.length ? "success" : undefined} />
      </StatStrip>

      <div className="pos-toolbar">
        <label>
          Show
          <select className="input" value={filter} onChange={(e) => setFilter(e.target.value as typeof filter)}>
            <option value="pending">Pending only</option>
            <option value="all">All statuses</option>
          </select>
        </label>
      </div>

      <section className="panel">
        {rows.length === 0 ? (
          <EmptyState title="No part requests" body="Nothing matches the current filter." />
        ) : (
          <ul className="part-list">
            {rows.map((pr) => {
              const assignedName = suppliers.find(
                (s) => s.id === (pr.assigned_supplier_id || supplierByRequest[pr.id]),
              )?.name;
              return (
              <li key={pr.id} className="part-card">
                <div className="part-head">
                  <div>
                    <strong>{pr.description}</strong>
                    <div className="muted">
                      Qty {pr.quantity} ·{" "}
                      <Link className="mono" to={`/repairs/${pr.repair_job_id}`}>
                        {pr.repair_job_id.slice(0, 8)}
                      </Link>
                      {assignedName ? ` · ${assignedName}` : ""}
                    </div>
                  </div>
                  <div className="chip-row">
                    <Badge tone={statusTone(pr.issue?.status || pr.status)}>
                      {pr.issue?.status || pr.status}
                    </Badge>
                  </div>
                </div>
                {pr.status === "pending" && canApprove ? (
                  <div className="assign-block">
                    <strong>Assign to supplier portal</strong>
                    <div className="inline-form">
                      <label>
                        Supplier
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
                      </label>
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
                  <div className="auth-block quote-block">
                    <strong>Supplier price</strong>
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
                  </div>
                ) : null}
                {pr.status === "pending" &&
                (pr.assigned_supplier_id || supplierByRequest[pr.id]) &&
                !(pr.quotes ?? []).some((q) => q.status === "pending" || q.status === "accepted") ? (
                  <p className="hint">Waiting for the supplier — their price is authorized automatically.</p>
                ) : null}
                {pr.issue?.status === "approved" ? (
                  <div className="auth-block">
                    <code className="auth-code">{pr.issue.auth_code}</code>
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
                  </div>
                ) : null}
              </li>
              );
            })}
          </ul>
        )}
      </section>
    </div>
  );
}
