import { useEffect, useState } from "react";
import { Link } from "react-router-dom";
import { useStorefront } from "../store/StorefrontContext";
import { ProductCard } from "../components/ProductCard";
import { loadWishlist, onWishlistChanged } from "../lib/wishlist";

export function SavedPage() {
  const { catalog } = useStorefront();
  const [ids, setIds] = useState<string[]>(() => loadWishlist());

  useEffect(() => onWishlistChanged(() => setIds(loadWishlist())), []);

  const items = catalog.filter((c) => ids.includes(c.variant_id));

  return (
    <section className="wide-page">
      <div className="page-header">
        <h1>Saved items</h1>
        <ol className="breadcrumb">
          <li>
            <Link to="/">Home</Link>
          </li>
          <li className="active">Saved</li>
        </ol>
      </div>

      <div className="li-container page-body">
        {items.length === 0 ? (
          <div className="empty-state cart-empty">
            <strong>Nothing saved yet</strong>
            <p>Tap the heart on a product to save it here.</p>
            <Link className="btn" to="/shop">
              Browse shop
            </Link>
          </div>
        ) : (
          <div className="catalog limupa-catalog">
            {items.map((item) => (
              <ProductCard key={item.variant_id} item={item} />
            ))}
          </div>
        )}
      </div>
    </section>
  );
}
