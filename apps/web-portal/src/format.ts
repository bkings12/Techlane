import type { Estimate, GuestRepair, Repair } from "./api";

export function estimateTotal(est: Estimate): number {
  if (typeof est.total_amount === "number") return est.total_amount;
  return (est.labor_amount ?? 0) + (est.parts_amount ?? 0);
}

export function deviceLabel(r: Repair | GuestRepair) {
  if ("device_brand" in r || "device_model" in r) {
    const brand = (r as Repair).device_brand;
    const model = (r as Repair).device_model;
    return [brand, model].filter(Boolean).join(" ");
  }
  const d = r.device;
  return [d?.brand, d?.model, d?.kind].filter(Boolean).join(" ");
}

export function pendingEstimate(r: Repair) {
  return r.estimate ?? r.pending_estimate ?? r.estimates?.find((e) => e.status === "pending") ?? null;
}

export function pillTone(status: string): "ok" | "warn" | "danger" | "" {
  if (["ready_for_pickup", "completed", "collected", "approved"].includes(status)) return "ok";
  if (["waiting_parts", "pending", "diagnosed", "intake"].includes(status)) return "warn";
  if (status.includes("fail") || status === "cancelled" || status === "unrepairable" || status === "rejected")
    return "danger";
  return "";
}

export function initials(name?: string | null, phone?: string | null) {
  if (name?.trim()) {
    return name
      .split(/\s+/)
      .slice(0, 2)
      .map((p) => p[0]?.toUpperCase() ?? "")
      .join("");
  }
  return (phone || "?").slice(-2);
}
