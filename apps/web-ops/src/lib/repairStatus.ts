/**
 * Display vocabulary for the repair job state machine. Kept in one place so the
 * board, the job detail page and Reports never disagree about what a status or
 * closure code means. The codes themselves are owned by the backend
 * (internal/repair/status.go).
 */

export const CLOSURE_REASON_LABELS: Record<string, string> = {
  customer_declined_quote: "Customer declined the quote",
  customer_withdrew: "Customer took the device back",
  no_response: "Customer never responded",
  duplicate_job: "Duplicate job card",
  beyond_economical_repair: "Beyond economical repair",
  parts_unavailable: "Parts not available",
  severe_liquid_damage: "Severe liquid damage",
  further_damage_found: "Further damage found on opening",
  other: "Other (see note)",
};

export const CLOSURE_STATUS_LABELS: Record<string, string> = {
  cancelled: "Cancelled — job will not go ahead",
  unrepairable: "Unrepairable — cannot be fixed",
};

/** Shop-floor labels for the Jobs desk and badges. */
export const STATUS_LABELS: Record<string, string> = {
  intake: "Intake",
  diagnosed: "Diagnosed",
  waiting_parts: "Waiting parts",
  in_progress: "On bench",
  ready_for_pickup: "Ready",
  completed: "Ready",
  collected: "Collected",
  cancelled: "Cancelled",
  unrepairable: "Unrepairable",
  details_corrected: "Details corrected",
};

/** Linear workshop order — used to sort status chips and stage rails. */
export const WORKFLOW_ORDER = [
  "intake",
  "diagnosed",
  "waiting_parts",
  "in_progress",
  "ready_for_pickup",
  "completed",
  "collected",
  "cancelled",
  "unrepairable",
] as const;

export function compareWorkflowStatus(a: string, b: string) {
  const ia = WORKFLOW_ORDER.indexOf(a as (typeof WORKFLOW_ORDER)[number]);
  const ib = WORKFLOW_ORDER.indexOf(b as (typeof WORKFLOW_ORDER)[number]);
  return (ia < 0 ? 99 : ia) - (ib < 0 ? 99 : ib);
}

export function isClosureStatus(status: string) {
  return status === "cancelled" || status === "unrepairable";
}

export function statusTone(status: string): "success" | "warning" | "danger" | "info" | "pending" {
  if (
    status === "ready_for_pickup" ||
    status === "completed" ||
    status === "collected" ||
    status === "approved" ||
    status === "issued_from_stock"
  )
    return "success";
  if (isClosureStatus(status) || status.includes("fail") || status === "orphan" || status === "rejected")
    return "danger";
  if (status === "waiting_parts" || status === "pending" || status === "provisional" || status === "pending_handover")
    return "pending";
  return "info";
}

export function statusLabel(status: string) {
  return STATUS_LABELS[status] ?? status.replaceAll("_", " ");
}

/** Part request statuses read better with a little rewording than raw codes. */
export const PART_STATUS_LABELS: Record<string, string> = {
  issued_from_stock: "from our stock",
  pending: "pending",
  approved: "approved",
  collected: "collected",
  cancelled: "cancelled",
};
