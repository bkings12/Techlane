import { useEffect, useMemo, useState, type ReactNode } from "react";
import { Link } from "react-router-dom";
import { useAuth } from "../auth/AuthContext";
import { Badge, EmptyState, ICONS as SHARED_ICONS, Icon, PageHeader, Stat, StatStrip } from "../components/ui";
import {
  getReportSummary,
  listRepairs,
  listRiskAlerts,
  listSyncCommands,
  type RepairJob,
  type ReportSummary,
  type RiskAlert,
  type SyncCommand,
} from "../lib/api";

const KPI_ICONS = {
  risk: SHARED_ICONS.risk,
  cash: SHARED_ICONS.cash,
  shortage: SHARED_ICONS.shortage,
  repairs: SHARED_ICONS.repairs,
  sales: SHARED_ICONS.reports,
  credit: SHARED_ICONS.package,
  stk: SHARED_ICONS.stk,
  ready: SHARED_ICONS.ready,
};

function fmtKES(n: number) {
  return `KES ${n.toLocaleString()}`;
}

export function HomePage() {
  const { primaryRole } = useAuth();
  if (primaryRole === "technician") return <TechnicianHome />;
  if (primaryRole === "cashier") return <CashierHome />;
  if (primaryRole === "accountant") return <AccountantHome />;
  return <OwnerHome />;
}

function KpiCard({
  label,
  value,
  icon,
  tone,
  to,
  hint,
}: {
  label: string;
  value: ReactNode;
  icon: ReactNode;
  tone?: "danger" | "warn" | "success";
  to: string;
  hint?: string;
}) {
  return (
    <Link className={`kpi-card ${tone ? `kpi-card-${tone}` : ""}`} to={to}>
      <span className="kpi-card-icon">{icon}</span>
      <span className="kpi-card-label">{label}</span>
      <span className="kpi-card-value">{value}</span>
      {hint ? <span className="kpi-card-hint">{hint}</span> : null}
    </Link>
  );
}

