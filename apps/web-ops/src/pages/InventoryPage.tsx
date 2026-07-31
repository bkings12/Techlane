import { useCallback, useEffect, useMemo, useState, type CSSProperties, type FormEvent } from "react";
import { Link } from "react-router-dom";
import { useBranch } from "../branch/BranchContext";
import { Badge, Button, EmptyState, ICONS, Input, PageHeader, SearchInput } from "../components/ui";
import {
  adjustStock,
  createCategory,
  createProduct,
  createVariant,
  deleteCategory,
  ensureStockLocations,
  listCategories,
  listProducts,
  listPOSCatalog,
  listStockBalances,
  listStockLocations,
  listStockMovements,
  listVariants,
  publishProduct,
  unpublishProduct,
  updateCategory,
  updateProduct,
  updateVariant,
  uploadProductImage,
  deleteProductImage,
  productImageURL,
  receiveStock,
  transferStock,
  type CatalogItem,
  type InventoryCategory,
  type Product,
  type StockBalance,
  type StockLocation,
  type StockMovement,
  type Variant,
} from "../lib/api";

type Mode = "receive" | "adjust" | "transfer";
type View = "products" | "categories" | "stock";
const LOW_STOCK_THRESHOLD = 3;
const MOVEMENTS_SHOWN = 40;

function generateSku(...parts: string[]) {
  const prefix = parts
    .join(" ")
    .normalize("NFKD")
    .replace(/[^a-zA-Z0-9 ]/g, " ")
    .trim()
    .split(/\s+/)
    .filter(Boolean)
    .slice(0, 3)
    .map((part) => part.slice(0, 4).toUpperCase())
    .join("-") || "ITEM";
  const bytes = new Uint16Array(1);
  crypto.getRandomValues(bytes);
  return prefix + "-" + bytes[0].toString(36).toUpperCase().padStart(3, "0");
}

function flattenCategoryTree(categories: InventoryCategory[]) {
  const byParent = new Map<string, InventoryCategory[]>();
  const roots: InventoryCategory[] = [];
  for (const c of categories) {
    if (c.parent_id) {
      const list = byParent.get(c.parent_id) ?? [];
      list.push(c);
      byParent.set(c.parent_id, list);
    } else {
      roots.push(c);
    }
  }
  const out: { node: InventoryCategory; depth: number }[] = [];
  function walk(list: InventoryCategory[], depth: number) {
    for (const c of list) {
      out.push({ node: c, depth });
      const kids = byParent.get(c.id);
      if (kids) walk(kids, depth + 1);
    }
  }
  walk(roots, 0);
  return out;
}

