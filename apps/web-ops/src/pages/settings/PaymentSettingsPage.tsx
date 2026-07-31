import { useEffect, useState, type FormEvent } from "react";
import { Link } from "react-router-dom";
import { Button, Input, PageHeader, PasswordInput } from "../../components/ui";
import {
  getPaymentSettings,
  updatePaymentSettings,
  type PaymentProviderSettings,
} from "../../lib/api";

export function PaymentSettingsPage() {
  const [cfg, setCfg] = useState<PaymentProviderSettings | null>(null);
  const [error, setError] = useState("");
  const [saved, setSaved] = useState("");
  const [busy, setBusy] = useState(false);

  const [environment, setEnvironment] = useState("sandbox");
  const [mpesaEnabled, setMpesaEnabled] = useState(false);
  const [shortcode, setShortcode] = useState("");
  const [consumerKey, setConsumerKey] = useState("");
  const [consumerSecret, setConsumerSecret] = useState("");
  const [passkey, setPasskey] = useState("");
  const [callbackURL, setCallbackURL] = useState("");
  const [bankEnabled, setBankEnabled] = useState(false);
  const [bankPaybill, setBankPaybill] = useState("");
  const [bankAccount, setBankAccount] = useState("");

  useEffect(() => {
    getPaymentSettings()
      .then((c) => {
        setCfg(c);
        setEnvironment(c.environment || "sandbox");
        setMpesaEnabled(c.mpesa_enabled);
        setShortcode(c.mpesa_shortcode || "");
        setConsumerKey(c.mpesa_consumer_key || "");
        setCallbackURL(c.mpesa_callback_url || "");
        setBankEnabled(c.bank_enabled);
        setBankPaybill(c.bank_paybill || "");
        setBankAccount(c.bank_account || "");
      })
      .catch((e) => setError(e instanceof Error ? e.message : "Failed to load"));
  }, []);

  async function submit(e: FormEvent) {
    e.preventDefault();
    setBusy(true);
    setError("");
    setSaved("");
    try {
      const next = await updatePaymentSettings({
        environment,
        mpesa_enabled: mpesaEnabled,
        mpesa_shortcode: shortcode,
        mpesa_consumer_key: consumerKey,
        mpesa_consumer_secret: consumerSecret || undefined,
        mpesa_passkey: passkey || undefined,
        mpesa_callback_url: callbackURL,
        bank_enabled: bankEnabled,
        bank_paybill: bankPaybill,
        bank_account: bankAccount,
      });
      setCfg(next);
      setConsumerSecret("");
      setPasskey("");
      setSaved("Payment credentials saved.");
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
        title="Payment service"
        subtitle="M-Pesa Daraja credentials — bank paybill reuses the same API keys"
        actions={
          <Link to="/payments" className="muted">
            Payments desk
          </Link>
        }
      />
      {error ? <p className="form-error">{error}</p> : null}
      {saved ? <p className="hint">{saved}</p> : null}

      <section className="board-pulse" aria-label="Payment pulse">
        <div className={cfg?.configured ? "" : "warn"}>
          <strong>{cfg?.configured ? "Ready" : "Not set"}</strong>
          <span>M-Pesa</span>
        </div>
        <div className={cfg?.bank_configured ? "" : "warn"}>
          <strong>{cfg?.bank_configured ? "Ready" : "Off"}</strong>
          <span>Bank paybill</span>
        </div>
        <div>
          <strong>{cfg?.environment ?? environment}</strong>
          <span>Environment</span>
        </div>
      </section>

      <form className="settings-form-card form-grid" onSubmit={submit}>
        <h2>M-Pesa (Daraja)</h2>
        <p className="hint">
          Consumer key, secret, and passkey power both Lipa Na M-Pesa and bank paybill collection. Secrets are never
          shown after save — leave blank to keep the current value.
        </p>

        <label className="checkbox-row">
          <input type="checkbox" checked={mpesaEnabled} onChange={(e) => setMpesaEnabled(e.target.checked)} />
          Enable M-Pesa payments
        </label>

        <label>
          Environment
          <select className="input" value={environment} onChange={(e) => setEnvironment(e.target.value)}>
            <option value="sandbox">Sandbox</option>
            <option value="production">Production</option>
          </select>
        </label>

        <label>
          Business shortcode (Till / Paybill)
          <Input value={shortcode} onChange={(e) => setShortcode(e.target.value)} placeholder="174379" />
        </label>

        <label>
          Consumer key
          <Input value={consumerKey} onChange={(e) => setConsumerKey(e.target.value)} autoComplete="off" />
        </label>

        <label>
          Consumer secret {cfg?.consumer_secret_set ? <span className="muted">(saved)</span> : null}
          <PasswordInput
            value={consumerSecret}
            onChange={(e) => setConsumerSecret(e.target.value)}
            placeholder={cfg?.consumer_secret_set ? "••••••••" : ""}
            autoComplete="new-password"
          />
        </label>

        <label>
          Passkey {cfg?.passkey_set ? <span className="muted">(saved)</span> : null}
          <PasswordInput
            value={passkey}
            onChange={(e) => setPasskey(e.target.value)}
            placeholder={cfg?.passkey_set ? "••••••••" : ""}
            autoComplete="new-password"
          />
        </label>

        <label>
          Callback URL (optional)
          <Input
            value={callbackURL}
            onChange={(e) => setCallbackURL(e.target.value)}
            placeholder="https://api.example.com/api/v1/webhooks/mpesa"
          />
        </label>

        <h2>Bank paybill</h2>
        <p className="hint">
          Uses the M-Pesa credentials above. Enter only the bank paybill number and the account number customers should
          use as the reference.
        </p>

        <label className="checkbox-row">
          <input
            type="checkbox"
            checked={bankEnabled}
            onChange={(e) => setBankEnabled(e.target.checked)}
            disabled={!mpesaEnabled}
          />
          Enable bank paybill (same Daraja credentials)
        </label>

        <label>
          Bank paybill
          <Input
            value={bankPaybill}
            onChange={(e) => setBankPaybill(e.target.value)}
            placeholder="Business paybill"
            disabled={!bankEnabled}
          />
        </label>

        <label>
          Account number
          <Input
            value={bankAccount}
            onChange={(e) => setBankAccount(e.target.value)}
            placeholder="Account / till reference"
            disabled={!bankEnabled}
          />
        </label>

        <div className="chip-row">
          <Button type="submit" disabled={busy}>
            Save payment settings
          </Button>
        </div>
      </form>
    </div>
  );
}
