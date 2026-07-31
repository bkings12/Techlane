import { useMemo, useState, type FormEvent } from "react";
import { Link, useNavigate } from "react-router-dom";
import { useStorefront } from "../store/StorefrontContext";
import { PasswordField } from "../components/PasswordField";
import type { AccountRepair, Order } from "../lib/api";

const orderStatusLabels: Record<string, string> = {
  pending_payment: "Awaiting payment",
  payment_failed: "Payment failed",
  confirmed: "Confirmed",
  ready_for_pickup: "Ready for pickup",
  delivered: "Delivered",
  collected: "Collected",
  cancelled: "Cancelled",
  expired: "Expired",
};

const repairStatusLabels: Record<string, string> = {
  intake: "Checked in",
  diagnosed: "Diagnosed",
  waiting_parts: "Waiting for parts",
  in_progress: "In progress",
  ready_for_pickup: "Ready for collection",
  completed: "Complete",
  ready: "Ready for collection",
  collected: "Collected",
  cancelled: "Cancelled",
  unrepairable: "Cannot be repaired",
};

function labelStatus(map: Record<string, string>, status: string) {
  return map[status] ?? status.replace(/_/g, " ");
}

function statusTone(status: string): "ok" | "warn" | "bad" | "muted" {
  if (["ready_for_pickup", "confirmed", "delivered", "collected", "completed", "ready"].includes(status)) {
    return "ok";
  }
  if (["payment_failed", "cancelled", "expired", "unrepairable"].includes(status)) return "bad";
  if (["pending_payment", "intake", "diagnosed", "waiting_parts", "in_progress"].includes(status)) return "warn";
  return "muted";
}

function initials(name: string) {
  const parts = name.trim().split(/\s+/).filter(Boolean);
  if (parts.length === 0) return "?";
  if (parts.length === 1) return parts[0]!.slice(0, 2).toUpperCase();
  return `${parts[0]![0] ?? ""}${parts[1]![0] ?? ""}`.toUpperCase();
}

function formatWhen(iso?: string) {
  if (!iso) return "";
  const d = new Date(iso);
  if (Number.isNaN(d.getTime())) return "";
  return d.toLocaleDateString(undefined, { day: "numeric", month: "short", year: "numeric" });
}

function orderRef(id: string) {
  return `ORD-${id.replace(/-/g, "").slice(0, 8).toUpperCase()}`;
}

