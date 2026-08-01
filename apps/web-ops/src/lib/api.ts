const API_BASE = import.meta.env.VITE_API_BASE ?? "http://localhost:8080/api/v1";

export type UserProfile = {
  id: string;
  email: string;
  display_name: string;
  tenant_id: string;
  roles: string[];
  permissions: string[];
  branch_ids: string[];
};

type TokenPair = {
  access_token: string;
  refresh_token: string;
  expires_in: number;
};

type LoginOutcome = {
  tokens?: TokenPair;
  mfa_required?: boolean;
  mfa_challenge?: string;
};

const TOKEN_KEY = "techlane.access";
const REFRESH_KEY = "techlane.refresh";
const SESSION_EXPIRED_EVENT = "techlane:session-expired";

export class SessionExpiredError extends Error {
  constructor(message = "Session expired. Please sign in again.") {
    super(message);
    this.name = "SessionExpiredError";
  }
}

export class AccountLockedError extends Error {
  lockedUntil: string;
  constructor(message: string, lockedUntil: string) {
    super(message);
    this.name = "AccountLockedError";
    this.lockedUntil = lockedUntil;
  }
}

export class ConflictError extends Error {
  constructor(message: string) {
    super(message);
    this.name = "ConflictError";
  }
}

export function getAccessToken() {
  return localStorage.getItem(TOKEN_KEY);
}

export function getRefreshToken() {
  return localStorage.getItem(REFRESH_KEY);
}

export function clearSession() {
  localStorage.removeItem(TOKEN_KEY);
  localStorage.removeItem(REFRESH_KEY);
}

function persistTokens(tokens: TokenPair) {
  localStorage.setItem(TOKEN_KEY, tokens.access_token);
  localStorage.setItem(REFRESH_KEY, tokens.refresh_token);
}

function notifySessionExpired() {
  clearSession();
  if (typeof window !== "undefined") {
    window.dispatchEvent(new Event(SESSION_EXPIRED_EVENT));
  }
}

export function onSessionExpired(handler: () => void) {
  if (typeof window === "undefined") return () => undefined;
  const listener = () => handler();
  window.addEventListener(SESSION_EXPIRED_EVENT, listener);
  return () => window.removeEventListener(SESSION_EXPIRED_EVENT, listener);
}

/**
 * "rejected" means the server refused the refresh token (sign-in required);
 * "unavailable" means we never got an answer (offline, network change, 5xx),
 * which must not destroy a still-valid session.
 */
type RefreshOutcome = "refreshed" | "rejected" | "unavailable";

let refreshInFlight: Promise<RefreshOutcome> | null = null;

async function refreshAccessToken(): Promise<RefreshOutcome> {
  if (refreshInFlight) return refreshInFlight;
  refreshInFlight = (async (): Promise<RefreshOutcome> => {
    const refresh = getRefreshToken();
    if (!refresh) return "rejected";
    let res: Response;
    try {
      res = await fetch(`${API_BASE}/auth/refresh`, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ refresh_token: refresh }),
      });
    } catch {
      return "unavailable";
    }
    if (res.status >= 500 || res.status === 429) return "unavailable";
    if (!res.ok) {
      // Refresh tokens are single-use and rotate server-side. If another tab
      // already refreshed with this same token while we were in flight, ours
      // gets rejected — but the session is fine, just not the token we tried.
      // Only declare it dead if localStorage still holds what we started with.
      if (getRefreshToken() !== refresh) return "refreshed";
      return "rejected";
    }
    try {
      const tokens = (await res.json()) as TokenPair;
      if (!tokens.access_token || !tokens.refresh_token) return "rejected";
      persistTokens(tokens);
      return "refreshed";
    } catch {
      return "unavailable";
    }
  })().finally(() => {
    refreshInFlight = null;
  });
  return refreshInFlight;
}

/** Proactively rotate the access token while a session is active. */
export async function refreshSession(): Promise<boolean> {
  if (!getRefreshToken()) return false;
  const outcome = await refreshAccessToken();
  return outcome === "refreshed";
}

async function parseErrorBody(res: Response) {
  return (await res.json().catch(() => ({}))) as {
    error?: { message?: string; code?: string; locked_until?: string };
  };
}

async function parseErrorMessage(res: Response) {
  const body = await parseErrorBody(res);
  return body?.error?.message ?? res.statusText;
}

export async function api<T>(
  path: string,
  options: RequestInit & { auth?: boolean; _retried?: boolean } = {},
): Promise<T> {
  const headers = new Headers(options.headers);
  if (options.body != null && !headers.has("Content-Type")) {
    headers.set("Content-Type", "application/json");
  }
  const wantsAuth = options.auth !== false;
  if (wantsAuth) {
    const token = getAccessToken();
    if (token) headers.set("Authorization", `Bearer ${token}`);
  }
  const res = await fetch(`${API_BASE}${path}`, { ...options, headers });
  if (res.status === 401 && wantsAuth) {
    if (!options._retried) {
      const outcome = await refreshAccessToken();
      if (outcome === "refreshed") {
        return api<T>(path, { ...options, _retried: true });
      }
      // Couldn't reach the auth server — keep the session and let the caller
      // retry rather than bouncing the user to the login screen on a blip.
      if (outcome === "unavailable") {
        throw new Error("Network unavailable — could not refresh session.");
      }
    }
    notifySessionExpired();
    throw new SessionExpiredError(await parseErrorMessage(res));
  }
  if (res.status === 423) {
    const body = await parseErrorBody(res);
    throw new AccountLockedError(
      body?.error?.message ?? "Account temporarily locked",
      body?.error?.locked_until ?? "",
    );
  }
  if (res.status === 409) {
    throw new ConflictError(await parseErrorMessage(res));
  }
  if (!res.ok) {
    throw new Error(await parseErrorMessage(res));
  }
  if (res.status === 204) return undefined as T;
  return res.json() as Promise<T>;
}

export type LoginResult = { status: "ok" } | { status: "mfa_required"; challenge: string };

export async function login(email: string, password: string): Promise<LoginResult> {
  const outcome = await api<LoginOutcome>("/auth/login", {
    method: "POST",
    auth: false,
    body: JSON.stringify({ email, password }),
  });
  if (outcome.mfa_required && outcome.mfa_challenge) {
    return { status: "mfa_required", challenge: outcome.mfa_challenge };
  }
  if (!outcome.tokens) throw new Error("Login failed: no tokens issued");
  persistTokens(outcome.tokens);
  return { status: "ok" };
}

export async function signup(input: {
  company_name: string;
  owner_name: string;
  email: string;
  password: string;
}): Promise<LoginResult> {
  const outcome = await api<LoginOutcome>("/auth/signup", {
    method: "POST",
    auth: false,
    body: JSON.stringify(input),
  });
  if (!outcome.tokens) throw new Error("Signup failed: no tokens issued");
  persistTokens(outcome.tokens);
  return { status: "ok" };
}

export async function verifyMfaLogin(challenge: string, code: string) {
  const tokens = await api<TokenPair>("/auth/mfa/verify", {
    method: "POST",
    auth: false,
    body: JSON.stringify({ mfa_challenge: challenge, code }),
  });
  persistTokens(tokens);
}

export async function getMfaStatus() {
  return api<{ enabled: boolean }>("/auth/mfa/status");
}

export async function setupMfa() {
  return api<{ secret: string; otpauth_url: string }>("/auth/mfa/setup", { method: "POST" });
}

export async function enableMfa(code: string) {
  return api<{ backup_codes: string[] }>("/auth/mfa/enable", {
    method: "POST",
    body: JSON.stringify({ code }),
  });
}

export async function disableMfa(password: string) {
  return api<void>("/auth/mfa/disable", {
    method: "POST",
    body: JSON.stringify({ password }),
  });
}

export async function forgotPassword(email: string) {
  return api<{ message: string }>("/auth/forgot-password", {
    method: "POST",
    auth: false,
    body: JSON.stringify({ email }),
  });
}

export async function resetPassword(token: string, password: string) {
  return api<{ message: string }>("/auth/reset-password", {
    method: "POST",
    auth: false,
    body: JSON.stringify({ token, password }),
  });
}

export async function getMe() {
  return api<UserProfile>("/me");
}

export async function listRepairs(params?: {
  status?: string;
  q?: string;
  branch_id?: string;
  technician_id?: string;
}) {
  const q = new URLSearchParams();
  if (params?.status) q.set("status", params.status);
  if (params?.q) q.set("q", params.q);
  if (params?.branch_id) q.set("branch_id", params.branch_id);
  if (params?.technician_id) q.set("technician_id", params.technician_id);
  const qs = q.toString();
  return api<{ items: RepairJob[] }>(`/repairs${qs ? `?${qs}` : ""}`);
}

export async function listTrashedRepairs() {
  return api<{ items: RepairJob[] }>("/repairs/trash");
}

export async function trashRepair(id: string) {
  return api<void>(`/repairs/${id}`, { method: "DELETE" });
}

export async function restoreRepair(id: string) {
  return api<void>(`/repairs/${id}/restore`, { method: "POST", body: "{}" });
}

