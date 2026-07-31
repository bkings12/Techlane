import { useEffect, useState, type FormEvent } from "react";
import { useNavigate, useSearchParams } from "react-router-dom";
import { statusLabels } from "../api";
import { PortalTabs } from "../components/PortalTabs";
import { EmptyState } from "../components/EmptyState";
import { deviceLabel, pendingEstimate, pillTone } from "../format";
import { claimRepair } from "../api";
import { useSession } from "../session";

function ClaimRepairForm({
  value,
  onChange,
  onSubmit,
  busy,
  error,
}: {
  value: string;
  onChange: (v: string) => void;
  onSubmit: (e: FormEvent) => void;
  busy: boolean;
  error: string;
}) {
  return (
    <form className="stack" onSubmit={onSubmit}>
      <label>
        Job code
        <input value={value} onChange={(e) => onChange(e.target.value.toUpperCase())} placeholder="JOB-104" required />
      </label>
      {error ? <p className="error">{error}</p> : null}
      <button className="btn" disabled={busy}>
        {busy ? "Claiming…" : "Claim my repair"}
      </button>
    </form>
  );
}

export function RepairsPage() {
  const navigate = useNavigate();
  const { repairs, refreshRepairs } = useSession();
  const [searchParams, setSearchParams] = useSearchParams();
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");
  const [claimJobCode, setClaimJobCode] = useState(() => searchParams.get("claim") ?? "");
  const [showClaimForm, setShowClaimForm] = useState(() => Boolean(searchParams.get("claim")));
  const [claimBusy, setClaimBusy] = useState(false);
  const [claimError, setClaimError] = useState("");

  useEffect(() => {
    setBusy(true);
    setError("");
    refreshRepairs()
      .catch((e) => setError(e instanceof Error ? e.message : "Failed to load"))
      .finally(() => setBusy(false));
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  useEffect(() => {
    if (searchParams.get("claim")) setSearchParams({}, { replace: true });
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  async function onClaimRepair(e: FormEvent) {
    e.preventDefault();
    if (!claimJobCode.trim()) {
      setClaimError("Enter the job code");
      return;
    }
    setClaimBusy(true);
    setClaimError("");
    try {
      await claimRepair(claimJobCode);
      setClaimJobCode("");
      setShowClaimForm(false);
      await refreshRepairs();
    } catch (err) {
      setClaimError(err instanceof Error ? err.message : "Failed to claim repair");
    } finally {
      setClaimBusy(false);
    }
  }

  return (
    <div className="app">
      <main className="shell">
        <div className="topbar">
          <div>
            <h1>My repairs</h1>
            <p className="muted">Approve estimates and pay when ready.</p>
          </div>
        </div>
        <PortalTabs />

        {error ? <p className="error">{error}</p> : null}

        <section className="list">
          {busy && repairs.length === 0 ? (
            <>
              <div className="skeleton" />
              <div className="skeleton" />
            </>
          ) : null}

          {!busy && repairs.length === 0 ? (
            <section className="section">
              <EmptyState
                title="No repairs yet"
                body="When you drop a device at the shop, it will show here."
                icon={
                  <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.8">
                    <rect x="5" y="2" width="14" height="20" rx="2" />
                    <path d="M9 18h6" strokeLinecap="round" />
                  </svg>
                }
              />
              <div className="panel" style={{ marginTop: "1rem" }}>
                <h3>Already dropped a device?</h3>
                <p className="muted" style={{ marginTop: 4 }}>
                  Claim it with the job code from your receipt. It will be linked to this phone number's account.
                </p>
                <ClaimRepairForm
                  value={claimJobCode}
                  onChange={setClaimJobCode}
                  onSubmit={onClaimRepair}
                  busy={claimBusy}
                  error={claimError}
                />
              </div>
            </section>
          ) : null}

          {repairs.map((r) => {
            const est = pendingEstimate(r);
            const due = (r.balance_due ?? r.amount_due ?? 0) > 0;
            return (
              <button
                key={r.id}
                className="repair-card"
                onClick={() => navigate(`/repairs/${r.id}`)}
              >
                <div className="repair-card-head">
                  <strong>{r.job_code}</strong>
                  <span className={`pill ${pillTone(r.status)}`}>
                    {statusLabels[r.status] ?? r.status}
                  </span>
                </div>
                <p>{deviceLabel(r) || "Device under repair"}</p>
                {est?.status === "pending" ? <p className="hint">Estimate awaiting your approval</p> : null}
                {due ? <p className="hint">Payment due</p> : null}
              </button>
            );
          })}

          {repairs.length > 0 ? (
            showClaimForm ? (
              <div className="panel">
                <h3>Claim another repair</h3>
                <p className="muted" style={{ marginTop: 4 }}>
                  Have a job code from a different drop-off? Link it to this account.
                </p>
                <ClaimRepairForm
                  value={claimJobCode}
                  onChange={setClaimJobCode}
                  onSubmit={onClaimRepair}
                  busy={claimBusy}
                  error={claimError}
                />
              </div>
            ) : (
              <button type="button" className="linkish" onClick={() => setShowClaimForm(true)}>
                Claim another repair with a job code
              </button>
            )
          ) : null}
        </section>
      </main>
    </div>
  );
}