export function AccountPage() {
  const navigate = useNavigate();
  const { session, accountOrders, accountRepairs, login, register, logout, formatPrice, boot, content } =
    useStorefront();
  const [mode, setMode] = useState<"login" | "register">("login");
  const [tab, setTab] = useState<"orders" | "repairs">("orders");
  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");
  const [name, setName] = useState("");
  const [phone, setPhone] = useState("");
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");

  const shopName =
    (content?.settings?.shop_display_name || boot?.tenant_name || "TechLane").trim() || "TechLane";

  const stats = useMemo(() => {
    const openOrders = accountOrders.filter((o) =>
      ["pending_payment", "confirmed", "ready_for_pickup"].includes(o.status),
    ).length;
    const activeRepairs = accountRepairs.filter((r) =>
      !["collected", "cancelled", "completed", "unrepairable"].includes(r.status),
    ).length;
    const ready = [
      ...accountOrders.filter((o) => o.status === "ready_for_pickup"),
      ...accountRepairs.filter((r) => r.status === "ready_for_pickup" || r.status === "ready"),
    ].length;
    return { openOrders, activeRepairs, ready, orderCount: accountOrders.length, repairCount: accountRepairs.length };
  }, [accountOrders, accountRepairs]);

  async function submit(e: FormEvent) {
    e.preventDefault();
    setBusy(true);
    setError("");
    try {
      if (mode === "register") {
        await register({
          full_name: name.trim(),
          email: email.trim(),
          phone: phone.trim() || undefined,
          password,
        });
      } else {
        await login(email.trim(), password);
      }
      setPassword("");
    } catch (err) {
      setError(err instanceof Error ? err.message : "Account action failed");
    } finally {
      setBusy(false);
    }
  }

  return (
    <section className="wide-page account-page">
      <div className="page-header">
        <h1>{session ? "My account" : "Account"}</h1>
        <ol className="breadcrumb">
          <li>
            <Link to="/">Home</Link>
          </li>
          <li className="active">Account</li>
        </ol>
      </div>

      <div className="li-container page-body">
        {session ? (
          <div className="account-dash">
            <header className="account-hero">
              <div className="account-hero-main">
                <div className="account-avatar" aria-hidden="true">
                  {initials(session.customer.full_name)}
                </div>
                <div className="account-hero-copy">
                  <p className="account-kicker">Welcome back</p>
                  <h2>{session.customer.full_name}</h2>
                  <p className="account-meta">
                    <span>{session.customer.email}</span>
                    {session.customer.phone ? <span>{session.customer.phone}</span> : null}
                  </p>
                </div>
              </div>
              <div className="account-hero-actions">
                <Link className="btn" to="/shop">
                  Continue shopping
                </Link>
                <Link className="btn btn-ghost account-btn-light" to="/lookup">
                  Track order / repair
                </Link>
                <button type="button" className="btn btn-ghost account-btn-light" onClick={logout}>
                  Sign out
                </button>
              </div>
            </header>

            <div className="account-stats" role="list">
              <div className="account-stat" role="listitem">
                <span className="account-stat-value">{stats.orderCount}</span>
                <span className="account-stat-label">Orders</span>
              </div>
              <div className="account-stat" role="listitem">
                <span className="account-stat-value">{stats.repairCount}</span>
                <span className="account-stat-label">Repairs</span>
              </div>
              <div className="account-stat account-stat-accent" role="listitem">
                <span className="account-stat-value">{stats.ready}</span>
                <span className="account-stat-label">Ready to collect</span>
              </div>
              <div className="account-stat" role="listitem">
                <span className="account-stat-value">{stats.openOrders + stats.activeRepairs}</span>
                <span className="account-stat-label">In progress</span>
              </div>
            </div>

            <div className="account-panels">
              <div className="account-panel">
                <div className="account-tabs" role="tablist" aria-label="Account activity">
                  <button
                    type="button"
                    role="tab"
                    aria-selected={tab === "orders"}
                    className={tab === "orders" ? "is-active" : ""}
                    onClick={() => setTab("orders")}
                  >
                    Orders
                    <span className="account-tab-count">{accountOrders.length}</span>
                  </button>
                  <button
                    type="button"
                    role="tab"
                    aria-selected={tab === "repairs"}
                    className={tab === "repairs" ? "is-active" : ""}
                    onClick={() => setTab("repairs")}
                  >
                    Repairs
                    <span className="account-tab-count">{accountRepairs.length}</span>
                  </button>
                </div>

                {tab === "orders" ? (
                  <OrdersPanel
                    orders={accountOrders}
                    formatPrice={formatPrice}
                    onOpen={(id) => navigate(`/lookup/${id}`)}
                  />
                ) : (
                  <RepairsPanel
                    repairs={accountRepairs}
                    onOpen={(code) => navigate(`/lookup?q=${encodeURIComponent(code)}`)}
                  />
                )}
              </div>

              <aside className="account-side">
                <div className="account-side-card">
                  <h3>Quick actions</h3>
                  <Link className="account-side-link" to="/shop">
                    <span>Browse shop</span>
                    <span aria-hidden="true">→</span>
                  </Link>
                  <Link className="account-side-link" to="/lookup">
                    <span>Track with phone or job code</span>
                    <span aria-hidden="true">→</span>
                  </Link>
                  <Link className="account-side-link" to="/cart">
                    <span>View cart</span>
                    <span aria-hidden="true">→</span>
                  </Link>
                  <Link className="account-side-link" to="/contact">
                    <span>Contact {shopName}</span>
                    <span aria-hidden="true">→</span>
                  </Link>
                </div>
                <div className="account-side-card account-side-tip">
                  <h3>Link a repair</h3>
                  <p>
                    Register with the same phone used on your job slip, or track with the job code from your intake
                    receipt.
                  </p>
                </div>
              </aside>
            </div>
          </div>
        ) : (
          <div className="account-auth">
            <div className="account-auth-brand">
              <p className="account-kicker">Customer account</p>
              <h2>Orders, repairs, and pickup — in one place</h2>
              <p>
                Sign in to follow online orders and repair jobs from {shopName}. Guests can still track anytime with an
                order number, job code, or phone.
              </p>
              <ul className="account-auth-points">
                <li>See order and repair status</li>
                <li>Know when something is ready for collection</li>
                <li>Track without hunting for SMS threads</li>
              </ul>
              <Link className="account-auth-track" to="/lookup">
                Track as guest →
              </Link>
            </div>

            <div className="account-auth-card">
              <div className="account-auth-modes" role="tablist" aria-label="Sign in or register">
                <button
                  type="button"
                  role="tab"
                  aria-selected={mode === "login"}
                  className={mode === "login" ? "is-active" : ""}
                  onClick={() => setMode("login")}
                >
                  Log in
                </button>
                <button
                  type="button"
                  role="tab"
                  aria-selected={mode === "register"}
                  className={mode === "register" ? "is-active" : ""}
                  onClick={() => setMode("register")}
                >
                  Register
                </button>
              </div>

              <form className="account-auth-form" onSubmit={(e) => void submit(e)}>
                {mode === "register" ? (
                  <>
                    <label className="field">
                      Full name
                      <input value={name} onChange={(e) => setName(e.target.value)} required autoComplete="name" />
                    </label>
                    <label className="field">
                      Phone
                      <input
                        value={phone}
                        onChange={(e) => setPhone(e.target.value)}
                        inputMode="tel"
                        autoComplete="tel"
                        placeholder="07…"
                      />
                    </label>
                  </>
                ) : null}
                <label className="field">
                  Email
                  <input
                    type="email"
                    value={email}
                    onChange={(e) => setEmail(e.target.value)}
                    required
                    autoComplete="email"
                  />
                </label>
                <label className="field">
                  Password
                  <PasswordField
                    value={password}
                    onChange={setPassword}
                    required
                    minLength={8}
                    autoComplete={mode === "login" ? "current-password" : "new-password"}
                  />
                </label>
                {mode === "register" ? (
                  <p className="account-auth-hint">Use the phone on your repair slip so jobs show up here.</p>
                ) : null}
                <button type="submit" className="btn account-auth-submit" disabled={busy}>
                  {busy ? "Please wait…" : mode === "login" ? "Log in" : "Create account"}
                </button>
              </form>
              {error ? <p className="error">{error}</p> : null}
            </div>
          </div>
        )}
      </div>
    </section>
  );
}

