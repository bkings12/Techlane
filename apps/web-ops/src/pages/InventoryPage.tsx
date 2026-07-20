import { useCallback, useEffect, useMemo, useState, type FormEvent } from "react";
import { Link } from "react-router-dom";
import { useBranch } from "../branch/BranchContext";
import { Badge, Button, EmptyState, ICONS, Input, PageHeader, Stat, StatStrip } from "../components/ui";
import {
  adjustStock,
  createProduct,
  createVariant,
  listProducts,
  listPOSCatalog,
  listStockBalances,
  listStockLocations,
  listStockMovements,
  listVariants,
  publishProduct,
  unpublishProduct,
  updateProduct,
  receiveStock,
  transferStock,
  type CatalogItem,
  type Product,
  type StockBalance,
  type StockLocation,
  type StockMovement,
  type Variant,
} from "../lib/api";

type Mode = "receive" | "adjust" | "transfer";

export function InventoryPage() {
  const { branchId } = useBranch();
  const [locations, setLocations] = useState<StockLocation[]>([]);
  const [locationId, setLocationId] = useState("");
  const [toLocationId, setToLocationId] = useState("");
  const [balances, setBalances] = useState<StockBalance[]>([]);
  const [movements, setMovements] = useState<StockMovement[]>([]);
  const [catalog, setCatalog] = useState<CatalogItem[]>([]);
  const [products, setProducts] = useState<Product[]>([]);
  const [variants, setVariants] = useState<Variant[]>([]);
  const [editProductId, setEditProductId] = useState<string | null>(null);
  const [editCategory, setEditCategory] = useState("");
  const [editDescription, setEditDescription] = useState("");
  const [editImageUrl, setEditImageUrl] = useState("");
  const [error, setError] = useState("");
  const [busy, setBusy] = useState(false);
  const [variantId, setVariantId] = useState("");
  const [qty, setQty] = useState("1");
  const [note, setNote] = useState("");
  const [mode, setMode] = useState<Mode>("receive");
  const [prodName, setProdName] = useState("");
  const [prodBrand, setProdBrand] = useState("");
  const [variantProductId, setVariantProductId] = useState("");
  const [sku, setSku] = useState("");
  const [sellPrice, setSellPrice] = useState("0");

  const refreshMeta = useCallback(async () => {
    const locs = await listStockLocations(branchId || undefined);
    const items = locs.items ?? [];
    setLocations(items);
    const counter = items.find((l) => l.location_type === "counter") || items[0];
    const store = items.find((l) => l.location_type === "store") || items.find((l) => l.id !== counter?.id);
    setLocationId((prev) => {
      if (prev && items.some((l) => l.id === prev)) return prev;
      return counter?.id || "";
    });
    setToLocationId((prev) => {
      if (prev && items.some((l) => l.id === prev)) return prev;
      return store?.id || items.find((l) => l.id !== counter?.id)?.id || "";
    });
  }, [branchId]);

  const refreshCatalogAdmin = useCallback(async () => {
    const [p, v] = await Promise.all([listProducts(), listVariants()]);
    setProducts(p.items ?? []);
    setVariants(v.items ?? []);
    setVariantProductId((prev) => prev || p.items?.[0]?.id || "");
  }, []);

  const refreshStock = useCallback(async (loc: string) => {
    const [b, m, c] = await Promise.all([
      listStockBalances(loc || undefined),
      listStockMovements(loc || undefined),
      listPOSCatalog(loc || undefined),
    ]);
    setBalances(b.items ?? []);
    setMovements(m.items ?? []);
    setCatalog(c.items ?? []);
    setVariantId((prev) => prev || c.items?.[0]?.variant_id || "");
  }, []);

  useEffect(() => {
    refreshMeta().catch((e) => setError(e instanceof Error ? e.message : "Failed to load"));
  }, [refreshMeta]);

  useEffect(() => {
    if (!locationId) return;
    refreshStock(locationId).catch((e) => setError(e instanceof Error ? e.message : "Stock failed"));
    refreshCatalogAdmin().catch((e) => setError(e instanceof Error ? e.message : "Catalog failed"));
  }, [locationId, refreshStock, refreshCatalogAdmin]);

  const totals = useMemo(() => {
    const physical = balances.reduce((s, b) => s + b.physical_qty, 0);
    const available = balances.reduce((s, b) => s + b.available_qty, 0);
    const reserved = balances.reduce((s, b) => s + b.reserved_qty, 0);
    return { physical, available, reserved, skus: balances.length };
  }, [balances]);

  const modeTitle =
    mode === "receive" ? "Receive stock" : mode === "adjust" ? "Adjust stock" : "Transfer stock";

  async function submitMovement(e: FormEvent) {
    e.preventDefault();
    if (!locationId || !variantId) return;
    const n = Number(qty);
    if (!Number.isFinite(n) || n === 0) {
      setError("Enter a non-zero quantity");
      return;
    }
    setBusy(true);
    setError("");
    try {
      if (mode === "receive") {
        if (n < 1) throw new Error("Receive quantity must be positive");
        await receiveStock({
          variant_id: variantId,
          location_id: locationId,
          quantity: Math.floor(n),
          note: note || undefined,
        });
      } else if (mode === "adjust") {
        await adjustStock({
          variant_id: variantId,
          location_id: locationId,
          qty_delta: Math.trunc(n),
          reason: note || "adjustment",
        });
      } else {
        if (n < 1) throw new Error("Transfer quantity must be positive");
        if (!toLocationId) throw new Error("Select a destination location");
        if (toLocationId === locationId) throw new Error("Pick a different destination");
        await transferStock({
          variant_id: variantId,
          from_location_id: locationId,
          to_location_id: toLocationId,
          quantity: Math.floor(n),
        });
      }
      setNote("");
      setQty(mode === "adjust" ? "-1" : "1");
      await refreshStock(locationId);
    } catch (err) {
      setError(err instanceof Error ? err.message : "Movement failed");
    } finally {
      setBusy(false);
    }
  }

  async function submitProduct(e: FormEvent) {
    e.preventDefault();
    setBusy(true);
    setError("");
    try {
      await createProduct({ name: prodName.trim(), brand: prodBrand.trim() || undefined });
      setProdName("");
      setProdBrand("");
      await refreshCatalogAdmin();
    } catch (err) {
      setError(err instanceof Error ? err.message : "Create product failed");
    } finally {
      setBusy(false);
    }
  }

  async function submitVariant(e: FormEvent) {
    e.preventDefault();
    if (!variantProductId) return;
    setBusy(true);
    setError("");
    try {
      await createVariant({
        product_id: variantProductId,
        sku: sku.trim(),
        sell_price: Number(sellPrice) || 0,
      });
      setSku("");
      setSellPrice("0");
      await Promise.all([refreshCatalogAdmin(), refreshStock(locationId)]);
    } catch (err) {
      setError(err instanceof Error ? err.message : "Create variant failed");
    } finally {
      setBusy(false);
    }
  }

  return (
    <div className="inventory-page">
      <PageHeader
        title="Inventory"
        subtitle="Catalog, stock by location — receive, adjust, and transfer"
        actions={
          <Link className="btn btn-ghost" to="/pos">
            Open POS
          </Link>
        }
      />
      {error ? <p className="form-error">{error}</p> : null}

      <div className="pos-toolbar">
        <label>
          Viewing location
          <select className="input" value={locationId} onChange={(e) => setLocationId(e.target.value)}>
            {locations.map((l) => (
              <option key={l.id} value={l.id}>
                {l.name} ({l.location_type})
              </option>
            ))}
          </select>
        </label>
      </div>

      <StatStrip>
        <Stat icon={ICONS.hash} label="SKUs" value={totals.skus} />
        <Stat icon={ICONS.boxes} label="Physical" value={totals.physical} />
        <Stat icon={ICONS.ready} label="Available" value={totals.available} tone="success" />
        <Stat icon={ICONS.clock} label="Reserved" value={totals.reserved} />
      </StatStrip>

      <div className="repair-grid">
        <section className="panel">
          <h2>Products & variants</h2>
          <form className="form-grid" onSubmit={submitProduct}>
            <label>
              New product name
              <Input value={prodName} onChange={(e) => setProdName(e.target.value)} required />
            </label>
            <label>
              Brand
              <Input value={prodBrand} onChange={(e) => setProdBrand(e.target.value)} />
            </label>
            <Button type="submit" disabled={busy}>
              Create product
            </Button>
          </form>
          <form className="form-grid" onSubmit={submitVariant} style={{ marginTop: "1rem" }}>
            <label>
              Product for variant
              <select
                className="input"
                value={variantProductId}
                onChange={(e) => setVariantProductId(e.target.value)}
                required
              >
                <option value="">Select…</option>
                {products.map((p) => (
                  <option key={p.id} value={p.id}>
                    {p.name}
                    {p.brand ? ` (${p.brand})` : ""}
                  </option>
                ))}
              </select>
            </label>
            <label>
              SKU
              <Input value={sku} onChange={(e) => setSku(e.target.value)} required className="mono" />
            </label>
            <label>
              Sell price (KES)
              <Input type="number" value={sellPrice} onChange={(e) => setSellPrice(e.target.value)} required />
            </label>
            <Button type="submit" disabled={busy || !variantProductId}>
              Create variant
            </Button>
          </form>

          {products.length === 0 ? (
            <EmptyState title="No products" body="Create a product, then add a SKU variant." icon={ICONS.package} />
          ) : (
            <ul className="part-list" style={{ marginTop: "1rem" }}>
              {products.map((p) => {
                const pVariants = variants.filter((v) => v.product_id === p.id);
                const online = Boolean(p.online_visible);
                const editing = editProductId === p.id;
                return (
                  <li key={p.id} className="part-card">
                    <div className="part-head">
                      <div>
                        <strong>{p.name}</strong>
                        {p.brand ? <span className="muted"> · {p.brand}</span> : null}
                        {p.category ? <div className="muted">Category: {p.category}</div> : null}
                        <div className="muted">
                          {pVariants.length} variant{pVariants.length === 1 ? "" : "s"}
                          {pVariants.length
                            ? ` · ${pVariants.map((v) => v.sku).join(", ")}`
                            : ""}
                        </div>
                      </div>
                      <Badge tone={online ? "success" : "pending"}>
                        {online ? "online" : "offline"}
                      </Badge>
                    </div>
                    {editing ? (
                      <form
                        className="form-grid"
                        onSubmit={(e) => {
                          e.preventDefault();
                          setBusy(true);
                          setError("");
                          updateProduct(p.id, {
                            category: editCategory.trim() || undefined,
                            description: editDescription.trim() || undefined,
                            image_url: editImageUrl.trim() || undefined,
                          })
                            .then(() => {
                              setEditProductId(null);
                              return refreshCatalogAdmin();
                            })
                            .catch((err) =>
                              setError(err instanceof Error ? err.message : "Update failed"),
                            )
                            .finally(() => setBusy(false));
                        }}
                      >
                        <label>
                          Category
                          <Input value={editCategory} onChange={(e) => setEditCategory(e.target.value)} />
                        </label>
                        <label>
                          Description
                          <Input value={editDescription} onChange={(e) => setEditDescription(e.target.value)} />
                        </label>
                        <label>
                          Image URL
                          <Input value={editImageUrl} onChange={(e) => setEditImageUrl(e.target.value)} />
                        </label>
                        <div className="btn-row">
                          <Button type="submit" disabled={busy}>
                            Save
                          </Button>
                          <Button type="button" variant="ghost" onClick={() => setEditProductId(null)}>
                            Cancel
                          </Button>
                        </div>
                      </form>
                    ) : (
                      <div className="btn-row">
                        <Button
                          type="button"
                          variant="ghost"
                          disabled={busy}
                          onClick={() => {
                            setEditProductId(p.id);
                            setEditCategory(p.category ?? "");
                            setEditDescription(p.description ?? "");
                            setEditImageUrl(p.image_url ?? "");
                          }}
                        >
                          Edit listing
                        </Button>
                        {online ? (
                          <Button
                            type="button"
                            variant="secondary"
                            disabled={busy}
                            onClick={() => {
                              setBusy(true);
                              setError("");
                              unpublishProduct(p.id)
                                .then(() => refreshCatalogAdmin())
                                .catch((err) =>
                                  setError(err instanceof Error ? err.message : "Unpublish failed"),
                                )
                                .finally(() => setBusy(false));
                            }}
                          >
                            Unpublish
                          </Button>
                        ) : (
                          <Button
                            type="button"
                            variant="secondary"
                            disabled={busy}
                            onClick={() => {
                              setBusy(true);
                              setError("");
                              publishProduct(p.id)
                                .then(() => refreshCatalogAdmin())
                                .catch((err) =>
                                  setError(err instanceof Error ? err.message : "Publish failed"),
                                )
                                .finally(() => setBusy(false));
                            }}
                          >
                            Publish online
                          </Button>
                        )}
                      </div>
                    )}
                  </li>
                );
              })}
            </ul>
          )}
        </section>

        <section className="panel">
          <div className="panel-head">
            <h2>{modeTitle}</h2>
            <div className="btn-row">
              <Button
                type="button"
                variant={mode === "receive" ? "primary" : "ghost"}
                onClick={() => {
                  setMode("receive");
                  setQty("1");
                }}
              >
                Receive
              </Button>
              <Button
                type="button"
                variant={mode === "transfer" ? "primary" : "ghost"}
                onClick={() => {
                  setMode("transfer");
                  setQty("1");
                }}
              >
                Transfer
              </Button>
              <Button
                type="button"
                variant={mode === "adjust" ? "primary" : "ghost"}
                onClick={() => {
                  setMode("adjust");
                  setQty("-1");
                }}
              >
                Adjust
              </Button>
            </div>
          </div>
          <form className="stack-form" onSubmit={submitMovement}>
            <label>
              Variant
              <select className="input" value={variantId} onChange={(e) => setVariantId(e.target.value)} required>
                <option value="">Select…</option>
                {catalog.map((c) => (
                  <option key={c.variant_id} value={c.variant_id}>
                    {c.product_name} — {c.sku} (avail {c.available_qty})
                  </option>
                ))}
                {variants
                  .filter((v) => !catalog.some((c) => c.variant_id === v.id))
                  .map((v) => {
                    const p = products.find((x) => x.id === v.product_id);
                    return (
                      <option key={v.id} value={v.id}>
                        {p?.name ?? "Product"} — {v.sku} (no stock yet)
                      </option>
                    );
                  })}
              </select>
            </label>
            {mode === "transfer" ? (
              <label>
                To location
                <select className="input" value={toLocationId} onChange={(e) => setToLocationId(e.target.value)} required>
                  {locations
                    .filter((l) => l.id !== locationId)
                    .map((l) => (
                      <option key={l.id} value={l.id}>
                        {l.name} ({l.location_type})
                      </option>
                    ))}
                </select>
              </label>
            ) : null}
            <label>
              {mode === "adjust" ? "Qty delta (+/−)" : "Quantity"}
              <Input type="number" value={qty} onChange={(e) => setQty(e.target.value)} required />
            </label>
            {mode !== "transfer" ? (
              <label>
                {mode === "receive" ? "Note (optional)" : "Reason"}
                <Input
                  value={note}
                  onChange={(e) => setNote(e.target.value)}
                  placeholder={mode === "receive" ? "Delivery ref" : "Count correction"}
                />
              </label>
            ) : null}
            <Button type="submit" disabled={busy || !variantId}>
              {busy
                ? "Saving…"
                : mode === "receive"
                  ? "Receive"
                  : mode === "transfer"
                    ? "Transfer"
                    : "Post adjustment"}
            </Button>
          </form>
        </section>
      </div>

      <section className="panel">
        <h2>Balances</h2>
        {balances.length === 0 ? (
          <EmptyState
            title="No stock recorded"
            body="Receive stock below to create balances for this location."
            icon={ICONS.boxes}
          />
        ) : (
          <div className="table-wrap">
            <table className="data-table">
              <thead>
                <tr>
                  <th>Product</th>
                  <th>SKU</th>
                  <th>Physical</th>
                  <th>Available</th>
                  <th>Reserved</th>
                  <th>Price</th>
                </tr>
              </thead>
              <tbody>
                {balances.map((b) => (
                  <tr key={`${b.variant_id}-${b.location_id}`}>
                    <td>
                      <button type="button" className="linkish" onClick={() => setVariantId(b.variant_id)}>
                        {b.product_name}
                      </button>
                    </td>
                    <td>
                      <code>{b.sku}</code>
                    </td>
                    <td>{b.physical_qty}</td>
                    <td>{b.available_qty}</td>
                    <td>{b.reserved_qty}</td>
                    <td>KES {b.sell_price.toLocaleString()}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}
      </section>

      <section className="panel">
        <h2>Recent movements</h2>
        {movements.length === 0 ? (
          <EmptyState title="No movements yet" body="Receives, transfers, sales, and adjustments appear here." icon={ICONS.clock} />
        ) : (
          <ul className="part-list">
            {movements.map((m) => (
              <li key={m.id} className="part-card">
                <div className="part-head">
                  <div>
                    <strong>{m.product_name}</strong>
                    <p className="muted">
                      <code>{m.sku}</code> · {m.reason}
                    </p>
                  </div>
                  <Badge tone={m.qty_delta >= 0 ? "success" : "warning"}>
                    {m.qty_delta >= 0 ? `+${m.qty_delta}` : m.qty_delta}
                  </Badge>
                </div>
                <p className="muted">{new Date(m.created_at).toLocaleString()}</p>
              </li>
            ))}
          </ul>
        )}
      </section>
    </div>
  );
}
