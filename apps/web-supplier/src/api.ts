const API_BASE = import.meta.env.VITE_API_BASE ?? "http://localhost:8080/api/v1";
const TOKEN_KEY = "supplier_session_token";
const SESSION_EXPIRED_EVENT = "techlane:supplier-session-expired";

export function onSessionExpired(handler: () => void) {
  if (typeof window === "undefined") return () => undefined;
  const listener = () => handler();
  window.addEventListener(SESSION_EXPIRED_EVENT, listener);
  return () => window.removeEventListener(SESSION_EXPIRED_EVENT, listener);
}

export type SupplierContact = {
  id: string;
  supplier_id: string;
  email: string;
  display_name: string;
  status: string;
  supplier_name?: string;
  phone?: string;
};

export type PartRequest = {
  id: string;
  repair_job_id?: string;
  job_code?: string;
  status: string;
  description: string;
  quantity: number;
  quote_status?: string | null;
  assigned_supplier_id?: string;
  created_at?: string;
  part_name?: string;
  notes?: string;
  quotes?: Array<{
    id: string;
    unit_cost: number;
    notes?: string;
    status: string;
  }>;
  issue?: {
    id: string;
    auth_code: string;
    status: string;
    unit_cost: number;
  };
};

export type Issue = {
  id: string;
  auth_code?: string;
  qr_payload?: string;
  status: string;
  unit_cost: number;
  part_name?: string;
  job_code?: string;
  description?: string;
};

export type CreditSummary = {
  supplier_id: string;
  supplier_name: string;
  outstanding_credit: number;
  outstanding?: number;
  balance?: number;
  entries: Array<{
    id: string;
    amount: number;
    entry_type?: string;
    type?: string;
    created_at: string;
  }>;
};

async function parseError(res: Response): Promise<never> {
  const data = await res.json().catch(() => ({}));
  throw new Error(data?.error?.message ?? data?.message ?? `HTTP ${res.status}`);
}

async function request<T>(path: string, init: RequestInit = {}, authed = false): Promise<T> {
  const headers = new Headers(init.headers);
  headers.set("Accept", "application/json");
  if (init.body && !headers.has("Content-Type")) headers.set("Content-Type", "application/json");
  if (authed) {
    const token = getToken();
    if (token) headers.set("Authorization", `Bearer ${token}`);
  }
  const res = await fetch(`${API_BASE}${path}`, { ...init, headers });
  if (!res.ok) {
    if (res.status === 401 && authed) {
      clearToken();
      if (typeof window !== "undefined") window.dispatchEvent(new Event(SESSION_EXPIRED_EVENT));
    }
    await parseError(res);
  }
  if (res.status === 204) return undefined as T;
  return (await res.json()) as T;
}

export function getToken() {
  return sessionStorage.getItem(TOKEN_KEY);
}

export function setToken(token: string) {
  sessionStorage.setItem(TOKEN_KEY, token);
}

export function clearToken() {
  sessionStorage.removeItem(TOKEN_KEY);
}

export async function login(email: string, password: string) {
  return request<{ token: string; contact: SupplierContact }>("/supplier/auth/login", {
    method: "POST",
    body: JSON.stringify({ email, password }),
  });
}

export async function acceptInvite(token: string, password: string) {
  return request<{ token: string; contact: SupplierContact }>("/supplier/auth/accept-invite", {
    method: "POST",
    body: JSON.stringify({ token, password }),
  });
}

export async function logout() {
  try {
    await request("/supplier/auth/logout", { method: "POST" }, true);
  } catch {
    /* ignore */
  }
  clearToken();
}

export async function me() {
  return request<SupplierContact>("/supplier/me", {}, true);
}

export async function listRequests(status?: string) {
  const q = status ? `?status=${encodeURIComponent(status)}` : "";
  const data = await request<{ items: PartRequest[] }>(`/supplier/requests${q}`, {}, true);
  return data.items ?? [];
}

export async function getRequest(id: string) {
  return request<PartRequest>(`/supplier/requests/${id}`, {}, true);
}

export async function quote(id: string, unitCost: number, notes?: string) {
  return request(`/supplier/requests/${id}/quote`, {
    method: "POST",
    body: JSON.stringify({ unit_cost: unitCost, notes: notes || undefined }),
  }, true);
}

export async function decline(id: string, notes?: string) {
  return request(`/supplier/requests/${id}/decline`, {
    method: "POST",
    body: JSON.stringify({ notes: notes || undefined }),
  }, true);
}

export async function markReady(id: string) {
  return request(`/supplier/requests/${id}/ready`, { method: "POST" }, true);
}

export async function issue(id: string) {
  return request<{
    issue?: Issue;
    auth_code?: string;
    qr_payload?: string;
    id?: string;
  }>(`/supplier/requests/${id}/issue`, { method: "POST" }, true);
}

export async function listIssues() {
  const data = await request<{ items: Issue[] }>("/supplier/issues", {}, true);
  return data.items ?? [];
}

export async function credit() {
  return request<CreditSummary>("/supplier/credit", {}, true);
}

export async function openIssueVoucher(issueId: string) {
  const headers = new Headers({ Accept: "text/html" });
  const token = getToken();
  if (token) headers.set("Authorization", `Bearer ${token}`);
  const res = await fetch(`${API_BASE}/supplier/issues/${issueId}/voucher.html`, { headers });
  if (!res.ok) {
    if (res.status === 401) {
      clearToken();
      if (typeof window !== "undefined") window.dispatchEvent(new Event(SESSION_EXPIRED_EVENT));
    }
    const data = await res.json().catch(() => ({}));
    throw new Error(data?.error?.message ?? data?.message ?? `HTTP ${res.status}`);
  }
  const html = await res.text();
  const win = window.open("", "_blank", "noopener,noreferrer");
  if (!win) throw new Error("Pop-up blocked — allow pop-ups to print the voucher");
  win.document.write(html);
  win.document.close();
}

export async function downloadIssueVoucherPDF(issueId: string) {
  const headers = new Headers();
  const token = getToken();
  if (token) headers.set("Authorization", `Bearer ${token}`);
  const res = await fetch(`${API_BASE}/supplier/issues/${issueId}/voucher.pdf`, { headers });
  if (!res.ok) {
    if (res.status === 401) {
      clearToken();
      if (typeof window !== "undefined") window.dispatchEvent(new Event(SESSION_EXPIRED_EVENT));
    }
    const data = await res.json().catch(() => ({}));
    throw new Error(data?.error?.message ?? data?.message ?? `HTTP ${res.status}`);
  }
  const blob = await res.blob();
  const url = URL.createObjectURL(blob);
  const a = document.createElement("a");
  a.href = url;
  a.download = `voucher-${issueId}.pdf`;
  a.click();
  URL.revokeObjectURL(url);
}
