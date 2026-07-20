import { useEffect, useState, type FormEvent } from "react";
import { Button, PageHeader } from "../../components/ui";
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
    <div>
      <PageHeader
        title="Shop profile & tax"
        subtitle="Legal name, TIN, and VAT used on receipts and tax invoices"
      />
      {error ? <p className="form-error">{error}</p> : null}
      {ok ? <p className="form-ok">{ok}</p> : null}
      <form className="panel form-grid" onSubmit={onSubmit}>
        <label>
          Legal name
          <input
            value={form.legal_name ?? ""}
            onChange={(e) => setForm((f) => ({ ...f, legal_name: e.target.value }))}
          />
        </label>
        <label>
          TIN / PIN
          <input value={form.tin ?? ""} onChange={(e) => setForm((f) => ({ ...f, tin: e.target.value }))} />
        </label>
        <label>
          Address line 1
          <input
            value={form.address_line1 ?? ""}
            onChange={(e) => setForm((f) => ({ ...f, address_line1: e.target.value }))}
          />
        </label>
        <label>
          Address line 2
          <input
            value={form.address_line2 ?? ""}
            onChange={(e) => setForm((f) => ({ ...f, address_line2: e.target.value }))}
          />
        </label>
        <label>
          City
          <input value={form.city ?? ""} onChange={(e) => setForm((f) => ({ ...f, city: e.target.value }))} />
        </label>
        <label>
          Country
          <input
            value={form.country ?? "KE"}
            onChange={(e) => setForm((f) => ({ ...f, country: e.target.value }))}
          />
        </label>
        <label>
          VAT rate (basis points, 1600 = 16%)
          <input
            type="number"
            value={form.vat_rate_bps ?? 1600}
            onChange={(e) => setForm((f) => ({ ...f, vat_rate_bps: Number(e.target.value) }))}
          />
        </label>
        <label>
          <input
            type="checkbox"
            checked={form.vat_inclusive ?? true}
            onChange={(e) => setForm((f) => ({ ...f, vat_inclusive: e.target.checked }))}
          />{" "}
          Prices are VAT inclusive
        </label>
        <Button type="submit" disabled={busy}>
          {busy ? "Saving…" : "Save"}
        </Button>
      </form>
    </div>
  );
}
