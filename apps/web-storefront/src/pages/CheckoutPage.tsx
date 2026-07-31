import { useMemo, useState } from "react";
import { Link, useNavigate } from "react-router-dom";
import { useStorefront } from "../store/StorefrontContext";
import { catalogItemImageURL, checkout } from "../lib/api";
import { rememberOrderId } from "../lib/storage";

type PayMethod = "mpesa_stk" | "mpesa_c2b" | "cash_on_pickup";
type Fulfilment = "branch_pickup" | "delivery";

const DETAILS_KEY = "techlane.storefront.checkoutDetails";

type CheckoutDetails = {
  fullName: string;
  email: string;
  phone: string;
  notes: string;
  fulfilment: Fulfilment;
  deliveryLocationId: string;
  addressLine1: string;
  addressLine2: string;
  landmark: string;
};

function loadDetails(fallback: Partial<CheckoutDetails>): CheckoutDetails {
  try {
    const raw = localStorage.getItem(DETAILS_KEY);
    if (raw) {
      const parsed = JSON.parse(raw) as Partial<CheckoutDetails>;
      return {
        fullName: parsed.fullName ?? fallback.fullName ?? "",
        email: parsed.email ?? fallback.email ?? "",
        phone: parsed.phone ?? fallback.phone ?? "",
        notes: parsed.notes ?? "",
        fulfilment: parsed.fulfilment === "delivery" ? "delivery" : "branch_pickup",
        deliveryLocationId: parsed.deliveryLocationId ?? "",
        addressLine1: parsed.addressLine1 ?? "",
        addressLine2: parsed.addressLine2 ?? "",
        landmark: parsed.landmark ?? "",
      };
    }
  } catch {
    /* ignore */
  }
  return {
    fullName: fallback.fullName ?? "",
    email: fallback.email ?? "",
    phone: fallback.phone ?? "",
    notes: "",
    fulfilment: "branch_pickup",
    deliveryLocationId: "",
    addressLine1: "",
    addressLine2: "",
    landmark: "",
  };
}