export async function purgeRepair(id: string) {
  return api<void>(`/repairs/${id}/purge`, { method: "DELETE" });
}

export type Customer = {
  id: string;
  full_name: string;
  phone?: string;
  email?: string;
};

export type Device = {
  id: string;
  customer_id?: string;
  anonymous: boolean;
  kind: string;
  brand?: string;
  model?: string;
  imei?: string;
  serial_number?: string;
};

export type RepairNote = {
  id: string;
  note: string;
  created_at: string;
  created_by?: string;
  author_name?: string;
};

export type SearchResult = {
  type: "customer" | "repair" | "order";
  id: string;
  title: string;
  subtitle?: string;
  url: string;
};

export async function globalSearch(q: string) {
  return api<{ results: SearchResult[] }>(`/search?q=${encodeURIComponent(q)}`);
}

export async function listCustomers(q?: string) {
  const qs = q ? `?q=${encodeURIComponent(q)}` : "";
  return api<{ items: Customer[] }>(`/customers${qs}`);
}

export async function getCustomer(id: string) {
  return api<{ customer: Customer; devices: Device[]; repairs: RepairJob[] }>(`/customers/${id}`);
}

export async function createCustomer(body: { full_name: string; phone?: string; email?: string }) {
  return api<Customer>("/customers", { method: "POST", body: JSON.stringify(body) });
}

export async function updateCustomer(
  id: string,
  body: { full_name: string; phone?: string | null; email?: string | null },
) {
  return api<Customer>(`/customers/${id}`, { method: "PATCH", body: JSON.stringify(body) });
}

export async function createDevice(body: {
  customer_id?: string;
  anonymous?: boolean;
  kind: string;
  brand?: string;
  model?: string;
  imei?: string;
  serial_number?: string;
}) {
  return api<Device>("/devices", { method: "POST", body: JSON.stringify(body) });
}

export async function createRepair(body: {
  branch_id: string;
  customer_id?: string;
  device_id: string;
  problem_summary: string;
  technician_id?: string;
  labor_amount?: number;
  promised_by?: string;
  customer_waiting?: boolean;
  estimated_wait_minutes?: number;
  intake_accessories?: string[];
  intake_condition?: string;
  device_passcode?: string;
  service_type?: "repair" | "quick_replacement";
  customer_credit?: boolean;
  credit_due_date?: string;
}) {
  return api<RepairJob>("/repairs", { method: "POST", body: JSON.stringify(body) });
}

export type IntakeResult = {
  customer?: Customer;
  device: Device;
  repair: RepairJob;
  estimate?: RepairEstimate;
};

/** Atomic counter intake — customer + device + job (+ optional estimate) in one request. */
export async function intakeRepair(body: {
  branch_id: string;
  customer_id?: string;
  anonymous?: boolean;
  customer_name?: string;
  customer_phone?: string;
  device_kind: string;
  brand?: string;
  model?: string;
  imei?: string;
  serial_number?: string;
  problem_summary: string;
  condition_tags?: string[];
  estimate_labor_amount?: number;
  estimate_parts_amount?: number;
  technician_id?: string;
}) {
  return api<IntakeResult>("/repairs/intake", { method: "POST", body: JSON.stringify(body) });
}

export async function updateRepairSchedule(id: string, promisedBy?: string) {
  return api<RepairJob>(`/repairs/${id}/schedule`, {
    method: "PATCH",
    body: JSON.stringify({ promised_by: promisedBy || null }),
  });
}

export async function updateRepairDetails(
  repairId: string,
  body: {
    expected_version: number;
    problem_summary?: string;
    device_kind?: string;
    device_brand?: string;
    device_model?: string;
    device_imei?: string;
    device_serial?: string;
    customer_id?: string;
    anonymous?: boolean;
    reason: string;
  },
) {
  return api<RepairJob>(`/repairs/${repairId}`, { method: "PATCH", body: JSON.stringify(body) });
}

export async function revealRepairPasscode(id: string) {
  return api<{ passcode: string }>(`/repairs/${id}/passcode/reveal`, { method: "POST", body: "{}" });
}

export async function createRepairRework(id: string, reason: string) {
  return api<RepairJob>(`/repairs/${id}/rework`, {
    method: "POST",
    body: JSON.stringify({ reason }),
  });
}

export async function listRepairNotes(repairId: string) {
  return api<{ items: RepairNote[] }>(`/repairs/${repairId}/notes`);
}

export async function addRepairNote(repairId: string, note: string) {
  return api<RepairNote>(`/repairs/${repairId}/notes`, {
    method: "POST",
    body: JSON.stringify({ note }),
  });
}

export async function listRiskAlerts(status = "open") {
  const q = status ? `?status=${encodeURIComponent(status)}` : "";
  return api<{ items: RiskAlert[] }>(`/risk/alerts${q}`);
}

export async function ackRiskAlert(id: string) {
  return api<RiskAlert>(`/risk/alerts/${id}/ack`, { method: "POST", body: "{}" });
}

export async function resolveRiskAlert(id: string) {
  return api<RiskAlert>(`/risk/alerts/${id}/resolve`, { method: "POST", body: "{}" });
}

export type RepairStatusEvent = {
  status: string;
  note?: string;
  at: string;
  by?: string;
};

export type WorkAuthorization = {
  authorized_at?: string;
  authorized_by?: string;
  source?: string;
  authorized_amount?: number;
  variance_reason?: string;
};

export type RepairJob = {
  id: string;
  job_number?: number;
  job_code?: string;
  pickup_code?: string;
  branch_id: string;
  customer_id?: string;
  customer_name?: string;
  customer?: Customer;
  device_id: string;
  device?: Device;
  technician_id?: string;
  status: string;
  problem_summary: string;
  labor_amount?: number;
  sale_lines_total?: number;
  sale_lines?: JobSaleLine[];
  authorized_amount?: number;
  approved_estimate_total?: number;
  pending_estimate_total?: number;
  paid_total?: number;
  amount_due?: number;
  balance_due?: number;
  quoted_value?: number;
  created_at?: string;
  promised_by?: string;
  customer_waiting?: boolean;
  estimated_wait_minutes?: number;
  intake_accessories?: string[];
  intake_condition?: string;
  has_device_passcode?: boolean;
  parent_job_id?: string;
  parent_job_code?: string;
  rework_reason?: string;
  deleted_at?: string;
  deleted_by?: string;
  closure_reason?: string;
  closed_at?: string;
  version?: number;
  authorization?: WorkAuthorization;
  handover?: Handover | null;
  timeline?: RepairStatusEvent[];
  next_statuses?: string[];
  closure_reasons?: Record<string, string[]>;
};

export type SupplierIssue = {
  id: string;
  part_request_id: string;
  repair_job_id: string;
  supplier_id: string;
  auth_code: string;
  status: string;
  unit_cost: number;
  collected_at?: string;
  reconciliation_status: string;
};

export type PartRequest = {
  id: string;
  repair_job_id: string;
  job_code?: string;
  status: string;
  description: string;
  quantity: number;
  assigned_supplier_id?: string;
  quote_status?: string;
  quotes?: PartRequestQuote[];
  issue?: SupplierIssue;
};

export type PartRequestQuote = {
  id: string;
  part_request_id: string;
  supplier_id: string;
  unit_cost: number;
  notes?: string;
  status: string;
  created_at: string;
  decided_at?: string;
};

export type RepairEstimate = {
  id: string;
  repair_job_id: string;
  total_amount: number;
  labor_amount?: number;
  parts_amount?: number;
  currency: string;
  notes?: string;
  status: string;
  expires_at?: string;
  decided_at?: string;
  created_at: string;
};

export type Payment = {
  id: string;
  method: string;
  amount: number;
  status: string;
  created_at?: string;
  checkout_request_id?: string;
  phone?: string;
  account_reference?: string;
  payable_type?: string;
  payable_id?: string;
  job_code?: string;
  customer_id?: string;
  customer_name?: string;
  sale_label?: string;
};

export async function getRepair(id: string) {
  return api<RepairJob>(`/repairs/${id}`);
}

export async function resendRepairIntakeSMS(id: string) {
  return api<{ status: string; template_key: string; phone: string }>(`/repairs/${id}/sms/resend-intake`, {
    method: "POST",
    body: "{}",
  });
}

export async function sendCustomSMS(body: {
  phone: string;
  message: string;
  customer_id?: string;
  repair_job_id?: string;
}) {
  return api<{ id: string; status: string }>("/sms/send", {
    method: "POST",
    body: JSON.stringify({
      phone: body.phone,
      body: body.message,
      customer_id: body.customer_id,
      repair_job_id: body.repair_job_id,
    }),
  });
}

export async function assignRepair(id: string, technicianId: string) {
  return api<RepairJob>(`/repairs/${id}/assign`, {
    method: "POST",
    body: JSON.stringify({ technician_id: technicianId }),
  });
}

export async function changeRepairStatus(
  id: string,
  body: {
    status: string;
    note?: string;
    labor_amount?: number;
    closure_reason?: string;
    variance_reason?: string;
    force?: boolean;
  },
) {
  return api<RepairJob>(`/repairs/${id}/status`, { method: "POST", body: JSON.stringify(body) });
}

