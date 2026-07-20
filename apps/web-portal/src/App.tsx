import { useCallback, useEffect, useState, type FormEvent, type ReactNode } from "react";
import {
  approveEstimate,
  claimRepairWarranty,
  clearSession,
  downloadRepairReceiptPDF,
  getRepair,
  getRepairWarranty,
  getToken,
  guestLookup,
  listRepairs,
  logout,
  me,
  openRepairReceipt,
  payRepair,
  paymentStatus,
  rejectEstimate,
  requestOtp,
  savedPhone,
  setSession,
  statusLabels,
  verifyOtp,
  type Customer,
  type GuestRepair,
  type Repair,
  type Warranty,
} from "./api";

type View = "auth" | "home" | "detail" | "profile" | "guest";

function deviceLabel(r: Repair | GuestRepair) {
  if ("device_brand" in r || "device_model" in r) {
    const brand = (r as Repair).device_brand;
    const model = (r as Repair).device_model;
    return [brand, model].filter(Boolean).join(" ");
  }
  const d = r.device;
  return [d?.brand, d?.model, d?.kind].filter(Boolean).join(" ");
}

function pendingEstimate(r: Repair) {
  return r.estimate ?? r.pending_estimate ?? r.estimates?.find((e) => e.status === "pending") ?? null;
}

