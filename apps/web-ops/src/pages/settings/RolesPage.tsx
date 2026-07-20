import { useEffect, useMemo, useState, type FormEvent } from "react";
import { Badge, Button, EmptyState, Input, PageHeader } from "../../components/ui";
import {
  createPermission,
  createRole,
  deleteRole,
  listPermissions,
  listRoles,
  updateRole,
  type PermissionDef,
  type RoleInfo,
} from "../../lib/api";

export function RolesPage() {
  const [roles, setRoles] = useState<RoleInfo[]>([]);
  const [perms, setPerms] = useState<PermissionDef[]>([]);
  const [selectedId, setSelectedId] = useState<string | null>(null);
  const [error, setError] = useState("");
  const [showCreate, setShowCreate] = useState(false);
  const [showPermCreate, setShowPermCreate] = useState(false);

  async function refresh() {
    try {
      const [r, p] = await Promise.all([listRoles(), listPermissions()]);
      setRoles(r.items ?? []);
      setPerms(p.items ?? []);
      if (!selectedId && r.items?.length) setSelectedId(r.items[0].id);
    } catch (e) {
      setError(e instanceof Error ? e.message : "Failed to load");
    }
  }

  useEffect(() => {
    void refresh();
  }, []);

  const selected = useMemo(() => roles.find((r) => r.id === selectedId) ?? null, [roles, selectedId]);

  return (
    <div>
      <PageHeader
        title="Roles & permissions"
        subtitle="Create custom roles and assign permissions from the catalog"
        actions={
          <div className="chip-row">
            <Button type="button" variant="secondary" onClick={() => setShowPermCreate((v) => !v)}>
              Add permission
            </Button>
            <Button type="button" onClick={() => setShowCreate((v) => !v)}>
              New role
            </Button>
          </div>
        }
      />
      {error ? <p className="form-error">{error}</p> : null}
      {showPermCreate ? (
        <CreatePermissionForm
          onCreated={() => {
            setShowPermCreate(false);
            void refresh();
          }}
        />
      ) : null}
      {showCreate ? (
        <CreateRoleForm
          permissions={perms}
          onCreated={(id) => {
            setShowCreate(false);
            setSelectedId(id);
            void refresh();
          }}
        />
      ) : null}

      <div className="roles-layout">
        <section className="panel">
          <h2>Roles</h2>
          {roles.length === 0 ? (
            <EmptyState title="No roles" body="System roles will appear after API restart." />
          ) : (
            <ul className="list">
              {roles.map((r) => (
                <li key={r.id}>
                  <button
                    type="button"
                    className={`role-pick ${selectedId === r.id ? "active" : ""}`}
                    onClick={() => setSelectedId(r.id)}
                  >
                    <strong>{r.name}</strong>
                    <span className="muted mono">{r.key}</span>
                    {r.is_system ? <Badge tone="info">system</Badge> : <Badge tone="pending">custom</Badge>}
                  </button>
                </li>
              ))}
            </ul>
          )}
        </section>

        <section className="panel">
          {selected ? (
            <EditRolePanel
              role={selected}
              catalog={perms}
              onSaved={() => void refresh()}
              onDeleted={() => {
                setSelectedId(null);
                void refresh();
              }}
              onError={setError}
            />
          ) : (
            <EmptyState title="Select a role" body="Choose a role to edit its permissions." />
          )}
        </section>
      </div>
    </div>
  );
}

function CreatePermissionForm({ onCreated }: { onCreated: () => void }) {
  const [code, setCode] = useState("");
  const [description, setDescription] = useState("");
  const [category, setCategory] = useState("custom");
  const [busy, setBusy] = useState(false);
  const [err, setErr] = useState("");

  async function submit(e: FormEvent) {
    e.preventDefault();
    setBusy(true);
    setErr("");
    try {
      await createPermission({ code, description, category });
      onCreated();
    } catch (error) {
      setErr(error instanceof Error ? error.message : "Failed");
    } finally {
      setBusy(false);
    }
  }

  return (
    <form className="panel form-grid" onSubmit={submit}>
      <h2>New permission</h2>
      <label>
        Code
        <Input className="mono" value={code} onChange={(e) => setCode(e.target.value)} placeholder="reports.export" required />
      </label>
      <label>
        Description
        <Input value={description} onChange={(e) => setDescription(e.target.value)} />
      </label>
      <label>
        Category
        <Input value={category} onChange={(e) => setCategory(e.target.value)} />
      </label>
      {err ? <p className="form-error">{err}</p> : null}
      <Button type="submit" disabled={busy}>
        {busy ? "Saving…" : "Create permission"}
      </Button>
    </form>
  );
}