export async function authorizeRepairWork(id: string, body: { amount?: number; note: string }) {
  return api<RepairJob>(`/repairs/${id}/authorize-work`, { method: "POST", body: JSON.stringify(body) });
}

/** Write the job charge down to successful payments so the remaining balance clears. */
export async function acceptPaidCharge(id: string) {
  return api<RepairJob>(`/repairs/${id}/accept-paid-charge`, { method: "POST", body: "{}" });
}

export type Handover = {
  id: string;
  repair_job_id: string;
  collected_by_name: string;
  relationship: string;
  id_number?: string;
  verification_method: "otp" | "staff_vouched";
  verified_at?: string;
  released_by?: string;
  note?: string;
  created_at: string;
};

export async function sendHandoverCode(repairId: string) {
  return api<{ status: string }>(`/repairs/${repairId}/handover/send-code`, { method: "POST", body: "{}" });
}

export async function recordHandover(
  repairId: string,
  body: {
    collected_by_name: string;
    relationship?: string;
    id_number?: string;
    note?: string;
    otp_code?: string;
    pickup_code?: string;
  },
) {
  return api<Handover>(`/repairs/${repairId}/handover`, { method: "POST", body: JSON.stringify(body) });
}

export type PickupLookup = {
  id: string;
  job_code?: string;
  pickup_code?: string;
  status: string;
  problem_summary?: string;
  balance_due?: number;
  can_release?: boolean;
  customer_name?: string;
};

export async function lookupRepairByPickupCode(code: string) {
  const q = encodeURIComponent(code.trim());
  return api<PickupLookup>(`/repairs/by-pickup-code?code=${q}`);
}

export async function collectRepairByPickupCode(body: {
  pickup_code: string;
  collected_by_name: string;
  relationship?: string;
  note?: string;
}) {
  return api<Handover>("/repairs/collect", { method: "POST", body: JSON.stringify(body) });
}

export type JobCost = {
  id: string;
  cost_type: string;
  description: string;
  quantity: number;
  unit_cost: number;
  amount: number;
  created_at: string;
};

export type JobMargin = {
  labor_amount: number;
  parts_cost: number;
  margin: number;
  margin_pct?: number;
  costs: JobCost[];
  unpriced_parts: number;
};

export async function getRepairMargin(id: string) {
  return api<JobMargin>(`/repairs/${id}/margin`);
}

export async function issuePartFromStock(
  partRequestId: string,
  body: { variant_id: string; location_id: string; quantity?: number },
) {
  return api<PartRequest>(`/part-requests/${partRequestId}/issue-from-stock`, {
    method: "POST",
    body: JSON.stringify(body),
  });
}

export async function listPartRequests(repairJobId: string) {
  return api<{ items: PartRequest[] }>(`/part-requests?repair_job_id=${repairJobId}`);
}

export async function listAllPartRequests(params?: { status?: string; repair_job_id?: string }) {
  const q = new URLSearchParams();
  if (params?.status) q.set("status", params.status);
  if (params?.repair_job_id) q.set("repair_job_id", params.repair_job_id);
  const qs = q.toString();
  return api<{ items: PartRequest[] }>(`/part-requests${qs ? `?${qs}` : ""}`);
}

export async function createPartRequest(body: {
  repair_job_id: string;
  branch_id: string;
  description: string;
  quantity?: number;
  supplier_id?: string;
}) {
  return api<PartRequest>("/part-requests", { method: "POST", body: JSON.stringify(body) });
}

export async function approvePartRequest(id: string, unitCost?: number) {
  return api<SupplierIssue>(`/part-requests/${id}/approve`, {
    method: "POST",
    body: JSON.stringify({ unit_cost: unitCost ?? 0 }),
  });
}

export async function assignPartRequest(id: string, supplierId: string) {
  return api<PartRequest>(`/part-requests/${id}/assign`, {
    method: "POST",
    body: JSON.stringify({ supplier_id: supplierId }),
  });
}

export async function acceptPartRequestQuote(partRequestId: string, quoteId: string) {
  return api<{ issue: SupplierIssue; auth_code: string; qr_payload: string }>(
    `/part-requests/${partRequestId}/quotes/${quoteId}/accept`,
    { method: "POST", body: "{}" },
  );
}

export async function listRepairEstimates(repairId: string) {
  return api<{ items: RepairEstimate[] }>(`/repairs/${repairId}/estimates`);
}

export async function createRepairEstimate(
  repairId: string,
  body: { total_amount: number; notes?: string; expires_hours?: number },
) {
  return api<RepairEstimate>(`/repairs/${repairId}/estimates`, {
    method: "POST",
    body: JSON.stringify(body),
  });
}

export async function collectSupplierIssue(id: string, authCode: string) {
  return api<SupplierIssue>(`/supplier-issues/${id}/collect`, {
    method: "POST",
    body: JSON.stringify({ auth_code: authCode }),
  });
}

export async function listRepairPayments(repairId: string) {
  return api<{ items: Payment[] }>(`/payments?payable_type=repair&payable_id=${repairId}`);
}

export async function createPayment(body: {
  method?: string;
  amount?: number;
  payable_type: string;
  payable_id: string;
  branch_id?: string;
  currency?: string;
  phone?: string;
  account_reference?: string;
  customer_id?: string;
  tenders?: Array<{ method: string; amount: number; phone?: string }>;
}) {
  return api<Payment | { items: Payment[] }>("/payments", { method: "POST", body: JSON.stringify(body) });
}

/** Reconcile STK via Daraja Query API (typed provider_ref is ignored server-side). */
export async function reconcileMpesaPayment(id: string) {
  return api<Payment>(`/payments/${id}/mpesa/reconcile`, {
    method: "POST",
    body: JSON.stringify({}),
  });
}

/** @deprecated use reconcileMpesaPayment */
export async function confirmMpesaPayment(id: string, _providerRef?: string) {
  return reconcileMpesaPayment(id);
}

export async function getPayment(id: string) {
  return api<Payment>(`/payments/${id}`);
}

export type CashHandover = {
  id: string;
  branch_id: string;
  from_user_id: string;
  to_user_id?: string;
  amount: number;
  status: string;
  shortage_amount?: number;
  created_at: string;
  confirmed_at?: string;
};

export async function listCashHandovers(status?: string) {
  const q = status ? `?status=${encodeURIComponent(status)}` : "";
  return api<{ items: CashHandover[] }>(`/cash/handovers${q}`);
}

export async function pendingCashTotal() {
  return api<{ amount: number }>("/cash/pending-total");
}

export async function requestCashHandover(body: { branch_id?: string; to_user_id?: string; amount: number }) {
  return api<CashHandover>("/cash/handovers", { method: "POST", body: JSON.stringify(body) });
}

export async function confirmCashHandover(id: string, countedAmount?: number) {
  const body =
    countedAmount === undefined ? "{}" : JSON.stringify({ counted_amount: countedAmount });
  return api<CashHandover>(`/cash/handovers/${id}/confirm`, { method: "POST", body });
}

export async function listAllPayments() {
  return api<{ items: Payment[] }>("/payments");
}

export type Refund = {
  id: string;
  payment_id: string;
  amount: number;
  status: string;
  reason?: string;
  created_by?: string;
  created_at?: string;
};

export async function listRefunds(status?: string) {
  const q = status ? `?status=${encodeURIComponent(status)}` : "";
  return api<{ items: Refund[] }>(`/refunds${q}`);
}

export async function createRefund(body: { payment_id: string; amount: number; reason?: string }) {
  return api<Refund>("/refunds", { method: "POST", body: JSON.stringify(body) });
}

export async function approveRefund(id: string) {
  return api<Refund>(`/refunds/${id}/approve`, { method: "POST", body: "{}" });
}

export type C2BTransaction = {
  id: string;
  payment_id?: string;
  trans_id?: string;
  amount: number;
  business_shortcode?: string;
  bill_ref_number?: string;
  msisdn?: string;
  status: string;
  created_at: string;
};

export async function listC2BTransactions(status?: string) {
  const q = status ? `?status=${encodeURIComponent(status)}` : "";
  return api<{ items: C2BTransaction[] }>(`/payments/c2b${q}`);
}

export async function matchC2BTransaction(id: string, paymentId: string) {
  return api<Payment>(`/payments/c2b/${id}/match`, {
    method: "POST",
    body: JSON.stringify({ payment_id: paymentId }),
  });
}

/** Resolves an unmatched C2B deposit by creating a one-line sale for the chosen
 * product/quantity — deducting stock — and allocating the full received amount
 * to it, instead of matching against a pre-existing payment. */
export async function matchC2BToNewSale(
  id: string,
  body: { branch_id: string; location_id: string; variant_id: string; quantity: number },
) {
  return api<Payment>(`/payments/c2b/${id}/match-sale`, {
    method: "POST",
    body: JSON.stringify(body),
  });
}

