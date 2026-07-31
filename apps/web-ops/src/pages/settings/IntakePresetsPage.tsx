import { useCallback, useEffect, useState, type FormEvent } from "react";
import { useAuth } from "../../auth/AuthContext";
import { Badge, Button, EmptyState, Input, PageHeader } from "../../components/ui";
import {
  createIntakePreset,
  deleteIntakePreset,
  listIntakePresets,
  updateIntakePreset,
  type IntakePreset,
} from "../../lib/api";

function can(permissions: string[] | undefined, permission: string) {
  if (!permissions?.length) return false;
  if (permissions.includes("*")) return true;
  return permissions.includes(permission);
}

type Kind = "condition_tag" | "issue";

function PresetSection({
  kind,
  title,
  hint,
  canWrite,
}: {
  kind: Kind;
  title: string;
  hint: string;
  canWrite: boolean;
}) {
  const [items, setItems] = useState<IntakePreset[]>([]);
  const [label, setLabel] = useState("");
  const [error, setError] = useState("");
  const [busy, setBusy] = useState(false);

  const refresh = useCallback(async () => {
    const res = await listIntakePresets(kind, { includeInactive: true });
    setItems(res.items ?? []);
  }, [kind]);

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

  async function onAdd(e: FormEvent) {
    e.preventDefault();
    const trimmed = label.trim();
    if (!trimmed || !canWrite) return;
    await run(() => createIntakePreset({ kind, label: trimmed }));
    setLabel("");
  }

  return (
    <section className="settings-form-card">
      <div className="panel-head" style={{ marginBottom: "0.75rem" }}>
        <div>
          <h2>{title}</h2>
          <p className="muted" style={{ margin: 0 }}>
            {hint}
          </p>
        </div>
      </div>

      {error ? <p className="form-error">{error}</p> : null}

      {canWrite ? (
        <form className="inline-form" onSubmit={(e) => void onAdd(e)}>
          <label>
            New label
            <Input
              value={label}
              onChange={(e) => setLabel(e.target.value)}
              placeholder={kind === "issue" ? "e.g. Face ID not working" : "e.g. Bent charging port"}
              disabled={busy}
            />
          </label>
          <Button type="submit" disabled={busy || !label.trim()}>
            Add
          </Button>
        </form>
      ) : (
        <p className="hint">You can view these presets, but only managers/owners can edit them.</p>
      )}

      {items.length === 0 ? (
        <EmptyState title="No presets" body="Add a label above, or wait for system defaults to seed." />
      ) : (
        <ul className="settings-roster">
          {items.map((p) => (
            <li key={p.id}>
              <div className={`settings-roster-row${p.is_active ? "" : " is-muted"}`}>
                <span>
                  <strong style={{ opacity: p.is_active ? 1 : 0.55 }}>{p.label}</strong>
                  <span className="muted">sort {p.sort_order}</span>
                </span>
                <span />
                <span className="chip-row">
                  {p.is_system ? <Badge tone="info">Default</Badge> : <Badge tone="pending">Custom</Badge>}
                  {!p.is_active ? <Badge tone="warning">Hidden</Badge> : null}
                </span>
                {canWrite ? (
                  <span className="chip-row">
                    <Button
                      type="button"
                      variant="ghost"
                      disabled={busy}
                      onClick={() => void run(() => updateIntakePreset(p.id, { is_active: !p.is_active }))}
                    >
                      {p.is_active ? "Hide" : "Show"}
                    </Button>
                    {!p.is_system ? (
                      <Button
                        type="button"
                        variant="secondary"
                        disabled={busy}
                        onClick={() => {
                          if (!window.confirm(`Delete “${p.label}”? Past jobs keep this text as stored.`)) return;
                          void run(() => deleteIntakePreset(p.id));
                        }}
                      >
                        Delete
                      </Button>
                    ) : null}
                  </span>
                ) : (
                  <span />
                )}
              </div>
            </li>
          ))}
        </ul>
      )}
    </section>
  );
}

export function IntakePresetsPage() {
  const { user } = useAuth();
  const canWrite = can(user?.permissions, "repairs.presets.write");

  return (
    <div className="settings-page">
      <PageHeader
        title="Intake presets"
        subtitle="Condition tags and common issues shown when taking a device in. Changing these never rewrites past jobs."
      />

      <div className="stack">
        <PresetSection
          kind="condition_tag"
          title="Condition tags"
          hint="Chips staff tap to note device condition at intake."
          canWrite={canWrite}
        />
        <PresetSection
          kind="issue"
          title="Issue presets"
          hint="Quick picks for the problem summary field."
          canWrite={canWrite}
        />
      </div>
    </div>
  );
}
