import { useCallback, useEffect, useRef, useState, type ReactNode } from "react";
import { NavLink, Outlet, useLocation, useNavigate } from "react-router-dom";
import { useAuth } from "../auth/AuthContext";
import { useBranch } from "../branch/BranchContext";
import { CommandPalette, useCommandPalette } from "../components/CommandPalette";
import { listNotifications } from "../lib/api";
import { useRealtimeEvents, type RealtimeEvent } from "../lib/realtime";

const LIVE_EVENT_PATTERNS = ["repair.*", "part_request.*", "payment.*", "estimate.*", "commission.*"];

function describeEvent(event: RealtimeEvent): string | null {
  const payload = event.payload ?? {};
  const jobCode = typeof payload.job_code === "string" ? payload.job_code : undefined;
  const ref = jobCode ? `Job ${jobCode}` : "A job";
  switch (event.event_type) {
    case "repair.created":
      return `${ref} was created`;
    case "repair.assigned":
      return `${ref} was assigned to a technician`;
    case "repair.status_changed":
      return `${ref} status changed${typeof payload.status === "string" ? ` to ${payload.status}` : ""}`;
    case "repair.completed":
      return `${ref} was marked complete`;
    case "estimate.pending":
      return `${ref} has an estimate awaiting customer approval`;
    case "part_request.created":
      return "A new part request was created";
    case "payment.confirmed":
      return "A payment was confirmed";
    default:
      return null;
  }
}

