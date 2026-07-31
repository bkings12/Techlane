import { useEffect, useMemo, useRef, useState, type FormEvent } from "react";
import { Link, NavLink, Outlet, useNavigate } from "react-router-dom";
import { useStorefront } from "../store/StorefrontContext";
import { catalogItemImageURL, storefrontLogoURL, subscribeNewsletter, type StorefrontCategory } from "../lib/api";
import { loadWishlist, onWishlistChanged, toggleWishlist } from "../lib/wishlist";

type CategoryNode = StorefrontCategory & { children: CategoryNode[] };

function buildCategoryTree(categories: StorefrontCategory[]): CategoryNode[] {
  const byId = new Map<string, CategoryNode>();
  for (const c of categories) {
    byId.set(c.id, { ...c, children: [] });
  }
  const roots: CategoryNode[] = [];
  for (const c of categories) {
    const node = byId.get(c.id)!;
    if (c.parent_id && byId.has(c.parent_id)) {
      byId.get(c.parent_id)!.children.push(node);
    } else {
      roots.push(node);
    }
  }
  // If API only sends flat depth without parent_id, rebuild from depth order.
  if (roots.length === categories.length && categories.some((c) => c.depth > 0)) {
    const stack: CategoryNode[] = [];
    const rebuilt: CategoryNode[] = [];
    for (const c of categories) {
      const node = byId.get(c.id)!;
      node.children = [];
      while (stack.length > c.depth) stack.pop();
      if (stack.length === 0) {
        rebuilt.push(node);
      } else {
        stack[stack.length - 1]!.children.push(node);
      }
      stack.push(node);
    }
    return rebuilt;
  }
  return roots;
}

function useShopBrand() {
  const { boot, content } = useStorefront();
  const s = content?.settings;
  const name = (s?.shop_display_name || boot?.tenant_name || "Shop").trim() || "Shop";
  const logoSrc =
    s?.has_logo && boot?.tenant_id
      ? storefrontLogoURL(boot.tenant_id, s.logo_updated_at)
      : null;
  return { name, logoSrc, settings: s };
}

function useThemeFromCMS() {
  const { content, boot } = useStorefront();
  const s = content?.settings;

  useEffect(() => {
    const root = document.documentElement;
    const primary = s?.color_primary || "#F2BE2A";
    const secondary = s?.color_secondary || "#1A1A1A";
    const accent = s?.color_accent || "#060386";
    root.style.setProperty("--sf-primary", primary);
    root.style.setProperty("--sf-secondary", secondary);
    root.style.setProperty("--sf-accent", accent);
    root.style.setProperty("--accent", primary);
    root.style.setProperty("--accent-hot", primary);
    root.style.setProperty("--tertiary", secondary);

    const shopName = (s?.shop_display_name || boot?.tenant_name || "Shop").trim() || "Shop";
    document.title = (s?.page_title || `${shopName} · Online store`).trim();
  }, [s, boot?.tenant_name]);
}

function BrandMark() {
  const { name } = useShopBrand();
  return (
    <Link className="limupa-logo" to="/" aria-label={name}>
      <img className="brand-mark" src="/brand/mark.svg" alt="" width={36} height={36} />
      <span className="accent">TechLane</span>
    </Link>
  );
}

type MegaColumn = {
  key: string;
  title: string;
  links: CategoryNode[];
  shopAllName: string | null;
};

function megaColumnsFor(cat: CategoryNode): MegaColumn[] {
  const kids = cat.children;
  if (kids.length === 0) {
    return [{ key: cat.id, title: cat.name, links: [], shopAllName: cat.name }];
  }

  const hasGroups = kids.some((k) => k.children.length > 0);
  if (hasGroups) {
    return kids.map((k) => ({
      key: k.id,
      title: k.name,
      links: k.children,
      shopAllName: k.name,
    }));
  }

  const colCount = Math.min(3, Math.max(1, Math.ceil(kids.length / 6)));
  const size = Math.ceil(kids.length / colCount);
  const cols: MegaColumn[] = [];
  for (let i = 0; i < colCount; i++) {
    const slice = kids.slice(i * size, (i + 1) * size);
    if (slice.length === 0) continue;
    cols.push({
      key: `${cat.id}-${i}`,
      title: i === 0 ? cat.name : "",
      links: slice,
      shopAllName: i === 0 ? cat.name : null,
    });
  }
  return cols;
}

