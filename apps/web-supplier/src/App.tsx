import { useCallback, useEffect, useState, type FormEvent, type ReactNode } from "react";
import {
  acceptInvite,
  clearToken,
  credit,
  decline,
  downloadIssueVoucherPDF,
  getRequest,
  getToken,
  issue,
  listIssues,
  listRequests,
  login,
  logout,
  markReady,
  me,
  openIssueVoucher,
  quote,
  setToken,
  type CreditSummary,
  type Issue,
  type PartRequest,
  type SupplierContact,
} from "./api";
import { QrImage } from "./QrImage";

type Tab = "queue" | "issues" | "credit" | "profile";
type Mode = "login" | "invite";

function titleFor(req: PartRequest) {
  return req.part_name || req.description || "Part request";
}

function EmptyState({ title, body, icon }: { title: string; body: string; icon?: ReactNode }) {
  return (
    <div className="empty-state">
      <div className="empty-icon">
        {icon ?? (
          <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.8">
            <path d="M4 7h16M4 12h10M4 17h14" strokeLinecap="round" />
          </svg>
        )}
      </div>
      <strong>{title}</strong>
      <p>{body}</p>
    </div>
  );
}

function initials(name?: string | null) {
  if (!name?.trim()) return "S";
  return name
    .split(/\s+/)
    .slice(0, 2)
    .map((p) => p[0]?.toUpperCase() ?? "")
    .join("");
}

