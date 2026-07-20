import { useCallback, useEffect, useState, type FormEvent } from "react";
import { Link } from "react-router-dom";
import { Avatar, Badge, Button, EmptyState, ICONS, PageHeader, SearchInput } from "../components/ui";
import { getCustomer, listCustomers, type Customer, type Device, type RepairJob } from "../lib/api";

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
    <div>
      <PageHeader title="Customers" subtitle="Search by name or phone" />
      {error ? <p className="form-error">{error}</p> : null}

      <form className="pos-toolbar" onSubmit={onSubmit}>
        <label>
          Search
          <SearchInput value={q} onChange={(e) => setQ(e.target.value)} placeholder="Name or phone" />
        </label>
        <Button type="submit" disabled={busy}>
          {busy ? "Searching…" : "Search"}
        </Button>
      </form>

      <div className="repair-grid">
        <section className="panel">
          {items.length === 0 ? (
            <EmptyState
              title="No customers"
              body="Try a different search, or create one during repair intake."
              icon={ICONS.customers}
            />
          ) : (
            <table className="table">
              <thead>
                <tr>
                  <th>Name</th>
                  <th>Phone</th>
                  <th />
                </tr>
              </thead>
              <tbody>
                {items.map((c) => (
                  <tr key={c.id}>
                    <td>
                      <span className="name-cell">
                        <Avatar name={c.full_name} size={30} />
                        <strong>{c.full_name}</strong>
                      </span>
                    </td>
                    <td>{c.phone ?? "—"}</td>
                    <td>
                      <Link className="button secondary" to={`/customers/${c.id}`}>
                        Open
                      </Link>
                      <Button
                        type="button"
                        variant={selectedId === c.id ? "primary" : "secondary"}
                        disabled={busy}
                        onClick={() => void loadDetail(c.id)}
                      >
                        Preview
                      </Button>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          )}
        </section>

        <section className="panel">
          {!detail ? (
            <EmptyState title="Customer detail" body="Select a customer to view devices and repairs." icon={ICONS.search} />
          ) : (
            <>
              <h2>{detail.customer.full_name}</h2>
              <dl className="meta-dl">
                <dt>Phone</dt>
                <dd>{detail.customer.phone ?? "—"}</dd>
                <dt>Email</dt>
                <dd>{detail.customer.email ?? "—"}</dd>
              </dl>

              <h3>Devices</h3>
              {detail.devices.length === 0 ? (
                <p className="muted">No devices on file.</p>
              ) : (
                <ul className="part-list">
                  {detail.devices.map((d) => (
                    <li key={d.id} className="part-card">
                      <strong>{[d.brand, d.model].filter(Boolean).join(" ") || d.kind}</strong>
                      <div className="muted">{d.kind}</div>
                    </li>
                  ))}
                </ul>
              )}

              <h3>Repairs</h3>
              {detail.repairs.length === 0 ? (
                <p className="muted">No repair jobs.</p>
              ) : (
                <ul className="part-list">
                  {detail.repairs.map((r) => (
                    <li key={r.id} className="part-card">
                      <div className="part-head">
                        <div>
                          <Link to={`/repairs/${r.id}`}>
                            {r.job_code ?? r.id.slice(0, 8)}
                          </Link>
                          <div className="muted">{r.problem_summary}</div>
                        </div>
                        <Badge tone="info">{r.status}</Badge>
                      </div>
                    </li>
                  ))}
                </ul>
              )}
            </>
          )}
        </section>
      </div>
    </div>
  );
}
