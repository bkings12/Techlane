import { useEffect, useMemo, useState } from "react";
import { Badge, Button, Input, PageHeader } from "../../components/ui";
import {
  deleteStorefrontLogo,
  getStorefrontSettings,
  storefrontLogoURL,
  updateStorefrontSettings,
  uploadStorefrontLogo,
  type StorefrontSettings,
} from "../../lib/api";

const EXTRA_CURRENCIES = ["USD", "EUR", "GBP"];

function parseCurrencies(raw: string): string[] {
  return raw
    .split(",")
    .map((c) => c.trim().toUpperCase())
    .filter(Boolean);
}

export function StorefrontSettingsPage() {
  const [settings, setSettings] = useState<StorefrontSettings | null>(null);
  const [draft, setDraft] = useState<StorefrontSettings | null>(null);
  const [error, setError] = useState("");
  const [saved, setSaved] = useState("");
  const [busy, setBusy] = useState(false);
  const [logoBusy, setLogoBusy] = useState(false);

  useEffect(() => {
    getStorefrontSettings()
      .then((s) => {
        setSettings(s);
        setDraft(s);
      })
      .catch((e) => setError(e instanceof Error ? e.message : "Failed to load storefront settings"));
  }, []);

  const dirty = useMemo(
    () => Boolean(settings && draft) && JSON.stringify(settings) !== JSON.stringify(draft),
    [settings, draft],
  );

  function set<K extends keyof StorefrontSettings>(key: K, value: StorefrontSettings[K]) {
    setDraft((prev) => (prev ? { ...prev, [key]: value } : prev));
    setSaved("");
  }

  async function save() {
    if (!draft) return;
    setBusy(true);
    setError("");
    setSaved("");
    try {
      const next = await updateStorefrontSettings({
        shop_display_name: draft.shop_display_name,
        page_title: draft.page_title,
        color_primary: draft.color_primary,
        color_secondary: draft.color_secondary,
        color_accent: draft.color_accent,
        topbar_help_href: draft.topbar_help_href,
        topbar_support_href: draft.topbar_support_href,
        topbar_contact_href: draft.topbar_contact_href,
        topbar_phone_label: draft.topbar_phone_label,
        header_promo_text: draft.header_promo_text,
        show_featured: draft.show_featured,
        show_new_arrivals: draft.show_new_arrivals,
        show_bestsellers: draft.show_bestsellers,
        show_deals: draft.show_deals,
        show_most_viewed: draft.show_most_viewed,
        hero_headline: draft.hero_headline,
        hero_subtext: draft.hero_subtext,
        hero_cta_label: draft.hero_cta_label,
        hero_cta_href: draft.hero_cta_href,
        newsletter_headline: draft.newsletter_headline,
        newsletter_subtext: draft.newsletter_subtext,
        footer_tagline: draft.footer_tagline,
        social_facebook: draft.social_facebook,
        social_instagram: draft.social_instagram,
        social_twitter: draft.social_twitter,
        social_tiktok: draft.social_tiktok,
        contact_phone: draft.contact_phone,
        contact_email: draft.contact_email,
        business_hours: draft.business_hours,
        app_store_url: draft.app_store_url,
        play_store_url: draft.play_store_url,
        enabled_currencies: draft.enabled_currencies,
        trust_badge_1_title: draft.trust_badge_1_title,
        trust_badge_1_subtext: draft.trust_badge_1_subtext,
        trust_badge_2_title: draft.trust_badge_2_title,
        trust_badge_2_subtext: draft.trust_badge_2_subtext,
        trust_badge_3_title: draft.trust_badge_3_title,
        trust_badge_3_subtext: draft.trust_badge_3_subtext,
        trust_badge_4_title: draft.trust_badge_4_title,
        trust_badge_4_subtext: draft.trust_badge_4_subtext,
        pay_label_stk: draft.pay_label_stk,
        pay_label_paybill: draft.pay_label_paybill,
        pay_label_cash: draft.pay_label_cash,
        pay_hint_stk: draft.pay_hint_stk,
        pay_hint_paybill: draft.pay_hint_paybill,
        pay_hint_cash: draft.pay_hint_cash,
        pay_cta_stk: draft.pay_cta_stk,
        pay_cta_paybill: draft.pay_cta_paybill,
        pay_cta_cash: draft.pay_cta_cash,
      });
      setSettings(next);
      setDraft(next);
      setSaved("Storefront content saved.");
    } catch (e) {
      setError(e instanceof Error ? e.message : "Save failed");
    } finally {
      setBusy(false);
    }
  }

  async function onLogoPick(file: File | null) {
    if (!file) return;
    setLogoBusy(true);
    setError("");
    try {
      const next = await uploadStorefrontLogo(file);
      setSettings(next);
      setDraft(next);
      setSaved("Logo uploaded.");
    } catch (e) {
      setError(e instanceof Error ? e.message : "Logo upload failed");
    } finally {
      setLogoBusy(false);
    }
  }

  async function onLogoClear() {
    setLogoBusy(true);
    setError("");
    try {
      const next = await deleteStorefrontLogo();
      setSettings(next);
      setDraft(next);
      setSaved("Logo removed.");
    } catch (e) {
      setError(e instanceof Error ? e.message : "Could not remove logo");
    } finally {
      setLogoBusy(false);
    }
  }

  if (!draft && !error) return <div className="boot">Loading…</div>;

  return (
    <div className="settings-page">
      <PageHeader
        title="Storefront"
        subtitle="Brand, colors, layout and copy for the public web shop — nothing here is hardcoded"
      />
      {error ? <p className="form-error">{error}</p> : null}
      {saved ? (
        <p className="form-success">
          <Badge tone="success">saved</Badge> {saved}
        </p>
      ) : null}

      {draft ? (
        <>
          <section className="settings-form-card form-grid">
            <h2 style={{ margin: 0 }}>Brand</h2>
            <p className="hint">Shown in the storefront header, footer and browser tab. Leave blank to use the tenant name.</p>
            <label>
              Shop display name
              <Input
                value={draft.shop_display_name ?? ""}
                placeholder="Your shop name"
                onChange={(e) => set("shop_display_name", e.target.value)}
              />
            </label>
            <label>
              Browser page title
              <Input
                value={draft.page_title ?? ""}
                placeholder="Shop name · Online store"
                onChange={(e) => set("page_title", e.target.value)}
              />
            </label>
            <div className="field-pair" style={{ alignItems: "end" }}>
              <label>
                Logo
                <input
                  type="file"
                  accept="image/png,image/jpeg,image/webp"
                  disabled={logoBusy}
                  onChange={(e) => void onLogoPick(e.target.files?.[0] ?? null)}
                />
              </label>
              {draft.has_logo ? (
                <div style={{ display: "flex", gap: 12, alignItems: "center" }}>
                  <img
                    src={storefrontLogoURL(draft.tenant_id, draft.logo_updated_at)}
                    alt="Shop logo"
                    style={{ maxHeight: 48, maxWidth: 160, objectFit: "contain", background: "#f4f4f4", padding: 6 }}
                  />
                  <Button type="button" variant="ghost" disabled={logoBusy} onClick={() => void onLogoClear()}>
                    Remove
                  </Button>
                </div>
              ) : (
                <p className="hint">No logo yet — the shop name is shown as text.</p>
              )}
            </div>
          </section>

          <section className="settings-form-card form-grid">
            <h2 style={{ margin: 0 }}>Colors</h2>
            <p className="hint">
              TechLane defaults: gold for buttons/highlights, charcoal for dark chrome, navy only for small accents
              (avoids a blue-heavy shop).
            </p>
            <div className="field-pair">
              <label>
                Primary (gold)
                <div style={{ display: "flex", gap: 8, alignItems: "center" }}>
                  <input
                    type="color"
                    value={draft.color_primary || "#F2BE2A"}
                    onChange={(e) => set("color_primary", e.target.value)}
                  />
                  <Input value={draft.color_primary ?? ""} onChange={(e) => set("color_primary", e.target.value)} />
                </div>
              </label>
              <label>
                Secondary (charcoal)
                <div style={{ display: "flex", gap: 8, alignItems: "center" }}>
                  <input
                    type="color"
                    value={draft.color_secondary || "#1A1A1A"}
                    onChange={(e) => set("color_secondary", e.target.value)}
                  />
                  <Input value={draft.color_secondary ?? ""} onChange={(e) => set("color_secondary", e.target.value)} />
                </div>
              </label>
            </div>
            <label>
              Accent (navy — sparingly)
              <div style={{ display: "flex", gap: 8, alignItems: "center", maxWidth: 320 }}>
                <input
                  type="color"
                  value={draft.color_accent || "#060386"}
                  onChange={(e) => set("color_accent", e.target.value)}
                />
                <Input value={draft.color_accent ?? ""} onChange={(e) => set("color_accent", e.target.value)} />
              </div>
            </label>
            <div style={{ display: "flex", gap: 8 }}>
              {[draft.color_primary || "#F2BE2A", draft.color_secondary || "#1A1A1A", draft.color_accent || "#060386"].map(
                (c) => (
                  <span
                    key={c}
                    title={c}
                    style={{
                      width: 36,
                      height: 36,
                      borderRadius: 6,
                      background: c,
                      border: "1px solid rgba(0,0,0,0.12)",
                    }}
                  />
                ),
              )}
            </div>
          </section>

          <section className="settings-form-card form-grid">
            <h2 style={{ margin: 0 }}>Top bar &amp; contact</h2>
            <p className="hint">Electro-style top strip links and phone. Contact page uses the footer phone/email/hours below.</p>
            <label>
              Phone label (top bar)
              <Input
                value={draft.topbar_phone_label ?? ""}
                placeholder="+254 …"
                onChange={(e) => set("topbar_phone_label", e.target.value)}
              />
            </label>
            <label>
              Header promo strip
              <Input
                value={draft.header_promo_text ?? ""}
                placeholder="Free branch pickup on all online orders"
                onChange={(e) => set("header_promo_text", e.target.value)}
              />
            </label>
            <div className="field-pair">
              <label>
                Help link
                <Input
                  value={draft.topbar_help_href ?? ""}
                  placeholder="/contact"
                  onChange={(e) => set("topbar_help_href", e.target.value)}
                />
              </label>
              <label>
                Support link
                <Input
                  value={draft.topbar_support_href ?? ""}
                  placeholder="/contact"
                  onChange={(e) => set("topbar_support_href", e.target.value)}
                />
              </label>
            </div>
            <label>
              Contact link
              <Input
                value={draft.topbar_contact_href ?? ""}
                placeholder="/contact"
                onChange={(e) => set("topbar_contact_href", e.target.value)}
              />
            </label>
          </section>

          <section className="settings-form-card form-grid">
            <h2 style={{ margin: 0 }}>Homepage rails</h2>
            <p className="hint">
              Toggle which product rows appear on the home page. Product flags (featured / new / bestseller) are still
              set in Inventory.
            </p>
            <div className="toggle-grid">
              {(
                [
                  ["show_featured", "Featured"],
                  ["show_new_arrivals", "New arrivals"],
                  ["show_bestsellers", "Bestsellers"],
                  ["show_deals", "Deals"],
                  ["show_most_viewed", "Most viewed"],
                ] as const
              ).map(([key, label]) => (
                <label key={key} className="toggle-card">
                  <input type="checkbox" checked={Boolean(draft[key])} onChange={(e) => set(key, e.target.checked)} />
                  <span>
                    <strong>{label}</strong>
                  </span>
                </label>
              ))}
            </div>
          </section>

          <section className="settings-form-card form-grid">
            <h2 style={{ margin: 0 }}>Hero</h2>
            <p className="hint">The banner slider (Settings → Storefront banners) sits above this; this is the fallback copy.</p>
            <label>
              Headline
              <Input value={draft.hero_headline} onChange={(e) => set("hero_headline", e.target.value)} />
            </label>
            <label>
              Subtext
              <textarea
                className="input"
                rows={2}
                value={draft.hero_subtext}
                onChange={(e) => set("hero_subtext", e.target.value)}
              />
            </label>
            <div className="field-pair">
              <label>
                Button label
                <Input value={draft.hero_cta_label} onChange={(e) => set("hero_cta_label", e.target.value)} />
              </label>
              <label>
                Button link
                <Input
                  value={draft.hero_cta_href}
                  placeholder="/shop"
                  onChange={(e) => set("hero_cta_href", e.target.value)}
                />
              </label>
            </div>
          </section>

          <section className="settings-form-card form-grid">
            <h2 style={{ margin: 0 }}>Newsletter</h2>
            <label>
              Headline
              <Input value={draft.newsletter_headline} onChange={(e) => set("newsletter_headline", e.target.value)} />
            </label>
            <label>
              Subtext
              <Input value={draft.newsletter_subtext} onChange={(e) => set("newsletter_subtext", e.target.value)} />
            </label>
            <p className="hint">
              Signups are captured for real — see <strong>Settings &rarr; Newsletter subscribers</strong>.
            </p>
          </section>

          <section className="settings-form-card form-grid">
            <h2 style={{ margin: 0 }}>Footer &amp; contact</h2>
            <label>
              Footer tagline
              <Input value={draft.footer_tagline} onChange={(e) => set("footer_tagline", e.target.value)} />
            </label>
            <div className="field-pair">
              <label>
                Contact phone
                <Input value={draft.contact_phone} onChange={(e) => set("contact_phone", e.target.value)} />
              </label>
              <label>
                Contact email
                <Input value={draft.contact_email} onChange={(e) => set("contact_email", e.target.value)} />
              </label>
            </div>
            <label>
              Business hours
              <Input
                value={draft.business_hours}
                placeholder="Mon–Sat, 8am–6pm"
                onChange={(e) => set("business_hours", e.target.value)}
              />
            </label>
            <div className="field-pair">
              <label>
                App Store link
                <Input value={draft.app_store_url} onChange={(e) => set("app_store_url", e.target.value)} />
              </label>
              <label>
                Play Store link
                <Input value={draft.play_store_url} onChange={(e) => set("play_store_url", e.target.value)} />
              </label>
            </div>
            <div className="field-pair">
              <label>
                Facebook
                <Input value={draft.social_facebook} onChange={(e) => set("social_facebook", e.target.value)} />
              </label>
              <label>
                Instagram
                <Input value={draft.social_instagram} onChange={(e) => set("social_instagram", e.target.value)} />
              </label>
            </div>
            <div className="field-pair">
              <label>
                Twitter / X
                <Input value={draft.social_twitter} onChange={(e) => set("social_twitter", e.target.value)} />
              </label>
              <label>
                TikTok
                <Input value={draft.social_tiktok} onChange={(e) => set("social_tiktok", e.target.value)} />
              </label>
            </div>
          </section>

          <section className="settings-form-card form-grid">
            <h2 style={{ margin: 0 }}>Currency switcher</h2>
            <p className="hint">
              Display-only — checkout always charges in KES via M-Pesa. Extra currencies convert the displayed
              price using a live exchange rate, purely for shopper convenience.
            </p>
            <div className="toggle-grid">
              {EXTRA_CURRENCIES.map((code) => {
                const current = parseCurrencies(draft.enabled_currencies);
                const checked = current.includes(code);
                return (
                  <label key={code} className="toggle-card">
                    <input
                      type="checkbox"
                      checked={checked}
                      onChange={(e) => {
                        const extras = new Set(current.filter((c) => c !== "KES"));
                        if (e.target.checked) extras.add(code);
                        else extras.delete(code);
                        const next = extras.size > 0 ? ["KES", ...extras].join(",") : "";
                        set("enabled_currencies", next);
                      }}
                    />
                    <span>
                      <strong>{code}</strong>
                    </span>
                  </label>
                );
              })}
            </div>
          </section>

          <section className="settings-form-card form-grid">
            <h2 style={{ margin: 0 }}>Checkout payment labels</h2>
            <p className="hint">
              Labels, hints and button text on the public checkout. Leave blank to use TechLane defaults (STK, paybill,
              cash on pickup).
            </p>
            <div className="field-pair">
              <label>
                STK label
                <Input
                  value={draft.pay_label_stk ?? ""}
                  placeholder="M-Pesa STK push"
                  onChange={(e) => set("pay_label_stk", e.target.value)}
                />
              </label>
              <label>
                STK CTA
                <Input
                  value={draft.pay_cta_stk ?? ""}
                  placeholder="Pay with M-Pesa STK"
                  onChange={(e) => set("pay_cta_stk", e.target.value)}
                />
              </label>
            </div>
            <label>
              STK phone field hint
              <Input
                value={draft.pay_hint_stk ?? ""}
                placeholder="Phone (07… or 2547…)"
                onChange={(e) => set("pay_hint_stk", e.target.value)}
              />
            </label>
            <div className="field-pair">
              <label>
                Paybill label
                <Input
                  value={draft.pay_label_paybill ?? ""}
                  placeholder="M-Pesa paybill"
                  onChange={(e) => set("pay_label_paybill", e.target.value)}
                />
              </label>
              <label>
                Paybill CTA
                <Input
                  value={draft.pay_cta_paybill ?? ""}
                  placeholder="Checkout with paybill"
                  onChange={(e) => set("pay_cta_paybill", e.target.value)}
                />
              </label>
            </div>
            <label>
              Paybill hint
              <Input
                value={draft.pay_hint_paybill ?? ""}
                placeholder="After checkout you will pay by paybill…"
                onChange={(e) => set("pay_hint_paybill", e.target.value)}
              />
            </label>
            <div className="field-pair">
              <label>
                Cash on pickup label
                <Input
                  value={draft.pay_label_cash ?? ""}
                  placeholder="Cash on pickup"
                  onChange={(e) => set("pay_label_cash", e.target.value)}
                />
              </label>
              <label>
                Cash on pickup CTA
                <Input
                  value={draft.pay_cta_cash ?? ""}
                  placeholder="Reserve for cash pickup"
                  onChange={(e) => set("pay_cta_cash", e.target.value)}
                />
              </label>
            </div>
            <label>
              Cash on pickup hint
              <Input
                value={draft.pay_hint_cash ?? ""}
                placeholder="Stock is held for pickup…"
                onChange={(e) => set("pay_hint_cash", e.target.value)}
              />
            </label>
          </section>

          <section className="settings-form-card form-grid">
            <h2 style={{ margin: 0 }}>Trust badges</h2>
            <p className="hint">
              Shown in a row on the homepage. Leave blank to hide — nothing here is a hardcoded claim about your
              business.
            </p>
            {([1, 2, 3, 4] as const).map((n) => (
              <div className="field-pair" key={n}>
                <label>
                  Badge {n} title
                  <Input
                    value={draft[`trust_badge_${n}_title`]}
                    onChange={(e) => set(`trust_badge_${n}_title`, e.target.value)}
                  />
                </label>
                <label>
                  Badge {n} subtext
                  <Input
                    value={draft[`trust_badge_${n}_subtext`]}
                    onChange={(e) => set(`trust_badge_${n}_subtext`, e.target.value)}
                  />
                </label>
              </div>
            ))}
          </section>

          <div className="chip-row settings-save-row">
            <Button type="button" disabled={busy || !dirty} onClick={() => void save()}>
              {busy ? "Saving…" : "Save storefront content"}
            </Button>
            {dirty ? <span className="hint">Unsaved changes</span> : null}
          </div>
        </>
      ) : null}
    </div>
  );
}
