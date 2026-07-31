import type { ReactNode } from "react";
import { NavLink, Outlet } from "react-router-dom";
import { useAuth } from "../../auth/AuthContext";
import { Badge, ICONS, Icon } from "../../components/ui";

type SettingsNavItem = {
  to: string;
  label: string;
  icon: ReactNode;
  ownerOnly?: boolean;
  end?: boolean;
};

const GROUPS: Array<{ label: string; items: SettingsNavItem[] }> = [
  {
    label: "People & access",
    items: [
      { to: "/settings/staff", label: "Staff", icon: ICONS.customers },
      {
        to: "/settings/roles",
        label: "Roles",
        icon: <Icon d="M12 22s8-4 8-10V5l-8-3-8 3v7c0 6 8 10 8 10z" />,
      },
      {
        to: "/settings/commissions",
        label: "Commissions",
        icon: <Icon d="M12 2v20m5-15H9.5a3.5 3.5 0 0 0 0 7h5a3.5 3.5 0 0 1 0 7H6" />,
      },
      {
        to: "/settings/security",
        label: "Security",
        icon: (
          <Icon
            d="M12 2 4 5v6c0 5.25 3.4 9.74 8 11 4.6-1.26 8-5.75 8-11V5z"
            extra={<path d="m9 12 2 2 4-4" />}
          />
        ),
      },
    ],
  },
  {
    label: "Business",
    items: [
      {
        to: "/settings/branches",
        label: "Branches",
        icon: <Icon d="M3 21h18M5 21V7l7-4 7 4v14M9 21v-6h6v6" />,
      },
      {
        to: "/settings/shop",
        label: "Shop & tax",
        icon: (
          <Icon
            d="M14 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V8z"
            extra={<path d="M14 2v6h6M9 13h6M9 17h6" />}
          />
        ),
      },
      {
        to: "/settings/receipts",
        label: "Receipts",
        icon: (
          <Icon
            d="M6 2h12v20l-3-2-3 2-3-2-3 2z"
            extra={<path d="M9 7h6M9 11h6M9 15h3" />}
          />
        ),
        ownerOnly: true,
      },
      {
        to: "/settings/storefront",
        label: "Storefront",
        icon: <Icon d="M3 9l1-6h16l1 6M3 9v11h18V9M3 9h18M9 21v-6h6v6" />,
        ownerOnly: true,
        end: true,
      },
      {
        to: "/settings/storefront/banners",
        label: "Storefront banners",
        icon: <Icon d="M3 5h18v14H3z" extra={<path d="m3 15 5-5 4 4 5-6 4 5" />} />,
        ownerOnly: true,
      },
      {
        to: "/settings/storefront/deals",
        label: "Storefront deals",
        icon: <Icon d="M20.59 13.41 11 3.83A2 2 0 0 0 9.59 3H4a1 1 0 0 0-1 1v5.59a2 2 0 0 0 .59 1.41l9.58 9.59a2 2 0 0 0 2.82 0l4.59-4.59a2 2 0 0 0 .01-2.59z" extra={<circle cx="7.5" cy="7.5" r="1.5" />} />,
        ownerOnly: true,
      },
      {
        to: "/settings/storefront/delivery",
        label: "Delivery locations",
        icon: <Icon d="M3 12h18M12 3v18M5 8l7-5 7 5M5 16l7 5 7-5" />,
        ownerOnly: true,
      },
      {
        to: "/settings/storefront/subscribers",
        label: "Newsletter subscribers",
        icon: <Icon d="M4 4h16v16H4z" extra={<path d="m4 6 8 7 8-7" />} />,
        ownerOnly: true,
      },
      {
        to: "/settings/storefront/reviews",
        label: "Storefront reviews",
        icon: <Icon d="M12 17.27 18.18 21l-1.64-7.03L22 9.24l-7.19-.61L12 2 9.19 8.63 2 9.24l5.46 4.73L5.82 21z" />,
        ownerOnly: true,
      },
    ],
  },
  {
    label: "Money & messaging",
    items: [
      { to: "/settings/payments", label: "Payments", icon: ICONS.cash },
      {
        to: "/settings/sms",
        label: "SMS",
        icon: <Icon d="M21 15a2 2 0 0 1-2 2H7l-4 4V5a2 2 0 0 1 2-2h14a2 2 0 0 1 2 2z" />,
        ownerOnly: true,
      },
      {
        to: "/settings/whatsapp",
        label: "WhatsApp",
        icon: <Icon d="M21 11.5a8.38 8.38 0 0 1-.9 3.8 8.5 8.5 0 0 1-7.6 4.7 8.38 8.38 0 0 1-3.8-.9L3 21l1.9-5.7a8.38 8.38 0 0 1-.9-3.8 8.5 8.5 0 0 1 4.7-7.6 8.38 8.38 0 0 1 3.8-.9h.5a8.48 8.48 0 0 1 8 8v.5z" />,
        ownerOnly: true,
      },
      {
        to: "/settings/loyalty",
        label: "Loyalty & marketing",
        icon: <Icon d="M12 21s-7.5-4.6-10-9.3C.4 8.1 2.3 4.5 6 4.1c2-.2 3.7.8 6 3 2.3-2.2 4-3.2 6-3 3.7.4 5.6 4 4 7.6-2.5 4.7-10 9.3-10 9.3z" />,
        ownerOnly: true,
      },
    ],
  },
];

export function SettingsLayout() {
  const { user } = useAuth();
  const isOwner = user?.roles?.includes("owner") ?? false;

  return (
    <div className="settings-desk">
      <aside className="settings-nav" aria-label="Settings">
        <div className="settings-group">
          <span className="settings-group-label">Configuration</span>
          <strong style={{ display: "block", padding: "0 0.7rem 0.55rem", color: "var(--navy-950)", fontFamily: "var(--font-display)", fontSize: "1.35rem" }}>
            Settings
          </strong>
        </div>
        <nav>
          {GROUPS.map((group) => {
            const items = group.items.filter((item) => !item.ownerOnly || isOwner);
            if (items.length === 0) return null;
            return (
              <div key={group.label} className="settings-group">
                <span className="settings-group-label">{group.label}</span>
                {items.map((item) => (
                  <NavLink
                    key={item.to}
                    to={item.to}
                    end={item.end}
                    className={({ isActive }) => (isActive ? "active" : undefined)}
                  >
                    <span className="settings-nav-icon">{item.icon}</span>
                    <span>{item.label}</span>
                    {item.ownerOnly ? <Badge tone="info">owner</Badge> : null}
                  </NavLink>
                ))}
              </div>
            );
          })}
        </nav>
      </aside>
      <div className="settings-canvas">
        <Outlet />
      </div>
    </div>
  );
}
