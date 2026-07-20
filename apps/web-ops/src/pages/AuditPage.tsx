import { useCallback, useEffect, useState, type FormEvent } from "react";
import { Badge, Button, EmptyState, ICONS, Input, PageHeader, SearchInput } from "../components/ui";
import { listAuditEvents, type AuditEvent } from "../lib/api";

export function AuditPage() {
  const [q, setQ] = useState("");
  const [action, setAction] = useState("");
  const [entityType, setEntityType] = useState("");
  const [items, setItems] = useState<AuditEvent[]>([]);
  const [error, setError] = useState("");
  const [busy, setBusy] = useState(false);

  const search = useCallback(async (params: { q?: string; action?: string; entity_type?: string }) => {
    setBusy(true);
    setError("");
    try {
      const res = await listAuditEvents({ ...params, limit: 100 });
      setItems(res.items ?? []);
    } catch (e) {
      setError(e instanceof Error ? e.message : "Failed to load");
    } finally {
      setBusy(false);
    }
  }, []);

  useEffect(() => {
    void search({});
  }, [search]);

  function onSubmit(e: FormEvent) {
    e.preventDefault();
    void search({
      q: q.trim() || undefined,
      action: action.trim() || undefined,
      entity_type: entityType.trim() || undefined,
    });
  }

  return (
    <div>
      <PageHeader title="Audit log" subtitle="Searchable activity trail" />
      {error ? <p className="form-error">{error}</p> : null}

      <form className="pos-toolbar" onSubmit={onSubmit}>
        <label>
          Search
          <SearchInput value={q} onChange={(e) => setQ(e.target.value)} placeholder="Free text" />
        </label>
        <label>
          Action
          <Input value={action} onChange={(e) => setAction(e.target.value)} placeholder="e.g. repair.status" />
        </label>
        <label>
          Entity type
          <Input value={entityType} onChange={(e) => setEntityType(e.target.value)} placeholder="e.g. repair_job" />
        </label>
        <Button type="submit" disabled={busy}>
          {busy ? "Loading…" : "Search"}
        </Button>
      </form>

      <section className="panel">
        {items.length === 0 ? (
          <EmptyState title="No events" body="Adjust filters or wait for activity." icon={ICONS.audit} />
        ) : (
          <div className="table-wrap">
            <table className="data-table">
              <thead>
                <tr>
                  <th>When</th>
                  <th>Actor</th>
                  <th>Action</th>
                  <th>Entity</th>
                  <th>Details</th>
                </tr>
              </thead>
              <tbody>
                {items.map((ev) => (
                  <tr key={ev.id}>
                    <td>{new Date(ev.created_at).toLocaleString()}</td>
                    <td>{ev.actor_name ?? ev.actor_id?.slice(0, 8) ?? "—"}</td>
                    <td>
                      <Badge tone="info">{ev.action}</Badge>
                    </td>
                    <td>
                      <span className="mono">{ev.entity_type}</span>
                      {ev.entity_id ? (
                        <div className="muted tiny">{ev.entity_id.slice(0, 8)}</div>
                      ) : null}
                    </td>
                    <td className="muted tiny">
                      {ev.reason ?? (ev.new_value ? JSON.stringify(ev.new_value).slice(0, 80) : "—")}
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}
      </section>
    </div>
  );
}
