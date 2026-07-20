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

const TOKEN_KEY = "techlane.access";
const REFRESH_KEY = "techlane.refresh";
const SESSION_EXPIRED_EVENT = "techlane:session-expired";

export class SessionExpiredError extends Error {
  constructor(message = "Session expired. Please sign in again.") {
    super(message);
    this.name = "SessionExpiredError";
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

let refreshInFlight: Promise<boolean> | null = null;

async function refreshAccessToken(): Promise<boolean> {
  if (refreshInFlight) return refreshInFlight;
  refreshInFlight = (async () => {
    const refresh = getRefreshToken();
    if (!refresh) return false;
    try {
      const res = await fetch(`${API_BASE}/auth/refresh`, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ refresh_token: refresh }),
      });
      if (!res.ok) return false;
      const tokens = (await res.json()) as TokenPair;
      if (!tokens.access_token || !tokens.refresh_token) return false;
      persistTokens(tokens);
      return true;
    } catch {
      return false;
    }
  })().finally(() => {
    refreshInFlight = null;
  });
  return refreshInFlight;
}

async function parseErrorMessage(res: Response) {
  const body = await res.json().catch(() => ({}));
  return (body as { error?: { message?: string } })?.error?.message ?? res.statusText;
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
      const refreshed = await refreshAccessToken();
      if (refreshed) {
        return api<T>(path, { ...options, _retried: true });
      }
    }
    notifySessionExpired();
    throw new SessionExpiredError(await parseErrorMessage(res));
  }
  if (!res.ok) {
    throw new Error(await parseErrorMessage(res));
  }
  if (res.status === 204) return undefined as T;
  return res.json() as Promise<T>;
}

export async function login(email: string, password: string) {
  const tokens = await api<TokenPair>("/auth/login", {
    method: "POST",
    auth: false,
    body: JSON.stringify({ email, password }),
  });
  persistTokens(tokens);
  return tokens;
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
}) {
  return api<RepairJob>("/repairs", { method: "POST", body: JSON.stringify(body) });
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

export type RepairJob = {
  id: string;
  job_number?: number;
  job_code?: string;
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
  created_at?: string;
  timeline?: RepairStatusEvent[];
  next_statuses?: string[];
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
  labor_amount: number;
  parts_amount: number;
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
  checkout_request_id?: string;
  phone?: string;
  account_reference?: string;
};

export async function getRepair(id: string) {
  return api<RepairJob>(`/repairs/${id}`);
}

export async function assignRepair(id: string, technicianId: string) {
  return api<RepairJob>(`/repairs/${id}/assign`, {
    method: "POST",
    body: JSON.stringify({ technician_id: technicianId }),
  });
}

export async function changeRepairStatus(
  id: string,
  body: { status: string; note?: string; labor_amount?: number },
) {
  return api<RepairJob>(`/repairs/${id}/status`, { method: "POST", body: JSON.stringify(body) });
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
  body: { labor_amount: number; parts_amount: number; notes?: string; expires_hours?: number },
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

export type ReportSummary = {
  generated_at: string;
  period_days: number;
  repairs_open: number;
  repairs_ready: number;
  repairs_completed_period: number;
  repairs_waiting_parts: number;
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

export type OperationsReport = {
  generated_at: string;
  period_days: number;
  daily: DailyMetric[];
  by_technician: TechnicianMetric[];
  by_branch: BranchMetric[];
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

export type Branch = { id: string; name: string; code: string };

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

export function createBranch(body: { name: string; code: string }) {
  return api<Branch>("/branches", { method: "POST", body: JSON.stringify(body) });
}

export function updateBranch(id: string, body: { name?: string; code?: string }) {
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
  quantity: number;
  unit_price: number;
  line_total?: number;
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
  category?: string;
  description?: string;
  image_url?: string;
  pos_visible?: boolean;
  online_visible?: boolean;
};

export type Variant = {
  id: string;
  product_id: string;
  sku: string;
  sell_price: number;
};

export async function listProducts() {
  return api<{ items: Product[] }>("/products");
}

export async function createProduct(body: { name: string; brand?: string }) {
  return api<Product>("/products", { method: "POST", body: JSON.stringify(body) });
}

export async function updateProduct(
  id: string,
  body: {
    name?: string;
    brand?: string;
    category?: string;
    description?: string;
    image_url?: string;
    online_visible?: boolean;
  },
) {
  return api<Product>(`/products/${id}`, { method: "PATCH", body: JSON.stringify(body) });
}

export async function listVariants(productId?: string) {
  const q = productId ? `?product_id=${encodeURIComponent(productId)}` : "";
  return api<{ items: Variant[] }>(`/variants${q}`);
}

export async function createVariant(body: { product_id: string; sku: string; sell_price: number }) {
  return api<Variant>("/variants", { method: "POST", body: JSON.stringify(body) });
}

export async function updateVariant(id: string, body: { sku?: string; sell_price?: number }) {
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

/** Open a printable customer receipt in a new tab. */
export async function openRepairReceipt(repairId: string) {
  const headers = new Headers({ Accept: "text/html" });
  const token = getAccessToken();
  if (token) headers.set("Authorization", `Bearer ${token}`);
  const res = await fetch(`${API_BASE}/repairs/${repairId}/receipt.html`, { headers });
  if (!res.ok) {
    throw new Error(await parseErrorMessage(res));
  }
  const html = await res.text();
  const win = window.open("", "_blank", "noopener,noreferrer");
  if (!win) throw new Error("Pop-up blocked — allow pop-ups to print the receipt");
  win.document.write(html);
  win.document.close();
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
  return api<Warranty>(`/repairs/${repairId}/warranty`);
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
};

export async function getShopProfile() {
  return api<ShopProfile>("/shop/profile");
}

export async function putShopProfile(body: Partial<ShopProfile>) {
  return api<ShopProfile>("/shop/profile", { method: "PUT", body: JSON.stringify(body) });
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

export async function posCheckout(body: {
  branch_id: string;
  location_id: string;
  items: { variant_id: string; quantity: number }[];
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
  branch_id?: string;
  fulfilment_type?: string;
  created_at?: string;
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
  location_id: string;
  location_name: string;
  physical_qty: number;
  available_qty: number;
  reserved_qty: number;
};

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