function OwnerHome() {
  const [summary, setSummary] = useState<ReportSummary | null>(null);
  const [alerts, setAlerts] = useState<RiskAlert[]>([]);
  const [repairs, setRepairs] = useState<RepairJob[]>([]);
  const [syncNeeds, setSyncNeeds] = useState(0);
  const [error, setError] = useState("");
  const [days, setDays] = useState(7);

  useEffect(() => {
    Promise.all([
      getReportSummary(days).catch(() => null),
      listRiskAlerts("open").catch(() => ({ items: [] as RiskAlert[] })),
      listRepairs().catch(() => ({ items: [] as RepairJob[] })),
      listSyncCommands("needs_attention").catch(() => ({ items: [] as SyncCommand[] })),
    ])
      .then(([s, a, r, sync]) => {
        setSummary(s);
        setAlerts(a.items ?? []);
        setRepairs(r.items ?? []);
        setSyncNeeds((sync.items ?? []).length);
      })
      .catch((e) => setError(e instanceof Error ? e.message : "Failed to load"));
  }, [days]);

  const openRepairs = useMemo(
    () => repairs.filter((r) => !["completed", "collected"].includes(r.status)),
    [repairs],
  );
  const openRisk = summary?.risk_open_total ?? alerts.length;
  const shortages = summary?.shortage_amount_period ?? 0;
  const provisional = summary?.payments_cash_provisional ?? 0;
  const sales = summary?.sales_completed_period ?? 0;
  const credit = summary?.supplier_credit_outstanding ?? 0;
  const stkPending = summary?.payments_stk_pending ?? 0;
  const ready = summary?.repairs_ready ?? 0;

  return (
    <div className="owner-home">
      <PageHeader
        title="Shop health"
        subtitle="Leakage and accountability across your branches"
        actions={
          <div className="period-tabs" role="tablist" aria-label="Period">
            {[1, 7, 30].map((d) => (
              <button
                key={d}
                type="button"
                role="tab"
                aria-selected={days === d}
                className={days === d ? "active" : ""}
                onClick={() => setDays(d)}
              >
                {d === 1 ? "Today" : `${d}d`}
              </button>
            ))}
          </div>
        }
      />
      {error ? <p className="form-error">{error}</p> : null}

      <section className="kpi-grid" aria-label="Shop health">
        <KpiCard
          label="Open risk"
          value={openRisk}
          icon={KPI_ICONS.risk}
          tone={openRisk > 0 ? "danger" : undefined}
          to="/risk"
          hint={openRisk > 0 ? "Needs attention" : "All clear"}
        />
        <KpiCard
          label="Provisional cash"
          value={fmtKES(provisional)}
          icon={KPI_ICONS.cash}
          to="/payments"
          hint="Unreconciled"
        />
        <KpiCard
          label="Shortages"
          value={fmtKES(shortages)}
          icon={KPI_ICONS.shortage}
          tone={shortages > 0 ? "warn" : undefined}
          to="/payments"
          hint={`${days === 1 ? "Today" : `${days}d`} window`}
        />
        <KpiCard
          label="Open repairs"
          value={openRepairs.length || (summary?.repairs_open ?? 0)}
          icon={KPI_ICONS.repairs}
          to="/repairs"
          hint={`${ready} ready for pickup`}
        />
        <KpiCard
          label="Sales"
          value={fmtKES(sales)}
          icon={KPI_ICONS.sales}
          tone="success"
          to="/reports"
          hint={`${summary?.sales_count_period ?? 0} transactions`}
        />
        <KpiCard
          label="Supplier credit"
          value={fmtKES(credit)}
          icon={KPI_ICONS.credit}
          to="/suppliers"
          hint="Outstanding"
        />
        <KpiCard
          label="STK pending"
          value={stkPending}
          icon={KPI_ICONS.stk}
          tone={stkPending > 0 ? "warn" : undefined}
          to="/payments"
          hint="Awaiting confirmation"
        />
        <KpiCard
          label="Sync conflicts"
          value={syncNeeds}
          icon={<Icon d="M21 12a9 9 0 0 0-9-9 9.75 9.75 0 0 0-6.74 2.74L3 8" extra={<path d="M3 3v5h5m-5 4a9 9 0 0 0 9 9 9.75 9.75 0 0 0 6.74-2.74L21 16m0 5v-5h-5" />} />}
          tone={syncNeeds > 0 ? "warn" : undefined}
          to="/sync"
          hint={syncNeeds > 0 ? "Needs attention" : "In sync"}
        />
      </section>

      <div className="owner-columns">
        <section className="panel owner-section owner-section-alerts">
          <div className="panel-head">
            <h2>Needs attention</h2>
            <Link className="panel-link" to="/risk">View all</Link>
          </div>
          {alerts.length === 0 ? (
            <EmptyState title="All clear" body="No open risk alerts right now." />
          ) : (
            <ul className="alert-list">
              {alerts.slice(0, 6).map((a) => (
                <li key={a.id}>
                  <Link className="alert-row" to="/risk">
                    <Badge tone={a.severity === "high" ? "danger" : "warning"}>
                      {a.kind.replaceAll("_", " ")}
                    </Badge>
                    <span className="alert-title">{a.title}</span>
                    <span className="alert-chevron" aria-hidden="true">
                      <Icon d="m9 18 6-6-6-6" size={16} />
                    </span>
                  </Link>
                </li>
              ))}
            </ul>
          )}
        </section>

        <section className="panel owner-section owner-section-jobs">
          <div className="panel-head">
            <h2>Open jobs</h2>
            <Link className="panel-link" to="/repairs">All repairs</Link>
          </div>
          {openRepairs.length === 0 ? (
            <EmptyState title="No open jobs" body="Active repairs will appear here." />
          ) : (
            <ul className="job-list">
              {openRepairs.slice(0, 6).map((j) => (
                <li key={j.id}>
                  <Link className="job-row" to={`/repairs/${j.id}`}>
                    <span className="job-code mono">{j.job_code ?? j.id.slice(0, 8)}</span>
                    <span className="job-problem">{j.problem_summary}</span>
                    <Badge tone={j.status === "ready_for_pickup" ? "success" : "pending"}>
                      {j.status.replaceAll("_", " ")}
                    </Badge>
                  </Link>
                </li>
              ))}
            </ul>
          )}
        </section>
      </div>

      <section className="owner-section">
        <div className="owner-section-head">
          <h2>Quick actions</h2>
        </div>
        <div className="goto-grid">
          <Link className="goto-link" to="/repairs">
            <span className="goto-icon">{KPI_ICONS.repairs}</span>
            <strong>Repairs</strong>
            <span>Intake, assign, estimate</span>
          </Link>
          <Link className="goto-link" to="/pos">
            <span className="goto-icon">{KPI_ICONS.sales}</span>
            <strong>POS</strong>
            <span>New sale</span>
          </Link>
          <Link className="goto-link" to="/payments">
            <span className="goto-icon">{KPI_ICONS.cash}</span>
            <strong>Payments</strong>
            <span>Handovers & cash</span>
          </Link>
          <Link className="goto-link" to="/suppliers">
            <span className="goto-icon">{KPI_ICONS.credit}</span>
            <strong>Suppliers</strong>
            <span>Credit reconciliation</span>
          </Link>
          <Link className="goto-link" to="/reports">
            <span className="goto-icon"><Icon d="M3 3v18h18" extra={<path d="m19 9-5 5-4-4-3 3" />} /></span>
            <strong>Reports</strong>
            <span>Period snapshot</span>
          </Link>
          <Link className="goto-link" to="/risk">
            <span className="goto-icon">{KPI_ICONS.risk}</span>
            <strong>Risk board</strong>
            <span>{openRisk} open alerts</span>
          </Link>
        </div>
      </section>
    </div>
  );
}

