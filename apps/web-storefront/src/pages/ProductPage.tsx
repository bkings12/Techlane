import { useEffect, useState, type FormEvent } from "react";
import { Link, useNavigate, useParams } from "react-router-dom";
import ReactMarkdown from "react-markdown";
import { useStorefront } from "../store/StorefrontContext";
import { StarRating, StarRatingInput } from "../components/StarRating";
import { listProductReviews, recordProductView, submitProductReview, catalogItemImageURL, type ProductReview } from "../lib/api";

function ReviewsSection({ productId }: { productId: string }) {
  const { session } = useStorefront();
  const [reviews, setReviews] = useState<ProductReview[]>([]);
  const [error, setError] = useState("");
  const [rating, setRating] = useState(0);
  const [title, setTitle] = useState("");
  const [body, setBody] = useState("");
  const [busy, setBusy] = useState(false);
  const [submitError, setSubmitError] = useState("");
  const [done, setDone] = useState(false);

  useEffect(() => {
    listProductReviews(productId)
      .then((r) => setReviews(r.items ?? []))
      .catch((e) => setError(e instanceof Error ? e.message : "Failed to load reviews"));
  }, [productId]);

  async function submit(e: FormEvent) {
    e.preventDefault();
    if (!session?.token || rating < 1) return;
    setBusy(true);
    setSubmitError("");
    try {
      await submitProductReview(productId, { rating, title: title.trim() || undefined, body: body.trim() || undefined }, session.token);
      setDone(true);
      const r = await listProductReviews(productId);
      setReviews(r.items ?? []);
    } catch (e) {
      setSubmitError(e instanceof Error ? e.message : "Could not submit review");
    } finally {
      setBusy(false);
    }
  }

  return (
    <section className="reviews-section">
      <h2 className="section-title">Reviews</h2>
      {error ? <p className="error">{error}</p> : null}

      {reviews.length === 0 ? (
        <p className="muted">No reviews yet.</p>
      ) : (
        <ul className="review-list">
          {reviews.map((r) => (
            <li key={r.id} className="review-item">
              <StarRating value={r.rating} />
              {r.title ? <strong>{r.title}</strong> : null}
              {r.body ? <p>{r.body}</p> : null}
              <p className="muted tiny">{new Date(r.created_at).toLocaleDateString()}</p>
            </li>
          ))}
        </ul>
      )}

      {session ? (
        done ? (
          <p className="muted">Thanks — your review was saved.</p>
        ) : (
          <form className="lookup-form form-column" onSubmit={(e) => void submit(e)}>
            <p className="muted tiny">
              Only shoppers who've collected this product can leave a review — if you haven't yet, submitting will
              tell you why.
            </p>
            <StarRatingInput value={rating} onChange={setRating} />
            <label className="field">
              Title (optional)
              <input value={title} onChange={(e) => setTitle(e.target.value)} maxLength={120} />
            </label>
            <label className="field">
              Review (optional)
              <textarea value={body} onChange={(e) => setBody(e.target.value)} rows={3} />
            </label>
            {submitError ? <p className="error">{submitError}</p> : null}
            <button type="submit" className="btn" disabled={busy || rating < 1}>
              {busy ? "Submitting…" : "Submit review"}
            </button>
          </form>
        )
      ) : (
        <p className="muted">
          <Link to="/account">Sign in</Link> to leave a review after your order is collected.
        </p>
      )}
    </section>
  );
}

export function ProductPage() {
  const { variantId } = useParams();
  const navigate = useNavigate();
  const { catalog, cart, setQty, addOne, pickup, formatPrice, content } = useStorefront();
  const selected = variantId ? catalog.find((c) => c.variant_id === variantId) ?? null : null;

  useEffect(() => {
    if (!selected) return;
    const key = `viewed:${selected.variant_id}`;
    if (sessionStorage.getItem(key)) return;
    sessionStorage.setItem(key, "1");
    void recordProductView(selected.variant_id).catch(() => undefined);
  }, [selected]);

  if (!selected) {
    return (
      <section className="wide-page">
        <div className="page-header">
          <h1>Product</h1>
          <ol className="breadcrumb">
            <li>
              <Link to="/">Home</Link>
            </li>
            <li>
              <Link to="/shop">Shop</Link>
            </li>
            <li className="active">Not found</li>
          </ol>
        </div>
        <div className="li-container page-body">
          <p className="muted">Product not found.</p>
          <button type="button" className="btn" onClick={() => navigate("/shop")}>
            Back to catalog
          </button>
        </div>
      </section>
    );
  }

  const qty = cart[selected.variant_id] ?? 0;
  const onDeal = selected.original_price != null && selected.original_price > selected.sell_price;
  const imageSrc = catalogItemImageURL(selected);
  const bargainEnabled = !!content?.settings?.bargain_enabled;
  const whatsappDigits = (content?.settings?.whatsapp_number || "").replace(/\D/g, "");
  const bargainHref =
    bargainEnabled && whatsappDigits
      ? `https://wa.me/${whatsappDigits}?text=${encodeURIComponent(
          `Hi, I'm interested in ${selected.product_name} (KES ${selected.sell_price.toLocaleString()}) — is the price negotiable?`,
        )}`
      : null;

  return (
    <section className="wide-page product-detail">
      <div className="page-header">
        <h1>{selected.product_name}</h1>
        <ol className="breadcrumb">
          <li>
            <Link to="/">Home</Link>
          </li>
          <li>
            <Link to="/shop">Shop</Link>
          </li>
          <li className="active">{selected.product_name}</li>
        </ol>
      </div>

      <div className="li-container page-body">
        <div className="pdp-layout">
          <div className="pdp-gallery">
            {imageSrc ? (
              <img className="product-image" src={imageSrc} alt={selected.product_name} />
            ) : (
              <div className="thumb-empty product-image" />
            )}
          </div>
          <div className="pdp-info">
            <p className="muted">
              {selected.brand ? `${selected.brand} · ` : ""}
              SKU {selected.sku}
            </p>
            {selected.rating_count ? <StarRating value={selected.rating_avg ?? 0} count={selected.rating_count} /> : null}
            <p className="price">
              {onDeal ? <span className="price-was">{formatPrice(selected.original_price!)}</span> : null}{" "}
              <span className="price-now">{formatPrice(selected.sell_price)}</span>
            </p>
            <p className="muted">
              {selected.available_qty} in stock at {pickup?.location_name ?? "counter"}
            </p>
            {selected.description ? (
              <div className="pdp-description">
                <h3>Description</h3>
                <ReactMarkdown>{selected.description}</ReactMarkdown>
              </div>
            ) : null}
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
                aria-label="Increase quantity"
                disabled={qty >= selected.available_qty || selected.available_qty <= 0}
                onClick={() => addOne(selected)}
              >
                +
              </button>
              <button type="button" className="btn" disabled={selected.available_qty <= 0} onClick={() => addOne(selected)}>
                Add to cart
              </button>
              {bargainHref ? (
                <a className="btn btn-ghost" href={bargainHref} target="_blank" rel="noreferrer">
                  Bargain
                </a>
              ) : null}
            </div>
            {qty > 0 ? (
              <div className="stack" style={{ justifyContent: "flex-start", marginTop: "0.75rem" }}>
                <button type="button" className="btn btn-ghost" onClick={() => navigate("/cart")}>
                  View cart
                </button>
                <button type="button" className="btn" onClick={() => navigate("/checkout")}>
                  Checkout
                </button>
              </div>
            ) : null}
          </div>
        </div>

        <ReviewsSection productId={selected.product_id} />
      </div>
    </section>
  );
}
