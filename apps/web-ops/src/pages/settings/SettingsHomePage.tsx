import { Link } from "react-router-dom";
import { Badge, PageHeader } from "../../components/ui";

const CARDS: Array<{ to: string; title: string; body: string; ownerOnly?: boolean }> = [
  { to: "/settings/staff", title: "Staff", body: "Create technicians and assign roles and branches" },
  { to: "/settings/branches", title: "Branches", body: "Create and edit shop branches" },
  { to: "/settings/roles", title: "Roles & permissions", body: "System role presets and permission matrix" },
  { to: "/settings/commissions", title: "Commissions", body: "Pending technician commission accruals" },
  {
    to: "/settings/payments",
    title: "Payments",
    body: "M-Pesa Daraja credentials and bank paybill / account",
  },
  {
    to: "/settings/shop",
    title: "Shop profile & tax",
    body: "Legal name, TIN, address, and VAT for receipts / tax invoices",
  },
  {
    to: "/settings/sms",
    title: "SMS (OTP)",
    body: "BlessedTexts sender ID and API key for customer repair OTP",
    ownerOnly: true,
  },
];

export function SettingsHomePage() {
  return (
    <div>
      <PageHeader
        title="Settings"
        subtitle="People, access, payments, and customer OTP delivery"
      />
      <div className="action-grid">
        {CARDS.map((c) => (
          <Link key={c.to} className="action-tile" to={c.to}>
            <strong>
              {c.title}
              {c.ownerOnly ? (
                <>
                  {" "}
                  <Badge tone="info">owner</Badge>
                </>
              ) : null}
            </strong>
            <span>{c.body}</span>
          </Link>
        ))}
      </div>
    </div>
  );
}
