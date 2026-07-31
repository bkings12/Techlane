import { useEffect, useMemo, useState } from "react";
import { Link, useLocation, useNavigate, useParams } from "react-router-dom";
import { useStorefront } from "../store/StorefrontContext";
import { getOrder, isOrderFailed, isOrderPaid, orderStatusMessage, type Order } from "../lib/api";

type PayMethod = "mpesa_c2b" | "mpesa_stk" | "cash_on_pickup" | "cash";
type NavState = { payRef?: string; payMethod?: PayMethod };

export function PayPage() {
  const { orderId } = useParams();
  const navigate = useNavigate();
  const location = useLocation();
  const { boot, pickup, branches } = useStorefront();
  const navState = (location.state as NavState | null) ?? null;

  const [order, setOrder] = useState<Order | null>(null);
  const [payRef] = useState(navState?.payRef);
  const [payMethod] = useState<PayMethod>(navState?.payMethod ?? "mpesa_stk");
  const [busy, setBusy] = useState(true);
  const [pollError, setPollError] = useState("");
  const [waiting, setWaiting] = useState(false);

  const cashPickup = payMethod === "cash_on_pickup" || payMethod === "cash";
  const branch = useMemo(() => {
    const id = order?.branch_id ?? pickup?.branch_id;
    return branches.find((b) => b.id === id) ?? null;
  }, [branches, order?.branch_id, pickup?.branch_id]);

  async function refresh() {
    if (!orderId) return;
    try {
      const o = await getOrder(orderId);
      setOrder(o);
      setPollError("");
      if (isOrderPaid(o)) {
        setWaiting(false);
        navigate(`/done/${o.id}`, { replace: true, state: { justPaid: true } });
      }
    } catch (e) {
      setPollError(e instanceof Error ? e.message : "Could not refresh order");
    } finally {
      setBusy(false);
    }
  }

  useEffect(() => {
    void refresh();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [orderId]);

  useEffect(() => {
    if (!order || cashPickup || isOrderPaid(order) || isOrderFailed(order)) {
      setWaiting(false);
      return;
    }
    setWaiting(true);
    const t = window.setInterval(() => void refresh(), 2500);
    return () => window.clearInterval(t);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [order?.status, cashPickup]);

  if (busy && !order) {
    return (
      <section className="wide-page">
        <div className="page-header">
          <h1>Payment</h1>
          <ol className="breadcrumb">
            <li>
              <Link to="/">Home</Link>
            </li>
            <li className="active">Pay</li>
          </ol>
        </div>
        <div className="li-container page-body">
          <p className="muted">Loading order…</p>
        </div>
      </section>
    );
  }

  if (!order) {
    return (
      <section className="wide-page">
        <div className="page-header">
          <h1>Payment</h1>
          <ol className="breadcrumb">
            <li>
              <Link to="/">Home</Link>
            </li>
            <li className="active">Pay</li>
          </ol>
        </div>
        <div className="li-container page-body">
          <p className="muted">No active payment. Look up an order or checkout again.</p>
          <div className="stack" style={{ justifyContent: "flex-start" }}>
            <button type="button" className="btn" onClick={() => navigate("/lookup")}>
              Order lookup
            </button>
            <button type="button" className="btn btn-ghost" onClick={() => navigate("/shop")}>
              Shop
            </button>
          </div>
        </div>
      </section>
    );
  }

  const failed = isOrderFailed(order);
  const title = failed
    ? "Order not active"
    : cashPickup
      ? "Reserved — pay cash at pickup"
      : payMethod === "mpesa_stk"
        ? "Approve STK push"
        : "Complete M-Pesa payment";

  return (
    <section className="wide-page pay">
      {waiting && !failed && !cashPickup ? (
        <div className="stk-wait-overlay" role="status" aria-live="polite">
          <div className="stk-wait-card">
            <div className="stk-wait-spinner" aria-hidden />
            <strong>Please wait</strong>
            <p className="muted">
              {payMethod === "mpesa_stk"
                ? "Enter your M-Pesa PIN on the phone. This page updates when payment succeeds."
                : "Complete the paybill payment. This page updates when we confirm it."}
            </p>
            <p className="ref">KES {order.total.toLocaleString()}</p>
          </div>
        </div>
      ) : null}
      <div className="page-header">
        <h1>Payment</h1>
        <ol className="breadcrumb">
          <li>
            <Link to="/">Home</Link>
          </li>
          <li className="active">Pay</li>
        </ol>
      </div>
      <div className="pay-card">
        <h2 className="section-title">{title}</h2>
        <p className="muted">{orderStatusMessage(order, { cashOnPickup: cashPickup })}</p>
        {!failed && cashPickup ? (
          <>
            <p className="muted">
              Bring this order to {branch?.name ?? pickup?.branch_name ?? "the branch"} and pay{" "}
              <strong>KES {order.total.toLocaleString()}</strong> in cash at the counter.
            </p>
            {branch?.address || pickup ? (
              <p className="muted">
                {[branch?.address, branch?.phone, branch?.hours].filter(Boolean).join(" · ") ||
                  `${pickup?.branch_name ?? ""} · ${pickup?.location_name ?? ""}`}
              </p>
            ) : null}
            {branch?.map_url ? (
              <p>
                <a className="btn btn-ghost" href={branch.map_url} target="_blank" rel="noreferrer">
                  Open map
                </a>
              </p>
            ) : null}
            <p className="muted tiny">Staff will confirm payment when you collect. Delivery cash is not offered.</p>
          </>
        ) : null}
        {!failed && !cashPickup ? (
          payMethod === "mpesa_c2b" ? (
            <>
              <p className="muted">
                Paybill {boot?.paybill || "—"} · KES {order.total.toLocaleString()}
              </p>
              <p className="ref">{payRef ?? "—"}</p>
              <p className="muted">Use the account reference above. This page refreshes when payment confirms.</p>
            </>
          ) : (
            <>
              <p className="muted">Check your phone for the M-Pesa prompt · KES {order.total.toLocaleString()}</p>
              {payRef ? <p className="ref">{payRef}</p> : null}
              <p className="muted">Waiting for STK confirmation…</p>
            </>
          )
        ) : null}
        {pollError ? <p className="error">{pollError}</p> : null}
        <p className="muted tiny">Order id: {order.id}</p>
        <div className="stack">
          {!cashPickup ? (
            <button type="button" className="btn" disabled={busy} onClick={() => void refresh()}>
              Check payment now
            </button>
          ) : (
            <button type="button" className="btn" onClick={() => navigate(`/lookup`)}>
              Track this order
            </button>
          )}
          <button type="button" className="btn btn-ghost" onClick={() => navigate("/shop")}>
            Back to shop
          </button>
          <button type="button" className="btn btn-ghost" onClick={() => navigate("/lookup")}>
            Order lookup
          </button>
        </div>
      </div>
    </section>
  );
}
