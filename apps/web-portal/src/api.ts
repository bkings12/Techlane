const API_BASE = import.meta.env.VITE_API_BASE ?? "http://localhost:8080/api/v1";

const TOKEN_KEY = "portal_session_token";
const PHONE_KEY = "portal_phone";

export type Customer = {
  id: string;
  full_name?: string;
  name?: string;
  phone?: string;
};

export type Estimate = {
  id: string;
  labor_amount: number;
  parts_amount: number;
  currency?: string;
  notes?: string;
  status: string;
  expires_at?: string;
};

export type TimelineEvent = {
  status?: string;
  event_type?: string;
  note?: string;
  created_at?: string;
  at?: string;
};

export type Receipt = {
  id: string;
  method: string;
  amount: number;
  currency?: string;
  status: string;
  created_at: string;
};

export type Repair = {
  id: string;
  job_code: string;
  job_number?: number;
  status: string;
  problem_summary?: string;
  device_brand?: string;
  device_model?: string;
  customer_name?: string;
  labor_amount?: number;
  balance_due?: number;
  amount_due?: number;
  created_at: string;
  device?: { brand?: string; model?: string; kind?: string };
  timeline?: TimelineEvent[];
  estimates?: Estimate[];
  estimate?: Estimate | null;
  pending_estimate?: Estimate | null;
  receipts?: Receipt[];
};

export type GuestRepair = {
  job_code: string;
  status: string;
  problem_summary: string;
  device?: { brand?: string; model?: string; kind?: string };
  timeline?: TimelineEvent[];
  created_at: string;
  receipts?: Receipt[];
};

async function parseError(res: Response): Promise<never> {
  const data = await res.json().catch(() => ({}));
  throw new Error(data?.error?.message ?? data?.message ?? `HTTP ${res.status}`);
}

async function request<T>(path: string, init: RequestInit = {}, authed = false): Promise<T> {
  const headers = new Headers(init.headers);
  headers.set("Accept", "application/json");
  if (init.body && !headers.has("Content-Type")) {
    headers.set("Content-Type", "application/json");
  }
  if (authed) {
    const token = getToken();
    if (token) headers.set("Authorization", `Bearer ${token}`);
  }
  const res = await fetch(`${API_BASE}${path}`, { ...init, headers });
  if (!res.ok) await parseError(res);
  if (res.status === 204) return undefined as T;
  return (await res.json()) as T;
}

export function getToken(): string | null {
  return sessionStorage.getItem(TOKEN_KEY);
}

export function setSession(token: string, phone?: string) {
  sessionStorage.setItem(TOKEN_KEY, token);
  if (phone) sessionStorage.setItem(PHONE_KEY, phone);
}

export function clearSession() {
  sessionStorage.removeItem(TOKEN_KEY);
}

export function savedPhone(): string {
  return sessionStorage.getItem(PHONE_KEY) ?? "";
}

export async function requestOtp(phone: string) {
  return request<{ status: string }>("/customer/auth/otp/request", {
    method: "POST",
    body: JSON.stringify({ phone }),
  });
}

export async function verifyOtp(phone: string, code: string) {
  return request<{ token: string; expires_at: string; customer: Customer }>("/customer/auth/otp/verify", {
    method: "POST",
    body: JSON.stringify({ phone, code }),
  });
}

export async function logout() {
  try {
    await request("/customer/auth/logout", { method: "POST" }, true);
  } catch {
    /* ignore */
  }
  clearSession();
}

export async function me() {
  return request<Customer>("/customer/me", {}, true);
}

export async function listRepairs() {
  const data = await request<{ items: Repair[] }>("/customer/repairs", {}, true);
  return data.items ?? [];
}

export async function getRepair(id: string) {
  return request<Repair>(`/customer/repairs/${id}`, {}, true);
}

export async function approveEstimate(repairId: string, estimateId: string) {
  return request<Estimate>(`/customer/repairs/${repairId}/estimates/${estimateId}/approve`, { method: "POST" }, true);
}

export async function rejectEstimate(repairId: string, estimateId: string) {
  return request<Estimate>(`/customer/repairs/${repairId}/estimates/${estimateId}/reject`, { method: "POST" }, true);
}

export async function payRepair(repairId: string, phone?: string) {
  return request<{ id?: string; payment_id?: string; status?: string; message?: string }>(
    `/customer/repairs/${repairId}/pay`,
    { method: "POST", body: JSON.stringify({ method: "mpesa_stk", phone: phone || undefined }) },
    true,
  );
}

export async function paymentStatus(repairId: string, paymentId: string) {
  return request<{ id: string; status: string; amount: number }>(
    `/customer/repairs/${repairId}/payments/${paymentId}`,
    {},
    true,
  );
}

export async function guestLookup(jobCode: string, phone: string) {
  const params = new URLSearchParams({ job_code: jobCode.trim(), phone: phone.trim() });
  return request<GuestRepair>(`/public/repairs/status?${params}`);
}

/** Fetch printable receipt HTML and open in a new tab. */
export async function openRepairReceipt(repairId: string) {
  const headers = new Headers({ Accept: "text/html" });
  const token = getToken();
  if (token) headers.set("Authorization", `Bearer ${token}`);
  const res = await fetch(`${API_BASE}/customer/repairs/${repairId}/receipt.html`, { headers });
  if (!res.ok) await parseError(res);
  const html = await res.text();
  const win = window.open("", "_blank", "noopener,noreferrer");
  if (!win) throw new Error("Pop-up blocked — allow pop-ups to print the receipt");
  win.document.write(html);
  win.document.close();
}

export async function downloadRepairReceiptPDF(repairId: string) {
  const headers = new Headers();
  const token = getToken();
  if (token) headers.set("Authorization", `Bearer ${token}`);
  const res = await fetch(`${API_BASE}/customer/repairs/${repairId}/receipt.pdf`, { headers });
  if (!res.ok) await parseError(res);
  const blob = await res.blob();
  const url = URL.createObjectURL(blob);
  const a = document.createElement("a");
  a.href = url;
  a.download = `receipt-${repairId}.pdf`;
  a.click();
  URL.revokeObjectURL(url);
}

export type Warranty = {
  id: string;
  repair_job_id: string;
  starts_at: string;
  ends_at: string;
  duration_days: number;
  status: string;
  claim_note?: string;
};

export async function getRepairWarranty(repairId: string) {
  return request<Warranty>(`/customer/repairs/${repairId}/warranty`, {}, true);
}

export async function claimRepairWarranty(repairId: string, note: string) {
  return request<Warranty>(`/customer/repairs/${repairId}/warranty/claim`, {
    method: "POST",
    body: JSON.stringify({ note }),
  }, true);
}

export const statusLabels: Record<string, string> = {
  intake: "Checked in",
  diagnosed: "Diagnosis complete",
  waiting_parts: "Waiting for parts",
  in_progress: "Repair in progress",
  completed: "Ready for collection",
  ready: "Ready for collection",
  collected: "Collected",
};
