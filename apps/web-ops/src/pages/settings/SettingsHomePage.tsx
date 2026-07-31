import { Link } from "react-router-dom";
import { useAuth } from "../../auth/AuthContext";
import { Badge, PageHeader } from "../../components/ui";

type HubCard = {
  to: string;
  title: string;
  body: string;
  ownerOnly?: boolean;
};

const CARDS: HubCard[] = [
  { to: "/settings/staff", title: "Staff", body: "People, roles on accounts, and branch access." },
  { to: "/settings/roles", title: "Roles", body: "Permission sets for owners, managers, and custom roles." },
  { to: "/settings/branches", title: "Branches", body: "Shop locations used across jobs, stock, and POS." },
  { to: "/settings/shop", title: "Shop & tax", body: "Business profile, currency, and tax details." },
  { to: "/settings/intake-presets", title: "Intake presets", body: "Condition tags and common issue suggestions." },
  {
    to: "/trash",
    title: "Trashed jobs",
    body: "Restore or permanently purge soft-deleted repair jobs.",
    ownerOnly: true,
  },
];

/** Settings hub — cards for common destinations; full list stays in the settings sidebar. */
export function SettingsHomePage() {
  const { user } = useAuth();
  const isOwner = user?.roles?.includes("owner") ?? false;
  const cards = CARDS.filter((c) => !c.ownerOnly || isOwner);

  return (
    <div className="settings-page">
      <PageHeader
        title="Settings"
        subtitle="Configure the shop. Use the sidebar for the full list, or jump from a card below."
      />
      <div className="hub-card-grid">
        {cards.map((c) => (
          <Link key={c.to} to={c.to} className="hub-card">
            <strong>
              {c.title}
              {c.ownerOnly ? <Badge tone="info">owner</Badge> : null}
            </strong>
            <span>{c.body}</span>
          </Link>
        ))}
      </div>
    </div>
  );
}
