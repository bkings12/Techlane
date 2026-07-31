import { useEffect, useMemo, useState, type FormEvent } from "react";
import { Link, useParams } from "react-router-dom";
import { Badge, Button, Input, PageHeader } from "../../components/ui";
import {
  getUser,
  listBranches,
  listRoles,
  setUserCommission,
  updateUser,
  type Branch,
  type RoleInfo,
  type StaffUser,
} from "../../lib/api";

export function StaffDetailPage() {
  const { id } = useParams();
  const [user, setUser] = useState<StaffUser | null>(null);
  const [branches, setBranches] = useState<Branch[]>([]);
  const [roleOptions, setRoleOptions] = useState<RoleInfo[]>([]);
  const [error, setError] = useState("");
  const [roles, setRoles] = useState<string[]>([]);
  const [branchIds, setBranchIds] = useState<string[]>([]);
  const [status, setStatus] = useState("active");
  const [enabled, setEnabled] = useState(false);
  const [ctype, setCtype] = useState("none");
  const [percent, setPercent] = useState("10");
  const [fixed, setFixed] = useState("500");
  const [busy, setBusy] = useState(false);

  useEffect(() => {
    if (!id) return;
    Promise.all([getUser(id), listBranches(), listRoles()])
      .then(([u, b, r]) => {
        setUser(u);
        setBranches(b.items ?? []);
        setRoleOptions(r.items ?? []);
        setRoles(u.roles ?? []);
        setBranchIds(u.branch_ids ?? []);
        setStatus(u.status);
        setEnabled(!!u.profile?.commission_enabled);
        setCtype(u.profile?.commission_type || "none");
        if (u.profile?.percent_bps != null) setPercent(String(u.profile.percent_bps / 100));
        if (u.profile?.fixed_amount != null) setFixed(String(u.profile.fixed_amount));
      })
      .catch((e) => setError(e instanceof Error ? e.message : "Failed"));
  }, [id]);

  const preview = useMemo(() => {
    if (!enabled) return "Commissions off — completing a repair will not create pay.";
    if (ctype === "percent_of_job") {
      const p = Number(percent) || 0;
      return `${p}% of KES 5,000 = KES ${((5000 * p) / 100).toFixed(2)}`;
    }
    if (ctype === "fixed_per_job") return `Fixed KES ${Number(fixed) || 0} per completed repair`;
    return "Select a commission type";
  }, [enabled, ctype, percent, fixed]);

  async function saveAccess(e: FormEvent) {
    e.preventDefault();
    if (!id) return;
    setBusy(true);
    setError("");
    try {
      const u = await updateUser(id, {
        roles,
        branch_ids: branchIds,
        status,
        is_technician: roles.includes("technician"),
      });
      setUser(u);
    } catch (err) {
      setError(err instanceof Error ? err.message : "Save failed");
    } finally {
      setBusy(false);
    }
  }

  async function saveCommission(e: FormEvent) {
    e.preventDefault();
    if (!id) return;
    setBusy(true);
    setError("");
    try {
      const body: Record<string, unknown> = {
        commission_enabled: enabled,
        commission_type: enabled ? ctype : "none",
      };
      if (ctype === "percent_of_job") body.percent_bps = Math.round(Number(percent) * 100);
      if (ctype === "fixed_per_job") body.fixed_amount = Number(fixed);
      const u = await setUserCommission(id, body);
      setUser(u);
    } catch (err) {
      setError(err instanceof Error ? err.message : "Commission save failed");
    } finally {
      setBusy(false);
    }
  }

  if (!user && !error) return <div className="boot">Loading…</div>;

  return (
    <div className="settings-page">
      <PageHeader
        title={user?.display_name ?? "Staff"}
        subtitle={user?.email}
        actions={
          <Link to="/settings/staff" className="muted">
            Back to staff
          </Link>
        }
      />
      {error ? <p className="form-error">{error}</p> : null}

      <div className="settings-split">
        <form className="settings-form-card form-grid" onSubmit={saveAccess}>
          <h2>Roles & branches</h2>
          <label>
            Status
            <select className="input" value={status} onChange={(e) => setStatus(e.target.value)}>
              <option value="active">active</option>
              <option value="inactive">inactive</option>
            </select>
          </label>
          <fieldset className="fieldset">
            <legend>Roles</legend>
            <div className="chip-row">
              {roleOptions.map((role) => (
                <label key={role.key} className="check-chip">
                  <input
                    type="checkbox"
                    checked={roles.includes(role.key)}
                    onChange={() => {
                      setRoles((prev) => {
                        const next = prev.includes(role.key)
                          ? prev.filter((r) => r !== role.key)
                          : [...prev, role.key];
                        if (next.includes("technician")) {
                          setBranchIds((ids) => ids.slice(0, 1));
                        }
                        return next;
                      });
                    }}
                  />
                  {role.name}
                </label>
              ))}
            </div>
          </fieldset>
          <fieldset className="fieldset">
            <legend>Branches</legend>
            {roles.includes("technician") ? <p className="hint">Technicians must belong to exactly one shop.</p> : null}
            <div className="chip-row">
              {branches.map((b) => (
                <label key={b.id} className="check-chip">
                  <input
                    type="checkbox"
                    checked={branchIds.includes(b.id)}
                    onChange={() =>
                      setBranchIds((prev) => {
                        if (roles.includes("technician")) {
                          return prev.includes(b.id) ? [] : [b.id];
                        }
                        return prev.includes(b.id) ? prev.filter((x) => x !== b.id) : [...prev, b.id];
                      })
                    }
                  />
                  {b.name}
                </label>
              ))}
            </div>
          </fieldset>
          <Button
            type="submit"
            disabled={busy || (roles.includes("technician") && branchIds.length !== 1)}
          >
            Save access
          </Button>
        </form>

        <form className="settings-form-card form-grid" onSubmit={saveCommission}>
          <h2>Optional commission</h2>
          <label className="check-chip">
            <input type="checkbox" checked={enabled} onChange={(e) => setEnabled(e.target.checked)} />
            Pay commissions for this technician
          </label>
          {enabled ? (
            <>
              <label>
                Type
                <select
                  className="input"
                  value={ctype}
                  onChange={(e) => setCtype(e.target.value)}
                >
                  <option value="percent_of_job">Percent of job labor</option>
                  <option value="fixed_per_job">Fixed amount per job</option>
                </select>
              </label>
              {ctype === "percent_of_job" ? (
                <label>
                  Percent
                  <Input type="number" min={0} max={100} step={0.1} value={percent} onChange={(e) => setPercent(e.target.value)} />
                </label>
              ) : (
                <label>
                  Fixed amount (KES)
                  <Input type="number" min={0} step={1} value={fixed} onChange={(e) => setFixed(e.target.value)} />
                </label>
              )}
            </>
          ) : null}
          <p className="hint">{preview}</p>
          {user?.profile?.commission_enabled ? <Badge tone="pending">Commission active</Badge> : null}
          <Button type="submit" disabled={busy}>
            Save commission
          </Button>
        </form>
      </div>
    </div>
  );
}
