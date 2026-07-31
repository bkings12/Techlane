import { Link, useNavigate } from "react-router-dom";
import { useStorefront } from "../store/StorefrontContext";

export function LocatorPage() {
  const { branches, pickup, changeBranch } = useStorefront();
  const navigate = useNavigate();

  return (
    <section className="wide-page locator">
      <div className="page-header">
        <h1>Our stores</h1>
        <ol className="breadcrumb">
          <li>
            <Link to="/">Home</Link>
          </li>
          <li className="active">Stores</li>
        </ol>
      </div>

      <div className="li-container page-body">
        <p className="muted" style={{ marginTop: 0 }}>
          Pick a branch to shop from, or get directions to visit in person.
        </p>

        {branches.length === 0 ? (
          <p className="muted">No branches configured yet.</p>
        ) : (
          <ul className="locator-list">
            {branches.map((b) => {
              const isPickup = pickup?.branch_id === b.id;
              return (
                <li key={b.id} className="locator-card">
                  <h2>{b.name}</h2>
                  {b.address ? <p className="muted">{b.address}</p> : null}
                  {b.phone ? <p className="muted">{b.phone}</p> : null}
                  {b.hours ? <p className="muted">{b.hours}</p> : null}
                  <div className="btn-row">
                    <button
                      type="button"
                      className={isPickup ? "btn" : "btn btn-ghost"}
                      onClick={() => {
                        void changeBranch(b);
                        navigate("/shop");
                      }}
                    >
                      {isPickup ? "Shopping from here" : "Shop from this branch"}
                    </button>
                    {b.map_url ? (
                      <a className="btn btn-ghost" href={b.map_url} target="_blank" rel="noreferrer">
                        Get directions
                      </a>
                    ) : null}
                  </div>
                </li>
              );
            })}
          </ul>
        )}
      </div>
    </section>
  );
}