function CategoryGlyph({ name }: { name: string }) {
  const letter = (name.trim()[0] || "?").toUpperCase();
  return (
    <span className="hb-mega-glyph" aria-hidden="true">
      {letter}
    </span>
  );
}

function CategoryMenu() {
  const { content } = useStorefront();
  const [open, setOpen] = useState(false);
  const [activeId, setActiveId] = useState<string | null>(null);
  const [expandedMobile, setExpandedMobile] = useState<Record<string, boolean>>({});
  const tree = useMemo(() => buildCategoryTree(content?.categories ?? []), [content?.categories]);

  useEffect(() => {
    if (!open) {
      setActiveId(null);
      setExpandedMobile({});
      return;
    }
    setActiveId((prev) => {
      if (prev && tree.some((c) => c.id === prev)) return prev;
      return tree[0]?.id ?? null;
    });
  }, [open, tree]);

  if (tree.length === 0) {
    return (
      <div className="hb-categories">
        <button type="button" className="hb-categories-btn" disabled>
          <span className="hb-categories-burger" aria-hidden="true">
            <span />
          </span>
          All Categories
        </button>
      </div>
    );
  }

  return (
    <div
      className={`hb-categories ${open ? "is-open" : ""}`}
      onMouseEnter={() => setOpen(true)}
      onMouseLeave={() => setOpen(false)}
    >
      <button
        type="button"
        className="hb-categories-btn"
        aria-expanded={open}
        aria-haspopup="true"
        onClick={() => setOpen((v) => !v)}
      >
        <span className="hb-categories-burger" aria-hidden="true">
          <span />
        </span>
        All Categories
      </button>

      <div className="hb-mega" hidden={!open}>
        <ul className="hb-mega-list" role="menu">
          {tree.map((cat) => {
            const hasKids = cat.children.length > 0;
            const isActive = activeId === cat.id;
            const mobileOpen = Boolean(expandedMobile[cat.id]);
            const columns = megaColumnsFor(cat);
            return (
              <li
                key={cat.id}
                className={`hb-mega-item ${hasKids ? "has-children" : ""} ${isActive ? "is-active" : ""} ${mobileOpen ? "is-expanded" : ""}`}
                onMouseEnter={() => setActiveId(cat.id)}
              >
                <div className="hb-mega-row">
                  <Link
                    to={`/shop?category=${encodeURIComponent(cat.name)}`}
                    role="menuitem"
                    className={hasKids ? "has-dropdown" : undefined}
                    onClick={() => setOpen(false)}
                  >
                    <CategoryGlyph name={cat.name} />
                    <span className="hb-mega-title">{cat.name}</span>
                  </Link>
                  {hasKids ? (
                    <button
                      type="button"
                      className="hb-mega-expand"
                      aria-label={`Show ${cat.name} subcategories`}
                      aria-expanded={mobileOpen}
                      onClick={(e) => {
                        e.preventDefault();
                        e.stopPropagation();
                        setExpandedMobile((prev) => ({ ...prev, [cat.id]: !prev[cat.id] }));
                        setActiveId(cat.id);
                      }}
                    >
                      {mobileOpen ? "−" : "+"}
                    </button>
                  ) : null}
                </div>

                <div className={`hb-mega-drop ${isActive ? "is-visible" : ""}`} aria-hidden={!isActive && !mobileOpen}>
                  {columns.map((col) => (
                    <div key={col.key} className="hb-mega-col">
                      {col.title ? <div className="hb-mega-col-title">{col.title}</div> : null}
                      {col.links.length > 0 ? (
                        <ul>
                          {col.links.map((link) => (
                            <li key={link.id}>
                              <Link
                                to={`/shop?category=${encodeURIComponent(link.name)}`}
                                onClick={() => setOpen(false)}
                              >
                                {link.name}
                              </Link>
                              {link.children.length > 0 ? (
                                <ul className="hb-mega-grand">
                                  {link.children.map((g) => (
                                    <li key={g.id}>
                                      <Link
                                        to={`/shop?category=${encodeURIComponent(g.name)}`}
                                        onClick={() => setOpen(false)}
                                      >
                                        {g.name}
                                      </Link>
                                    </li>
                                  ))}
                                </ul>
                              ) : null}
                            </li>
                          ))}
                        </ul>
                      ) : null}
                      {col.shopAllName ? (
                        <div className="hb-mega-shopall">
                          <Link
                            to={`/shop?category=${encodeURIComponent(col.shopAllName)}`}
                            onClick={() => setOpen(false)}
                          >
                            Shop All
                          </Link>
                        </div>
                      ) : null}
                    </div>
                  ))}
                </div>
              </li>
            );
          })}
        </ul>
      </div>
    </div>
  );
}

