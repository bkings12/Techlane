import { useState, type FormEvent } from "react";
import { Link } from "react-router-dom";
import { forgotPassword } from "../lib/api";
import { Button, Input } from "../components/ui";

export function ForgotPasswordPage() {
  const [email, setEmail] = useState("");
  const [error, setError] = useState("");
  const [submitting, setSubmitting] = useState(false);
  const [sent, setSent] = useState(false);

  async function onSubmit(e: FormEvent) {
    e.preventDefault();
    setSubmitting(true);
    setError("");
    try {
      await forgotPassword(email);
      setSent(true);
    } catch (err) {
      setError(err instanceof Error ? err.message : "Something went wrong");
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
            <h2>Reset your password</h2>
            <p>We'll email you a link to choose a new one</p>
          </header>
          {sent ? (
            <p className="hint" role="status">
              If that email exists, a reset link has been sent. Check your inbox — the link expires in 30
              minutes.
            </p>
          ) : (
            <>
              <label>
                Email
                <Input
                  type="email"
                  value={email}
                  onChange={(e) => setEmail(e.target.value)}
                  required
                  autoComplete="username"
                  autoFocus
                />
              </label>
              {error ? (
                <p className="form-error" role="alert">
                  {error}
                </p>
              ) : null}
              <Button type="submit" variant="gold" disabled={submitting}>
                {submitting ? "Sending…" : "Send reset link"}
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
