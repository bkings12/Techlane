import { useEffect, useState } from "react";
import { Badge, Button, EmptyState, Input, PageHeader } from "../../components/ui";
import {
  createDeliveryLocation,
  deleteDeliveryLocation,
  listDeliveryLocations,
  updateDeliveryLocation,
  type DeliveryLocation,
} from "../../lib/api";

export function DeliveryLocationsPage() {
  const [items, setItems] = useState<DeliveryLocation[]>([]);
  const [error, setError] = useState("");
  const [busy, setBusy] = useState(false);

  const [name, setName] = useState("");
  const [description, setDescription] = useState("");
  const [fee, setFee] = useState("");
  const [sortOrder, setSortOrder] = useState("0");

  function refresh() {
    return listDeliveryLocations()
      .then((res) => setItems(res.items))
      .catch((e) => setError(e instanceof Error ? e.message : "Failed to load delivery locations"));
  }

  useEffect(() => {
    void refresh();
  }, []);

  async function create() {
    setBusy(true);
    setError("");
    try {
      const feeNum = Number(fee);
      if (!name.trim()) {
        setError("Name is required");
        setBusy(false);
        return;
      }
      if (Number.isNaN(feeNum) || feeNum < 0) {
        setError("Enter a valid delivery fee (0 or more)");
        setBusy(false);
        return;
      }
      await createDeliveryLocation({
        name: name.trim(),
        description: description.trim() || undefined,
        fee: feeNum,
        sort_order: Number(sortOrder) || 0,
        active: true,
      });
      setName("");
      setDescription("");
      setFee("");
      setSortOrder("0");
      await refresh();
    } catch (e) {
      setError(e instanceof Error ? e.message : "Could not create location");
    } finally {
      setBusy(false);
    }
  }

  async function toggleActive(loc: DeliveryLocation) {
    setBusy(true);
    setError("");
    try {
      const next = await updateDeliveryLocation(loc.id, {
        name: loc.name,
        description: loc.description,
        fee: loc.fee,
        sort_order: loc.sort_order,
        active: !loc.active,
      });
      setItems((prev) => prev.map((d) => (d.id === next.id ? next : d)));
    } catch (e) {
      setError(e instanceof Error ? e.message : "Update failed");
    } finally {
      setBusy(false);
    }
  }

  async function saveFee(loc: DeliveryLocation, nextFee: string) {
    const feeNum = Number(nextFee);
    if (Number.isNaN(feeNum) || feeNum < 0) {
      setError("Enter a valid delivery fee");
      return;
    }
    setBusy(true);
    setError("");
    try {
      const next = await updateDeliveryLocation(loc.id, {
        name: loc.name,
        description: loc.description,
        fee: feeNum,
        sort_order: loc.sort_order,
        active: loc.active,
      });
      setItems((prev) => prev.map((d) => (d.id === next.id ? next : d)));
    } catch (e) {
      setError(e instanceof Error ? e.message : "Update failed");
    } finally {
      setBusy(false);
    }
  }

  async function remove(id: string) {
    setBusy(true);
    setError("");
    try {
      await deleteDeliveryLocation(id);
      await refresh();
    } catch (e) {
      setError(e instanceof Error ? e.message : "Delete failed");
    } finally {
      setBusy(false);
    }
  }

  return (
    <div className="settings-page">
      <PageHeader
        title="Delivery locations"
        subtitle="Areas customers can choose at checkout, each with its own delivery fee. Pickup stays free."
      />
      {error ? <p className="form-error">{error}</p> : null}

      <section className="settings-form-card form-grid">
        <h2 style={{ margin: 0 }}>New location</h2>
        <label>
          Name
          <Input value={name} placeholder="CBD / Town" onChange={(e) => setName(e.target.value)} />
        </label>
        <label>
          Description (optional)
          <Input
            value={description}
            placeholder="Within Nairobi CBD, same-day where possible"
            onChange={(e) => setDescription(e.target.value)}
          />
        </label>
        <div className="field-pair">
          <label>
            Delivery fee (KES)
            <Input type="number" min={0} value={fee} placeholder="200" onChange={(e) => setFee(e.target.value)} />
          </label>
          <label>
            Sort order
            <Input type="number" value={sortOrder} onChange={(e) => setSortOrder(e.target.value)} />
          </label>
        </div>
        <div className="btn-row">
          <Button type="button" disabled={busy} onClick={() => void create()}>
            Add location
          </Button>
        </div>
      </section>

      {items.length === 0 ? (
        <EmptyState
          title="No delivery locations yet"
          body="Add areas above. Until you do, the storefront only offers branch pickup."
        />
      ) : (
        <div className="table-wrap">
          <table className="table">
            <thead>
              <tr>
                <th>Location</th>
                <th>Fee</th>
                <th>Order</th>
                <th>Status</th>
                <th />
              </tr>
            </thead>
            <tbody>
              {items.map((loc) => (
                <tr key={loc.id}>
                  <td>
                    <strong>{loc.name}</strong>
                    {loc.description ? <div className="muted tiny">{loc.description}</div> : null}
                  </td>
                  <td style={{ minWidth: 120 }}>
                    <Input
                      type="number"
                      min={0}
                      defaultValue={String(loc.fee)}
                      disabled={busy}
                      onBlur={(e) => {
                        if (String(loc.fee) !== e.target.value) void saveFee(loc, e.target.value);
                      }}
                    />
                  </td>
                  <td>{loc.sort_order}</td>
                  <td>
                    <Badge tone={loc.active ? "success" : "pending"}>{loc.active ? "active" : "paused"}</Badge>
                  </td>
                  <td>
                    <div className="btn-row">
                      <Button type="button" variant="ghost" disabled={busy} onClick={() => void toggleActive(loc)}>
                        {loc.active ? "Pause" : "Resume"}
                      </Button>
                      <Button type="button" variant="ghost" disabled={busy} onClick={() => void remove(loc.id)}>
                        Delete
                      </Button>
                    </div>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}
    </div>
  );
}
