import { NavLink } from "react-router-dom";

export function PortalTabs() {
  return (
    <nav className="tabs">
      <NavLink to="/" end className={({ isActive }) => (isActive ? "active" : "")}>
        Repairs
      </NavLink>
      <NavLink to="/profile" className={({ isActive }) => (isActive ? "active" : "")}>
        Profile
      </NavLink>
      <NavLink to="/guest" className={({ isActive }) => (isActive ? "active" : "")}>
        Guest lookup
      </NavLink>
    </nav>
  );
}
