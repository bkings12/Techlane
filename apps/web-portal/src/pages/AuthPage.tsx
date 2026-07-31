import { useState, type FormEvent } from "react";
import { Navigate, useLocation, useNavigate } from "react-router-dom";
import { requestOtp, savedPhone, setSession, verifyOtp } from "../api";
import { useSession } from "../session";

export function AuthPage() {
  const { token, setToken, setCustomer } = useSession();
  const navigate = useNavigate();
  const location = useLocation();
  const notice = (location.state as { notice?: string } | null)?.notice ?? "";

  const [phone, setPhone] = useState(() => savedPhone());
  const [code, setCode] = useState("");
  const [otpStep, setOtpStep] = useState<"phone" | "code">("phone");
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");
  const [hint, setHint] = useState("");

  if (token) return <Navigate to="/" replace />;

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
      navigate("/", { replace: true });
    } catch (err) {
      setError(err instanceof Error ? err.message : "Invalid code");
    } finally {
      setBusy(false);
    }
  }

  return (
    <div className="app">
      <div className="hero-auth">
        <div className="brand">
          <img className="brand-mark" src="/logo.svg" alt="TechLane" />
        </div>
        <h1>Track your repair</h1>
        <p className="lede">Sign in with the phone number you left at the shop.</p>
        <section className="panel stack">
          {notice ? (
            <p className="hint" role="status">
              {notice}
            </p>
          ) : null}
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
          <button type="button" className="linkish" onClick={() => navigate("/guest")}>
            Look up with job code instead
          </button>
        </section>
      </div>
    </div>
  );
}
