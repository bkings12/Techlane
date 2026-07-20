import { useCallback, useEffect, useState } from "react";
import { Link } from "react-router-dom";
import { Badge, Button, EmptyState, ICONS, PageHeader, Stat, StatStrip } from "../components/ui";
import {
  ackRiskAlert,
  listRiskAlerts,
  resolveRiskAlert,
  type RiskAlert,
} from "../lib/api";

function alertLink(a: RiskAlert): string | null {
  const repairID = a.details?.repair_job_id;
  if (typeof repairID === "string" && repairID) return `/repairs/${repairID}`;
  if (a.kind === "cash_shortage" || a.kind === "unverified_payment") return "/payments";
  if (a.kind === "unmatched_c2b" || a.kind === "c2b_amount_mismatch") return "/payments";
  if (a.kind === "orphan_part") return "/suppliers";
  return null;
}

function toneFor(kind: string, severity: string): "danger" | "warning" | "pending" | "info" | "success" {
  if (severity === "high" || kind === "cash_shortage" || kind === "unverified_payment") return "danger";
  if (kind === "unmatched_c2b" || kind === "c2b_amount_mismatch") return "danger";
  if (kind === "stuck_job" || kind === "uncollected_ready") return "warning";
  if (kind === "orphan_part") return "pending";
  return "info";
}

export function RiskPage() {
  const [items, setItems] = useState<RiskAlert[]>([]);
  const [filter, setFilter] = useState<"open" | "acknowledged" | "resolved" | "">("open");
  const [error, setError] = useState("");
  const [busy, setBusy] = useState<string | null>(null);

  const refresh = useCallback(async () => {
    const r = await listRiskAlerts(filter || undefined);
    setItems(r.items ?? []);
  }, [filter]);

  useEffect(() => {
    refresh().catch((e) => setError(e instanceof Error ? e.message : "Failed to load"));
  }, [refresh]);

  async function act(id: string, action: "ack" | "resolve") {
    setBusy(id);
    setError("");
    try {
      if (action === "ack") await ackRiskAlert(id);
      else await resolveRiskAlert(id);
      await refresh();
    } catch (e) {
      setError(e instanceof Error ? e.message : "Action failed");
    } finally {
      setBusy(null);
    }
  }

  const byKind = items.reduce<Record<string, number>>((acc, a) => {
    acc[a.kind] = (acc[a.kind] || 0) + 1;
    return acc;
  }, {});

  return (
    <div>
      <PageHeader
        title="Risk & leakage"
        subtitle="Orphan parts, shortages, unverified payments, stuck jobs"
        actions={
          <label className="period-picker">
            Status
            <select className="input" value={filter} onChange={(e) => setFilter(e.target.value as typeof filter)}>
              <option value="open">Open</option>
              <option value="acknowledged">Acknowledged</option>
              <option value="resolved">Resolved</option>
              <option value="">All</option>
            </select>
          </label>
        }
      />
      {error ? <p className="form-error">{error}</p> : null}
      <StatStrip>
        <Stat icon={ICONS.risk} label="Showing" value={items.length} />
        <Stat
          icon={ICONS.cash}
          label="Unverified pay"
          value={byKind.unverified_payment || 0}
          tone={byKind.unverified_payment ? "danger" : undefined}
        />
        <Stat
          icon={ICONS.clock}
          label="Stuck jobs"
          value={(byKind.stuck_job || 0) + (byKind.uncollected_ready || 0)}
          tone={byKind.stuck_job || byKind.uncollected_ready ? "warn" : undefined}
        />
        <Stat
          icon={ICONS.shortage}
          label="Cash / orphans"
          value={(byKind.cash_shortage || 0) + (byKind.orphan_part || 0)}
          tone={byKind.cash_shortage || byKind.orphan_part ? "warn" : undefined}
        />
      </StatStrip>
      <section className="panel">
        {items.length === 0 ? (
          <EmptyState
            title={filter === "open" ? "No open alerts" : "Nothing here"}
            body="Orphan parts, short cash counts, stuck STK, and aging jobs appear when open."
            icon={ICONS.ready}
          />
        ) : (
          <ul className="part-list">
            {items.map((a) => {
              const href = alertLink(a);
              const open = a.status === "open";
              return (
                <li key={a.id} className="part-card">
                  <div className="part-head">
                    <div>
                      <Badge tone={toneFor(a.kind, a.severity)}>{a.kind.replaceAll("_", " ")}</Badge>
                      <div style={{ marginTop: "0.5rem" }}>
                        {href ? (
                          <Link to={href}>
                            <strong>{a.title}</strong>
                          </Link>
                        ) : (
                          <strong>{a.title}</strong>
                        )}
                      </div>
                      <p className="muted">
                        {a.status}
                        {typeof a.details?.job_code === "string" ? ` · ${a.details.job_code}` : ""}
                        {typeof a.details?.age_hours === "number" ? ` · ${a.details.age_hours}h` : ""}
                        {typeof a.details?.age_minutes === "number" ? ` · ${a.details.age_minutes}m` : ""}
                      </p>
                    </div>
                    {open ? (
                      <div className="btn-row">
                        <Button
                          type="button"
                          variant="ghost"
                          disabled={busy === a.id}
                          onClick={() => void act(a.id, "ack")}
                        >
                          Ack
                        </Button>
                        <Button
                          type="button"
                          variant="secondary"
                          disabled={busy === a.id}
                          onClick={() => void act(a.id, "resolve")}
                        >
                          Resolve
                        </Button>
                      </div>
                    ) : null}
                  </div>
                </li>
              );
            })}
          </ul>
        )}
      </section>
    </div>
  );
}

export function PlaceholderPage({ title, body }: { title: string; body: string }) {
  return (
    <div>
      <PageHeader title={title} subtitle={body} />
      <section className="panel">
        <EmptyState title="Coming online" body="Wired to the Go API as each phase lands." />
      </section>
    </div>
  );
}
