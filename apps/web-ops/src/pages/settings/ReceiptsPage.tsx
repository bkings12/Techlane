import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { useAuth } from "../../auth/AuthContext";
import { Badge, Button, Input, PageHeader } from "../../components/ui";
import {
  deleteReceiptLogo,
  getReceiptSettings,
  previewReceipt,
  updateReceiptSettings,
  uploadReceiptLogo,
  type ReceiptPaper,
  type ReceiptSettings,
} from "../../lib/api";

const PAPERS: Array<{ value: ReceiptPaper; label: string; hint: string }> = [
  { value: "thermal80", label: "80mm", hint: "Standard counter / POS printer" },
  { value: "thermal58", label: "58mm", hint: "Narrow handheld printer" },
  { value: "a4", label: "A4", hint: "Office printer and PDF" },
];

const KINDS = [
  { value: "repair", label: "Repair receipt" },
  { value: "sale", label: "POS sale" },
  { value: "tax_invoice", label: "Tax invoice" },
];

const TOGGLES: Array<{ key: keyof ReceiptSettings; label: string; hint: string }> = [
  { key: "show_vat_breakdown", label: "VAT breakdown", hint: "Print the net subtotal and VAT lines" },
  { key: "show_imei", label: "IMEI / serial", hint: "Include the device identifier on repair receipts" },
  { key: "show_payments", label: "Payment list", hint: "Itemise how the customer paid" },
  { key: "show_balance", label: "Paid and balance", hint: "Show what is still owed" },
  { key: "show_served_by", label: "Served by", hint: "Credit the technician or cashier at the bottom" },
];

