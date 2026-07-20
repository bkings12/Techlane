import { useCallback, useEffect, useState, type ReactNode } from "react";
import { NavLink, Outlet, useNavigate } from "react-router-dom";
import { useAuth } from "../auth/AuthContext";
import { useBranch } from "../branch/BranchContext";
import { Button } from "../components/ui";
import { listNotifications } from "../lib/api";

function Icon({ d, extra, width = 20, height = 20 }: { d: string; extra?: ReactNode; width?: number; height?: number }) {
  return (
    <svg
      width={width}
      height={height}
      viewBox="0 0 24 24"
      fill="none"
      stroke="currentColor"
      strokeWidth="1.9"
      strokeLinecap="round"
      strokeLinejoin="round"
      aria-hidden="true"
    >
      <path d={d} />
      {extra}
    </svg>
  );
}

const Logo = () => (
  <svg width="28" height="28" viewBox="0 0 32 32" fill="none" aria-hidden="true">
    <rect width="32" height="32" rx="7" fill="url(#logoGradient)" />
    <rect x="6" y="11" width="3.4" height="12.5" rx="1.7" fill="#F2BE2A" />
    <rect x="12.3" y="8.5" width="3.4" height="17.5" rx="1.7" fill="#FFFFFF" />
    <rect x="18.6" y="12.5" width="3.4" height="9" rx="1.7" fill="#F2BE2A" />
    <defs>
      <linearGradient id="logoGradient" x1="4" y1="2" x2="28" y2="30" gradientUnits="userSpaceOnUse">
        <stop stopColor="#100d8b" />
        <stop offset="1" stopColor="#040257" />
      </linearGradient>
    </defs>
  </svg>
);

const RAIL_TOP = [
  { to: "/", label: "Home", icon: <Icon d="m3 9 9-7 9 7v11a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2z" extra={<path d="M9 22V12h6v10" />} /> },
  { to: "/reports", label: "Reports", icon: <Icon d="M3 3v18h18" extra={<path d="m19 9-5 5-4-4-3 3" />} /> },
  { to: "/audit", label: "Audit", icon: <Icon d="M14 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V8z" extra={<path d="M14 2v6h6" />} /> },
];

const RAIL_BOTTOM = [
  { to: "/settings", label: "Settings", icon: <Icon d="M12.22 2h-.44a2 2 0 0 0-2 2v.18a2 2 0 0 1-1 1.73l-.43.25a2 2 0 0 1-2 0l-.15-.08a2 2 0 0 0-2.73.73l-.22.38a2 2 0 0 0 .73 2.73l.15.1a2 2 0 0 1 1 1.72v.51a2 2 0 0 1-1 1.74l-.15.09a2 2 0 0 0-.73 2.73l.22.38a2 2 0 0 0 2.73.73l.15-.08a2 2 0 0 1 2 0l.43.25a2 2 0 0 1 1 1.73V20a2 2 0 0 0 2 2h.44a2 2 0 0 0 2-2v-.18a2 2 0 0 1 1-1.73l.43-.25a2 2 0 0 1 2 0l.15.08a2 2 0 0 0 2.73-.73l.22-.39a2 2 0 0 0-.73-2.73l-.15-.08a2 2 0 0 1-1-1.74v-.5a2 2 0 0 1 1-1.74l.15-.09a2 2 0 0 0 .73-2.73l-.22-.38a2 2 0 0 0-2.73-.73l-.15.08a2 2 0 0 1-2 0l-.43-.25a2 2 0 0 1-1-1.73V4a2 2 0 0 0-2-2z" extra={<circle cx="12" cy="12" r="3" />} /> },
];

const NAV_GROUPS = [
  {
    id: "work",
    label: "Work",
    items: [
      { to: "/repairs", label: "Repairs", roles: ["owner", "manager", "technician", "cashier"] },
      { to: "/customers", label: "Customers", roles: ["owner", "manager", "technician", "cashier"] },
      { to: "/parts", label: "Part requests", roles: ["owner", "manager", "technician", "inventory"] },
      { to: "/inventory", label: "Inventory", roles: ["owner", "manager", "inventory"] },
      { to: "/suppliers", label: "Suppliers", roles: ["owner", "manager", "inventory", "accountant"] },
    ],
  },
  {
    id: "money",
    label: "Money",
    items: [
      { to: "/pos", label: "POS", roles: ["owner", "manager", "cashier"] },
      { to: "/orders", label: "Orders", roles: ["owner", "manager", "cashier"] },
      { to: "/payments", label: "Payments", roles: ["owner", "manager", "cashier", "accountant"] },
    ],
  },
  {
    id: "oversight",
    label: "Oversight",
    items: [
      { to: "/risk", label: "Risk", roles: ["owner", "manager", "accountant"] },
      { to: "/reports", label: "Reports", roles: ["owner", "manager", "accountant"] },
      { to: "/sync", label: "Sync", roles: ["owner", "manager"] },
    ],
  },
];

