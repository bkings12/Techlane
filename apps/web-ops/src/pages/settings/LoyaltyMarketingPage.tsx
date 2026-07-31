import { useEffect, useState, type FormEvent } from "react";
import { useAuth } from "../../auth/AuthContext";
import { Badge, Button, Input, PageHeader } from "../../components/ui";
import {
  createWebhook,
  deleteWebhook,
  getLoyaltySettings,
  listWebhooks,
  updateLoyaltySettings,
  type LoyaltySettings,
  type WebhookSubscription,
} from "../../lib/api";
import { useCurrency } from "../../lib/currency";

const EVENT_TYPE_OPTIONS = [
  "repair.completed",
  "repair.status_changed",
  "payment.confirmed",
  "part_request.created",
  "commission.accrued",
];

export function LoyaltyMarketingPage() {
  const { user } = useAuth();
  const { currencyCode } = useCurrency();
  const isOwner = user?.roles?.includes("owner") ?? false;

  const [settings, setSettings] = useState<LoyaltySettings | null>(null);
  const [enabled, setEnabled] = useState(false);
  const [perRepair, setPerRepair] = useState(10);
  const [perCurrency, setPerCurrency] = useState(0);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");
  const [saved, setSaved] = useState("");

  const [webhooks, setWebhooks] = useState<WebhookSubscription[]>([]);
  const [newURL, setNewURL] = useState("");
  const [newEvents, setNewEvents] = useState<string[]>(["repair.completed"]);
  const [webhookBusy, setWebhookBusy] = useState(false);
  const [revealedSecret, setRevealedSecret] = useState<{ id: string; secret: string } | null>(null);

  useEffect(() => {
    getLoyaltySettings()
      .then((s) => {
        setSettings(s);
        setEnabled(s.enabled);
        setPerRepair(s.points_per_completed_repair);
        setPerCurrency(s.points_per_currency_unit);
      })
      .catch((e) => setError(e instanceof Error ? e.message : "Failed to load"));
    listWebhooks()
      .then((r) => setWebhooks(r.items ?? []))
      .catch(() => {});
  }, []);

  async function submit(e: FormEvent) {
    e.preventDefault();
    setBusy(true);
    setError("");
    setSaved("");
    try {
      const next = await updateLoyaltySettings({
        enabled,
        points_per_completed_repair: perRepair,
        points_per_currency_unit: perCurrency,
      });
      setSettings(next);
      setSaved("Loyalty program settings saved.");
    } catch (err) {
      setError(err instanceof Error ? err.message : "Save failed");
    } finally {
      setBusy(false);
    }
  }

  function toggleEventType(type: string) {
    setNewEvents((prev) => (prev.includes(type) ? prev.filter((t) => t !== type) : [...prev, type]));
  }

  async function addWebhook() {
    setWebhookBusy(true);
    setError("");
    try {
      const sub = await createWebhook({ url: newURL.trim(), event_types: newEvents });
      setWebhooks((prev) => [sub, ...prev]);
      setRevealedSecret(sub.secret ? { id: sub.id, secret: sub.secret } : null);
      setNewURL("");
      setNewEvents(["repair.completed"]);
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to register webhook");
    } finally {
      setWebhookBusy(false);
    }
  }

  async function removeWebhook(id: string) {
    if (!confirm("Remove this webhook? Integrations using it will stop receiving events.")) return;
    try {
      await deleteWebhook(id);
      setWebhooks((prev) => prev.filter((w) => w.id !== id));
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to remove webhook");
    }
  }

  if (!settings && !error) return <div className="boot">Loading…</div>;

  return (
    <div className="settings-page">
      <PageHeader
        title="Loyalty & marketing"
        subtitle="Reward repeat customers with points, and push key events to your marketing tools"
      />
      {error ? <p className="form-error">{error}</p> : null}
      {saved ? (
        <p className="form-success">
          <Badge tone="success">saved</Badge> {saved}
        </p>
      ) : null}
      {!isOwner ? (
        <p className="form-error">Viewing as non-owner. Only the shop owner can change these settings.</p>
      ) : null}

      <section className="board-pulse" aria-label="Loyalty pulse">
        <div>
          <strong>Owner</strong>
          <span>Access</span>
        </div>
        <div>
          <strong>{settings?.enabled ? "On" : "Off"}</strong>
          <span>Loyalty program</span>
        </div>
        <div>
          <strong>{settings?.points_per_completed_repair ?? 0}</strong>
          <span>Per repair (pts)</span>
        </div>
        <div>
          <strong>{settings?.points_per_currency_unit ?? 0}</strong>
          <span>Per {currencyCode} (pts)</span>
        </div>
      </section>

      <form className="settings-form-card form-grid" onSubmit={submit}>
        <h2>Points program</h2>
        <p className="hint">
          Customers automatically earn points when a repair is marked completed, and (optionally) as a share of what
          they spend. Balances and history are visible on each customer's profile.
        </p>

        <label className="checkbox-row">
          <input type="checkbox" checked={enabled} onChange={(e) => setEnabled(e.target.checked)} disabled={!isOwner} />
          Enable loyalty points
        </label>

        <label>
          Points per completed repair
          <Input
            type="number"
            min={0}
            value={perRepair}
            onChange={(e) => setPerRepair(Number(e.target.value))}
            disabled={!isOwner}
          />
        </label>

        <label>
          Points per {currencyCode} spent
          <Input
            type="number"
            min={0}
            step="0.01"
            value={perCurrency}
            onChange={(e) => setPerCurrency(Number(e.target.value))}
            disabled={!isOwner}
          />
          <span className="hint">Leave at 0 to only award points for completed repairs, not payment amounts.</span>
        </label>

        <div className="chip-row">
          <Button type="submit" disabled={busy || !isOwner}>
            {busy ? "Saving…" : "Save loyalty settings"}
          </Button>
        </div>
      </form>

      <section className="settings-form-card">
        <div className="chip-row">
          <h2 style={{ margin: 0 }}>Outbound webhooks</h2>
          <Badge tone="info">{webhooks.length} registered</Badge>
        </div>
        <p className="hint">
          Fire a signed HTTP POST to an external URL (Zapier, n8n, your CRM, a marketing tool) whenever a matching
          event happens. Each request includes an <code>X-TechLane-Signature</code> header — an HMAC-SHA256 of the
          body using the webhook's secret — so the receiver can verify authenticity.
        </p>

        {revealedSecret ? (
          <p className="form-success">
            <Badge tone="success">created</Badge> Save this secret now — it will not be shown again:{" "}
            <code className="mono">{revealedSecret.secret}</code>
          </p>
        ) : null}

        <div className="template-card">
          <label>
            Endpoint URL
            <Input
              value={newURL}
              onChange={(e) => setNewURL(e.target.value)}
              placeholder="https://hooks.example.com/techlane"
              disabled={!isOwner}
            />
          </label>
          <p className="hint">Events to send:</p>
          <div className="chip-row">
            {EVENT_TYPE_OPTIONS.map((type) => (
              <button
                key={type}
                type="button"
                className={`chip ${newEvents.includes(type) ? "chip-active" : ""}`}
                disabled={!isOwner}
                onClick={() => toggleEventType(type)}
              >
                {type}
              </button>
            ))}
          </div>
          <div className="chip-row">
            <Button
              type="button"
              disabled={!isOwner || webhookBusy || !newURL.trim() || newEvents.length === 0}
              onClick={() => void addWebhook()}
            >
              {webhookBusy ? "Registering…" : "Register webhook"}
            </Button>
          </div>
        </div>

        {webhooks.length === 0 ? (
          <p className="hint">No webhooks registered yet.</p>
        ) : (
          <ul className="settings-roster">
            {webhooks.map((w) => (
              <li key={w.id}>
                <div className="settings-roster-row">
                  <span>
                    <strong className="mono">{w.url}</strong>
                    <p className="hint">{w.event_types.join(", ")}</p>
                  </span>
                  <span className="hint">
                    {w.last_triggered_at
                      ? `Last: ${new Date(w.last_triggered_at).toLocaleString()} — ${w.last_status ?? "unknown"}`
                      : "No deliveries yet."}
                  </span>
                  <Badge tone={w.is_active ? "success" : "pending"}>{w.is_active ? "active" : "inactive"}</Badge>
                  <Button type="button" variant="secondary" disabled={!isOwner} onClick={() => void removeWebhook(w.id)}>
                    Remove
                  </Button>
                </div>
              </li>
            ))}
          </ul>
        )}
      </section>
    </div>
  );
}
