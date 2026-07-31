import { useEffect, useState } from "react";
import { Avatar, Badge, Button, EmptyState, ICONS, PageHeader } from "../../components/ui";
import {
  approveCommission,
  listCommissions,
  markCommissionPaid,
  type CommissionEntry,
} from "../../lib/api";

export function CommissionsPage() {
  const [items, setItems] = useState<CommissionEntry[]>([]);
  const [status, setStatus] = useState("pending");
  const [error, setError] = useState("");
  const [busy, setBusy] = useState<string | null>(null);

  async function refresh(nextStatus = status) {
    try {
      const res = await listCommissions({ status: nextStatus || undefined });
      setItems(res.items ?? []);
    } catch (e) {
      setError(e instanceof Error ? e.message : "Failed to load");
    }
  }

  useEffect(() => {
    void refresh();
  }, [status]);

  async function onApprove(id: string) {
    setBusy(id);
    try {
      await approveCommission(id);
      await refresh();
    } catch (e) {
      setError(e instanceof Error ? e.message : "Approve failed");
    } finally {
      setBusy(null);
    }
  }

  async function onPaid(id: string) {
    setBusy(id);
    try {
      await markCommissionPaid(id);
      await refresh();
    } catch (e) {
      setError(e instanceof Error ? e.message : "Mark paid failed");
    } finally {
      setBusy(null);
    }
  }

  const totalCommission = items.reduce((sum, c) => sum + c.commission_amount, 0);
  const totalBase = items.reduce((sum, c) => sum + c.base_amount, 0);

  return (
    <div className="settings-page">
      <PageHeader
        title="Commissions"
        subtitle="Accruals from completed repairs"
        actions={
          <select
            className="input"
            style={{ width: 160 }}
            value={status}
            onChange={(e) => setStatus(e.target.value)}
            aria-label="Filter by status"
          >
            <option value="pending">pending</option>
            <option value="approved">approved</option>
            <option value="paid">paid</option>
            <option value="">all</option>
          </select>
        }
      />
      {error ? <p className="form-error">{error}</p> : null}

      <section className="board-pulse" aria-label="Commissions pulse">
        <div>
          <strong>{items.length}</strong>
          <span>{status || "All"} entries</span>
        </div>
        <div>
          <strong>KES {totalBase.toFixed(0)}</strong>
          <span>Base labor</span>
        </div>
        <div>
          <strong>KES {totalCommission.toFixed(0)}</strong>
          <span>Commission due</span>
        </div>
      </section>

      {items.length === 0 ? (
        <EmptyState
          title="No commissions"
          body="Enable commission on a technician, complete a repair with labor amount."
          icon={ICONS.cash}
        />
      ) : (
        <ul className="settings-roster">
          {items.map((c) => (
            <li key={c.id}>
              <div className="settings-roster-row">
                <span className="name-cell">
                  <Avatar name={c.technician_name || c.user_id} size={30} />
                  <span className="name-cell-text">
                    <strong>{c.technician_name || c.user_id.slice(0, 8)}</strong>
                    <span className="muted mono">{c.repair_job_id.slice(0, 8)}…</span>
                  </span>
                </span>
                <span>
                  <span className="muted">Base</span> {c.currency} {c.base_amount.toFixed(2)}
                  <br />
                  <strong>
                    {c.currency} {c.commission_amount.toFixed(2)}
                  </strong>
                </span>
                <Badge tone={c.status === "paid" ? "success" : c.status === "approved" ? "info" : "pending"}>
                  {c.status}
                </Badge>
                <span className="chip-row">
                  {c.status === "pending" ? (
                    <Button type="button" variant="secondary" disabled={busy === c.id} onClick={() => void onApprove(c.id)}>
                      Approve
                    </Button>
                  ) : null}
                  {c.status === "pending" || c.status === "approved" ? (
                    <Button type="button" disabled={busy === c.id} onClick={() => void onPaid(c.id)}>
                      Mark paid
                    </Button>
                  ) : null}
                </span>
              </div>
            </li>
          ))}
        </ul>
      )}
    </div>
  );
}
