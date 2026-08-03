import { useEffect, useMemo, useState } from "react";
import { Link } from "react-router-dom";
import { useStorefront } from "../store/StorefrontContext";
import {
  storefrontBannerImageURL,
  catalogItemImageURL,
  type CatalogItem,
  type StorefrontBanner,
  type StorefrontCategory,
} from "../lib/api";
import { ProductCard } from "../components/ProductCard";
import { Countdown } from "../components/Countdown";

function isSidePlacement(p: string) {
  return p === "side" || p === "promo_tile";
}

function isMidPlacement(p: string) {
  return p === "mid" || p === "mid_banner" || p === "static_banner";
}

type HeroSlide = {
  key: string;
  headline?: string;
  subtext?: string;
  ctaLabel?: string;
  ctaHref: string;
  imageUrl?: string;
  dealPrice?: number;
  dealBase?: number;
};

function buildHeroSlides(
  banners: StorefrontBanner[],
  settings: {
    hero_headline?: string;
    hero_subtext?: string;
    hero_cta_label?: string;
    hero_cta_href?: string;
  } | undefined,
): HeroSlide[] {
  if (banners.length > 0) {
    return banners.map((b) => {
      const hasDeal = b.deal_price != null && b.deal_base_price != null;
      return {
        key: b.id,
        headline: b.headline || undefined,
        subtext: b.subtext || undefined,
        ctaLabel: hasDeal ? "Shop this deal" : b.cta_label || undefined,
        ctaHref:
          hasDeal && b.deal_variant_id
            ? `/product/${b.deal_variant_id}`
            : b.cta_href || "/shop",
        imageUrl: b.has_image ? storefrontBannerImageURL(b.id, b.image_updated_at) : undefined,
        dealPrice: hasDeal ? b.deal_price! : undefined,
        dealBase: hasDeal ? b.deal_base_price! : undefined,
      };
    });
  }

  // Single fallback from owner Settings → Storefront hero fields only.
  if (settings?.hero_headline || settings?.hero_subtext || settings?.hero_cta_label) {
    return [
      {
        key: "settings-hero",
        headline: settings.hero_headline || undefined,
        subtext: settings.hero_subtext || undefined,
        ctaLabel: settings.hero_cta_label || undefined,
        ctaHref: settings.hero_cta_href || "/shop",
      },
    ];
  }

  return [];
}