function SearchCategoryPicker({
  value,
  onChange,
  tree,
}: {
  value: string;
  onChange: (name: string) => void;
  tree: CategoryNode[];
}) {
  const [open, setOpen] = useState(false);
  const wrapRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    if (!open) return;
    function onDoc(e: MouseEvent) {
      if (!wrapRef.current?.contains(e.target as Node)) setOpen(false);
    }
    function onKey(e: KeyboardEvent) {
      if (e.key === "Escape") setOpen(false);
    }
    document.addEventListener("mousedown", onDoc);
    document.addEventListener("keydown", onKey);
    return () => {
      document.removeEventListener("mousedown", onDoc);
      document.removeEventListener("keydown", onKey);
    };
  }, [open]);

  const label = value.trim() || "All Category";

  return (
    <div className={`hm-cat-wrap ${open ? "is-open" : ""}`} ref={wrapRef}>
      <button
        type="button"
        className="hm-cat-btn"
        aria-expanded={open}
        aria-haspopup="listbox"
        onClick={() => setOpen((v) => !v)}
      >
        <span className="hm-cat-label">{label}</span>
        <span className="hm-cat-caret" aria-hidden="true">
          ▾
        </span>
      </button>
      <div className="hm-cat-panel" hidden={!open} role="listbox" aria-label="Search category">
        <button
          type="button"
          className={`hm-cat-all ${!value ? "is-selected" : ""}`}
          role="option"
          aria-selected={!value}
          onClick={() => {
            onChange("");
            setOpen(false);
          }}
        >
          All Category
        </button>
        {tree.map((parent) => (
          <div key={parent.id} className="hm-cat-group">
            <button
              type="button"
              className={`hm-cat-title ${value === parent.name ? "is-selected" : ""}`}
              role="option"
              aria-selected={value === parent.name}
              onClick={() => {
                onChange(parent.name);
                setOpen(false);
              }}
            >
              {parent.name}
            </button>
            {parent.children.length > 0 ? (
              <ul>
                {parent.children.map((child) => (
                  <li key={child.id}>
                    <button
                      type="button"
                      className={value === child.name ? "is-selected" : ""}
                      role="option"
                      aria-selected={value === child.name}
                      onClick={() => {
                        onChange(child.name);
                        setOpen(false);
                      }}
                    >
                      {child.name}
                    </button>
                    {child.children.length > 0 ? (
                      <ul className="hm-cat-grand">
                        {child.children.map((g) => (
                          <li key={g.id}>
                            <button
                              type="button"
                              className={value === g.name ? "is-selected" : ""}
                              role="option"
                              aria-selected={value === g.name}
                              onClick={() => {
                                onChange(g.name);
                                setOpen(false);
                              }}
                            >
                              {g.name}
                            </button>
                          </li>
                        ))}
                      </ul>
                    ) : null}
                  </li>
                ))}
              </ul>
            ) : null}
          </div>
        ))}
      </div>
    </div>
  );
}

