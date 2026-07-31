import { useCallback, useEffect, useState } from "react";
import { Link, useSearchParams } from "react-router-dom";
import { useBranch } from "../branch/BranchContext";
import { useRealtimeEvents } from "../lib/realtime";
import { Badge, Button, EmptyState, ICONS, PageHeader, SearchInput } from "../components/ui";
import { listRepairs, type RepairJob } from "../lib/api";
import { statusLabel, statusTone } from "../lib/repairStatus";
import { useCurrency } from "../lib/currency";

const STAGES: { value: string; label: string; hint: string }[] = [
  { value: "open", label: "Open", hint: "Still in the shop" },
  { value: "waiting_parts", label: "Waiting parts", hint: "Blocked on parts" },
  { value: "in_progress", label: "On bench", hint: "Being worked" },
  { value: "ready", label: "Ready", hint: "QC / complete" },
  { value: "overdue", label: "Overdue", hint: "Past promise" },
  { value: "collected", label: "Collected", hint: "Handed back" },
  { value: "", label: "All", hint: "Every job" },
];

function jobAge(createdAt?: string) {
  if (!createdAt) return "—";
  const days = Math.max(0, Math.floor((Date.now() - new Date(createdAt).getTime()) / 86_400_000));
  return days === 0 ? "Today" : `${days}d`;
}

function isOverdue(job: RepairJob) {
  return Boolean(
    job.promised_by &&
      new Date(job.promised_by).getTime() < Date.now() &&
      !["completed", "collected", "cancelled", "unrepairable", "ready_for_pickup"].includes(job.status),
  );
}

function deviceLabel(job: RepairJob) {
  const brand = job.device?.brand?.trim();
  const model = job.device?.model?.trim();
  if (brand && model) return `${brand} ${model}`;
  if (brand) return brand;
  if (model) return model;
  return "Device";
}

function moneyLabel(job: RepairJob, formatMoney: (n: number) => string) {
  if ((job.balance_due ?? 0) > 0) return `${formatMoney(job.balance_due ?? 0)} due`;
  if ((job.quoted_value ?? 0) > 0) return formatMoney(job.quoted_value ?? 0);
  return "Unpriced";
}

function promiseLabel(job: RepairJob, overdue: boolean) {
  if (job.customer_waiting && job.estimated_wait_minutes) {
    return `~${job.estimated_wait_minutes} min wait`;
  }
  if (job.promised_by) {
    if (overdue) return "Overdue";
    return `Due ${new Date(job.promised_by).toLocaleDateString("en-KE", { day: "numeric", month: "short" })}`;
  }
  return "No promise";
}