function HeroSlider() {
  const { content, formatPrice } = useStorefront();
  const banners = (content?.banners ?? []).filter((b) => b.placement === "hero");
  const promoTiles = (content?.banners ?? []).filter((b) => isSidePlacement(b.placement)).slice(0, 2);
  const slides = useMemo(() => buildHeroSlides(banners, content?.settings), [banners, content?.settings]);
  const [index, setIndex] = useState(0);

  useEffect(() => {
    setIndex(0);
  }, [slides.length]);

  useEffect(() => {
    if (slides.length <= 1) return;
    const t = window.setInterval(() => setIndex((i) => (i + 1) % slides.length), 6000);
    return () => window.clearInterval(t);
  }, [slides.length]);

  const slide = slides[index];
  if (!slide && promoTiles.length === 0) return null;

  return (
    <div className="slider-with-banner">
      <div className={`li-container${promoTiles.length === 0 ? " slider-only" : ""}`}>
        {slide ? (
          <div className="slider-area">
            <div
              className={`single-slide align-center-left${slide.imageUrl ? " has-cms-image" : " no-image"}`}
              style={
                slide.imageUrl
                  ? {
                      backgroundImage: `linear-gradient(90deg, rgba(255,255,255,0.88), rgba(255,255,255,0.35)), url(${slide.imageUrl})`,
                    }
                  : undefined
              }
            >
              <div className="slider-content">
                {slide.headline ? <h2>{slide.headline}</h2> : null}
                {slide.dealPrice != null ? (
                  <h3>
                    <span>{formatPrice(slide.dealPrice)}</span>
                    {slide.dealBase != null ? (
                      <del className="price-was" style={{ marginLeft: "0.5rem", fontSize: "0.9em" }}>
                        {formatPrice(slide.dealBase)}
                      </del>
                    ) : null}
                  </h3>
                ) : slide.subtext ? (
                  <h3>{slide.subtext}</h3>
                ) : null}
                {slide.ctaLabel ? (
                  <div className="default-btn slide-btn">
                    <Link className="links" to={slide.ctaHref}>
                      {slide.ctaLabel}
                    </Link>
                  </div>
                ) : null}
                {slides.length > 1 ? (
                  <div className="slider-dots">
                    {slides.map((s, i) => (
                      <button
                        key={s.key}
                        type="button"
                        className={i === index ? "active" : ""}
                        aria-label={`Slide ${i + 1}`}
                        onClick={() => setIndex(i)}
                      />
                    ))}
                  </div>
                ) : null}
              </div>
              {slides.length > 1 ? (
                <>
                  <button
                    type="button"
                    className="slider-nav slider-nav-prev"
                    aria-label="Previous slide"
                    onClick={() => setIndex((i) => (i - 1 + slides.length) % slides.length)}
                  >
                    <svg viewBox="0 0 24 24" aria-hidden="true">
                      <path d="M15.4 4.6 8 12l7.4 7.4 1.4-1.4L10.8 12l6-6z" fill="currentColor" />
                    </svg>
                  </button>
                  <button
                    type="button"
                    className="slider-nav slider-nav-next"
                    aria-label="Next slide"
                    onClick={() => setIndex((i) => (i + 1) % slides.length)}
                  >
                    <svg viewBox="0 0 24 24" aria-hidden="true">
                      <path d="M8.6 4.6 15 12l-6.4 7.4-1.4-1.4 5-6-5-6z" fill="currentColor" />
                    </svg>
                  </button>
                </>
              ) : null}
            </div>
          </div>
        ) : null}

        {promoTiles.length > 0 ? (
          <div className="li-side-banners">
            {promoTiles.slice(0, 2).map((t) => (
              <div key={t.id} className="li-banner">
                <Link to={t.deal_variant_id ? `/product/${t.deal_variant_id}` : t.cta_href || "/shop"}>
                  {t.has_image ? (
                    <img src={storefrontBannerImageURL(t.id, t.image_updated_at)} alt={t.headline || ""} />
                  ) : (
                    <div className="li-banner promo-copy">
                      {t.headline ? <strong>{t.headline}</strong> : null}
                      {t.subtext ? <span className="muted">{t.subtext}</span> : null}
                      {t.cta_label ? <span className="tiny">{t.cta_label}</span> : null}
                    </div>
                  )}
                </Link>
              </div>
            ))}
          </div>
        ) : null}
      </div>
    </div>
  );
}

type Tab = "new" | "best" | "featured";

const RAIL_LIMIT = 5;

function ProductTabs() {
  const { content } = useStorefront();
  const tabs = useMemo(() => {
    if (!content) return [] as Array<{ id: Tab; label: string; items: CatalogItem[] }>;
    const out: Array<{ id: Tab; label: string; items: CatalogItem[] }> = [];
    if (content.new_arrivals.length) out.push({ id: "new", label: "New Arrival", items: content.new_arrivals });
    if (content.bestsellers.length) out.push({ id: "best", label: "Bestseller", items: content.bestsellers });
    if (content.featured.length) out.push({ id: "featured", label: "Featured Products", items: content.featured });
    return out;
  }, [content]);

  const [tab, setTab] = useState<Tab>("new");

  useEffect(() => {
    if (tabs.length && !tabs.some((t) => t.id === tab)) setTab(tabs[0]!.id);
  }, [tabs, tab]);

  if (tabs.length === 0) return null;
  const active = tabs.find((t) => t.id === tab) ?? tabs[0]!;

  return (
    <div className="product-area">
      <div className="li-container">
        <ul className="li-product-menu">
          {tabs.map((t) => (
            <li key={t.id}>
              <button type="button" className={active.id === t.id ? "active" : ""} onClick={() => setTab(t.id)}>
                {t.label}
              </button>
            </li>
          ))}
        </ul>
        <div className="catalog limupa-catalog">
          {active.items.slice(0, RAIL_LIMIT).map((item) => (
            <ProductCard key={item.variant_id} item={item} />
          ))}
        </div>
      </div>
    </div>
  );
}