export function Header() {
  const { cartCount, session, pickup, branches, changeBranch, total, formatPrice, content, lines, setQty, catalog } =
    useStorefront();
  const navigate = useNavigate();
  const { settings: s } = useShopBrand();
  const [query, setQuery] = useState("");
  const [category, setCategory] = useState("");
  const [savedIds, setSavedIds] = useState<string[]>(() => loadWishlist());
  const phone = s?.topbar_phone_label || s?.contact_phone;
  const categoryTree = useMemo(() => buildCategoryTree(content?.categories ?? []), [content?.categories]);
  const savedItems = useMemo(
    () => catalog.filter((c) => savedIds.includes(c.variant_id)).slice(0, 5),
    [catalog, savedIds],
  );

  useEffect(() => onWishlistChanged(() => setSavedIds(loadWishlist())), []);

  function submitSearch(e: FormEvent) {
    e.preventDefault();
    const params = new URLSearchParams();
    if (query.trim()) params.set("q", query.trim());
    if (category) params.set("category", category);
    navigate(`/shop?${params.toString()}`);
  }

  return (
    <header>
      <div className="header-top">
        <div className="header-top-inner">
          <ul className="phone-wrap">
            <li>
              {phone ? (
                <>
                  <span>Call:</span>
                  <a href={`tel:${phone.replace(/\s+/g, "")}`}>{phone}</a>
                </>
              ) : (
                <Link to="/contact">Contact</Link>
              )}
            </li>
          </ul>
          <ul className="ht-menu">
            <li>
              <Link to="/account">{session ? "My Account" : "Sign In"}</Link>
            </li>
            <li>
              <Link to="/checkout">Checkout</Link>
            </li>
            <li>
              <Link to="/lookup">Track</Link>
            </li>
            <li>
              <Link to="/contact">Contact</Link>
            </li>
            {branches.length > 0 ? (
              <li>
                <select
                  value={pickup?.branch_id ?? ""}
                  onChange={(e) => {
                    const b = branches.find((x) => x.id === e.target.value);
                    if (b) void changeBranch(b);
                  }}
                  aria-label="Pickup branch"
                >
                  {branches.map((b) => (
                    <option key={b.id} value={b.id}>
                      {b.name}
                    </option>
                  ))}
                </select>
              </li>
            ) : null}
          </ul>
        </div>
      </div>

      {s?.header_promo_text ? <div className="header-promo">{s.header_promo_text}</div> : null}

      <div className="header-middle">
        <div className="header-middle-inner">
          <BrandMark />
          <form className="hm-searchbox" onSubmit={submitSearch}>
            <SearchCategoryPicker value={category} onChange={setCategory} tree={categoryTree} />
            <input
              value={query}
              onChange={(e) => setQuery(e.target.value)}
              placeholder="Search what you looking for?"
              inputMode="search"
              aria-label="Search products"
            />
            <button type="submit" className="li-btn" aria-label="Search">
              <svg width="18" height="18" viewBox="0 0 24 24" fill="none" aria-hidden="true">
                <circle cx="11" cy="11" r="7" stroke="currentColor" strokeWidth="2" />
                <path d="m20 20-3.5-3.5" stroke="currentColor" strokeWidth="2" strokeLinecap="round" />
              </svg>
            </button>
          </form>
          <div className="header-middle-right">
            <div className="hm-icon-wrap hm-wishlist">
              <Link to="/saved" title="Wishlist" className="hm-icon-btn">
                <svg width="18" height="18" viewBox="0 0 24 24" fill="none" aria-hidden="true">
                  <path
                    d="M12 21s-7-4.6-9.5-9A5.4 5.4 0 0 1 12 5.2 5.4 5.4 0 0 1 21.5 12C19 16.4 12 21 12 21Z"
                    stroke="currentColor"
                    strokeWidth="1.8"
                    strokeLinejoin="round"
                  />
                </svg>
                {savedIds.length ? <sup>{savedIds.length}</sup> : null}
              </Link>
              <div className="hm-dropdown hm-wishlist-drop">
                {savedItems.length === 0 ? (
                  <p className="hm-drop-empty">No saved items yet.</p>
                ) : (
                  <ul>
                    {savedItems.map((item) => (
                      <li key={item.variant_id}>
                        <Link to={`/product/${item.variant_id}`} className="hm-drop-product">
                          <span className="img-product">
                            {catalogItemImageURL(item) ? (
                              <img src={catalogItemImageURL(item)} alt="" />
                            ) : (
                              <span className="thumb-empty" />
                            )}
                          </span>
                          <span className="info-product">
                            <span className="name">{item.product_name}</span>
                            <span className="price">{formatPrice(item.sell_price)}</span>
                          </span>
                        </Link>
                        <button
                          type="button"
                          className="delete"
                          aria-label={`Remove ${item.product_name}`}
                          onClick={() => setSavedIds(toggleWishlist(item.variant_id))}
                        >
                          ×
                        </button>
                      </li>
                    ))}
                  </ul>
                )}
                <div className="btn-cart">
                  <Link to="/saved" className="view-cart">
                    View Wishlist
                  </Link>
                  <Link to="/shop" className="check-out">
                    Continue Shopping
                  </Link>
                </div>
              </div>
            </div>

            <div className="hm-icon-wrap hm-account">
              <Link to="/account" title="Account" className="hm-icon-btn">
                <svg width="18" height="18" viewBox="0 0 24 24" fill="none" aria-hidden="true">
                  <circle cx="12" cy="8" r="3.5" stroke="currentColor" strokeWidth="1.8" />
                  <path d="M5 19.5a7 7 0 0 1 14 0" stroke="currentColor" strokeWidth="1.8" strokeLinecap="round" />
                </svg>
              </Link>
            </div>

            <div className="hm-icon-wrap hm-minicart">
              <Link className="hm-minicart-trigger" to="/cart">
                <span className="hm-cart-icon" aria-hidden="true">
                  <svg width="20" height="20" viewBox="0 0 24 24" fill="none">
                    <path
                      d="M6 7h15l-1.5 9H8L6 7Z"
                      stroke="currentColor"
                      strokeWidth="1.8"
                      strokeLinejoin="round"
                    />
                    <path d="M6 7 5 3H2" stroke="currentColor" strokeWidth="1.8" strokeLinecap="round" />
                    <circle cx="9" cy="20" r="1.3" fill="currentColor" />
                    <circle cx="18" cy="20" r="1.3" fill="currentColor" />
                  </svg>
                  {cartCount > 0 ? <span className="cart-item-count">{cartCount}</span> : null}
                </span>
                <span className="hm-cart-total">{formatPrice(total)}</span>
              </Link>
              <div className="hm-dropdown hm-cart-drop">
                {lines.length === 0 ? (
                  <p className="hm-drop-empty">Your cart is empty.</p>
                ) : (
                  <ul>
                    {lines.slice(0, 6).map(({ item, qty }) => (
                      <li key={item.variant_id}>
                        <Link to={`/product/${item.variant_id}`} className="hm-drop-product">
                          <span className="img-product">
                            {catalogItemImageURL(item) ? (
                              <img src={catalogItemImageURL(item)} alt="" />
                            ) : (
                              <span className="thumb-empty" />
                            )}
                          </span>
                          <span className="info-product">
                            <span className="name">{item.product_name}</span>
                            <span className="price">
                              <span>{qty} ×</span> {formatPrice(item.sell_price)}
                            </span>
                          </span>
                        </Link>
                        <button
                          type="button"
                          className="delete"
                          aria-label={`Remove ${item.product_name}`}
                          onClick={() => setQty(item.variant_id, 0, item.available_qty)}
                        >
                          ×
                        </button>
                      </li>
                    ))}
                  </ul>
                )}
                <div className="total">
                  <span>Subtotal:</span>
                  <span className="price">{formatPrice(total)}</span>
                </div>
                <div className="btn-cart">
                  <Link to="/cart" className="view-cart">
                    View Cart
                  </Link>
                  <Link to="/checkout" className="check-out">
                    Checkout
                  </Link>
                </div>
              </div>
            </div>
          </div>
        </div>
      </div>

      <div className="header-bottom">
        <div className="header-bottom-inner">
          <CategoryMenu />
          <nav className="hb-menu">
            <NavLink to="/" end>
              Home
            </NavLink>
            <NavLink to="/shop">Shop</NavLink>
            <NavLink to="/stores">Stores</NavLink>
            <NavLink to="/contact">Contact</NavLink>
            <NavLink to="/lookup">Track</NavLink>
            <a className="hb-deals" href="/#deals">
              Today Deals
            </a>
          </nav>
        </div>
      </div>
    </header>
  );
}

