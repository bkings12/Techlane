import { useCallback, useEffect, useMemo, useState } from "react";
import { Badge, Button, EmptyState, PageHeader } from "../components/ui";
import {
  listSyncCommands,
  resolveSyncCommand,
  type SyncCommand,
} from "../lib/api";

const COMMAND_TYPES = [
  "",
  "repair.create_draft",
  "repair.add_note",
  "repair.add_attachment",
  "parts.request",
  "payments.cash_provisional",
];

function statusTone(status: string): "danger" | "warning" | "success" | "pending" | "info" {
  if (status === "conflict" || status === "failed") return "danger";
  if (status === "processing") return "pending";
  if (status === "applied") return "success";
  if (status === "discarded") return "info";
  return "warning";
}

export function SyncPage() {
  const [filter, setFilter] = useState("needs_attention");
  const [commandType, setCommandType] = useState("");
  const [items, setItems] = useState<SyncCommand[]>([]);
  const [error, setError] = useState("");
  const [busy, setBusy] = useState<string | null>(null);

  const refresh = useCallback(async () => {
    const r = await listSyncCommands(filter || undefined, commandType || undefined);
    setItems(r.items ?? []);
  }, [filter, commandType]);

  useEffect(() => {
    refresh().catch((e) => setError(e instanceof Error ? e.message : "Failed to load"));
  }, [refresh]);

  const filtered = useMemo(() => {
    if (!commandType) return items;
    return items.filter((c) => c.command_type === commandType);
  }, [items, commandType]);

  async function act(id: string, resolution: "discard" | "retry") {
    setBusy(id);
    setError("");
    try {
      await resolveSyncCommand(id, resolution);
      await refresh();
    } catch (e) {
      setError(e instanceof Error ? e.message : "Resolve failed");
    } finally {
      setBusy(null);
    }
  }

  const attention = filtered.filter((c) => c.sync_status === "failed" || c.sync_status === "conflict").length;

  return (
    <div className="sync-desk">
      <PageHeader
        title="Sync & conflicts"
        subtitle="Offline outbox commands that failed or collided — discard or retry."
      />
      {error ? <p className="form-error">{error}</p> : null}

      <section className="board-pulse" aria-label="Sync pulse">
        <button type="button" className={filter === "needs_attention" ? "active" : ""} onClick={() => setFilter("needs_attention")}>
          <strong>{filter === "needs_attention" ? filtered.length : attention}</strong>
          <span>Needs attention</span>
        </button>
        <button type="button" className={filter === "failed" ? "active" : ""} onClick={() => setFilter("failed")}>
          <strong>{filter === "failed" ? filtered.length : "—"}</strong>
          <span>Failed</span>
        </button>
        <button type="button" className={filter === "conflict" ? "active" : ""} onClick={() => setFilter("conflict")}>
          <strong>{filter === "conflict" ? filtered.length : "—"}</strong>
          <span>Conflict</span>
        </button>
        <div>
          <strong>{filtered.length}</strong>
          <span>Showing</span>
        </div>
        <button type="button" className={filter === "" ? "active" : ""} onClick={() => setFilter("")}>
          <strong>All</strong>
          <span>Statuses</span>
        </button>
      </section>

      <div className="board-filters">
        <label>
          Command type
          <select className="input" value={commandType} onChange={(e) => setCommandType(e.target.value)}>
            <option value="">All types</option>
            {COMMAND_TYPES.filter(Boolean).map((t) => (
              <option key={t} value={t}>
                {t}
              </option>
            ))}
          </select>
        </label>
      </div>

      <section className="panel" style={{ padding: "0.85rem" }}>
        <div className="panel-head">
          <h2>Command queue</h2>
        </div>
        {filtered.length === 0 ? (
          <EmptyState
            title="Queue clear"
            body="Failed drafts and idempotency payload mismatches appear here for discard or retry."
          />
        ) : (
          <ul className="sync-board">
            {filtered.map((c) => {
              const actionable = c.sync_status === "failed" || c.sync_status === "conflict";
              return (
                <li key={c.action_id} className="sync-row">
                  <div>
                    <Badge tone={statusTone(c.sync_status)}>{c.sync_status}</Badge>
                    <div style={{ marginTop: "0.45rem" }}>
                      <strong>{c.command_type}</strong>
                    </div>
                    <p className="muted">
                      <code>{c.action_id.slice(0, 8)}…</code>
                      {c.retry_count > 0 ? ` · retries ${c.retry_count}` : ""}
                      {" · "}
                      {new Date(c.updated_at).toLocaleString()}
                    </p>
                    {c.last_error ? <p className="hint">{c.last_error}</p> : null}
                    {c.payload && Object.keys(c.payload).length > 0 ? (
                      <pre className="payload-preview">{JSON.stringify(c.payload, null, 2)}</pre>
                    ) : null}
                  </div>
                  {actionable ? (
                    <div className="btn-row">
                      <Button type="button" variant="ghost" disabled={busy === c.action_id} onClick={() => void act(c.action_id, "discard")}>
                        Discard
                      </Button>
                      <Button type="button" variant="secondary" disabled={busy === c.action_id} onClick={() => void act(c.action_id, "retry")}>
                        Retry
                      </Button>
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