function PromoBannerRow() {
  const { content } = useStorefront();
  const tiles = (content?.banners ?? []).filter((b) => isMidPlacement(b.placement)).slice(0, 3);
  if (tiles.length === 0) return null;

  return (
    <div className="li-static-banner">
      <div className="li-container">
        {tiles.map((t) => (
          <div key={t.id} className="single-banner">
            <Link to={t.deal_variant_id ? `/product/${t.deal_variant_id}` : t.cta_href || "/shop"}>
              {t.has_image ? (
                <img
                  className="banner-media"
                  src={storefrontBannerImageURL(t.id, t.image_updated_at)}
                  alt={t.headline || ""}
                />
              ) : (
                <div className="li-banner promo-copy">
                  {t.headline ? <strong>{t.headline}</strong> : null}
                  {t.subtext ? <span className="muted">{t.subtext}</span> : null}
                </div>
              )}
            </Link>
          </div>
        ))}
      </div>
    </div>
  );
}

/** Compact Electro-style lists: one product per list, max 5, no cross-column repeats. */
function CompactBestsellers() {
  const { content, formatPrice } = useStorefront();
  const items = (content?.bestsellers ?? []).slice(0, RAIL_LIMIT);
  if (items.length === 0) return null;

  return (
    <section className="product-area home-bestsellers-compact">
      <div className="li-container">
        <div className="featured-product home-featured-list">
          <div className="home-rail-head">
            <h3>Bestsellers</h3>
            <Link to="/shop" className="tiny">
              View all
            </Link>
          </div>
          {items.map((item) => {
            const imageSrc = catalogItemImageURL(item);
            return (
              <Link key={item.variant_id} to={`/product/${item.variant_id}`} className="featured-product-item">
                {imageSrc ? <img src={imageSrc} alt="" /> : <div className="thumb-empty" />}
                <div>
                  <strong>{item.product_name}</strong>
                  <span className="price-now">{formatPrice(item.sell_price)}</span>
                </div>
              </Link>
            );
          })}
        </div>
      </div>
    </section>
  );
}

/** Map a category (and its aliases) up to the top-level parent name. */
function rootCategoryName(c: StorefrontCategory, byId: Map<string, StorefrontCategory>): string {
  let cur = c;
  const guard = new Set<string>();
  while (cur.parent_id && byId.has(cur.parent_id) && !guard.has(cur.id)) {
    guard.add(cur.id);
    cur = byId.get(cur.parent_id)!;
  }
  return cur.name.trim();
}

/** Products grouped by top-level category only — nested leaves roll up. */
function ProductsByCategory() {
  const { catalog, content } = useStorefront();

  const groups = useMemo(() => {
    const cats = content?.categories ?? [];
    const byId = new Map(cats.map((c) => [c.id, c]));

    // If the same display name exists as a nested category elsewhere, prefer that
    // tree's root (avoids duplicate rails like root "Phone Chargers" + nested twin).
    const nestedOwnerByName = new Map<string, string>();
    for (const c of cats) {
      const name = c.name?.trim();
      if (!name) continue;
      if (c.parent_id || c.depth > 0) {
        nestedOwnerByName.set(name, rootCategoryName(c, byId));
      }
    }

    // leaf display name → root display name (skip nested names as section titles)
    const leafToRoot = new Map<string, string>();
    const rootOrder: string[] = [];
    const rootSeen = new Set<string>();
    for (const c of cats) {
      const name = c.name?.trim();
      if (!name) continue;
      const nested = Boolean(c.parent_id) || c.depth > 0;
      let root = rootCategoryName(c, byId);
      const folded = nestedOwnerByName.get(name);
      if (!nested && folded && folded !== name) {
        root = folded;
        leafToRoot.set(name, root);
        continue;
      }
      leafToRoot.set(name, root);
      if (!nested && !rootSeen.has(root)) {
        rootOrder.push(root);
        rootSeen.add(root);
      }
    }

    const byRoot = new Map<string, CatalogItem[]>();
    for (const item of catalog) {
      const leaf = item.category?.trim();
      if (!leaf) continue;
      const root = leafToRoot.get(leaf);
      // Only show products that belong under a known top-level category.
      if (!root || !rootSeen.has(root)) continue;
      const list = byRoot.get(root) ?? [];
      if (list.length >= RAIL_LIMIT) continue;
      list.push(item);
      byRoot.set(root, list);
    }

    return rootOrder
      .filter((title) => (byRoot.get(title)?.length ?? 0) > 0)
      .map((title) => ({ title, items: byRoot.get(title)! }));
  }, [catalog, content?.categories]);

  if (groups.length === 0) return null;

  return (
    <>
      {groups.map((g) => (
        <section key={g.title} className="product-area" style={{ paddingTop: 0 }}>
          <div className="li-container">
            <div className="home-rail-head">
              <ul className="li-product-menu" style={{ marginBottom: 0, borderBottom: 0 }}>
                <li>
                  <button type="button" className="active">
                    {g.title}
                  </button>
                </li>
              </ul>
              <Link to={`/shop?category=${encodeURIComponent(g.title)}`} className="tiny">
                View all
              </Link>
            </div>
            <div className="catalog limupa-catalog">
              {g.items.map((item) => (
                <ProductCard key={item.variant_id} item={item} showCategory={false} />
              ))}
            </div>
          </div>
        </section>
      ))}
    </>
  );
}