export type ReportSummary = {
  generated_at: string;
  period_days: number;
  repairs_open: number;
  repairs_ready: number;
  repairs_completed_period: number;
  repairs_waiting_parts: number;
  repairs_closed_period: number;
  repairs_overdue: number;
  repairs_pipeline_value: number;
  repairs_collectible_value: number;
  repairs_unpriced: number;
  payments_allocated_period: number;
  payments_cash_provisional: number;
  payments_stk_pending: number;
  sales_completed_period: number;
  sales_count_period: number;
  handovers_open: number;
  shortage_amount_period: number;
  supplier_credit_outstanding: number;
  risk_open_total: number;
  risk_orphan_parts: number;
  risk_cash_shortage: number;
  risk_unverified_payment: number;
  risk_stuck_jobs: number;
};

export async function getReportSummary(days = 7) {
  return api<ReportSummary>(`/reports/summary?days=${days}`);
}

export type DailyMetric = {
  date: string;
  payments_allocated: number;
  sales_completed: number;
  repairs_completed: number;
};

export type TechnicianMetric = {
  technician_id: string;
  name: string;
  open_jobs: number;
  completed_period: number;
  labor_amount_period: number;
};

export type BranchMetric = {
  branch_id: string;
  name: string;
  open_jobs: number;
  completed_period: number;
  sales_total_period: number;
};

export type RepairProfitability = {
  jobs: number;
  labor_revenue: number;
  parts_cost: number;
  margin: number;
  margin_pct?: number;
  loss_making_jobs: number;
  jobs_with_unpriced_parts: number;
};

export type ClosureMetric = {
  status: string;
  reason: string;
  count: number;
  lost_labor_value: number;
};

export type OperationsReport = {
  generated_at: string;
  period_days: number;
  daily: DailyMetric[];
  by_technician: TechnicianMetric[];
  by_branch: BranchMetric[];
  closures: ClosureMetric[];
  repair_profitability: RepairProfitability;
};

export async function getOperationsReport(days = 7) {
  return api<OperationsReport>(`/reports/operations?days=${days}`);
}

export type AuditEvent = {
  id: string;
  branch_id?: string;
  actor_id?: string;
  actor_name?: string;
  action: string;
  entity_type: string;
  entity_id?: string;
  previous_value?: Record<string, unknown>;
  new_value?: Record<string, unknown>;
  reason?: string;
  correlation_id?: string;
  created_at: string;
};

export async function listAuditEvents(params?: {
  q?: string;
  action?: string;
  entity_type?: string;
  limit?: number;
}) {
  const q = new URLSearchParams();
  if (params?.q) q.set("q", params.q);
  if (params?.action) q.set("action", params.action);
  if (params?.entity_type) q.set("entity_type", params.entity_type);
  if (params?.limit) q.set("limit", String(params.limit));
  const qs = q.toString();
  return api<{ items: AuditEvent[] }>(`/audit/events${qs ? `?${qs}` : ""}`);
}

export type ErrorEvent = {
  id: string;
  tenant_id?: string;
  method: string;
  route: string;
  status: number;
  message: string;
  stack?: string;
  correlation_id?: string;
  created_at: string;
};

export async function listErrorEvents(limit = 100) {
  return api<{ items: ErrorEvent[] }>(`/platform/errors?limit=${limit}`);
}

export type PaymentProviderSettings = {
  tenant_id: string;
  environment: string;
  mpesa_enabled: boolean;
  mpesa_shortcode: string;
  mpesa_consumer_key: string;
  mpesa_callback_url: string;
  consumer_secret_set: boolean;
  passkey_set: boolean;
  bank_enabled: boolean;
  bank_paybill: string;
  bank_account: string;
  configured: boolean;
  bank_configured: boolean;
  updated_at: string;
};

export async function getPaymentSettings() {
  return api<PaymentProviderSettings>("/payments/settings");
}

export async function updatePaymentSettings(body: {
  environment?: string;
  mpesa_enabled?: boolean;
  mpesa_shortcode?: string;
  mpesa_consumer_key?: string;
  mpesa_consumer_secret?: string;
  mpesa_passkey?: string;
  mpesa_callback_url?: string;
  bank_enabled?: boolean;
  bank_paybill?: string;
  bank_account?: string;
}) {
  return api<PaymentProviderSettings>("/payments/settings", {
    method: "PUT",
    body: JSON.stringify(body),
  });
}

export type SMSSettings = {
  tenant_id: string;
  provider: string;
  enabled: boolean;
  sender_id: string;
  base_url: string;
  api_key_set: boolean;
  configured: boolean;
  updated_at: string;
};

export type SMSTemplate = {
  key: string;
  label: string;
  description: string;
  audience: string;
  helpers: string[];
  default_body: string;
  body: string;
  is_customized: boolean;
  updated_at?: string;
};

export async function getSMSSettings() {
  return api<SMSSettings>("/sms/settings");
}

export async function updateSMSSettings(body: {
  enabled?: boolean;
  provider?: string;
  api_key?: string;
  sender_id?: string;
  base_url?: string;
}) {
  return api<SMSSettings>("/sms/settings", {
    method: "PUT",
    body: JSON.stringify(body),
  });
}

export async function listSMSTemplates() {
  return api<{ items: SMSTemplate[] }>("/sms/templates");
}

export async function updateSMSTemplate(key: string, body: string) {
  return api<SMSTemplate>(`/sms/templates/${encodeURIComponent(key)}`, {
    method: "PUT",
    body: JSON.stringify({ body }),
  });
}

export type WhatsAppSettings = {
  tenant_id: string;
  enabled: boolean;
  notify_customers: boolean;
  notify_suppliers: boolean;
  also_send_sms: boolean;
  service_configured: boolean;
  connected: boolean;
  connection_status: string;
  session_id: string;
  updated_at: string;
};

export type WhatsAppQR = {
  success?: boolean;
  status: string;
  qr?: string | null;
  message?: string;
  user?: unknown;
  error?: string;
};

export async function getWhatsAppSettings() {
  return api<WhatsAppSettings>("/whatsapp/settings");
}

export async function updateWhatsAppSettings(body: {
  enabled?: boolean;
  notify_customers?: boolean;
  notify_suppliers?: boolean;
  also_send_sms?: boolean;
}) {
  return api<WhatsAppSettings>("/whatsapp/settings", {
    method: "PUT",
    body: JSON.stringify(body),
  });
}

export async function getWhatsAppQR() {
  return api<WhatsAppQR>("/whatsapp/qr");
}

export async function disconnectWhatsApp() {
  return api<{ ok: boolean }>("/whatsapp/disconnect", { method: "POST" });
}

export async function reconnectWhatsApp() {
  return api<{ ok: boolean }>("/whatsapp/reconnect", { method: "POST" });
}

export type Supplier = {
  id: string;
  name: string;
  phone?: string;
  outstanding_credit: number;
  pending_issue_count: number;
};

export type SupplierContact = {
  id: string;
  supplier_id: string;
  email: string;
  phone?: string;
  display_name: string;
  status: string;
  supplier_name?: string;
};

export type SupplierInviteResult = {
  contact: SupplierContact;
  invite_token: string;
  expires_at: string;
};

export type PendingSupplierIssue = {
  id: string;
  part_request_id: string;
  repair_job_id: string;
  job_code?: string;
  supplier_id: string;
  supplier_name: string;
  auth_code: string;
  status: string;
  unit_cost: number;
  collected_at?: string;
  reconciliation_status: string;
  description: string;
};

export type CreditEntry = {
  id: string;
  supplier_id: string;
  supplier_issue_id?: string;
  amount: number;
  entry_type: string;
  created_at: string;
};

export async function listSuppliers() {
  return api<{ items: Supplier[] }>("/suppliers");
}

export async function createSupplier(body: { name: string; phone?: string }) {
  return api<Supplier>("/suppliers", { method: "POST", body: JSON.stringify(body) });
}

export async function inviteSupplierContact(
  supplierId: string,
  body: { email: string; display_name: string; phone?: string },
) {
  return api<SupplierInviteResult>(`/suppliers/${supplierId}/contacts/invite`, {
    method: "POST",
    body: JSON.stringify(body),
  });
}

export async function listPendingReconciliation() {
  return api<{ items: PendingSupplierIssue[] }>("/supplier-issues/pending-reconciliation");
}

export async function listSupplierCredit(supplierId: string) {
  return api<{ items: CreditEntry[] }>(`/suppliers/${supplierId}/credit`);
}

export async function reconcileSupplierIssue(id: string) {
  return api<SupplierIssue>(`/supplier-issues/${id}/reconcile`, { method: "POST", body: "{}" });
}

export async function listOrphanIssues() {
  return api<{ items: SupplierIssue[] }>("/supplier-issues/orphans");
}

export type RiskAlert = {
  id: string;
  kind: string;
  severity: string;
  title: string;
  entity_type?: string;
  entity_id?: string;
  status: string;
  details?: Record<string, unknown>;
};

export type Branch = {
  id: string;
  name: string;
  code: string;
  location?: string;
  phone?: string;
  hours?: string;
  map_url?: string;
};

