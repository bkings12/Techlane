import { useCallback, useEffect, useState } from "react";
import { Link } from "react-router-dom";
import { Badge, EmptyState, ICONS, PageHeader, Stat, StatStrip } from "../components/ui";
import { getOperationsReport, getReportSummary, type OperationsReport, type ReportSummary } from "../lib/api";

export function ReportsPage() {
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
    <div className="reports-page">
      <PageHeader
        title="Reports"
        subtitle="Shop health for the selected window — leakage first"
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

          <StatStrip>
            <Stat
              icon={ICONS.risk}
              label="Open risk"
              value={data.risk_open_total}
              tone={data.risk_open_total > 0 ? "danger" : undefined}
            />
            <Stat
              icon={ICONS.shortage}
              label="Cash shortages"
              value={`KES ${data.shortage_amount_period.toLocaleString()}`}
              tone={data.shortage_amount_period > 0 ? "warn" : undefined}
            />
            <Stat icon={ICONS.cash} label="Provisional cash" value={`KES ${data.payments_cash_provisional.toLocaleString()}`} />
            <Stat icon={ICONS.suppliers} label="Supplier credit" value={`KES ${data.supplier_credit_outstanding.toLocaleString()}`} />
          </StatStrip>

          <div className="repair-grid">
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
              </dl>
              <Link to="/repairs">Open repairs →</Link>
            </section>

            <section className="panel">
              <h2>Money</h2>
              <dl className="meta-dl">
                <dt>Allocated payments</dt>
                <dd>KES {data.payments_allocated_period.toLocaleString()}</dd>
                <dt>STK pending</dt>
                <dd>KES {data.payments_stk_pending.toLocaleString()}</dd>
                <dt>POS sales</dt>
                <dd>
                  KES {data.sales_completed_period.toLocaleString()} ({data.sales_count_period})
                </dd>
                <dt>Open handovers</dt>
                <dd>{data.handovers_open}</dd>
              </dl>
              <Link to="/payments">Payments & cash →</Link>
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
                <p className="muted">
                  Short counts in period: KES {data.shortage_amount_period.toLocaleString()}
                </p>
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
                    KES {data.supplier_credit_outstanding.toLocaleString()}
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
                <h2>Operations — daily</h2>
                {ops.daily.length === 0 ? (
                  <p className="muted">No daily metrics for this period.</p>
                ) : (
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
                            <td>KES {d.payments_allocated.toLocaleString()}</td>
                            <td>KES {d.sales_completed.toLocaleString()}</td>
                            <td>{d.repairs_completed}</td>
                          </tr>
                        ))}
                      </tbody>
                    </table>
                  </div>
                )}
              </section>

              <div className="repair-grid">
                <section className="panel">
                  <h2>By technician</h2>
                  {ops.by_technician.length === 0 ? (
                    <p className="muted">No technician metrics.</p>
                  ) : (
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
                              <td>KES {t.labor_amount_period.toLocaleString()}</td>
                            </tr>
                          ))}
                        </tbody>
                      </table>
                    </div>
                  )}
                </section>

                <section className="panel">
                  <h2>By branch</h2>
                  {ops.by_branch.length === 0 ? (
                    <p className="muted">No branch metrics.</p>
                  ) : (
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
                              <td>KES {b.sales_total_period.toLocaleString()}</td>
                            </tr>
                          ))}
                        </tbody>
                      </table>
                    </div>
                  )}
                </section>
              </div>
            </>
          ) : null}
        </>
      )}
    </div>
  );
}