function pillTone(status: string): "ok" | "warn" | "danger" | "" {
  if (["completed", "collected", "approved"].includes(status)) return "ok";
  if (["waiting_parts", "pending", "diagnosed", "intake"].includes(status)) return "warn";
  if (status.includes("fail") || status === "cancelled" || status === "rejected") return "danger";
  return "";
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

function initials(name?: string | null, phone?: string | null) {
  if (name?.trim()) {
    return name
      .split(/\s+/)
      .slice(0, 2)
      .map((p) => p[0]?.toUpperCase() ?? "")
      .join("");
  }
  return (phone || "?").slice(-2);
}

function App() {
  const [view, setView] = useState<View>(() => (getToken() ? "home" : "auth"));
  const [token, setToken] = useState(() => getToken());
  const [phone, setPhone] = useState(() => savedPhone());
  const [code, setCode] = useState("");
  const [otpStep, setOtpStep] = useState<"phone" | "code">("phone");
  const [customer, setCustomer] = useState<Customer | null>(null);
  const [repairs, setRepairs] = useState<Repair[]>([]);
  const [selectedId, setSelectedId] = useState<string | null>(null);
  const [detail, setDetail] = useState<Repair | null>(null);
  const [warranty, setWarranty] = useState<Warranty | null>(null);
  const [warrantyNote, setWarrantyNote] = useState("");
  const [guest, setGuest] = useState<GuestRepair | null>(null);
  const [jobCode, setJobCode] = useState("");
  const [guestPhone, setGuestPhone] = useState("");
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");
  const [hint, setHint] = useState("");
  const [payMsg, setPayMsg] = useState("");

  const refreshList = useCallback(async () => {
    const items = await listRepairs();
    setRepairs(items);
  }, []);

  const refreshDetail = useCallback(async (id: string) => {
    const r = await getRepair(id);
    setDetail(r);
    setWarranty(null);
    void getRepairWarranty(r.id)
      .then(setWarranty)
      .catch(() => setWarranty(null));
  }, []);

  useEffect(() => {
    if (!token) return;
    me()
      .then(setCustomer)
      .catch(() => {
        clearSession();
        setToken(null);
        setView("auth");
      });
  }, [token]);

  useEffect(() => {
    if (view !== "home" || !token) return;
    setBusy(true);
    refreshList()
      .catch((e) => setError(e instanceof Error ? e.message : "Failed to load"))
      .finally(() => setBusy(false));
  }, [view, token, refreshList]);

  useEffect(() => {
    if (view !== "detail" || !selectedId) return;
    setBusy(true);
    refreshDetail(selectedId)
      .catch((e) => setError(e instanceof Error ? e.message : "Failed to load"))
      .finally(() => setBusy(false));
  }, [view, selectedId, refreshDetail]);

  async function onRequestOtp(e: FormEvent) {
    e.preventDefault();
    setBusy(true);
    setError("");
    setHint("");
    try {
      await requestOtp(phone.trim());
      setOtpStep("code");
      setHint("We sent a verification code by SMS.");
    } catch (err) {
      setError(err instanceof Error ? err.message : "Could not send code");
    } finally {
      setBusy(false);
    }
  }

  async function onVerifyOtp(e: FormEvent) {
    e.preventDefault();
    setBusy(true);
    setError("");
    try {
      const res = await verifyOtp(phone.trim(), code.trim());
      setSession(res.token, phone.trim());
      setToken(res.token);
      setCustomer(res.customer);
      setView("home");
      setCode("");
      setOtpStep("phone");
    } catch (err) {
      setError(err instanceof Error ? err.message : "Invalid code");
    } finally {
      setBusy(false);
    }
  }

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

  async function onApprove() {
    if (!detail) return;
    const est = pendingEstimate(detail);
    if (!est) return;
    setBusy(true);
    setError("");
    try {
      await approveEstimate(detail.id, est.id);
      await refreshDetail(detail.id);
    } catch (err) {
      setError(err instanceof Error ? err.message : "Approve failed");
    } finally {
      setBusy(false);
    }
  }

  async function onReject() {
    if (!detail) return;
    const est = pendingEstimate(detail);
    if (!est) return;
    setBusy(true);
    setError("");
    try {
      await rejectEstimate(detail.id, est.id);
      await refreshDetail(detail.id);
    } catch (err) {
      setError(err instanceof Error ? err.message : "Reject failed");
    } finally {
      setBusy(false);
    }
  }

  async function onPay() {
    if (!detail) return;
    setBusy(true);
    setError("");
    setPayMsg("");
    try {
      const res = await payRepair(detail.id, phone || savedPhone());
      const paymentId = res.payment_id || res.id;
      setPayMsg(res.message || "STK push sent — check your phone.");
      if (paymentId) {
        for (let i = 0; i < 12; i++) {
          await new Promise((r) => setTimeout(r, 2500));
          const st = await paymentStatus(detail.id, paymentId);
          setPayMsg(`Payment: ${st.status}`);
          if (["confirmed", "allocated", "failed", "cancelled"].includes(st.status)) {
            await refreshDetail(detail.id);
            break;
          }
        }
      }
    } catch (err) {
      setError(err instanceof Error ? err.message : "Payment failed");
    } finally {
      setBusy(false);
    }
  }

  async function onSignOut() {
    await logout();
    setToken(null);
    setCustomer(null);
    setRepairs([]);
    setDetail(null);
    setView("auth");
  }

  if (view === "auth") {
    return (
      <div className="app">
        <div className="hero-auth">
          <div className="brand">
            <img className="brand-mark" src="/logo.svg" alt="TechLane" />
          </div>
          <h1>Track your repair</h1>
          <p className="lede">Sign in with the phone number you left at the shop.</p>
          <section className="panel stack">
            {otpStep === "phone" ? (
              <form className="stack" onSubmit={onRequestOtp}>
                <label>
                  Phone number
                  <input
                    inputMode="tel"
                    placeholder="07… or 254…"
                    value={phone}
                    onChange={(e) => setPhone(e.target.value)}
                    required
                  />
                </label>
                {error ? <p className="error">{error}</p> : null}
                <button className="btn" disabled={busy || !phone.trim()}>
                  {busy ? "Sending…" : "Send code"}
                </button>
              </form>
            ) : (
              <form className="stack" onSubmit={onVerifyOtp}>
                <label>
                  6-digit code
                  <input
                    inputMode="numeric"
                    value={code}
                    onChange={(e) => setCode(e.target.value.replace(/\D/g, "").slice(0, 6))}
                    required
                  />
                </label>
                {hint ? <p className="hint">{hint}</p> : null}
                {error ? <p className="error">{error}</p> : null}
                <button className="btn" disabled={busy || code.length < 4}>
                  {busy ? "Verifying…" : "Continue"}
                </button>
                <button
                  type="button"
                  className="linkish"
                  onClick={() => {
                    setOtpStep("phone");
                    setCode("");
                    setError("");
                  }}
                >
                  Use a different number
                </button>
              </form>
            )}
            <button type="button" className="linkish" onClick={() => { setView("guest"); setError(""); }}>
              Look up with job code instead
            </button>
          </section>
        </div>
      </div>
    );
  }

  if (view === "guest") {
    return (
      <div className="app">
        <main className="shell">
          <div className="topbar">
            <div>
              <h1>Guest lookup</h1>
              <p className="muted">Read-only status with job code and phone.</p>
            </div>
            <button className="btn-ghost" onClick={() => { setView(token ? "home" : "auth"); setGuest(null); }}>
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

  if (view === "detail" && detail) {
    const est = pendingEstimate(detail);
    const balance = detail.balance_due ?? detail.amount_due ?? 0;
    return (
      <div className="app">
        <main className="shell">
          <div className="topbar">
            <div>
              <h1>{detail.job_code}</h1>
              <p className="muted">{deviceLabel(detail) || "Device under repair"}</p>
            </div>
            <button
              className="btn-ghost"
              onClick={() => {
                setView("home");
                setSelectedId(null);
                setDetail(null);
                setPayMsg("");
              }}
            >
              Back
            </button>
          </div>
          {error ? <p className="error">{error}</p> : null}
          <section className="section">
            <span className={`pill ${pillTone(detail.status)}`}>
              {statusLabels[detail.status] ?? detail.status}
            </span>
            <p className="muted" style={{ marginTop: "0.85rem" }}>
              {detail.problem_summary}
            </p>
          </section>

          {est && est.status === "pending" ? (
            <section className="section estimate">
              <h2>Estimate for approval</h2>
              <div className="estimate-lines">
                <p>
                  <span>Labor</span>
                  <span>{est.currency || "KES"} {est.labor_amount.toLocaleString()}</span>
                </p>
                <p>
                  <span>Parts</span>
                  <span>{est.currency || "KES"} {est.parts_amount.toLocaleString()}</span>
                </p>
              </div>
              <p className="money">
                {(est.currency || "KES")} {(est.labor_amount + est.parts_amount).toLocaleString()}
              </p>
              {est.notes ? <p className="muted">{est.notes}</p> : null}
              <div className="btn-row">
                <button className="btn" disabled={busy} onClick={() => void onApprove()}>
                  Approve
                </button>
                <button className="btn btn-secondary" disabled={busy} onClick={() => void onReject()}>
                  Reject
                </button>
              </div>
            </section>
          ) : null}

          {balance > 0 ? (
            <section className="section">
              <h2>Pay balance</h2>
              <p className="money">KES {balance.toLocaleString()}</p>
              {payMsg ? <p className="hint">{payMsg}</p> : null}
              <button className="btn" disabled={busy} onClick={() => void onPay()}>
                {busy ? "Sending STK…" : "Pay with M-Pesa"}
              </button>
            </section>
          ) : null}

          <section className="section">
            <h3>Timeline</h3>
            {(detail.timeline ?? []).length === 0 ? (
              <EmptyState title="No updates yet" body="You'll see progress here as work happens." />
            ) : (
              <ol className="timeline">
                {(detail.timeline ?? []).map((ev, i) => (
                  <li key={`${ev.status}-${ev.created_at ?? ev.at}-${i}`}>
                    <strong>{statusLabels[ev.status ?? ""] ?? ev.status ?? ev.event_type}</strong>
                    <time>
                      {ev.created_at || ev.at ? new Date(ev.created_at || ev.at || "").toLocaleString() : ""}
                    </time>
                    {ev.note ? <p>{ev.note}</p> : null}
                  </li>
                ))}
              </ol>
            )}
          </section>

          <section className="section">
            <h3>Receipts</h3>
            <div className="row" style={{ marginBottom: 12 }}>
              <button
                className="btn btn-secondary"
                type="button"
                onClick={() => {
                  void openRepairReceipt(detail.id).catch((err) =>
                    setError(err instanceof Error ? err.message : "Could not open receipt"),
                  );
                }}
              >
                Print receipt
              </button>
              <button
                className="btn btn-secondary"
                type="button"
                onClick={() => {
                  void downloadRepairReceiptPDF(detail.id).catch((err) =>
                    setError(err instanceof Error ? err.message : "Could not download PDF"),
                  );
                }}
              >
                Receipt PDF
              </button>
            </div>
            {warranty ? (
              <div className="card" style={{ marginBottom: 12 }}>
                <strong>Warranty</strong>
                <p className="muted">
                  {warranty.status} · {warranty.duration_days} days · ends{" "}
                  {new Date(warranty.ends_at).toLocaleDateString()}
                </p>
                {warranty.status === "active" ? (
                  <form
                    onSubmit={(e) => {
                      e.preventDefault();
                      void claimRepairWarranty(detail.id, warrantyNote)
                        .then(setWarranty)
                        .catch((err) => setError(err instanceof Error ? err.message : "Claim failed"));
                    }}
                  >
                    <input
                      value={warrantyNote}
                      onChange={(e) => setWarrantyNote(e.target.value)}
                      placeholder="Claim note"
                    />
                    <button className="btn" type="submit">
                      Claim warranty
                    </button>
                  </form>
                ) : null}
              </div>
            ) : null}
            {(detail.receipts ?? []).length === 0 ? (
              <EmptyState title="No payments yet" body="Receipts appear here after you pay." />
            ) : (
              <ul className="receipts">
                {(detail.receipts ?? []).map((r) => (
                  <li key={r.id}>
                    <div>
                      <strong>
                        {r.currency || "KES"} {r.amount.toLocaleString()}
                      </strong>
                      <div className="muted">
                        {r.method.replaceAll("_", " ")} · {r.status}
                      </div>
                    </div>
                    <time>{new Date(r.created_at).toLocaleString()}</time>
                  </li>
                ))}
              </ul>
            )}
          </section>
        </main>
      </div>
    );
  }

  const displayName = customer?.full_name || customer?.name || "Customer";
  const displayPhone = customer?.phone || phone || savedPhone();

  return (
    <div className="app">
      <main className="shell">
        <div className="topbar">
          <div>
            <h1>{view === "profile" ? "Profile" : "My repairs"}</h1>
            <p className="muted">
              {view === "profile"
                ? "Your signed-in customer account"
                : "Approve estimates and pay when ready."}
            </p>
          </div>
        </div>
        <div className="tabs">
          <button className={view === "home" ? "active" : ""} onClick={() => setView("home")}>
            Repairs
          </button>
          <button className={view === "profile" ? "active" : ""} onClick={() => setView("profile")}>
            Profile
          </button>
          <button onClick={() => { setView("guest"); setError(""); }}>Guest lookup</button>
        </div>

        {error ? <p className="error">{error}</p> : null}

        {view === "profile" ? (
          <section className="section profile-card">
            <div className="profile-avatar">{initials(displayName, displayPhone)}</div>
            <div>
              <h2 style={{ margin: 0 }}>{displayName}</h2>
              <p className="muted" style={{ margin: "0.35rem 0 0" }}>{displayPhone}</p>
            </div>
            <div className="stat-row">
              <div className="stat-tile">
                <span>Open repairs</span>
                <strong>{repairs.filter((r) => !["collected", "cancelled"].includes(r.status)).length}</strong>
              </div>
              <div className="stat-tile">
                <span>Total jobs</span>
                <strong>{repairs.length}</strong>
              </div>
            </div>
            <button className="btn btn-secondary" onClick={() => void onSignOut()}>
              Sign out
            </button>
          </section>
        ) : (
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
              </section>
            ) : null}
            {repairs.map((r) => {
              const est = pendingEstimate(r);
              const due = (r.balance_due ?? r.amount_due ?? 0) > 0;
              return (
                <button
                  key={r.id}
                  className="repair-card"
                  onClick={() => {
                    setSelectedId(r.id);
                    setView("detail");
                    setError("");
                  }}
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
          </section>
        )}
      </main>
    </div>
  );
}

export default App;
