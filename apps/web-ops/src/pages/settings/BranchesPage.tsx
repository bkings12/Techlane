import { useCallback, useEffect, useState, type FormEvent } from "react";
import { Badge, Button, EmptyState, Input, PageHeader } from "../../components/ui";
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
  const [location, setLocation] = useState("");
  const [phone, setPhone] = useState("");
  const [hours, setHours] = useState("");
  const [mapUrl, setMapUrl] = useState("");
  const [editId, setEditId] = useState<string | null>(null);
  const [editName, setEditName] = useState("");
  const [editCode, setEditCode] = useState("");
  const [editLocation, setEditLocation] = useState("");
  const [editPhone, setEditPhone] = useState("");
  const [editHours, setEditHours] = useState("");
  const [editMapUrl, setEditMapUrl] = useState("");

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
    setEditLocation(b.location ?? "");
    setEditPhone(b.phone ?? "");
    setEditHours(b.hours ?? "");
    setEditMapUrl(b.map_url ?? "");
  }

  async function submitCreate(e: FormEvent) {
    e.preventDefault();
    if (!name.trim() || !code.trim()) return;
    await run(() =>
      createBranch({
        name: name.trim(),
        code: code.trim(),
        location: location.trim() || undefined,
        phone: phone.trim() || undefined,
        hours: hours.trim() || undefined,
        map_url: mapUrl.trim() || undefined,
      }),
    );
    setName("");
    setCode("");
    setLocation("");
    setPhone("");
    setHours("");
    setMapUrl("");
  }

  async function submitEdit(e: FormEvent) {
    e.preventDefault();
    if (!editId) return;
    await run(() =>
      updateBranch(editId, {
        name: editName.trim() || undefined,
        code: editCode.trim() || undefined,
        location: editLocation.trim(),
        phone: editPhone.trim(),
        hours: editHours.trim(),
        map_url: editMapUrl.trim(),
      }),
    );
    setEditId(null);
  }

  return (
    <div className="settings-page">
      <PageHeader
        title="Branches"
        subtitle="Create and manage shop locations — location appears on customer SMS"
      />
      {error ? <p className="form-error">{error}</p> : null}

      <section className="board-pulse" aria-label="Branches pulse">
        <div>
          <strong>{items.length}</strong>
          <span>Locations</span>
        </div>
      </section>

      <form className="settings-form-card form-grid" onSubmit={submitCreate}>
        <h2>New branch</h2>
        <label>
          Name
          <Input value={name} onChange={(e) => setName(e.target.value)} required />
        </label>
        <label>
          Code
          <Input value={code} onChange={(e) => setCode(e.target.value)} required className="mono" />
        </label>
        <label className="branch-location-field">
          Location (for SMS)
          <Input
            value={location}
            onChange={(e) => setLocation(e.target.value)}
            placeholder="e.g. Westlands, along Waiyaki Way"
          />
        </label>
        <p className="hint">Customer SMS will say “See you at ShopName &amp; location”.</p>
        <div className="field-pair">
          <label>
            Phone (store locator)
            <Input value={phone} onChange={(e) => setPhone(e.target.value)} placeholder="+254 700 000000" />
          </label>
          <label>
            Hours (store locator)
            <Input value={hours} onChange={(e) => setHours(e.target.value)} placeholder="Mon–Sat, 8am–6pm" />
          </label>
        </div>
        <label>
          Map link (store locator)
          <Input value={mapUrl} onChange={(e) => setMapUrl(e.target.value)} placeholder="https://maps.google.com/..." />
        </label>
        <Button type="submit" disabled={busy}>
          Create
        </Button>
      </form>

      {items.length === 0 ? (
        <EmptyState title="No branches" body="Create the first branch above." />
      ) : (
        <ul className="settings-roster">
          {items.map((b) => (
            <li key={b.id}>
              {editId === b.id ? (
                <form className="settings-form-card form-grid" onSubmit={submitEdit}>
                  <label>
                    Name
                    <Input value={editName} onChange={(e) => setEditName(e.target.value)} required />
                  </label>
                  <label>
                    Code
                    <Input value={editCode} onChange={(e) => setEditCode(e.target.value)} required className="mono" />
                  </label>
                  <label className="branch-location-field">
                    Location (for SMS)
                    <Input
                      value={editLocation}
                      onChange={(e) => setEditLocation(e.target.value)}
                      placeholder="e.g. Westlands, along Waiyaki Way"
                    />
                  </label>
                  <div className="field-pair">
                    <label>
                      Phone (store locator)
                      <Input value={editPhone} onChange={(e) => setEditPhone(e.target.value)} placeholder="+254 700 000000" />
                    </label>
                    <label>
                      Hours (store locator)
                      <Input value={editHours} onChange={(e) => setEditHours(e.target.value)} placeholder="Mon–Sat, 8am–6pm" />
                    </label>
                  </div>
                  <label>
                    Map link (store locator)
                    <Input value={editMapUrl} onChange={(e) => setEditMapUrl(e.target.value)} placeholder="https://maps.google.com/..." />
                  </label>
                  <div className="chip-row">
                    <Button type="submit" disabled={busy}>
                      Save
                    </Button>
                    <Button type="button" variant="ghost" onClick={() => setEditId(null)}>
                      Cancel
                    </Button>
                  </div>
                </form>
              ) : (
                <div className="settings-roster-row">
                  <span>
                    <strong>{b.name}</strong>
                    <div className="muted mono">{b.code}</div>
                    {b.location ? <div className="muted">{b.location}</div> : (
                      <div className="muted">No location set — SMS will use branch name</div>
                    )}
                  </span>
                  <Badge tone="info">branch</Badge>
                  <span />
                  <span className="chip-row">
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
                  </span>
                </div>
              )}
            </li>
          ))}
        </ul>
      )}
    </div>
  );
}