function Newsletter() {
  const { content } = useStorefront();
  const [email, setEmail] = useState("");
  const [status, setStatus] = useState<"idle" | "busy" | "done" | "error">("idle");
  const s = content?.settings;

  async function submit(e: FormEvent) {
    e.preventDefault();
    if (!email.trim()) return;
    setStatus("busy");
    try {
      await subscribeNewsletter(email.trim());
      setStatus("done");
      setEmail("");
    } catch {
      setStatus("error");
    }
  }

  return (
    <div className="footer-newsletter">
      <h3>{s?.newsletter_headline || "Sign up for newsletter"}</h3>
      <p>{s?.newsletter_subtext || "Get updates on new arrivals and deals."}</p>
      <form onSubmit={(e) => void submit(e)}>
        <input
          type="email"
          required
          placeholder="Your email address"
          value={email}
          onChange={(e) => setEmail(e.target.value)}
          disabled={status === "busy" || status === "done"}
        />
        <button type="submit" disabled={status === "busy" || status === "done"}>
          {status === "done" ? "Done" : "Subscribe"}
        </button>
      </form>
      {status === "error" ? <p className="error">Could not subscribe — try again.</p> : null}
    </div>
  );
}

function ShippingStrip() {
  const { content } = useStorefront();
  const s = content?.settings;
  const badges = [
    { icon: "/limupa/shipping/1.png", title: s?.trust_badge_1_title, sub: s?.trust_badge_1_subtext },
    { icon: "/limupa/shipping/2.png", title: s?.trust_badge_2_title, sub: s?.trust_badge_2_subtext },
    { icon: "/limupa/shipping/3.png", title: s?.trust_badge_3_title, sub: s?.trust_badge_3_subtext },
    { icon: "/limupa/shipping/4.png", title: s?.trust_badge_4_title, sub: s?.trust_badge_4_subtext },
  ].filter((b) => b.title);
  if (badges.length === 0) return null;
  return (
    <div className="li-container">
      <div className="footer-shipping">
        {badges.map((b, i) => (
          <div key={i} className="li-shipping-inner-box">
            <div className="shipping-icon">
              <img src={b.icon} alt="" />
            </div>
            <div className="shipping-text">
              <h2>{b.title}</h2>
              {b.sub ? <p>{b.sub}</p> : null}
            </div>
          </div>
        ))}
      </div>
    </div>
  );
}

