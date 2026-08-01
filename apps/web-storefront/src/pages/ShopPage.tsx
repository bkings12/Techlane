import { useEffect, useMemo, useRef, useState } from "react";
import { Link, useSearchParams } from "react-router-dom";
import { useStorefront } from "../store/StorefrontContext";
import { ProductCard } from "../components/ProductCard";
import {
  catalogItemImageURL,
  storefrontBannerImageURL,
  type StorefrontBanner,
  type StorefrontCategory,
} from "../lib/api";

type CatNode = StorefrontCategory & { count: number; children: CatNode[] };

function buildShopCategoryTree(
  categories: StorefrontCategory[],
  counts: Map<string, number>,
): CatNode[] {
  const byId = new Map<string, CatNode>();
  for (const c of categories) {
    byId.set(c.id, { ...c, count: counts.get(c.name) ?? 0, children: [] });
  }
  const roots: CatNode[] = [];
  for (const c of categories) {
    const node = byId.get(c.id)!;
    if (c.parent_id && byId.has(c.parent_id)) {
      byId.get(c.parent_id)!.children.push(node);
    } else {
      roots.push(node);
    }
  }
  if (roots.length === categories.length && categories.some((c) => c.depth > 0)) {
    const stack: CatNode[] = [];
    const rebuilt: CatNode[] = [];
    for (const c of categories) {
      const node = byId.get(c.id)!;
      node.children = [];
      while (stack.length > c.depth) stack.pop();
      if (stack.length === 0) rebuilt.push(node);
      else stack[stack.length - 1]!.children.push(node);
      stack.push(node);
    }
    return rebuilt;
  }
  return roots;
}

/** Compact windowed page list: 1 … 4 5 [6] 7 8 … 20 */
function pageWindow(current: number, total: number): Array<number | "ellipsis"> {
  if (total <= 7) return Array.from({ length: total }, (_, i) => i + 1);
  const pages = new Set<number>();
  pages.add(1);
  pages.add(total);
  for (let p = current - 2; p <= current + 2; p++) {
    if (p >= 1 && p <= total) pages.add(p);
  }
  const sorted = [...pages].sort((a, b) => a - b);
  const out: Array<number | "ellipsis"> = [];
  for (let i = 0; i < sorted.length; i++) {
    const n = sorted[i]!;
    if (i > 0 && n - sorted[i - 1]! > 1) out.push("ellipsis");
    out.push(n);
  }
  return out;
}

type SortKey = "popular" | "name" | "price-asc" | "price-desc" | "newest";