function App() {
  const [authed, setAuthed] = useState(() => !!getToken());
  const [mode, setMode] = useState<Mode>("login");
  const [email, setEmail] = useState("supplier@techlane.local");
  const [password, setPassword] = useState("password");
  const [inviteToken, setInviteToken] = useState("");
  const [contact, setContact] = useState<SupplierContact | null>(null);
  const [tab, setTab] = useState<Tab>("queue");
  const [filter, setFilter] = useState<string>("");
  const [requests, setRequests] = useState<PartRequest[]>([]);
  const [selectedId, setSelectedId] = useState<string | null>(null);
  const [detail, setDetail] = useState<PartRequest | null>(null);
  const [issues, setIssues] = useState<Issue[]>([]);
  const [creditSummary, setCreditSummary] = useState<CreditSummary | null>(null);
  const [issued, setIssued] = useState<{ auth_code: string; qr_payload: string } | null>(null);
  const [unitCost, setUnitCost] = useState("");
  const [notes, setNotes] = useState("");
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");

  const loadMe = useCallback(async () => {
    setContact(await me());
  }, []);

  useEffect(() => {
    if (!authed) return;
    loadMe().catch(() => {
      clearToken();
      setAuthed(false);
    });
  }, [authed, loadMe]);

  useEffect(() => {
    if (!authed || tab !== "queue" || selectedId) return;
    setBusy(true);
    listRequests(filter || undefined)
      .then(setRequests)
      .catch((e) => setError(e instanceof Error ? e.message : "Failed to load"))
      .finally(() => setBusy(false));
  }, [authed, tab, filter, selectedId]);

  useEffect(() => {
    if (!authed || !selectedId) return;
    setBusy(true);
    getRequest(selectedId)
      .then(setDetail)
      .catch((e) => setError(e instanceof Error ? e.message : "Failed to load"))
      .finally(() => setBusy(false));
  }, [authed, selectedId]);

  useEffect(() => {
    if (!authed || tab !== "issues" || issued) return;
    listIssues()
      .then(setIssues)
      .catch((e) => setError(e instanceof Error ? e.message : "Failed to load"));
  }, [authed, tab, issued]);

  useEffect(() => {
    if (!authed || tab !== "credit") return;
    credit()
      .then(setCreditSummary)
      .catch((e) => setError(e instanceof Error ? e.message : "Failed to load"));
  }, [authed, tab]);

  async function onAuth(e: FormEvent) {
    e.preventDefault();
    setBusy(true);
    setError("");
    try {
      const res =
        mode === "login"
          ? await login(email.trim(), password)
          : await acceptInvite(inviteToken.trim(), password);
      setToken(res.token);
      setContact(res.contact);
      setAuthed(true);
      setTab("queue");
    } catch (err) {
      setError(err instanceof Error ? err.message : "Sign-in failed");
    } finally {
      setBusy(false);
    }
  }

  async function onQuote() {
    if (!selectedId) return;
    setBusy(true);
    setError("");
    try {
      await quote(selectedId, Number(unitCost), notes);
      const updated = await getRequest(selectedId);
      setDetail(updated);
      setUnitCost("");
      setNotes("");
      // Quote is auto-authorized server-side; jump straight to the pickup QR.
      if (updated.issue) {
        setIssued({
          auth_code: updated.issue.auth_code,
          qr_payload: `techlane://auth/${updated.issue.id}/${updated.issue.auth_code}`,
        });
      }
    } catch (err) {
      setError(err instanceof Error ? err.message : "Quote failed");
    } finally {
      setBusy(false);
    }
  }

  async function onDecline() {
    if (!selectedId) return;
    setBusy(true);
    try {
      await decline(selectedId, notes);
      setSelectedId(null);
      setDetail(null);
    } catch (err) {
      setError(err instanceof Error ? err.message : "Decline failed");
    } finally {
      setBusy(false);
    }
  }

  async function onReady() {
    if (!selectedId) return;
    setBusy(true);
    try {
      await markReady(selectedId);
      setDetail(await getRequest(selectedId));
    } catch (err) {
      setError(err instanceof Error ? err.message : "Ready failed");
    } finally {
      setBusy(false);
    }
  }

  async function onIssue() {
    if (!selectedId) return;
    setBusy(true);
    setError("");
    try {
      const res = await issue(selectedId);
      const auth = res.auth_code || res.issue?.auth_code || "";
      const id = res.issue?.id || res.id || "";
      const qr = res.qr_payload || (id && auth ? `techlane://auth/${id}/${auth}` : "");
      setIssued({ auth_code: auth, qr_payload: qr });
      setSelectedId(null);
      setDetail(null);
      setTab("issues");
    } catch (err) {
      setError(err instanceof Error ? err.message : "Issue failed");
    } finally {
      setBusy(false);
    }
  }

  if (!authed) {
    return (
      <div className="app">
        <div className="auth">
          <div className="brand-row">
            <img className="brand-mark" src="/logo.svg" alt="TechLane" />
            <span className="muted">TechLane</span>
          </div>
          <h1>Supplier portal</h1>
          <p className="lede">Quote requests, issue parts, and track credit.</p>
          <section className="panel stack">
            <div className="chips">
              <button type="button" className={`chip ${mode === "login" ? "active" : ""}`} onClick={() => setMode("login")}>
                Sign in
              </button>
              <button type="button" className={`chip ${mode === "invite" ? "active" : ""}`} onClick={() => setMode("invite")}>
                Accept invite
              </button>
            </div>
            <form className="stack" onSubmit={onAuth}>
              {mode === "login" ? (
                <label>
                  Email
                  <input type="email" value={email} onChange={(e) => setEmail(e.target.value)} required />
                </label>
              ) : (
                <label>
                  Invite token
                  <input value={inviteToken} onChange={(e) => setInviteToken(e.target.value)} required />
                </label>
              )}
              <label>
                {mode === "invite" ? "Set password" : "Password"}
                <input
                  type="password"
                  value={password}
                  onChange={(e) => setPassword(e.target.value)}
                  required
                  minLength={6}
                />
              </label>
              {error ? <p className="error">{error}</p> : null}
              <button className="btn" disabled={busy}>
                {busy ? "Please wait…" : mode === "login" ? "Sign in" : "Activate account"}
              </button>
            </form>
          </section>
        </div>
      </div>
    );
  }

  if (issued) {
    return (
      <div className="app">
        <main className="shell">
          <div className="topbar">
            <div>
              <h1>Collection QR</h1>
              <p className="muted">Show this to shop staff to collect the part.</p>
            </div>
            <button className="btn-ghost" onClick={() => setIssued(null)}>
              Done
            </button>
          </div>
          <section className="panel qr-panel">
            <p className="muted">
              After you submit a quote and the shop accepts it, mark the part ready, then issue to generate this pickup QR.
            </p>
            {issued.qr_payload ? <QrImage payload={issued.qr_payload} /> : null}
            <p className="muted mono" style={{ wordBreak: "break-all", fontSize: "0.8rem" }}>
              {issued.qr_payload || "—"}
            </p>
            <p className="qr-label">Auth code</p>
            <p className="auth-code">{issued.auth_code || "—"}</p>
          </section>
        </main>
      </div>
    );
  }

  if (selectedId && detail) {
    const qs = detail.quote_status || detail.status;
    const canQuote = ["pending", "assigned", "awaiting", "open", "invited"].includes(detail.status) ||
      qs === "awaiting";
    const canIssue = !detail.issue &&
      (["quoted", "accepted", "ready", "quote_accepted"].includes(qs || "") ||
        detail.quotes?.some((q) => q.status === "accepted" || q.status === "pending"));
    return (
      <div className="app">
        <main className="shell">
          <div className="topbar">
            <div>
              <h1>{titleFor(detail)}</h1>
              <p className="muted">
                Qty {detail.quantity}
                {detail.job_code ? ` · Job ${detail.job_code}` : ""}
              </p>
            </div>
            <button
              className="btn-ghost"
              onClick={() => {
                setSelectedId(null);
                setDetail(null);
              }}
            >
              Back
            </button>
          </div>
          {error ? <p className="error">{error}</p> : null}
          <section className="panel stack">
            <span className="status">{(detail.quote_status || detail.status).replaceAll("_", " ")}</span>
            <p>{detail.description}</p>
            {canQuote ? (
              <>
                <label>
                  Unit cost (KES)
                  <input
                    inputMode="decimal"
                    value={unitCost}
                    onChange={(e) => setUnitCost(e.target.value.replace(/[^\d.]/g, ""))}
                    placeholder="Your price"
                  />
                </label>
                <label>
                  Notes
                  <textarea value={notes} onChange={(e) => setNotes(e.target.value)} placeholder="Optional notes for the shop" />
                </label>
                <p className="muted">
                  Step 1: submit your unit price. Step 2: after acceptance, mark ready and issue the part to create the collection QR.
                </p>
                <div className="row">
                  <button className="btn" disabled={busy || !unitCost} onClick={() => void onQuote()}>
                    Set price & authorize
                  </button>
                  <button className="btn btn-secondary" disabled={busy} onClick={() => void onDecline()}>
                    Decline
                  </button>
                </div>
              </>
            ) : null}
            {canIssue ? (
              <div className="row">
                <button className="btn btn-secondary" disabled={busy} onClick={() => void onReady()}>
                  Mark ready
                </button>
                <button className="btn" disabled={busy} onClick={() => void onIssue()}>
                  Issue part + show QR
                </button>
              </div>
            ) : null}
            {detail.issue ? (
              <div className="row">
                <button
                  className="btn"
                  onClick={() =>
                    setIssued({
                      auth_code: detail.issue!.auth_code,
                      qr_payload: `techlane://auth/${detail.issue!.id}/${detail.issue!.auth_code}`,
                    })
                  }
                >
                  Show existing QR
                </button>
                <button
                  className="btn btn-secondary"
                  type="button"
                  onClick={() => {
                    void openIssueVoucher(detail.issue!.id).catch((err) =>
                      setError(err instanceof Error ? err.message : "Could not open voucher"),
                    );
                  }}
                >
                  Print credit voucher
                </button>
                <button
                  className="btn btn-secondary"
                  type="button"
                  onClick={() => {
                    void downloadIssueVoucherPDF(detail.issue!.id).catch((err) =>
                      setError(err instanceof Error ? err.message : "Could not download voucher PDF"),
                    );
                  }}
                >
                  Voucher PDF
                </button>
              </div>
            ) : null}
          </section>
        </main>
      </div>
    );
  }

  const outstanding =
    creditSummary?.outstanding_credit ??
    creditSummary?.outstanding ??
    creditSummary?.balance ??
    0;

  return (
    <div className="app">
      <main className="shell">
        <div className="topbar">
          <div>
            <h1>
              {tab === "queue"
                ? "Request queue"
                : tab === "issues"
                  ? "Issued parts"
                  : tab === "credit"
                    ? "Credit"
                    : "Profile"}
            </h1>
            <p className="muted">{contact?.supplier_name || contact?.display_name}</p>
          </div>
        </div>
        <div className="tabs">
          {(["queue", "issues", "credit", "profile"] as Tab[]).map((t) => (
            <button key={t} className={tab === t ? "active" : ""} onClick={() => { setTab(t); setError(""); }}>
              {t === "queue" ? "Queue" : t === "issues" ? "Issued" : t === "credit" ? "Credit" : "Profile"}
            </button>
          ))}
        </div>
        {error ? <p className="error">{error}</p> : null}

        {tab === "queue" ? (
          <>
            <div className="chips">
              {[
                ["", "All"],
                ["assigned", "New"],
                ["quoted", "Quoted"],
                ["ready", "Ready"],
              ].map(([value, label]) => (
                <button
                  key={label}
                  type="button"
                  className={`chip ${filter === value ? "active" : ""}`}
                  onClick={() => setFilter(value)}
                >
                  {label}
                </button>
              ))}
            </div>
            <div className="list">
              {busy && requests.length === 0 ? (
                <>
                  <div className="skeleton" />
                  <div className="skeleton" />
                </>
              ) : null}
              {!busy && requests.length === 0 ? (
                <EmptyState
                  title="Queue is clear"
                  body="New part requests assigned to you will show up here."
                />
              ) : null}
              {requests.map((req) => (
                <button key={req.id} className="card" onClick={() => setSelectedId(req.id)}>
                  <div className="card-head">
                    <strong>{titleFor(req)}</strong>
                    <span className="status">{(req.quote_status || req.status).replaceAll("_", " ")}</span>
                  </div>
                  <p className="muted">
                    Qty {req.quantity}
                    {req.job_code ? ` · ${req.job_code}` : ""}
                  </p>
                </button>
              ))}
            </div>
          </>
        ) : null}

        {tab === "issues" ? (
          <div className="list">
            {issues.length === 0 ? (
              <EmptyState
                title="No issued parts yet"
                body="After you issue a part, the collection code appears here."
              />
            ) : null}
            {issues.map((item) => (
              <div key={item.id} className="card">
                <button
                  type="button"
                  className="card"
                  style={{ border: "none", padding: 0, background: "transparent", width: "100%", textAlign: "left" }}
                  onClick={() =>
                    setIssued({
                      auth_code: item.auth_code || "",
                      qr_payload: item.qr_payload || `techlane://auth/${item.id}/${item.auth_code || ""}`,
                    })
                  }
                >
                  <div className="card-head">
                    <strong>{item.part_name || item.description || item.job_code || "Issue"}</strong>
                    <span className="status">{item.status}</span>
                  </div>
                  <p className="muted">KES {item.unit_cost.toLocaleString()}</p>
                </button>
                <button
                  type="button"
                  className="btn btn-secondary"
                  style={{ marginTop: 8 }}
                  onClick={() => {
                    void openIssueVoucher(item.id).catch((err) =>
                      setError(err instanceof Error ? err.message : "Could not open voucher"),
                    );
                  }}
                >
                  Print credit voucher
                </button>
                <button
                  type="button"
                  className="btn btn-secondary"
                  style={{ marginTop: 8 }}
                  onClick={() => {
                    void downloadIssueVoucherPDF(item.id).catch((err) =>
                      setError(err instanceof Error ? err.message : "Could not download voucher PDF"),
                    );
                  }}
                >
                  Voucher PDF
                </button>
              </div>
            ))}
          </div>
        ) : null}

        {tab === "credit" ? (
          <section className="panel">
            <div className="stat-strip">
              <div className="stat-tile">
                <span>Outstanding</span>
                <strong>KES {outstanding.toLocaleString()}</strong>
              </div>
              <div className="stat-tile">
                <span>Ledger entries</span>
                <strong>{(creditSummary?.entries ?? []).length}</strong>
              </div>
            </div>
            <p className="muted">Outstanding credit</p>
            <p className="money">KES {outstanding.toLocaleString()}</p>
            <h3>Ledger</h3>
            {(creditSummary?.entries ?? []).length === 0 ? (
              <EmptyState title="No ledger entries" body="Credit movements will appear as parts are issued and reconciled." />
            ) : (
              <ul className="ledger">
                {(creditSummary?.entries ?? []).map((entry) => (
                  <li key={entry.id}>
                    <div>
                      <strong>{entry.entry_type || entry.type}</strong>
                      <div className="muted">{new Date(entry.created_at).toLocaleDateString()}</div>
                    </div>
                    <strong>KES {entry.amount.toLocaleString()}</strong>
                  </li>
                ))}
              </ul>
            )}
          </section>
        ) : null}

        {tab === "profile" ? (
          <section className="panel stack">
            <div className="profile-avatar">{initials(contact?.display_name)}</div>
            <div>
              <h2 style={{ margin: 0 }}>{contact?.display_name}</h2>
              <p className="muted" style={{ margin: "0.35rem 0 0" }}>{contact?.email}</p>
            </div>
            <p>{contact?.supplier_name}</p>
            <button
              className="btn btn-secondary"
              onClick={() =>
                void logout().then(() => {
                  setAuthed(false);
                  setContact(null);
                })
              }
            >
              Sign out
            </button>
          </section>
        ) : null}
      </main>
    </div>
  );
}

export default App;