function Icon({ d, extra, size = 19 }: { d: string; extra?: ReactNode; size?: number }) {
  return (
    <svg
      width={size}
      height={size}
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

const Logo = ({ size = 36 }: { size?: number }) => (
  <svg width={size} height={size} viewBox="0 0 32 32" fill="none" aria-hidden="true">
    <rect width="32" height="32" rx="8" fill="rgba(255,255,255,0.1)" />
    <rect x="0.5" y="0.5" width="31" height="31" rx="7.5" stroke="rgba(255,255,255,0.18)" />
    <rect x="5" y="10" width="3" height="12" rx="1.5" fill="#F2BE2A" />
    <rect x="11" y="8" width="3" height="16" rx="1.5" fill="#FFFFFF" />
    <rect x="17" y="12" width="3" height="8" rx="1.5" fill="#F2BE2A" />
    <rect x="23" y="7" width="3" height="18" rx="1.5" fill="#FFFFFF" />
  </svg>
);

const NAV_ICONS: Record<string, ReactNode> = {
  "/": <Icon d="m3 9 9-7 9 7v11a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2z" extra={<path d="M9 22V12h6v10" />} />,
  "/repairs": <Icon d="M14.7 6.3a1 1 0 0 0 0 1.4l1.6 1.6a1 1 0 0 0 1.4 0l3.77-3.77a6 6 0 0 1-7.94 7.94l-6.91 6.91a2.12 2.12 0 0 1-3-3l6.91-6.91a6 6 0 0 1 7.94-7.94l-3.76 3.76z" />,
  "/repairs/pos": <Icon d="M4 5h16v3H4zM6 11h4v8H6zM14 11h4v8h-4z" />,
  "/customers": <Icon d="M16 21v-2a4 4 0 0 0-4-4H6a4 4 0 0 0-4 4v2" extra={<><circle cx="9" cy="7" r="4" /><path d="M22 21v-2a4 4 0 0 0-3-3.87M16 3.13a4 4 0 0 1 0 7.75" /></>} />,
  "/parts": <Icon d="M21 16V8a2 2 0 0 0-1-1.73l-7-4a2 2 0 0 0-2 0l-7 4A2 2 0 0 0 3 8v8a2 2 0 0 0 1 1.73l7 4a2 2 0 0 0 2 0l7-4A2 2 0 0 0 21 16z" extra={<path d="m3.29 7 8.71 5 8.71-5M12 22V12" />} />,
  "/inventory": <Icon d="M22 8.35V20a2 2 0 0 1-2 2H4a2 2 0 0 1-2-2V8.35A2 2 0 0 1 3.26 6.5l8-3.2a2 2 0 0 1 1.48 0l8 3.2A2 2 0 0 1 22 8.35z" />,
  "/suppliers": <Icon d="M5 8h14M5 8a2 2 0 1 0 0-4h14a2 2 0 1 0 0 4M5 8v10a2 2 0 0 0 2 2h10a2 2 0 0 0 2-2V8" />,
  "/pos": <Icon d="M6 2 3 6v14a2 2 0 0 0 2 2h14a2 2 0 0 0 2-2V6l-3-4z" extra={<path d="M3 6h18m-5 4a4 4 0 0 1-8 0" />} />,
  "/counter": <Icon d="M4 4h16v16H4z" extra={<path d="M8 8h3v3H8zM13 8h3v3h-3zM8 13h3v3H8zM13 13h3v3h-3z" />} />,
  "/counter/fix": <Icon d="M13 2 3 14h9l-1 8 10-12h-9l1-8z" />,
  "/counter/pickup": <Icon d="M21 16V8a2 2 0 0 0-1-1.73l-7-4a2 2 0 0 0-2 0l-7 4A2 2 0 0 0 3 8v8a2 2 0 0 0 1 1.73l7 4a2 2 0 0 0 2 0l7-4A2 2 0 0 0 21 16z" extra={<path d="m9 12 2 2 4-4" />} />,
  "/orders": <Icon d="M9 11V6a3 3 0 0 1 6 0v5" extra={<path d="M5.5 8h13l1 13h-15z" />} />,
  "/payments": <Icon d="M2 7h20v10H2z" extra={<circle cx="12" cy="12" r="2.5" />} />,
  "/risk": <Icon d="m21.73 18-8-14a2 2 0 0 0-3.48 0l-8 14A2 2 0 0 0 4 21h16a2 2 0 0 0 1.73-3z" extra={<path d="M12 9v4m0 4h.01" />} />,
  "/reports": <Icon d="M3 3v18h18" extra={<path d="m19 9-5 5-4-4-3 3" />} />,
  "/audit": <Icon d="M14 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V8z" extra={<path d="M14 2v6h6M9 13h6M9 17h6" />} />,
  "/sync": <Icon d="M21 12a9 9 0 0 0-9-9 9.75 9.75 0 0 0-6.74 2.74L3 8" extra={<path d="M3 3v5h5m-5 4a9 9 0 0 0 9 9 9.75 9.75 0 0 0 6.74-2.74L21 16m0 5v-5h-5" />} />,
  "/notifications": <Icon d="M6 8a6 6 0 0 1 12 0c0 7 3 9 3 9H3s3-2 3-9" extra={<path d="M10.3 21a1.94 1.94 0 0 0 3.4 0" />} />,
  "/trash": <Icon d="M3 6h18M8 6V4h8v2m-9 0 1 15h8l1-15M10 11v6m4-6v6" />,
  "/settings": <Icon d="M12.22 2h-.44a2 2 0 0 0-2 2v.18a2 2 0 0 1-1 1.73l-.43.25a2 2 0 0 1-2 0l-.15-.08a2 2 0 0 0-2.73.73l-.22.38a2 2 0 0 0 .73 2.73l.15.1a2 2 0 0 1 1 1.72v.51a2 2 0 0 1-1 1.74l-.15.09a2 2 0 0 0-.73 2.73l.22.38a2 2 0 0 0 2.73.73l.15-.08a2 2 0 0 1 2 0l.43.25a2 2 0 0 1 1 1.73V20a2 2 0 0 0 2 2h.44a2 2 0 0 0 2-2v-.18a2 2 0 0 1 1-1.73l.43-.25a2 2 0 0 1 2 0l.15.08a2 2 0 0 0 2.73-.73l.22-.39a2 2 0 0 0-.73-2.73l-.15-.08a2 2 0 0 1-1-1.74v-.5a2 2 0 0 1 1-1.74l.15-.09a2 2 0 0 0 .73-2.73l-.22-.38a2 2 0 0 0-2.73-.73l-.15.08a2 2 0 0 1-2 0l-.43-.25a2 2 0 0 1-1-1.73V4a2 2 0 0 0-2-2z" extra={<circle cx="12" cy="12" r="3" />} />,
};

type NavItem = { to: string; label: string; roles: string[]; end?: boolean; short?: string };

const NAV_GROUPS: { id: string; label: string; items: NavItem[] }[] = [
  {
    id: "workspaces",
    label: "Workshop",
    items: [
      { to: "/", label: "Home", short: "Home", roles: [], end: true },
      { to: "/repairs", label: "Jobs", short: "Jobs", roles: ["owner", "manager", "technician", "cashier"], end: true },
      { to: "/repairs/pos", label: "Job POS", short: "Job POS", roles: ["owner", "manager", "technician", "cashier"] },
      { to: "/customers", label: "Customers", short: "Clients", roles: ["owner", "manager", "technician", "cashier"] },
    ],
  },
  {
    id: "money",
    label: "Money",
    items: [
      { to: "/payments", label: "Transactions", short: "Pay", roles: ["owner", "manager", "cashier", "accountant"] },
      { to: "/orders", label: "Orders", short: "Orders", roles: ["owner", "manager", "cashier"] },
    ],
  },
  {
    id: "stock",
    label: "Stock",
    items: [
      { to: "/parts", label: "Part requests", short: "Parts", roles: ["owner", "manager", "technician", "inventory"] },
      { to: "/inventory", label: "Inventory", short: "Stock", roles: ["owner", "manager", "inventory"] },
      { to: "/suppliers", label: "Suppliers", short: "Supply", roles: ["owner", "manager", "inventory", "accountant"] },
    ],
  },
  {
    id: "oversight",
    label: "Oversight",
    items: [
      { to: "/risk", label: "Risk", short: "Risk", roles: ["owner", "manager", "accountant"] },
      { to: "/reports", label: "Reports", short: "Reports", roles: ["owner", "manager", "accountant"] },
      { to: "/audit", label: "Audit & alerts", short: "Audit", roles: ["owner", "manager", "accountant"] },
      { to: "/sync", label: "Sync", short: "Sync", roles: ["owner", "manager"] },
    ],
  },
];

function canSee(item: NavItem, roles: string[] | undefined, primaryRole: string) {
  if (item.roles.length === 0) return true;
  // Owner may not be roles[0] if custom roles were prepended — treat any owner grant as full nav.
  if (primaryRole === "owner" || roles?.includes("owner")) return true;
  return roles?.some((r) => item.roles.includes(r)) ?? false;
}

function roleLabel(role: string) {
  if (!role) return "Staff";
  return role.replace(/_/g, " ");
}

export function AppShell() {
  const { user, logout, primaryRole } = useAuth();
  const { branches, branchId, setBranchId } = useBranch();
  const navigate = useNavigate();
  const location = useLocation();
  const [showBranchPicker] = useState(() => (user?.branch_ids?.length ?? 0) > 1 || branches.length > 1);
  const [mobileOpen, setMobileOpen] = useState(false);
  const [collapsed, setCollapsed] = useState(() => {
    try {
      return localStorage.getItem("tl.sidebar.collapsed") === "1";
    } catch {
      return false;
    }
  });
  const [unackedCount, setUnackedCount] = useState(0);
  const [toasts, setToasts] = useState<{ id: number; text: string }[]>([]);
  const toastId = useRef(0);
  const { open: paletteOpen, setOpen: setPaletteOpen } = useCommandPalette();
  const isMac = typeof navigator !== "undefined" && /Mac|iPhone|iPad/.test(navigator.platform ?? navigator.userAgent);

  useEffect(() => {
    document.documentElement.setAttribute("data-theme", "light");
  }, []);

  useEffect(() => {
    setMobileOpen(false);
  }, [location.pathname]);

  useEffect(() => {
    try {
      localStorage.setItem("tl.sidebar.collapsed", collapsed ? "1" : "0");
    } catch {
      /* ignore */
    }
  }, [collapsed]);

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

  useRealtimeEvents(LIVE_EVENT_PATTERNS, (event) => {
    void refreshNotifications();
    const text = describeEvent(event);
    if (!text) return;
    const id = ++toastId.current;
    setToasts((prev) => [...prev.slice(-3), { id, text }]);
    window.setTimeout(() => {
      setToasts((prev) => prev.filter((t) => t.id !== id));
    }, 6000);
  });

  const groups = NAV_GROUPS.map((group) => ({
    ...group,
    items: group.items.filter((item) => canSee(item, user?.roles, primaryRole)),
  })).filter((group) => group.items.length > 0);
  // Mobile bottom bar: workshop + counter essentials.
  const mobileItems = [
    ...(groups.find((g) => g.id === "workspaces")?.items ?? []),
    ...(groups.find((g) => g.id === "counter")?.items ?? []),
  ].slice(0, 5);

  const initials = user?.display_name
    ?.split(" ")
    .map((p) => p[0])
    .slice(0, 2)
    .join("")
    .toUpperCase() || "?";

  const branchName = showBranchPicker
    ? branches.find((b) => b.id === branchId)?.name || "Select branch"
    : branches[0]?.name || "Main branch";
  const currentSection =
    groups.flatMap((group) => group.items).find((item) =>
      item.end ? location.pathname === item.to : location.pathname.startsWith(item.to),
    )?.label ?? (location.pathname.startsWith("/settings") ? "Settings" : "Operations");
  const pageSlug = location.pathname === "/"
    ? "home"
    : location.pathname.split("/").filter(Boolean).slice(0, 2).join("-").replace(/[^a-z0-9-]/gi, "");

  const isCounterSurface =
    location.pathname === "/pos" ||
    location.pathname === "/counter" ||
    location.pathname.startsWith("/counter/");

  return (
    <div
      className="shell"
      data-role={primaryRole}
      data-ui="ops-v4"
      data-counter={isCounterSurface ? "true" : undefined}
      data-sidebar={collapsed ? "collapsed" : "expanded"}
    >
      <a href="#main-content" className="skip-link">
        Skip to main content
      </a>
      <header className="mobile-header">
        <button
          type="button"
          className="mobile-menu-btn"
          aria-label={mobileOpen ? "Close menu" : "Open menu"}
          onClick={() => setMobileOpen((v) => !v)}
        >
          <Icon d={mobileOpen ? "M18 6 6 18M6 6l12 12" : "M4 12h16M4 6h16M4 18h16"} />
        </button>
        <span className="mobile-header-brand">
          <Logo size={26} />
          <strong>TechLane</strong>
        </span>
        <button type="button" className="mobile-search-btn" aria-label="Search" onClick={() => setPaletteOpen(true)}>
          <Icon d="m21 21-4.34-4.34" extra={<circle cx="11" cy="11" r="7" />} size={19} />
        </button>
        <NavLink to="/notifications" className="mobile-bell" aria-label="Notifications">
          {NAV_ICONS["/notifications"]}
          {unackedCount > 0 ? <span className="nav-badge">{unackedCount > 9 ? "9+" : unackedCount}</span> : null}
        </NavLink>
      </header>

      <aside className="sidebar" data-mobile-open={mobileOpen}>
        <div className="sidebar-top">
          <div className="sidebar-brand">
            <Logo />
            <div className="sidebar-brand-text">
              <strong>TechLane</strong>
              <span>Ops</span>
            </div>
            <button
              type="button"
              className="sidebar-collapse"
              aria-label={collapsed ? "Expand sidebar" : "Collapse sidebar"}
              title={collapsed ? "Expand" : "Collapse"}
              onClick={() => setCollapsed((v) => !v)}
            >
              <Icon d={collapsed ? "m9 18 6-6-6-6" : "m15 18-6-6 6-6"} size={16} />
            </button>
          </div>

          <div className="branch-picker" title={branchName}>
            <span className="branch-picker-icon">
              <Icon d="M3 21h18M5 21V7l7-4 7 4v14M9 21v-6h6v6" size={16} />
            </span>
            <span className="branch-picker-name">{branchName}</span>
            {showBranchPicker ? (
              <>
                <span className="branch-picker-caret">
                  <Icon d="m6 9 6 6 6-6" size={14} />
                </span>
                <select
                  className="branch-picker-overlay"
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
              </>
            ) : null}
          </div>

          <button type="button" className="sidebar-search-trigger" onClick={() => setPaletteOpen(true)} title="Search">
            <Icon d="m21 21-4.34-4.34" extra={<circle cx="11" cy="11" r="7" />} size={16} />
            <span>Search</span>
            <kbd>{isMac ? "⌘K" : "Ctrl+K"}</kbd>
          </button>
        </div>

        <nav className="sidebar-nav" aria-label="Primary">
          {groups.map((group) => (
            <div key={group.id} className="nav-group">
              <p className="nav-group-label">{group.label}</p>
              {group.items.map((item) => (
                <NavLink
                  key={item.to}
                  to={item.to}
                  end={item.end}
                  title={item.label}
                  className={({ isActive }) => `nav-link ${isActive ? "active" : ""}`}
                >
                  <span className="nav-link-icon">{NAV_ICONS[item.to]}</span>
                  <span className="nav-link-label">{item.label}</span>
                  {item.to === "/notifications" && unackedCount > 0 ? (
                    <span className="nav-badge">{unackedCount > 9 ? "9+" : unackedCount}</span>
                  ) : null}
                </NavLink>
              ))}
            </div>
          ))}
        </nav>

        <div className="sidebar-footer">
          <NavLink
            to="/settings"
            title="Settings"
            className={({ isActive }) => `nav-link nav-link-settings ${isActive ? "active" : ""}`}
          >
            <span className="nav-link-icon">{NAV_ICONS["/settings"]}</span>
            <span className="nav-link-label">Settings</span>
          </NavLink>
          <div className="user-card">
            <button type="button" className="user-card-main" onClick={() => navigate("/settings")} title={user?.display_name}>
              <span className="user-avatar">{initials}</span>
              <span className="user-meta">
                <strong>{user?.display_name}</strong>
                <span>{roleLabel(primaryRole)}</span>
              </span>
            </button>
            <button type="button" className="user-signout" onClick={logout} title="Sign out" aria-label="Sign out">
              <Icon d="M9 21H5a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2h4m7 14 5-5-5-5m5 5H9" size={17} />
            </button>
          </div>
        </div>
      </aside>

      {mobileOpen ? (
        <button type="button" className="sidebar-backdrop" aria-label="Close menu" onClick={() => setMobileOpen(false)} />
      ) : null}

      <div className="workspace">
        <header className="workspace-bar">
          <div className="workspace-context">
            <span>{branchName}</span>
            <strong>{currentSection}</strong>
          </div>
          <div className="workspace-tools">
            <button type="button" className="workspace-search" onClick={() => setPaletteOpen(true)}>
              <Icon d="m21 21-4.34-4.34" extra={<circle cx="11" cy="11" r="7" />} size={16} />
              <span>Search workspace</span>
              <kbd>{isMac ? "⌘K" : "Ctrl K"}</kbd>
            </button>
            <NavLink to="/notifications" className="workspace-notifications" aria-label="Notifications">
              {NAV_ICONS["/notifications"]}
              {unackedCount > 0 ? <span>{unackedCount > 9 ? "9+" : unackedCount}</span> : null}
            </NavLink>
            <span className="workspace-date">
              {new Intl.DateTimeFormat("en", { weekday: "short", day: "numeric", month: "short" }).format(new Date())}
            </span>
          </div>
        </header>
        <main className={`content page-view page-${pageSlug}`} id="main-content" tabIndex={-1}>
          <Outlet />
        </main>
      </div>

      <nav className="mobile-workspace-nav" aria-label="Main workspaces">
        {mobileItems.map((item) => (
          <NavLink key={item.to} to={item.to} end={item.end} className={({ isActive }) => `mobile-workspace-link ${isActive ? "active" : ""}`}>
            <span>{NAV_ICONS[item.to]}</span>
            <small>{item.short ?? item.label}</small>
            {item.to === "/notifications" && unackedCount > 0 ? <b>{unackedCount > 9 ? "9+" : unackedCount}</b> : null}
          </NavLink>
        ))}
      </nav>

      {toasts.length > 0 ? (
        <div className="toast-stack" role="status" aria-live="polite">
          {toasts.map((t) => (
            <div key={t.id} className="toast">
              <span className="toast-dot" aria-hidden="true" />
              {t.text}
            </div>
          ))}
        </div>
      ) : null}

      <CommandPalette open={paletteOpen} onClose={() => setPaletteOpen(false)} />
    </div>
  );
}