const SECONDARY_NAV = [
  { to: "/audit", label: "Audit & Alerts", roles: ["owner", "manager", "accountant"] },
];

type NavItem = { to: string; label: string; roles: string[] };

function canSee(item: NavItem, roles: string[] | undefined, primaryRole: string) {
  if (primaryRole === "owner") return true;
  return roles?.some((r) => item.roles.includes(r)) ?? false;
}

const icons: Record<string, ReactNode> = {
  "/repairs": <Icon d="M14.7 6.3a1 1 0 0 0 0 1.4l1.6 1.6a1 1 0 0 0 1.4 0l3.77-3.77a6 6 0 0 1-7.94 7.94l-6.91 6.91a2.12 2.12 0 0 1-3-3l6.91-6.91a6 6 0 0 1 7.94-7.94l-3.76 3.76z" />,
  "/customers": <Icon d="M16 21v-2a4 4 0 0 0-4-4H6a4 4 0 0 0-4 4v2" extra={<><circle cx="9" cy="7" r="4" /><path d="M22 21v-2a4 4 0 0 0-3-3.87M16 3.13a4 4 0 0 1 0 7.75" /></>} />,
  "/parts": <Icon d="M21 16V8a2 2 0 0 0-1-1.73l-7-4a2 2 0 0 0-2 0l-7 4A2 2 0 0 0 3 8v8a2 2 0 0 0 1 1.73l7 4a2 2 0 0 0 2 0l7-4A2 2 0 0 0 21 16z" extra={<path d="m3.29 7 8.71 5 8.71-5M12 22V12" />} />,
  "/inventory": <Icon d="M22 8.35V20a2 2 0 0 1-2 2H4a2 2 0 0 1-2-2V8.35A2 2 0 0 1 3.26 6.5l8-3.2a2 2 0 0 1 1.48 0l8 3.2A2 2 0 0 1 22 8.35z" />,
  "/suppliers": <Icon d="M5 8h14M5 8a2 2 0 1 0 0-4h14a2 2 0 1 0 0 4M5 8v10a2 2 0 0 0 2 2h10a2 2 0 0 0 2-2V8" />,
  "/pos": <Icon d="M6 2 3 6v14a2 2 0 0 0 2 2h14a2 2 0 0 0 2-2V6l-3-4z" extra={<path d="M3 6h18m-5 4a4 4 0 0 1-8 0" />} />,
  "/orders": <Icon d="M6 2 3 6v14a2 2 0 0 0 2 2h14a2 2 0 0 0 2-2V6l-3-4z" extra={<path d="M3 6h18m-5 4a4 4 0 0 1-8 0" />} />,
  "/payments": <Icon d="M2 7h20v10H2z" extra={<circle cx="12" cy="12" r="2.5" />} />,
  "/risk": <Icon d="m21.73 18-8-14a2 2 0 0 0-3.48 0l-8 14A2 2 0 0 0 4 21h16a2 2 0 0 0 1.73-3z" extra={<path d="M12 9v4m0 4h.01" />} />,
  "/reports": <Icon d="M3 3v18h18" extra={<path d="m19 9-5 5-4-4-3 3" />} />,
  "/audit": <Icon d="M14 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V8z" extra={<path d="M14 2v6h6M9 13h6M9 17h6" />} />,
  "/sync": <Icon d="M21 12a9 9 0 0 0-9-9 9.75 9.75 0 0 0-6.74 2.74L3 8" extra={<path d="M3 3v5h5m-5 4a9 9 0 0 0 9 9 9.75 9.75 0 0 0 6.74-2.74L21 16m0 5v-5h-5" />} />,
  "/settings": <Icon d="M12.22 2h-.44a2 2 0 0 0-2 2v.18a2 2 0 0 1-1 1.73l-.43.25a2 2 0 0 1-2 0l-.15-.08a2 2 0 0 0-2.73.73l-.22.38a2 2 0 0 0 .73 2.73l.15.1a2 2 0 0 1 1 1.72v.51a2 2 0 0 1-1 1.74l-.15.09a2 2 0 0 0-.73 2.73l.22.38a2 2 0 0 0 2.73.73l.15-.08a2 2 0 0 1 2 0l.43.25a2 2 0 0 1 1 1.73V20a2 2 0 0 0 2 2h.44a2 2 0 0 0 2-2v-.18a2 2 0 0 1 1-1.73l.43-.25a2 2 0 0 1 2 0l.15.08a2 2 0 0 0 2.73-.73l.22-.39a2 2 0 0 0-.73-2.73l-.15-.08a2 2 0 0 1-1-1.74v-.5a2 2 0 0 1 1-1.74l.15-.09a2 2 0 0 0 .73-2.73l-.22-.38a2 2 0 0 0-2.73-.73l-.15.08a2 2 0 0 1-2 0l-.43-.25a2 2 0 0 1-1-1.73V4a2 2 0 0 0-2-2z" extra={<circle cx="12" cy="12" r="3" />} />,
};