export function ReceiptsPage() {
  const { user } = useAuth();
  const isOwner = user?.roles?.includes("owner") ?? false;

  const [settings, setSettings] = useState<ReceiptSettings | null>(null);
  const [draft, setDraft] = useState<ReceiptSettings | null>(null);
  const [error, setError] = useState("");
  const [saved, setSaved] = useState("");
  const [busy, setBusy] = useState(false);
  const [logoBusy, setLogoBusy] = useState(false);

  const [previewKind, setPreviewKind] = useState("repair");
  const [previewPaper, setPreviewPaper] = useState<ReceiptPaper>("thermal80");
  const [previewHTML, setPreviewHTML] = useState("");
  const [previewError, setPreviewError] = useState("");
  const fileInput = useRef<HTMLInputElement>(null);

  useEffect(() => {
    getReceiptSettings()
      .then((s) => {
        setSettings(s);
        setDraft(s);
        setPreviewPaper(s.default_paper);
      })
      .catch((e) => setError(e instanceof Error ? e.message : "Failed to load receipt settings"));
  }, []);

  const dirty = useMemo(
    () => Boolean(settings && draft) && JSON.stringify(settings) !== JSON.stringify(draft),
    [settings, draft],
  );

  // Preview is rendered server-side so it is byte-for-byte what the printer gets.
  const previewKey = draft ? JSON.stringify(draft) : "";
  useEffect(() => {
    if (!draft) return;
    let cancelled = false;
    const timer = window.setTimeout(() => {
      previewReceipt(draft, previewKind, previewPaper)
        .then((html) => {
          if (cancelled) return;
          setPreviewHTML(html);
          setPreviewError("");
        })
        .catch((e) => {
          if (cancelled) return;
          setPreviewError(e instanceof Error ? e.message : "Preview failed");
        });
    }, 250);
    return () => {
      cancelled = true;
      window.clearTimeout(timer);
    };
  }, [previewKey, previewKind, previewPaper, draft]);

  const set = useCallback(<K extends keyof ReceiptSettings>(key: K, value: ReceiptSettings[K]) => {
    setDraft((prev) => (prev ? { ...prev, [key]: value } : prev));
    setSaved("");
  }, []);

  async function save() {
    if (!draft) return;
    setBusy(true);
    setError("");
    setSaved("");
    try {
      const next = await updateReceiptSettings({
        header_note: draft.header_note,
        phone: draft.phone,
        email: draft.email,
        website: draft.website,
        show_logo: draft.show_logo,
        show_address: draft.show_address,
        show_tin: draft.show_tin,
        thank_you_text: draft.thank_you_text,
        footer_text: draft.footer_text,
        warranty_text: draft.warranty_text,
        show_vat_breakdown: draft.show_vat_breakdown,
        show_imei: draft.show_imei,
        show_payments: draft.show_payments,
        show_balance: draft.show_balance,
        show_served_by: draft.show_served_by,
        default_paper: draft.default_paper,
        number_prefix: draft.number_prefix,
        next_number: draft.next_number,
      });
      setSettings(next);
      setDraft(next);
      setSaved("Receipt design saved. New receipts print with these settings.");
    } catch (e) {
      setError(e instanceof Error ? e.message : "Save failed");
    } finally {
      setBusy(false);
    }
  }

  async function onLogoPicked(file: File | undefined) {
    if (!file) return;
    setLogoBusy(true);
    setError("");
    setSaved("");
    try {
      const next = await uploadReceiptLogo(file);
      setSettings(next);
      setDraft((prev) => (prev ? { ...prev, has_logo: next.has_logo, logo_updated_at: next.logo_updated_at } : next));
      setSaved("Logo uploaded.");
    } catch (e) {
      setError(e instanceof Error ? e.message : "Logo upload failed");
    } finally {
      setLogoBusy(false);
      if (fileInput.current) fileInput.current.value = "";
    }
  }

  async function removeLogo() {
    setLogoBusy(true);
    setError("");
    try {
      const next = await deleteReceiptLogo();
      setSettings(next);
      setDraft((prev) => (prev ? { ...prev, has_logo: false, logo_updated_at: undefined } : next));
      setSaved("Logo removed.");
    } catch (e) {
      setError(e instanceof Error ? e.message : "Could not remove the logo");
    } finally {
      setLogoBusy(false);
    }
  }

  function openPreviewWindow() {
    const win = window.open("", "_blank", "noopener,noreferrer");
    if (!win) {
      setPreviewError("Pop-up blocked — allow pop-ups to open the full preview");
      return;
    }
    win.document.write(previewHTML);
    win.document.close();
  }

  if (!draft && !error) return <div className="boot">Loading…</div>;

  return (
    <div className="settings-page">
      <PageHeader
        title="Receipts"
        subtitle="Branding, wording and paper size for every receipt, invoice and voucher you print"
      />
      {error ? <p className="form-error">{error}</p> : null}
      {saved ? (
        <p className="form-success">
          <Badge tone="success">saved</Badge> {saved}
        </p>
      ) : null}
      {!isOwner ? (
        <p className="form-error">Viewing as non-owner. Only the shop owner can change the receipt design.</p>
      ) : null}

      {draft ? (
        <div className="receipt-studio">
          <div className="receipt-controls">
            <section className="settings-form-card form-grid">
              <div className="chip-row">
                <h2 style={{ margin: 0 }}>Branding</h2>
                <Badge tone={draft.has_logo ? "success" : "pending"}>{draft.has_logo ? "logo set" : "no logo"}</Badge>
              </div>
              <p className="hint">
                The shop name, address, PIN and VAT rate come from{" "}
                <strong>Settings &rarr; Shop &amp; tax</strong>. Everything below is receipt-only.
              </p>

              <div className="logo-row">
                <div className="logo-frame">
                  {draft.has_logo ? <span className="logo-set">Logo on file</span> : <span className="logo-empty">No logo</span>}
                </div>
                <div className="logo-actions">
                  <input
                    ref={fileInput}
                    type="file"
                    accept="image/png,image/jpeg,image/webp,image/gif,image/svg+xml"
                    style={{ display: "none" }}
                    onChange={(e) => void onLogoPicked(e.target.files?.[0])}
                  />
                  <Button
                    type="button"
                    variant="secondary"
                    disabled={!isOwner || logoBusy}
                    onClick={() => fileInput.current?.click()}
                  >
                    {logoBusy ? "Working…" : draft.has_logo ? "Replace logo" : "Upload logo"}
                  </Button>
                  {draft.has_logo ? (
                    <Button type="button" variant="secondary" disabled={!isOwner || logoBusy} onClick={() => void removeLogo()}>
                      Remove
                    </Button>
                  ) : null}
                  <p className="hint">PNG, JPEG, WebP, GIF or SVG up to 512&nbsp;KB. A wide, high-contrast mark prints best on thermal paper.</p>
                </div>
              </div>

              <label className="checkbox-row">
                <input
                  type="checkbox"
                  checked={draft.show_logo}
                  disabled={!isOwner}
                  onChange={(e) => set("show_logo", e.target.checked)}
                />
                Print the logo
              </label>

              <label>
                Header line
                <Input
                  value={draft.header_note}
                  disabled={!isOwner}
                  placeholder="Authorised device service centre"
                  onChange={(e) => set("header_note", e.target.value)}
                />
              </label>

              <div className="field-pair">
                <label>
                  Phone
                  <Input
                    value={draft.phone}
                    disabled={!isOwner}
                    placeholder="+254 720 000 000"
                    onChange={(e) => set("phone", e.target.value)}
                  />
                </label>
                <label>
                  Email
                  <Input
                    value={draft.email}
                    disabled={!isOwner}
                    placeholder="hello@yourshop.co.ke"
                    onChange={(e) => set("email", e.target.value)}
                  />
                </label>
              </div>

              <label>
                Website
                <Input
                  value={draft.website}
                  disabled={!isOwner}
                  placeholder="yourshop.co.ke"
                  onChange={(e) => set("website", e.target.value)}
                />
              </label>

              <label className="checkbox-row">
                <input
                  type="checkbox"
                  checked={draft.show_address}
                  disabled={!isOwner}
                  onChange={(e) => set("show_address", e.target.checked)}
                />
                Print the shop address
              </label>
              <label className="checkbox-row">
                <input
                  type="checkbox"
                  checked={draft.show_tin}
                  disabled={!isOwner}
                  onChange={(e) => set("show_tin", e.target.checked)}
                />
                Print the KRA PIN / TIN
              </label>
            </section>

            <section className="settings-form-card form-grid">
              <h2 style={{ margin: 0 }}>Footer wording</h2>
              <p className="hint">Printed at the bottom of every receipt. Leave a field blank to drop that line.</p>

              <label>
                Thank-you line
                <Input
                  value={draft.thank_you_text}
                  disabled={!isOwner}
                  placeholder="Thank you for your business"
                  onChange={(e) => set("thank_you_text", e.target.value)}
                />
              </label>

              <label>
                Warranty / returns note
                <textarea
                  className="input"
                  rows={2}
                  value={draft.warranty_text}
                  disabled={!isOwner}
                  onChange={(e) => set("warranty_text", e.target.value)}
                />
              </label>

              <label>
                Extra footer line
                <textarea
                  className="input"
                  rows={2}
                  value={draft.footer_text}
                  disabled={!isOwner}
                  placeholder="Goods once collected are checked and accepted by the customer."
                  onChange={(e) => set("footer_text", e.target.value)}
                />
              </label>
            </section>

            <section className="settings-form-card form-grid">
              <h2 style={{ margin: 0 }}>What to print</h2>
              <div className="toggle-grid">
                {TOGGLES.map((toggle) => (
                  <label key={toggle.key} className="toggle-card">
                    <input
                      type="checkbox"
                      checked={Boolean(draft[toggle.key])}
                      disabled={!isOwner}
                      onChange={(e) => set(toggle.key, e.target.checked as never)}
                    />
                    <span>
                      <strong>{toggle.label}</strong>
                      <em>{toggle.hint}</em>
                    </span>
                  </label>
                ))}
              </div>
            </section>

            <section className="settings-form-card form-grid">
              <h2 style={{ margin: 0 }}>Paper &amp; numbering</h2>

              <div className="paper-picker">
                {PAPERS.map((paper) => (
                  <button
                    key={paper.value}
                    type="button"
                    className={draft.default_paper === paper.value ? "paper-option active" : "paper-option"}
                    disabled={!isOwner}
                    onClick={() => {
                      set("default_paper", paper.value);
                      setPreviewPaper(paper.value);
                    }}
                  >
                    <strong>{paper.label}</strong>
                    <em>{paper.hint}</em>
                  </button>
                ))}
              </div>
              <p className="hint">
                Staff can still print any receipt on another size; this is what prints when nobody chooses.
              </p>

              <div className="field-pair">
                <label>
                  Number prefix
                  <Input
                    value={draft.number_prefix}
                    disabled={!isOwner}
                    placeholder="RCT-"
                    maxLength={12}
                    onChange={(e) => set("number_prefix", e.target.value)}
                  />
                </label>
                <label>
                  Next receipt number
                  <Input
                    type="number"
                    min={1}
                    value={draft.next_number}
                    disabled={!isOwner}
                    onChange={(e) => set("next_number", Math.max(1, Number(e.target.value) || 1))}
                  />
                </label>
              </div>
              <p className="hint">
                Next receipt prints as{" "}
                <code>{`${draft.number_prefix}${String(draft.next_number).padStart(5, "0")}`}</code>. A number is
                assigned once per job or sale, so reprints keep the same serial.
              </p>
            </section>

            <div className="chip-row settings-save-row">
              <Button type="button" disabled={!isOwner || busy || !dirty} onClick={() => void save()}>
                {busy ? "Saving…" : "Save receipt design"}
              </Button>
              {dirty ? <span className="hint">Unsaved changes — the preview already shows them.</span> : null}
            </div>
          </div>

          <aside className="receipt-preview">
            <div className="receipt-preview-head">
              <strong>Live preview</strong>
              <Button type="button" variant="secondary" onClick={openPreviewWindow} disabled={!previewHTML}>
                Open full size
              </Button>
            </div>

            <div className="chip-row">
              {KINDS.map((kind) => (
                <button
                  key={kind.value}
                  type="button"
                  className={previewKind === kind.value ? "chip active" : "chip"}
                  onClick={() => setPreviewKind(kind.value)}
                >
                  {kind.label}
                </button>
              ))}
            </div>
            <div className="chip-row">
              {PAPERS.map((paper) => (
                <button
                  key={paper.value}
                  type="button"
                  className={previewPaper === paper.value ? "chip active" : "chip"}
                  onClick={() => setPreviewPaper(paper.value)}
                >
                  {paper.label}
                </button>
              ))}
            </div>

            {previewError ? <p className="form-error">{previewError}</p> : null}
            <div className={previewPaper === "a4" ? "receipt-frame wide" : "receipt-frame"}>
              <iframe title="Receipt preview" srcDoc={previewHTML} />
            </div>
          </aside>
        </div>
      ) : null}
    </div>
  );
}
