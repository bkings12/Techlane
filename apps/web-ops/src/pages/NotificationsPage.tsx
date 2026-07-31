import { useCallback, useEffect, useState } from "react";
import { Badge, Button, EmptyState, ICONS, PageHeader } from "../components/ui";
import { ackNotification, listNotifications, type StaffNotification } from "../lib/api";

export function NotificationsPage() {
  const [items, setItems] = useState<StaffNotification[]>([]);
  const [unackedOnly, setUnackedOnly] = useState(true);
  const [error, setError] = useState("");
  const [busy, setBusy] = useState(false);

  const load = useCallback(async () => {
    setBusy(true);
    setError("");
    try {
      const res = await listNotifications(unackedOnly);
      setItems(res.items ?? []);
    } catch (e) {
      setError(e instanceof Error ? e.message : "Failed to load");
    } finally {
      setBusy(false);
    }
  }, [unackedOnly]);

  useEffect(() => {
    void load();
  }, [load]);

  async function onAck(id: string) {
    try {
      await ackNotification(id);
      await load();
    } catch (e) {
      setError(e instanceof Error ? e.message : "Ack failed");
    }
  }

  const unacked = items.filter((n) => !n.acked_at).length;

  return (
    <div className="notify-inbox">
      <PageHeader
        title="Notifications"
        subtitle="Staff inbox — unacked first, then acknowledge and clear."
        actions={
          <Button type="button" onClick={() => void load()} disabled={busy}>
            Refresh
          </Button>
        }
      />
      {error ? <p className="form-error">{error}</p> : null}

      <section className="board-pulse" aria-label="Inbox pulse">
        <button type="button" className={unackedOnly ? "active" : ""} onClick={() => setUnackedOnly(true)}>
          <strong>{unackedOnly ? items.length : unacked}</strong>
          <span>Unacked</span>
        </button>
        <button type="button" className={!unackedOnly ? "active" : ""} onClick={() => setUnackedOnly(false)}>
          <strong>{!unackedOnly ? items.length : "All"}</strong>
          <span>Full inbox</span>
        </button>
        <div>
          <strong>{items.length}</strong>
          <span>Showing</span>
        </div>
      </section>

      <section className="panel" style={{ padding: "0.85rem" }}>
        <div className="panel-head">
          <h2>Inbox</h2>
          <label className="checkbox-row">
            <input type="checkbox" checked={unackedOnly} onChange={(e) => setUnackedOnly(e.target.checked)} />
            Unacked only
          </label>
        </div>
        {items.length === 0 ? (
          <EmptyState title="Inbox clear" body="No notifications to show." icon={ICONS.risk} />
        ) : (
          <ul className="inbox-board">
            {items.map((n) => (
              <li key={n.id} className={`inbox-row ${!n.acked_at ? "is-new" : ""}`}>
                <div>
                  <div className="muted">{new Date(n.created_at).toLocaleString()}</div>
                  <strong>
                    {n.title} {!n.acked_at ? <Badge tone="warning">new</Badge> : null}
                  </strong>
                  <p className="muted" style={{ margin: "0.25rem 0 0" }}>
                    {n.body}
                  </p>
                </div>
                {!n.acked_at ? (
                  <Button type="button" onClick={() => void onAck(n.id)}>
                    Ack
                  </Button>
                ) : (
                  <span className="muted">acked</span>
                )}
              </li>
            ))}
          </ul>
        )}
      </section>
    </div>
  );
}
