import { useEffect, useMemo, useState } from "react";
import { Link, useLocation, useNavigate, useParams } from "react-router-dom";
import { useStorefront } from "../store/StorefrontContext";
import { getOrder, orderStatusMessage, type Order } from "../lib/api";

export function DonePage() {
  const { orderId } = useParams();
  const navigate = useNavigate();
  const location = useLocation();
  const justPaid = Boolean((location.state as { justPaid?: boolean } | null)?.justPaid);
  const { pickup, boot, branches } = useStorefront();
  const [order, setOrder] = useState<Order | null>(null);
  const [error, setError] = useState("");

  useEffect(() => {
    if (!orderId) return;
    getOrder(orderId)
      .then(setOrder)
      .catch((e) => setError(e instanceof Error ? e.message : "Order not found"));
  }, [orderId]);

  const branch = useMemo(() => {
    const id = order?.branch_id ?? pickup?.branch_id;
    return branches.find((b) => b.id === id) ?? null;
  }, [branches, order?.branch_id, pickup?.branch_id]);

  const branchName = branch?.name ?? pickup?.branch_name ?? boot?.branch_name ?? "the branch";

  if (error) {
    return (
      <section className="wide-page">
        <div className="page-header">
          <h1>Order</h1>
          <ol className="breadcrumb">
            <li>
              <Link to="/">Home</Link>
            </li>
            <li className="active">Done</li>
          </ol>
        </div>
        <div className="li-container page-body">
          <p className="error">{error}</p>
          <button type="button" className="btn" onClick={() => navigate("/lookup")}>
            Order lookup
          </button>
        </div>
      </section>
    );
  }

  if (!order) {
    return (
      <section className="wide-page">
        <div className="page-header">
          <h1>Order</h1>
          <ol className="breadcrumb">
            <li>
              <Link to="/">Home</Link>
            </li>
            <li className="active">Done</li>
          </ol>
        </div>
        <div className="li-container page-body">
          <p className="muted">Loading…</p>
        </div>
      </section>
    );
  }

  return (
    <section className="wide-page done">
      <div className="page-header">
        <h1>{justPaid ? "Payment successful" : "Ready for pickup"}</h1>
        <ol className="breadcrumb">
          <li>
            <Link to="/">Home</Link>
          </li>
          <li className="active">Done</li>
        </ol>
      </div>
      <div className="done-card">
        {justPaid ? <p className="ok">Payment received — thank you.</p> : null}
        <h2 className="section-title">Ready for pickup</h2>
        <p className="muted">Show this code at {branchName}.</p>
        <p className="code">{order.collection_code ?? "…"}</p>
        {branch?.address ? <p className="muted">{branch.address}</p> : null}
        {branch?.phone || branch?.hours ? (
          <p className="muted tiny">
            {[branch.phone, branch.hours].filter(Boolean).join(" · ")}
          </p>
        ) : null}
        <p className="muted">{orderStatusMessage(order)}</p>
        <p className="muted tiny">Order id: {order.id}</p>
        <div className="stack">
          <button type="button" className="btn" onClick={() => navigate("/shop")}>
            Shop again
          </button>
          <button type="button" className="btn btn-ghost" onClick={() => navigate("/lookup")}>
            Look up later
          </button>
        </div>
      </div>
    </section>
  );
}