export function InventoryPage() {
  const { branchId } = useBranch();
  const [view, setView] = useState<View>("products");
  const [locations, setLocations] = useState<StockLocation[]>([]);
  const [locationId, setLocationId] = useState("");
  const [toLocationId, setToLocationId] = useState("");
  const [balances, setBalances] = useState<StockBalance[]>([]);
  const [movements, setMovements] = useState<StockMovement[]>([]);
  const [catalog, setCatalog] = useState<CatalogItem[]>([]);
  const [overviewBalances, setOverviewBalances] = useState<StockBalance[]>([]);
  const [products, setProducts] = useState<Product[]>([]);
  const [variants, setVariants] = useState<Variant[]>([]);
  const [categories, setCategories] = useState<InventoryCategory[]>([]);
  const [editProductId, setEditProductId] = useState<string | null>(null);
  const [editCategoryId, setEditCategoryId] = useState("");
  const [editDescription, setEditDescription] = useState("");
  const [editImageUrl, setEditImageUrl] = useState("");
  const [editFeatured, setEditFeatured] = useState(false);
  const [editNewArrival, setEditNewArrival] = useState(false);
  const [editBestseller, setEditBestseller] = useState(false);
  const [editSortOrder, setEditSortOrder] = useState("0");
  const [editVariantId, setEditVariantId] = useState<string | null>(null);
  const [editVariantSku, setEditVariantSku] = useState("");
  const [editVariantPrice, setEditVariantPrice] = useState("0");
  const [editVariantCost, setEditVariantCost] = useState("0");
  const [error, setError] = useState("");
  const [busy, setBusy] = useState(false);
  const [variantId, setVariantId] = useState("");
  const [qty, setQty] = useState("1");
  const [note, setNote] = useState("");
  const [mode, setMode] = useState<Mode>("receive");

  const [showNewProduct, setShowNewProduct] = useState(false);
  const [prodName, setProdName] = useState("");
  const [prodBrand, setProdBrand] = useState("");
  const [prodCategoryId, setProdCategoryId] = useState("");
  const [newSku, setNewSku] = useState("");
  const [newSellPrice, setNewSellPrice] = useState("0");
  const [newCostPrice, setNewCostPrice] = useState("0");
  const [newProductImage, setNewProductImage] = useState<File | null>(null);

  const [addVariantFor, setAddVariantFor] = useState<Product | null>(null);
  const [sku, setSku] = useState("");
  const [sellPrice, setSellPrice] = useState("0");
  const [costPrice, setCostPrice] = useState("0");

  const [balanceQuery, setBalanceQuery] = useState("");
  const [movementQuery, setMovementQuery] = useState("");
  const [catalogQuery, setCatalogQuery] = useState("");
  const [categoryFilter, setCategoryFilter] = useState("");

  const [showNewCategory, setShowNewCategory] = useState(false);
  const [newCatName, setNewCatName] = useState("");
  const [newCatParentId, setNewCatParentId] = useState("");
  const [editCatId, setEditCatId] = useState<string | null>(null);
  const [editCatName, setEditCatName] = useState("");
  const [editCatParentId, setEditCatParentId] = useState("");

  const refreshMeta = useCallback(async () => {
    let locs = (await listStockLocations(branchId || undefined)).items ?? [];
    if (locs.length === 0) {
      locs = (await ensureStockLocations()).items ?? [];
    }
    setLocations(locs);
    const counter = locs.find((l) => l.location_type === "counter") || locs[0];
    const store = locs.find((l) => l.location_type === "store") || locs.find((l) => l.id !== counter?.id);
    setLocationId((prev) => {
      if (prev && locs.some((l) => l.id === prev)) return prev;
      return counter?.id || "";
    });
    setToLocationId((prev) => {
      if (prev && locs.some((l) => l.id === prev)) return prev;
      return store?.id || locs.find((l) => l.id !== counter?.id)?.id || "";
    });
  }, [branchId]);

  const refreshCatalogAdmin = useCallback(async () => {
    const [p, v, c] = await Promise.all([listProducts(), listVariants(), listCategories()]);
    setProducts(p.items ?? []);
    setVariants(v.items ?? []);
    setCategories(c.items ?? []);
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

  const refreshOverview = useCallback(async () => {
    const b = await listStockBalances();
    setOverviewBalances(b.items ?? []);
  }, []);

  useEffect(() => {
    refreshMeta().catch((e) => setError(e instanceof Error ? e.message : "Failed to load"));
  }, [refreshMeta]);

  useEffect(() => {
    refreshCatalogAdmin().catch((e) => setError(e instanceof Error ? e.message : "Catalog failed"));
  }, [refreshCatalogAdmin]);

  useEffect(() => {
    refreshOverview().catch((e) => setError(e instanceof Error ? e.message : "Stock overview failed"));
  }, [refreshOverview]);

  useEffect(() => {
    if (!locationId) return;
    refreshStock(locationId).catch((e) => setError(e instanceof Error ? e.message : "Stock failed"));
  }, [locationId, refreshStock]);

  const categoryDescendants = useMemo(() => {
    const kids = new Map<string, string[]>();
    for (const c of categories) {
      if (!c.parent_id) continue;
      const list = kids.get(c.parent_id) ?? [];
      list.push(c.id);
      kids.set(c.parent_id, list);
    }
    const memo = new Map<string, Set<string>>();
    function subtree(id: string): Set<string> {
      const hit = memo.get(id);
      if (hit) return hit;
      const set = new Set<string>([id]);
      for (const child of kids.get(id) ?? []) {
        for (const x of subtree(child)) set.add(x);
      }
      memo.set(id, set);
      return set;
    }
    return subtree;
  }, [categories]);

  const categoryTree = useMemo(() => flattenCategoryTree(categories), [categories]);

  const overviewTotals = useMemo(() => {
    const available = overviewBalances.reduce((s, b) => s + b.available_qty, 0);
    const lowStock = overviewBalances.filter((b) => b.available_qty > 0 && b.available_qty <= LOW_STOCK_THRESHOLD).length;
    const outOfStock = overviewBalances.filter((b) => b.available_qty <= 0).length;
    return { available, lowStock, outOfStock };
  }, [overviewBalances]);

  const filteredBalances = useMemo(() => {
    const q = balanceQuery.trim().toLowerCase();
    if (!q) return balances;
    return balances.filter(
      (b) => b.product_name.toLowerCase().includes(q) || b.sku.toLowerCase().includes(q),
    );
  }, [balances, balanceQuery]);

  const filteredMovements = useMemo(() => {
    const q = movementQuery.trim().toLowerCase();
    const list = !q
      ? movements
      : movements.filter(
          (m) =>
            m.product_name.toLowerCase().includes(q) ||
            m.sku.toLowerCase().includes(q) ||
            m.reason.toLowerCase().includes(q),
        );
    return list.slice(0, MOVEMENTS_SHOWN);
  }, [movements, movementQuery]);

  const filteredProducts = useMemo(() => {
    const q = catalogQuery.trim().toLowerCase();
    const catIds = categoryFilter ? categoryDescendants(categoryFilter) : null;
    return products.filter((p) => {
      if (catIds) {
        if (!p.category_id || !catIds.has(p.category_id)) return false;
      }
      if (!q) return true;
      return (
        p.name.toLowerCase().includes(q) ||
        (p.brand ?? "").toLowerCase().includes(q) ||
        (p.category_path ?? p.category ?? "").toLowerCase().includes(q) ||
        variants.some((v) => v.product_id === p.id && v.sku.toLowerCase().includes(q))
      );
    });
  }, [products, variants, catalogQuery, categoryFilter, categoryDescendants]);

  function availableFor(variantId: string): number | undefined {
    const rows = overviewBalances.filter((b) => b.variant_id === variantId);
    if (rows.length === 0) return undefined;
    return rows.reduce((sum, b) => sum + b.available_qty, 0);
  }

  async function submitVariantEdit(e: FormEvent) {
    e.preventDefault();
    if (!editVariantId) return;
    setBusy(true);
    setError("");
    try {
      await updateVariant(editVariantId, {
        sku: editVariantSku.trim() || undefined,
        sell_price: Number(editVariantPrice) || 0,
        cost_price: Number(editVariantCost) || 0,
      });
      setEditVariantId(null);
      await Promise.all([refreshCatalogAdmin(), refreshStock(locationId), refreshOverview()]);
    } catch (err) {
      setError(err instanceof Error ? err.message : "Update variant failed");
    } finally {
      setBusy(false);
    }
  }

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
      await Promise.all([refreshStock(locationId), refreshOverview()]);
    } catch (err) {
      setError(err instanceof Error ? err.message : "Movement failed");
    } finally {
      setBusy(false);
    }
  }

  async function submitCategory(e: FormEvent) {
    e.preventDefault();
    const name = newCatName.trim();
    if (!name) {
      setError("Category name is required");
      return;
    }
    setBusy(true);
    setError("");
    try {
      await createCategory({
        name,
        parent_id: newCatParentId || undefined,
      });
      setNewCatName("");
      setShowNewCategory(false);
      await refreshCatalogAdmin();
    } catch (err) {
      setError(err instanceof Error ? err.message : "Create category failed");
    } finally {
      setBusy(false);
    }
  }

  async function submitRenameCategory(e: FormEvent) {
    e.preventDefault();
    if (!editCatId) return;
    const name = editCatName.trim();
    if (!name) {
      setError("Category name is required");
      return;
    }
    setBusy(true);
    setError("");
    try {
      const body: { name: string; parent_id?: string; clear_parent?: boolean } = { name };
      const current = categories.find((c) => c.id === editCatId);
      const nextParent = editCatParentId || "";
      const prevParent = current?.parent_id ?? "";
      if (nextParent !== prevParent) {
        if (!nextParent) body.clear_parent = true;
        else body.parent_id = nextParent;
      }
      await updateCategory(editCatId, body);
      setEditCatId(null);
      setEditCatName("");
      setEditCatParentId("");
      await refreshCatalogAdmin();
    } catch (err) {
      setError(err instanceof Error ? err.message : "Rename category failed");
    } finally {
      setBusy(false);
    }
  }

  async function submitProduct(e: FormEvent) {
    e.preventDefault();
    const name = prodName.trim();
    const skuCode = newSku.trim() || generateSku(prodBrand, name);
    if (!name) {
      setError("Product name is required");
      return;
    }
    setBusy(true);
    setError("");
    try {
      const product = await createProduct({
        name,
        brand: prodBrand.trim() || undefined,
        category_id: prodCategoryId || undefined,
      });
      await createVariant({
        product_id: product.id,
        sku: skuCode,
        sell_price: Number(newSellPrice) || 0,
        cost_price: Number(newCostPrice) || 0,
      });
      if (newProductImage) {
        await uploadProductImage(product.id, newProductImage);
      }
      setProdName("");
      setProdBrand("");
      setProdCategoryId("");
      setNewSku("");
      setNewSellPrice("0");
      setNewCostPrice("0");
      setNewProductImage(null);
      setShowNewProduct(false);
      await Promise.all([refreshCatalogAdmin(), refreshOverview()]);
    } catch (err) {
      setError(err instanceof Error ? err.message : "Create product failed");
    } finally {
      setBusy(false);
    }
  }

  async function submitVariant(e: FormEvent) {
    e.preventDefault();
    if (!addVariantFor) return;
    const skuCode = sku.trim() || generateSku(addVariantFor.brand || "", addVariantFor.name || "item");
    setBusy(true);
    setError("");
    try {
      await createVariant({
        product_id: addVariantFor.id,
        sku: skuCode,
        sell_price: Number(sellPrice) || 0,
        cost_price: Number(costPrice) || 0,
      });
      setSku("");
      setSellPrice("0");
      setCostPrice("0");
      setAddVariantFor(null);
      await Promise.all([refreshCatalogAdmin(), refreshOverview()]);
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
        subtitle="Catalog defines products, categories & SKUs. Stock floor receives and moves quantity."
        actions={
          <>
            {view === "products" ? (
              <Button
                type="button"
                onClick={() => {
                  setProdCategoryId(categoryFilter);
                  setShowNewProduct(true);
                }}
              >
                New product
              </Button>
            ) : view === "categories" ? (
              <Button
                type="button"
                onClick={() => {
                  setNewCatParentId("");
                  setShowNewCategory(true);
                }}
              >
                New category
              </Button>
            ) : null}
            <Link className="btn btn-ghost" to="/pos">
              Open POS
            </Link>
          </>
        }
      />
      {error ? <p className="form-error">{error}</p> : null}

      <section className="board-pulse" aria-label="Inventory pulse">
        <div>
          <strong>{products.length}</strong>
          <span>Products</span>
        </div>
        <div>
          <strong>{variants.length}</strong>
          <span>SKUs</span>
        </div>
        <div>
          <strong>{overviewTotals.available}</strong>
          <span>Units available</span>
        </div>
        <div className={overviewTotals.lowStock > 0 ? "warn" : ""}>
          <strong>{overviewTotals.lowStock}</strong>
          <span>Low stock</span>
        </div>
        <div className={overviewTotals.outOfStock > 0 ? "warn" : ""}>
          <strong>{overviewTotals.outOfStock}</strong>
          <span>Out of stock</span>
        </div>
      </section>

      <div className="lane-tabs" role="tablist" aria-label="Inventory sections">
        <button
          type="button"
          role="tab"
          aria-selected={view === "products"}
          className={view === "products" ? "active" : ""}
          onClick={() => setView("products")}
        >
          Products
        </button>
        <button
          type="button"
          role="tab"
          aria-selected={view === "categories"}
          className={view === "categories" ? "active" : ""}
          onClick={() => setView("categories")}
        >
          Categories
        </button>
        <button
          type="button"
          role="tab"
          aria-selected={view === "stock"}
          className={view === "stock" ? "active" : ""}
          onClick={() => setView("stock")}
        >
          Stock floor
        </button>
      </div>

      {view === "products" ? (
        <section className="panel">
          <div className="panel-head">
            <div>
              <h2>Products</h2>
              <span className="muted">{products.length} products · {variants.length} SKUs</span>
            </div>
            <div className="inv-toolbar">
              <select className="input" value={categoryFilter} onChange={(e) => setCategoryFilter(e.target.value)}>
                <option value="">All categories</option>
                {categories.map((c) => (
                  <option key={c.id} value={c.id}>
                    {" ".repeat(c.depth * 2)}
                    {c.path}
                  </option>
                ))}
              </select>
              <SearchInput
                value={catalogQuery}
                onChange={(e) => setCatalogQuery(e.target.value)}
                placeholder="Search product, brand or SKU…"
                aria-label="Search products"
              />
            </div>
          </div>

          {filteredProducts.length === 0 ? (
            <EmptyState
              title={products.length === 0 ? "No products yet" : "No matches"}
              body={
                products.length === 0
                  ? "Add your first product — name, category, and first SKU."
                  : "Try a different search or category filter."
              }
              icon={ICONS.package}
              action={
                products.length === 0 ? (
                  <Button type="button" onClick={() => setShowNewProduct(true)}>
                    New product
                  </Button>
                ) : undefined
              }
            />
          ) : (
            <ul className="inv-product-grid">
              {filteredProducts.map((p) => {
                const pVariants = variants.filter((v) => v.product_id === p.id);
                const online = Boolean(p.online_visible);
                const editing = editProductId === p.id;
                return (
                  <li key={p.id} className="part-card">
                    <div className="part-head">
                      <div>
                        <strong>{p.name}</strong>
                        {p.brand ? <span className="muted"> · {p.brand}</span> : null}
                        {p.category_path || p.category ? (
                          <div className="muted">Category: {p.category_path || p.category}</div>
                        ) : null}
                      </div>
                      <Badge tone={online ? "success" : "pending"}>
                        {online ? "online" : "offline"}
                      </Badge>
                    </div>

                    {pVariants.length > 0 ? (
                      <ul className="variant-list">
                        {pVariants.map((v) => {
                          const avail = availableFor(v.id);
                          const editingVariant = editVariantId === v.id;
                          return (
                            <li key={v.id} className="variant-row">
                              {editingVariant ? (
                                <form className="variant-edit-form" onSubmit={submitVariantEdit}>
                                  <Input
                                    value={editVariantSku}
                                    onChange={(e) => setEditVariantSku(e.target.value)}
                                    className="mono"
                                    required
                                  />
                                  <Input
                                    type="number"
                                    value={editVariantPrice}
                                    onChange={(e) => setEditVariantPrice(e.target.value)}
                                    aria-label="Sell price"
                                    required
                                  />
                                  <Input
                                    type="number"
                                    value={editVariantCost}
                                    onChange={(e) => setEditVariantCost(e.target.value)}
                                    aria-label="Cost price"
                                    placeholder="Cost"
                                  />
                                  <Button type="submit" disabled={busy}>
                                    Save
                                  </Button>
                                  <Button type="button" variant="ghost" onClick={() => setEditVariantId(null)}>
                                    Cancel
                                  </Button>
                                </form>
                              ) : (
                                <>
                                  <code>{v.sku}</code>
                                  <span className="muted">KES {v.sell_price.toLocaleString()}</span>
                                  {v.cost_price > 0 ? (
                                    <span className="muted">cost {v.cost_price.toLocaleString()}</span>
                                  ) : (
                                    <Badge tone="warning">no cost price</Badge>
                                  )}
                                  {avail !== undefined ? (
                                    <Badge tone={avail <= 0 ? "danger" : avail <= LOW_STOCK_THRESHOLD ? "warning" : "info"}>
                                      {avail} in stock
                                    </Badge>
                                  ) : (
                                    <span className="muted">no stock yet</span>
                                  )}
                                  <button
                                    type="button"
                                    className="linkish"
                                    onClick={() => {
                                      setEditVariantId(v.id);
                                      setEditVariantSku(v.sku);
                                      setEditVariantPrice(String(v.sell_price));
                                      setEditVariantCost(String(v.cost_price ?? 0));
                                    }}
                                  >
                                    Edit prices
                                  </button>
                                </>
                              )}
                            </li>
                          );
                        })}
                      </ul>
                    ) : (
                      <p className="muted">No SKU variants yet.</p>
                    )}

                    {editing ? (
                      <form
                        className="form-grid"
                        onSubmit={(e) => {
                          e.preventDefault();
                          setBusy(true);
                          setError("");
                          updateProduct(p.id, {
                            category_id: editCategoryId || undefined,
                            clear_category: !editCategoryId,
                            description: editDescription.trim() || undefined,
                            image_url: editImageUrl.trim() || undefined,
                            featured: editFeatured,
                            new_arrival: editNewArrival,
                            bestseller: editBestseller,
                            storefront_sort_order: Number(editSortOrder) || 0,
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
                          <select
                            className="input"
                            value={editCategoryId}
                            onChange={(e) => setEditCategoryId(e.target.value)}
                          >
                            <option value="">— Uncategorized —</option>
                            {categories.map((c) => (
                              <option key={c.id} value={c.id}>
                                {c.path}
                              </option>
                            ))}
                          </select>
                        </label>
                        <label>
                          Description
                          <Input value={editDescription} onChange={(e) => setEditDescription(e.target.value)} />
                        </label>
                        <label>
                          Product photo
                          <div className="logo-row" style={{ marginTop: 8 }}>
                            <div className="logo-frame">
                              {p.has_image ? (
                                <img
                                  src={productImageURL(p.id, p.image_updated_at)}
                                  alt=""
                                  style={{ maxWidth: "100%", maxHeight: 96, objectFit: "contain" }}
                                />
                              ) : (
                                <span className="logo-empty">No photo</span>
                              )}
                            </div>
                            <div className="logo-actions">
                              <input
                                type="file"
                                accept="image/png,image/jpeg,image/webp"
                                style={{ display: "none" }}
                                id={`product-image-${p.id}`}
                                onChange={(e) => {
                                  const file = e.target.files?.[0];
                                  if (!file) return;
                                  setBusy(true);
                                  setError("");
                                  uploadProductImage(p.id, file)
                                    .then(() => refreshCatalogAdmin())
                                    .catch((err) =>
                                      setError(err instanceof Error ? err.message : "Image upload failed"),
                                    )
                                    .finally(() => {
                                      setBusy(false);
                                      e.target.value = "";
                                    });
                                }}
                              />
                              <Button
                                type="button"
                                variant="secondary"
                                disabled={busy}
                                onClick={() => document.getElementById(`product-image-${p.id}`)?.click()}
                              >
                                {p.has_image ? "Replace photo" : "Upload photo"}
                              </Button>
                              {p.has_image ? (
                                <Button
                                  type="button"
                                  variant="secondary"
                                  disabled={busy}
                                  onClick={() => {
                                    setBusy(true);
                                    setError("");
                                    deleteProductImage(p.id)
                                      .then(() => refreshCatalogAdmin())
                                      .catch((err) =>
                                        setError(err instanceof Error ? err.message : "Could not remove photo"),
                                      )
                                      .finally(() => setBusy(false));
                                  }}
                                >
                                  Remove
                                </Button>
                              ) : null}
                              <p className="hint">PNG, JPEG or WebP up to 2&nbsp;MB. Shows on shop and deals.</p>
                            </div>
                          </div>
                        </label>
                        <label>
                          Image URL (optional fallback)
                          <Input value={editImageUrl} onChange={(e) => setEditImageUrl(e.target.value)} />
                        </label>
                        <label className="checkbox-row">
                          <input
                            type="checkbox"
                            checked={editFeatured}
                            onChange={(e) => setEditFeatured(e.target.checked)}
                          />
                          Featured on storefront
                        </label>
                        <label className="checkbox-row">
                          <input
                            type="checkbox"
                            checked={editNewArrival}
                            onChange={(e) => setEditNewArrival(e.target.checked)}
                          />
                          New arrival
                        </label>
                        <label className="checkbox-row">
                          <input
                            type="checkbox"
                            checked={editBestseller}
                            onChange={(e) => setEditBestseller(e.target.checked)}
                          />
                          Bestseller
                        </label>
                        <label>
                          Storefront sort order
                          <Input
                            type="number"
                            value={editSortOrder}
                            onChange={(e) => setEditSortOrder(e.target.value)}
                          />
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
                            setSku("");
                            setSellPrice("0");
                            setCostPrice("0");
                            setAddVariantFor(p);
                          }}
                        >
                          Add SKU
                        </Button>
                        <Button
                          type="button"
                          variant="ghost"
                          disabled={busy}
                          onClick={() => {
                            setEditProductId(p.id);
                            setEditCategoryId(p.category_id ?? "");
                            setEditDescription(p.description ?? "");
                            setEditImageUrl(p.image_url ?? "");
                            setEditFeatured(Boolean(p.featured));
                            setEditNewArrival(Boolean(p.new_arrival));
                            setEditBestseller(Boolean(p.bestseller));
                            setEditSortOrder(String(p.storefront_sort_order ?? 0));
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
      ) : null}

      {view === "categories" ? (
        <section className="panel">
          <div className="panel-head">
            <div>
              <h2>Categories</h2>
              <span className="muted">{categories.length} categories</span>
            </div>
          </div>
          <p className="muted" style={{ marginTop: 0 }}>
            Build a category tree (e.g. Screens → iPhone → OLED), then add products under a leaf.
            Variants cover sizes/colours. Receive quantity on the Stock floor.
          </p>

          {categories.length === 0 ? (
            <EmptyState
              title="No categories yet"
              body="Add Screens, Accessories, etc. to organize your catalog."
              icon={ICONS.hash}
              action={
                <Button type="button" onClick={() => { setNewCatParentId(""); setShowNewCategory(true); }}>
                  New category
                </Button>
              }
            />
          ) : (
            <ul className="inv-tree">
              {categoryTree.map(({ node: c, depth }) => (
                <li
                  key={c.id}
                  className={`inv-tree-row ${depth > 0 ? "inv-tree-child" : ""}`}
                  style={{ "--depth": depth } as CSSProperties}
                >
                  {editCatId === c.id ? (
                    <div className="inv-tree-card">
                      <form className="form-grid" onSubmit={submitRenameCategory} style={{ margin: 0, flex: 1 }}>
                        <label>
                          Name
                          <Input
                            value={editCatName}
                            onChange={(e) => setEditCatName(e.target.value)}
                            required
                            autoFocus
                          />
                        </label>
                        <label>
                          Parent
                          <select
                            className="input"
                            value={editCatParentId}
                            onChange={(e) => setEditCatParentId(e.target.value)}
                          >
                            <option value="">— Top level —</option>
                            {categories
                              .filter((p) => p.id !== c.id)
                              .map((p) => (
                                <option key={p.id} value={p.id}>
                                  {p.path}
                                </option>
                              ))}
                          </select>
                        </label>
                        <div className="btn-row">
                          <Button type="submit" disabled={busy}>
                            Save
                          </Button>
                          <Button
                            type="button"
                            variant="ghost"
                            onClick={() => {
                              setEditCatId(null);
                              setEditCatName("");
                              setEditCatParentId("");
                            }}
                          >
                            Cancel
                          </Button>
                        </div>
                      </form>
                    </div>
                  ) : (
                    <div className="inv-tree-card">
                      <div>
                        <strong>{c.name}</strong>
                        {depth > 0 ? <div className="muted">{c.path}</div> : null}
                      </div>
                      <div className="btn-row">
                        <Button
                          type="button"
                          variant="ghost"
                          onClick={() => {
                            setEditCatId(c.id);
                            setEditCatName(c.name);
                            setEditCatParentId(c.parent_id ?? "");
                          }}
                        >
                          Rename
                        </Button>
                        <Button
                          type="button"
                          variant="ghost"
                          onClick={() => {
                            setNewCatParentId(c.id);
                            setShowNewCategory(true);
                          }}
                        >
                          Add child
                        </Button>
                        <Button
                          type="button"
                          variant="ghost"
                          disabled={busy}
                          onClick={() => {
                            if (!window.confirm(`Delete category “${c.name}”?`)) return;
                            setBusy(true);
                            setError("");
                            deleteCategory(c.id)
                              .then(() => refreshCatalogAdmin())
                              .catch((err) =>
                                setError(err instanceof Error ? err.message : "Delete failed"),
                              )
                              .finally(() => setBusy(false));
                          }}
                        >
                          Delete
                        </Button>
                      </div>
                    </div>
                  )}
                </li>
              ))}
            </ul>
          )}
        </section>
      ) : null}

      {view === "stock" ? (
        <>
          <section className="panel">
            <div className="panel-head">
              <div>
                <h2>Stock floor</h2>
                <span className="muted">Receive, transfer, and adjust quantity at a location</span>
              </div>
              <label>
                Viewing location
                <select className="input" value={locationId} onChange={(e) => setLocationId(e.target.value)}>
                  {locations.length === 0 ? <option value="">No locations yet</option> : null}
                  {locations.map((l) => (
                    <option key={l.id} value={l.id}>
                      {l.name} ({l.location_type})
                    </option>
                  ))}
                </select>
              </label>
            </div>
          </section>

          <div className="inv-stock-layout">
            <section className="panel">
              <div className="panel-head">
                <h2>Balances</h2>
                <SearchInput
                  value={balanceQuery}
                  onChange={(e) => setBalanceQuery(e.target.value)}
                  placeholder="Find product or SKU…"
                  aria-label="Find product or SKU in balances"
                  style={{ maxWidth: 220 }}
                />
              </div>
              {filteredBalances.length === 0 ? (
                <EmptyState
                  title={balances.length === 0 ? "No stock recorded" : "No matches"}
                  body={
                    balances.length === 0
                      ? products.length === 0
                        ? "First create a product under Products, then come back here to receive stock."
                        : "Receive stock on the right to create balances for this location."
                      : "Try a different search."
                  }
                  icon={ICONS.boxes}
                  action={
                    products.length === 0 ? (
                      <Button type="button" onClick={() => setView("products")}>
                        Go to Products
                      </Button>
                    ) : undefined
                  }
                />
              ) : (
                <div className="table-wrap">
                  <table className="table">
                    <thead>
                      <tr>
                        <th>Product</th>
                        <th>SKU</th>
                        <th>Physical</th>
                        <th>Available</th>
                        <th>Price</th>
                      </tr>
                    </thead>
                    <tbody>
                      {filteredBalances.map((b) => {
                        const selected = variantId === b.variant_id;
                        return (
                          <tr
                            key={`${b.variant_id}-${b.location_id}`}
                            className={`inv-balance-row ${selected ? "is-selected" : ""}`}
                            tabIndex={0}
                            role="button"
                            aria-pressed={selected}
                            onClick={() => setVariantId(b.variant_id)}
                            onKeyDown={(e) => {
                              if (e.key === "Enter" || e.key === " ") {
                                e.preventDefault();
                                setVariantId(b.variant_id);
                              }
                            }}
                          >
                            <td>{b.product_name}</td>
                            <td>
                              <code>{b.sku}</code>
                            </td>
                            <td>{b.physical_qty}</td>
                            <td>
                              {b.available_qty <= LOW_STOCK_THRESHOLD ? (
                                <Badge tone={b.available_qty <= 0 ? "danger" : "warning"}>
                                  {b.available_qty}
                                </Badge>
                              ) : (
                                b.available_qty
                              )}
                            </td>
                            <td className="mono">KES {b.sell_price.toLocaleString()}</td>
                          </tr>
                        );
                      })}
                    </tbody>
                  </table>
                </div>
              )}
            </section>

            <aside className="inv-movement-rail">
              <section className="panel">
                <div className="panel-head">
                  <h2>{modeTitle}</h2>
                </div>
                <div className="lane-tabs" role="tablist" aria-label="Stock movement type" style={{ marginBottom: "0.75rem" }}>
                  <button
                    type="button"
                    role="tab"
                    aria-selected={mode === "receive"}
                    className={mode === "receive" ? "active" : ""}
                    onClick={() => {
                      setMode("receive");
                      setQty("1");
                    }}
                  >
                    Receive
                  </button>
                  <button
                    type="button"
                    role="tab"
                    aria-selected={mode === "transfer"}
                    className={mode === "transfer" ? "active" : ""}
                    onClick={() => {
                      setMode("transfer");
                      setQty("1");
                    }}
                  >
                    Transfer
                  </button>
                  <button
                    type="button"
                    role="tab"
                    aria-selected={mode === "adjust"}
                    className={mode === "adjust" ? "active" : ""}
                    onClick={() => {
                      setMode("adjust");
                      setQty("-1");
                    }}
                  >
                    Adjust
                  </button>
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
            </aside>
          </div>

          <section className="panel">
            <div className="panel-head">
              <h2>Recent movements</h2>
              <SearchInput
                value={movementQuery}
                onChange={(e) => setMovementQuery(e.target.value)}
                placeholder="Find product, SKU, or reason…"
                aria-label="Find product, SKU, or reason in movements"
                style={{ maxWidth: 260 }}
              />
            </div>
            {filteredMovements.length === 0 ? (
              <EmptyState
                title={movements.length === 0 ? "No movements yet" : "No matches"}
                body={
                  movements.length === 0
                    ? "Receives, transfers, sales, and adjustments appear here."
                    : "Try a different search."
                }
                icon={ICONS.clock}
              />
            ) : (
              <>
                <div className="table-wrap">
                  <table className="table">
                    <thead>
                      <tr>
                        <th>Product</th>
                        <th>SKU</th>
                        <th>Reason</th>
                        <th>Qty</th>
                        <th>When</th>
                      </tr>
                    </thead>
                    <tbody>
                      {filteredMovements.map((m) => (
                        <tr key={m.id}>
                          <td>{m.product_name}</td>
                          <td>
                            <code>{m.sku}</code>
                          </td>
                          <td>{m.reason}</td>
                          <td>
                            <Badge tone={m.qty_delta >= 0 ? "success" : "warning"}>
                              {m.qty_delta >= 0 ? `+${m.qty_delta}` : m.qty_delta}
                            </Badge>
                          </td>
                          <td className="muted">{new Date(m.created_at).toLocaleString()}</td>
                        </tr>
                      ))}
                    </tbody>
                  </table>
                </div>
                {movements.length > filteredMovements.length ? (
                  <p className="hint">Showing the {MOVEMENTS_SHOWN} most recent matches.</p>
                ) : null}
              </>
            )}
          </section>
        </>
      ) : null}

      {showNewProduct ? (
        <div
          className="cmdk-backdrop"
          role="presentation"
          onClick={() => {
            if (!busy) setShowNewProduct(false);
          }}
        >
          <div className="cmdk-panel" role="dialog" aria-modal="true" aria-label="New product" onClick={(e) => e.stopPropagation()}>
            <div className="panel-head" style={{ padding: "1rem 1.25rem 0" }}>
              <h2 style={{ margin: 0 }}>New product</h2>
            </div>
            <form className="form-grid inv-modal-body" style={{ padding: "1rem 1.25rem 1.25rem" }} onSubmit={submitProduct}>
              <label>
                Product name
                <Input value={prodName} onChange={(e) => setProdName(e.target.value)} required autoFocus placeholder="e.g. Tempered glass" />
              </label>
              <label>
                Brand
                <Input value={prodBrand} onChange={(e) => setProdBrand(e.target.value)} placeholder="Optional" />
              </label>
              <label>
                Category
                <select className="input" value={prodCategoryId} onChange={(e) => setProdCategoryId(e.target.value)}>
                  <option value="">— Uncategorized —</option>
                  {categories.map((c) => (
                    <option key={c.id} value={c.id}>
                      {" ".repeat(c.depth * 2)}
                      {c.path}
                    </option>
                  ))}
                </select>
              </label>
              <label>
                SKU (optional)
                <Input
                  value={newSku}
                  onChange={(e) => setNewSku(e.target.value)}
                  className="mono"
                  placeholder="Leave blank to generate automatically"
                />
                <span className="hint">Generated from the brand and product name when left blank.</span>
              </label>
              <label>
                Sell price (KES)
                <Input type="number" value={newSellPrice} onChange={(e) => setNewSellPrice(e.target.value)} required />
              </label>
              <label>
                Cost price (KES)
                <Input type="number" value={newCostPrice} onChange={(e) => setNewCostPrice(e.target.value)} />
                <span className="hint">What you pay — used for repair margins.</span>
              </label>
              <label>
                Product photo (optional)
                <input
                  type="file"
                  accept="image/png,image/jpeg,image/webp"
                  disabled={busy}
                  onChange={(e) => setNewProductImage(e.target.files?.[0] ?? null)}
                />
                <span className="hint">Shown on the shop, deals, and product page. You can replace it later from the product row.</span>
              </label>
              {error ? <p className="form-error">{error}</p> : null}
              <div className="btn-row">
                <Button type="submit" disabled={busy}>
                  {busy ? "Creating…" : "Create product + SKU"}
                </Button>
                <Button
                  type="button"
                  variant="secondary"
                  disabled={busy}
                  onClick={() => {
                    setShowNewProduct(false);
                    setNewProductImage(null);
                  }}
                >
                  Cancel
                </Button>
              </div>
            </form>
          </div>
        </div>
      ) : null}

      {addVariantFor ? (
        <div
          className="cmdk-backdrop"
          role="presentation"
          onClick={() => {
            if (!busy) setAddVariantFor(null);
          }}
        >
          <div className="cmdk-panel" role="dialog" aria-modal="true" aria-label={`Add SKU to ${addVariantFor.name}`} onClick={(e) => e.stopPropagation()}>
            <div className="panel-head" style={{ padding: "1rem 1.25rem 0" }}>
              <h2 style={{ margin: 0 }}>Add SKU</h2>
              <span className="muted">{addVariantFor.name}</span>
            </div>
            <form className="form-grid inv-modal-body" style={{ padding: "1rem 1.25rem 1.25rem" }} onSubmit={submitVariant}>
              <label>
                SKU (optional)
                <Input value={sku} onChange={(e) => setSku(e.target.value)} className="mono" placeholder="Leave blank to generate automatically" autoFocus />
                <span className="hint">A unique SKU will be created automatically.</span>
              </label>
              <label>
                Sell price (KES)
                <Input type="number" value={sellPrice} onChange={(e) => setSellPrice(e.target.value)} required />
              </label>
              <label>
                Cost price (KES)
                <Input type="number" value={costPrice} onChange={(e) => setCostPrice(e.target.value)} />
              </label>
              {error ? <p className="form-error">{error}</p> : null}
              <div className="btn-row">
                <Button type="submit" disabled={busy}>
                  {busy ? "Adding…" : "Add SKU"}
                </Button>
                <Button type="button" variant="secondary" disabled={busy} onClick={() => setAddVariantFor(null)}>
                  Cancel
                </Button>
              </div>
            </form>
          </div>
        </div>
      ) : null}

      {showNewCategory ? (
        <div
          className="cmdk-backdrop"
          role="presentation"
          onClick={() => {
            if (!busy) setShowNewCategory(false);
          }}
        >
          <div className="cmdk-panel" role="dialog" aria-modal="true" aria-label="New category" onClick={(e) => e.stopPropagation()}>
            <div className="panel-head" style={{ padding: "1rem 1.25rem 0" }}>
              <h2 style={{ margin: 0 }}>New category</h2>
            </div>
            <form className="form-grid inv-modal-body" style={{ padding: "1rem 1.25rem 1.25rem" }} onSubmit={submitCategory}>
              <label>
                Category name
                <Input value={newCatName} onChange={(e) => setNewCatName(e.target.value)} required autoFocus placeholder="e.g. Screens or iPhone" />
              </label>
              <label>
                Parent category
                <select className="input" value={newCatParentId} onChange={(e) => setNewCatParentId(e.target.value)}>
                  <option value="">— Top level —</option>
                  {categories.map((c) => (
                    <option key={c.id} value={c.id}>
                      {" ".repeat(c.depth * 2)}
                      {c.path}
                    </option>
                  ))}
                </select>
                <span className="hint">Pick a parent to create a child category.</span>
              </label>
              {error ? <p className="form-error">{error}</p> : null}
              <div className="btn-row">
                <Button type="submit" disabled={busy}>
                  {busy ? "Adding…" : "Add category"}
                </Button>
                <Button type="button" variant="secondary" disabled={busy} onClick={() => setShowNewCategory(false)}>
                  Cancel
                </Button>
              </div>
            </form>
          </div>
        </div>
      ) : null}
    </div>
  );
}
