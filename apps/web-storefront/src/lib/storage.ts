const CART_KEY = "techlane.storefront.cart";
const RECENT_ORDERS_KEY = "techlane.storefront.recentOrders";
const BRANCH_KEY = "techlane.storefront.branch";
const SESSION_KEY = "techlane.storefront.session";

export type CartMap = Record<string, number>;

export type StoredSession = {
  token: string;
  expires_at: string;
  customer: {
    id: string;
    full_name: string;
    email: string;
    phone?: string;
  };
};

export type StoredBranch = {
  branch_id: string;
  branch_name: string;
  location_id: string;
  location_name: string;
};

export function loadCart(): CartMap {
  try {
    const raw = localStorage.getItem(CART_KEY);
    if (!raw) return {};
    const parsed = JSON.parse(raw) as unknown;
    if (!parsed || typeof parsed !== "object") return {};
    const out: CartMap = {};
    for (const [k, v] of Object.entries(parsed as Record<string, unknown>)) {
      const n = typeof v === "number" ? v : Number(v);
      if (Number.isFinite(n) && n > 0) out[k] = Math.floor(n);
    }
    return out;
  } catch {
    return {};
  }
}

export function saveCart(cart: CartMap) {
  const cleaned: CartMap = {};
  for (const [k, v] of Object.entries(cart)) {
    if (v > 0) cleaned[k] = v;
  }
  localStorage.setItem(CART_KEY, JSON.stringify(cleaned));
}

export function loadRecentOrderIds(): string[] {
  try {
    const raw = localStorage.getItem(RECENT_ORDERS_KEY);
    if (!raw) return [];
    const parsed = JSON.parse(raw) as unknown;
    if (!Array.isArray(parsed)) return [];
    return parsed.filter((x): x is string => typeof x === "string" && x.length > 0);
  } catch {
    return [];
  }
}

export function rememberOrderId(id: string) {
  const next = [id, ...loadRecentOrderIds().filter((x) => x !== id)].slice(0, 10);
  localStorage.setItem(RECENT_ORDERS_KEY, JSON.stringify(next));
}

export function loadBranch(): StoredBranch | null {
  try {
    const raw = localStorage.getItem(BRANCH_KEY);
    if (!raw) return null;
    return JSON.parse(raw) as StoredBranch;
  } catch {
    return null;
  }
}

export function saveBranch(branch: StoredBranch) {
  localStorage.setItem(BRANCH_KEY, JSON.stringify(branch));
}

export function loadSession(): StoredSession | null {
  try {
    const raw = localStorage.getItem(SESSION_KEY);
    if (!raw) return null;
    const session = JSON.parse(raw) as StoredSession;
    if (!session.token || !session.expires_at) return null;
    if (new Date(session.expires_at).getTime() <= Date.now()) {
      localStorage.removeItem(SESSION_KEY);
      return null;
    }
    return session;
  } catch {
    return null;
  }
}

export function saveSession(session: StoredSession) {
  localStorage.setItem(SESSION_KEY, JSON.stringify(session));
}

export function clearSession() {
  localStorage.removeItem(SESSION_KEY);
}
