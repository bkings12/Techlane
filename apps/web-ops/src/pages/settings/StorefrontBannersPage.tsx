import { useEffect, useMemo, useRef, useState } from "react";
import { Badge, Button, EmptyState, Input, PageHeader } from "../../components/ui";
import {
  createStorefrontBanner,
  deleteStorefrontBanner,
  deleteStorefrontBannerImage,
  listStorefrontBanners,
  listStorefrontDeals,
  storefrontBannerImageURL,
  updateStorefrontBanner,
  uploadStorefrontBannerImage,
  type StorefrontBanner,
  type StorefrontDeal,
} from "../../lib/api";

type Placement = "hero" | "side" | "mid";

function normalizePlacement(p: string): Placement {
  if (p === "side" || p === "promo_tile") return "side";
  if (p === "mid" || p === "mid_banner" || p === "static_banner") return "mid";
  return "hero";
}

const SECTIONS: Array<{
  placement: Placement;
  title: string;
  body: string;
  addLabel: string;
  defaultHeadline: string;
  maxHint?: string;
}> = [
  {
    placement: "hero",
    title: "Hero slider",
    body: "Large slides at the top of the homepage. Add as many as you like — they rotate automatically.",
    addLabel: "Add hero slide",
    defaultHeadline: "New hero slide",
  },
  {
    placement: "side",
    title: "Side cards (next to slider)",
    body: "Two stacked promo cards beside the hero — same layout as Electro’s right-hand banners.",
    addLabel: "Add side card",
    defaultHeadline: "New side card",
    maxHint: "Best with 2 cards.",
  },
  {
    placement: "mid",
    title: "Mid-page banners",
    body: "Wide banner row below product rails — typically 3 image cards across.",
    addLabel: "Add mid-page banner",
    defaultHeadline: "New mid-page banner",
    maxHint: "Best with up to 3 banners.",
  },
];

