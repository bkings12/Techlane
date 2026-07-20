import { useEffect, useState } from "react";
import { Link, useParams } from "react-router-dom";
import { Badge, EmptyState, ICONS, PageHeader } from "../components/ui";
import { getCustomer, type Customer, type Device, type RepairJob } from "../lib/api";

export function CustomerDetailPage() {
  const { id } = useParams<{ id: string }>();
  const [customer, setCustomer] = useState<Customer | null>(null);
  const [devices, setDevices] = useState<Device[]>([]);
  const [repairs, setRepairs] = useState<RepairJob[]>([]);
  const [error, setError] = useState("");

  useEffect(() => {
    if (!id) return;
    void (async () => {
      try {
        const res = await getCustomer(id);
        setCustomer(res.customer);
        setDevices(res.devices ?? []);
        setRepairs(res.repairs ?? []);
      } catch (e) {
        setError(e instanceof Error ? e.message : "Failed to load customer");
      }
    })();
  }, [id]);

  if (error) {
    return (
      <div>
        <PageHeader title="Customer" subtitle="Detail" />
        <p className="form-error">{error}</p>
      </div>
    );
  }
  if (!customer) {
    return <div className="boot">Loading…</div>;
  }

  return (
    <div>
      <PageHeader
        title={customer.full_name}
        subtitle={[customer.phone, customer.email].filter(Boolean).join(" · ") || "No contact"}
      />
      <p>
        <Link to="/customers">← All customers</Link>
      </p>

      <div className="repair-grid">
        <section className="panel">
          <h3>Devices</h3>
          {devices.length === 0 ? (
            <EmptyState title="No devices" body="Devices appear when registered on intake." icon={ICONS.repairs} />
          ) : (
            <table className="table">
              <thead>
                <tr>
                  <th>Kind</th>
                  <th>Brand / model</th>
                  <th>IMEI</th>
                </tr>
              </thead>
              <tbody>
                {devices.map((d) => (
                  <tr key={d.id}>
                    <td>{d.kind}</td>
                    <td>
                      {[d.brand, d.model].filter(Boolean).join(" ")}
                    </td>
                    <td>{d.imei || "—"}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          )}
        </section>

        <section className="panel">
          <h3>Repairs</h3>
          {repairs.length === 0 ? (
            <EmptyState title="No repairs" body="No jobs for this customer yet." icon={ICONS.repairs} />
          ) : (
            <table className="table">
              <thead>
                <tr>
                  <th>Job</th>
                  <th>Status</th>
                  <th>Problem</th>
                  <th>Warranty</th>
                </tr>
              </thead>
              <tbody>
                {repairs.map((r) => (
                  <tr key={r.id}>
                    <td>
                      <Link to={`/repairs/${r.id}`}>{r.job_code || r.id.slice(0, 8)}</Link>
                    </td>
                    <td>
                      <Badge>{r.status}</Badge>
                    </td>
                    <td>{r.problem_summary}</td>
                    <td>
                      {(r.status === "completed" || r.status === "collected") && (
                        <Link to={`/repairs/${r.id}`}>View warranty</Link>
                      )}
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          )}
        </section>
      </div>
    </div>
  );
}