export type EmployeeProfile = {
  user_id: string;
  employee_code?: string;
  phone?: string;
  is_technician: boolean;
  commission_enabled: boolean;
  commission_type: string;
  percent_bps?: number;
  fixed_amount?: number;
};

export type StaffUser = {
  id: string;
  email: string;
  display_name: string;
  status: string;
  roles: string[];
  branch_ids: string[];
  profile?: EmployeeProfile;
  created_at: string;
};

export type RoleInfo = {
  id: string;
  key: string;
  name: string;
  description?: string;
  is_system?: boolean;
  permissions: string[];
  created_at?: string;
};

export type PermissionDef = {
  code: string;
  description: string;
  category: string;
};

export function listRoles() {
  return api<{ items: RoleInfo[] }>("/roles");
}

export function createRole(body: { key: string; name: string; description?: string; permissions: string[] }) {
  return api<RoleInfo>("/roles", { method: "POST", body: JSON.stringify(body) });
}

export function updateRole(id: string, body: { name?: string; description?: string; permissions?: string[] }) {
  return api<RoleInfo>(`/roles/${id}`, { method: "PATCH", body: JSON.stringify(body) });
}

export function deleteRole(id: string) {
  return api<void>(`/roles/${id}`, { method: "DELETE" });
}

export type IntakePreset = {
  id: string;
  kind: "condition_tag" | "issue" | string;
  label: string;
  sort_order: number;
  is_system: boolean;
  is_active: boolean;
};

export function listIntakePresets(kind?: string, opts?: { includeInactive?: boolean }) {
  const q = new URLSearchParams();
  if (kind) q.set("kind", kind);
  if (opts?.includeInactive) q.set("include_inactive", "true");
  const qs = q.toString();
  return api<{ items: IntakePreset[] }>(`/intake-presets${qs ? `?${qs}` : ""}`);
}

export function createIntakePreset(body: { kind: string; label: string }) {
  return api<IntakePreset>("/intake-presets", { method: "POST", body: JSON.stringify(body) });
}

export function updateIntakePreset(id: string, body: { label?: string; is_active?: boolean }) {
  return api<IntakePreset>(`/intake-presets/${id}`, { method: "PATCH", body: JSON.stringify(body) });
}

export function deleteIntakePreset(id: string) {
  return api<void>(`/intake-presets/${id}`, { method: "DELETE" });
}

export function listPermissions() {
  return api<{ items: PermissionDef[] }>("/permissions");
}

export function createPermission(body: { code: string; description?: string; category?: string }) {
  return api<PermissionDef>("/permissions", { method: "POST", body: JSON.stringify(body) });
}

export type CommissionEntry = {
  id: string;
  user_id: string;
  repair_job_id: string;
  entry_type: string;
  base_amount: number;
  commission_amount: number;
  currency: string;
  status: string;
  created_at: string;
  technician_name?: string;
};

export function listUsers(params?: { role?: string; status?: string }) {
  const q = new URLSearchParams();
  if (params?.role) q.set("role", params.role);
  if (params?.status) q.set("status", params.status);
  const qs = q.toString();
  return api<{ items: StaffUser[] }>(`/users${qs ? `?${qs}` : ""}`);
}

export function getUser(id: string) {
  return api<StaffUser>(`/users/${id}`);
}

export function createUser(body: Record<string, unknown>) {
  return api<StaffUser>("/users", { method: "POST", body: JSON.stringify(body) });
}

export function updateUser(id: string, body: Record<string, unknown>) {
  return api<StaffUser>(`/users/${id}`, { method: "PATCH", body: JSON.stringify(body) });
}

export function setUserCommission(id: string, body: Record<string, unknown>) {
  return api<StaffUser>(`/users/${id}/commission`, { method: "PUT", body: JSON.stringify(body) });
}

export function listBranches() {
  return api<{ items: Branch[] }>("/branches");
}

export function createBranch(body: {
  name: string;
  code: string;
  location?: string;
  phone?: string;
  hours?: string;
  map_url?: string;
}) {
  return api<Branch>("/branches", { method: "POST", body: JSON.stringify(body) });
}

export function updateBranch(
  id: string,
  body: { name?: string; code?: string; location?: string; phone?: string; hours?: string; map_url?: string },
) {
  return api<Branch>(`/branches/${id}`, { method: "PATCH", body: JSON.stringify(body) });
}

export function deleteBranch(id: string) {
  return api<void>(`/branches/${id}`, { method: "DELETE" });
}

export function listTechnicians() {
  return api<{ items: StaffUser[] }>("/technicians");
}

export function listCommissions(params?: { status?: string; user_id?: string }) {
  const q = new URLSearchParams();
  if (params?.status) q.set("status", params.status);
  if (params?.user_id) q.set("user_id", params.user_id);
  const qs = q.toString();
  return api<{ items: CommissionEntry[] }>(`/commissions${qs ? `?${qs}` : ""}`);
}

export function approveCommission(id: string) {
  return api<CommissionEntry>(`/commissions/${id}/approve`, { method: "POST", body: "{}" });
}

export function markCommissionPaid(id: string) {
  return api<CommissionEntry>(`/commissions/${id}/mark-paid`, { method: "POST", body: "{}" });
}

export type CatalogItem = {
  variant_id: string;
  product_id: string;
  product_name: string;
  brand?: string;
  sku: string;
  sell_price: number;
  available_qty: number;
  location_id?: string;
};

export type StockLocation = {
  id: string;
  branch_id?: string;
  name: string;
  location_type: string;
};

export type SaleItem = {
  id?: string;
  variant_id: string;
  /** Set (with variant_id all-zero) for a quick-sale line not in the catalog. */
  description?: string;
  quantity: number;
  unit_price: number;
  line_total?: number;
  /** Internal only — never present on the customer receipt. */
  unit_cost?: number;
  supplier_id?: string;
  margin?: number;
};

export type Sale = {
  id: string;
  branch_id: string;
  channel: string;
  status: string;
  subtotal: number;
  total: number;
  created_at?: string;
  items?: SaleItem[];
};

export type POSCheckoutResult = {
  sale: Sale;
  payment: Payment;
  completed: boolean;
};

export type Product = {
  id: string;
  name: string;
  brand?: string;
  category_id?: string;
  category?: string;
  category_path?: string;
  description?: string;
  image_url?: string;
  has_image?: boolean;
  image_updated_at?: string;
  pos_visible?: boolean;
  online_visible?: boolean;
  featured?: boolean;
  new_arrival?: boolean;
  bestseller?: boolean;
  storefront_sort_order?: number;
};

export type InventoryCategory = {
  id: string;
  name: string;
  parent_id?: string;
  path: string;
  depth: number;
  children?: InventoryCategory[];
};

export type Variant = {
  id: string;
  product_id: string;
  sku: string;
  sell_price: number;
  cost_price: number;
};

export async function listProducts() {
  return api<{ items: Product[] }>("/products");
}

export async function listCategories() {
  return api<{ items: InventoryCategory[] }>("/categories");
}

export async function createCategory(body: { name: string; parent_id?: string }) {
  return api<InventoryCategory>("/categories", { method: "POST", body: JSON.stringify(body) });
}

export async function updateCategory(
  id: string,
  body: { name?: string; parent_id?: string; clear_parent?: boolean },
) {
  return api<InventoryCategory>(`/categories/${id}`, { method: "PATCH", body: JSON.stringify(body) });
}

export async function deleteCategory(id: string) {
  return api<void>(`/categories/${id}`, { method: "DELETE" });
}

export async function createProduct(body: {
  name: string;
  brand?: string;
  category_id?: string;
  category?: string;
  description?: string;
  image_url?: string;
}) {
  return api<Product>("/products", { method: "POST", body: JSON.stringify(body) });
}

export async function copyProduct(id: string) {
  return api<Product>(`/products/${id}/copy`, { method: "POST", body: "{}" });
}

export async function updateProduct(
  id: string,
  body: {
    name?: string;
    brand?: string;
    category_id?: string;
    clear_category?: boolean;
    category?: string;
    description?: string;
    image_url?: string;
    online_visible?: boolean;
    featured?: boolean;
    new_arrival?: boolean;
    bestseller?: boolean;
    storefront_sort_order?: number;
  },
) {
  return api<Product>(`/products/${id}`, { method: "PATCH", body: JSON.stringify(body) });
}

export async function uploadProductImage(id: string, file: File) {
  const form = new FormData();
  form.append("file", file);
  const headers = new Headers();
  const token = getAccessToken();
  if (token) headers.set("Authorization", `Bearer ${token}`);
  const res = await fetch(`${API_BASE}/products/${id}/image`, { method: "POST", headers, body: form });
  if (!res.ok) throw new Error(await parseErrorMessage(res));
  return (await res.json()) as Product;
}

export async function deleteProductImage(id: string) {
  return api<Product>(`/products/${id}/image`, { method: "DELETE" });
}

export function productImageURL(productId: string, cacheBust?: string) {
  const base = `${API_BASE}/inventory/public/products/${encodeURIComponent(productId)}/image`;
  return cacheBust ? `${base}?v=${encodeURIComponent(cacheBust)}` : base;
}

