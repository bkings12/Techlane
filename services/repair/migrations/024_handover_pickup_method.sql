-- QR / intake-slip release records verification_method = 'pickup_code'.
-- The original handover table only allowed otp + staff_vouched, so scanning
-- the intake QR and releasing failed the CHECK constraint.
ALTER TABLE repair.job_handovers
  DROP CONSTRAINT IF EXISTS job_handovers_verification_method_check;

ALTER TABLE repair.job_handovers
  ADD CONSTRAINT job_handovers_verification_method_check
  CHECK (verification_method IN ('otp', 'staff_vouched', 'pickup_code'));
