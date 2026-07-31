import { Link, useNavigate } from "react-router-dom";
import { useStorefront } from "../store/StorefrontContext";
import { catalogItemImageURL } from "../lib/api";

export function CartPage() {
  const navigate = useNavigate();
  const { lines, total, setQty, formatPrice, cartCount, pickup, branches } = useStorefront();
  const branch = branches.find((b) => b.id === pickup?.branch_id) ?? null;

  return (
    <section className="cart-page limupa-cart">
      <div className="page-header">
        <h1>Shopping cart</h1>
        <ol className="breadcrumb">
          <li>
            <Link to="/">Home</Link>
          </li>
          <li className="active">Cart</li>
        </ol>
      </div>

      <div className="li-container cart-body">
        {lines.length === 0 ? (
          <div className="empty-state cart-empty">
            <strong>Your cart is empty</strong>
            <p>Browse the shop and add products, then check out here.</p>
            <Link className="btn" to="/shop">
              Continue shopping
            </Link>
          </div>
        ) : (
          <div className="cart-layout">
            <div className="cart-main">
              <div className="cart-table-head" aria-hidden="true">
                <span>Product</span>
                <span>Price</span>
                <span>Qty</span>
                <span>Total</span>
              </div>
              <ul className="cart-list">
                {lines.map(({ item, qty }) => (
                  <li key={item.variant_id} className="cart-line electro-cart-line">
                    <div className="cart-product">
                      <Link to={`/product/${item.variant_id}`} className="cart-thumb">
                        {catalogItemImageURL(item) ? (
                          <img src={catalogItemImageURL(item)} alt="" />
                        ) : (
                          <div className="thumb-empty" />
                        )}
                      </Link>
                      <div className="cart-product-meta">
                        <Link className="cart-product-name" to={`/product/${item.variant_id}`}>
                          {item.product_name}
                        </Link>
                        {item.brand ? <div className="muted tiny">{item.brand}</div> : null}
                        <div className="muted tiny">{item.sku}</div>
                        <button
                          type="button"
                          className="linkish"
                          onClick={() => setQty(item.variant_id, 0, item.available_qty)}
                        >
                          Remove
                        </button>
                      </div>
                    </div>
                    <div className="cart-price" data-label="Price">
                      {formatPrice(item.sell_price)}
                    </div>
                    <div className="cart-qty" data-label="Qty">
                      <div className="qty-row compact">
                        <button
                          type="button"
                          className="btn btn-ghost qty-btn"
                          aria-label={`Decrease ${item.product_name}`}
                          onClick={() => setQty(item.variant_id, qty - 1, item.available_qty)}
                        >
                          −
                        </button>
                        <span className="qty-value">{qty}</span>
                        <button
                          type="button"
                          className="btn btn-ghost qty-btn"
                          aria-label={`Increase ${item.product_name}`}
                          disabled={qty >= item.available_qty}
                          onClick={() => setQty(item.variant_id, qty + 1, item.available_qty)}
                        >
                          +
                        </button>
                      </div>
                    </div>
                    <div className="line-total" data-label="Total">
                      {formatPrice(item.sell_price * qty)}
                    </div>
                  </li>
                ))}
              </ul>
              <div className="cart-actions-row">
                <Link className="btn btn-ghost" to="/shop">
                  Continue shopping
                </Link>
                <span className="muted tiny">
                  {cartCount} item{cartCount === 1 ? "" : "s"} in cart
                </span>
              </div>
            </div>

            <aside className="cart-summary">
              <h2>Order summary</h2>
              <div className="total">
                <span>Subtotal</span>
                <span>{formatPrice(total)}</span>
              </div>
              {branch || pickup ? (
                <p className="muted tiny cart-pickup-note">
                  Pickup: {branch?.name ?? pickup?.branch_name}
                  {branch?.address ? ` · ${branch.address}` : ""}
                </p>
              ) : null}
              <p className="muted tiny">Pay at checkout with M-Pesa STK, paybill, or cash on pickup.</p>
              <button type="button" className="btn" onClick={() => navigate("/checkout")}>
                Proceed to checkout
              </button>
            </aside>
          </div>
        )}
      </div>
    </section>
  );
}