export async function listVariants(productId?: string) {
  const q = productId ? `?product_id=${encodeURIComponent(productId)}` : "";
  return api<{ items: Variant[] }>(`/variants${q}`);
}

export async function createVariant(body: {
  product_id: string;
  sku: string;
  sell_price: number;
  cost_price?: number;
}) {
  return api<Variant>("/variants", { method: "POST", body: JSON.stringify(body) });
}

export async function updateVariant(id: string, body: { sku?: string; sell_price?: number; cost_price?: number }) {
  return api<Variant>(`/variants/${id}`, { method: "PATCH", body: JSON.stringify(body) });
}

export async function publishProduct(id: string) {
  return api<{ status: string }>(`/commerce/products/${id}/publish`, { method: "POST", body: "{}" });
}

export async function unpublishProduct(id: string) {
  return api<{ status: string }>(`/commerce/products/${id}/publish`, {
    method: "POST",
    body: JSON.stringify({ published: false }),
  });
}

// --- Storefront CMS -------------------------------------------------------

export type StorefrontSettings = {
  tenant_id: string;
  shop_display_name: string;
  page_title: string;
  has_logo: boolean;
  logo_content_type?: string;
  logo_updated_at?: string;
  color_primary: string;
  color_secondary: string;
  color_accent: string;
  topbar_help_href: string;
  topbar_support_href: string;
  topbar_contact_href: string;
  topbar_phone_label: string;
  header_promo_text: string;
  show_featured: boolean;
  show_new_arrivals: boolean;
  show_bestsellers: boolean;
  show_deals: boolean;
  show_most_viewed: boolean;
  hero_headline: string;
  hero_subtext: string;
  hero_cta_label: string;
  hero_cta_href: string;
  newsletter_headline: string;
  newsletter_subtext: string;
  footer_tagline: string;
  social_facebook: string;
  social_instagram: string;
  social_twitter: string;
  social_tiktok: string;
  contact_phone: string;
  contact_email: string;
  business_hours: string;
  app_store_url: string;
  play_store_url: string;
  enabled_currencies: string;
  trust_badge_1_title: string;
  trust_badge_1_subtext: string;
  trust_badge_2_title: string;
  trust_badge_2_subtext: string;
  trust_badge_3_title: string;
  trust_badge_3_subtext: string;
  trust_badge_4_title: string;
  trust_badge_4_subtext: string;
  pay_label_stk: string;
  pay_label_paybill: string;
  pay_label_cash: string;
  pay_hint_stk: string;
  pay_hint_paybill: string;
  pay_hint_cash: string;
  pay_cta_stk: string;
  pay_cta_paybill: string;
  pay_cta_cash: string;
  updated_at: string;
};

export async function getStorefrontSettings() {
  return api<StorefrontSettings>("/storefront/settings");
}

export async function updateStorefrontSettings(body: Partial<StorefrontSettings>) {
  return api<StorefrontSettings>("/storefront/settings", { method: "PUT", body: JSON.stringify(body) });
}

export async function uploadStorefrontLogo(file: File) {
  const form = new FormData();
  form.append("file", file);
  const headers = new Headers();
  const token = getAccessToken();
  if (token) headers.set("Authorization", `Bearer ${token}`);
  const res = await fetch(`${API_BASE}/storefront/settings/logo`, { method: "POST", headers, body: form });
  if (!res.ok) throw new Error(await parseErrorMessage(res));
  return (await res.json()) as StorefrontSettings;
}

export async function deleteStorefrontLogo() {
  return api<StorefrontSettings>("/storefront/settings/logo", { method: "DELETE" });
}

export function storefrontLogoURL(tenantId: string, cacheBust?: string) {
  const base = `${API_BASE}/storefront/public/logo/${tenantId}`;
  return cacheBust ? `${base}?v=${encodeURIComponent(cacheBust)}` : base;
}

export type StorefrontBanner = {
  id: string;
  headline: string;
  subtext: string;
  cta_label: string;
  cta_href: string;
  has_image: boolean;
  image_content_type?: string;
  image_updated_at?: string;
  placement: "hero" | "side" | "mid" | "promo_tile";
  deal_id?: string;
  deal_variant_id?: string;
  deal_price?: number;
  deal_base_price?: number;
  sort_order: number;
  active: boolean;
  created_at: string;
  updated_at: string;
};

export async function listStorefrontBanners() {
  return api<{ items: StorefrontBanner[] }>("/storefront/banners");
}

export async function createStorefrontBanner(body: {
  headline?: string;
  subtext?: string;
  cta_label?: string;
  cta_href?: string;
  placement?: "hero" | "side" | "mid" | "promo_tile";
  deal_id?: string;
  sort_order?: number;
  active?: boolean;
}) {
  return api<StorefrontBanner>("/storefront/banners", { method: "POST", body: JSON.stringify(body) });
}

export async function updateStorefrontBanner(
  id: string,
  body: {
    headline?: string;
    subtext?: string;
    cta_label?: string;
    cta_href?: string;
    placement?: "hero" | "side" | "mid" | "promo_tile";
    deal_id?: string;
    clear_deal_id?: boolean;
    sort_order?: number;
    active?: boolean;
  },
) {
  return api<StorefrontBanner>(`/storefront/banners/${id}`, { method: "PUT", body: JSON.stringify(body) });
}

export async function deleteStorefrontBanner(id: string) {
  return api<{ status: string }>(`/storefront/banners/${id}`, { method: "DELETE" });
}

export async function uploadStorefrontBannerImage(id: string, file: File) {
  const form = new FormData();
  form.append("file", file);
  const headers = new Headers();
  const token = getAccessToken();
  if (token) headers.set("Authorization", `Bearer ${token}`);
  const res = await fetch(`${API_BASE}/storefront/banners/${id}/image`, { method: "POST", headers, body: form });
  if (!res.ok) throw new Error(await parseErrorMessage(res));
  return (await res.json()) as StorefrontBanner;
}

export async function deleteStorefrontBannerImage(id: string) {
  return api<StorefrontBanner>(`/storefront/banners/${id}/image`, { method: "DELETE" });
}

export function storefrontBannerImageURL(id: string) {
  return `${API_BASE}/storefront/public/banners/${id}/image`;
}

export type StorefrontDeal = {
  id: string;
  variant_id: string;
  title: string;
  deal_price: number;
  ends_at?: string;
  active: boolean;
  sort_order: number;
  created_at: string;
  updated_at: string;
  product_id?: string;
  product_name?: string;
  sku?: string;
  base_price?: number;
  has_image?: boolean;
  image_updated_at?: string;
};

export async function listStorefrontDeals() {
  return api<{ items: StorefrontDeal[] }>("/storefront/deals");
}

export async function createStorefrontDeal(body: {
  variant_id: string;
  title?: string;
  deal_price: number;
  ends_at?: string;
  active?: boolean;
  sort_order?: number;
}) {
  return api<StorefrontDeal>("/storefront/deals", { method: "POST", body: JSON.stringify(body) });
}

export async function updateStorefrontDeal(
  id: string,
  body: {
    variant_id?: string;
    title?: string;
    deal_price?: number;
    ends_at?: string;
    clear_ends_at?: boolean;
    active?: boolean;
    sort_order?: number;
  },
) {
  return api<StorefrontDeal>(`/storefront/deals/${id}`, { method: "PUT", body: JSON.stringify(body) });
}

export async function deleteStorefrontDeal(id: string) {
  return api<{ status: string }>(`/storefront/deals/${id}`, { method: "DELETE" });
}

export type DeliveryLocation = {
  id: string;
  name: string;
  description?: string;
  fee: number;
  active: boolean;
  sort_order: number;
  created_at?: string;
  updated_at?: string;
};

export async function listDeliveryLocations() {
  return api<{ items: DeliveryLocation[] }>("/commerce/delivery-locations");
}

export async function createDeliveryLocation(body: {
  name: string;
  description?: string;
  fee: number;
  active?: boolean;
  sort_order?: number;
}) {
  return api<DeliveryLocation>("/commerce/delivery-locations", { method: "POST", body: JSON.stringify(body) });
}

export async function updateDeliveryLocation(
  id: string,
  body: {
    name?: string;
    description?: string;
    fee?: number;
    active?: boolean;
    sort_order?: number;
  },
) {
  return api<DeliveryLocation>(`/commerce/delivery-locations/${id}`, { method: "PUT", body: JSON.stringify(body) });
}

export async function deleteDeliveryLocation(id: string) {
  return api<{ status: string }>(`/commerce/delivery-locations/${id}`, { method: "DELETE" });
}

export type NewsletterSubscriber = {
  id: string;
  email: string;
  created_at: string;
};

export async function listNewsletterSubscribers() {
  return api<{ items: NewsletterSubscriber[] }>("/storefront/subscribers");
}

export type StorefrontReview = {
  id: string;
  product_id: string;
  rating: number;
  title?: string;
  body?: string;
  status: "published" | "hidden";
  created_at: string;
  product_name?: string;
  customer_name?: string;
};

