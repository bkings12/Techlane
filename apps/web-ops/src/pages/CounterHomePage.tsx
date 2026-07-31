import { Link } from "react-router-dom";
import { PageHeader } from "../components/ui";

const DESTINATIONS = [
  {
    to: "/pos",
    title: "Sell",
    body: "Counter POS — scan, cart, cash or M-Pesa.",
  },
  {
    to: "/counter/fix",
    title: "Same-day fix",
    body: "Quick walk-in repair when the customer is waiting.",
  },
  {
    to: "/counter/pickup",
    title: "Pickup",
    body: "Hand a device back with pickup code and payment check.",
  },
] as const;

export function CounterHomePage() {
  return (
    <div className="counter-home">
      <PageHeader
        title="Counter"
        subtitle="Choose a counter workflow. Sell, take a same-day fix, or release a pickup."
      />
      <div className="hub-card-grid">
        {DESTINATIONS.map((d) => (
          <Link key={d.to} to={d.to} className="hub-card">
            <strong>{d.title}</strong>
            <span>{d.body}</span>
          </Link>
        ))}
      </div>
    </div>
  );
}
