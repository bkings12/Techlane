import { useCallback, useEffect, useState } from "react";
import { Badge, Button, EmptyState, PageHeader } from "../../components/ui";
import { listStorefrontReviews, setStorefrontReviewStatus, type StorefrontReview } from "../../lib/api";

function Stars({ rating }: { rating: number }) {
  return (
    <span aria-label={`${rating} out of 5 stars`} style={{ letterSpacing: "1px" }}>
      {"★".repeat(rating)}
      <span className="muted">{"★".repeat(5 - rating)}</span>
    </span>
  );
}

export function StorefrontReviewsPage() {
  const [items, setItems] = useState<StorefrontReview[]>([]);
  const [filter, setFilter] = useState<"" | "published" | "hidden">("");
  const [error, setError] = useState("");
  const [busy, setBusy] = useState<string | null>(null);

  const refresh = useCallback(async () => {
    const r = await listStorefrontReviews(filter || undefined);
    setItems(r.items ?? []);
  }, [filter]);

  useEffect(() => {
    refresh().catch((e) => setError(e instanceof Error ? e.message : "Failed to load"));
  }, [refresh]);

  async function toggle(review: StorefrontReview) {
    const next = review.status === "published" ? "hidden" : "published";
    setBusy(review.id);
    setError("");
    try {
      await setStorefrontReviewStatus(review.id, next);
      await refresh();
    } catch (e) {
      setError(e instanceof Error ? e.message : "Action failed");
    } finally {
      setBusy(null);
    }
  }

  return (
    <div className="settings-page">
      <PageHeader
        title="Storefront reviews"
        subtitle="Verified-purchase product reviews from the public web-storefront"
      />
      <div className="chip-row">
        {(["", "published", "hidden"] as const).map((f) => (
          <button
            key={f || "all"}
            type="button"
            className={filter === f ? "chip active" : "chip"}
            onClick={() => setFilter(f)}
          >
            {f === "" ? "All" : f === "published" ? "Published" : "Hidden"}
          </button>
        ))}
      </div>
      {error ? <p className="form-error">{error}</p> : null}

      {items.length === 0 ? (
        <EmptyState title="No reviews yet" body="Reviews left by customers who've collected their order will appear here." />
      ) : (
        <div className="table-wrap">
          <table className="table">
            <thead>
              <tr>
                <th>Product</th>
                <th>Customer</th>
                <th>Rating</th>
                <th>Review</th>
                <th>Status</th>
                <th>Left</th>
                <th />
              </tr>
            </thead>
            <tbody>
              {items.map((r) => (
                <tr key={r.id}>
                  <td>{r.product_name}</td>
                  <td>{r.customer_name}</td>
                  <td>
                    <Stars rating={r.rating} />
                  </td>
                  <td>
                    {r.title ? <strong>{r.title}</strong> : null}
                    {r.body ? <div className="muted">{r.body}</div> : null}
                  </td>
                  <td>
                    <Badge tone={r.status === "published" ? "success" : "pending"}>{r.status}</Badge>
                  </td>
                  <td>{new Date(r.created_at).toLocaleDateString()}</td>
                  <td>
                    <Button type="button" variant="ghost" disabled={busy === r.id} onClick={() => void toggle(r)}>
                      {r.status === "published" ? "Hide" : "Publish"}
                    </Button>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}
    </div>
  );
}