export async function listStorefrontReviews(status?: string) {
  const q = status ? `?status=${encodeURIComponent(status)}` : "";
  return api<{ items: StorefrontReview[] }>(`/storefront/reviews${q}`);
}

export async function setStorefrontReviewStatus(id: string, status: "published" | "hidden") {
  return api<{ status: string }>(`/storefront/reviews/${id}/status`, {
    method: "POST",
    body: JSON.stringify({ status }),
  });
}

export type RepairAttachment = {
  id: string;
  repair_job_id: string;
  file_name: string;
  content_type: string;
  size_bytes: number;
  created_at: string;
  created_by?: string;
};

export async function listRepairAttachments(repairId: string) {
  return api<{ items: RepairAttachment[] }>(`/repairs/${repairId}/attachments`);
}

export async function uploadRepairAttachment(
  repairId: string,
  body: { file_name: string; content_type: string; data_base64: string },
) {
  return api<RepairAttachment>(`/repairs/${repairId}/attachments`, {
    method: "POST",
    body: JSON.stringify(body),
  });
}

export async function deleteRepairAttachment(repairId: string, attachmentId: string) {
  return api<void>(`/repairs/${repairId}/attachments/${attachmentId}`, { method: "DELETE" });
}

export type ReceiptPaper = "thermal80" | "thermal58" | "a4";

export type ReceiptSettings = {
  tenant_id: string;
  header_note: string;
  phone: string;
  email: string;
  website: string;
  show_logo: boolean;
  show_address: boolean;
  show_tin: boolean;
  thank_you_text: string;
  footer_text: string;
  warranty_text: string;
  show_vat_breakdown: boolean;
  show_imei: boolean;
  show_payments: boolean;
  show_balance: boolean;
  show_served_by: boolean;
  default_paper: ReceiptPaper;
  number_prefix: string;
  next_number: number;
  has_logo: boolean;
  logo_content_type?: string;
  logo_updated_at?: string;
  updated_at: string;
};

export async function getReceiptSettings() {
  return api<ReceiptSettings>("/receipt-settings");
}

export async function updateReceiptSettings(body: Partial<ReceiptSettings>) {
  return api<ReceiptSettings>("/receipt-settings", {
    method: "PUT",
    body: JSON.stringify(body),
  });
}

export async function uploadReceiptLogo(file: File) {
  const form = new FormData();
  form.append("file", file);
  const headers = new Headers();
  const token = getAccessToken();
  if (token) headers.set("Authorization", `Bearer ${token}`);
  const res = await fetch(`${API_BASE}/receipt-settings/logo`, {
    method: "POST",
    headers,
    body: form,
  });
  if (!res.ok) throw new Error(await parseErrorMessage(res));
  return (await res.json()) as ReceiptSettings;
}

export async function deleteReceiptLogo() {
  return api<ReceiptSettings>("/receipt-settings/logo", { method: "DELETE" });
}

/** Render a sample receipt from unsaved settings, for the live preview. */
export async function previewReceipt(
  draft: Partial<ReceiptSettings>,
  kind: string,
  paper: ReceiptPaper,
) {
  const headers = new Headers({ "Content-Type": "application/json" });
  const token = getAccessToken();
  if (token) headers.set("Authorization", `Bearer ${token}`);
  const query = new URLSearchParams({ kind, paper });
  const res = await fetch(`${API_BASE}/receipt-settings/preview?${query}`, {
    method: "POST",
    headers,
    body: JSON.stringify(draft),
  });
  if (!res.ok) throw new Error(await parseErrorMessage(res));
  return res.text();
}

async function openPrintable(path: string) {
  const headers = new Headers({ Accept: "text/html" });
  const token = getAccessToken();
  if (token) headers.set("Authorization", `Bearer ${token}`);
  const res = await fetch(`${API_BASE}${path}`, { headers });
  if (!res.ok) {
    throw new Error(await parseErrorMessage(res));
  }
  const html = await res.text();
  const win = window.open("", "_blank", "noopener,noreferrer");
  if (!win) throw new Error("Pop-up blocked — allow pop-ups to print the receipt");
  win.document.write(html);
  win.document.close();
}

/** Open a printable customer receipt in a new tab. */
export async function openRepairReceipt(repairId: string, paper?: ReceiptPaper) {
  const query = paper ? `?paper=${paper}` : "";
  return openPrintable(`/repairs/${repairId}/receipt.html${query}`);
}

export async function openIntakeSlip(repairId: string) {
  return openPrintable(`/repairs/${repairId}/intake-slip.html`);
}

/** Open a printable POS sale receipt in a new tab. */
export async function openSaleReceipt(saleId: string, paper?: ReceiptPaper) {
  const query = paper ? `?paper=${paper}` : "";
  return openPrintable(`/sales/${saleId}/receipt.html${query}`);
}

export async function downloadSaleReceiptPDF(saleId: string) {
  return downloadAuthed(`/sales/${saleId}/receipt.pdf`, `receipt-${saleId}.pdf`);
}

async function downloadAuthed(path: string, filename: string) {
  const headers = new Headers();
  const token = getAccessToken();
  if (token) headers.set("Authorization", `Bearer ${token}`);
  const res = await fetch(`${API_BASE}${path}`, { headers });
  if (!res.ok) throw new Error(await parseErrorMessage(res));
  const blob = await res.blob();
  const url = URL.createObjectURL(blob);
  const a = document.createElement("a");
  a.href = url;
  a.download = filename;
  a.click();
  URL.revokeObjectURL(url);
}

export async function downloadRepairReceiptPDF(repairId: string) {
  return downloadAuthed(`/repairs/${repairId}/receipt.pdf`, `receipt-${repairId}.pdf`);
}

export async function downloadRepairTaxInvoicePDF(repairId: string) {
  return downloadAuthed(`/repairs/${repairId}/tax-invoice.pdf`, `tax-invoice-${repairId}.pdf`);
}

export type Warranty = {
  id: string;
  repair_job_id: string;
  starts_at: string;
  ends_at: string;
  duration_days: number;
  status: string;
  claim_note?: string;
  claimed_at?: string;
};

export async function getRepairWarranty(repairId: string) {
  const warr = await api<Warranty | undefined>(`/repairs/${repairId}/warranty`);
  return warr ?? null;
}

export async function createRepairWarranty(repairId: string) {
  return api<Warranty>(`/repairs/${repairId}/warranty`, { method: "POST", body: "{}" });
}

export async function claimRepairWarranty(repairId: string, note: string) {
  return api<Warranty>(`/repairs/${repairId}/warranty/claim`, {
    method: "POST",
    body: JSON.stringify({ note }),
  });
}

export type StaffNotification = {
  id: string;
  title: string;
  body: string;
  template_key?: string;
  acked_at?: string;
  created_at: string;
};

export async function listNotifications(unackedOnly = false) {
  const q = unackedOnly ? "?unacked=1" : "";
  return api<{ items: StaffNotification[] }>(`/notifications${q}`);
}

export async function ackNotification(id: string) {
  return api<void>(`/notifications/${id}/ack`, { method: "POST" });
}

export type ShopProfile = {
  tenant_id: string;
  legal_name: string;
  tin?: string;
  address_line1?: string;
  address_line2?: string;
  city?: string;
  country: string;
  vat_rate_bps: number;
  vat_inclusive: boolean;
  currency_code: string;
  locale: string;
};

export async function getShopProfile() {
  return api<ShopProfile>("/shop/profile");
}

export async function putShopProfile(body: Partial<ShopProfile>) {
  return api<ShopProfile>("/shop/profile", { method: "PUT", body: JSON.stringify(body) });
}

export type LoyaltySettings = {
  tenant_id: string;
  enabled: boolean;
  points_per_completed_repair: number;
  points_per_currency_unit: number;
};

export async function getLoyaltySettings() {
  return api<LoyaltySettings>("/loyalty/settings");
}

export async function updateLoyaltySettings(body: {
  enabled: boolean;
  points_per_completed_repair: number;
  points_per_currency_unit: number;
}) {
  return api<LoyaltySettings>("/loyalty/settings", { method: "PUT", body: JSON.stringify(body) });
}

export type LoyaltyLedgerEntry = {
  id: string;
  delta: number;
  reason: string;
  reference_type?: string;
  reference_id?: string;
  created_at: string;
};

export type LoyaltyAccount = {
  customer_id: string;
  points_balance: number;
  updated_at?: string;
};

export async function getCustomerLoyalty(customerId: string) {
  return api<{ account: LoyaltyAccount; ledger: LoyaltyLedgerEntry[] }>(`/loyalty/customers/${customerId}`);
}

export type WebhookSubscription = {
  id: string;
  url: string;
  event_types: string[];
  is_active: boolean;
  created_at: string;
  last_triggered_at?: string;
  last_status?: string;
  secret?: string;
};

export async function listWebhooks() {
  return api<{ items: WebhookSubscription[] }>("/loyalty/webhooks");
}

export async function createWebhook(body: { url: string; event_types: string[] }) {
  return api<WebhookSubscription>("/loyalty/webhooks", { method: "POST", body: JSON.stringify(body) });
}

