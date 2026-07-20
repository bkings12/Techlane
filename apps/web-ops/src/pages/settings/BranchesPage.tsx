import { useCallback, useEffect, useState, type FormEvent } from "react";
import { Link } from "react-router-dom";
import { Button, EmptyState, Input, PageHeader } from "../../components/ui";
import {
  createBranch,
  deleteBranch,
  listBranches,
  updateBranch,
  type Branch,
} from "../../lib/api";

export function BranchesPage() {
  const [items, setItems] = useState<Branch[]>([]);
  const [error, setError] = useState("");
  const [busy, setBusy] = useState(false);
  const [name, setName] = useState("");
  const [code, setCode] = useState("");
  const [editId, setEditId] = useState<string | null>(null);
  const [editName, setEditName] = useState("");
  const [editCode, setEditCode] = useState("");

  const refresh = useCallback(async () => {
    const res = await listBranches();
    setItems(res.items ?? []);
  }, []);

  useEffect(() => {
    refresh().catch((e) => setError(e instanceof Error ? e.message : "Failed to load"));
  }, [refresh]);

  async function run(action: () => Promise<unknown>) {
    setBusy(true);
    setError("");
    try {
      await action();
      await refresh();
    } catch (e) {
      setError(e instanceof Error ? e.message : "Action failed");
    } finally {
      setBusy(false);
    }
  }

  function startEdit(b: Branch) {
    setEditId(b.id);
    setEditName(b.name);
    setEditCode(b.code);
  }

  async function submitCreate(e: FormEvent) {
    e.preventDefault();
    if (!name.trim() || !code.trim()) return;
    await run(() => createBranch({ name: name.trim(), code: code.trim() }));
    setName("");
    setCode("");
  }

  async function submitEdit(e: FormEvent) {
    e.preventDefault();
    if (!editId) return;
    await run(() =>
      updateBranch(editId, {
        name: editName.trim() || undefined,
        code: editCode.trim() || undefined,
      }),
    );
    setEditId(null);
  }

  return (
    <div>
      <PageHeader
        title="Branches"
        subtitle="Create and manage shop locations"
        actions={
          <Link className="btn btn-ghost" to="/settings">
            ← Settings
          </Link>
        }
      />
      {error ? <p className="form-error">{error}</p> : null}

      <section className="panel">
        <h2>New branch</h2>
        <form className="form-grid" onSubmit={submitCreate}>
          <label>
            Name
            <Input value={name} onChange={(e) => setName(e.target.value)} required />
          </label>
          <label>
            Code
            <Input value={code} onChange={(e) => setCode(e.target.value)} required className="mono" />
          </label>
          <Button type="submit" disabled={busy}>
            Create
          </Button>
        </form>
      </section>

      <section className="panel">
        <h2>All branches</h2>
        {items.length === 0 ? (
          <EmptyState title="No branches" body="Create the first branch above." />
        ) : (
          <ul className="part-list">
            {items.map((b) => (
              <li key={b.id} className="part-card">
                {editId === b.id ? (
                  <form className="form-grid" onSubmit={submitEdit}>
                    <label>
                      Name
                      <Input value={editName} onChange={(e) => setEditName(e.target.value)} required />
                    </label>
                    <label>
                      Code
                      <Input value={editCode} onChange={(e) => setEditCode(e.target.value)} required className="mono" />
                    </label>
                    <div className="btn-row">
                      <Button type="submit" disabled={busy}>
                        Save
                      </Button>
                      <Button type="button" variant="ghost" onClick={() => setEditId(null)}>
                        Cancel
                      </Button>
                    </div>
                  </form>
                ) : (
                  <>
                    <div className="part-head">
                      <div>
                        <strong>{b.name}</strong>
                        <div className="muted mono">{b.code}</div>
                      </div>
                      <div className="btn-row">
                        <Button type="button" variant="secondary" disabled={busy} onClick={() => startEdit(b)}>
                          Edit
                        </Button>
                        <Button
                          type="button"
                          variant="ghost"
                          disabled={busy}
                          onClick={() => {
                            if (confirm(`Delete branch "${b.name}"?`)) {
                              void run(() => deleteBranch(b.id));
                            }
                          }}
                        >
                          Delete
                        </Button>
                      </div>
                    </div>
                  </>
                )}
              </li>
            ))}
          </ul>
        )}
      </section>
    </div>
  );
}