function Footer() {
  const { content } = useStorefront();
  const { name } = useShopBrand();
  const s = content?.settings;
  const categories = (content?.categories ?? []).filter((c) => c.depth === 0).slice(0, 6);
  const year = new Date().getFullYear();

  return (
    <footer className="limupa-footer">
      <ShippingStrip />
      <div className="li-container footer-middle">
        <div>
          <Link className="footer-logo" to="/" aria-label={name}>
            <img src="/brand/techlane-logo.jpg" alt={name} />
          </Link>
          {s?.footer_tagline ? <p>{s.footer_tagline}</p> : null}
          {s?.contact_phone ? <p>{s.contact_phone}</p> : null}
          {s?.contact_email ? <p>{s.contact_email}</p> : null}
          {s?.business_hours ? <p>{s.business_hours}</p> : null}
        </div>
        {categories.length > 0 ? (
          <div>
            <h3>Product</h3>
            {categories.map((c) => (
              <Link key={c.id} to={`/shop?category=${encodeURIComponent(c.name)}`}>
                {c.name}
              </Link>
            ))}
          </div>
        ) : null}
        <div>
          <h3>Our company</h3>
          <Link to="/stores">Store locator</Link>
          <Link to="/contact">Contact us</Link>
          <Link to="/lookup">Track order / repair</Link>
          <Link to="/account">Your account</Link>
          <Link to="/cart">Shopping cart</Link>
        </div>
        <Newsletter />
      </div>
      <div className="footer-bottom">
        <div className="li-container">
          © {year} {name}. All rights reserved.
        </div>
      </div>
    </footer>
  );
}

export function Layout() {
  useThemeFromCMS();
  return (
    <div className="app limupa electro">
      <Header />
      <main className="site-main">
        <Outlet />
      </main>
      <Footer />
    </div>
  );
}

export function NotFoundPage() {
  const { name } = useShopBrand();
  return (
    <section className="section not-found li-container" style={{ padding: "3rem 1rem" }}>
      <h1>Page not found</h1>
      <p className="muted">That page doesn&apos;t exist on {name}.</p>
      <div className="stack" style={{ justifyContent: "flex-start" }}>
        <Link className="btn" to="/">
          Back to home
        </Link>
        <Link className="btn btn-ghost" to="/shop">
          Browse shop
        </Link>
      </div>
    </section>
  );
}
