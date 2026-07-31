import { useEffect, useState, type FormEvent } from "react";
import { Button, Input, PageHeader, PasswordInput } from "../../components/ui";
import { disableMfa, enableMfa, getMfaStatus, setupMfa } from "../../lib/api";

export function SecuritySettingsPage() {
  const [enabled, setEnabled] = useState<boolean | null>(null);
  const [error, setError] = useState("");
  const [busy, setBusy] = useState(false);

  const [setupSecret, setSetupSecret] = useState<{ secret: string; otpauth_url: string } | null>(null);
  const [code, setCode] = useState("");
  const [backupCodes, setBackupCodes] = useState<string[] | null>(null);

  const [disablePassword, setDisablePassword] = useState("");
  const [confirmingDisable, setConfirmingDisable] = useState(false);

  function load() {
    getMfaStatus()
      .then((s) => setEnabled(s.enabled))
      .catch((e) => setError(e instanceof Error ? e.message : "Failed to load"));
  }

  useEffect(load, []);

  async function startSetup() {
    setError("");
    setBusy(true);
    try {
      const result = await setupMfa();
      setSetupSecret(result);
    } catch (err) {
      setError(err instanceof Error ? err.message : "Could not start setup");
    } finally {
      setBusy(false);
    }
  }

  async function confirmEnable(e: FormEvent) {
    e.preventDefault();
    setError("");
    setBusy(true);
    try {
      const result = await enableMfa(code);
      setBackupCodes(result.backup_codes);
      setSetupSecret(null);
      setCode("");
      setEnabled(true);
    } catch (err) {
      setError(err instanceof Error ? err.message : "Invalid code — check your authenticator app and try again");
    } finally {
      setBusy(false);
    }
  }

  async function confirmDisable(e: FormEvent) {
    e.preventDefault();
    setError("");
    setBusy(true);
    try {
      await disableMfa(disablePassword);
      setDisablePassword("");
      setConfirmingDisable(false);
      setEnabled(false);
    } catch (err) {
      setError(err instanceof Error ? err.message : "Could not disable — check your password");
    } finally {
      setBusy(false);
    }
  }

  return (
    <div className="settings-page">
      <PageHeader
        title="Security"
        subtitle="Two-factor authentication and account protection"
      />
      {error ? <p className="form-error">{error}</p> : null}

      <section className="board-pulse" aria-label="Security pulse">
        <div className={enabled === false ? "warn" : ""}>
          <strong>{enabled === null ? "…" : enabled ? "On" : "Off"}</strong>
          <span>Two-factor auth</span>
        </div>
      </section>

      {backupCodes ? (
        <section className="settings-form-card">
          <h2>Save your backup codes</h2>
          <p className="hint">
            Two-factor authentication is now on. Store these one-time backup codes somewhere safe — each can be
            used once if you lose access to your authenticator app. They won't be shown again.
          </p>
          <div className="backup-code-grid">
            {backupCodes.map((c) => (
              <code key={c} className="backup-code">
                {c}
              </code>
            ))}
          </div>
          <div className="chip-row">
            <Button type="button" onClick={() => setBackupCodes(null)}>
              I've saved these codes
            </Button>
          </div>
        </section>
      ) : null}

      {enabled === false && !setupSecret && !backupCodes ? (
        <section className="settings-form-card">
          <h2>Turn on two-factor authentication</h2>
          <p className="hint">
            Protects this account even if your password is ever stolen or guessed. We strongly recommend this for
            owner and manager accounts.
          </p>
          <div className="chip-row">
            <Button type="button" onClick={startSetup} disabled={busy}>
              Set up two-factor authentication
            </Button>
          </div>
        </section>
      ) : null}

      {setupSecret ? (
        <form className="settings-form-card form-grid" onSubmit={confirmEnable}>
          <h2>Scan or enter this key</h2>
          <p className="hint">
            Add a new account in Google Authenticator, Authy, or 1Password using this manual entry key (QR scanning
            isn't available yet), then enter the 6-digit code it generates.
          </p>
          <label>
            Manual entry key
            <Input readOnly value={setupSecret.secret} onFocus={(e) => e.currentTarget.select()} />
          </label>
          <label>
            6-digit code
            <Input
              value={code}
              onChange={(e) => setCode(e.target.value)}
              inputMode="numeric"
              maxLength={10}
              placeholder="123456"
              required
              autoFocus
            />
          </label>
          <div className="chip-row">
            <Button type="submit" disabled={busy}>
              Confirm & enable
            </Button>
            <Button type="button" variant="secondary" onClick={() => setSetupSecret(null)}>
              Cancel
            </Button>
          </div>
        </form>
      ) : null}

      {enabled === true && !backupCodes ? (
        <section className="settings-form-card">
          <h2>Two-factor authentication is on</h2>
          <p className="hint">Your account requires an authenticator code at every sign-in.</p>
          {confirmingDisable ? (
            <form className="form-grid" onSubmit={confirmDisable}>
              <label>
                Confirm your password to disable
                <PasswordInput
                  value={disablePassword}
                  onChange={(e) => setDisablePassword(e.target.value)}
                  autoComplete="current-password"
                  required
                  autoFocus
                />
              </label>
              <div className="chip-row">
                <Button type="submit" variant="secondary" disabled={busy}>
                  Turn off two-factor authentication
                </Button>
                <Button type="button" variant="secondary" onClick={() => setConfirmingDisable(false)}>
                  Cancel
                </Button>
              </div>
            </form>
          ) : (
            <div className="chip-row">
              <Button type="button" variant="secondary" onClick={() => setConfirmingDisable(true)}>
                Turn off two-factor authentication
              </Button>
            </div>
          )}
        </section>
      ) : null}
    </div>
  );
}
