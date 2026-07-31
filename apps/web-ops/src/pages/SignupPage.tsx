import { useState, type FormEvent } from "react";
import { Link, Navigate } from "react-router-dom";
import { useAuth } from "../auth/AuthContext";
import { Button, Input, PasswordInput } from "../components/ui";

export function SignupPage() {
  const { user, registerTenant, loading } = useAuth();
  const [companyName, setCompanyName] = useState("");
  const [ownerName, setOwnerName] = useState("");
  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");
  const [confirmPassword, setConfirmPassword] = useState("");
  const [error, setError] = useState("");
  const [submitting, setSubmitting] = useState(false);

  if (!loading && user) return <Navigate to="/" replace />;

  async function onSubmit(e: FormEvent) {
    e.preventDefault();
    setError("");
    if (password.length < 8) {
      setError("Password must be at least 8 characters.");
      return;
    }
    if (password !== confirmPassword) {
      setError("Passwords do not match.");
      return;
    }
    setSubmitting(true);
    try {
      await registerTenant({
        company_name: companyName.trim(),
        owner_name: ownerName.trim(),
        email: email.trim(),
        password,
      });
    } catch (err) {
      setError(err instanceof Error ? err.message : "Signup failed");
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
          <span className="login-chip">Get started</span>
          <p>Set up your shop in minutes — repairs, inventory, POS, and reporting from day one.</p>
        </div>
      </section>

      <section className="login-panel">
        <form className="login-card" onSubmit={onSubmit}>
          <header className="login-card-head">
            <h2>Create your shop</h2>
            <p>You'll be the owner with full access — invite your team afterwards</p>
          </header>
          <label>
            Company / shop name
            <Input
              type="text"
              value={companyName}
              onChange={(e) => setCompanyName(e.target.value)}
              required
              autoComplete="organization"
              placeholder="e.g. Unganisha Networks"
            />
          </label>
          <label>
            Your name
            <Input
              type="text"
              value={ownerName}
              onChange={(e) => setOwnerName(e.target.value)}
              required
              autoComplete="name"
              placeholder="e.g. Jane Doe"
            />
          </label>
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
              minLength={8}
              autoComplete="new-password"
            />
          </label>
          <label>
            Confirm password
            <PasswordInput
              value={confirmPassword}
              onChange={(e) => setConfirmPassword(e.target.value)}
              required
              minLength={8}
              autoComplete="new-password"
            />
          </label>
          {error ? (
            <p className="form-error" role="alert">
              {error}
            </p>
          ) : null}
          <Button type="submit" variant="gold" disabled={submitting}>
            {submitting ? "Creating your shop…" : "Create shop & sign in"}
          </Button>
          <p className="hint">
            Already have an account? <Link to="/login">Sign in</Link>
          </p>
        </form>
      </section>
    </div>
  );
}
