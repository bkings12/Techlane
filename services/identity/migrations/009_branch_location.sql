-- Physical / area location shown on customer SMS (e.g. "Westlands, along Waiyaki Way").
ALTER TABLE identity.branches
  ADD COLUMN IF NOT EXISTS location TEXT NOT NULL DEFAULT '';
