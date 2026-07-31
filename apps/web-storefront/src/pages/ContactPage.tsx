import { Link } from "react-router-dom";
import { useStorefront } from "../store/StorefrontContext";

export function ContactPage() {
  const { content, boot, branches } = useStorefront();
  const s = content?.settings;
  const shopName = (s?.shop_display_name || boot?.tenant_name || "Shop").trim() || "Shop";
  const phone = s?.contact_phone || s?.topbar_phone_label;
  const email = s?.contact_email;
  const hours = s?.business_hours;

  return (
    <section className="wide-page contact-page">
      <div className="page-header">
        <h1>Contact</h1>
        <ol className="breadcrumb">
          <li>
            <Link to="/">Home</Link>
          </li>
          <li className="active">Contact</li>
        </ol>
      </div>

      <div className="li-container page-body">
        <p className="muted" style={{ marginTop: 0 }}>
          Reach {shopName} or visit a branch for pickup and support.
        </p>

        <div className="contact-grid">
          <div className="contact-card">
            <h2>Get in touch</h2>
            {phone ? (
              <p>
                <strong>Phone</strong>
                <br />
                <a href={`tel:${phone.replace(/\s+/g, "")}`}>{phone}</a>
              </p>
            ) : null}
            {email ? (
              <p>
                <strong>Email</strong>
                <br />
                <a href={`mailto:${email}`}>{email}</a>
              </p>
            ) : null}
            {hours ? (
              <p>
                <strong>Hours</strong>
                <br />
                <span className="muted">{hours}</span>
              </p>
            ) : null}
            {!phone && !email && !hours ? (
              <p className="muted">Contact details will appear here once the shop owner adds them in settings.</p>
            ) : null}
            <div className="stack" style={{ justifyContent: "flex-start", marginTop: "1rem" }}>
              <Link className="btn" to="/stores">
                Store locator
              </Link>
              <Link className="btn btn-ghost" to="/lookup">
                Track an order
              </Link>
            </div>
          </div>

          <div className="contact-card">
            <h2>Branches</h2>
            {(branches?.length ? branches : []).length === 0 ? (
              <p className="muted">No branches listed yet.</p>
            ) : (
              <ul className="contact-branch-list">
                {branches.map((b) => (
                  <li key={b.id}>
                    <strong>{b.name}</strong>
                    {b.address ? <div className="muted">{b.address}</div> : null}
                    {b.phone ? <div className="muted">{b.phone}</div> : null}
                    {b.hours ? <div className="muted tiny">{b.hours}</div> : null}
                    {b.map_url ? (
                      <a href={b.map_url} target="_blank" rel="noreferrer">
                        Open map
                      </a>
                    ) : null}
                  </li>
                ))}
              </ul>
            )}
          </div>
        </div>
      </div>
    </section>
  );
}