function useTheme() {
  const [theme, setTheme] = useState<"light" | "dark">(() => {
    if (typeof document === "undefined") return "light";
    return (document.documentElement.getAttribute("data-theme") as "light" | "dark") || "light";
  });

  useEffect(() => {
    document.documentElement.setAttribute("data-theme", theme);
  }, [theme]);

  return { theme, setTheme };
}

export function AppShell() {
  const { user, logout, primaryRole } = useAuth();
  const { branches, branchId, setBranchId } = useBranch();
  const navigate = useNavigate();
  const { theme, setTheme } = useTheme();
  const [showBranchPicker] = useState(() => (user?.branch_ids?.length ?? 0) > 1 || branches.length > 1);
  const [mobileOpen, setMobileOpen] = useState(false);
  const [unackedCount, setUnackedCount] = useState(0);

  const refreshNotifications = useCallback(async () => {
    if (!user) {
      setUnackedCount(0);
      return;
    }
    try {
      const res = await listNotifications(true);
      setUnackedCount((res.items ?? []).filter((n) => !n.acked_at).length);
    } catch {
      setUnackedCount(0);
    }
  }, [user]);

  useEffect(() => {
    void refreshNotifications();
    const id = window.setInterval(() => void refreshNotifications(), 60_000);
    return () => window.clearInterval(id);
  }, [refreshNotifications]);

  const groups = NAV_GROUPS.map((group) => ({
    ...group,
    items: group.items.filter((item) => canSee(item, user?.roles, primaryRole)),
  })).filter((group) => group.items.length > 0);

  const secondary = SECONDARY_NAV.filter((item) => canSee(item, user?.roles, primaryRole));

  const initials = user?.display_name
    ?.split(" ")
    .map((p) => p[0])
    .slice(0, 2)
    .join("")
    .toUpperCase() || "?";

  return (
    <div className="shell">
      <aside className="sidebar" data-mobile-open={mobileOpen}>
        <div className="sidebar-rail">
          <div className="rail-brand">
            <Logo />
          </div>
          <div className="rail-top">
            <button
              type="button"
              className="rail-icon mobile-menu-toggle"
              aria-label={mobileOpen ? "Close menu" : "Open menu"}
              onClick={() => setMobileOpen((v) => !v)}
            >
              <Icon d={mobileOpen ? "M18 6 6 18M6 6l12 12" : "M4 12h16M4 6h16M4 18h16"} />
            </button>
            {RAIL_TOP.map((item) => (
              <NavLink
                key={item.to}
                to={item.to}
                end={item.to === "/"}
                className={({ isActive }) => `rail-icon ${isActive ? "active" : ""}`}
                title={item.label}
              >
                {item.icon}
              </NavLink>
            ))}
          </div>
          <div className="rail-bottom">
            <div className="rail-divider" />
            {RAIL_BOTTOM.map((item) => (
              <NavLink
                key={item.to}
                to={item.to}
                className={({ isActive }) => `rail-icon ${isActive ? "active" : ""}`}
                title={item.label}
              >
                {item.icon}
              </NavLink>
            ))}
            <NavLink
              to="/notifications"
              className={({ isActive }) => `rail-icon ${isActive ? "active" : ""}`}
              title={unackedCount > 0 ? `${unackedCount} unread notifications` : "Notifications"}
              onClick={() => setMobileOpen(false)}
            >
              <Icon d="M6 8a6 6 0 0 1 12 0c0 7 3 9 3 9H3s3-2 3-9" extra={<path d="M10.3 21a1.94 1.94 0 0 0 3.4 0" />} />
              {unackedCount > 0 ? <span className="rail-badge">{unackedCount > 9 ? "9+" : unackedCount}</span> : null}
            </NavLink>
            <div className="theme-toggle" role="group" aria-label="Theme">
              <button
                type="button"
                className={theme === "light" ? "active" : ""}
                onClick={() => setTheme("light")}
                aria-label="Light mode"
                title="Light mode"
              >
                <Icon d="M12 3V2m0 20v-1m9-9h1M2 12h1m15.5-6.5L20 4M4 20l1.5-1.5M20 20l-1.5-1.5M4 4l1.5 1.5M12 7a5 5 0 1 0 0 10 5 5 0 0 0 0-10z" />
              </button>
              <button
                type="button"
                className={theme === "dark" ? "active" : ""}
                onClick={() => setTheme("dark")}
                aria-label="Dark mode"
                title="Dark mode"
              >
                <Icon d="M21 12.79A9 9 0 1 1 11.21 3 7 7 0 0 0 21 12.79z" />
              </button>
            </div>
            <button className="rail-avatar" type="button" onClick={() => logout()} title={`Sign out (${user?.display_name})`}>
              <span>{initials}</span>
            </button>
          </div>
        </div>

        <div className="sidebar-pane">
          <div className="workspace-selector">
            <span>{showBranchPicker ? branches.find((b) => b.id === branchId)?.name || "Select branch" : branches[0]?.name || "TechLane"}</span>
            {showBranchPicker ? (
              <select
                className="workspace-selector-overlay"
                value={branchId}
                onChange={(e) => setBranchId(e.target.value)}
                aria-label="Branch"
              >
                {branches.map((b) => (
                  <option key={b.id} value={b.id}>
                    {b.name}
                  </option>
                ))}
              </select>
            ) : null}
            <span className="workspace-add" aria-hidden="true">
              <Icon d="M12 5v14M5 12h14" width={14} height={14} />
            </span>
          </div>

          <nav className="pane-nav" aria-label="Primary">
            {groups.map((group) => (
              <div key={group.id} className="pane-group">
                <p className="pane-label">{group.label}</p>
                <div className="pane-links">
                  {group.items.map((item) => (
                    <NavLink
                      key={item.to}
                      to={item.to}
                      end={item.to === "/"}
                      className={({ isActive }) => `pane-link ${isActive ? "active" : ""}`}
                      onClick={() => setMobileOpen(false)}
                    >
                      {icons[item.to]}
                      <span>{item.label}</span>
                    </NavLink>
                  ))}
                </div>
              </div>
            ))}
          </nav>

          {secondary.length > 0 ? (
            <div className="pane-secondary">
              <div className="pane-divider" />
              <div className="pane-links">
                {secondary.map((item) => (
                  <NavLink
                    key={item.to}
                    to={item.to}
                    className={({ isActive }) => `pane-link ${isActive ? "active" : ""}`}
                    onClick={() => setMobileOpen(false)}
                  >
                    {icons[item.to]}
                    <span>{item.label}</span>
                  </NavLink>
                ))}
              </div>
            </div>
          ) : null}

          <div className="pane-cta">
            <p>Get Unlimited Access & More!</p>
            <Button type="button" onClick={() => navigate("/settings")}>
              <Icon d="M10.3 4.5a2 2 0 0 1 3.4 0l6.8 11.3A2 2 0 0 1 18.8 19H5.2a2 2 0 0 1-1.7-2.2l6.8-11.3z" width={14} height={14} />
              Upgrade Plan
            </Button>
          </div>
        </div>
      </aside>

      <div className="main">
        <header className="topbar">
          <div className="topbar-meta">
            <strong>{user?.display_name}</strong>
            <span className="topbar-role">{primaryRole}</span>
          </div>
          <Button variant="ghost" onClick={logout}>
            Sign out
          </Button>
        </header>
        <main className="content">
          <Outlet />
        </main>
      </div>
    </div>
  );
}
