import { useEffect, useState } from "react";
import { Button, EmptyState, PageHeader } from "../../components/ui";
import { downloadCSV } from "../../lib/csv";
import { listNewsletterSubscribers, type NewsletterSubscriber } from "../../lib/api";

export function StorefrontSubscribersPage() {
  const [subscribers, setSubscribers] = useState<NewsletterSubscriber[]>([]);
  const [error, setError] = useState("");

  useEffect(() => {
    listNewsletterSubscribers()
      .then((r) => setSubscribers(r.items))
      .catch((e) => setError(e instanceof Error ? e.message : "Failed to load subscribers"));
  }, []);

  function exportCSV() {
    downloadCSV("newsletter-subscribers.csv", [
      ["Email", "Subscribed at"],
      ...subscribers.map((s) => [s.email, new Date(s.created_at).toISOString()]),
    ]);
  }

  return (
    <div className="settings-page">
      <PageHeader
        title="Newsletter subscribers"
        subtitle="Emails captured from the storefront newsletter form"
        actions={
          <Button type="button" variant="secondary" disabled={subscribers.length === 0} onClick={exportCSV}>
            Download CSV
          </Button>
        }
      />
      {error ? <p className="form-error">{error}</p> : null}

      {subscribers.length === 0 ? (
        <EmptyState title="No subscribers yet" body="Signups from the storefront newsletter form will appear here." />
      ) : (
        <div className="table-wrap">
          <table className="table">
            <thead>
              <tr>
                <th>Email</th>
                <th>Subscribed</th>
              </tr>
            </thead>
            <tbody>
              {subscribers.map((s) => (
                <tr key={s.id}>
                  <td>{s.email}</td>
                  <td>{new Date(s.created_at).toLocaleString()}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}
    </div>
  );
}
