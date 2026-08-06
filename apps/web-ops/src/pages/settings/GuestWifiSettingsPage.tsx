import { useEffect, useState, type FormEvent } from "react";
import { Badge, Button, Input, PageHeader, PasswordInput } from "../../components/ui";
import {
  getWifiSettings,
  updateWifiSettings,
  type WifiSettings,
} from "../../lib/api";

export function GuestWifiSettingsPage() {
  const [cfg, setCfg] = useState<WifiSettings | null>(null);
  const [error, setError] = useState("");
  const [saved, setSaved] = useState("");
  const [busy, setBusy] = useState(false);

  const [enabled, setEnabled] = useState(false);
  const [apiBaseURL, setApiBaseURL] = useState("https://api.bytepesa.co.ke");
  const [apiKey, setApiKey] = useState("");
  const [siteID, setSiteID] = useState("");
  const [packageID, setPackageID] = useState("");
  const [durationMins, setDurationMins] = useState("60");

  useEffect(() => {
    getWifiSettings()
      .then((c) => {
        setCfg(c);
        setEnabled(c.enabled);
        setApiBaseURL(c.api_base_url || "https://api.bytepesa.co.ke");
        setSiteID(c.site_id || "");
        setPackageID(c.package_id || "");
        setDurationMins(String(c.default_duration_mins || 60));
      })
      .catch((e) => setError(e instanceof Error ? e.message : "Failed to load"));
  }, []);

  async function submit(e: FormEvent) {
    e.preventDefault();
    setBusy(true);
    setError("");
    setSaved("");

    if (enabled && !cfg?.api_key_set && !apiKey.trim()) {
      setError("BytePesa partner API key is required the first time you enable Guest WiFi.");
      setBusy(false);
      return;
    }
    if (enabled && !siteID.trim()) {
      setError("BytePesa site ID is required when Guest WiFi is enabled.");
      setBusy(false);
      return;
    }

    const mins = Number(durationMins);
    if (!Number.isFinite(mins) || mins < 1) {
      setError("Default duration must be at least 1 minute.");
      setBusy(false);
      return;
    }

    try {
      const next = await updateWifiSettings({
        enabled,
        api_base_url: apiBaseURL.trim(),
        api_key: apiKey.trim() || undefined,
        site_id: siteID.trim(),
        package_id: packageID.trim(),
        default_duration_mins: mins,
      });
      setCfg(next);
      setApiKey("");
      setSaved(
        next.enabled && next.configured
          ? "Saved. Staff can issue complimentary Guest WiFi from repair jobs."
          : "Saved. Finish enabling and credentials before vouchers will issue.",
      );
    } catch (err) {
      setError(err instanceof Error ? err.message : "Save failed");
    } finally {
      setBusy(false);
    }
  }

  if (!cfg && !error) return <div className="boot">Loading…</div>;

  return (
    <div className="settings-page">
      <PageHeader
        title="Guest WiFi"
        subtitle="Issue complimentary BytePesa hotspot vouchers while customers wait"
      />
      {error ? <p className="form-error">{error}</p> : null}
      {saved ? <p className="hint">{saved}</p> : null}

      <section className="board-pulse" aria-label="Guest WiFi pulse">
        <div className={cfg?.configured ? "" : "warn"}>
          <strong>{cfg?.configured ? "Ready" : "Not set"}</strong>
          <span>BytePesa link</span>
        </div>
        <div className={cfg?.enabled ? "" : "warn"}>
          <strong>{cfg?.enabled ? "On" : "Off"}</strong>
          <span>Enabled</span>
        </div>
        <div>
          <strong>{cfg?.default_duration_mins ?? durationMins}m</strong>
          <span>Default wait</span>
        </div>
      </section>

      <form className="settings-form-card form-grid" onSubmit={submit}>
        <h2>BytePesa partner</h2>
        <p className="hint">
          Create a partner API key in the BytePesa ISP dashboard (Advanced → Guest WiFi keys), enable Guest WiFi on
          the hotspot site, then paste the key here. Duration is the wait-bench time given at issue.
        </p>

        <label className="checkbox-row">
          <input type="checkbox" checked={enabled} onChange={(e) => setEnabled(e.target.checked)} />
          Enable Guest WiFi vouchers
        </label>

        <label>
          API base URL
          <Input
            value={apiBaseURL}
            onChange={(e) => setApiBaseURL(e.target.value)}
            placeholder="https://api.bytepesa.co.ke"
          />
        </label>

        <label>
          Partner API key {cfg?.api_key_set ? <Badge tone="success">saved</Badge> : null}
          <PasswordInput
            value={apiKey}
            onChange={(e) => setApiKey(e.target.value)}
            placeholder={cfg?.api_key_set ? "••••••••" : "bp_live_…"}
            autoComplete="new-password"
          />
        </label>

        <label>
          BytePesa site ID (UUID)
          <Input value={siteID} onChange={(e) => setSiteID(e.target.value)} placeholder="xxxxxxxx-xxxx-…" />
        </label>

        <label>
          Default package ID (optional)
          <Input
            value={packageID}
            onChange={(e) => setPackageID(e.target.value)}
            placeholder="Uses site Guest WiFi package if blank"
          />
        </label>

        <label>
          Default duration (minutes)
          <Input
            type="number"
            min={1}
            value={durationMins}
            onChange={(e) => setDurationMins(e.target.value)}
          />
        </label>

        <div className="btn-row">
          <Button type="submit" disabled={busy}>
            {busy ? "Saving…" : "Save Guest WiFi"}
          </Button>
        </div>
      </form>
    </div>
  );
}
