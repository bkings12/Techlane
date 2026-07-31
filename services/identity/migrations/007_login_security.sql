-- Login hardening: lockout tracking + audit-friendly timestamps
ALTER TABLE identity.users
  ADD COLUMN IF NOT EXISTS failed_login_count INT NOT NULL DEFAULT 0,
  ADD COLUMN IF NOT EXISTS locked_until TIMESTAMPTZ,
  ADD COLUMN IF NOT EXISTS last_login_at TIMESTAMPTZ;

-- TOTP-based MFA enrollment
CREATE TABLE IF NOT EXISTS identity.mfa_totp (
  user_id UUID PRIMARY KEY REFERENCES identity.users(id),
  secret_base32 TEXT NOT NULL,
  enabled BOOLEAN NOT NULL DEFAULT false,
  backup_codes TEXT[] NOT NULL DEFAULT '{}',
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  enabled_at TIMESTAMPTZ
);

-- Short-lived challenge issued between password-check and MFA-code verification
CREATE TABLE IF NOT EXISTS identity.mfa_challenges (
  id UUID PRIMARY KEY,
  user_id UUID NOT NULL REFERENCES identity.users(id),
  expires_at TIMESTAMPTZ NOT NULL,
  consumed_at TIMESTAMPTZ,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_mfa_challenges_expiry ON identity.mfa_challenges (expires_at);

-- Password reset tokens (email-based recovery)
CREATE TABLE IF NOT EXISTS identity.password_reset_tokens (
  id UUID PRIMARY KEY,
  user_id UUID NOT NULL REFERENCES identity.users(id),
  token_hash TEXT NOT NULL UNIQUE,
  expires_at TIMESTAMPTZ NOT NULL,
  used_at TIMESTAMPTZ,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_password_reset_tokens_user ON identity.password_reset_tokens (user_id);
