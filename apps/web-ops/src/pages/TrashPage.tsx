import { useEffect, useState } from "react";
import { useAuth } from "../auth/AuthContext";
import { Badge, Button, EmptyState, ICONS, PageHeader } from "../components/ui";
import { listTrashedRepairs, purgeRepair, restoreRepair, type RepairJob } from "../lib/api";
import { statusTone } from "../lib/repairStatus";

export function TrashPage() {
  const { user } = useAuth();
  const [items, setItems] = useState<RepairJob[]>([]);
  const [busy, setBusy] = useState("");
  const [error, setError] = useState("");
  const isOwner = user?.roles?.includes("owner") ?? false;

  async function refresh() {
    try {
      setError("");
      const result = await listTrashedRepairs();
      setItems(result.items ?? []);
    } catch (err) {
      setError(err instanceof Error ? err.message : "Could not load trashed jobs");
    }
  }

  useEffect(() => {
    void refresh();
  }, []);

  async function restore(job: RepairJob) {
    setBusy(job.id);
    try {
      await restoreRepair(job.id);
      await refresh();
    } catch (err) {
      setError(err instanceof Error ? err.message : "Could not restore job");
    } finally {
      setBusy("");
    }
  }

  async function purge(job: RepairJob) {
    const code = job.job_code ?? job.id.slice(0, 8);
    if (!window.confirm(`Permanently delete ${code}? This cannot be undone.`)) return;
    setBusy(job.id);
    try {
      await purgeRepair(job.id);
      await refresh();
    } catch (err) {
      setError(err instanceof Error ? err.message : "Could not permanently delete job");
    } finally {
      setBusy("");
    }
  }

  return (
    <div className="trash-desk">
      <PageHeader
        title="Trashed"
        subtitle="Recover intake mistakes and duplicate job cards before permanent removal."
        actions={<span className="trash-count">{items.length} archived</span>}
      />
      {error ? <p className="form-error">{error}</p> : null}

      <section className="board-pulse" aria-label="Trash pulse">
        <div>
          <strong>{items.length}</strong>
          <span>Archived</span>
        </div>
        <div>
          <strong>{isOwner ? "Yes" : "No"}</strong>
          <span>Can purge</span>
        </div>
      </section>

      <section className="panel" style={{ padding: "0.85rem" }}>
        <div className="panel-head">
          <h2>Recoverable jobs</h2>
        </div>
        {items.length === 0 ? (
          <EmptyState
            title="Trash is empty"
            body="Deleted repair jobs remain recoverable here. Active and completed records are never shown."
            icon={ICONS.inbox}
          />
        ) : (
          <ul className="trash-board">
            {items.map((job) => (
              <li key={job.id} className="trash-row">
                <div>
                  <strong className="mono">{job.job_code ?? job.id.slice(0, 8)}</strong>
                  <div className="muted">
                    {job.customer_name ?? "Walk-in"} · {job.problem_summary}
                  </div>
                  <div className="muted" style={{ marginTop: "0.35rem" }}>
                    Deleted {job.deleted_at ? new Date(job.deleted_at).toLocaleString() : "—"}
                  </div>
                  <div style={{ marginTop: "0.45rem" }}>
                    <Badge tone={statusTone(job.status)}>{job.status.replaceAll("_", " ")}</Badge>
                  </div>
                </div>
                <div className="trash-actions">
                  <Button type="button" variant="secondary" disabled={busy === job.id} onClick={() => void restore(job)}>
                    Restore
                  </Button>
                  {isOwner ? (
                    <Button type="button" variant="danger" disabled={busy === job.id} onClick={() => void purge(job)}>
                      Delete forever
                    </Button>
                  ) : null}
                </div>
              </li>
            ))}
          </ul>
        )}
      </section>
    </div>
  );
}
