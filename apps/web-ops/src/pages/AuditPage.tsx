import { useCallback, useEffect, useState, type FormEvent } from "react";
import { Badge, Button, EmptyState, ICONS, Input, PageHeader, SearchInput } from "../components/ui";
import { listAuditEvents, listErrorEvents, type AuditEvent, type ErrorEvent } from "../lib/api";

export function AuditPage() {
  const [tab, setTab] = useState<"activity" | "errors">("activity");
  const [q, setQ] = useState("");
  const [action, setAction] = useState("");
  const [entityType, setEntityType] = useState("");
  const [items, setItems] = useState<AuditEvent[]>([]);
  const [errorItems, setErrorItems] = useState<ErrorEvent[]>([]);
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

  const loadErrors = useCallback(async () => {
    setBusy(true);
    setError("");
    try {
      const res = await listErrorEvents(100);
      setErrorItems(res.items ?? []);
    } catch (e) {
      setError(e instanceof Error ? e.message : "Failed to load");
    } finally {
      setBusy(false);
    }
  }, []);

  useEffect(() => {
    if (tab === "activity") void search({});
    else void loadErrors();
  }, [tab, search, loadErrors]);

  function onSubmit(e: FormEvent) {
    e.preventDefault();
    void search({
      q: q.trim() || undefined,
      action: action.trim() || undefined,
      entity_type: entityType.trim() || undefined,
    });
  }

  return (
    <div className="audit-desk">
      <PageHeader
        title="Audit & errors"
        subtitle="Activity trail and server error feed — filter first, then read the ledger."
      />
      {error ? <p className="form-error">{error}</p> : null}

      <div className="lane-tabs" role="tablist" aria-label="Audit sections">
        <button type="button" role="tab" aria-selected={tab === "activity"} className={tab === "activity" ? "active" : ""} onClick={() => setTab("activity")}>
          Activity
        </button>
        <button type="button" role="tab" aria-selected={tab === "errors"} className={tab === "errors" ? "active" : ""} onClick={() => setTab("errors")}>
          Errors {errorItems.length > 0 ? <Badge tone="danger">{errorItems.length}</Badge> : null}
        </button>
      </div>

      {tab === "activity" ? (
        <>
          <form className="board-filters" onSubmit={onSubmit}>
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

          <section className="desk-ledger">
            <div className="panel-head">
              <h2>Activity ledger</h2>
              <span className="muted">{items.length} events</span>
            </div>
            {items.length === 0 ? (
              <div style={{ padding: "1rem" }}>
                <EmptyState title="No events" body="Adjust filters or wait for activity." icon={ICONS.audit} />
              </div>
            ) : (
              <table className="table">
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
                        {ev.entity_id ? <div className="muted tiny">{ev.entity_id.slice(0, 8)}</div> : null}
                      </td>
                      <td className="muted tiny">
                        {ev.reason ?? (ev.new_value ? JSON.stringify(ev.new_value).slice(0, 80) : "—")}
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            )}
          </section>
        </>
      ) : (
        <section className="desk-ledger">
          <div className="panel-head">
            <h2>Server errors</h2>
            <span className="muted">Newest first</span>
          </div>
          <p className="hint" style={{ padding: "0 1rem" }}>
            Unhandled server errors and 5xx responses. Each row includes a correlation ID you can grep in server logs.
          </p>
          {errorItems.length === 0 ? (
            <div style={{ padding: "1rem" }}>
              <EmptyState title="No errors recorded" body="Clean bill of health — nothing to see here." icon={ICONS.audit} />
            </div>
          ) : (
            <table className="table">
              <thead>
                <tr>
                  <th>When</th>
                  <th>Route</th>
                  <th>Status</th>
                  <th>Message</th>
                  <th>Correlation ID</th>
                </tr>
              </thead>
              <tbody>
                {errorItems.map((ev) => (
                  <tr key={ev.id}>
                    <td>{new Date(ev.created_at).toLocaleString()}</td>
                    <td className="mono">{ev.route}</td>
                    <td>
                      <Badge tone="danger">{ev.status}</Badge>
                    </td>
                    <td className="muted tiny">{ev.message}</td>
                    <td className="mono tiny">{ev.correlation_id || "—"}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          )}
        </section>
      )}
    </div>
  );
}