function OrdersPanel({
  orders,
  formatPrice,
  onOpen,
}: {
  orders: Order[];
  formatPrice: (n: number) => string;
  onOpen: (id: string) => void;
}) {
  if (orders.length === 0) {
    return (
      <div className="account-empty">
        <strong>No orders yet</strong>
        <p>When you check out online, your orders will appear here.</p>
        <Link className="btn" to="/shop">
          Start shopping
        </Link>
      </div>
    );
  }

  return (
    <ul className="account-list">
      {orders.map((o, i) => {
        const tone = statusTone(o.status);
        return (
          <li key={o.id} style={{ animationDelay: `${Math.min(i, 8) * 40}ms` }}>
            <button type="button" className="account-item" onClick={() => onOpen(o.id)}>
              <div className="account-item-top">
                <span className="account-item-ref">{orderRef(o.id)}</span>
                <span className={`account-pill tone-${tone}`}>{labelStatus(orderStatusLabels, o.status)}</span>
              </div>
              <div className="account-item-body">
                <strong>{formatPrice(o.total)}</strong>
                <span className="muted">
                  {o.fulfilment_type === "delivery" ? "Delivery" : "Branch pickup"}
                  {o.created_at ? ` · ${formatWhen(o.created_at)}` : ""}
                </span>
              </div>
              <span className="account-item-cta">
                View status <span aria-hidden="true">→</span>
              </span>
            </button>
          </li>
        );
      })}
    </ul>
  );
}

function RepairsPanel({
  repairs,
  onOpen,
}: {
  repairs: AccountRepair[];
  onOpen: (code: string) => void;
}) {
  if (repairs.length === 0) {
    return (
      <div className="account-empty">
        <strong>No repairs linked</strong>
        <p>Use the same phone as your job slip, or track with the job code from intake.</p>
        <Link className="btn" to="/lookup">
          Track a repair
        </Link>
      </div>
    );
  }

  return (
    <ul className="account-list">
      {repairs.map((r, i) => {
        const tone = statusTone(r.status);
        return (
          <li key={r.id} style={{ animationDelay: `${Math.min(i, 8) * 40}ms` }}>
            <button type="button" className="account-item" onClick={() => onOpen(r.job_code)}>
              <div className="account-item-top">
                <span className="account-item-ref">{r.job_code}</span>
                <span className={`account-pill tone-${tone}`}>{labelStatus(repairStatusLabels, r.status)}</span>
              </div>
              <div className="account-item-body">
                <strong>{r.problem_summary?.trim() || "Repair job"}</strong>
                <span className="muted">{formatWhen(r.created_at)}</span>
              </div>
              <span className="account-item-cta">
                View progress <span aria-hidden="true">→</span>
              </span>
            </button>
          </li>
        );
      })}
    </ul>
  );
}
