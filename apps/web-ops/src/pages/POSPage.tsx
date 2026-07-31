import { useCallback, useEffect, useMemo, useRef, useState, type FormEvent } from "react";
import { Link } from "react-router-dom";
import { useBranch } from "../branch/BranchContext";
import { Badge, Button, EmptyState, ICONS, Input, SearchInput, StkWaitOverlay, isTerminalStkError } from "../components/ui";
import {
  completeSale,
  confirmMpesaPayment,
  downloadSaleReceiptPDF,
  getPayment,
  getPaymentSettings,
  listPOSCatalog,
  listSales,
  listStockLocations,
  listSuppliers,
  openSaleReceipt,
  posCheckout,
  reverseSale,
  type CatalogItem,
  type PaymentProviderSettings,
  type POSCheckoutItem,
  type POSCheckoutResult,
  type Sale,
  type StockLocation,
  type Supplier,
} from "../lib/api";

// A cart line is either a catalog item (variantId set, stock-backed) or a quick
// sale — something sold on the spot that isn't in inventory. unitCost/supplierId
// are internal-only bookkeeping (what we owe the supplier, and margin) and are
// never sent anywhere near the customer receipt.
type CartLine = {
  id: string;
  description: string;
  sku?: string;
  unitPrice: number;
  qty: number;
  variantId?: string;
  availableQty?: number;
  unitCost?: number;
  supplierId?: string;
  supplierName?: string;
};