function TechnicianHome() {
  const [repairs, setRepairs] = useState<RepairJob[]>([]);
  useEffect(() => {
    listRepairs()
      .then((r) => setRepairs(r.items ?? []))
      .catch(() => setRepairs([]));
  }, []);
  return (
    <div>
      <PageHeader title="My jobs" subtitle="Assigned work and waiting actions" />
      <div className="action-grid" style={{ marginBottom: "1.25rem" }}>
        <Link className="action-tile" to="/repairs">
          <strong>All repairs</strong>
          <span>Status, parts, timeline</span>
        </Link>
      </div>
      <section className="panel">
        {repairs.length === 0 ? (
          <EmptyState title="No jobs yet" body="New assignments will appear here." />
        ) : (
          <ul className="list">
            {repairs.map((j) => (
              <li key={j.id}>
                <Badge tone="pending">{j.status}</Badge>
                <Link to={`/repairs/${j.id}`}>
                  <span className="mono">{j.job_code ?? j.id.slice(0, 8)}</span>
                  {" · "}
                  {j.problem_summary}
                </Link>
              </li>
            ))}
          </ul>
        )}
      </section>
    </div>
  );
}

function CashierHome() {
  return (
    <div>
      <PageHeader title="Cashier" subtitle="Sales, repair payments, and cash drawer" />
      <div className="action-grid">
        <Link className="action-tile" to="/pos">
          <strong>Start sale</strong>
          <span>Open POS workspace</span>
        </Link>
        <Link className="action-tile" to="/repairs">
          <strong>Repair payment</strong>
          <span>Look up a job</span>
        </Link>
        <Link className="action-tile" to="/payments">
          <strong>Cash & payments</strong>
          <span>Drawer and handovers</span>
        </Link>
        <Link className="action-tile" to="/inventory">
          <strong>Stock check</strong>
          <span>Counter balances</span>
        </Link>
      </div>
    </div>
  );
}

function AccountantHome() {
  const [summary, setSummary] = useState<ReportSummary | null>(null);
  useEffect(() => {
    getReportSummary(7)
      .then(setSummary)
      .catch(() => setSummary(null));
  }, []);
  return (
    <div>
      <PageHeader title="Finance" subtitle="Cash accountability and period totals" />
      <StatStrip>
        <Stat icon={SHARED_ICONS.reports} label="Allocated (7d)" value={`KES ${(summary?.payments_allocated_period ?? 0).toLocaleString()}`} />
        <Stat icon={SHARED_ICONS.cash} label="Provisional cash" value={`KES ${(summary?.payments_cash_provisional ?? 0).toLocaleString()}`} />
        <Stat
          icon={SHARED_ICONS.shortage}
          label="Shortages"
          value={`KES ${(summary?.shortage_amount_period ?? 0).toLocaleString()}`}
          tone={(summary?.shortage_amount_period ?? 0) > 0 ? "warn" : undefined}
        />
        <Stat icon={SHARED_ICONS.suppliers} label="Supplier credit" value={`KES ${(summary?.supplier_credit_outstanding ?? 0).toLocaleString()}`} />
      </StatStrip>
      <div className="action-grid">
        <Link className="action-tile" to="/reports">
          <strong>Reports</strong>
          <span>Period snapshot</span>
        </Link>
        <Link className="action-tile" to="/payments">
          <strong>Handovers</strong>
          <span>Confirm and shortages</span>
        </Link>
        <Link className="action-tile" to="/risk">
          <strong>Risk</strong>
          <span>{summary?.risk_open_total ?? 0} open alerts</span>
        </Link>
        <Link className="action-tile" to="/suppliers">
          <strong>Suppliers</strong>
          <span>Credit reconciliation</span>
        </Link>
      </div>
    </div>
  );
}
