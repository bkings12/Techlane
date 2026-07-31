import { useCallback, useEffect, useState, type FormEvent } from "react";
import { useAuth } from "../../auth/AuthContext";
import { Badge, Button, Input, PageHeader } from "../../components/ui";
import {
  disconnectWhatsApp,
  getWhatsAppQR,
  getWhatsAppSettings,
  reconnectWhatsApp,
  updateWhatsAppSettings,
  type WhatsAppQR,
  type WhatsAppSettings,
} from "../../lib/api";

export function WhatsAppSettingsPage() {
  const { user } = useAuth();
  const isOwner = user?.roles?.includes("owner") ?? false;

  const [cfg, setCfg] = useState<WhatsAppSettings | null>(null);
  const [qr, setQr] = useState<WhatsAppQR | null>(null);
  const [error, setError] = useState("");
  const [saved, setSaved] = useState("");
  const [busy, setBusy] = useState(false);
  const [qrBusy, setQrBusy] = useState(false);

  const [enabled, setEnabled] = useState(false);
  const [notifyCustomers, setNotifyCustomers] = useState(true);
  const [notifySuppliers, setNotifySuppliers] = useState(true);
  const [alsoSMS, setAlsoSMS] = useState(false);

  const load = useCallback(() => {
    getWhatsAppSettings()
      .then((c) => {
        setCfg(c);
        setEnabled(c.enabled);
        setNotifyCustomers(c.notify_customers);
        setNotifySuppliers(c.notify_suppliers);
        setAlsoSMS(c.also_send_sms);
      })
      .catch((e) => setError(e instanceof Error ? e.message : "Failed to load"));
  }, []);

  useEffect(() => {
    load();
  }, [load]);

  useEffect(() => {
    if (!cfg?.enabled || cfg.connected || !cfg.service_configured) return;
    let cancelled = false;
    const tick = () => {
      getWhatsAppQR()
        .then((q) => {
          if (!cancelled) setQr(q);
          if (!cancelled && q.status === "connected") load();
        })
        .catch(() => {
          /* keep previous QR */
        });
    };
    tick();
    const id = window.setInterval(tick, 4000);
    return () => {
      cancelled = true;
      window.clearInterval(id);
    };
  }, [cfg?.enabled, cfg?.connected, cfg?.service_configured, load]);

  async function submit(e: FormEvent) {
    e.preventDefault();
    setBusy(true);
    setError("");
    setSaved("");
    try {
      const next = await updateWhatsAppSettings({
        enabled,
        notify_customers: notifyCustomers,
        notify_suppliers: notifySuppliers,
        also_send_sms: alsoSMS,
      });
      setCfg(next);
      setSaved(
        next.enabled
          ? "Saved. Link WhatsApp below if it is not connected yet."
          : "Saved. WhatsApp notifications are off.",
      );
    } catch (err) {
      setError(err instanceof Error ? err.message : "Save failed");
    } finally {
      setBusy(false);
    }
  }

  async function refreshQR() {
    setQrBusy(true);
    setError("");
    try {
      const q = await getWhatsAppQR();
      setQr(q);
      load();
    } catch (err) {
      setError(err instanceof Error ? err.message : "Could not load QR");
    } finally {
      setQrBusy(false);
    }
  }

  async function doReconnect() {
    setQrBusy(true);
    setError("");
    try {
      await reconnectWhatsApp();
      await refreshQR();
      setSaved("Reconnecting — scan the QR with WhatsApp on your phone.");
    } catch (err) {
      setError(err instanceof Error ? err.message : "Reconnect failed");
    } finally {
      setQrBusy(false);
    }
  }

  async function doDisconnect() {
    if (!window.confirm("Disconnect this shop's WhatsApp session?")) return;
    setQrBusy(true);
    setError("");
    try {
      await disconnectWhatsApp();
      setQr(null);
      load();
      setSaved("Disconnected.");
    } catch (err) {
      setError(err instanceof Error ? err.message : "Disconnect failed");
    } finally {
      setQrBusy(false);
    }
  }

  if (!isOwner) {
    return (
      <div>
        <PageHeader title="WhatsApp" subtitle="Owner access required." />
        <p className="muted">Only the shop owner can link WhatsApp.</p>
      </div>
    );
  }

  const statusLabel = !cfg?.service_configured
    ? "Server not configured"
    : cfg.connected
      ? "Connected"
      : cfg.connection_status || "Not linked";

  return (
    <div>
      <PageHeader
        title="WhatsApp"
        subtitle="Send estimates and part requests on WhatsApp. Customers reply YES/NO; suppliers reply QUOTE 2500."
      />

      {error ? <p className="form-error">{error}</p> : null}
      {saved ? <p className="form-success">{saved}</p> : null}

      <form className="settings-form stack-md" onSubmit={submit}>
        <div className="row-between">
          <div>
            <strong>Enable WhatsApp</strong>
            <p className="muted" style={{ margin: "4px 0 0" }}>
              When on (and linked), messages go via WhatsApp instead of SMS unless you also keep SMS.
            </p>
          </div>
          <Badge tone={cfg?.connected ? "success" : cfg?.enabled ? "warning" : "info"}>{statusLabel}</Badge>
        </div>

        <label className="checkbox-row">
          <input type="checkbox" checked={enabled} onChange={(e) => setEnabled(e.target.checked)} />
          Use WhatsApp for notifications
        </label>
        <label className="checkbox-row">
          <input
            type="checkbox"
            checked={notifyCustomers}
            onChange={(e) => setNotifyCustomers(e.target.checked)}
            disabled={!enabled}
          />
          Customers (intake, estimates, ready, payments)
        </label>
        <label className="checkbox-row">
          <input
            type="checkbox"
            checked={notifySuppliers}
            onChange={(e) => setNotifySuppliers(e.target.checked)}
            disabled={!enabled}
          />
          Suppliers (part requests — reply QUOTE amount)
        </label>
        <label className="checkbox-row">
          <input
            type="checkbox"
            checked={alsoSMS}
            onChange={(e) => setAlsoSMS(e.target.checked)}
            disabled={!enabled}
          />
          Also send SMS (duplicate)
        </label>

        <div className="row gap">
          <Button type="submit" disabled={busy}>
            {busy ? "Saving…" : "Save"}
          </Button>
        </div>
      </form>

      <section className="stack-md" style={{ marginTop: 28 }}>
        <h3 style={{ margin: 0 }}>Link phone</h3>
        <p className="muted">
          Works with WhatsApp and WhatsApp Business. On the phone: Settings → Linked devices → Link a
          device, then scan this QR. Keep the phone online after linking.
        </p>
        {!cfg?.service_configured ? (
          <p className="form-error">
            WhatsApp sidecar is not configured on the server (WHATSAPP_SERVICE_URL / SECRET).
          </p>
        ) : (
          <>
            <div className="row gap">
              <Button type="button" variant="secondary" disabled={qrBusy} onClick={refreshQR}>
                {qrBusy ? "Loading…" : "Show QR"}
              </Button>
              <Button type="button" variant="secondary" disabled={qrBusy} onClick={doReconnect}>
                Reconnect
              </Button>
              <Button type="button" variant="danger" disabled={qrBusy || !cfg?.connected} onClick={doDisconnect}>
                Disconnect
              </Button>
            </div>
            {cfg.connected ? (
              <p className="form-success">WhatsApp is linked and ready.</p>
            ) : qr?.qr ? (
              <div className="wa-qr-wrap">
                <img src={qr.qr} alt="WhatsApp QR code" width={280} height={280} />
                <p className="muted">
                  {qr.message || "Scan with WhatsApp or WhatsApp Business → Linked devices"}
                </p>
              </div>
            ) : (
              <p className="muted">Click Show QR to start linking.</p>
            )}
            <label>
              Session id
              <Input value={cfg.session_id || ""} readOnly />
            </label>
          </>
        )}
      </section>

      <section className="stack-md" style={{ marginTop: 28 }}>
        <h3 style={{ margin: 0 }}>How replies work</h3>
        <ul className="muted" style={{ paddingLeft: 18, margin: 0 }}>
          <li>
            <strong>Customer estimate:</strong> reply <code>YES</code> or <code>NO</code>
          </li>
          <li>
            <strong>Supplier part:</strong> reply <code>QUOTE 2500</code> or <code>DECLINE</code>
          </li>
          <li>
            Send <code>HELP</code> anytime for a short reminder
          </li>
        </ul>
      </section>
    </div>
  );
}