function CreateRoleForm({
  permissions,
  onCreated,
}: {
  permissions: PermissionDef[];
  onCreated: (id: string) => void;
}) {
  const [key, setKey] = useState("");
  const [name, setName] = useState("");
  const [description, setDescription] = useState("");
  const [selected, setSelected] = useState<string[]>([]);
  const [busy, setBusy] = useState(false);
  const [err, setErr] = useState("");

  function toggle(code: string) {
    setSelected((prev) => (prev.includes(code) ? prev.filter((c) => c !== code) : [...prev, code]));
  }

  async function submit(e: FormEvent) {
    e.preventDefault();
    setBusy(true);
    setErr("");
    try {
      const role = await createRole({ key, name, description, permissions: selected });
      onCreated(role.id);
    } catch (error) {
      setErr(error instanceof Error ? error.message : "Failed");
    } finally {
      setBusy(false);
    }
  }

  const byCategory = groupByCategory(permissions);

  return (
    <form className="panel form-grid" onSubmit={submit}>
      <h2>New custom role</h2>
      <label>
        Key
        <Input className="mono" value={key} onChange={(e) => setKey(e.target.value)} placeholder="senior_tech" required />
      </label>
      <label>
        Display name
        <Input value={name} onChange={(e) => setName(e.target.value)} placeholder="Senior Technician" required />
      </label>
      <label>
        Description
        <Input value={description} onChange={(e) => setDescription(e.target.value)} />
      </label>
      <PermissionPicker byCategory={byCategory} selected={selected} onToggle={toggle} />
      {err ? <p className="form-error">{err}</p> : null}
      <Button type="submit" disabled={busy || !key || selected.length === 0}>
        {busy ? "Creating…" : "Create role"}
      </Button>
    </form>
  );
}

function EditRolePanel({
  role,
  catalog,
  onSaved,
  onDeleted,
  onError,
}: {
  role: RoleInfo;
  catalog: PermissionDef[];
  onSaved: () => void;
  onDeleted: () => void;
  onError: (msg: string) => void;
}) {
  const [name, setName] = useState(role.name);
  const [description, setDescription] = useState(role.description ?? "");
  const [selected, setSelected] = useState<string[]>(role.permissions.filter((p) => p !== "*"));
  const [busy, setBusy] = useState(false);

  useEffect(() => {
    setName(role.name);
    setDescription(role.description ?? "");
    setSelected(role.permissions.filter((p) => p !== "*"));
  }, [role]);

  const byCategory = groupByCategory(catalog);
  const lockedOwner = role.key === "owner";

  async function save(e: FormEvent) {
    e.preventDefault();
    setBusy(true);
    try {
      await updateRole(role.id, {
        name,
        description,
        permissions: lockedOwner ? undefined : selected,
      });
      onSaved();
    } catch (err) {
      onError(err instanceof Error ? err.message : "Save failed");
    } finally {
      setBusy(false);
    }
  }

  async function remove() {
    if (!confirm(`Delete role “${role.name}”?`)) return;
    setBusy(true);
    try {
      await deleteRole(role.id);
      onDeleted();
    } catch (err) {
      onError(err instanceof Error ? err.message : "Delete failed");
    } finally {
      setBusy(false);
    }
  }

  return (
    <form className="form-grid" onSubmit={save}>
      <div className="chip-row">
        <h2 style={{ margin: 0 }}>{role.name}</h2>
        {role.is_system ? <Badge tone="info">system</Badge> : <Badge tone="pending">custom</Badge>}
      </div>
      <p className="muted mono">{role.key}</p>
      <label>
        Display name
        <Input value={name} onChange={(e) => setName(e.target.value)} required />
      </label>
      <label>
        Description
        <Input value={description} onChange={(e) => setDescription(e.target.value)} />
      </label>
      {lockedOwner ? (
        <p className="hint">Owner always has full access (*). Permissions cannot be reduced.</p>
      ) : (
        <PermissionPicker
          byCategory={byCategory}
          selected={selected}
          onToggle={(code) =>
            setSelected((prev) => (prev.includes(code) ? prev.filter((c) => c !== code) : [...prev, code]))
          }
        />
      )}
      <div className="chip-row">
        <Button type="submit" disabled={busy}>
          {busy ? "Saving…" : "Save role"}
        </Button>
        {!role.is_system ? (
          <Button type="button" variant="secondary" disabled={busy} onClick={() => void remove()}>
            Delete
          </Button>
        ) : null}
      </div>
    </form>
  );
}

function PermissionPicker({
  byCategory,
  selected,
  onToggle,
}: {
  byCategory: Record<string, PermissionDef[]>;
  selected: string[];
  onToggle: (code: string) => void;
}) {
  return (
    <div className="perm-groups">
      {Object.entries(byCategory).map(([cat, items]) => (
        <fieldset key={cat} className="fieldset">
          <legend>{cat}</legend>
          <div className="chip-row">
            {items.map((p) => (
              <label key={p.code} className="check-chip" title={p.description}>
                <input type="checkbox" checked={selected.includes(p.code)} onChange={() => onToggle(p.code)} />
                <span className="mono">{p.code}</span>
              </label>
            ))}
          </div>
        </fieldset>
      ))}
    </div>
  );
}

function groupByCategory(permissions: PermissionDef[]) {
  return permissions.reduce<Record<string, PermissionDef[]>>((acc, p) => {
    const cat = p.category || "general";
    (acc[cat] ??= []).push(p);
    return acc;
  }, {});
}
