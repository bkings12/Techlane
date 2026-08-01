import { useEffect, useState, type FormEvent } from "react";
import { useAuth } from "../../auth/AuthContext";
import { Badge, Button, Input, PageHeader, PasswordInput } from "../../components/ui";
import {
  getSMSSettings,
  listSMSTemplates,
  updateSMSSettings,
  updateSMSTemplate,
  type SMSSettings,
  type SMSTemplate,
} from "../../lib/api";

export function SMSSettingsPage() {
  const { user } = useAuth();
  const isOwner = user?.roles?.includes("owner") ?? false;

  const [cfg, setCfg] = useState<SMSSettings | null>(null);
  const [templates, setTemplates] = useState<SMSTemplate[]>([]);
  const [drafts, setDrafts] = useState<Record<string, string>>({});
  const [error, setError] = useState("");
  const [saved, setSaved] = useState("");
  const [busy, setBusy] = useState(false);
  const [templateBusy, setTemplateBusy] = useState<string>("");

  const [enabled, setEnabled] = useState(false);
  const [senderID, setSenderID] = useState("");
  const [apiKey, setApiKey] = useState("");
  const [baseURL, setBaseURL] = useState("https://sms.blessedtexts.com/api/sms/v1");

  useEffect(() => {
    Promise.all([getSMSSettings(), listSMSTemplates()])
      .then(([c, t]) => {
        setCfg(c);
        setEnabled(c.enabled);
        setSenderID(c.sender_id || "");
        setBaseURL(c.base_url || "https://sms.blessedtexts.com/api/sms/v1");
        setTemplates(t.items ?? []);
        const next: Record<string, string> = {};
        for (const item of t.items ?? []) next[item.key] = item.body;
        setDrafts(next);
      })
      .catch((e) => setError(e instanceof Error ? e.message : "Failed to load"));
  }, []);

  async function submit(e: FormEvent) {
    e.preventDefault();
    setBusy(true);
    setError("");
    setSaved("");

    if (enabled && !senderID.trim()) {
      setError("Sender ID is required when SMS is enabled.");
      setBusy(false);
      return;
    }
    if (enabled && !cfg?.api_key_set && !apiKey.trim()) {
      setError("API key is required the first time you enable BlessedTexts.");
      setBusy(false);
      return;
    }

    try {
      const next = await updateSMSSettings({
        enabled,
        provider: "blessedtexts",
        sender_id: senderID.trim(),
        base_url: baseURL.trim(),
        api_key: apiKey.trim() || undefined,
      });
      setCfg(next);
      setApiKey("");
      setSaved(
        next.enabled && next.configured
          ? "Saved. SMS delivery is ready for OTP and job notifications."
          : "Saved. Enable SMS and finish configuration before messages will send.",
      );
    } catch (err) {
      setError(err instanceof Error ? err.message : "Save failed");
    } finally {
      setBusy(false);
    }
  }

  async function saveTemplate(key: string) {
    setTemplateBusy(key);
    setError("");
    setSaved("");
    try {
      const updated = await updateSMSTemplate(key, drafts[key] ?? "");
      setTemplates((prev) => prev.map((t) => (t.key === key ? updated : t)));
      setDrafts((prev) => ({ ...prev, [key]: updated.body }));
      setSaved(`Template “${updated.label}” saved.`);
    } catch (err) {
      setError(err instanceof Error ? err.message : "Template save failed");
    } finally {
      setTemplateBusy("");
    }
  }

  async function resetTemplate(key: string, defaultBody: string) {
    setDrafts((prev) => ({ ...prev, [key]: defaultBody }));
    setTemplateBusy(key);
    setError("");
    setSaved("");
    try {
      const updated = await updateSMSTemplate(key, "");
      setTemplates((prev) => prev.map((t) => (t.key === key ? updated : t)));
      setDrafts((prev) => ({ ...prev, [key]: updated.body }));
      setSaved(`Template reset to default.`);
    } catch (err) {
      setError(err instanceof Error ? err.message : "Reset failed");
    } finally {
      setTemplateBusy("");
    }
  }

  function insertHelper(key: string, helper: string) {
    const token = `{{${helper}}}`;
    setDrafts((prev) => ({ ...prev, [key]: `${prev[key] ?? ""}${token}` }));
  }

  if (!cfg && !error) return <div className="boot">Loading…</div>;

  const ready = Boolean(cfg?.configured && cfg?.enabled);

  return (
    <div className="settings-page">
      <PageHeader
        title="SMS"
        subtitle="BlessedTexts delivery, OTP, and editable notification templates"
      />
      {error ? <p className="form-error">{error}</p> : null}
      {saved ? (
        <p className="form-success">
          <Badge tone="success">saved</Badge> {saved}
        </p>
      ) : null}

      {!isOwner ? (
        <p className="form-error">
          Viewing as non-owner. Only the shop owner can change SMS credentials and templates.
        </p>
      ) : null}

      <section className="board-pulse" aria-label="SMS pulse">
        <div>
          <strong>Owner</strong>
          <span>Access</span>
        </div>
        <div className={ready ? "" : "warn"}>
          <strong>{ready ? "Ready" : "Not ready"}</strong>
          <span>Delivery</span>
        </div>
        <div>
          <strong>{cfg?.enabled ? "On" : "Off"}</strong>
          <span>Enabled</span>
        </div>
        <div>
          <strong className="mono">{cfg?.sender_id || "—"}</strong>
          <span>Sender ID</span>
        </div>
      </section>

      <form className="settings-form-card form-grid" onSubmit={submit}>
        <div className="chip-row">
          <h2 style={{ margin: 0 }}>BlessedTexts</h2>
          <Badge tone={cfg?.api_key_set ? "success" : "warning"}>
            {cfg?.api_key_set ? "key on file" : "key missing"}
          </Badge>
        </div>
        <p className="hint">
          Used for customer OTP and all SMS notifications below. The API key is never shown after save — leave blank to
          keep the current value.
        </p>

        <label className="checkbox-row">
          <input type="checkbox" checked={enabled} onChange={(e) => setEnabled(e.target.checked)} />
          Enable BlessedTexts SMS
        </label>

        <label>
          Sender ID
          <Input
            value={senderID}
            onChange={(e) => setSenderID(e.target.value)}
            placeholder="23107"
            required={enabled}
          />
        </label>

        <label>
          API key {cfg?.api_key_set ? <span className="muted">(saved — enter a new key only to replace)</span> : null}
          <PasswordInput
            value={apiKey}
            onChange={(e) => setApiKey(e.target.value)}
            placeholder={cfg?.api_key_set ? "••••••••" : "From BlessedTexts profile"}
            autoComplete="new-password"
          />
        </label>

        <label>
          API base URL
          <Input
            value={baseURL}
            onChange={(e) => setBaseURL(e.target.value)}
            placeholder="https://sms.blessedtexts.com/api/sms/v1"
          />
        </label>

        <div className="chip-row">
          <Button type="submit" disabled={busy || !isOwner}>
            {busy ? "Saving…" : "Save SMS settings"}
          </Button>
        </div>
      </form>

      <section className="settings-form-card">
        <div className="chip-row">
          <h2 style={{ margin: 0 }}>Message templates</h2>
          <Badge tone="info">{templates.length} templates</Badge>
        </div>
        <p className="hint">
          Click a helper chip to insert it into the message. Empty save / Reset restores the built-in default. Use{" "}
          <code>{"{{helper}}"}</code> placeholders — they are replaced when the SMS is sent. Customized templates keep
          your wording until you insert new helpers (e.g. <code>{"{{recommendation_line}}"}</code> on Estimate ready) or
          reset to default.
        </p>

        <div className="template-list">
          {templates.map((tpl) => (
            <div key={tpl.key} className="template-card">
              <div className="template-card-head">
                <div>
                  <strong>{tpl.label}</strong>
                  <p className="hint">
                    {tpl.description} · to {tpl.audience}
                  </p>
                </div>
                <Badge tone={tpl.is_customized ? "success" : "pending"}>
                  {tpl.is_customized ? "custom" : "default"}
                </Badge>
              </div>

              <div className="chip-row">
                {tpl.helpers.map((h) => (
                  <button
                    key={h}
                    type="button"
                    className="chip"
                    disabled={!isOwner}
                    onClick={() => insertHelper(tpl.key, h)}
                    title={`Insert {{${h}}}`}
                  >
                    {`{{${h}}}`}
                  </button>
                ))}
              </div>

              <label>
                Message
                <textarea
                  className="input"
                  rows={3}
                  value={drafts[tpl.key] ?? ""}
                  disabled={!isOwner}
                  onChange={(e) => setDrafts((prev) => ({ ...prev, [tpl.key]: e.target.value }))}
                />
              </label>

              <div className="chip-row">
                <Button
                  type="button"
                  disabled={!isOwner || templateBusy === tpl.key}
                  onClick={() => void saveTemplate(tpl.key)}
                >
                  {templateBusy === tpl.key ? "Saving…" : "Save template"}
                </Button>
                <Button
                  type="button"
                  variant="secondary"
                  disabled={!isOwner || templateBusy === tpl.key}
                  onClick={() => void resetTemplate(tpl.key, tpl.default_body)}
                >
                  Reset to default
                </Button>
              </div>
            </div>
          ))}
        </div>
      </section>
    </div>
  );
}
