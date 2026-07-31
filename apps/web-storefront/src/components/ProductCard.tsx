import { Link, useNavigate } from "react-router-dom";
import { useState } from "react";
import { useStorefront } from "../store/StorefrontContext";
import { catalogItemImageURL, type CatalogItem } from "../lib/api";
import { loadWishlist, toggleWishlist } from "../lib/wishlist";

export function ProductCard({ item }: { item: CatalogItem }) {
  const { cart, setQty, addOne, formatPrice } = useStorefront();
  const navigate = useNavigate();
  const qty = cart[item.variant_id] ?? 0;
  const onDeal = item.original_price != null && item.original_price > item.sell_price;
  const [wished, setWished] = useState(() => loadWishlist().includes(item.variant_id));
  const outOfStock = item.available_qty <= 0;
  const imageSrc = catalogItemImageURL(item);
  const inCart = qty > 0;
  const category = item.category || item.brand || "";

  return (
    <div
      className={`single-product-wrap ${inCart ? "is-in-cart" : ""} ${outOfStock ? "is-oos" : ""}`}
      onMouseLeave={(e) => {
        // Qty / wishlist clicks leave focus inside the card; blur so the hover panel closes.
        const active = document.activeElement;
        if (active instanceof HTMLElement && e.currentTarget.contains(active)) {
          active.blur();
        }
      }}
    >
      <div className="product-image">
        <Link to={`/product/${item.variant_id}`} tabIndex={-1} aria-hidden="true">
          {imageSrc ? <img src={imageSrc} alt="" loading="lazy" /> : <div className="thumb-empty" />}
        </Link>
        {item.new_arrival ? <span className="sticker">New</span> : null}
        {onDeal && !item.new_arrival ? <span className="sticker sale">Sale</span> : null}
      </div>

      <div className="product_desc">
        {category ? (
          <Link className="cat-name" to={`/shop?category=${encodeURIComponent(category)}`}>
            {category}
          </Link>
        ) : (
          <span className="cat-name">&nbsp;</span>
        )}

        <h4>
          <Link className="product_name" to={`/product/${item.variant_id}`}>
            {item.product_name}
          </Link>
        </h4>

        <div className="price-box">
          <span className="new-price">{formatPrice(item.sell_price)}</span>
          {onDeal ? <span className="old-price">{formatPrice(item.original_price!)}</span> : null}
        </div>

        <div className="add-actions">
          <div className="btn-add-cart">
            {inCart ? (
              <div className="qty-row compact">
                <button
                  type="button"
                  className="qty-btn"
                  onClick={() => setQty(item.variant_id, qty - 1, item.available_qty)}
                >
                  −
                </button>
                <span>{qty}</span>
                <button
                  type="button"
                  className="qty-btn"
                  disabled={qty >= item.available_qty}
                  onClick={() => addOne(item)}
                >
                  +
                </button>
              </div>
            ) : (
              <button type="button" disabled={outOfStock} onClick={() => addOne(item)}>
                <svg width="15" height="15" viewBox="0 0 24 24" fill="none" aria-hidden="true">
                  <path
                    d="M6 6h15l-1.5 9h-12L6 6Zm0 0L5 3H2"
                    stroke="currentColor"
                    strokeWidth="1.8"
                    strokeLinecap="round"
                    strokeLinejoin="round"
                  />
                  <circle cx="9" cy="20" r="1.4" fill="currentColor" />
                  <circle cx="17" cy="20" r="1.4" fill="currentColor" />
                </svg>
                {outOfStock ? "Out of stock" : "Add to Cart"}
              </button>
            )}
          </div>
          <div className="compare-wishlist">
            <button
              type="button"
              className="cw-link"
              aria-label={`View ${item.product_name}`}
              onClick={() => navigate(`/product/${item.variant_id}`)}
            >
              <svg width="14" height="14" viewBox="0 0 24 24" fill="none" aria-hidden="true">
                <path
                  d="M2 12s3.5-7 10-7 10 7 10 7-3.5 7-10 7S2 12 2 12Z"
                  stroke="currentColor"
                  strokeWidth="1.8"
                />
                <circle cx="12" cy="12" r="3" stroke="currentColor" strokeWidth="1.8" />
              </svg>
              Quick view
            </button>
            <button
              type="button"
              className={`cw-link ${wished ? "is-active" : ""}`}
              aria-label={wished ? "Remove from saved" : "Save"}
              aria-pressed={wished}
              onClick={() => setWished(toggleWishlist(item.variant_id).includes(item.variant_id))}
            >
              <svg width="14" height="14" viewBox="0 0 24 24" fill={wished ? "currentColor" : "none"} aria-hidden="true">
                <path
                  d="M12 21s-7-4.6-9.5-9A5.4 5.4 0 0 1 12 5.2 5.4 5.4 0 0 1 21.5 12C19 16.4 12 21 12 21Z"
                  stroke="currentColor"
                  strokeWidth="1.8"
                  strokeLinejoin="round"
                />
              </svg>
              Wishlist
            </button>
          </div>
        </div>
      </div>
    </div>
  );
}
