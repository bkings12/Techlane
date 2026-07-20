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

  return (
    <div>
      <PageHeader
        title="Commissions"
        subtitle="Accruals from completed repairs"
        actions={
          <select className="input" style={{ width: 160 }} value={status} onChange={(e) => setStatus(e.target.value)}>
            <option value="pending">pending</option>
            <option value="approved">approved</option>
            <option value="paid">paid</option>
            <option value="">all</option>
          </select>
        }
      />
      {error ? <p className="form-error">{error}</p> : null}
      <section className="panel">
        {items.length === 0 ? (
          <EmptyState
            title="No commissions"
            body="Enable commission on a technician, complete a repair with labor amount."
            icon={ICONS.cash}
          />
        ) : (
          <table className="table">
            <thead>
              <tr>
                <th>Technician</th>
                <th>Base</th>
                <th>Commission</th>
                <th>Status</th>
                <th />
              </tr>
            </thead>
            <tbody>
              {items.map((c) => (
                <tr key={c.id}>
                  <td>
                    <span className="name-cell">
                      <Avatar name={c.technician_name || c.user_id} size={30} />
                      <span className="name-cell-text">
                        <strong>{c.technician_name || c.user_id.slice(0, 8)}</strong>
                        <span className="muted mono">{c.repair_job_id.slice(0, 8)}…</span>
                      </span>
                    </span>
                  </td>
                  <td>
                    {c.currency} {c.base_amount.toFixed(2)}
                  </td>
                  <td>
                    {c.currency} {c.commission_amount.toFixed(2)}
                  </td>
                  <td>
                    <Badge tone={c.status === "paid" ? "success" : c.status === "approved" ? "info" : "pending"}>
                      {c.status}
                    </Badge>
                  </td>
                  <td className="chip-row">
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
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        )}
      </section>
    </div>
  );
}
