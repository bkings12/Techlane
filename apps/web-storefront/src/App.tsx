import { useEffect, useMemo, useState, type ReactNode } from "react";
import {
  checkout,
  getBootstrap,
  getOrder,
  isOrderFailed,
  isOrderPaid,
  listAccountOrders,
  listCatalog,
  loginAccount,
  onStorefrontSessionExpired,
  orderStatusMessage,
  registerAccount,
  type Bootstrap,
  type CatalogItem,
  type CustomerSession,
  type Order,
  type PublicBranch,
} from "./lib/api";
import {
  clearSession,
  loadBranch,
  loadCart,
  loadRecentOrderIds,
  loadSession,
  rememberOrderId,
  saveBranch,
  saveCart,
  saveSession,
  type StoredBranch,
  type StoredSession,
  type CartMap,
} from "./lib/storage";

type View = "hero" | "shop" | "product" | "checkout" | "pay" | "done" | "lookup" | "account";
type PayMethod = "mpesa_c2b" | "mpesa_stk";

export default function App() {
  const [view, setView] = useState<View>("hero");
  const [boot, setBoot] = useState<Bootstrap | null>(null);
  const [catalog, setCatalog] = useState<CatalogItem[]>([]);
  const [cart, setCart] = useState<CartMap>(() => loadCart());
  const [session, setSession] = useState<StoredSession | null>(() => loadSession());
  const [selectedBranch, setSelectedBranch] = useState<StoredBranch | null>(() => loadBranch());
  const [accountOrders, setAccountOrders] = useState<Order[]>([]);
  const [accountMode, setAccountMode] = useState<"login" | "register">("login");
  const [acctEmail, setAcctEmail] = useState("");
  const [acctPassword, setAcctPassword] = useState("");
  const [acctName, setAcctName] = useState("");
  const [acctPhone, setAcctPhone] = useState("");
  const [selectedId, setSelectedId] = useState<string | null>(null);
  const [order, setOrder] = useState<Order | null>(null);
  const [payRef, setPayRef] = useState<string | undefined>();
  const [payMethodUsed, setPayMethodUsed] = useState<PayMethod>("mpesa_c2b");
  const [method, setMethod] = useState<PayMethod>("mpesa_c2b");
  const [phone, setPhone] = useState("");
  const [query, setQuery] = useState("");
  const [brandFilter, setBrandFilter] = useState("");
  const [categoryFilter, setCategoryFilter] = useState("");
  const [lookupId, setLookupId] = useState("");
  const [recentIds, setRecentIds] = useState<string[]>(() => loadRecentOrderIds());
  const [error, setError] = useState("");
  const [busy, setBusy] = useState(false);
  const [loadingShop, setLoadingShop] = useState(true);
  const [pollError, setPollError] = useState("");

  useEffect(() => {
    saveCart(cart);
  }, [cart]);

  useEffect(() => {
    let cancelled = false;
    setLoadingShop(true);
    getBootstrap()
      .then(async (b) => {
        if (cancelled) return;
        setBoot(b);
        const stored = loadBranch();
        const branch = pickBranch(b, stored);
        setSelectedBranch(branch);
        saveBranch(branch);
        const cat = await listCatalog(branch.location_id);
        if (cancelled) return;
        setCatalog(cat.items ?? []);
      })
      .catch((e) => {
        if (!cancelled) setError(e instanceof Error ? e.message : "Failed to load shop");
      })
      .finally(() => {
        if (!cancelled) setLoadingShop(false);
      });
    return () => {
      cancelled = true;
    };
  }, []);

  useEffect(() => {
    if (!session?.token) {
      setAccountOrders([]);
      return;
    }
    listAccountOrders(session.token)
      .then((res) => setAccountOrders(res.items ?? []))
      .catch(() => setAccountOrders([]));
  }, [session?.token]);

  // Resume recent order from local storage on first paint if hash asks.
  useEffect(() => {
    const hash = window.location.hash.replace(/^#\/?/, "");
    if (hash.startsWith("order/")) {
      const id = hash.slice("order/".length).split(/[?/]/)[0];
      if (id) {
        setLookupId(id);
        setView("lookup");
        void fetchOrderStatus(id, true);
      }
    } else if (hash === "orders" || hash === "lookup") {
      setView("lookup");
    } else if (hash === "shop") {
      setView("shop");
    }
  }, []);

  useEffect(() => {
    if (view !== "pay" || !order?.id) return;
    if (isOrderPaid(order)) {
      setView("done");
      return;
    }
    if (isOrderFailed(order)) return;

    let stopped = false;
    const tick = () => {
      getOrder(order.id)
        .then((o) => {
          if (stopped) return;
          setOrder(o);
          setPollError("");
          if (isOrderPaid(o)) setView("done");
        })
        .catch((e) => {
          if (!stopped) setPollError(e instanceof Error ? e.message : "Could not refresh order");
        });
    };
    const t = window.setInterval(tick, 4000);
    return () => {
      stopped = true;
      window.clearInterval(t);
    };
  }, [view, order?.id, order?.status]);

  const brands = useMemo(() => {
    const set = new Set<string>();
    for (const item of catalog) {
      if (item.brand?.trim()) set.add(item.brand.trim());
    }
    return [...set].sort((a, b) => a.localeCompare(b));
  }, [catalog]);

  const categories = useMemo(() => {
    const set = new Set<string>();
    for (const item of catalog) {
      if (item.category?.trim()) set.add(item.category.trim());
    }
    return [...set].sort((a, b) => a.localeCompare(b));
  }, [catalog]);

  const filtered = useMemo(() => {
    const q = query.trim().toLowerCase();
    return catalog.filter((item) => {
      if (brandFilter && (item.brand ?? "") !== brandFilter) return false;
      if (categoryFilter && (item.category ?? "") !== categoryFilter) return false;
      if (!q) return true;
      const hay = `${item.product_name} ${item.brand ?? ""} ${item.category ?? ""} ${item.sku}`.toLowerCase();
      return hay.includes(q);
    });
  }, [catalog, query, brandFilter, categoryFilter]);

  const pickup = selectedBranch ?? (boot
    ? {
        branch_id: boot.branch_id,
        branch_name: boot.branch_name,
        location_id: boot.location_id,
        location_name: boot.location_name,
      }
    : null);

  function pickBranch(b: Bootstrap, stored: StoredBranch | null): StoredBranch {
    const branches = b.branches?.length
      ? b.branches
      : [{ id: b.branch_id, name: b.branch_name, location_id: b.location_id, location_name: b.location_name }];
    if (stored) {
      const match = branches.find((x) => x.id === stored.branch_id && x.location_id === stored.location_id);
      if (match) {
        return {
          branch_id: match.id,
          branch_name: match.name,
          location_id: match.location_id,
          location_name: match.location_name,
        };
      }
    }
    const first = branches[0]!;
    return {
      branch_id: first.id,
      branch_name: first.name,
      location_id: first.location_id,
      location_name: first.location_name,
    };
  }

  async function changeBranch(branch: PublicBranch) {
    const next: StoredBranch = {
      branch_id: branch.id,
      branch_name: branch.name,
      location_id: branch.location_id,
      location_name: branch.location_name,
    };
    setSelectedBranch(next);
    saveBranch(next);
    setCart({});
    setLoadingShop(true);
    setError("");
    try {
      const cat = await listCatalog(branch.location_id);
      setCatalog(cat.items ?? []);
    } catch (e) {
      setError(e instanceof Error ? e.message : "Failed to load catalog");
    } finally {
      setLoadingShop(false);
    }
  }

  const selected = selectedId ? catalog.find((c) => c.variant_id === selectedId) ?? null : null;

  const lines = useMemo(() => {
    return Object.entries(cart)
      .filter(([, q]) => q > 0)
      .map(([variantId, qty]) => {
        const item = catalog.find((c) => c.variant_id === variantId);
        return item ? { item, qty } : null;
      })
      .filter((l): l is { item: CatalogItem; qty: number } => Boolean(l));
  }, [cart, catalog]);

  const total = lines.reduce((s, l) => s + l.item.sell_price * l.qty, 0);
  const cartCount = lines.reduce((s, l) => s + l.qty, 0);

  function setQty(variantId: string, next: number, maxQty: number) {
    setCart((prev) => {
      const qty = Math.max(0, Math.min(maxQty, Math.floor(next)));
      if (qty <= 0) {
        const { [variantId]: _, ...rest } = prev;
        return rest;
      }
      return { ...prev, [variantId]: qty };
    });
  }

  function addOne(item: CatalogItem) {
    const current = cart[item.variant_id] ?? 0;
    if (current >= item.available_qty) return;
    setQty(item.variant_id, current + 1, item.available_qty);
  }

  function goShop() {
    setError("");
    setView("shop");
    window.location.hash = "shop";
  }

  function goLookup() {
    setError("");
    setView("lookup");
    window.location.hash = "orders";
  }

  async function fetchOrderStatus(id: string, navigate = false) {
    const trimmed = id.trim();
    if (!trimmed) {
      setError("Enter an order id");
      return;
    }
    setBusy(true);
    setError("");
    try {
      const o = await getOrder(trimmed);
      setOrder(o);
      rememberOrderId(o.id);
      setRecentIds(loadRecentOrderIds());
      window.location.hash = `order/${o.id}`;
      if (navigate) {
        if (isOrderPaid(o)) setView("done");
        else if (isOrderFailed(o)) setView("lookup");
        else setView("pay");
      }
    } catch (e) {
      setError(e instanceof Error ? e.message : "Order not found");
      if (navigate) setView("lookup");
    } finally {
      setBusy(false);
    }
  }

  async function placeOrder() {
    if (!pickup || lines.length === 0) return;
    if (method === "mpesa_stk" && !phone.trim()) {
      setError("Phone number is required for STK push (07… or 2547…)");
      return;
    }
    setBusy(true);
    setError("");
    try {
      const res = await checkout(
        {
          branch_id: pickup.branch_id,
          location_id: pickup.location_id,
          method,
          phone:
            method === "mpesa_stk"
              ? phone.trim() || session?.customer.phone || undefined
              : undefined,
          items: lines.map((l) => ({ variant_id: l.item.variant_id, quantity: l.qty })),
        },
        session?.token,
      );
      setCart({});
      rememberOrderId(res.order.id);
      setRecentIds(loadRecentOrderIds());
      setOrder(res.order);
      setPayRef(res.payment?.account_reference);
      setPayMethodUsed((res.payment?.method as PayMethod) || method);
      window.location.hash = `order/${res.order.id}`;
      if (isOrderPaid(res.order)) {
        setView("done");
      } else {
        setView("pay");
      }
    } catch (e) {
      setError(e instanceof Error ? e.message : "Checkout failed");
    } finally {
      setBusy(false);
    }
  }

  function applySession(s: CustomerSession) {
    const stored: StoredSession = {
      token: s.token,
      expires_at: s.expires_at,
      customer: s.customer,
    };
    saveSession(stored);
    setSession(stored);
    if (s.customer.phone) setPhone(s.customer.phone);
  }

  async function submitAccount(e: React.FormEvent) {
    e.preventDefault();
    setBusy(true);
    setError("");
    try {
      const s =
        accountMode === "register"
          ? await registerAccount({
              full_name: acctName.trim(),
              email: acctEmail.trim(),
              phone: acctPhone.trim() || undefined,
              password: acctPassword,
            })
          : await loginAccount({ email: acctEmail.trim(), password: acctPassword });
      applySession(s);
      setAcctPassword("");
    } catch (err) {
      setError(err instanceof Error ? err.message : "Account action failed");
    } finally {
      setBusy(false);
    }
  }

  function logout() {
    clearSession();
    setSession(null);
    setAccountOrders([]);
  }

  useEffect(() => onStorefrontSessionExpired(() => {
    setSession(null);
    setAccountOrders([]);
  }), []);

  function BranchPicker({ className }: { className?: string }) {
    const branches = boot?.branches?.length
      ? boot.branches
      : pickup
        ? [{
            id: pickup.branch_id,
            name: pickup.branch_name,
            location_id: pickup.location_id,
            location_name: pickup.location_name,
          }]
        : [];
    if (branches.length <= 1) {
      return pickup ? (
        <p className={className ?? "muted"}>
          Pickup: {pickup.branch_name} · {pickup.location_name}
        </p>
      ) : null;
    }
    return (
      <label className={className ?? "field"}>
        Pickup branch
        <select
          value={pickup?.branch_id ?? ""}
          onChange={(e) => {
            const b = branches.find((x) => x.id === e.target.value);
            if (b) void changeBranch(b);
          }}
        >
          {branches.map((b) => (
            <option key={b.id} value={b.id}>
              {b.name} · {b.location_name}
            </option>
          ))}
        </select>
      </label>
    );
  }

  function Shell({ children }: { children: ReactNode }) {
    return (
      <div className="app">
        <header className="topbar">
          <button type="button" className="brand-link" onClick={() => { setView("hero"); window.location.hash = ""; }}>
            <img className="brand-logo" src="/logo.svg" alt="" />
            Tech<span>Lane</span>
          </button>
          <nav className="nav">
            <button type="button" className="nav-link" onClick={goShop}>
              Shop
            </button>
            <button type="button" className="nav-link" onClick={() => { setError(""); setView("checkout"); }}>
              Bag{cartCount ? ` (${cartCount})` : ""}
            </button>
            <button type="button" className="nav-link" onClick={goLookup}>
              Orders
            </button>
            <button
              type="button"
              className="nav-link"
              onClick={() => {
                setError("");
                setView("account");
              }}
            >
              {session ? session.customer.full_name.split(" ")[0] : "Account"}
            </button>
          </nav>
        </header>
        {children}
      </div>
    );
  }

  if (view === "hero") {
    return (
      <div className="app">
        <section className="hero">
          <div className="hero-inner">
            <p className="brand">
              <img className="brand-logo" src="/logo.svg" alt="" />
              Tech<span>Lane</span>
            </p>
            <h2>Accessories ready for branch pickup</h2>
            <p>Order online, pay by M-Pesa, collect at the counter with your code.</p>
            <div className="stack" style={{ justifyContent: "flex-start" }}>
              <button type="button" className="btn" onClick={goShop}>
                Shop now
              </button>
              <button type="button" className="btn btn-ghost" onClick={goLookup}>
                Look up order
              </button>
            </div>
            {pickup ? (
              <p className="muted" style={{ marginTop: "1.25rem" }}>
                Pickup: {pickup.branch_name} · {pickup.location_name}
              </p>
            ) : null}
            {error ? <p className="error">{error}</p> : null}
          </div>
        </section>
      </div>
    );
  }

  if (view === "account") {
    return (
      <Shell>
        <section className="panel">
          <h1 className="section-title">Account</h1>
          {session ? (
            <>
              <p>
                Signed in as <strong>{session.customer.full_name}</strong>
              </p>
              <p className="muted">{session.customer.email}</p>
              {accountOrders.length > 0 ? (
                <div className="recent">
                  <h2>Your orders</h2>
                  <ul>
                    {accountOrders.map((o) => (
                      <li key={o.id}>
                        <button
                          type="button"
                          className="linkish"
                          onClick={() => {
                            setLookupId(o.id);
                            void fetchOrderStatus(o.id, true);
                          }}
                        >
                          {o.id.slice(0, 8)}… — KES {o.total.toLocaleString()} — {o.status}
                        </button>
                      </li>
                    ))}
                  </ul>
                </div>
              ) : (
                <p className="muted">No orders on this account yet.</p>
              )}
              <button type="button" className="btn btn-ghost" onClick={logout}>
                Sign out
              </button>
            </>
          ) : (
            <>
              <div className="stack" style={{ justifyContent: "flex-start", marginBottom: "1rem" }}>
                <button
                  type="button"
                  className={accountMode === "login" ? "btn" : "btn btn-ghost"}
                  onClick={() => setAccountMode("login")}
                >
                  Log in
                </button>
                <button
                  type="button"
                  className={accountMode === "register" ? "btn" : "btn btn-ghost"}
                  onClick={() => setAccountMode("register")}
                >
                  Register
                </button>
              </div>
              <form className="lookup-form" onSubmit={(e) => void submitAccount(e)}>
                {accountMode === "register" ? (
                  <>
                    <label className="field">
                      Full name
                      <input value={acctName} onChange={(e) => setAcctName(e.target.value)} required />
                    </label>
                    <label className="field">
                      Phone
                      <input value={acctPhone} onChange={(e) => setAcctPhone(e.target.value)} inputMode="tel" />
                    </label>
                  </>
                ) : null}
                <label className="field">
                  Email
                  <input
                    type="email"
                    value={acctEmail}
                    onChange={(e) => setAcctEmail(e.target.value)}
                    required
                    autoComplete="email"
                  />
                </label>
                <label className="field">
                  Password
                  <input
                    type="password"
                    value={acctPassword}
                    onChange={(e) => setAcctPassword(e.target.value)}
                    required
                    minLength={8}
                    autoComplete={accountMode === "login" ? "current-password" : "new-password"}
                  />
                </label>
                <button type="submit" className="btn" disabled={busy}>
                  {busy ? "Please wait…" : accountMode === "login" ? "Log in" : "Create account"}
                </button>
              </form>
            </>
          )}
          {error ? <p className="error">{error}</p> : null}
        </section>
      </Shell>
    );
  }

  if (view === "lookup") {
    return (
      <Shell>
        <section className="panel">
          <h1 className="section-title">Order status</h1>
          <p className="muted">
            Look up any order by id (guest), or sign in to see your order history.
          </p>
          <form
            className="lookup-form"
            onSubmit={(e) => {
              e.preventDefault();
              void fetchOrderStatus(lookupId, true);
            }}
          >
            <label className="field">
              Order id
              <input
                value={lookupId}
                onChange={(e) => setLookupId(e.target.value)}
                placeholder="UUID from checkout"
                autoComplete="off"
                inputMode="text"
              />
            </label>
            <button type="submit" className="btn" disabled={busy}>
              {busy ? "Looking up…" : "Check status"}
            </button>
          </form>
          {error ? <p className="error">{error}</p> : null}
          {order && !isOrderPaid(order) ? (
            <div className="status-box">
              <p>
                <strong>Order</strong> {order.id}
              </p>
              <p>{orderStatusMessage(order)}</p>
              <p className="muted">Total KES {order.total.toLocaleString()}</p>
              {!isOrderFailed(order) ? (
                <button type="button" className="btn" onClick={() => setView("pay")}>
                  Open payment screen
                </button>
              ) : (
                <button type="button" className="btn" onClick={goShop}>
                  Shop again
                </button>
              )}
            </div>
          ) : null}
          {session && accountOrders.length > 0 ? (
            <div className="recent">
              <h2>Your account orders</h2>
              <ul>
                {accountOrders.map((o) => (
                  <li key={o.id}>
                    <button
                      type="button"
                      className="linkish"
                      onClick={() => {
                        setLookupId(o.id);
                        void fetchOrderStatus(o.id, true);
                      }}
                    >
                      {o.id} — {o.status}
                    </button>
                  </li>
                ))}
              </ul>
            </div>
          ) : null}
          {recentIds.length > 0 ? (
            <div className="recent">
              <h2>Recent on this device</h2>
              <ul>
                {recentIds.map((id) => (
                  <li key={id}>
                    <button
                      type="button"
                      className="linkish"
                      onClick={() => {
                        setLookupId(id);
                        void fetchOrderStatus(id, true);
                      }}
                    >
                      {id}
                    </button>
                  </li>
                ))}
              </ul>
            </div>
          ) : null}
        </section>
      </Shell>
    );
  }

  if (view === "pay") {
    if (!order) {
      return (
        <Shell>
          <section className="panel">
            <p className="muted">No active payment. Look up an order or checkout again.</p>
            <div className="stack" style={{ justifyContent: "flex-start" }}>
              <button type="button" className="btn" onClick={goLookup}>
                Order lookup
              </button>
              <button type="button" className="btn btn-ghost" onClick={goShop}>
                Shop
              </button>
            </div>
          </section>
        </Shell>
      );
    }
    const failed = isOrderFailed(order);
    return (
      <Shell>
        <section className="pay">
          <div className="pay-card">
            <h2 className="section-title">
              {failed ? "Order not active" : payMethodUsed === "mpesa_stk" ? "Approve STK push" : "Complete M-Pesa payment"}
            </h2>
            <p className="muted">{orderStatusMessage(order)}</p>
            {!failed ? (
              <>
                {payMethodUsed === "mpesa_c2b" ? (
                  <>
                    <p className="muted">
                      Paybill {boot?.paybill || "—"} · KES {order.total.toLocaleString()}
                    </p>
                    <p className="ref">{payRef ?? "—"}</p>
                    <p className="muted">Use the account reference above. This page refreshes when payment confirms.</p>
                  </>
                ) : (
                  <>
                    <p className="muted">
                      Check your phone for the M-Pesa prompt · KES {order.total.toLocaleString()}
                    </p>
                    {payRef ? <p className="ref">{payRef}</p> : null}
                    <p className="muted">Waiting for STK confirmation…</p>
                  </>
                )}
              </>
            ) : null}
            {pollError ? <p className="error">{pollError}</p> : null}
            <p className="muted tiny">Order id: {order.id}</p>
            <div className="stack">
              <button
                type="button"
                className="btn"
                disabled={busy}
                onClick={() => void fetchOrderStatus(order.id, true)}
              >
                Refresh now
              </button>
              <button type="button" className="btn btn-ghost" onClick={goShop}>
                Back to shop
              </button>
              <button type="button" className="btn btn-ghost" onClick={goLookup}>
                Order lookup
              </button>
            </div>
          </div>
        </section>
      </Shell>
    );
  }

  if (view === "done") {
    if (!order) {
      return (
        <Shell>
          <section className="panel">
            <p className="muted">No completed order loaded.</p>
            <button type="button" className="btn" onClick={goLookup}>
              Order lookup
            </button>
          </section>
        </Shell>
      );
    }
    return (
      <Shell>
        <section className="done">
          <div className="done-card">
            <h2 className="section-title">Ready for pickup</h2>
            <p className="muted">Show this code at {pickup?.branch_name ?? boot?.branch_name ?? "the branch"}.</p>
            <p className="code">{order.collection_code ?? "…"}</p>
            <p className="muted">{orderStatusMessage(order)}</p>
            <p className="muted tiny">Order id: {order.id}</p>
            <div className="stack">
              <button
                type="button"
                className="btn"
                onClick={() => {
                  setOrder(null);
                  goShop();
                }}
              >
                Shop again
              </button>
              <button type="button" className="btn btn-ghost" onClick={goLookup}>
                Look up later
              </button>
            </div>
          </div>
        </section>
      </Shell>
    );
  }

  if (view === "product") {
    if (!selected) {
      return (
        <Shell>
          <section className="panel">
            <p className="muted">Product not found.</p>
            <button type="button" className="btn" onClick={() => setView("shop")}>
              Back to catalog
            </button>
          </section>
        </Shell>
      );
    }
    const qty = cart[selected.variant_id] ?? 0;
    return (
      <Shell>
        <section className="panel">
          <button type="button" className="linkish" onClick={() => setView("shop")}>
            ← Catalog
          </button>
          {selected.image_url ? (
            <img className="product-image" src={selected.image_url} alt={selected.product_name} />
          ) : null}
          <h1 className="section-title">{selected.product_name}</h1>
          <p className="muted">
            {selected.brand ? `${selected.brand} · ` : ""}
            SKU {selected.sku}
          </p>
          <p className="price">KES {selected.sell_price.toLocaleString()}</p>
          <p className="muted">{selected.available_qty} in stock at {pickup?.location_name ?? "counter"}</p>
          {selected.description ? <p className="muted">{selected.description}</p> : null}
          {error ? <p className="error">{error}</p> : null}
          <div className="qty-row">
            <button
              type="button"
              className="btn btn-ghost qty-btn"
              disabled={qty <= 0}
              aria-label="Decrease quantity"
              onClick={() => setQty(selected.variant_id, qty - 1, selected.available_qty)}
            >
              −
            </button>
            <span aria-live="polite">{qty}</span>
            <button
              type="button"
              className="btn btn-ghost qty-btn"
              disabled={qty >= selected.available_qty || selected.available_qty <= 0}
              aria-label="Increase quantity"
              onClick={() => addOne(selected)}
            >
              +
            </button>
            <button
              type="button"
              className="btn"
              disabled={selected.available_qty <= 0}
              onClick={() => addOne(selected)}
            >
              Add to bag
            </button>
          </div>
          {qty > 0 ? (
            <button type="button" className="btn btn-ghost" onClick={() => setView("checkout")}>
              View bag
            </button>
          ) : null}
        </section>
      </Shell>
    );
  }

  if (view === "checkout") {
    return (
      <Shell>
        <section className="panel checkout">
          <h1 className="section-title">Checkout</h1>
          <BranchPicker />
          {session ? <p className="muted tiny">Checking out as {session.customer.full_name}</p> : null}
          {lines.length === 0 ? (
            <div className="empty-state">
              <strong>Your bag is empty</strong>
              <p>Add products from the shop, then come back to check out.</p>
            </div>
          ) : (
            <ul className="cart-list">
              {lines.map(({ item, qty }) => (
                <li key={item.variant_id} className="cart-line">
                  <div>
                    <div>{item.product_name}</div>
                    <div className="muted tiny">KES {item.sell_price.toLocaleString()} each</div>
                  </div>
                  <div className="qty-row compact">
                    <button
                      type="button"
                      className="btn btn-ghost qty-btn"
                      aria-label={`Decrease ${item.product_name}`}
                      onClick={() => setQty(item.variant_id, qty - 1, item.available_qty)}
                    >
                      −
                    </button>
                    <span>{qty}</span>
                    <button
                      type="button"
                      className="btn btn-ghost qty-btn"
                      aria-label={`Increase ${item.product_name}`}
                      disabled={qty >= item.available_qty}
                      onClick={() => setQty(item.variant_id, qty + 1, item.available_qty)}
                    >
                      +
                    </button>
                    <button
                      type="button"
                      className="linkish"
                      onClick={() => setQty(item.variant_id, 0, item.available_qty)}
                    >
                      Remove
                    </button>
                  </div>
                  <div className="line-total">KES {(item.sell_price * qty).toLocaleString()}</div>
                </li>
              ))}
            </ul>
          )}
          <div className="total">
            <span>Total</span>
            <span>KES {total.toLocaleString()}</span>
          </div>

          <fieldset className="pay-methods" disabled={lines.length === 0}>
            <legend>Payment</legend>
            <label className="radio">
              <input
                type="radio"
                name="method"
                checked={method === "mpesa_c2b"}
                onChange={() => setMethod("mpesa_c2b")}
              />
              M-Pesa paybill (C2B)
            </label>
            <label className="radio">
              <input
                type="radio"
                name="method"
                checked={method === "mpesa_stk"}
                onChange={() => setMethod("mpesa_stk")}
              />
              M-Pesa STK push
            </label>
            {method === "mpesa_stk" ? (
              <label className="field">
                Phone (07… or 2547…)
                <input
                  value={phone}
                  onChange={(e) => setPhone(e.target.value)}
                  placeholder="07XXXXXXXX"
                  inputMode="tel"
                  autoComplete="tel"
                />
              </label>
            ) : (
              <p className="muted tiny">
                After checkout you will pay paybill {boot?.paybill || "—"} with an ORD-… account reference.
              </p>
            )}
          </fieldset>

          {error ? <p className="error">{error}</p> : null}
          <button
            type="button"
            className="btn"
            style={{ width: "100%" }}
            disabled={busy || lines.length === 0}
            onClick={() => void placeOrder()}
          >
            {busy ? "Reserving stock…" : method === "mpesa_stk" ? "Pay with STK" : "Checkout with paybill"}
          </button>
          <button type="button" className="btn btn-ghost" style={{ width: "100%", marginTop: "0.5rem" }} onClick={goShop}>
            Continue shopping
          </button>
        </section>
      </Shell>
    );
  }

  // shop (default)
  return (
    <Shell>
      <div className="shop">
        <section>
          <h1 className="section-title">Catalog</h1>
          <BranchPicker />
          <div className="filters">
            <label className="field grow">
              Search
              <input
                value={query}
                onChange={(e) => setQuery(e.target.value)}
                placeholder="Name, brand, or SKU"
                inputMode="search"
              />
            </label>
            <label className="field">
              Brand
              <select value={brandFilter} onChange={(e) => setBrandFilter(e.target.value)}>
                <option value="">All brands</option>
                {brands.map((b) => (
                  <option key={b} value={b}>
                    {b}
                  </option>
                ))}
              </select>
            </label>
            <label className="field">
              Category
              <select value={categoryFilter} onChange={(e) => setCategoryFilter(e.target.value)}>
                <option value="">All categories</option>
                {categories.map((c) => (
                  <option key={c} value={c}>
                    {c}
                  </option>
                ))}
              </select>
            </label>
          </div>
          {error ? <p className="error">{error}</p> : null}
          {loadingShop ? (
            <div className="catalog" style={{ marginTop: "1rem", gap: "0.75rem" }}>
              <div className="skeleton" />
              <div className="skeleton" />
              <div className="skeleton" />
            </div>
          ) : null}
          <div className="catalog" style={{ marginTop: "1rem" }}>
            {!loadingShop && filtered.length === 0 ? (
              <div className="empty-state">
                <strong>{catalog.length === 0 ? "No products yet" : "No matches"}</strong>
                <p>
                  {catalog.length === 0
                    ? "Online catalog is empty for this branch."
                    : "Try clearing filters or searching a different term."}
                </p>
              </div>
            ) : (
              filtered.map((item, i) => {
                const qty = cart[item.variant_id] ?? 0;
                return (
                  <div key={item.variant_id} className="product" style={{ animationDelay: `${i * 30}ms` }}>
                    <button
                      type="button"
                      className="product-main"
                      onClick={() => {
                        setSelectedId(item.variant_id);
                        setView("product");
                      }}
                    >
                      {item.image_url ? (
                        <img className="thumb" src={item.image_url} alt="" />
                      ) : null}
                      <div>
                        <h3>{item.product_name}</h3>
                        <div className="meta">
                          {item.category ? `${item.category} · ` : ""}
                          {item.brand ? `${item.brand} · ` : ""}
                          KES {item.sell_price.toLocaleString()} · {item.available_qty} in stock
                        </div>
                      </div>
                    </button>
                    <div className="product-actions">
                      {qty > 0 ? (
                        <div className="qty-row compact">
                          <button
                            type="button"
                            className="btn btn-ghost qty-btn"
                            aria-label={`Decrease ${item.product_name}`}
                            onClick={() => setQty(item.variant_id, qty - 1, item.available_qty)}
                          >
                            −
                          </button>
                          <span>{qty}</span>
                          <button
                            type="button"
                            className="btn btn-ghost qty-btn"
                            aria-label={`Increase ${item.product_name}`}
                            disabled={qty >= item.available_qty}
                            onClick={() => addOne(item)}
                          >
                            +
                          </button>
                        </div>
                      ) : (
                        <button
                          type="button"
                          className="btn"
                          disabled={item.available_qty <= 0 || busy}
                          onClick={() => addOne(item)}
                        >
                          Add
                        </button>
                      )}
                    </div>
                  </div>
                );
              })
            )}
          </div>
        </section>

        <aside className="cart-panel">
          <h2>Your bag</h2>
          {lines.length === 0 ? (
            <p className="muted">Add accessories to reserve stock at checkout.</p>
          ) : (
            lines.map(({ item, qty }) => (
              <div key={item.variant_id} className="cart-line">
                <span>
                  {item.product_name} × {qty}
                </span>
                <span>KES {(item.sell_price * qty).toLocaleString()}</span>
              </div>
            ))
          )}
          <div className="total">
            <span>Total</span>
            <span>KES {total.toLocaleString()}</span>
          </div>
          <button
            type="button"
            className="btn"
            style={{ width: "100%" }}
            disabled={lines.length === 0}
            onClick={() => {
              setError("");
              setView("checkout");
            }}
          >
            Checkout
          </button>
        </aside>
      </div>
    </Shell>
  );
}
