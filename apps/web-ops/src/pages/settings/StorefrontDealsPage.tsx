import { useEffect, useMemo, useState } from "react";
import { Badge, Button, EmptyState, Input, PageHeader } from "../../components/ui";
import {
  createStorefrontDeal,
  deleteStorefrontDeal,
  listProducts,
  listStorefrontDeals,
  listVariants,
  productImageURL,
  updateStorefrontDeal,
  type Product,
  type StorefrontDeal,
  type Variant,
} from "../../lib/api";

export function StorefrontDealsPage() {
  const [deals, setDeals] = useState<StorefrontDeal[]>([]);
  const [products, setProducts] = useState<Product[]>([]);
  const [variants, setVariants] = useState<Variant[]>([]);
  const [error, setError] = useState("");
  const [busy, setBusy] = useState(false);

  const [newVariantId, setNewVariantId] = useState("");
  const [newTitle, setNewTitle] = useState("");
  const [newPrice, setNewPrice] = useState("");
  const [newEndsAt, setNewEndsAt] = useState("");

  function refresh() {
    return Promise.all([listStorefrontDeals(), listProducts(), listVariants()])
      .then(([d, p, v]) => {
        setDeals(d.items);
        setProducts(p.items);
        setVariants(v.items);
      })
      .catch((e) => setError(e instanceof Error ? e.message : "Failed to load deals"));
  }

  useEffect(() => {
    void refresh();
  }, []);

  const productById = useMemo(() => new Map(products.map((p) => [p.id, p])), [products]);
  const variantOptions = useMemo(
    () =>
      variants.map((v) => ({
        variant: v,
        label: `${productById.get(v.product_id)?.name ?? "Unknown"} — ${v.sku} (KES ${v.sell_price.toLocaleString()})`,
      })),
    [variants, productById],
  );

  async function create() {
    setBusy(true);
    setError("");
    try {
      const price = Number(newPrice);
      if (!newVariantId || !price) {
        setError("Pick a product SKU and enter a deal price");
        setBusy(false);
        return;
      }
      await createStorefrontDeal({
        variant_id: newVariantId,
        title: newTitle.trim() || undefined,
        deal_price: price,
        ends_at: newEndsAt ? new Date(newEndsAt).toISOString() : undefined,
      });
      setNewVariantId("");
      setNewTitle("");
      setNewPrice("");
      setNewEndsAt("");
      await refresh();
    } catch (e) {
      setError(e instanceof Error ? e.message : "Could not create deal");
    } finally {
      setBusy(false);
    }
  }

  async function toggleActive(deal: StorefrontDeal) {
    setBusy(true);
    setError("");
    try {
      const next = await updateStorefrontDeal(deal.id, { active: !deal.active });
      setDeals((prev) => prev.map((d) => (d.id === next.id ? next : d)));
    } catch (e) {
      setError(e instanceof Error ? e.message : "Update failed");
    } finally {
      setBusy(false);
    }
  }

  async function remove(id: string) {
    setBusy(true);
    setError("");
    try {
      await deleteStorefrontDeal(id);
      await refresh();
    } catch (e) {
      setError(e instanceof Error ? e.message : "Delete failed");
    } finally {
      setBusy(false);
    }
  }

  return (
    <div className="settings-page">
      <PageHeader
        title="Storefront deals"
        subtitle="Real discounts charged at checkout. Product photos come from Inventory — upload them there so deal cards show images."
      />
      {error ? <p className="form-error">{error}</p> : null}

      <section className="settings-form-card form-grid">
        <h2 style={{ margin: 0 }}>New deal</h2>
        <label>
          Product SKU
          <select className="input" value={newVariantId} onChange={(e) => setNewVariantId(e.target.value)}>
            <option value="">— Choose a SKU —</option>
            {variantOptions.map(({ variant, label }) => (
              <option key={variant.id} value={variant.id}>
                {label}
              </option>
            ))}
          </select>
        </label>
        <label>
          Title (optional)
          <Input value={newTitle} placeholder="Weekend flash sale" onChange={(e) => setNewTitle(e.target.value)} />
        </label>
        <div className="field-pair">
          <label>
            Deal price (KES)
            <Input type="number" value={newPrice} onChange={(e) => setNewPrice(e.target.value)} />
          </label>
          <label>
            Ends at (optional)
            <Input type="datetime-local" value={newEndsAt} onChange={(e) => setNewEndsAt(e.target.value)} />
          </label>
        </div>
        <div className="btn-row">
          <Button type="button" disabled={busy} onClick={() => void create()}>
            Add deal
          </Button>
        </div>
      </section>

      {deals.length === 0 ? (
        <EmptyState title="No deals yet" body="Create a deal above to feature it in Today's Deals on the storefront." />
      ) : (
        <div className="table-wrap">
          <table className="table">
            <thead>
              <tr>
                <th />
                <th>Product</th>
                <th>SKU</th>
                <th>Base price</th>
                <th>Deal price</th>
                <th>Ends</th>
                <th>Status</th>
                <th />
              </tr>
            </thead>
            <tbody>
              {deals.map((d) => (
                <tr key={d.id}>
                  <td style={{ width: 56 }}>
                    {d.has_image && d.product_id ? (
                      <img
                        src={productImageURL(d.product_id, d.image_updated_at)}
                        alt=""
                        style={{ width: 44, height: 44, objectFit: "contain", borderRadius: 6, background: "#f5f6fb" }}
                      />
                    ) : (
                      <div
                        title="Upload a photo on Inventory → product"
                        style={{
                          width: 44,
                          height: 44,
                          borderRadius: 6,
                          background: "#f0f0f0",
                          border: "1px solid #e4e6ef",
                        }}
                      />
                    )}
                  </td>
                  <td>{d.product_name}</td>
                  <td className="mono">{d.sku}</td>
                  <td>KES {(d.base_price ?? 0).toLocaleString()}</td>
                  <td>KES {d.deal_price.toLocaleString()}</td>
                  <td>{d.ends_at ? new Date(d.ends_at).toLocaleString() : "No end date"}</td>
                  <td>
                    <Badge tone={d.active ? "success" : "pending"}>{d.active ? "active" : "paused"}</Badge>
                  </td>
                  <td>
                    <div className="btn-row">
                      <Button type="button" variant="ghost" disabled={busy} onClick={() => void toggleActive(d)}>
                        {d.active ? "Pause" : "Resume"}
                      </Button>
                      <Button type="button" variant="ghost" disabled={busy} onClick={() => void remove(d.id)}>
                        Delete
                      </Button>
                    </div>
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
