-- A technician works from one shop. Keep the first existing assignment and
-- enforce the limit for role- and profile-based technician accounts.
WITH ranked AS (
  SELECT
    bm.user_id,
    bm.branch_id,
    row_number() OVER (PARTITION BY bm.user_id ORDER BY bm.branch_id) AS position
  FROM identity.branch_memberships bm
  WHERE EXISTS (
    SELECT 1
    FROM identity.user_roles ur
    WHERE ur.user_id = bm.user_id AND ur.role = 'technician'
  )
  OR EXISTS (
    SELECT 1
    FROM identity.employee_profiles ep
    WHERE ep.user_id = bm.user_id AND ep.is_technician
  )
)
DELETE FROM identity.branch_memberships bm
USING ranked r
WHERE bm.user_id = r.user_id
  AND bm.branch_id = r.branch_id
  AND r.position > 1;

CREATE OR REPLACE FUNCTION identity.enforce_technician_single_branch()
RETURNS trigger
LANGUAGE plpgsql
AS $$
DECLARE
  target_user_id UUID;
  technician BOOLEAN;
  branch_count INTEGER;
BEGIN
  target_user_id := NEW.user_id;

  SELECT
    EXISTS (
      SELECT 1 FROM identity.user_roles
      WHERE user_id = target_user_id AND role = 'technician'
    )
    OR EXISTS (
      SELECT 1 FROM identity.employee_profiles
      WHERE user_id = target_user_id AND is_technician
    )
  INTO technician;

  IF TG_TABLE_NAME = 'user_roles' AND NEW.role = 'technician' THEN
    technician := true;
  ELSIF TG_TABLE_NAME = 'employee_profiles' AND NEW.is_technician THEN
    technician := true;
  END IF;

  IF technician THEN
    SELECT count(*) INTO branch_count
    FROM identity.branch_memberships
    WHERE user_id = target_user_id;

    IF TG_TABLE_NAME = 'branch_memberships' AND NOT EXISTS (
      SELECT 1 FROM identity.branch_memberships
      WHERE user_id = target_user_id AND branch_id = NEW.branch_id
    ) THEN
      branch_count := branch_count + 1;
    END IF;

    IF branch_count > 1 THEN
      RAISE EXCEPTION 'a technician can only be assigned to one branch'
        USING ERRCODE = '23514';
    END IF;
  END IF;

  RETURN NEW;
END;
$$;

DROP TRIGGER IF EXISTS technician_single_branch_membership ON identity.branch_memberships;
CREATE TRIGGER technician_single_branch_membership
BEFORE INSERT OR UPDATE ON identity.branch_memberships
FOR EACH ROW EXECUTE FUNCTION identity.enforce_technician_single_branch();

DROP TRIGGER IF EXISTS technician_single_branch_role ON identity.user_roles;
CREATE TRIGGER technician_single_branch_role
BEFORE INSERT OR UPDATE ON identity.user_roles
FOR EACH ROW EXECUTE FUNCTION identity.enforce_technician_single_branch();

DROP TRIGGER IF EXISTS technician_single_branch_profile ON identity.employee_profiles;
CREATE TRIGGER technician_single_branch_profile
BEFORE INSERT OR UPDATE OF is_technician ON identity.employee_profiles
FOR EACH ROW EXECUTE FUNCTION identity.enforce_technician_single_branch();