export function RepairsPage() {
  const [searchParams] = useSearchParams();
  const { branchId } = useBranch();
  const { formatMoney } = useCurrency();
  const [items, setItems] = useState<RepairJob[]>([]);
  const [allOpen, setAllOpen] = useState<RepairJob[]>([]);
  const [collectedCount, setCollectedCount] = useState(0);
  const [error, setError] = useState("");
  const [q, setQ] = useState("");
  const [status, setStatus] = useState(() => {
    const raw = searchParams.get("status") ?? "open";
    // Home / old links used completed; shop-floor stage is "ready".
    if (raw === "completed" || raw === "ready_for_pickup") return "ready";
    return raw;
  });
  const [myJobs, setMyJobs] = useState(false);

  const refresh = useCallback(async () => {
    try {
      const [filtered, openBoard, collectedBoard] = await Promise.all([
        listRepairs({
          q: q.trim() || undefined,
          status: status || undefined,
          branch_id: branchId || undefined,
          technician_id: myJobs ? "me" : undefined,
        }),
        listRepairs({
          status: "open",
          branch_id: branchId || undefined,
          technician_id: myJobs ? "me" : undefined,
        }),
        listRepairs({
          status: "collected",
          branch_id: branchId || undefined,
          technician_id: myJobs ? "me" : undefined,
        }),
      ]);
      setItems(filtered.items ?? []);
      setAllOpen(openBoard.items ?? []);
      setCollectedCount((collectedBoard.items ?? []).length);
      setError("");
    } catch (e) {
      setError(e instanceof Error ? e.message : "Failed to load");
    }
  }, [q, status, branchId, myJobs]);

  useEffect(() => {
    void refresh();
  }, [refresh]);

  useRealtimeEvents(["repair.*"], () => {
    void refresh();
  });

  const pulse: Record<string, number> = {
    open: allOpen.length,
    waiting_parts: allOpen.filter((j) => j.status === "waiting_parts").length,
    in_progress: allOpen.filter((j) => j.status === "in_progress").length,
    ready: allOpen.filter((j) => j.status === "ready_for_pickup" || j.status === "completed").length,
    overdue: allOpen.filter((j) => isOverdue(j)).length,
    collected: collectedCount,
    "": 0,
  };

  const stageLabel = STAGES.find((s) => s.value === status)?.label ?? "Jobs";

  return (
    <div className="jobs-desk">
      <PageHeader
        title="Job records"
        subtitle="Search and manage repair jobs across branches."
        actions={
          <Link className="btn btn-primary" to="/repairs/pos">
            New job
          </Link>
        }
      />
      {error ? <p className="form-error">{error}</p> : null}

      <section className="jobs-stages" aria-label="Job stages">
        {STAGES.map((stage) => {
          const count = stage.value === "" ? null : pulse[stage.value] ?? 0;
          const active = status === stage.value;
          const warn = stage.value === "overdue" && (pulse.overdue ?? 0) > 0;
          return (
            <button
              key={stage.value || "all"}
              type="button"
              className={`jobs-stage ${active ? "active" : ""} ${warn ? "warn" : ""}`}
              onClick={() => setStatus(stage.value)}
              aria-pressed={active}
            >
              <span className="jobs-stage-label">{stage.label}</span>
              {count !== null ? <strong>{count}</strong> : <strong className="jobs-stage-all">···</strong>}
              <em>{stage.hint}</em>
            </button>
          );
        })}
      </section>

      <section className="jobs-ledger desk-ledger">
        <div className="panel-head jobs-ledger-head">
          <div>
            <h2>{stageLabel}</h2>
            <p className="muted">
              {items.length === 0
                ? "No jobs in this view"
                : `${items.length} job${items.length === 1 ? "" : "s"} · overdue & wait-bench first, then oldest promise`}
            </p>
          </div>
          <div className="jobs-toolbar">
            <SearchInput
              value={q}
              onChange={(e) => setQ(e.target.value)}
              placeholder="Search job, client, device, problem…"
              aria-label="Search jobs"
            />
            <label className="checkbox-row jobs-mine">
              <input type="checkbox" checked={myJobs} onChange={(e) => setMyJobs(e.target.checked)} />
              My jobs
            </label>
          </div>
        </div>

        {items.length === 0 ? (
          <div className="jobs-empty">
            <EmptyState
              title="No jobs here"
              body={
                status === "open"
                  ? "Nothing open on this branch. Take a device in from Job POS."
                  : "Try another stage or clear search."
              }
              icon={ICONS.repairs}
            />
            <div className="btn-row jobs-empty-actions">
              {status !== "open" || q ? (
                <Button
                  type="button"
                  variant="secondary"
                  onClick={() => {
                    setStatus("open");
                    setQ("");
                  }}
                >
                  Show open jobs
                </Button>
              ) : null}
              <Link className="btn btn-primary" to="/repairs/pos">
                New job
              </Link>
            </div>
          </div>
        ) : (
          <ul className="jobs-list" aria-label="Repair jobs">
            {items.map((j) => {
              const overdue = isOverdue(j);
              return (
                <li key={j.id}>
                  <Link className={`jobs-row ${overdue ? "is-overdue" : ""}`} to={`/repairs/${j.id}`}>
                    <div className="jobs-row-id">
                      <strong className="mono">{j.job_code ?? j.id.slice(0, 8)}</strong>
                      <Badge tone={statusTone(j.status)}>{statusLabel(j.status)}</Badge>
                    </div>
                    <div className="jobs-row-body">
                      <div className="jobs-row-customer">
                        <span>{j.customer_name ?? j.customer?.full_name ?? "Walk-in"}</span>
                        {j.customer_waiting ? <Badge tone="pending">wait bench</Badge> : null}
                        {j.parent_job_id ? <Badge tone="warning">return</Badge> : null}
                      </div>
                      <div className="jobs-row-device">{deviceLabel(j)}</div>
                      <p>{j.problem_summary}</p>
                    </div>
                    <div className="jobs-row-meta">
                      <span className={(j.quoted_value ?? 0) > 0 || (j.balance_due ?? 0) > 0 ? "jobs-row-money" : "muted"}>
                        {moneyLabel(j, formatMoney)}
                      </span>
                      <span>{jobAge(j.created_at)}</span>
                      <span className={overdue ? "repair-overdue" : "muted"}>{promiseLabel(j, overdue)}</span>
                    </div>
                  </Link>
                </li>
              );
            })}
          </ul>
        )}
      </section>
    </div>
  );
}