export function ShopPage() {
  const { catalog, content, loadingShop, error, formatPrice } = useStorefront();
  // TODO: move to server-side pagination if catalog size becomes a performance problem
  const [searchParams, setSearchParams] = useSearchParams();
  const [query, setQuery] = useState(() => searchParams.get("q") ?? "");
  const brandFilter = searchParams.get("brand") ?? "";
  const categoryFilter = searchParams.get("category") ?? "";
  const [minPrice, setMinPrice] = useState("");
  const [maxPrice, setMaxPrice] = useState("");
  const [brandSearch, setBrandSearch] = useState("");
  const [sort, setSort] = useState<SortKey>("popular");
  const [pageSize, setPageSize] = useState(15);
  const [page, setPage] = useState(1);
  const [filtersOpen, setFiltersOpen] = useState(false);
  const [openWidgets, setOpenWidgets] = useState<Record<string, boolean>>({
    categories: true,
    price: true,
    brands: true,
    bestsellers: true,
  });
  const [expandedCats, setExpandedCats] = useState<Record<string, boolean>>({});
  const [bannerIdx, setBannerIdx] = useState(0);
  const catalogRef = useRef<HTMLDivElement>(null);

  const brands = useMemo(() => {
    // Facet counts ignore the active brand so staff can switch brands without
    // the list collapsing to a single option. Still respect category/price/q.
    const q = query.trim().toLowerCase();
    const min = minPrice ? Number(minPrice) : undefined;
    const max = maxPrice ? Number(maxPrice) : undefined;
    const counts = new Map<string, number>();
    for (const item of catalog) {
      if (categoryFilter && (item.category ?? "") !== categoryFilter) continue;
      if (min != null && !Number.isNaN(min) && item.sell_price < min) continue;
      if (max != null && !Number.isNaN(max) && item.sell_price > max) continue;
      if (q) {
        const hay = `${item.product_name} ${item.brand ?? ""} ${item.category ?? ""} ${item.sku}`.toLowerCase();
        if (!hay.includes(q)) continue;
      }
      const b = item.brand?.trim();
      if (!b) continue;
      counts.set(b, (counts.get(b) ?? 0) + 1);
    }
    return [...counts.entries()].sort((a, b) => a[0].localeCompare(b[0]));
  }, [catalog, query, categoryFilter, minPrice, maxPrice]);

  const categoryCounts = useMemo(() => {
    const counts = new Map<string, number>();
    for (const item of catalog) {
      const c = item.category?.trim();
      if (!c) continue;
      counts.set(c, (counts.get(c) ?? 0) + 1);
    }
    return counts;
  }, [catalog]);

  const categoryTree = useMemo(() => {
    // Prefer a unique name list driven by products that are actually for sale.
    // CMS can contain duplicate display names (same leaf under different parents)
    // which would otherwise render twice with inflated shared counts.
    const fromCatalog = [...categoryCounts.entries()]
      .filter(([, count]) => count > 0)
      .sort((a, b) => a[0].localeCompare(b[0]))
      .map(([name, count]) => ({
        id: name,
        name,
        path: name,
        depth: 0,
        count,
        children: [] as CatNode[],
      }));

    const fromCms = content?.categories ?? [];
    if (fromCms.length === 0) return fromCatalog;

    const tree = buildShopCategoryTree(fromCms, categoryCounts);
    const deduped: CatNode[] = [];
    const seen = new Set<string>();

    function walk(nodes: CatNode[]) {
      for (const node of nodes) {
        const name = node.name.trim();
        const kids = node.children.filter((c) => (categoryCounts.get(c.name) ?? 0) > 0);
        const count = categoryCounts.get(name) ?? 0;
        if (count <= 0 && kids.length === 0) continue;
        if (seen.has(name)) {
          // Already listed — still walk children under the first occurrence.
          walk(kids);
          continue;
        }
        seen.add(name);
        deduped.push({ ...node, count, children: [] });
        // Flatten children into the top-level list by name so duplicate parents
        // don't create a second "Phone Chargers" row.
        walk(kids);
      }
    }
    walk(tree);

    // Any catalog categories missing from CMS still appear.
    for (const row of fromCatalog) {
      if (!seen.has(row.name)) deduped.push(row);
    }
    return deduped;
  }, [content?.categories, categoryCounts]);

  const shopBanners = useMemo(() => {
    const all = content?.banners ?? [];
    const mid = all.filter((b) => b.placement === "mid" || b.placement === "promo_tile");
    if (mid.length > 0) return mid;
    return all.filter((b) => b.placement === "hero").slice(0, 3);
  }, [content?.banners]);

  useEffect(() => {
    const q = searchParams.get("q");
    if (q != null) setQuery(q);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [searchParams.get("q")]);

  useEffect(() => {
    if (shopBanners.length <= 1) return;
    const t = window.setInterval(() => setBannerIdx((i) => (i + 1) % shopBanners.length), 5000);
    return () => window.clearInterval(t);
  }, [shopBanners.length]);

  useEffect(() => {
    setPage(1);
  }, [query, brandFilter, categoryFilter, minPrice, maxPrice, sort, pageSize]);

  const filtered = useMemo(() => {
    const q = query.trim().toLowerCase();
    const min = minPrice ? Number(minPrice) : undefined;
    const max = maxPrice ? Number(maxPrice) : undefined;
    let list = catalog.filter((item) => {
      if (brandFilter && (item.brand ?? "") !== brandFilter) return false;
      if (categoryFilter && (item.category ?? "") !== categoryFilter) return false;
      if (min != null && !Number.isNaN(min) && item.sell_price < min) return false;
      if (max != null && !Number.isNaN(max) && item.sell_price > max) return false;
      if (!q) return true;
      const hay = `${item.product_name} ${item.brand ?? ""} ${item.category ?? ""} ${item.sku}`.toLowerCase();
      return hay.includes(q);
    });

    list = [...list];
    switch (sort) {
      case "name":
        list.sort((a, b) => a.product_name.localeCompare(b.product_name));
        break;
      case "price-asc":
        list.sort((a, b) => a.sell_price - b.sell_price);
        break;
      case "price-desc":
        list.sort((a, b) => b.sell_price - a.sell_price);
        break;
      case "newest":
        list.sort((a, b) => Number(b.new_arrival) - Number(a.new_arrival));
        break;
      default:
        break;
    }
    return list;
  }, [catalog, query, brandFilter, categoryFilter, minPrice, maxPrice, sort]);

  const totalPages = Math.max(1, Math.ceil(filtered.length / pageSize));
  const safePage = Math.min(page, totalPages);
  const rangeStart = filtered.length === 0 ? 0 : (safePage - 1) * pageSize + 1;
  const rangeEnd = Math.min(filtered.length, safePage * pageSize);
  const visible = filtered.slice((safePage - 1) * pageSize, safePage * pageSize);

  function goToPage(next: number) {
    const clamped = Math.max(1, Math.min(totalPages, next));
    setPage(clamped);
    catalogRef.current?.scrollIntoView({ behavior: "smooth", block: "start" });
  }

  function setParam(key: string, value: string) {
    const next = new URLSearchParams(searchParams);
    if (value) next.set(key, value);
    else next.delete(key);
    setSearchParams(next, { replace: true });
  }

  const activeFilterCount =
    (brandFilter ? 1 : 0) + (categoryFilter ? 1 : 0) + (minPrice || maxPrice ? 1 : 0);

  function clearFilters() {
    setMinPrice("");
    setMaxPrice("");
    const next = new URLSearchParams(searchParams);
    next.delete("brand");
    next.delete("category");
    setSearchParams(next, { replace: true });
  }

  function toggleWidget(key: string) {
    setOpenWidgets((prev) => ({ ...prev, [key]: !prev[key] }));
  }

  const heading = categoryFilter || brandFilter || "All products";
  const brandList = brands.filter(([name]) =>
    brandSearch.trim() ? name.toLowerCase().includes(brandSearch.trim().toLowerCase()) : true,
  );

  return (
    <div className="techno-shop-page">
      <div className="flat-breadcrumb">
        <div className="li-container">
          <ul className="breadcrumbs">
            <li>
              <Link to="/">Home</Link>
            </li>
            <li>
              <Link to="/shop">Shop</Link>
            </li>
            <li className="trail-end">
              <span>{heading}</span>
            </li>
          </ul>
        </div>
      </div>

      <div className={`shop techno-shop${filtersOpen ? " filters-open" : ""}`}>
        <aside className="shop-sidebar">
          <div className="shop-filters-bar">
            <button
              type="button"
              className="shop-filters-toggle"
              aria-expanded={filtersOpen}
              onClick={() => setFiltersOpen((v) => !v)}
            >
              {filtersOpen ? "Hide filters" : "Filter"}
              {activeFilterCount > 0 ? <span className="filter-count">{activeFilterCount}</span> : null}
            </button>
            {activeFilterCount > 0 ? (
              <button type="button" className="shop-filters-clear" onClick={clearFilters}>
                Clear
              </button>
            ) : null}
          </div>

          <div className="shop-sidebar-body">
            <div className={`widget widget-categories ${openWidgets.categories ? "is-open" : ""}`}>
              <button type="button" className="widget-title" onClick={() => toggleWidget("categories")}>
                <h3>Categories</h3>
                <span aria-hidden="true">{openWidgets.categories ? "−" : "+"}</span>
              </button>
              {openWidgets.categories ? (
                <ul className="cat-list">
                  <li>
                    <button
                      type="button"
                      className={categoryFilter === "" ? "is-active" : ""}
                      onClick={() => setParam("category", "")}
                    >
                      <span>All categories</span>
                      <i>({catalog.length})</i>
                    </button>
                  </li>
                  {categoryTree.map((cat) => (
                    <CategoryRow
                      key={cat.id}
                      node={cat}
                      active={categoryFilter}
                      expanded={expandedCats}
                      onToggleExpand={(id) =>
                        setExpandedCats((prev) => ({ ...prev, [id]: !prev[id] }))
                      }
                      onSelect={(name) => setParam("category", name)}
                    />
                  ))}
                </ul>
              ) : null}
            </div>

            <div className={`widget widget-price ${openWidgets.price ? "is-open" : ""}`}>
              <button type="button" className="widget-title" onClick={() => toggleWidget("price")}>
                <h3>Price</h3>
                <span aria-hidden="true">{openWidgets.price ? "−" : "+"}</span>
              </button>
              {openWidgets.price ? (
                <div className="widget-content price-box-filter">
                  <div className="price-inputs">
                    <input
                      type="number"
                      inputMode="decimal"
                      placeholder="Min"
                      value={minPrice}
                      onChange={(e) => setMinPrice(e.target.value)}
                      aria-label="Minimum price"
                    />
                    <span>—</span>
                    <input
                      type="number"
                      inputMode="decimal"
                      placeholder="Max"
                      value={maxPrice}
                      onChange={(e) => setMaxPrice(e.target.value)}
                      aria-label="Maximum price"
                    />
                  </div>
                </div>
              ) : null}
            </div>

            {brands.length > 0 ? (
              <div className={`widget widget-brands ${openWidgets.brands ? "is-open" : ""}`}>
                <button type="button" className="widget-title" onClick={() => toggleWidget("brands")}>
                  <h3>Brands</h3>
                  <span aria-hidden="true">{openWidgets.brands ? "−" : "+"}</span>
                </button>
                {openWidgets.brands ? (
                  <div className="widget-content">
                    <input
                      className="brand-search"
                      type="search"
                      value={brandSearch}
                      onChange={(e) => setBrandSearch(e.target.value)}
                      placeholder="Brands Search"
                      aria-label="Search brands"
                    />
                    <ul className="brand-list">
                      <li>
                        <label className={brandFilter === "" ? "is-active" : ""}>
                          <input
                            type="checkbox"
                            checked={brandFilter === ""}
                            onChange={() => setParam("brand", "")}
                          />
                          <span>All brands</span>
                        </label>
                      </li>
                      {brandList.map(([name, count]) => (
                        <li key={name}>
                          <label className={brandFilter === name ? "is-active" : ""}>
                            <input
                              type="checkbox"
                              checked={brandFilter === name}
                              onChange={() => setParam("brand", brandFilter === name ? "" : name)}
                            />
                            <span>
                              {name} <i>({count})</i>
                            </span>
                          </label>
                        </li>
                      ))}
                    </ul>
                  </div>
                ) : null}
              </div>
            ) : null}

            {(content?.bestsellers ?? []).slice(0, 5).length > 0 ? (
              <div className={`widget widget-products ${openWidgets.bestsellers ? "is-open" : ""}`}>
                <button type="button" className="widget-title" onClick={() => toggleWidget("bestsellers")}>
                  <h3>Best Seller</h3>
                  <span aria-hidden="true">{openWidgets.bestsellers ? "−" : "+"}</span>
                </button>
                {openWidgets.bestsellers ? (
                  <ul className="best-seller-list">
                    {(content?.bestsellers ?? []).slice(0, 5).map((item) => (
                      <li key={item.variant_id}>
                        <Link to={`/product/${item.variant_id}`} className="best-seller-item">
                          {catalogItemImageURL(item) ? (
                            <img src={catalogItemImageURL(item)} alt="" />
                          ) : (
                            <div className="thumb-empty" />
                          )}
                          <div>
                            <strong>{item.product_name}</strong>
                            <span className="price-now">{formatPrice(item.sell_price)}</span>
                          </div>
                        </Link>
                      </li>
                    ))}
                  </ul>
                ) : null}
              </div>
            ) : null}
          </div>
        </aside>

        <section className="main-shop">
          <ShopBanner banners={shopBanners} index={bannerIdx} onSelect={setBannerIdx} />

          <div className="shop-head">
            <h2 className="shop-title">{heading}</h2>
            <p className="shop-count">
              Showing {rangeStart}–{rangeEnd} of {filtered.length} results
            </p>
          </div>

          <div className="sort-product">
            <div className="sort">
              <div className="popularity">
                <select
                  value={sort}
                  onChange={(e) => setSort(e.target.value as SortKey)}
                  aria-label="Sort products"
                >
                  <option value="popular">Sort by popularity</option>
                  <option value="newest">Sort by newest</option>
                  <option value="name">Sort by name</option>
                  <option value="price-asc">Price: low to high</option>
                  <option value="price-desc">Price: high to low</option>
                </select>
              </div>
              <div className="showed">
                <select
                  value={pageSize}
                  onChange={(e) => setPageSize(Number(e.target.value))}
                  aria-label="Products per page"
                >
                  <option value={9}>Show 9</option>
                  <option value={15}>Show 15</option>
                  <option value={30}>Show 30</option>
                </select>
              </div>
            </div>
          </div>

          {error ? <p className="error">{error}</p> : null}
          {loadingShop ? (
            <div className="catalog limupa-catalog" ref={catalogRef}>
              <div className="skeleton" style={{ minHeight: "14rem" }} />
              <div className="skeleton" style={{ minHeight: "14rem" }} />
              <div className="skeleton" style={{ minHeight: "14rem" }} />
            </div>
          ) : filtered.length === 0 ? (
            <div className="empty-state" ref={catalogRef}>
              <strong>{catalog.length === 0 ? "No products yet" : "No matches"}</strong>
              <p>
                {catalog.length === 0
                  ? "Online catalog is empty for this branch."
                  : "Try clearing filters or searching a different term."}
              </p>
            </div>
          ) : (
            <>
              <div className="catalog limupa-catalog" ref={catalogRef}>
                {visible.map((item) => (
                  <ProductCard key={item.variant_id} item={item} />
                ))}
              </div>
              {totalPages > 1 ? (
                <nav className="shop-pagination" aria-label="Product pages">
                  <button
                    type="button"
                    className="shop-page-btn"
                    disabled={safePage <= 1}
                    onClick={() => goToPage(safePage - 1)}
                  >
                    Previous
                  </button>
                  <div className="shop-page-numbers">
                    {pageWindow(safePage, totalPages).map((item, idx) =>
                      item === "ellipsis" ? (
                        <span key={`e-${idx}`} className="shop-page-ellipsis" aria-hidden="true">
                          …
                        </span>
                      ) : (
                        <button
                          key={item}
                          type="button"
                          className={`shop-page-btn${item === safePage ? " is-active" : ""}`}
                          aria-current={item === safePage ? "page" : undefined}
                          onClick={() => goToPage(item)}
                        >
                          {item}
                        </button>
                      ),
                    )}
                  </div>
                  <span className="shop-page-status">
                    Page {safePage} of {totalPages}
                  </span>
                  <button
                    type="button"
                    className="shop-page-btn"
                    disabled={safePage >= totalPages}
                    onClick={() => goToPage(safePage + 1)}
                  >
                    Next
                  </button>
                </nav>
              ) : null}
            </>
          )}
        </section>
      </div>
    </div>
  );
}

function CategoryRow({
  node,
  active,
  expanded,
  onToggleExpand,
  onSelect,
}: {
  node: CatNode;
  active: string;
  expanded: Record<string, boolean>;
  onToggleExpand: (id: string) => void;
  onSelect: (name: string) => void;
}) {
  const hasKids = node.children.length > 0;
  const isOpen = Boolean(expanded[node.id]);

  return (
    <li className={hasKids ? "has-child" : ""}>
      <div className="cat-row">
        {hasKids ? (
          <button
            type="button"
            className={`cat-expand ${isOpen ? "is-open" : ""}`}
            aria-label={isOpen ? "Collapse" : "Expand"}
            onClick={() => onToggleExpand(node.id)}
          >
            ›
          </button>
        ) : (
          <span className="cat-expand-spacer" />
        )}
        <button
          type="button"
          className={active === node.name ? "is-active" : ""}
          onClick={() => onSelect(node.name)}
        >
          <span>{node.name}</span>
          <i>({node.count})</i>
        </button>
      </div>
      {hasKids && isOpen ? (
        <ul className="cat-child">
          {node.children.map((child) => (
            <CategoryRow
              key={child.id}
              node={child}
              active={active}
              expanded={expanded}
              onToggleExpand={onToggleExpand}
              onSelect={onSelect}
            />
          ))}
        </ul>
      ) : null}
    </li>
  );
}

function ShopBanner({
  banners,
  index,
  onSelect,
}: {
  banners: StorefrontBanner[];
  index: number;
  onSelect: (i: number) => void;
}) {
  if (banners.length === 0) {
    return (
      <div className="shop-banner shop-banner-fallback">
        <div className="shop-banner-copy">
          <h3>Shop Banner</h3>
          <p>Browse devices, parts, and accessories ready for pickup.</p>
        </div>
      </div>
    );
  }

  const b = banners[Math.min(index, banners.length - 1)]!;
  const img = b.has_image ? storefrontBannerImageURL(b.id, b.image_updated_at) : undefined;
  const href =
    b.deal_variant_id != null
      ? `/product/${b.deal_variant_id}`
      : b.cta_href || "/shop";

  return (
    <div className="shop-banner">
      <Link to={href} className="shop-banner-link">
        {img ? <img src={img} alt="" className="shop-banner-media" /> : null}
        <div className="shop-banner-copy">
          <h3>{b.headline || "Shop Banner"}</h3>
          {b.subtext ? <p>{b.subtext}</p> : null}
        </div>
      </Link>
      {banners.length > 1 ? (
        <div className="shop-banner-dots">
          {banners.map((x, i) => (
            <button
              key={x.id}
              type="button"
              className={i === index ? "is-active" : ""}
              aria-label={`Banner ${i + 1}`}
              onClick={() => onSelect(i)}
            />
          ))}
        </div>
      ) : null}
    </div>
  );
}