export function CheckoutPage() {
  const navigate = useNavigate();
  const { lines, total, pickup, branches, changeBranch, session, clearCart, formatPrice, boot, content, cartCount } =
    useStorefront();
  const [method, setMethod] = useState<PayMethod>("mpesa_stk");
  const [details, setDetails] = useState<CheckoutDetails>(() =>
    loadDetails({
      fullName: session?.customer.full_name ?? "",
      email: session?.customer.email ?? "",
      phone: session?.customer.phone ?? "",
    }),
  );
  const [accepted, setAccepted] = useState(true);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");

  const deliveryLocations = boot?.delivery_locations ?? [];
  const deliveryAvailable = deliveryLocations.length > 0;
  const isDelivery = details.fulfilment === "delivery" && deliveryAvailable;
  const selectedDelivery = useMemo(
    () => deliveryLocations.find((l) => l.id === details.deliveryLocationId) ?? null,
    [deliveryLocations, details.deliveryLocationId],
  );
  const deliveryFee = isDelivery ? selectedDelivery?.fee ?? 0 : 0;
  const grandTotal = total + deliveryFee;

  const pay = content?.settings;
  const labelSTK = pay?.pay_label_stk?.trim() || "M-Pesa STK push";
  const labelPaybill = pay?.pay_label_paybill?.trim() || "M-Pesa paybill";
  const labelCash = isDelivery
    ? pay?.pay_label_cash?.trim() || "Cash on delivery"
    : pay?.pay_label_cash?.trim() || "Cash on pickup";
  const hintSTK = pay?.pay_hint_stk?.trim() || "We send an STK prompt to this number.";
  const hintPaybill =
    pay?.pay_hint_paybill?.trim() || "Pay by paybill after checkout with an ORD-… account reference.";
  const hintCash = isDelivery
    ? "Stock is held while we arrange delivery. Pay cash when your order arrives."
    : pay?.pay_hint_cash?.trim() || "Stock is held for pickup. Pay cash at the branch counter.";
  const ctaSTK = pay?.pay_cta_stk?.trim() || "Pay with M-Pesa STK";
  const ctaPaybill = pay?.pay_cta_paybill?.trim() || "Place order · Paybill";
  const ctaCash = isDelivery
    ? pay?.pay_cta_cash?.trim() || "Place order · Cash on delivery"
    : pay?.pay_cta_cash?.trim() || "Place order · Cash pickup";

  const selected = useMemo(
    () => branches.find((b) => b.id === pickup?.branch_id) ?? branches[0] ?? null,
    [branches, pickup?.branch_id],
  );

  function updateDetail<K extends keyof CheckoutDetails>(key: K, value: CheckoutDetails[K]) {
    setDetails((prev) => {
      const next = { ...prev, [key]: value };
      try {
        localStorage.setItem(DETAILS_KEY, JSON.stringify(next));
      } catch {
        /* ignore */
      }
      return next;
    });
  }

  async function placeOrder() {
    if (!pickup || lines.length === 0) return;
    const fullName = details.fullName.trim();
    const phone = details.phone.trim();
    if (!fullName) {
      setError("Full name is required");
      return;
    }
    if (!phone) {
      setError("Phone number is required for payment and contact");
      return;
    }
    if (method === "mpesa_stk" && phone.length < 9) {
      setError("Enter a valid M-Pesa phone (07… or 2547…)");
      return;
    }
    if (isDelivery && !details.deliveryLocationId) {
      setError("Choose a delivery location");
      return;
    }
    if (!accepted) {
      setError("Please accept the terms to continue");
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
          phone,
          fulfilment_type: isDelivery ? "delivery" : "branch_pickup",
          customer_name: fullName,
          customer_email: details.email.trim() || undefined,
          customer_notes: details.notes.trim() || undefined,
          delivery_location_id: isDelivery ? details.deliveryLocationId : undefined,
          delivery_address_line1: isDelivery ? details.addressLine1.trim() || undefined : undefined,
          delivery_address_line2: isDelivery ? details.addressLine2.trim() || undefined : undefined,
          delivery_landmark: isDelivery ? details.landmark.trim() || undefined : undefined,
          items: lines.map((l) => ({ variant_id: l.item.variant_id, quantity: l.qty })),
        },
        session?.token,
      );
      clearCart();
      rememberOrderId(res.order.id);
      navigate(`/pay/${res.order.id}`, {
        state: {
          payRef: res.payment?.account_reference,
          payMethod: res.payment?.method || method,
          customerName: fullName,
          customerPhone: phone,
          customerEmail: details.email.trim() || undefined,
          orderNotes: details.notes.trim() || undefined,
          fulfilment: isDelivery ? "delivery" : "branch_pickup",
        },
      });
    } catch (e) {
      setError(e instanceof Error ? e.message : "Checkout failed");
    } finally {
      setBusy(false);
    }
  }

  const cta = method === "mpesa_stk" ? ctaSTK : method === "cash_on_pickup" ? ctaCash : ctaPaybill;
  const canPlace =
    !busy &&
    !!pickup &&
    accepted &&
    !(isDelivery && !details.deliveryLocationId) &&
    !!details.fullName.trim() &&
    !!details.phone.trim();

  return (
    <section className="techno-checkout-page">
      <div className="flat-breadcrumb">
        <div className="li-container">
          <ul className="breadcrumbs">
            <li>
              <Link to="/">Home</Link>
            </li>
            <li>
              <Link to="/cart">Cart</Link>
            </li>
            <li className="trail-end">
              <span>Checkout</span>
            </li>
          </ul>
        </div>
      </div>

      <div className="li-container techno-checkout">
        <h1 className="checkout-page-title">Checkout</h1>

        {lines.length === 0 ? (
          <div className="empty-state cart-empty">
            <strong>Your cart is empty</strong>
            <p>Add products from the shop, then return to check out.</p>
            <Link className="btn" to="/shop">
              Browse shop
            </Link>
          </div>
        ) : (
          <div className="checkout-3col">
            {/* Col 1 — Billing */}
            <section className="checkout-col checkout-col-billing">
              <h2>Billing details</h2>
              {session ? (
                <p className="checkout-signed-in">
                  Checking out as <strong>{session.customer.full_name}</strong>
                  {" · "}
                  <Link to="/account">Account</Link>
                </p>
              ) : (
                <p className="checkout-signed-in">
                  Already have an account? <Link to="/account">Sign in</Link>
                </p>
              )}

              <div className="billing-fields">
                <fieldset className="fulfilment-fieldset">
                  <legend>Fulfilment *</legend>
                  <div className="fulfilment-options">
                    <label className={`fulfilment-option ${!isDelivery ? "is-active" : ""}`}>
                      <input
                        type="radio"
                        name="fulfilment"
                        checked={!isDelivery}
                        onChange={() => updateDetail("fulfilment", "branch_pickup")}
                      />
                      <span>
                        <strong>Pickup</strong>
                        <em>Collect from a branch</em>
                      </span>
                    </label>
                    {deliveryAvailable ? (
                      <label className={`fulfilment-option ${isDelivery ? "is-active" : ""}`}>
                        <input
                          type="radio"
                          name="fulfilment"
                          checked={isDelivery}
                          onChange={() => updateDetail("fulfilment", "delivery")}
                        />
                        <span>
                          <strong>Delivery</strong>
                          <em>Choose an area · fee applies</em>
                        </span>
                      </label>
                    ) : null}
                  </div>
                  {!deliveryAvailable ? (
                    <p className="muted tiny fulfilment-hint">Delivery is not available yet for this shop.</p>
                  ) : null}
                </fieldset>

                <label className="field">
                  Full name *
                  <input
                    value={details.fullName}
                    onChange={(e) => updateDetail("fullName", e.target.value)}
                    placeholder="Ali Tufan"
                    autoComplete="name"
                    required
                  />
                </label>
                <label className="field">
                  Phone *
                  <input
                    value={details.phone}
                    onChange={(e) => updateDetail("phone", e.target.value)}
                    placeholder="07XXXXXXXX"
                    inputMode="tel"
                    autoComplete="tel"
                    required
                  />
                </label>
                <label className="field">
                  Email address
                  <input
                    type="email"
                    value={details.email}
                    onChange={(e) => updateDetail("email", e.target.value)}
                    placeholder="you@example.com"
                    autoComplete="email"
                  />
                </label>

                {branches.length > 0 ? (
                  <label className="field">
                    {isDelivery ? "Ship from branch *" : "Pickup branch *"}
                    <select
                      value={pickup?.branch_id ?? ""}
                      onChange={(e) => {
                        const b = branches.find((x) => x.id === e.target.value);
                        if (b) void changeBranch(b);
                      }}
                    >
                      {branches.map((b) => (
                        <option key={b.id} value={b.id}>
                          {b.name}
                          {b.location_name ? ` · ${b.location_name}` : ""}
                        </option>
                      ))}
                    </select>
                  </label>
                ) : (
                  <p className="error">No branch is configured yet.</p>
                )}

                {!isDelivery && selected ? (
                  <div className="pickup-card">
                    <strong>Pickup details</strong>
                    {selected.address ? <p>{selected.address}</p> : null}
                    {selected.phone ? <p>{selected.phone}</p> : null}
                    {selected.hours ? <p>{selected.hours}</p> : null}
                    {selected.map_url ? (
                      <p>
                        <a href={selected.map_url} target="_blank" rel="noreferrer">
                          Open map
                        </a>
                      </p>
                    ) : null}
                  </div>
                ) : null}

                {isDelivery ? (
                  <div className="delivery-fields">
                    <label className="field">
                      Delivery location *
                      <select
                        value={details.deliveryLocationId}
                        onChange={(e) => updateDetail("deliveryLocationId", e.target.value)}
                        required
                      >
                        <option value="">— Choose an area —</option>
                        {deliveryLocations.map((loc) => (
                          <option key={loc.id} value={loc.id}>
                            {loc.name}
                            {loc.fee > 0 ? ` · ${formatPrice(loc.fee)}` : " · Free"}
                          </option>
                        ))}
                      </select>
                    </label>
                    {selectedDelivery?.description ? (
                      <p className="muted tiny">{selectedDelivery.description}</p>
                    ) : null}
                    <label className="field">
                      Street / building
                      <input
                        value={details.addressLine1}
                        onChange={(e) => updateDetail("addressLine1", e.target.value)}
                        placeholder="Building, street, estate (optional)"
                        autoComplete="address-line1"
                      />
                    </label>
                    <label className="field">
                      Apartment / floor
                      <input
                        value={details.addressLine2}
                        onChange={(e) => updateDetail("addressLine2", e.target.value)}
                        placeholder="Optional"
                        autoComplete="address-line2"
                      />
                    </label>
                    <label className="field">
                      Landmark
                      <input
                        value={details.landmark}
                        onChange={(e) => updateDetail("landmark", e.target.value)}
                        placeholder="Near …"
                      />
                    </label>
                  </div>
                ) : null}

                <label className="field">
                  Order notes
                  <textarea
                    value={details.notes}
                    onChange={(e) => updateDetail("notes", e.target.value)}
                    placeholder={
                      isDelivery
                        ? "Delivery instructions, preferred time, gate code…"
                        : "Notes about your order, e.g. preferred pickup time."
                    }
                    rows={3}
                  />
                </label>
              </div>
            </section>

            {/* Col 2 — Cart */}
            <section className="checkout-col checkout-col-cart">
              <div className="checkout-panel-head">
                <h2>Your cart</h2>
                <Link className="muted tiny" to="/cart">
                  Edit
                </Link>
              </div>
              <ul className="checkout-cart-list">
                {lines.map(({ item, qty }) => (
                  <li key={item.variant_id} className="checkout-cart-line">
                    <span className="order-thumb">
                      {catalogItemImageURL(item) ? (
                        <img src={catalogItemImageURL(item)} alt="" />
                      ) : (
                        <span className="thumb-empty" />
                      )}
                    </span>
                    <div className="checkout-cart-meta">
                      <strong>{item.product_name}</strong>
                      <span className="muted tiny">
                        {qty} × {formatPrice(item.sell_price)}
                      </span>
                    </div>
                    <span className="checkout-cart-line-total">{formatPrice(item.sell_price * qty)}</span>
                  </li>
                ))}
              </ul>
            </section>

            {/* Col 3 — Payment (always top-aligned / sticky) */}
            <aside className="checkout-col checkout-col-pay" aria-label="Payment">
              <div className="pay-rail-inner">
                <h2>Payment</h2>
                <p className="pay-rail-count muted tiny">
                  {cartCount} item{cartCount === 1 ? "" : "s"}
                </p>

                <div className="pay-rail-totals">
                  <div>
                    <span>Subtotal</span>
                    <strong>{formatPrice(total)}</strong>
                  </div>
                  <div>
                    <span>{isDelivery ? "Delivery" : "Fulfilment"}</span>
                    <strong>
                      {isDelivery
                        ? selectedDelivery
                          ? deliveryFee > 0
                            ? formatPrice(deliveryFee)
                            : "Free"
                          : "—"
                        : "Free"}
                    </strong>
                  </div>
                  {isDelivery && selectedDelivery ? (
                    <p className="muted tiny pay-rail-area">{selectedDelivery.name}</p>
                  ) : null}
                  <div className="pay-rail-grand">
                    <span>Total</span>
                    <strong>{formatPrice(grandTotal)}</strong>
                  </div>
                </div>

                <div className="order-pay-methods">
                  <label className={`pay-option ${method === "mpesa_stk" ? "is-active" : ""}`}>
                    <input
                      type="radio"
                      name="method"
                      checked={method === "mpesa_stk"}
                      onChange={() => setMethod("mpesa_stk")}
                    />
                    <span>
                      <strong>{labelSTK}</strong>
                      <em>{hintSTK}</em>
                    </span>
                  </label>
                  {method === "mpesa_stk" ? (
                    <label className="pay-phone-field">
                      Phone *
                      <input
                        value={details.phone}
                        onChange={(e) => updateDetail("phone", e.target.value)}
                        placeholder="07XXXXXXXX or 2547…"
                        inputMode="tel"
                        autoComplete="tel"
                        required
                      />
                    </label>
                  ) : null}
                  <label className={`pay-option ${method === "mpesa_c2b" ? "is-active" : ""}`}>
                    <input
                      type="radio"
                      name="method"
                      checked={method === "mpesa_c2b"}
                      onChange={() => setMethod("mpesa_c2b")}
                    />
                    <span>
                      <strong>
                        {labelPaybill}
                        {boot?.paybill ? ` (${boot.paybill})` : ""}
                      </strong>
                      <em>{hintPaybill}</em>
                    </span>
                  </label>
                  <label className={`pay-option ${method === "cash_on_pickup" ? "is-active" : ""}`}>
                    <input
                      type="radio"
                      name="method"
                      checked={method === "cash_on_pickup"}
                      onChange={() => setMethod("cash_on_pickup")}
                    />
                    <span>
                      <strong>{labelCash}</strong>
                      <em>{hintCash}</em>
                    </span>
                  </label>
                </div>

                <label className="order-terms">
                  <input type="checkbox" checked={accepted} onChange={(e) => setAccepted(e.target.checked)} />
                  <span>I’ve read and accept the terms &amp; conditions *</span>
                </label>

                {error ? <p className="error">{error}</p> : null}

                <button
                  type="button"
                  className="btn-place-order"
                  disabled={!canPlace}
                  onClick={() => void placeOrder()}
                >
                  {busy ? "Reserving stock…" : cta}
                </button>
              </div>
            </aside>
          </div>
        )}
      </div>
    </section>
  );
}
