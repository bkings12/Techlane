import { useCallback, useEffect, useMemo, useState, type FormEvent } from "react";
import { Link } from "react-router-dom";
import { useBranch } from "../branch/BranchContext";
import { Badge, Button, EmptyState, ICONS, Input, PageHeader } from "../components/ui";
import {
  completeSale,
  confirmMpesaPayment,
  getPayment,
  getPaymentSettings,
  listPOSCatalog,
  listSales,
  listStockLocations,
  posCheckout,
  reverseSale,
  type CatalogItem,
  type PaymentProviderSettings,
  type POSCheckoutResult,
  type Sale,
  type StockLocation,
} from "../lib/api";

type CartLine = { item: CatalogItem; qty: number };

function saleTone(status: string): "success" | "warning" | "danger" | "info" | "pending" {
  if (status === "completed") return "success";
  if (status === "reversed") return "danger";
  if (status === "pending" || status === "awaiting_payment") return "pending";
  return "info";
}

export function POSPage() {
  const { branchId, setBranchId, branches } = useBranch();
  const [catalog, setCatalog] = useState<CatalogItem[]>([]);
  const [locations, setLocations] = useState<StockLocation[]>([]);
  const [locationId, setLocationId] = useState("");
  const [cart, setCart] = useState<CartLine[]>([]);
  const [method, setMethod] = useState("cash");
  const [phone, setPhone] = useState("");
  const [cfg, setCfg] = useState<PaymentProviderSettings | null>(null);
  const [error, setError] = useState("");
  const [busy, setBusy] = useState(false);
  const [last, setLast] = useState<POSCheckoutResult | null>(null);
  const [sales, setSales] = useState<Sale[]>([]);

  const refreshSales = useCallback(async () => {
    const res = await listSales({
      branch_id: branchId || undefined,
      limit: 25,
    });
    setSales(res.items ?? []);
  }, [branchId]);

  const refresh = useCallback(async () => {
    const [locs, pay] = await Promise.all([
      listStockLocations(branchId || undefined),
      getPaymentSettings().catch(() => null),
    ]);
    setLocations(locs.items ?? []);
    setCfg(pay);
    const loc = locs.items?.find((l) => l.location_type === "counter") || locs.items?.[0];
    setLocationId((prev) => {
      if (prev && locs.items?.some((l) => l.id === prev)) return prev;
      return loc?.id || "";
    });
    await refreshSales().catch(() => setSales([]));
  }, [branchId, refreshSales]);

  useEffect(() => {
    refresh().catch((e) => setError(e instanceof Error ? e.message : "Failed to load"));
  }, [refresh]);

  useEffect(() => {
    if (!locationId) return;
    listPOSCatalog(locationId)
      .then((r) => setCatalog(r.items ?? []))
      .catch((e) => setError(e instanceof Error ? e.message : "Catalog failed"));
  }, [locationId]);

  const total = useMemo(
    () => cart.reduce((sum, l) => sum + l.item.sell_price * l.qty, 0),
    [cart],
  );

  function addToCart(item: CatalogItem) {
    setCart((prev) => {
      const existing = prev.find((l) => l.item.variant_id === item.variant_id);
      if (existing) {
        return prev.map((l) =>
          l.item.variant_id === item.variant_id ? { ...l, qty: Math.min(l.qty + 1, item.available_qty || 99) } : l,
        );
      }
      return [...prev, { item, qty: 1 }];
    });
    setLast(null);
  }

  function setQty(variantId: string, qty: number) {
    setCart((prev) =>
      prev
        .map((l) => (l.item.variant_id === variantId ? { ...l, qty } : l))
        .filter((l) => l.qty > 0),
    );
  }

  async function checkout(e: FormEvent) {
    e.preventDefault();
    if (!branchId || !locationId || cart.length === 0) return;
    setBusy(true);
    setError("");
    try {
      const result = await posCheckout({
        branch_id: branchId,
        location_id: locationId,
        method,
        phone: method === "mpesa_stk" ? phone : undefined,
        items: cart.map((l) => ({ variant_id: l.item.variant_id, quantity: l.qty })),
      });
      setLast(result);
      if (result.completed) {
        setCart([]);
        const cat = await listPOSCatalog(locationId);
        setCatalog(cat.items ?? []);
      }
      await refreshSales().catch(() => undefined);
    } catch (err) {
      setError(err instanceof Error ? err.message : "Checkout failed");
    } finally {
      setBusy(false);
    }
  }

  async function doReverse(saleId: string) {
    if (!locationId) return;
    setBusy(true);
    setError("");
    try {
      await reverseSale(saleId, locationId);
      await Promise.all([refreshSales(), listPOSCatalog(locationId).then((r) => setCatalog(r.items ?? []))]);
    } catch (err) {
      setError(err instanceof Error ? err.message : "Reverse failed");
    } finally {
      setBusy(false);
    }
  }

  async function finishSTK() {
    if (!last?.sale || !last.payment) return;
    setBusy(true);
    setError("");
    try {
      await confirmMpesaPayment(last.payment.id);
      const sale = await completeSale(last.sale.id, locationId);
      setLast({ ...last, sale, completed: true, payment: { ...last.payment, status: "allocated" } });
      setCart([]);
      const cat = await listPOSCatalog(locationId);
      setCatalog(cat.items ?? []);
      await refreshSales().catch(() => undefined);
    } catch (err) {
      setError(err instanceof Error ? err.message : "Complete failed");
    } finally {
      setBusy(false);
    }
  }

  async function finishC2B() {
    if (!last?.sale || !last.payment) return;
    setBusy(true);
    setError("");
    try {
      const pay = await getPayment(last.payment.id);
      if (pay.status !== "allocated" && pay.status !== "confirmed") {
        throw new Error("Waiting for paybill confirmation — ask customer to pay with the account ref shown");
      }
      const sale = await completeSale(last.sale.id, locationId);
      setLast({ ...last, sale, completed: true, payment: { ...pay } });
      setCart([]);
      const cat = await listPOSCatalog(locationId);
      setCatalog(cat.items ?? []);
      await refreshSales().catch(() => undefined);
    } catch (err) {
      setError(err instanceof Error ? err.message : "Complete failed");
    } finally {
      setBusy(false);
    }
  }

  return (
    <div className="pos-page">
      <PageHeader
        title="POS"
        subtitle="Retail accessories and quick checkout"
        actions={
          <Link to="/settings/payments" className="muted">
            Payment rails
          </Link>
        }
      />
      {error ? <p className="form-error">{error}</p> : null}

      <div className="pos-toolbar">
        <label>
          Branch
          <select className="input" value={branchId} onChange={(e) => setBranchId(e.target.value)}>
            {branches.map((b) => (
              <option key={b.id} value={b.id}>
                {b.name}
              </option>
            ))}
          </select>
        </label>
        <label>
          Stock location
          <select className="input" value={locationId} onChange={(e) => setLocationId(e.target.value)}>
            {locations.map((l) => (
              <option key={l.id} value={l.id}>
                {l.name}
              </option>
            ))}
          </select>
        </label>
      </div>

      <div className="repair-grid">
        <section className="panel">
          <h2>Catalog</h2>
          {catalog.length === 0 ? (
            <EmptyState title="No POS items" body="Restart the API to seed demo accessories, or add products." icon={ICONS.pos} />
          ) : (
            <ul className="pos-catalog">
              {catalog.map((it) => (
                <li key={it.variant_id}>
                  <button type="button" className="pos-item" onClick={() => addToCart(it)} disabled={it.available_qty <= 0}>
                    <strong>{it.product_name}</strong>
                    <span className="muted">
                      {it.sku} · {it.available_qty} left
                    </span>
                    <span className="pos-price">KES {it.sell_price.toLocaleString()}</span>
                  </button>
                </li>
              ))}
            </ul>
          )}
        </section>

        <aside className="stack">
          <section className="panel">
            <h2>Cart</h2>
            {cart.length === 0 ? (
              <EmptyState title="Empty cart" body="Tap catalog items to add." />
            ) : (
              <ul className="part-list">
                {cart.map((l) => (
                  <li key={l.item.variant_id} className="part-card">
                    <div className="part-head">
                      <strong>{l.item.product_name}</strong>
                      <span>KES {(l.item.sell_price * l.qty).toLocaleString()}</span>
                    </div>
                    <label>
                      Qty
                      <Input
                        type="number"
                        min={1}
                        max={l.item.available_qty || 99}
                        value={l.qty}
                        onChange={(e) => setQty(l.item.variant_id, Number(e.target.value) || 0)}
                      />
                    </label>
                  </li>
                ))}
              </ul>
            )}

            <form className="form-grid" onSubmit={checkout} style={{ marginTop: "1rem" }}>
              <div className="pos-total">
                <span className="muted">Total</span>
                <strong>KES {total.toLocaleString()}</strong>
              </div>
              <label>
                Pay with
                <select className="input" value={method} onChange={(e) => setMethod(e.target.value)}>
                  <option value="cash">Cash</option>
                  <option value="mpesa_stk" disabled={!cfg?.configured}>
                    M-Pesa STK
                  </option>
                  <option value="mpesa_c2b" disabled={!cfg?.configured}>
                    M-Pesa paybill (C2B)
                  </option>
                  <option value="bank_paybill" disabled={!cfg?.bank_configured}>
                    Bank paybill
                  </option>
                </select>
              </label>
              {method === "mpesa_stk" ? (
                <label>
                  Customer phone
                  <Input value={phone} onChange={(e) => setPhone(e.target.value)} placeholder="07XXXXXXXX" required />
                </label>
              ) : null}
              {method === "mpesa_c2b" && cfg?.configured ? (
                <p className="hint">
                  After checkout, customer pays Till/Paybill <span className="mono">{cfg.mpesa_shortcode}</span> using the
                  sale account ref shown below.
                </p>
              ) : null}
              <Button type="submit" disabled={busy || cart.length === 0 || total <= 0}>
                Checkout
              </Button>
            </form>
          </section>

          {last ? (
            <section className="panel context-panel">
              <h2>Last sale</h2>
              <dl className="meta-dl">
                <dt>Status</dt>
                <dd>
                  <Badge tone={last.completed ? "success" : "pending"}>
                    {last.completed ? "completed" : last.sale.status}
                  </Badge>
                </dd>
                <dt>Total</dt>
                <dd>KES {last.sale.total.toLocaleString()}</dd>
                <dt>Payment</dt>
                <dd>
                  {last.payment.method.replaceAll("_", " ")} · {last.payment.status}
                </dd>
                {last.payment.account_reference ? (
                  <>
                    <dt>Account ref</dt>
                    <dd className="mono">{last.payment.account_reference}</dd>
                  </>
                ) : null}
                {last.payment.method === "mpesa_c2b" && cfg?.mpesa_shortcode ? (
                  <>
                    <dt>Paybill</dt>
                    <dd className="mono">{cfg.mpesa_shortcode}</dd>
                  </>
                ) : null}
              </dl>
              {!last.completed && last.payment.method === "mpesa_stk" ? (
                <Button type="button" disabled={busy} onClick={() => void finishSTK()}>
                  Confirm STK & complete sale
                </Button>
              ) : null}
              {!last.completed && last.payment.method === "mpesa_c2b" ? (
                <Button type="button" disabled={busy} onClick={() => void finishC2B()}>
                  Check paybill & complete sale
                </Button>
              ) : null}
            </section>
          ) : null}
        </aside>
      </div>

      <section className="panel" style={{ marginTop: "1.25rem" }}>
        <div className="panel-head">
          <h2>Sale history</h2>
          <Button type="button" variant="ghost" disabled={busy} onClick={() => void refreshSales()}>
            Refresh
          </Button>
        </div>
        {sales.length === 0 ? (
          <EmptyState title="No sales yet" body="Completed and pending POS sales for this branch appear here." />
        ) : (
          <ul className="part-list">
            {sales.map((s) => (
              <li key={s.id} className="part-card">
                <div className="part-head">
                  <div>
                    <strong className="mono">{s.id.slice(0, 8)}…</strong>
                    <div className="muted">
                      {s.channel} · {s.created_at ? new Date(s.created_at).toLocaleString() : "—"}
                    </div>
                  </div>
                  <Badge tone={saleTone(s.status)}>{s.status}</Badge>
                </div>
                <div className="inline-form">
                  <strong>KES {s.total.toLocaleString()}</strong>
                  {s.status === "completed" ? (
                    <Button type="button" variant="secondary" disabled={busy || !locationId} onClick={() => void doReverse(s.id)}>
                      Reverse
                    </Button>
                  ) : null}
                </div>
              </li>
            ))}
          </ul>
        )}
      </section>
    </div>
  );
}
