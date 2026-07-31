import { useState, type FormEvent } from "react";
import { Link, Navigate } from "react-router-dom";
import { useAuth } from "../auth/AuthContext";
import { AccountLockedError } from "../lib/api";
import { Button, Input, PasswordInput } from "../components/ui";

export function LoginPage() {
  const { user, login, completeMfaLogin, loading, sessionExpired, clearSessionExpired } = useAuth();
  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");
  const [error, setError] = useState("");
  const [submitting, setSubmitting] = useState(false);

  const [mfaChallenge, setMfaChallenge] = useState<string | null>(null);
  const [mfaCode, setMfaCode] = useState("");

  if (!loading && user) return <Navigate to="/" replace />;

  async function onSubmit(e: FormEvent) {
    e.preventDefault();
    setSubmitting(true);
    setError("");
    clearSessionExpired();
    try {
      const result = await login(email, password);
      if (result.status === "mfa_required") {
        setMfaChallenge(result.challenge);
      }
    } catch (err) {
      if (err instanceof AccountLockedError) {
        const until = err.lockedUntil ? new Date(err.lockedUntil) : null;
        const when = until ? until.toLocaleTimeString([], { hour: "2-digit", minute: "2-digit" }) : "shortly";
        setError(`Too many failed attempts. Account is locked until ${when}. Try again then, or reset your password.`);
      } else {
        setError(err instanceof Error ? err.message : "Login failed");
      }
    } finally {
      setSubmitting(false);
    }
  }

  async function onSubmitMfa(e: FormEvent) {
    e.preventDefault();
    if (!mfaChallenge) return;
    setSubmitting(true);
    setError("");
    try {
      await completeMfaLogin(mfaChallenge, mfaCode);
    } catch (err) {
      setError(err instanceof Error ? err.message : "Invalid code");
    } finally {
      setSubmitting(false);
    }
  }

  return (
    <div className="login-page">
      <section className="login-hero" aria-hidden="false">
        <div className="login-hero-inner">
          <svg className="login-mark" width="72" height="72" viewBox="0 0 32 32" fill="none" aria-hidden="true">
            <rect width="32" height="32" rx="7" fill="rgba(255,255,255,0.12)" />
            <rect x="5" y="10" width="3" height="12" rx="1.5" fill="#F2BE2A" />
            <rect x="11" y="8" width="3" height="16" rx="1.5" fill="#FFFFFF" />
            <rect x="17" y="12" width="3" height="8" rx="1.5" fill="#F2BE2A" />
            <rect x="23" y="7" width="3" height="18" rx="1.5" fill="#FFFFFF" />
          </svg>
          <h1>TechLane</h1>
          <span className="login-chip">Operations</span>
          <p>Every repair, every shilling, every device — in one clear view.</p>
        </div>
      </section>

      <section className="login-panel">
        {mfaChallenge ? (
          <form className="login-card" onSubmit={onSubmitMfa}>
            <header className="login-card-head">
              <h2>Two-factor verification</h2>
              <p>Enter the 6-digit code from your authenticator app</p>
            </header>
            <label>
              Authentication code
              <Input
                type="text"
                inputMode="numeric"
                pattern="[0-9]*"
                maxLength={10}
                autoFocus
                value={mfaCode}
                onChange={(e) => setMfaCode(e.target.value)}
                placeholder="123456"
                required
              />
            </label>
            {error ? (
              <p className="form-error" role="alert">
                {error}
              </p>
            ) : null}
            <Button type="submit" variant="gold" disabled={submitting}>
              {submitting ? "Verifying…" : "Verify & sign in"}
            </Button>
            <button
              type="button"
              className="link-button"
              onClick={() => {
                setMfaChallenge(null);
                setMfaCode("");
                setError("");
              }}
            >
              Back to sign in
            </button>
            <p className="hint">Lost your device? Use one of your backup codes instead.</p>
          </form>
        ) : (
          <form className="login-card" onSubmit={onSubmit}>
            <header className="login-card-head">
              <h2>Welcome back</h2>
              <p>Sign in with your staff account</p>
            </header>
            {sessionExpired ? (
              <p className="hint" role="status">
                Your session expired. Please sign in again.
              </p>
            ) : null}
            <label>
              Email
              <Input
                type="email"
                value={email}
                onChange={(e) => setEmail(e.target.value)}
                required
                autoComplete="username"
              />
            </label>
            <label>
              Password
              <PasswordInput
                value={password}
                onChange={(e) => setPassword(e.target.value)}
                required
                autoComplete="current-password"
              />
            </label>
            {error ? (
              <p className="form-error" role="alert">
                {error}
              </p>
            ) : null}
            <Button type="submit" variant="gold" disabled={submitting}>
              {submitting ? "Signing in…" : "Sign in"}
            </Button>
            <Link to="/forgot-password" className="link-button">
              Forgot password?
            </Link>
            <p className="hint">
              New shop? <Link to="/signup">Create your TechLane account</Link>
            </p>
          </form>
        )}
      </section>
    </div>
  );
}
