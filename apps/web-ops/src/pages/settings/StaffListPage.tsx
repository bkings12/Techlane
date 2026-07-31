import { useEffect, useState, type FormEvent } from "react";
import { Link } from "react-router-dom";
import { Avatar, Badge, Button, EmptyState, ICONS, Input, PageHeader, PasswordInput } from "../../components/ui";
import {
  createUser,
  listBranches,
  listRoles,
  listUsers,
  type Branch,
  type RoleInfo,
  type StaffUser,
} from "../../lib/api";

export function StaffListPage() {
  const [items, setItems] = useState<StaffUser[]>([]);
  const [branches, setBranches] = useState<Branch[]>([]);
  const [roleOptions, setRoleOptions] = useState<RoleInfo[]>([]);
  const [error, setError] = useState("");
  const [showCreate, setShowCreate] = useState(false);

  async function refresh() {
    try {
      const [u, b, r] = await Promise.all([listUsers(), listBranches(), listRoles()]);
      setItems(u.items ?? []);
      setBranches(b.items ?? []);
      setRoleOptions(r.items ?? []);
    } catch (e) {
      setError(e instanceof Error ? e.message : "Failed to load staff");
    }
  }

  useEffect(() => {
    void refresh();
  }, []);

  const activeCount = items.filter((u) => u.status === "active").length;
  const commissionCount = items.filter((u) => u.profile?.commission_enabled).length;

  return (
    <div className="settings-page">
      <PageHeader
        title="Staff"
        subtitle="Technicians, cashiers, and branch access"
        actions={
          <Button type="button" onClick={() => setShowCreate((v) => !v)}>
            Add staff
          </Button>
        }
      />
      {error ? <p className="form-error">{error}</p> : null}

      <section className="board-pulse" aria-label="Staff pulse">
        <div>
          <strong>{items.length}</strong>
          <span>Total staff</span>
        </div>
        <div>
          <strong>{activeCount}</strong>
          <span>Active</span>
        </div>
        <div>
          <strong>{commissionCount}</strong>
          <span>Commission on</span>
        </div>
      </section>

      {showCreate ? (
        <CreateStaffForm
          branches={branches}
          roleOptions={roleOptions}
          onCreated={() => {
            setShowCreate(false);
            void refresh();
          }}
        />
      ) : null}

      {items.length === 0 ? (
        <EmptyState title="No staff yet" body="Add a technician to start assigning repairs." icon={ICONS.customers} />
      ) : (
        <ul className="settings-roster">
          {items.map((u) => (
            <li key={u.id}>
              <Link to={`/settings/staff/${u.id}`} className="settings-roster-row">
                <span className="name-cell">
                  <Avatar name={u.display_name} size={30} />
                  <span className="name-cell-text">
                    <strong>{u.display_name}</strong>
                    <span className="muted">{u.email}</span>
                  </span>
                </span>
                <span className="chip-row">
                  {u.roles.map((r) => (
                    <Badge key={r} tone="info">
                      {r}
                    </Badge>
                  ))}
                </span>
                <Badge tone={u.status === "active" ? "success" : "warning"}>{u.status}</Badge>
                <span>
                  {u.profile?.commission_enabled ? (
                    <Badge tone="pending">on</Badge>
                  ) : (
                    <span className="muted">off</span>
                  )}
                </span>
              </Link>
            </li>
          ))}
        </ul>
      )}
    </div>
  );
}

function CreateStaffForm({
  branches,
  roleOptions,
  onCreated,
}: {
  branches: Branch[];
  roleOptions: RoleInfo[];
  onCreated: () => void;
}) {
  const [displayName, setDisplayName] = useState("");
  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("password");
  const [roles, setRoles] = useState<string[]>(["technician"]);
  const [branchIds, setBranchIds] = useState<string[]>([]);
  const [busy, setBusy] = useState(false);
  const [err, setErr] = useState("");

  function toggleRole(role: string) {
    setRoles((prev) => {
      const next = prev.includes(role) ? prev.filter((r) => r !== role) : [...prev, role];
      if (next.includes("technician")) {
        setBranchIds((ids) => ids.slice(0, 1));
      }
      return next;
    });
  }

  function toggleBranch(id: string) {
    setBranchIds((prev) => {
      if (roles.includes("technician")) {
        return prev.includes(id) ? [] : [id];
      }
      return prev.includes(id) ? prev.filter((b) => b !== id) : [...prev, id];
    });
  }

  async function submit(e: FormEvent) {
    e.preventDefault();
    setBusy(true);
    setErr("");
    try {
      await createUser({
        email,
        display_name: displayName,
        password,
        roles,
        branch_ids: branchIds,
        is_technician: roles.includes("technician"),
      });
      onCreated();
    } catch (error) {
      setErr(error instanceof Error ? error.message : "Failed");
    } finally {
      setBusy(false);
    }
  }

  return (
    <form className="settings-form-card form-grid" onSubmit={submit}>
      <h2>New staff member</h2>
      <label>
        Display name
        <Input value={displayName} onChange={(e) => setDisplayName(e.target.value)} required />
      </label>
      <label>
        Email
        <Input type="email" value={email} onChange={(e) => setEmail(e.target.value)} required />
      </label>
      <label>
        Temporary password
        <PasswordInput value={password} onChange={(e) => setPassword(e.target.value)} required autoComplete="new-password" />
      </label>
      <fieldset className="fieldset">
        <legend>Roles</legend>
        <div className="chip-row">
          {roleOptions.map((role) => (
            <label key={role.key} className="check-chip">
              <input type="checkbox" checked={roles.includes(role.key)} onChange={() => toggleRole(role.key)} />
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
              <input type="checkbox" checked={branchIds.includes(b.id)} onChange={() => toggleBranch(b.id)} />
              {b.name}
            </label>
          ))}
        </div>
      </fieldset>
      {err ? <p className="form-error">{err}</p> : null}
      <Button
        type="submit"
        disabled={busy || roles.length === 0 || (roles.includes("technician") && branchIds.length !== 1)}
      >
        {busy ? "Creating…" : "Create staff"}
      </Button>
    </form>
  );
}
