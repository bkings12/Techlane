import { useCallback, useEffect, useState, type FormEvent } from "react";
import { Badge, Button, EmptyState, ICONS, Input, PageHeader, Stat, StatStrip } from "../components/ui";
import {
  collectOnlineOrder,
  confirmOnlineOrderPaid,
  expireOnlineHolds,
  listOnlineCatalog,
  listOnlineOrders,
  listStockLocations,
  placeOnlineCheckout,
  type CatalogItem,
  type OnlineOrder,
  type StockLocation,
} from "../lib/api";
import { useAuth } from "../auth/AuthContext";

export function OrdersPage() {
  const { user } = useAuth();
  const [orders, setOrders] = useState<OnlineOrder[]>([]);
  const [catalog, setCatalog] = useState<CatalogItem[]>([]);
  const [locations, setLocations] = useState<StockLocation[]>([]);
  const [locationId, setLocationId] = useState("");
  const [cart, setCart] = useState<Record<string, number>>({});
  const [code, setCode] = useState("");
  const [filter, setFilter] = useState("ready_for_pickup");
  const [error, setError] = useState("");
  const [message, setMessage] = useState("");
  const [busy, setBusy] = useState(false);

  const refresh = useCallback(async () => {
    const [o, locs] = await Promise.all([
      listOnlineOrders(filter || undefined),
      listStockLocations(user?.branch_ids?.[0]),
    ]);
    setOrders(o.items ?? []);
    setLocations(locs.items ?? []);
    if (!locationId && (locs.items?.length ?? 0) > 0) {
      setLocationId(locs.items![0].id);
    }
  }, [filter, locationId, user?.branch_ids]);

  useEffect(() => {
    refresh().catch((e) => setError(e instanceof Error ? e.message : "Failed to load"));
  }, [refresh]);

  useEffect(() => {
    if (!locationId) return;
    listOnlineCatalog(locationId)
      .then((c) => setCatalog(c.items ?? []))
      .catch(() => setCatalog([]));
  }, [locationId]);

  async function collect(e: FormEvent) {
    e.preventDefault();
    setBusy(true);
    setError("");
    setMessage("");
    try {
      const order = await collectOnlineOrder(code.trim().toUpperCase());
      setMessage(`Collected order ${order.id.slice(0, 8)}…`);
      setCode("");
      await refresh();
    } catch (err) {
      setError(err instanceof Error ? err.message : "Collect failed");
    } finally {
      setBusy(false);
    }
  }

  async function demoCheckout() {
    const items = Object.entries(cart)
      .filter(([, q]) => q > 0)
      .map(([variant_id, quantity]) => ({ variant_id, quantity }));
    if (items.length === 0) {
      setError("Add at least one online catalog item");
      return;
    }
    const branchId = user?.branch_ids?.[0];
    if (!branchId || !locationId) {
      setError("Branch and location required");
      return;
    }
    setBusy(true);
    setError("");
    setMessage("");
    try {
      const result = await placeOnlineCheckout({
        branch_id: branchId,
        location_id: locationId,
        method: "mpesa_c2b",
        fulfilment_type: "branch_pickup",
        items,
      });
      const ref = result.payment?.account_reference;
      setMessage(
        `Order pending payment${ref ? ` · paybill ref ${ref}` : ""}. Confirm after C2B webhook or use Confirm paid.`,
      );
      setCart({});
      await refresh();
    } catch (err) {
      setError(err instanceof Error ? err.message : "Checkout failed");
    } finally {
      setBusy(false);
    }
  }

  async function confirmPaid(id: string) {
    setBusy(true);
    setError("");
    try {
      await confirmOnlineOrderPaid(id);
      setMessage("Marked paid — ready for pickup");
      await refresh();
    } catch (err) {
      setError(err instanceof Error ? err.message : "Confirm failed");
    } finally {
      setBusy(false);
    }
  }

  const ready = orders.filter((o) => o.status === "ready_for_pickup");

  return (
    <div>
      <PageHeader
        title="Online orders"
        subtitle="Buy online, collect in branch — reservations hold stock until paid"
      />
      {error ? <p className="form-error">{error}</p> : null}
      {message ? <p className="hint">{message}</p> : null}

      <StatStrip>
        <Stat icon={ICONS.ready} label="Ready for pickup" value={ready.length} tone={ready.length ? "success" : undefined} />
        <Stat icon={ICONS.orders} label="Listed" value={orders.length} />
        <Stat icon={ICONS.inventory} label="Online SKUs" value={catalog.length} />
      </StatStrip>

      <div className="repair-grid">
        <section className="panel">
          <h2>Collect in branch</h2>
          <p className="hint">
            Customer presents the collection code shown after payment. Unpaid holds expire after ~15 minutes and stock
            returns to available.
          </p>
          <form className="form-grid" onSubmit={collect}>
            <label>
              Collection code
              <Input
                value={code}
                onChange={(e) => setCode(e.target.value.toUpperCase())}
                placeholder="ABC123"
                required
              />
            </label>
            <Button type="submit" disabled={busy || code.trim().length < 4}>
              Mark collected
            </Button>
          </form>

          <div className="panel-head" style={{ marginTop: "1.5rem" }}>
            <h2>Queue</h2>
            <select className="input" value={filter} onChange={(e) => setFilter(e.target.value)}>
              <option value="ready_for_pickup">Ready for pickup</option>
              <option value="pending_payment">Pending payment</option>
              <option value="expired">Expired holds</option>
              <option value="delivered">Delivered</option>
              <option value="">All</option>
            </select>
          </div>
          {orders.length === 0 ? (
            <EmptyState title="No orders" body="Place a demo checkout or wait for storefront traffic." />
          ) : (
            <ul className="part-list">
              {orders.map((o) => (
                <li key={o.id} className="part-card">
                  <div className="part-head">
                    <div>
                      <strong>KES {o.total.toLocaleString()}</strong>
                      <div className="muted mono">{o.id.slice(0, 8)}…</div>
                      {o.collection_code ? (
                        <div className="hint">
                          Code <span className="mono">{o.collection_code}</span>
                        </div>
                      ) : (
                        <div className="hint">Code hidden until paid</div>
                      )}
                    </div>
                    <Badge tone={o.status === "ready_for_pickup" ? "success" : o.status === "delivered" ? "info" : "pending"}>
                      {o.status.replaceAll("_", " ")}
                    </Badge>
                  </div>
                  {o.status === "pending_payment" ? (
                    <Button type="button" disabled={busy} onClick={() => void confirmPaid(o.id)} style={{ marginTop: "0.75rem" }}>
                      Confirm paid (sandbox)
                    </Button>
                  ) : null}
                </li>
              ))}
            </ul>
          )}
        </section>

        <aside className="stack">
          <section className="panel">
            <h2>Demo checkout</h2>
            <p className="hint">Ops-side cart against the online catalog. Reserves stock, creates C2B payment.</p>
            <label>
              Fulfillment location
              <select className="input" value={locationId} onChange={(e) => setLocationId(e.target.value)}>
                {locations.map((l) => (
                  <option key={l.id} value={l.id}>
                    {l.name}
                  </option>
                ))}
              </select>
            </label>
            {catalog.length === 0 ? (
              <EmptyState title="No online products" body="Publish products or restart API to seed online_visible accessories." />
            ) : (
              <ul className="list" style={{ marginTop: "0.75rem" }}>
                {catalog.slice(0, 8).map((it) => (
                  <li key={it.variant_id}>
                    <span>
                      {it.product_name} · KES {it.sell_price.toLocaleString()}
                      <span className="muted"> · {it.available_qty} avail</span>
                    </span>
                    <Button
                      type="button"
                      disabled={it.available_qty <= 0 || busy}
                      onClick={() => setCart((prev) => ({ ...prev, [it.variant_id]: (prev[it.variant_id] || 0) + 1 }))}
                    >
                      +{cart[it.variant_id] ? ` ${cart[it.variant_id]}` : ""}
                    </Button>
                  </li>
                ))}
              </ul>
            )}
            <Button type="button" disabled={busy} onClick={() => void demoCheckout()} style={{ marginTop: "1rem" }}>
              Place C2B order
            </Button>
            <Button
              type="button"
              disabled={busy}
              onClick={() => {
                setBusy(true);
                setError("");
                expireOnlineHolds()
                  .then(async (r) => {
                    setMessage(`Released ${r.released} expired hold(s)`);
                    await refresh();
                  })
                  .catch((err) => setError(err instanceof Error ? err.message : "Expire failed"))
                  .finally(() => setBusy(false));
              }}
              style={{ marginTop: "0.5rem" }}
            >
              Release expired holds
            </Button>
          </section>
        </aside>
      </div>
    </div>
  );
}