function TodaysDeals() {
  const { content } = useStorefront();
  const deals = content?.deals ?? [];
  if (deals.length === 0) return null;

  return (
    <section className="product-area" id="deals">
      <div className="li-container">
        <ul className="li-product-menu">
          <li>
            <button type="button" className="active">
              Deals
            </button>
          </li>
        </ul>
        <div className="catalog limupa-catalog">
          {deals.slice(0, RAIL_LIMIT).map((item) => (
            <div key={item.variant_id}>
              <ProductCard item={item} />
              {item.deal_ends_at ? (
                <div className="deal-timer" style={{ textAlign: "center", marginTop: "0.35rem" }}>
                  Ends in <Countdown endsAt={item.deal_ends_at} />
                </div>
              ) : null}
            </div>
          ))}
        </div>
      </div>
    </section>
  );
}

function MostViewedRail() {
  const { content } = useStorefront();
  const items = content?.most_viewed ?? [];
  if (items.length === 0) return null;
  return (
    <section className="product-area" style={{ paddingTop: 0 }}>
      <div className="li-container">
        <ul className="li-product-menu">
          <li>
            <button type="button" className="active">
              Most viewed
            </button>
          </li>
        </ul>
        <div className="catalog limupa-catalog">
          {items.slice(0, RAIL_LIMIT).map((item) => (
            <ProductCard key={item.variant_id} item={item} />
          ))}
        </div>
      </div>
    </section>
  );
}

export function HomePage() {
  const { loadingShop, error, content, catalog } = useStorefront();
  const hasAnyProducts =
    catalog.length > 0 ||
    (content?.featured.length ?? 0) +
      (content?.new_arrivals.length ?? 0) +
      (content?.bestsellers.length ?? 0) +
      (content?.deals.length ?? 0) +
      (content?.most_viewed.length ?? 0) >
      0;

  return (
    <>
      <HeroSlider />
      {error ? <p className="error li-container">{error}</p> : null}
      {loadingShop ? (
        <div className="li-container catalog limupa-catalog" style={{ padding: "1rem 0" }}>
          <div className="skeleton" style={{ minHeight: "16rem" }} />
          <div className="skeleton" style={{ minHeight: "16rem" }} />
          <div className="skeleton" style={{ minHeight: "16rem" }} />
          <div className="skeleton" style={{ minHeight: "16rem" }} />
        </div>
      ) : (
        <>
          <ProductTabs />
          <PromoBannerRow />
          <TodaysDeals />
          <ProductsByCategory />
          <CompactBestsellers />
          <MostViewedRail />
          {hasAnyProducts ? (
            <div className="li-container" style={{ textAlign: "center", paddingBottom: "2rem" }}>
              <Link
                className="links"
                style={{ display: "inline-block", background: "var(--sf-primary)", padding: "0.75rem 1.5rem", fontWeight: 700 }}
                to="/shop"
              >
                {content?.settings.hero_cta_label || "Shop"}
              </Link>
            </div>
          ) : null}
        </>
      )}
    </>
  );
}
