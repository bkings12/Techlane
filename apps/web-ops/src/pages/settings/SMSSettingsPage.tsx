import { useEffect, useState, type FormEvent } from "react";
import { Link } from "react-router-dom";
import { useAuth } from "../../auth/AuthContext";
import { Badge, Button, ICONS, Input, PageHeader, Stat, StatStrip } from "../../components/ui";
import { getSMSSettings, updateSMSSettings, type SMSSettings } from "../../lib/api";

export function SMSSettingsPage() {
  const { user } = useAuth();
  const isOwner = user?.roles?.includes("owner") ?? false;

  const [cfg, setCfg] = useState<SMSSettings | null>(null);
  const [error, setError] = useState("");
  const [saved, setSaved] = useState("");
  const [busy, setBusy] = useState(false);

  const [enabled, setEnabled] = useState(false);
  const [senderID, setSenderID] = useState("");
  const [apiKey, setApiKey] = useState("");
  const [baseURL, setBaseURL] = useState("https://sms.blessedtexts.com/api/sms/v1");

  useEffect(() => {
    getSMSSettings()
      .then((c) => {
        setCfg(c);
        setEnabled(c.enabled);
        setSenderID(c.sender_id || "");
        setBaseURL(c.base_url || "https://sms.blessedtexts.com/api/sms/v1");
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
          ? "Saved. Customer OTP is ready to send via BlessedTexts."
          : "Saved. Enable SMS and finish configuration before customer OTP will work.",
      );
    } catch (err) {
      setError(err instanceof Error ? err.message : "Save failed");
    } finally {
      setBusy(false);
    }
  }

  if (!cfg && !error) return <div className="boot">Loading…</div>;

  const ready = Boolean(cfg?.configured && cfg?.enabled);

  return (
    <div>
      <PageHeader
        title="SMS (OTP)"
        subtitle="BlessedTexts delivery for customer repair verification codes"
        actions={
          <Link to="/settings" className="muted">
            All settings
          </Link>
        }
      />
      {error ? <p className="form-error">{error}</p> : null}
      {saved ? (
        <p className="form-success">
          <Badge tone="success">saved</Badge> {saved}
        </p>
      ) : null}

      {!isOwner ? (
        <p className="hint panel" style={{ marginBottom: "1rem" }}>
          Viewing as non-owner. Only the shop owner can change SMS credentials; the API will reject saves from other
          roles.
        </p>
      ) : null}

      <StatStrip>
        <Stat icon={ICONS.settings} label="Access" value={<Badge tone="info">owner only</Badge>} />
        <Stat icon={ICONS.stk} label="Delivery" value={<Badge tone={ready ? "success" : "warning"}>{ready ? "ready" : "not ready"}</Badge>} />
        <Stat icon={ICONS.ready} label="Enabled" value={<Badge tone={cfg?.enabled ? "success" : "pending"}>{cfg?.enabled ? "on" : "off"}</Badge>} />
        <Stat icon={ICONS.hash} label="Sender ID" value={<span className="mono">{cfg?.sender_id || "—"}</span>} />
      </StatStrip>

      <form className="panel form-grid" onSubmit={submit}>
        <div className="panel-head">
          <h2>BlessedTexts</h2>
          <Badge tone={cfg?.api_key_set ? "success" : "warning"}>
            {cfg?.api_key_set ? "key on file" : "key missing"}
          </Badge>
        </div>
        <p className="hint">
          Customer portal and Android OTP require this to be enabled and configured. The API key is never shown after
          save — leave the field blank to keep the current value. Verification codes are never written to server logs.
        </p>

        <label className="checkbox-row">
          <input type="checkbox" checked={enabled} onChange={(e) => setEnabled(e.target.checked)} />
          Enable BlessedTexts SMS for customer OTP
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
          <Input
            type="password"
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
    </div>
  );
}
