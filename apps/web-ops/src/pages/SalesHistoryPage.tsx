import { useCallback, useEffect, useState } from "react";
import { Link } from "react-router-dom";
import { useBranch } from "../branch/BranchContext";
import { Badge, Button, EmptyState, PageHeader } from "../components/ui";
import { listSales, listStockLocations, openSaleReceipt, reverseSale, type Sale } from "../lib/api";

function saleTone(status: string): "success" | "warning" | "danger" | "info" | "pending" {
  if (status === "completed") return "success";
  if (status === "reversed") return "danger";
  if (status === "pending" || status === "awaiting_payment") return "pending";
  return "info";
}

export function SalesHistoryPage() {
  const { branchId } = useBranch();
  const [sales, setSales] = useState<Sale[]>([]);
  const [locationId, setLocationId] = useState("");
  const [error, setError] = useState("");
  const [busy, setBusy] = useState(false);

  const refresh = useCallback(async () => {
    const [salesRes, locs] = await Promise.all([
      listSales({
        branch_id: branchId || undefined,
        limit: 25,
      }),
      listStockLocations(branchId || undefined),
    ]);
    setSales(salesRes.items ?? []);
    const loc = locs.items?.find((l) => l.location_type === "counter") || locs.items?.[0];
    setLocationId((prev) => {
      if (prev && locs.items?.some((l) => l.id === prev)) return prev;
      return loc?.id || "";
    });
  }, [branchId]);

  useEffect(() => {
    refresh().catch((e) => setError(e instanceof Error ? e.message : "Failed to load sales"));
  }, [refresh]);

  async function printReceipt(saleId: string) {
    try {
      await openSaleReceipt(saleId);
    } catch (err) {
      setError(err instanceof Error ? err.message : "Could not open the receipt");
    }
  }

  async function doReverse(saleId: string) {
    if (!locationId) return;
    setBusy(true);
    setError("");
    try {
      await reverseSale(saleId, locationId);
      await refresh();
    } catch (err) {
      setError(err instanceof Error ? err.message : "Reverse failed");
    } finally {
      setBusy(false);
    }
  }

  return (
    <div className="page">
      <PageHeader
        title="Sales"
        subtitle="Past transactions across this branch."
        actions={
          <>
            <Link className="btn btn-ghost" to="/pos">
              Open Sell
            </Link>
            <Button type="button" variant="ghost" disabled={busy} onClick={() => void refresh()}>
              Refresh
            </Button>
          </>
        }
      />

      {error ? <p className="form-error">{error}</p> : null}

      <section className="panel sales-strip">
        {sales.length === 0 ? (
          <div style={{ padding: "1rem" }}>
            <EmptyState title="No sales yet" body="Completed and pending POS sales for this branch appear here." />
          </div>
        ) : (
          <table className="table">
            <thead>
              <tr>
                <th>Sale</th>
                <th>When</th>
                <th>Total</th>
                <th>Status</th>
                <th />
              </tr>
            </thead>
            <tbody>
              {sales.map((s) => (
                <tr key={s.id}>
                  <td className="mono">{s.id.slice(0, 8)}…</td>
                  <td className="muted">{s.created_at ? new Date(s.created_at).toLocaleString() : "—"}</td>
                  <td className="mono">KES {s.total.toLocaleString()}</td>
                  <td>
                    <Badge tone={saleTone(s.status)}>{s.status}</Badge>
                  </td>
                  <td>
                    <div className="chip-row">
                      <Button type="button" variant="ghost" onClick={() => void printReceipt(s.id)}>
                        Receipt
                      </Button>
                      {s.status === "completed" ? (
                        <Button
                          type="button"
                          variant="secondary"
                          disabled={busy || !locationId}
                          onClick={() => void doReverse(s.id)}
                        >
                          Reverse
                        </Button>
                      ) : null}
                    </div>
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
