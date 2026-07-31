import { useState, type FormEvent } from "react";
import { useNavigate } from "react-router-dom";
import { guestLookup, statusLabels, type GuestRepair } from "../api";
import { EmptyState } from "../components/EmptyState";
import { deviceLabel, pillTone } from "../format";
import { useSession } from "../session";

export function GuestPage() {
  const { token } = useSession();
  const navigate = useNavigate();
  const [guest, setGuest] = useState<GuestRepair | null>(null);
  const [jobCode, setJobCode] = useState("");
  const [guestPhone, setGuestPhone] = useState("");
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");

  async function onGuestLookup(e: FormEvent) {
    e.preventDefault();
    setBusy(true);
    setError("");
    try {
      setGuest(await guestLookup(jobCode, guestPhone));
    } catch (err) {
      setGuest(null);
      setError(err instanceof Error ? err.message : "Lookup failed");
    } finally {
      setBusy(false);
    }
  }

  return (
    <div className="app">
      <main className="shell">
        <div className="topbar">
          <div>
            <h1>Guest lookup</h1>
            <p className="muted">Read-only status with job code and phone.</p>
          </div>
          <button className="btn-ghost" onClick={() => navigate(token ? "/" : "/login")}>
            Back
          </button>
        </div>
        <section className="panel">
          <form className="stack" onSubmit={onGuestLookup}>
            <label>
              Job code
              <input
                value={jobCode}
                onChange={(e) => setJobCode(e.target.value.toUpperCase())}
                placeholder="JOB-104"
                required
              />
            </label>
            <label>
              Phone
              <input
                inputMode="tel"
                value={guestPhone}
                onChange={(e) => setGuestPhone(e.target.value)}
                required
              />
            </label>
            {error ? <p className="error">{error}</p> : null}
            <button className="btn" disabled={busy}>
              {busy ? "Checking…" : "Check status"}
            </button>
          </form>
          {!token ? (
            <p className="hint" style={{ marginTop: "1rem" }}>
              This is read-only. To approve estimates or pay online, sign in with your phone number and
              claim this job code from your account.{" "}
              <button type="button" className="linkish" onClick={() => navigate("/login")}>
                Sign in
              </button>
            </p>
          ) : null}
        </section>
        {guest ? (
          <section className="section">
            <div className="repair-card-head">
              <div>
                <p className="muted">{guest.job_code}</p>
                <h2 style={{ margin: "0.25rem 0 0" }}>{deviceLabel(guest) || "Device under repair"}</h2>
              </div>
              <span className={`pill ${pillTone(guest.status)}`}>
                {statusLabels[guest.status] ?? guest.status}
              </span>
            </div>
            <p className="muted" style={{ marginTop: "0.75rem" }}>{guest.problem_summary}</p>
            {token ? (
              <div className="panel" style={{ marginTop: "1rem" }}>
                <p className="muted" style={{ margin: 0 }}>
                  Want to approve estimates or pay online for this repair?
                </p>
                <button
                  className="btn btn-secondary"
                  style={{ marginTop: "0.75rem" }}
                  type="button"
                  onClick={() => navigate(`/?claim=${encodeURIComponent(guest.job_code)}`)}
                >
                  Claim this repair to my account
                </button>
              </div>
            ) : null}
            <h3 style={{ marginTop: "1.25rem" }}>Progress</h3>
            {(guest.timeline ?? []).length === 0 ? (
              <EmptyState title="No updates yet" body="Status will appear as the shop works on your device." />
            ) : (
              <ol className="timeline">
                {(guest.timeline ?? []).map((ev, i) => (
                  <li key={`${ev.status}-${ev.at ?? ev.created_at}-${i}`}>
                    <strong>{statusLabels[ev.status ?? ""] ?? ev.status ?? ev.event_type}</strong>
                    <time>{ev.at || ev.created_at ? new Date(ev.at || ev.created_at || "").toLocaleString() : ""}</time>
                    {ev.note ? <p>{ev.note}</p> : null}
                  </li>
                ))}
              </ol>
            )}
          </section>
        ) : null}
      </main>
    </div>
  );
}
