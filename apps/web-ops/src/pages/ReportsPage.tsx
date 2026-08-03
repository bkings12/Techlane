import { useCallback, useEffect, useState } from "react";
import { Link } from "react-router-dom";
import { BarChart, LineChart } from "../components/Charts";
import { Badge, Button, EmptyState, PageHeader, Stat, StatStrip } from "../components/ui";
import { downloadCSV } from "../lib/csv";
import { useCurrency } from "../lib/currency";
import { getOperationsReport, getReportSummary, type OperationsReport, type ReportSummary } from "../lib/api";
import { CLOSURE_REASON_LABELS } from "../lib/repairStatus";

function shortDate(iso: string) {
  const d = new Date(iso);
  return d.toLocaleDateString(undefined, { month: "short", day: "numeric" });
}

export function ReportsPage() {
  const { formatMoney, currencyCode } = useCurrency();
  const [days, setDays] = useState(7);
  const [data, setData] = useState<ReportSummary | null>(null);
  const [ops, setOps] = useState<OperationsReport | null>(null);
  const [error, setError] = useState("");

  const refresh = useCallback(async () => {
    const [s, o] = await Promise.all([getReportSummary(days), getOperationsReport(days)]);
    setData(s);
    setOps(o);
  }, [days]);

  useEffect(() => {
    refresh().catch((e) => setError(e instanceof Error ? e.message : "Failed to load"));
  }, [refresh]);

  return (
    <div className="reports-briefing">
      <PageHeader
        title="Reports"
        subtitle="Leakage first — then repairs, money, and ops charts."
        actions={
          <label className="period-picker">
            Period
            <select className="input" value={days} onChange={(e) => setDays(Number(e.target.value))}>
              <option value={1}>Today / 1 day</option>
              <option value={7}>7 days</option>
              <option value={30}>30 days</option>
            </select>
          </label>
        }
      />
      {error ? <p className="form-error">{error}</p> : null}
      {!data ? (
        <EmptyState title="Loading…" body="Pulling live totals from repairs, payments, and risk." />
      ) : (
        <>
          <p className="muted">
            Snapshot {new Date(data.generated_at).toLocaleString()} · last {data.period_days} day
            {data.period_days === 1 ? "" : "s"}
          </p>

          <section className="leakage-strip" aria-label="Leakage signals">
            <div className={`leakage-tile ${data.risk_open_total > 0 ? "danger" : ""}`}>
              <span>Open risk</span>
              <strong>{data.risk_open_total}</strong>
            </div>
            <div className={`leakage-tile ${data.payments_stk_pending > 0 ? "warn" : ""}`}>
              <span>STK pending</span>
              <strong>{formatMoney(data.payments_stk_pending)}</strong>
            </div>
            <div className="leakage-tile">
              <span>Supplier credit</span>
              <strong>{formatMoney(data.supplier_credit_outstanding)}</strong>
            </div>
          </section>

          <div className="briefing-grid">
            <section className="panel">
              <h2>Repairs</h2>
              <dl className="meta-dl">
                <dt>In progress</dt>
                <dd>{data.repairs_open}</dd>
                <dt>Ready for collection</dt>
                <dd>{data.repairs_ready}</dd>
                <dt>Waiting parts</dt>
                <dd>{data.repairs_waiting_parts}</dd>
                <dt>Collected (period)</dt>
                <dd>{data.repairs_completed_period}</dd>
                <dt>Closed without repair (period)</dt>
                <dd>{data.repairs_closed_period}</dd>
              </dl>
              <Link to="/repairs">Open repairs →</Link>
            </section>

            <section className="panel">
              <h2>Money</h2>
              <dl className="meta-dl">
                <dt>Allocated payments</dt>
                <dd>{formatMoney(data.payments_allocated_period)}</dd>
                <dt>STK pending</dt>
                <dd>{formatMoney(data.payments_stk_pending)}</dd>
                <dt>POS sales</dt>
                <dd>
                  {formatMoney(data.sales_completed_period)} ({data.sales_count_period})
                </dd>
              </dl>
              <Link to="/payments">Payments →</Link>
            </section>
          </div>

          <section className="panel">
            <div className="panel-head">
              <h2>Leakage signals</h2>
              <Link to="/risk">Risk board →</Link>
            </div>
            <ul className="part-list">
              <li className="part-card">
                <div className="part-head">
                  <strong>Orphan parts</strong>
                  <Badge tone={data.risk_orphan_parts > 0 ? "warning" : "success"}>
                    {data.risk_orphan_parts} open
                  </Badge>
                </div>
                <p className="muted">Parts without a matching settled job context.</p>
              </li>
              <li className="part-card">
                <div className="part-head">
                  <strong>Cash shortage alerts</strong>
                  <Badge tone={data.risk_cash_shortage > 0 ? "danger" : "success"}>
                    {data.risk_cash_shortage} open
                  </Badge>
                </div>
                <p className="muted">Historical till-count shortage alerts still open on the risk board.</p>
              </li>
              <li className="part-card">
                <div className="part-head">
                  <strong>Unverified payments</strong>
                  <Badge tone={data.risk_unverified_payment > 0 ? "danger" : "success"}>
                    {data.risk_unverified_payment} open
                  </Badge>
                </div>
                <p className="muted">STK / bank pay still pending confirmation.</p>
              </li>
              <li className="part-card">
                <div className="part-head">
                  <strong>Stuck / uncollected jobs</strong>
                  <Badge tone={data.risk_stuck_jobs > 0 ? "warning" : "success"}>
                    {data.risk_stuck_jobs} open
                  </Badge>
                </div>
                <p className="muted">Aging repairs and devices waiting for pickup.</p>
              </li>
              <li className="part-card">
                <div className="part-head">
                  <strong>Supplier credit outstanding</strong>
                  <Badge tone={data.supplier_credit_outstanding > 0 ? "pending" : "success"}>
                    {formatMoney(data.supplier_credit_outstanding)}
                  </Badge>
                </div>
                <p className="muted">
                  <Link to="/suppliers">Reconcile suppliers</Link>
                </p>
              </li>
            </ul>
          </section>

          {ops ? (
            <>
              <section className="panel">
                <div className="panel-head">
                  <h2>Trend — payments, sales & repairs completed</h2>
                  <Button
                    type="button"
                    variant="secondary"
                    disabled={ops.daily.length === 0}
                    onClick={() =>
                      downloadCSV("techlane-daily-trend", [
                        ["Date", `Payments allocated (${currencyCode})`, `Sales completed (${currencyCode})`, "Repairs completed"],
                        ...ops.daily.map((d) => [d.date, d.payments_allocated, d.sales_completed, d.repairs_completed]),
                      ])
                    }
                  >
                    Export CSV
                  </Button>
                </div>
                {ops.daily.length === 0 ? (
                  <p className="muted">No daily metrics for this period.</p>
                ) : (
                  <>
                    <LineChart
                      labels={ops.daily.map((d) => shortDate(d.date))}
                      series={[
                        { label: `Payments (${currencyCode})`, color: "#063086", values: ops.daily.map((d) => d.payments_allocated) },
                        { label: `Sales (${currencyCode})`, color: "#f2be2a", values: ops.daily.map((d) => d.sales_completed) },
                      ]}
                      formatValue={(v) => formatMoney(v)}
                    />
                    <div className="table-wrap">
                      <table className="data-table">
                        <thead>
                          <tr>
                            <th>Date</th>
                            <th>Payments</th>
                            <th>Sales</th>
                            <th>Repairs done</th>
                          </tr>
                        </thead>
                        <tbody>
                          {ops.daily.map((d) => (
                            <tr key={d.date}>
                              <td>{d.date}</td>
                              <td>{formatMoney(d.payments_allocated)}</td>
                              <td>{formatMoney(d.sales_completed)}</td>
                              <td>{d.repairs_completed}</td>
                            </tr>
                          ))}
                        </tbody>
                      </table>
                    </div>
                  </>
                )}
              </section>

              <div className="repair-grid">
                <section className="panel">
                  <div className="panel-head">
                    <h2>By technician</h2>
                    <Button
                      type="button"
                      variant="secondary"
                      disabled={ops.by_technician.length === 0}
                      onClick={() =>
                        downloadCSV("techlane-by-technician", [
                          ["Name", "Open jobs", "Completed (period)", `Labor amount (${currencyCode})`],
                          ...ops.by_technician.map((t) => [t.name, t.open_jobs, t.completed_period, t.labor_amount_period]),
                        ])
                      }
                    >
                      Export CSV
                    </Button>
                  </div>
                  {ops.by_technician.length === 0 ? (
                    <p className="muted">No technician metrics.</p>
                  ) : (
                    <>
                      <BarChart
                        items={ops.by_technician.map((t) => ({ label: t.name.split(" ")[0], value: t.completed_period }))}
                        height={140}
                      />
                      <div className="table-wrap">
                        <table className="data-table">
                          <thead>
                            <tr>
                              <th>Name</th>
                              <th>Open</th>
                              <th>Completed</th>
                              <th>Labor</th>
                            </tr>
                          </thead>
                          <tbody>
                            {ops.by_technician.map((t) => (
                              <tr key={t.technician_id}>
                                <td>{t.name}</td>
                                <td>{t.open_jobs}</td>
                                <td>{t.completed_period}</td>
                                <td>{formatMoney(t.labor_amount_period)}</td>
                              </tr>
                            ))}
                          </tbody>
                        </table>
                      </div>
                    </>
                  )}
                </section>

                <section className="panel">
                  <div className="panel-head">
                    <h2>By branch</h2>
                    <Button
                      type="button"
                      variant="secondary"
                      disabled={ops.by_branch.length === 0}
                      onClick={() =>
                        downloadCSV("techlane-by-branch", [
                          ["Branch", "Open jobs", "Completed (period)", `Sales total (${currencyCode})`],
                          ...ops.by_branch.map((b) => [b.name, b.open_jobs, b.completed_period, b.sales_total_period]),
                        ])
                      }
                    >
                      Export CSV
                    </Button>
                  </div>
                  {ops.by_branch.length === 0 ? (
                    <p className="muted">No branch metrics.</p>
                  ) : (
                    <>
                      <BarChart
                        items={ops.by_branch.map((b) => ({ label: b.name, value: b.sales_total_period, color: "#f2be2a" }))}
                        height={140}
                        formatValue={(v) => formatMoney(v)}
                      />
                      <div className="table-wrap">
                        <table className="data-table">
                          <thead>
                            <tr>
                              <th>Branch</th>
                              <th>Open</th>
                              <th>Completed</th>
                              <th>Sales</th>
                            </tr>
                          </thead>
                          <tbody>
                            {ops.by_branch.map((b) => (
                              <tr key={b.branch_id}>
                                <td>{b.name}</td>
                                <td>{b.open_jobs}</td>
                                <td>{b.completed_period}</td>
                                <td>{formatMoney(b.sales_total_period)}</td>
                              </tr>
                            ))}
                          </tbody>
                        </table>
                      </div>
                    </>
                  )}
                </section>
              </div>

              <section className="panel">
                <h2>Repair profitability</h2>
                <p className="muted">
                  Labor charged against the cost of parts consumed, for the {ops.repair_profitability.jobs} job
                  {ops.repair_profitability.jobs === 1 ? "" : "s"} finished in this period.
                </p>
                <StatStrip>
                  <Stat label="Labor charged" value={formatMoney(ops.repair_profitability.labor_revenue)} />
                  <Stat label="Parts cost" value={formatMoney(ops.repair_profitability.parts_cost)} />
                  <Stat
                    label="Margin"
                    value={formatMoney(ops.repair_profitability.margin)}
                    hint={
                      ops.repair_profitability.margin_pct != null
                        ? `${ops.repair_profitability.margin_pct.toFixed(1)}% of labor charged`
                        : undefined
                    }
                    tone={ops.repair_profitability.margin < 0 ? "danger" : "success"}
                  />
                  <Stat
                    label="Jobs that lost money"
                    value={String(ops.repair_profitability.loss_making_jobs)}
                    tone={ops.repair_profitability.loss_making_jobs > 0 ? "warn" : undefined}
                    hint={
                      ops.repair_profitability.loss_making_jobs > 0
                        ? "Parts cost more than we charged"
                        : "Every job covered its parts"
                    }
                  />
                </StatStrip>
                {ops.repair_profitability.jobs_with_unpriced_parts > 0 ? (
                  <p className="hint warn-text">
                    {ops.repair_profitability.jobs_with_unpriced_parts} of these jobs used a part with no buying
                    price on record, so the margin above is an upper bound. Add cost prices to those stock items
                    under Inventory for a true figure.
                  </p>
                ) : null}
              </section>

              <section className="panel">
                <div className="panel-head">
                  <h2>Why jobs were lost</h2>
                  <Button
                    type="button"
                    variant="secondary"
                    disabled={ops.closures.length === 0}
                    onClick={() =>
                      downloadCSV("techlane-lost-jobs", [
                        ["Outcome", "Reason", "Jobs", `Quoted value written off (${currencyCode})`],
                        ...ops.closures.map((c) => [c.status, c.reason, c.count, c.lost_labor_value]),
                      ])
                    }
                  >
                    Export CSV
                  </Button>
                </div>
                {ops.closures.length === 0 ? (
                  <p className="muted">
                    No jobs were cancelled or written off in this period — every job either finished or is still moving.
                  </p>
                ) : (
                  <div className="table-wrap">
                    <table className="data-table">
                      <thead>
                        <tr>
                          <th>Outcome</th>
                          <th>Reason</th>
                          <th>Jobs</th>
                        </tr>
                      </thead>
                      <tbody>
                        {ops.closures.map((c) => (
                          <tr key={`${c.status}-${c.reason}`}>
                            <td>
                              <Badge tone="danger">{c.status}</Badge>
                            </td>
                            <td>{CLOSURE_REASON_LABELS[c.reason] ?? c.reason.replaceAll("_", " ")}</td>
                            <td>{c.count}</td>
                          </tr>
                        ))}
                      </tbody>
                    </table>
                  </div>
                )}
              </section>
            </>
          ) : null}
        </>
      )}
    </div>
  );
}
