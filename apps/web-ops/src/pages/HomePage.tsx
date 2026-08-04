import { useEffect, useMemo, useState, type ReactNode } from "react";
import { Link } from "react-router-dom";
import { useAuth } from "../auth/AuthContext";
import { Badge, EmptyState, ICONS as SHARED_ICONS, Icon, PageHeader, Stat, StatStrip } from "../components/ui";
import { useCurrency } from "../lib/currency";
import {
  getReportSummary,
  listRepairs,
  listRiskAlerts,
  type RepairJob,
  type ReportSummary,
  type RiskAlert,
} from "../lib/api";

const KPI_ICONS = {
  risk: SHARED_ICONS.risk,
  cash: SHARED_ICONS.cash,
  repairs: SHARED_ICONS.repairs,
  sales: SHARED_ICONS.reports,
};

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

export function HomePage() {
  const { primaryRole } = useAuth();
  if (primaryRole === "technician") return <TechnicianHome />;
  if (primaryRole === "cashier") return <CashierHome />;
  if (primaryRole === "accountant") return <AccountantHome />;
  return <OwnerHome />;
}

function OwnerHome() {
  const { formatMoney } = useCurrency();
  const [summary, setSummary] = useState<ReportSummary | null>(null);
  const [alerts, setAlerts] = useState<RiskAlert[]>([]);
  const [repairs, setRepairs] = useState<RepairJob[]>([]);
  const [error, setError] = useState("");
  const [days, setDays] = useState(7);

  useEffect(() => {
    Promise.all([
      getReportSummary(days).catch(() => null),
      listRiskAlerts("open").catch(() => ({ items: [] as RiskAlert[] })),
      listRepairs().catch(() => ({ items: [] as RepairJob[] })),
    ])
      .then(([s, a, r]) => {
        setSummary(s);
        setAlerts(a.items ?? []);
        setRepairs(r.items ?? []);
      })
      .catch((e) => setError(e instanceof Error ? e.message : "Failed to load"));
  }, [days]);

  const openRepairs = useMemo(
    () => repairs.filter((r) => !["completed", "collected", "cancelled", "unrepairable"].includes(r.status)),
    [repairs],
  );
  const openRisk = summary?.risk_open_total ?? alerts.length;
  const sales = summary?.sales_completed_period ?? 0;
  const stkPending = summary?.payments_stk_pending ?? 0;
  const ready = summary?.repairs_ready ?? 0;
  const cashTone: "warn" | "danger" | undefined = stkPending > 0 ? "warn" : undefined;
  const cashHint = `${days === 1 ? "Today" : `${days}d`} window`;

  // TODO: surface supplier_credit_outstanding / sync backlog in Needs attention when
  // listRiskAlerts emits dedicated kinds (none today for supplier credit or sync).

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
          label="Open repairs"
          value={openRepairs.length || (summary?.repairs_open ?? 0)}
          icon={KPI_ICONS.repairs}
          to="/repairs"
          hint={`${ready} ready for pickup`}
        />
        <KpiCard
          label="Sales"
          value={formatMoney(sales)}
          icon={KPI_ICONS.sales}
          tone="success"
          to="/reports"
          hint={`${summary?.sales_count_period ?? 0} transactions`}
        />
        <KpiCard
          label="STK pending"
          value={formatMoney(stkPending)}
          icon={KPI_ICONS.cash}
          tone={cashTone}
          to="/payments"
          hint={cashHint}
        />
      </section>

      {summary ? (
        <StatStrip>
          <Stat label="Today's repair revenue" value={formatMoney(summary.today_repair_revenue)} />
          <Stat label="Today's product revenue" value={formatMoney(summary.today_product_revenue)} />
          <Stat
            label="Today's gross profit"
            value={formatMoney(summary.today_gross_profit)}
            tone={summary.today_gross_profit < 0 ? "danger" : "success"}
          />
        </StatStrip>
      ) : null}

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
  const open = repairs.filter((j) => !["collected", "cancelled", "unrepairable"].includes(j.status));
  const collected = repairs.filter((j) => j.status === "collected").length;
  return (
    <div className="role-home">
      <PageHeader title="My jobs" subtitle="Assigned work and waiting actions — open the board for full detail." />
      <section className="board-pulse" aria-label="Tech pulse">
        <div>
          <strong>{open.length}</strong>
          <span>Open</span>
        </div>
        <div>
          <strong>{collected}</strong>
          <span>Collected</span>
        </div>
        <div>
          <strong>{repairs.length}</strong>
          <span>Loaded</span>
        </div>
      </section>
      <div className="workspace-links">
        <Link to="/repairs/pos">
          <strong>New job</strong>
          <em>Take a device in</em>
        </Link>
        <Link to="/repairs">
          <strong>All repairs</strong>
          <em>Status, parts, timeline</em>
        </Link>
        <Link to="/parts">
          <strong>Part requests</strong>
          <em>Quotes and collection</em>
        </Link>
      </div>
      <section className="panel" style={{ padding: "0.85rem" }}>
        <div className="panel-head">
          <h2>Job list</h2>
        </div>
        {repairs.length === 0 ? (
          <EmptyState title="No jobs yet" body="New assignments will appear here." />
        ) : (
          <ul className="job-board">
            {repairs.map((j) => (
              <li key={j.id}>
                <Link className="job-board-row" to={`/repairs/${j.id}`}>
                  <div className="job-board-id">
                    <strong className="mono">{j.job_code ?? j.id.slice(0, 8)}</strong>
                    <Badge tone="pending">{j.status.replaceAll("_", " ")}</Badge>
                  </div>
                  <div className="job-board-body">
                    <p>{j.problem_summary}</p>
                  </div>
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
    <div className="role-home">
      <PageHeader title="Cashier" subtitle="Sales, repair payments, and cash drawer — pick a workspace." />
      <div className="workspace-links">
        <Link to="/pos">
          <strong>Start sale</strong>
          <em>Open POS workspace</em>
        </Link>
        <Link to="/repairs/pos">
          <strong>New repair</strong>
          <em>Take a device in</em>
        </Link>
        <Link to="/repairs">
          <strong>Repair payment</strong>
          <em>Look up a job</em>
        </Link>
        <Link to="/payments">
          <strong>Cash & payments</strong>
          <em>Ledger and refunds</em>
        </Link>
        <Link to="/inventory">
          <strong>Stock check</strong>
          <em>Counter balances</em>
        </Link>
        <Link to="/orders">
          <strong>Online pickup</strong>
          <em>Collect with code</em>
        </Link>
      </div>
    </div>
  );
}

function AccountantHome() {
  const { formatMoney } = useCurrency();
  const [summary, setSummary] = useState<ReportSummary | null>(null);
  useEffect(() => {
    getReportSummary(7)
      .then(setSummary)
      .catch(() => setSummary(null));
  }, []);
  return (
    <div className="role-home">
      <PageHeader title="Finance" subtitle="Cash accountability and period totals — leakage first." />
      <section className="leakage-strip" aria-label="Finance pulse">
        <div className="leakage-tile">
          <span>Allocated (7d)</span>
          <strong>{formatMoney(summary?.payments_allocated_period ?? 0)}</strong>
        </div>
        <div className={`leakage-tile ${(summary?.payments_stk_pending ?? 0) > 0 ? "warn" : ""}`}>
          <span>STK pending</span>
          <strong>{formatMoney(summary?.payments_stk_pending ?? 0)}</strong>
        </div>
        <div className="leakage-tile">
          <span>Supplier credit</span>
          <strong>{formatMoney(summary?.supplier_credit_outstanding ?? 0)}</strong>
        </div>
      </section>
      <div className="workspace-links">
        <Link to="/reports">
          <strong>Reports</strong>
          <em>Period snapshot</em>
        </Link>
        <Link to="/payments">
          <strong>Payments</strong>
          <em>Ledger and refunds</em>
        </Link>
        <Link to="/risk">
          <strong>Risk</strong>
          <em>{summary?.risk_open_total ?? 0} open alerts</em>
        </Link>
        <Link to="/suppliers">
          <strong>Suppliers</strong>
          <em>Credit reconciliation</em>
        </Link>
      </div>
    </div>
  );
}
