import { useEffect } from "react";
import { PortalTabs } from "../components/PortalTabs";
import { initials } from "../format";
import { savedPhone } from "../api";
import { useSession } from "../session";

export function ProfilePage() {
  const { customer, repairs, refreshRepairs, signOut } = useSession();

  useEffect(() => {
    refreshRepairs().catch(() => undefined);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  const displayName = customer?.full_name || customer?.name || "Customer";
  const displayPhone = customer?.phone || savedPhone();

  return (
    <div className="app">
      <main className="shell">
        <div className="topbar">
          <div>
            <h1>Profile</h1>
            <p className="muted">Your signed-in customer account</p>
          </div>
        </div>
        <PortalTabs />

        <section className="section profile-card">
          <div className="profile-avatar">{initials(displayName, displayPhone)}</div>
          <div>
            <h2 style={{ margin: 0 }}>{displayName}</h2>
            <p className="muted" style={{ margin: "0.35rem 0 0" }}>{displayPhone}</p>
          </div>
          <div className="stat-row">
            <div className="stat-tile">
              <span>Open repairs</span>
              <strong>{repairs.filter((r) => !["collected", "cancelled"].includes(r.status)).length}</strong>
            </div>
            <div className="stat-tile">
              <span>Total jobs</span>
              <strong>{repairs.length}</strong>
            </div>
          </div>
          <button className="btn btn-secondary" onClick={() => void signOut()}>
            Sign out
          </button>
        </section>
      </main>
    </div>
  );
}
