import { useState, type FormEvent } from "react";
import { Link, useSearchParams } from "react-router-dom";
import { resetPassword } from "../lib/api";
import { Button, PasswordInput } from "../components/ui";

export function ResetPasswordPage() {
  const [params] = useSearchParams();
  const token = params.get("token") ?? "";
  const [password, setPassword] = useState("");
  const [confirm, setConfirm] = useState("");
  const [error, setError] = useState("");
  const [submitting, setSubmitting] = useState(false);
  const [done, setDone] = useState(false);

  async function onSubmit(e: FormEvent) {
    e.preventDefault();
    setError("");
    if (password.length < 8) {
      setError("Password must be at least 8 characters");
      return;
    }
    if (password !== confirm) {
      setError("Passwords don't match");
      return;
    }
    setSubmitting(true);
    try {
      await resetPassword(token, password);
      setDone(true);
    } catch (err) {
      setError(err instanceof Error ? err.message : "Reset failed");
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
        <form className="login-card" onSubmit={onSubmit}>
          <header className="login-card-head">
            <h2>Choose a new password</h2>
            <p>Make it something you don't use anywhere else</p>
          </header>
          {!token ? (
            <p className="form-error" role="alert">
              This reset link is missing its token. Request a new one from the sign-in page.
            </p>
          ) : done ? (
            <p className="hint" role="status">
              Password updated. You can now sign in with your new password.
            </p>
          ) : (
            <>
              <label>
                New password
                <PasswordInput
                  value={password}
                  onChange={(e) => setPassword(e.target.value)}
                  required
                  autoComplete="new-password"
                  autoFocus
                />
              </label>
              <label>
                Confirm password
                <PasswordInput
                  value={confirm}
                  onChange={(e) => setConfirm(e.target.value)}
                  required
                  autoComplete="new-password"
                />
              </label>
              {error ? (
                <p className="form-error" role="alert">
                  {error}
                </p>
              ) : null}
              <Button type="submit" variant="gold" disabled={submitting}>
                {submitting ? "Updating…" : "Update password"}
              </Button>
            </>
          )}
          <Link to="/login" className="link-button">
            Back to sign in
          </Link>
        </form>
      </section>
    </div>
  );
}