export async function deleteWebhook(id: string) {
  return api<void>(`/loyalty/webhooks/${id}`, { method: "DELETE" });
}

export type RegisteredDevice = {
  id: string;
  device_name?: string;
  platform?: string;
  fingerprint?: string;
  last_seen_at?: string;
  created_at: string;
  revoked_at?: string;
};

export async function listRegisteredDevices() {
  return api<{ items?: RegisteredDevice[] } | RegisteredDevice[]>("/devices").then((r) =>
    Array.isArray(r) ? r : r.items ?? [],
  );
}

export async function registerDevice(body: {
  id?: string;
  device_name?: string;
  platform?: string;
  fingerprint?: string;
}) {
  return api<RegisteredDevice>("/devices/register", { method: "POST", body: JSON.stringify(body) });
}

export async function revokeRegisteredDevice(id: string) {
  return api<void>(`/devices/${id}/revoke`, { method: "POST" });
}

export async function fetchRepairAttachmentBlob(
  repairId: string,
  attachmentId: string,
  retried = false,
): Promise<Blob> {
  const headers = new Headers();
  const token = getAccessToken();
  if (token) headers.set("Authorization", `Bearer ${token}`);
  const res = await fetch(
    `${API_BASE}/repairs/${repairId}/attachments/${attachmentId}/content`,
    { headers },
  );
  if (res.status === 401) {
    if (!retried) {
      const refreshed = await refreshAccessToken();
      if (refreshed) {
        return fetchRepairAttachmentBlob(repairId, attachmentId, true);
      }
    }
    notifySessionExpired();
    throw new SessionExpiredError(await parseErrorMessage(res));
  }
  if (!res.ok) {
    throw new Error(await parseErrorMessage(res));
  }
  return res.blob();
}

export async function listSales(params?: { branch_id?: string; status?: string; limit?: number }) {
  const q = new URLSearchParams();
  if (params?.branch_id) q.set("branch_id", params.branch_id);
  if (params?.status) q.set("status", params.status);
  if (params?.limit) q.set("limit", String(params.limit));
  const qs = q.toString();
  return api<{ items: Sale[] }>(`/sales${qs ? `?${qs}` : ""}`);
}

export async function getSale(id: string) {
  return api<Sale>(`/sales/${id}`);
}

export async function reverseSale(id: string, locationId: string) {
  return api<Sale>(`/sales/${id}/reverse`, {
    method: "POST",
    body: JSON.stringify({ location_id: locationId }),
  });
}

export async function listPOSCatalog(locationId?: string) {
  const q = locationId ? `?location_id=${encodeURIComponent(locationId)}` : "";
  return api<{ items: CatalogItem[] }>(`/catalog${q}`);
}

export async function listStockLocations(branchId?: string) {
  const q = branchId ? `?branch_id=${encodeURIComponent(branchId)}` : "";
  return api<{ items: StockLocation[] }>(`/stock-locations${q}`);
}

export async function ensureStockLocations() {
  return api<{ items: StockLocation[] }>("/stock-locations/ensure", { method: "POST", body: "{}" });
}

export type POSCheckoutItem =
  | { variant_id: string; quantity: number }
  | {
      // Quick sale: not in the catalog. unit_cost/supplier_id are internal-only —
      // sourced together or not at all — and never appear on the customer receipt.
      description: string;
      quantity: number;
      unit_price: number;
      unit_cost?: number;
      supplier_id?: string;
    };

export async function posCheckout(body: {
  branch_id: string;
  location_id: string;
  items: POSCheckoutItem[];
  method: string;
  phone?: string;
}) {
  return api<POSCheckoutResult>("/pos/checkout", { method: "POST", body: JSON.stringify(body) });
}

export async function completeSale(id: string, locationId: string) {
  return api<Sale>(`/sales/${id}/complete`, {
    method: "POST",
    body: JSON.stringify({ location_id: locationId }),
  });
}

export type OnlineOrder = {
  id: string;
  status: string;
  collection_code?: string;
  total: number;
  delivery_fee?: number;
  branch_id?: string;
  fulfilment_type?: string;
  created_at?: string;
  guest_name?: string;
  guest_phone?: string;
  guest_email?: string;
  customer_notes?: string;
  delivery_location_id?: string;
  delivery_location_name?: string;
  delivery_address_line1?: string;
  delivery_address_line2?: string;
  delivery_landmark?: string;
};

export type OnlineCheckoutResult = {
  order: OnlineOrder;
  payment?: Payment;
};

export async function listOnlineCatalog(locationId?: string) {
  const q = locationId ? `?location_id=${encodeURIComponent(locationId)}` : "";
  return api<{ items: CatalogItem[] }>(`/commerce/catalog${q}`);
}

export async function listOnlineOrders(status?: string) {
  const q = status ? `?status=${encodeURIComponent(status)}` : "";
  return api<{ items: OnlineOrder[] }>(`/commerce/orders${q}`);
}

export async function placeOnlineCheckout(body: {
  branch_id: string;
  location_id: string;
  items: { variant_id: string; quantity: number }[];
  method?: string;
  phone?: string;
  fulfilment_type?: string;
  customer_name?: string;
  customer_email?: string;
  customer_notes?: string;
  delivery_location_id?: string;
}) {
  return api<OnlineCheckoutResult>("/commerce/checkout", { method: "POST", body: JSON.stringify(body) });
}

export async function confirmOnlineOrderPaid(id: string) {
  return api<OnlineOrder>(`/commerce/orders/${id}/confirm-paid`, { method: "POST", body: "{}" });
}

export async function collectOnlineOrder(collectionCode: string) {
  return api<OnlineOrder>("/commerce/collect", {
    method: "POST",
    body: JSON.stringify({ collection_code: collectionCode }),
  });
}

export async function expireOnlineHolds() {
  return api<{ released: number }>("/commerce/expire-holds", { method: "POST", body: "{}" });
}

export type StockBalance = {
  variant_id: string;
  product_id: string;
  product_name: string;
  sku: string;
  sell_price: number;
  cost_price: number;
  location_id: string;
  location_name: string;
  physical_qty: number;
  available_qty: number;
  reserved_qty: number;
};

export type JobSaleLine = {
  id: string;
  repair_job_id: string;
  variant_id?: string | null;
  location_id?: string | null;
  description: string;
  quantity: number;
  unit_price: number;
  line_total: number;
  is_custom?: boolean;
  created_at: string;
};

export async function addRepairSaleLine(
  repairId: string,
  body:
    | { variant_id: string; location_id: string; quantity: number }
    | { description: string; unit_price: number; quantity?: number },
) {
  return api<JobSaleLine>(`/repairs/${repairId}/sale-lines`, {
    method: "POST",
    body: JSON.stringify(body),
  });
}

export async function removeRepairSaleLine(repairId: string, lineId: string) {
  return api<void>(`/repairs/${repairId}/sale-lines/${lineId}`, { method: "DELETE" });
}

export type StockMovement = {
  id: string;
  variant_id: string;
  sku: string;
  product_name: string;
  location_id: string;
  qty_delta: number;
  reason: string;
  created_at: string;
};

export async function listStockBalances(locationId?: string) {
  const q = locationId ? `?location_id=${encodeURIComponent(locationId)}` : "";
  return api<{ items: StockBalance[] }>(`/inventory/balances${q}`);
}

export async function listStockMovements(locationId?: string) {
  const q = locationId ? `?location_id=${encodeURIComponent(locationId)}` : "";
  return api<{ items: StockMovement[] }>(`/inventory/movements${q}`);
}

export async function receiveStock(body: {
  variant_id: string;
  location_id: string;
  quantity: number;
  note?: string;
}) {
  return api<void>("/inventory/receive", { method: "POST", body: JSON.stringify(body) });
}

export async function adjustStock(body: {
  variant_id: string;
  location_id: string;
  qty_delta: number;
  reason?: string;
}) {
  return api<void>("/inventory/adjust", { method: "POST", body: JSON.stringify(body) });
}

export async function transferStock(body: {
  variant_id: string;
  from_location_id: string;
  to_location_id: string;
  quantity: number;
}) {
  return api<void>("/inventory/transfer", { method: "POST", body: JSON.stringify(body) });
}

export type SyncCommand = {
  action_id: string;
  tenant_id: string;
  branch_id?: string;
  device_id?: string;
  user_id: string;
  command_type: string;
  local_timestamp?: string;
  payload: Record<string, unknown>;
  sync_status: string;
  retry_count: number;
  last_error?: string;
  result?: Record<string, unknown>;
  created_at: string;
  updated_at: string;
};

export async function listSyncCommands(status?: string, commandType?: string) {
  const params = new URLSearchParams();
  if (status) params.set("status", status);
  if (commandType) params.set("command_type", commandType);
  const q = params.toString();
  return api<{ items: SyncCommand[] }>(`/sync/commands${q ? `?${q}` : ""}`);
}

export async function resolveSyncCommand(id: string, resolution: "discard" | "retry") {
  return api<{ action_id: string; sync_status: string; error?: string }>(`/sync/commands/${id}/resolve`, {
    method: "POST",
    body: JSON.stringify({ resolution }),
  });
}