export function StorefrontBannersPage() {
  const [banners, setBanners] = useState<StorefrontBanner[]>([]);
  const [deals, setDeals] = useState<StorefrontDeal[]>([]);
  const [error, setError] = useState("");
  const [busy, setBusy] = useState(false);
  const [imageBusyId, setImageBusyId] = useState<string | null>(null);
  const fileInputs = useRef<Record<string, HTMLInputElement | null>>({});

  function refresh() {
    return Promise.all([listStorefrontBanners(), listStorefrontDeals()])
      .then(([b, d]) => {
        setBanners(b.items);
        setDeals(d.items);
      })
      .catch((e) => setError(e instanceof Error ? e.message : "Failed to load banners"));
  }

  useEffect(() => {
    void refresh();
  }, []);

  const grouped = useMemo(() => {
    const by: Record<Placement, StorefrontBanner[]> = { hero: [], side: [], mid: [] };
    for (const b of banners) {
      by[normalizePlacement(b.placement)].push(b);
    }
    for (const key of Object.keys(by) as Placement[]) {
      by[key].sort((a, c) => a.sort_order - c.sort_order);
    }
    return by;
  }, [banners]);

  async function addBanner(placement: Placement, headline: string) {
    setBusy(true);
    setError("");
    try {
      await createStorefrontBanner({
        headline,
        placement,
        sort_order: grouped[placement].length,
        cta_href: "/shop",
        cta_label: "Shop now",
      });
      await refresh();
    } catch (e) {
      setError(e instanceof Error ? e.message : "Could not create banner");
    } finally {
      setBusy(false);
    }
  }

  function patch(id: string, field: keyof StorefrontBanner, value: string | number | boolean | undefined) {
    setBanners((prev) => prev.map((b) => (b.id === id ? { ...b, [field]: value } : b)));
  }

  async function save(banner: StorefrontBanner) {
    setBusy(true);
    setError("");
    try {
      const next = await updateStorefrontBanner(banner.id, {
        headline: banner.headline,
        subtext: banner.subtext,
        cta_label: banner.cta_label,
        cta_href: banner.cta_href,
        placement: normalizePlacement(banner.placement),
        deal_id: banner.deal_id || undefined,
        clear_deal_id: !banner.deal_id,
        sort_order: banner.sort_order,
        active: banner.active,
      });
      setBanners((prev) => prev.map((b) => (b.id === next.id ? next : b)));
    } catch (e) {
      setError(e instanceof Error ? e.message : "Save failed");
    } finally {
      setBusy(false);
    }
  }

  async function remove(id: string) {
    setBusy(true);
    setError("");
    try {
      await deleteStorefrontBanner(id);
      await refresh();
    } catch (e) {
      setError(e instanceof Error ? e.message : "Delete failed");
    } finally {
      setBusy(false);
    }
  }

  async function onImagePicked(id: string, file: File | undefined) {
    if (!file) return;
    setImageBusyId(id);
    setError("");
    try {
      const next = await uploadStorefrontBannerImage(id, file);
      setBanners((prev) => prev.map((b) => (b.id === next.id ? next : b)));
    } catch (e) {
      setError(e instanceof Error ? e.message : "Image upload failed");
    } finally {
      setImageBusyId(null);
      const input = fileInputs.current[id];
      if (input) input.value = "";
    }
  }

  async function removeImage(id: string) {
    setImageBusyId(id);
    setError("");
    try {
      const next = await deleteStorefrontBannerImage(id);
      setBanners((prev) => prev.map((b) => (b.id === next.id ? next : b)));
    } catch (e) {
      setError(e instanceof Error ? e.message : "Could not remove image");
    } finally {
      setImageBusyId(null);
    }
  }

  function renderBannerCard(b: StorefrontBanner) {
    const placement = normalizePlacement(b.placement);
    return (
      <li key={b.id} className="part-card">
        <div className="part-head">
          <strong>{b.headline || "Untitled banner"}</strong>
          <Badge tone={b.active ? "success" : "pending"}>{b.active ? "active" : "hidden"}</Badge>
        </div>

        <div className="logo-row">
          <div className="logo-frame">
            {b.has_image ? (
              <img
                src={storefrontBannerImageURL(b.id)}
                alt={b.headline || "Banner"}
                style={{ maxWidth: "100%", maxHeight: 96, objectFit: "contain" }}
              />
            ) : (
              <span className="logo-empty">No image</span>
            )}
          </div>
          <div className="logo-actions">
            <input
              ref={(el) => {
                fileInputs.current[b.id] = el;
              }}
              type="file"
              accept="image/png,image/jpeg,image/webp"
              style={{ display: "none" }}
              onChange={(e) => void onImagePicked(b.id, e.target.files?.[0])}
            />
            <Button
              type="button"
              variant="secondary"
              disabled={imageBusyId === b.id}
              onClick={() => fileInputs.current[b.id]?.click()}
            >
              {imageBusyId === b.id ? "Working…" : b.has_image ? "Replace image" : "Upload image"}
            </Button>
            {b.has_image ? (
              <Button type="button" variant="secondary" disabled={imageBusyId === b.id} onClick={() => void removeImage(b.id)}>
                Remove
              </Button>
            ) : null}
            <p className="hint">PNG, JPEG or WebP up to 2&nbsp;MB.</p>
          </div>
        </div>

        <div className="form-grid">
          <label>
            Placement
            <select className="input" value={placement} onChange={(e) => patch(b.id, "placement", e.target.value)}>
              <option value="hero">Hero slider</option>
              <option value="side">Side card (next to slider)</option>
              <option value="mid">Mid-page banner</option>
            </select>
          </label>
          <label>
            Headline
            <Input value={b.headline} onChange={(e) => patch(b.id, "headline", e.target.value)} />
          </label>
          <label>
            Subtext
            <Input value={b.subtext} onChange={(e) => patch(b.id, "subtext", e.target.value)} />
          </label>
          <div className="field-pair">
            <label>
              Button label
              <Input value={b.cta_label} onChange={(e) => patch(b.id, "cta_label", e.target.value)} />
            </label>
            <label>
              Button link
              <Input value={b.cta_href} placeholder="/shop" onChange={(e) => patch(b.id, "cta_href", e.target.value)} />
            </label>
          </div>
          <label>
            Link to a real deal (shows its actual was/now price)
            <select
              className="input"
              value={b.deal_id ?? ""}
              onChange={(e) => patch(b.id, "deal_id", e.target.value || undefined)}
            >
              <option value="">— No linked deal —</option>
              {deals.map((d) => (
                <option key={d.id} value={d.id}>
                  {d.product_name} — KES {d.deal_price.toLocaleString()} (was {(d.base_price ?? 0).toLocaleString()})
                </option>
              ))}
            </select>
          </label>
          <div className="field-pair">
            <label>
              Sort order
              <Input
                type="number"
                value={b.sort_order}
                onChange={(e) => patch(b.id, "sort_order", Number(e.target.value) || 0)}
              />
            </label>
            <label className="checkbox-row">
              <input type="checkbox" checked={b.active} onChange={(e) => patch(b.id, "active", e.target.checked)} />
              Show on storefront
            </label>
          </div>
        </div>

        <div className="btn-row">
          <Button type="button" disabled={busy} onClick={() => void save(b)}>
            Save
          </Button>
          <Button type="button" variant="ghost" disabled={busy} onClick={() => void remove(b.id)}>
            Delete
          </Button>
        </div>
      </li>
    );
  }

  return (
    <div className="settings-page">
      <PageHeader
        title="Storefront banners"
        subtitle="Electro-style homepage: hero slider, side cards beside it, and mid-page banner row"
      />
      {error ? <p className="form-error">{error}</p> : null}

      {SECTIONS.map((section) => {
        const items = grouped[section.placement];
        return (
          <section key={section.placement} className="settings-form-card" style={{ marginBottom: "1.25rem" }}>
            <div className="part-head" style={{ marginBottom: "0.75rem" }}>
              <div>
                <h2 style={{ margin: 0 }}>{section.title}</h2>
                <p className="hint" style={{ margin: "0.35rem 0 0" }}>
                  {section.body}
                  {section.maxHint ? ` ${section.maxHint}` : ""}
                </p>
              </div>
              <Button type="button" disabled={busy} onClick={() => void addBanner(section.placement, section.defaultHeadline)}>
                {section.addLabel}
              </Button>
            </div>
            {items.length === 0 ? (
              <EmptyState title={`No ${section.title.toLowerCase()} yet`} body={`Use “${section.addLabel}” to create one.`} />
            ) : (
              <ul className="inv-product-grid">{items.map(renderBannerCard)}</ul>
            )}
          </section>
        );
      })}
    </div>
  );
}
