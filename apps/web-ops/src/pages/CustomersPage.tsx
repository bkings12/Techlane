import { useCallback, useEffect, useState, type FormEvent } from "react";
import { Link } from "react-router-dom";
import { Avatar, Badge, Button, EmptyState, ICONS, PageHeader, SearchInput } from "../components/ui";
import { getCustomer, listCustomers, type Customer, type Device, type RepairJob } from "../lib/api";
import { statusTone } from "../lib/repairStatus";

export function CustomersPage() {
  const [q, setQ] = useState("");
  const [items, setItems] = useState<Customer[]>([]);
  const [selectedId, setSelectedId] = useState<string | null>(null);
  const [detail, setDetail] = useState<{
    customer: Customer;
    devices: Device[];
    repairs: RepairJob[];
  } | null>(null);
  const [error, setError] = useState("");
  const [busy, setBusy] = useState(false);

  const search = useCallback(async (query: string) => {
    setBusy(true);
    setError("");
    try {
      const res = await listCustomers(query.trim() || undefined);
      setItems(res.items ?? []);
    } catch (e) {
      setError(e instanceof Error ? e.message : "Failed to load");
    } finally {
      setBusy(false);
    }
  }, []);

  const loadDetail = useCallback(async (id: string) => {
    setBusy(true);
    setError("");
    try {
      const res = await getCustomer(id);
      setSelectedId(id);
      setDetail(res);
    } catch (e) {
      setError(e instanceof Error ? e.message : "Failed to load customer");
    } finally {
      setBusy(false);
    }
  }, []);

  useEffect(() => {
    void search("");
  }, [search]);

  function onSubmit(e: FormEvent) {
    e.preventDefault();
    void search(q);
  }

  return (
    <div className="customer-directory">
      <PageHeader
        title="Customers"
        subtitle="Directory on the left — devices and jobs preview on the right."
      />
      {error ? <p className="form-error">{error}</p> : null}

      <form className="pos-toolbar" onSubmit={onSubmit}>
        <label>
          Search
          <SearchInput value={q} onChange={(e) => setQ(e.target.value)} placeholder="Name or phone" />
        </label>
        <Button type="submit" disabled={busy}>
          {busy ? "Searching…" : "Search"}
        </Button>
        <div className="trash-count">{items.length} on file</div>
      </form>

      <div className="directory-layout">
        <section className="panel" style={{ padding: "0.85rem" }}>
          <div className="panel-head">
            <h2>Directory</h2>
          </div>
          {items.length === 0 ? (
            <EmptyState
              title="No customers"
              body="Try a different search, or create one during repair intake."
              icon={ICONS.customers}
            />
          ) : (
            <ul className="customer-board">
              {items.map((c) => (
                <li key={c.id}>
                  <button
                    type="button"
                    className={`customer-row ${selectedId === c.id ? "active" : ""}`}
                    disabled={busy}
                    onClick={() => void loadDetail(c.id)}
                  >
                    <span className="name-cell">
                      <Avatar name={c.full_name} size={34} />
                      <span>
                        <strong>{c.full_name}</strong>
                        <div className="muted">{c.phone ?? "No phone"}</div>
                      </span>
                    </span>
                    <Link
                      className="btn btn-ghost"
                      to={`/customers/${c.id}`}
                      onClick={(e) => e.stopPropagation()}
                    >
                      Open
                    </Link>
                  </button>
                </li>
              ))}
            </ul>
          )}
        </section>

        <aside className="preview-rail" aria-label="Customer preview">
          {!detail ? (
            <EmptyState title="Select a customer" body="Preview devices and repairs without leaving the directory." icon={ICONS.search} />
          ) : (
            <>
              <div>
                <h2 style={{ margin: 0 }}>{detail.customer.full_name}</h2>
                <p className="muted" style={{ margin: "0.35rem 0 0" }}>
                  {[detail.customer.phone, detail.customer.email].filter(Boolean).join(" · ") || "No contact"}
                </p>
                <div className="btn-row" style={{ marginTop: "0.75rem" }}>
                  <Link className="btn btn-primary" to={`/customers/${detail.customer.id}`}>
                    Full dossier
                  </Link>
                </div>
              </div>

              <div>
                <h3>Devices · {detail.devices.length}</h3>
                {detail.devices.length === 0 ? (
                  <p className="muted">No devices on file.</p>
                ) : (
                  <ul className="device-board">
                    {detail.devices.slice(0, 6).map((d) => (
                      <li key={d.id} className="device-tile">
                        <strong>{[d.brand, d.model].filter(Boolean).join(" ") || d.kind}</strong>
                        <span className="muted">{d.kind}{d.imei ? ` · IMEI ${d.imei}` : ""}</span>
                      </li>
                    ))}
                  </ul>
                )}
              </div>

              <div>
                <h3>Repairs · {detail.repairs.length}</h3>
                {detail.repairs.length === 0 ? (
                  <p className="muted">No repair jobs.</p>
                ) : (
                  <ul className="repair-history">
                    {detail.repairs.slice(0, 8).map((r) => (
                      <li key={r.id}>
                        <Link className="repair-history-row" to={`/repairs/${r.id}`}>
                          <span>
                            <strong className="mono">{r.job_code ?? r.id.slice(0, 8)}</strong>
                            <div className="muted">{r.problem_summary}</div>
                          </span>
                          <Badge tone={statusTone(r.status)}>{r.status.replaceAll("_", " ")}</Badge>
                        </Link>
                      </li>
                    ))}
                  </ul>
                )}
              </div>
            </>
          )}
        </aside>
      </div>
    </div>
  );
}
