import { useEffect, useState, type FormEvent } from "react";
import { Link, useNavigate, useParams, useSearchParams } from "react-router-dom";
import { useStorefront } from "../store/StorefrontContext";
import {
  getOrder,
  getPublicRepairStatus,
  isOrderFailed,
  isOrderPaid,
  orderStatusMessage,
  trackLookup,
  type Order,
  type PublicRepairStatus,
  type TrackOrderHit,
  type TrackRepairHit,
} from "../lib/api";
import { loadRecentOrderIds, rememberOrderId } from "../lib/storage";

function deviceLabel(device?: { kind?: string; brand?: string; model?: string }) {
  if (!device) return "Device";
  const parts = [device.brand, device.model].filter(Boolean);
  if (parts.length) return parts.join(" ");
  return device.kind || "Device";
}

function statusLabel(status: string) {
  return status.replace(/_/g, " ");
}

export function LookupPage() {
  const { orderId: routeQuery } = useParams();
  const [searchParams] = useSearchParams();
  const navigate = useNavigate();
  const { session, accountOrders, accountRepairs } = useStorefront();
  const initialQuery = routeQuery || searchParams.get("q") || "";
  const [query, setQuery] = useState(initialQuery);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");
  const [orders, setOrders] = useState<TrackOrderHit[]>([]);
  const [repairs, setRepairs] = useState<TrackRepairHit[]>([]);
  const [selectedOrder, setSelectedOrder] = useState<Order | null>(null);
  const [selectedRepair, setSelectedRepair] = useState<PublicRepairStatus | null>(null);
  const [recentIds, setRecentIds] = useState<string[]>(() => loadRecentOrderIds());

  async function runTrack(raw: string, navigateSingleOrder: boolean) {
    const trimmed = raw.trim();
    if (!trimmed) {
      setError("Enter an order number, job code, or phone number");
      return;
    }
    setBusy(true);
    setError("");
    setSelectedOrder(null);
    setSelectedRepair(null);
    try {
      const result = await trackLookup(trimmed);
      setOrders(result.orders ?? []);
      setRepairs(result.repairs ?? []);
      const orderHits = result.orders ?? [];
      const repairHits = result.repairs ?? [];

      if (orderHits.length === 0 && repairHits.length === 0) {
        setError("No orders or repairs matched that search");
        return;
      }

      if (orderHits.length === 1 && repairHits.length === 0) {
        const o = await getOrder(orderHits[0]!.id);
        setSelectedOrder(o);
        rememberOrderId(o.id);
        setRecentIds(loadRecentOrderIds());
        if (navigateSingleOrder) {
          if (isOrderPaid(o)) navigate(`/done/${o.id}`);
          else if (!isOrderFailed(o)) navigate(`/pay/${o.id}`);
        }
        return;
      }

      if (repairHits.length === 1 && orderHits.length === 0) {
        const detail = await getPublicRepairStatus(repairHits[0]!.job_code);
        setSelectedRepair(detail);
      }
    } catch (e) {
      setError(e instanceof Error ? e.message : "Lookup failed");
      setOrders([]);
      setRepairs([]);
    } finally {
      setBusy(false);
    }
  }

  useEffect(() => {
    if (initialQuery) {
      setQuery(initialQuery);
      void runTrack(initialQuery, Boolean(routeQuery));
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [initialQuery, routeQuery]);

  function onSubmit(e: FormEvent) {
    e.preventDefault();
    const trimmed = query.trim();
    if (!trimmed) return;
    navigate(`/lookup?q=${encodeURIComponent(trimmed)}`, { replace: true });
    void runTrack(trimmed, true);
  }

  return (
    <section className="wide-page">
      <div className="page-header">
        <h1>Track</h1>
        <ol className="breadcrumb">
          <li>
            <Link to="/">Home</Link>
          </li>
          <li className="active">Track</li>
        </ol>
      </div>

      <div className="li-container page-body">
        <p className="muted" style={{ marginTop: 0 }}>
          Track online orders and repair jobs with an order number, job code (e.g. JOB-110), or phone number.
        </p>

        <div className="lookup-layout">
          <div className="content-card">
            <form className="lookup-form" onSubmit={onSubmit}>
              <label className="field">
                Order #, job code, or phone
                <input
                  value={query}
                  onChange={(e) => setQuery(e.target.value)}
                  placeholder="ORD-946851B7 · JOB-110 · 07…"
                  autoComplete="off"
                  inputMode="text"
                />
              </label>
              <button type="submit" className="btn" disabled={busy}>
                {busy ? "Looking up…" : "Track"}
              </button>
            </form>
            {error ? <p className="error">{error}</p> : null}

            {selectedOrder ? (
              <div className="status-box">
                <p>
                  <strong>Order</strong> {selectedOrder.id.slice(0, 8).toUpperCase()}…
                </p>
                <p>{orderStatusMessage(selectedOrder)}</p>
                <p className="muted">Total KES {selectedOrder.total.toLocaleString()}</p>
                {!isOrderFailed(selectedOrder) && !isOrderPaid(selectedOrder) ? (
                  <button type="button" className="btn" onClick={() => navigate(`/pay/${selectedOrder.id}`)}>
                    Open payment screen
                  </button>
                ) : isOrderPaid(selectedOrder) ? (
                  <button type="button" className="btn" onClick={() => navigate(`/done/${selectedOrder.id}`)}>
                    View order
                  </button>
                ) : (
                  <button type="button" className="btn" onClick={() => navigate("/shop")}>
                    Shop again
                  </button>
                )}
              </div>
            ) : null}

            {selectedRepair ? (
              <div className="status-box">
                <p>
                  <strong>{selectedRepair.job_code}</strong> · {statusLabel(selectedRepair.status)}
                </p>
                <p>{deviceLabel(selectedRepair.device)}</p>
                {selectedRepair.problem_summary ? <p className="muted">{selectedRepair.problem_summary}</p> : null}
                {(selectedRepair.timeline ?? []).length > 0 ? (
                  <ol className="track-timeline">
                    {(selectedRepair.timeline ?? []).map((ev, i) => (
                      <li key={`${ev.status}-${ev.at}-${i}`}>
                        <strong>{statusLabel(ev.status)}</strong>
                        {ev.at ? <time>{new Date(ev.at).toLocaleString()}</time> : null}
                        {ev.note ? <p>{ev.note}</p> : null}
                      </li>
                    ))}
                  </ol>
                ) : null}
              </div>
            ) : null}

            {!selectedOrder && !selectedRepair && (orders.length > 0 || repairs.length > 0) ? (
              <div className="track-results">
                {orders.length > 0 ? (
                  <div className="recent" style={{ marginTop: "1rem" }}>
                    <h2 style={{ marginTop: 0, fontSize: "1.1rem" }}>Orders</h2>
                    <ul>
                      {orders.map((o) => (
                        <li key={o.id}>
                          <button
                            type="button"
                            className="linkish"
                            onClick={() => void runTrack(o.id, true)}
                          >
                            {o.ref} — KES {o.total.toLocaleString()} — {statusLabel(o.status)}
                          </button>
                        </li>
                      ))}
                    </ul>
                  </div>
                ) : null}
                {repairs.length > 0 ? (
                  <div className="recent">
                    <h2 style={{ fontSize: "1.1rem" }}>Repairs</h2>
                    <ul>
                      {repairs.map((r) => (
                        <li key={r.id}>
                          <button
                            type="button"
                            className="linkish"
                            onClick={() => void runTrack(r.job_code, false)}
                          >
                            {r.job_code} — {deviceLabel(r.device)} — {statusLabel(r.status)}
                          </button>
                        </li>
                      ))}
                    </ul>
                  </div>
                ) : null}
              </div>
            ) : null}
          </div>

          <div className="content-card">
            {session ? (
              <>
                {accountOrders.length > 0 ? (
                  <div className="recent" style={{ marginTop: 0 }}>
                    <h2 style={{ marginTop: 0, fontSize: "1.1rem" }}>Your orders</h2>
                    <ul>
                      {accountOrders.map((o) => (
                        <li key={o.id}>
                          <button type="button" className="linkish" onClick={() => void runTrack(o.id, true)}>
                            {o.id.slice(0, 8).toUpperCase()}… — {statusLabel(o.status)}
                          </button>
                        </li>
                      ))}
                    </ul>
                  </div>
                ) : null}
                {accountRepairs.length > 0 ? (
                  <div className="recent">
                    <h2 style={{ marginTop: accountOrders.length ? undefined : 0, fontSize: "1.1rem" }}>Your repairs</h2>
                    <ul>
                      {accountRepairs.map((r) => (
                        <li key={r.id}>
                          <button type="button" className="linkish" onClick={() => void runTrack(r.job_code, false)}>
                            {r.job_code} — {statusLabel(r.status)}
                          </button>
                        </li>
                      ))}
                    </ul>
                  </div>
                ) : null}
                {accountOrders.length === 0 && accountRepairs.length === 0 ? (
                  <p className="muted" style={{ marginTop: 0 }}>
                    No account orders or repairs yet.
                  </p>
                ) : null}
              </>
            ) : (
              <p className="muted" style={{ marginTop: 0 }}>
                <Link to="/account">Sign in</Link> to see your order and repair history.
              </p>
            )}
            {recentIds.length > 0 ? (
              <div className="recent">
                <h2 style={{ fontSize: "1.1rem" }}>Recent on this device</h2>
                <ul>
                  {recentIds.map((id) => (
                    <li key={id}>
                      <button type="button" className="linkish" onClick={() => void runTrack(id, true)}>
                        {id.slice(0, 8).toUpperCase()}…
                      </button>
                    </li>
                  ))}
                </ul>
              </div>
            ) : null}
          </div>
        </div>
      </div>
    </section>
  );
}