function newLineId() {
  return typeof crypto.randomUUID === "function" ? crypto.randomUUID() : `line-${Date.now()}-${Math.random()}`;
}

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
  const [catalogQuery, setCatalogQuery] = useState("");
  const [cashReceived, setCashReceived] = useState("");
  const [suppliers, setSuppliers] = useState<Supplier[]>([]);
  const [showQuickSale, setShowQuickSale] = useState(false);
  const [qsDescription, setQsDescription] = useState("");
  const [qsPrice, setQsPrice] = useState("");
  const [qsQty, setQsQty] = useState("1");
  const [qsSourced, setQsSourced] = useState(false);
  const [qsSupplierId, setQsSupplierId] = useState("");
  const [qsCost, setQsCost] = useState("");
  const [qsError, setQsError] = useState("");
  const [showQuickStk, setShowQuickStk] = useState(false);
  const [qStkAmount, setQStkAmount] = useState("");
  const [qStkPhone, setQStkPhone] = useState("");
  const [qStkError, setQStkError] = useState("");
  const [stkPolling, setStkPolling] = useState(false);
  const [stkSuccess, setStkSuccess] = useState("");
  const searchRef = useRef<HTMLInputElement>(null);
  const pollingRef = useRef(false);

  const printReceipt = useCallback(async (saleId: string) => {
    try {
      await openSaleReceipt(saleId);
    } catch (err) {
      setError(err instanceof Error ? err.message : "Could not open the receipt");
    }
  }, []);

  const refreshSales = useCallback(async () => {
    const res = await listSales({
      branch_id: branchId || undefined,
      limit: 25,
    });
    setSales(res.items ?? []);
  }, [branchId]);

  const refresh = useCallback(async () => {
    const [locs, pay, sup] = await Promise.all([
      listStockLocations(branchId || undefined),
      getPaymentSettings().catch(() => null),
      listSuppliers().catch(() => ({ items: [] })),
    ]);
    setLocations(locs.items ?? []);
    setCfg(pay);
    setSuppliers(sup.items ?? []);
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

  const total = useMemo(() => cart.reduce((sum, l) => sum + l.unitPrice * l.qty, 0), [cart]);
  const cartUnits = useMemo(() => cart.reduce((sum, line) => sum + line.qty, 0), [cart]);
  const cashAmount = Number(cashReceived) || 0;
  const changeDue = Math.max(0, cashAmount - total);

  const visibleCatalog = useMemo(() => {
    const needle = catalogQuery.trim().toLowerCase();
    const matches = needle
      ? catalog.filter((item) => (item.product_name + " " + item.sku).toLowerCase().includes(needle))
      : catalog;
    return matches.slice(0, needle ? 40 : 12);
  }, [catalog, catalogQuery]);

  function addToCart(item: CatalogItem) {
    setCart((prev) => {
      const existing = prev.find((l) => l.variantId === item.variant_id);
      if (existing) {
        return prev.map((l) =>
          l.variantId === item.variant_id ? { ...l, qty: Math.min(l.qty + 1, item.available_qty || 99) } : l,
        );
      }
      return [
        ...prev,
        {
          id: newLineId(),
          description: item.product_name,
          sku: item.sku,
          unitPrice: item.sell_price,
          qty: 1,
          variantId: item.variant_id,
          availableQty: item.available_qty,
        },
      ];
    });
    setLast(null);
  }

  function addQuickSaleLine(e: FormEvent) {
    e.preventDefault();
    const description = qsDescription.trim();
    const price = Number(qsPrice);
    const qty = Number(qsQty);
    if (!description) { setQsError("Describe the item"); return; }
    if (!Number.isFinite(price) || price <= 0) { setQsError("Enter a positive sell price"); return; }
    if (!Number.isFinite(qty) || qty <= 0) { setQsError("Enter a positive quantity"); return; }
    let unitCost: number | undefined;
    let supplierId: string | undefined;
    let supplierName: string | undefined;
    if (qsSourced) {
      const cost = Number(qsCost);
      if (!qsSupplierId) { setQsError("Choose which supplier this came from"); return; }
      if (!Number.isFinite(cost) || cost < 0) { setQsError("Enter what we paid the supplier"); return; }
      unitCost = cost;
      supplierId = qsSupplierId;
      supplierName = suppliers.find((s) => s.id === qsSupplierId)?.name;
    }
    setCart((prev) => [
      ...prev,
      { id: newLineId(), description, unitPrice: price, qty, unitCost, supplierId, supplierName },
    ]);
    setQsError("");
    setQsDescription("");
    setQsPrice("");
    setQsQty("1");
    setQsSourced(false);
    setQsSupplierId("");
    setQsCost("");
    setShowQuickSale(false);
    setLast(null);
  }

  function setQty(id: string, qty: number) {
    setCart((prev) => prev.map((l) => (l.id === id ? { ...l, qty } : l)).filter((l) => l.qty > 0));
  }

  function removeFromCart(id: string) {
    setCart((current) => current.filter((line) => line.id !== id));
  }

  function clearCart() {
    setCart([]);
    setCashReceived("");
    setLast(null);
  }

  async function runCheckout(items: POSCheckoutItem[], chargeMethod: string, chargePhone?: string) {
    if (!branchId || !locationId || items.length === 0) return;
    setBusy(true);
    setError("");
    try {
      const result = await posCheckout({
        branch_id: branchId,
        location_id: locationId,
        method: chargeMethod,
        phone: chargeMethod === "mpesa_stk" ? chargePhone : undefined,
        items,
      });
      setLast(result);
      setCashReceived("");
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

  async function checkout(e: FormEvent) {
    e.preventDefault();
    await runCheckout(
      cart.map((l) =>
        l.variantId
          ? { variant_id: l.variantId, quantity: l.qty }
          : {
              description: l.description,
              quantity: l.qty,
              unit_price: l.unitPrice,
              unit_cost: l.unitCost,
              supplier_id: l.supplierId,
            },
      ),
      method,
      phone,
    );
  }

  async function sendQuickStk(e: FormEvent) {
    e.preventDefault();
    const amount = Number(qStkAmount);
    const stkPhone = qStkPhone.trim();
    if (!Number.isFinite(amount) || amount <= 0) { setQStkError("Enter a positive amount"); return; }
    if (!stkPhone) { setQStkError("Enter the customer's phone number"); return; }
    setQStkError("");
    await runCheckout([{ description: "Quick payment", quantity: 1, unit_price: amount }], "mpesa_stk", stkPhone);
    setShowQuickStk(false);
    setQStkAmount("");
    setQStkPhone("");
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
      void printReceipt(sale.id);
    } catch (err) {
      setError(err instanceof Error ? err.message : "Complete failed");
    } finally {
      setBusy(false);
    }
  }

  // Auto-poll STK like the Android counter — cashier stays with the customer.
  useEffect(() => {
    if (!last || last.completed || last.payment.method !== "mpesa_stk" || !locationId) return;
    if (pollingRef.current) return;
    let cancelled = false;
    pollingRef.current = true;
    setStkPolling(true);
    setStkSuccess("");
    setError("");
    (async () => {
      for (let i = 0; i < 48 && !cancelled; i++) {
        await new Promise((r) => window.setTimeout(r, 2500));
        if (cancelled) break;
        try {
          await confirmMpesaPayment(last.payment.id);
          const sale = await completeSale(last.sale.id, locationId);
          if (cancelled) break;
          setLast({ ...last, sale, completed: true, payment: { ...last.payment, status: "allocated" } });
          setCart([]);
          const cat = await listPOSCatalog(locationId);
          setCatalog(cat.items ?? []);
          await refreshSales().catch(() => undefined);
          void printReceipt(sale.id);
          setStkSuccess("Payment successful — sale complete");
          window.setTimeout(() => setStkSuccess(""), 2500);
          break;
        } catch (err) {
          const msg = err instanceof Error ? err.message : "";
          if (isTerminalStkError(msg)) {
            setError(msg || "STK failed or cancelled");
            break;
          }
          // keep polling until timeout
        }
      }
      if (!cancelled) setStkPolling(false);
      pollingRef.current = false;
    })();
    return () => {
      cancelled = true;
      pollingRef.current = false;
      setStkPolling(false);
    };
  }, [last?.sale?.id, last?.payment?.id, last?.completed, last?.payment?.method, locationId, printReceipt, refreshSales]);

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
    <div className="pos-fullscreen">
      <header className="pos-fullscreen-bar">
        <div className="pos-fullscreen-brand">
          <strong>Sell</strong>
          <span>Counter · full screen</span>
        </div>
        <div className="pos-fullscreen-meta">
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
            Stock
            <select className="input" value={locationId} onChange={(e) => setLocationId(e.target.value)}>
              {locations.map((l) => (
                <option key={l.id} value={l.id}>
                  {l.name}
                </option>
              ))}
            </select>
          </label>
        </div>
        <nav className="pos-fullscreen-actions" aria-label="Counter tools">
          <Link to="/counter/fix" className="btn btn-ghost">Same-day fix</Link>
          <Link to="/counter/pickup" className="btn btn-ghost">Pickup</Link>
          <Link to="/" className="btn btn-secondary">Exit to ops</Link>
        </nav>
      </header>

      {error ? <p className="form-error pos-fullscreen-error">{error}</p> : null}
      <StkWaitOverlay
        visible={stkPolling}
        message="Waiting for M-Pesa"
        detail="Ask the customer to enter their PIN on the phone. Receipt prints when payment succeeds."
        success={stkSuccess}
      />

      <div className="pos-counter pos-counter-immersive">
      <div className="counter-layout">
        <section className="panel" style={{ padding: "0.85rem" }}>
          <div className="panel-head">
            <div>
              <h2>Find an item</h2>
              <span className="muted">{catalog.length} stocked · type SKU and press Enter</span>
            </div>
            <SearchInput
              ref={searchRef}
              value={catalogQuery}
              onChange={(e) => setCatalogQuery(e.target.value)}
              onKeyDown={(e) => {
                if (e.key !== "Enter") return;
                const exact = catalog.find((item) => item.sku.toLowerCase() === catalogQuery.trim().toLowerCase());
                if (!exact || exact.available_qty <= 0) return;
                e.preventDefault();
                addToCart(exact);
                setCatalogQuery("");
              }}
              placeholder="Search name or scan SKU…"
              aria-label="Search or scan POS catalog"
              autoFocus
            />
          </div>
          {!showQuickSale ? (
            <button type="button" className="linkish" onClick={() => setShowQuickSale(true)} style={{ margin: "0.5rem 0" }}>
              + Quick sale (item not in stock)
            </button>
          ) : (
            <form className="form-grid" onSubmit={addQuickSaleLine} style={{ margin: "0.5rem 0 1rem", padding: "0.85rem", border: "1px solid var(--gray-200)", borderRadius: 7 }}>
              <p className="muted" style={{ margin: 0, gridColumn: "1 / -1" }}>
                For something you don't stock but sourced on the spot. The customer only ever sees the description and
                sell price — supplier and cost stay internal.
              </p>
              <label>
                What are you selling?
                <Input value={qsDescription} onChange={(e) => setQsDescription(e.target.value)} placeholder="e.g. iPhone 13 back glass" />
              </label>
              <label>
                Sell price (KES)
                <Input type="number" min={0} value={qsPrice} onChange={(e) => setQsPrice(e.target.value)} />
              </label>
              <label>
                Quantity
                <Input type="number" min={1} value={qsQty} onChange={(e) => setQsQty(e.target.value)} />
              </label>
              <label className="checkbox-row">
                <input type="checkbox" checked={qsSourced} onChange={(e) => setQsSourced(e.target.checked)} />
                Sourced from a supplier (track what we owe them)
              </label>
              {qsSourced ? (
                <>
                  <label>
                    Supplier
                    <select className="input" value={qsSupplierId} onChange={(e) => setQsSupplierId(e.target.value)}>
                      <option value="">Select…</option>
                      {suppliers.map((s) => (
                        <option key={s.id} value={s.id}>{s.name}</option>
                      ))}
                    </select>
                    {suppliers.length === 0 ? (
                      <span className="hint">
                        No suppliers yet — add one under <Link to="/suppliers">Suppliers</Link>.
                      </span>
                    ) : null}
                  </label>
                  <label>
                    What we paid them (KES, per item)
                    <Input type="number" min={0} value={qsCost} onChange={(e) => setQsCost(e.target.value)} />
                    <span className="hint">Internal only — used for the supplier credit ledger and margin, never shown to the customer.</span>
                  </label>
                </>
              ) : null}
              {qsError ? <p className="form-error">{qsError}</p> : null}
              <div className="btn-row">
                <Button type="submit">Add to cart</Button>
                <Button type="button" variant="ghost" onClick={() => { setShowQuickSale(false); setQsError(""); }}>Cancel</Button>
              </div>
            </form>
          )}
          {catalog.length === 0 ? (
            <EmptyState title="No POS items" body="Add products under Inventory, then stock this location." icon={ICONS.pos} />
          ) : (
            <ul className="pos-catalog-grid">
              {visibleCatalog.map((it) => (
                <li key={it.variant_id}>
                  <button type="button" className="pos-item" onClick={() => addToCart(it)} disabled={it.available_qty <= 0}>
                    <strong>{it.product_name}</strong>
                    <span className="muted">
                      {it.sku} · {it.available_qty} left
                    </span>
                    <span className="pos-price">KES {it.sell_price.toLocaleString()}</span>
                    <span className="pos-add-label">{it.available_qty > 0 ? "Add to cart" : "Out of stock"}</span>
                  </button>
                </li>
              ))}
            </ul>
          )}
          {catalog.length > visibleCatalog.length && !catalogQuery.trim() ? (
            <p className="pos-search-hint">Showing 12 items. Search to find any other product.</p>
          ) : null}
        </section>

        <aside className="cart-rail">
          <div className="cart-head">
            <div>
              <span className="cart-eyebrow">Current sale</span>
              <h2>Cart <small>{cartUnits} {cartUnits === 1 ? "item" : "items"}</small></h2>
            </div>
            {cart.length > 0 ? <button type="button" className="cart-clear" onClick={clearCart}>Clear</button> : null}
          </div>

          {!showQuickStk ? (
            <button type="button" className="linkish" onClick={() => setShowQuickStk(true)}>
              Quick STK — amount only
            </button>
          ) : (
            <form className="form-grid" onSubmit={sendQuickStk} style={{ padding: "0.75rem", border: "1px solid var(--gray-200)", borderRadius: 7, marginBottom: "0.75rem" }}>
              <p className="muted" style={{ margin: 0, gridColumn: "1 / -1" }}>
                Skip the cart — just send an STK prompt for an amount and phone number.
              </p>
              <label>
                Amount (KES)
                <Input type="number" min={0} value={qStkAmount} onChange={(e) => setQStkAmount(e.target.value)} autoFocus />
              </label>
              <label>
                Customer phone
                <Input value={qStkPhone} onChange={(e) => setQStkPhone(e.target.value)} placeholder="07XXXXXXXX" />
              </label>
              {qStkError ? <p className="form-error">{qStkError}</p> : null}
              <div className="btn-row">
                <Button type="submit" disabled={busy}>{busy ? "Sending…" : "Send STK"}</Button>
                <Button type="button" variant="ghost" onClick={() => { setShowQuickStk(false); setQStkError(""); }}>Cancel</Button>
              </div>
            </form>
          )}

          {cart.length === 0 ? (
            <EmptyState title="Empty cart" body="Tap catalog tiles to add." />
          ) : (
            <ul className="part-list">
              {cart.map((l) => (
                <li key={l.id} className="part-card">
                  <div className="part-head">
                    <strong>
                      {l.description}
                      {!l.variantId ? <Badge tone="pending">Quick sale</Badge> : null}
                    </strong>
                    <span>KES {(l.unitPrice * l.qty).toLocaleString()}</span>
                  </div>
                  <div className="cart-line-foot">
                    <span className="mono">
                      {l.sku ? `${l.sku} · ` : ""}KES {l.unitPrice.toLocaleString()} each
                    </span>
                    <div className="qty-stepper" aria-label={"Quantity for " + l.description}>
                      <button type="button" onClick={() => setQty(l.id, l.qty - 1)} aria-label="Decrease quantity">−</button>
                      <strong>{l.qty}</strong>
                      <button type="button" onClick={() => setQty(l.id, Math.min(l.qty + 1, l.availableQty || 99))} aria-label="Increase quantity">+</button>
                    </div>
                    <button type="button" className="cart-remove" onClick={() => removeFromCart(l.id)}>Remove</button>
                  </div>
                  {l.supplierId ? (
                    <p className="muted" style={{ margin: "0.35rem 0 0", fontSize: "0.78rem" }}>
                      Internal only — from {l.supplierName ?? "supplier"} @ KES {(l.unitCost ?? 0).toLocaleString()} each ·
                      margin KES {((l.unitPrice - (l.unitCost ?? 0)) * l.qty).toLocaleString()}
                    </p>
                  ) : null}
                </li>
              ))}
            </ul>
          )}

          <form className="form-grid" onSubmit={checkout}>
            <div className="pos-total">
              <span className="muted">Total</span>
              <strong>KES {total.toLocaleString()}</strong>
            </div>
            <fieldset className="payment-methods">
              <legend>Payment method</legend>
              <div className="payment-method-grid">
                <button type="button" className={method === "cash" ? "active" : ""} onClick={() => setMethod("cash")}>Cash</button>
                <button type="button" className={method === "mpesa_stk" ? "active" : ""} onClick={() => setMethod("mpesa_stk")} disabled={!cfg?.configured}>M-Pesa STK</button>
                <button type="button" className={method === "mpesa_c2b" ? "active" : ""} onClick={() => setMethod("mpesa_c2b")} disabled={!cfg?.configured}>Paybill</button>
                <button type="button" className={method === "bank_paybill" ? "active" : ""} onClick={() => setMethod("bank_paybill")} disabled={!cfg?.bank_configured}>Bank</button>
              </div>
            </fieldset>
            {method === "cash" ? (
              <div className="cash-tender">
                <div className="cash-tender-head">
                  <label>Cash received<Input type="number" min={0} value={cashReceived} onChange={(e) => setCashReceived(e.target.value)} placeholder="0" inputMode="decimal" /></label>
                  <button type="button" onClick={() => setCashReceived(String(total))}>Exact</button>
                </div>
                <div className={"cash-change " + (cashAmount > 0 && cashAmount < total ? "short" : "")}>
                  <span>{cashAmount > 0 && cashAmount < total ? "Still needed" : "Change"}</span>
                  <strong>KES {(cashAmount > 0 && cashAmount < total ? total - cashAmount : changeDue).toLocaleString()}</strong>
                </div>
              </div>
            ) : null}
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
            <Button type="submit" disabled={busy || stkPolling || cart.length === 0 || total <= 0 || (method === "cash" && cashAmount < total)}>
              {busy || stkPolling ? "Processing…" : "Charge KES " + total.toLocaleString()}
            </Button>
          </form>

          {last ? (
            <div className="action-block" style={{ marginTop: "1rem" }}>
              <p className="muted">Last sale</p>
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
              </dl>
              {!last.completed && last.payment.method === "mpesa_stk" ? (
                <Button type="button" disabled={busy || stkPolling} onClick={() => void finishSTK()}>
                  Check payment now
                </Button>
              ) : null}
              {!last.completed && last.payment.method === "mpesa_c2b" ? (
                <Button type="button" disabled={busy} onClick={() => void finishC2B()}>
                  Check paybill & complete sale
                </Button>
              ) : null}
              <div className="chip-row" style={{ marginTop: "0.6rem" }}>
                <Button type="button" variant="secondary" onClick={() => void printReceipt(last.sale.id)}>
                  Print receipt
                </Button>
                <Button type="button" variant="ghost" onClick={() => void downloadSaleReceiptPDF(last.sale.id)}>
                  Receipt PDF
                </Button>
              </div>
            </div>
          ) : null}
        </aside>
      </div>

      <section className="sales-strip">
        <div className="panel-head">
          <h2>Sale history</h2>
          <Button type="button" variant="ghost" disabled={busy} onClick={() => void refreshSales()}>
            Refresh
          </Button>
        </div>
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
    </div>
  );
}
