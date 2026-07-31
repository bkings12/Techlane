import { useCallback, useEffect, useState, type FormEvent } from "react";
import { Badge, Button, EmptyState, ICONS, Input, PageHeader } from "../components/ui";
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
        customer_name: "Demo customer",
        phone: "0700000000",
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
  const cartCount = Object.values(cart).reduce((sum, q) => sum + q, 0);

  return (
    <div className="orders-counter">
      <PageHeader
        title="Online orders"
        subtitle="Collect at the counter first — queue and demo checkout stay beside it."
      />
      {error ? <p className="form-error">{error}</p> : null}
      {message ? <p className="hint">{message}</p> : null}

      <section className="board-pulse" aria-label="Orders pulse">
        <button type="button" className={filter === "ready_for_pickup" ? "active" : ""} onClick={() => setFilter("ready_for_pickup")}>
          <strong>{ready.length}</strong>
          <span>Ready</span>
        </button>
        <button type="button" className={filter === "pending_payment" ? "active" : ""} onClick={() => setFilter("pending_payment")}>
          <strong>{filter === "pending_payment" ? orders.length : "—"}</strong>
          <span>Pending pay</span>
        </button>
        <div>
          <strong>{orders.length}</strong>
          <span>Listed</span>
        </div>
        <div>
          <strong>{catalog.length}</strong>
          <span>Online SKUs</span>
        </div>
        <button type="button" className={filter === "" ? "active" : ""} onClick={() => setFilter("")}>
          <strong>All</strong>
          <span>Statuses</span>
        </button>
      </section>

      <div className="orders-layout">
        <div className="stack">
          <section className="panel collect-panel">
            <h2>Collect in branch</h2>
            <p className="hint">
              Customer presents the collection code after payment. Unpaid holds expire after ~15 minutes.
            </p>
            <form className="form-grid" onSubmit={collect}>
              <label>
                Collection code
                <Input
                  className="auth-code"
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
          </section>

          <section className="panel" style={{ padding: "0.85rem" }}>
            <div className="panel-head">
              <h2>Pickup queue</h2>
              <select className="input" value={filter} onChange={(e) => setFilter(e.target.value)} aria-label="Filter queue by status">
                <option value="ready_for_pickup">Ready for pickup</option>
                <option value="confirmed">Confirmed (delivery)</option>
                <option value="pending_payment">Pending payment</option>
                <option value="expired">Expired holds</option>
                <option value="delivered">Delivered / completed</option>
                <option value="">All</option>
              </select>
            </div>
            {orders.length === 0 ? (
              <EmptyState title="No orders" body="Place a demo checkout or wait for storefront traffic." icon={ICONS.orders} />
            ) : (
              <ul className="order-queue">
                {orders.map((o) => (
                  <li key={o.id} className={`order-row ${o.status === "ready_for_pickup" ? "is-ready" : ""}`}>
                    <div>
                      <strong>KES {o.total.toLocaleString()}</strong>
                      <div className="muted mono">{o.id.slice(0, 8)}…</div>
                      {o.guest_name ? (
                        <div className="hint">
                          {o.guest_name}
                          {o.guest_phone ? ` · ${o.guest_phone}` : ""}
                        </div>
                      ) : null}
                      <div className="hint">
                        {o.fulfilment_type === "delivery"
                          ? `Delivery${o.delivery_location_name ? ` · ${o.delivery_location_name}` : ""}${
                              o.delivery_fee ? ` · fee KES ${o.delivery_fee.toLocaleString()}` : ""
                            }`
                          : "Branch pickup"}
                      </div>
                      {o.fulfilment_type === "delivery" && (o.delivery_address_line1 || o.delivery_landmark) ? (
                        <div className="muted tiny">
                          {[o.delivery_address_line1, o.delivery_address_line2, o.delivery_landmark]
                            .filter(Boolean)
                            .join(", ")}
                        </div>
                      ) : null}
                      {o.collection_code ? (
                        <div className="hint">
                          Code <span className="mono">{o.collection_code}</span>
                        </div>
                      ) : (
                        <div className="hint">Code hidden until paid</div>
                      )}
                      {o.status === "pending_payment" ? (
                        <Button type="button" disabled={busy} onClick={() => void confirmPaid(o.id)} style={{ marginTop: "0.55rem" }}>
                          Confirm paid / cash collected
                        </Button>
                      ) : null}
                    </div>
                    <Badge tone={o.status === "ready_for_pickup" ? "success" : o.status === "delivered" ? "info" : "pending"}>
                      {o.status.replaceAll("_", " ")}
                    </Badge>
                  </li>
                ))}
              </ul>
            )}
          </section>
        </div>

        <aside className="cart-rail">
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
          <div className="pos-total">
            <span className="muted">In cart</span>
            <strong>{cartCount}</strong>
          </div>
          <Button type="button" disabled={busy || cartCount === 0} onClick={() => void demoCheckout()}>
            Place C2B order
          </Button>
          <Button
            type="button"
            variant="secondary"
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
        </aside>
      </div>
    </div>
  );
}
