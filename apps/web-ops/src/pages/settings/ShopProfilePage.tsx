import { useEffect, useState, type FormEvent } from "react";
import { Button, Input, PageHeader } from "../../components/ui";
import { getShopProfile, putShopProfile, type ShopProfile } from "../../lib/api";

export function ShopProfilePage() {
  const [form, setForm] = useState<Partial<ShopProfile>>({});
  const [error, setError] = useState("");
  const [ok, setOk] = useState("");
  const [busy, setBusy] = useState(false);

  useEffect(() => {
    void (async () => {
      try {
        setForm(await getShopProfile());
      } catch (e) {
        setError(e instanceof Error ? e.message : "Failed to load");
      }
    })();
  }, []);

  async function onSubmit(e: FormEvent) {
    e.preventDefault();
    setBusy(true);
    setError("");
    setOk("");
    try {
      const saved = await putShopProfile({
        legal_name: form.legal_name,
        tin: form.tin,
        address_line1: form.address_line1,
        address_line2: form.address_line2,
        city: form.city,
        country: form.country,
        vat_rate_bps: form.vat_rate_bps,
        vat_inclusive: form.vat_inclusive,
        currency_code: form.currency_code,
        locale: form.locale,
      });
      setForm(saved);
      setOk("Saved");
    } catch (err) {
      setError(err instanceof Error ? err.message : "Save failed");
    } finally {
      setBusy(false);
    }
  }

  return (
    <div className="settings-page">
      <PageHeader
        title="Shop profile & tax"
        subtitle="Legal name, TIN, and VAT used on receipts and tax invoices"
      />
      {error ? <p className="form-error">{error}</p> : null}
      {ok ? <p className="form-success">{ok}</p> : null}

      <form className="settings-form-card form-grid" onSubmit={onSubmit}>
        <label>
          Legal name
          <Input
            value={form.legal_name ?? ""}
            onChange={(e) => setForm((f) => ({ ...f, legal_name: e.target.value }))}
          />
        </label>
        <label>
          TIN / PIN
          <Input value={form.tin ?? ""} onChange={(e) => setForm((f) => ({ ...f, tin: e.target.value }))} />
        </label>
        <label>
          Address line 1
          <Input
            value={form.address_line1 ?? ""}
            onChange={(e) => setForm((f) => ({ ...f, address_line1: e.target.value }))}
          />
        </label>
        <label>
          Address line 2
          <Input
            value={form.address_line2 ?? ""}
            onChange={(e) => setForm((f) => ({ ...f, address_line2: e.target.value }))}
          />
        </label>
        <label>
          City
          <Input value={form.city ?? ""} onChange={(e) => setForm((f) => ({ ...f, city: e.target.value }))} />
        </label>
        <label>
          Country
          <Input
            value={form.country ?? "KE"}
            onChange={(e) => setForm((f) => ({ ...f, country: e.target.value }))}
          />
        </label>
        <label>
          VAT rate (basis points, 1600 = 16%)
          <Input
            type="number"
            value={form.vat_rate_bps ?? 1600}
            onChange={(e) => setForm((f) => ({ ...f, vat_rate_bps: Number(e.target.value) }))}
          />
        </label>
        <label className="checkbox-row">
          <input
            type="checkbox"
            checked={form.vat_inclusive ?? true}
            onChange={(e) => setForm((f) => ({ ...f, vat_inclusive: e.target.checked }))}
          />
          Prices are VAT inclusive
        </label>
        <label>
          Currency
          <select
            className="input"
            value={form.currency_code ?? "KES"}
            onChange={(e) => setForm((f) => ({ ...f, currency_code: e.target.value }))}
          >
            <option value="KES">KES — Kenyan Shilling</option>
            <option value="UGX">UGX — Ugandan Shilling</option>
            <option value="TZS">TZS — Tanzanian Shilling</option>
            <option value="USD">USD — US Dollar</option>
            <option value="EUR">EUR — Euro</option>
            <option value="GBP">GBP — British Pound</option>
            <option value="NGN">NGN — Nigerian Naira</option>
            <option value="ZAR">ZAR — South African Rand</option>
          </select>
        </label>
        <label>
          Locale (number & date formatting)
          <select
            className="input"
            value={form.locale ?? "en-KE"}
            onChange={(e) => setForm((f) => ({ ...f, locale: e.target.value }))}
          >
            <option value="en-KE">English (Kenya)</option>
            <option value="en-UG">English (Uganda)</option>
            <option value="en-TZ">English (Tanzania)</option>
            <option value="en-NG">English (Nigeria)</option>
            <option value="en-ZA">English (South Africa)</option>
            <option value="en-GB">English (UK)</option>
            <option value="en-US">English (US)</option>
            <option value="fr-FR">French (France)</option>
          </select>
        </label>
        <p className="hint">
          Currency and locale control how amounts, dates, and numbers are formatted across the dashboard.
        </p>
        <Button type="submit" disabled={busy}>
          {busy ? "Saving…" : "Save"}
        </Button>
      </form>
    </div>
  );
}
