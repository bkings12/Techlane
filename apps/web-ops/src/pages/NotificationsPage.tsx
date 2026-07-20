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

  return (
    <div>
      <PageHeader title="Notifications" subtitle="Staff inbox from repair and payment events" />
      {error ? <p className="form-error">{error}</p> : null}
      <div className="pos-toolbar">
        <label>
          <input
            type="checkbox"
            checked={unackedOnly}
            onChange={(e) => setUnackedOnly(e.target.checked)}
          />{" "}
          Unacked only
        </label>
        <Button type="button" onClick={() => void load()} disabled={busy}>
          Refresh
        </Button>
      </div>
      {items.length === 0 ? (
        <EmptyState title="Inbox clear" body="No notifications to show." icon={ICONS.risk} />
      ) : (
        <table className="table">
          <thead>
            <tr>
              <th>When</th>
              <th>Title</th>
              <th>Body</th>
              <th />
            </tr>
          </thead>
          <tbody>
            {items.map((n) => (
              <tr key={n.id}>
                <td>{new Date(n.created_at).toLocaleString()}</td>
                <td>
                  {n.title}{" "}
                  {!n.acked_at ? <Badge tone="warning">new</Badge> : null}
                </td>
                <td>{n.body}</td>
                <td>
                  {!n.acked_at ? (
                    <Button type="button" onClick={() => void onAck(n.id)}>
                      Ack
                    </Button>
                  ) : (
                    "—"
                  )}
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      )}
    </div>
  );
}
