import { clearSession } from "./storage";

const API_BASE = import.meta.env.VITE_API_BASE ?? "http://localhost:8080/api/v1";

export type PublicBranch = {
  id: string;
  name: string;
  location_id: string;
  location_name: string;
};

export type Bootstrap = {
  tenant_id: string;
  tenant_name: string;
  branch_id: string;
  branch_name: string;
  location_id: string;
  location_name: string;
  branches: PublicBranch[];
  paybill?: string;
};

export type CatalogItem = {
  variant_id: string;
  product_id: string;
  product_name: string;
  brand?: string;
  category?: string;
  description?: string;
  sku: string;
  sell_price: number;
  available_qty: number;
  location_id?: string;
  image_url?: string;
};

export type Payment = {
  id: string;
  method: string;
  amount: number;
  status: string;
  account_reference?: string;
};

export type Order = {
  id: string;
  status: string;
  collection_code?: string;
  total: number;
  branch_id?: string;
  fulfilment_type?: string;
  created_at?: string;
  payment?: Payment;
};

export type CheckoutResult = {
  order: Order;
  payment?: Payment;
};

export type CustomerAccount = {
  id: string;
  full_name: string;
  email: string;
  phone?: string;
};

export type CustomerSession = {
  token: string;
  expires_at: string;
  customer: CustomerAccount;
};

export class ApiError extends Error {
  code?: string;
  status: number;

  constructor(message: string, status: number, code?: string) {
    super(message);
    this.name = "ApiError";
    this.status = status;
    this.code = code;
  }
}

async function api<T>(path: string, options: RequestInit & { token?: string } = {}): Promise<T> {
  const headers = new Headers(options.headers);
  if (options.body != null) {
    headers.set("Content-Type", "application/json");
  }
  if (options.token) {
    headers.set("Authorization", `Bearer ${options.token}`);
  }
  const { token: _, ...fetchOptions } = options;
  const res = await fetch(`${API_BASE}${path}`, { ...fetchOptions, headers });
  if (!res.ok) {
    const body = (await res.json().catch(() => ({}))) as {
      error?: { message?: string; code?: string };
    };
    if (res.status === 401 && options.token) {
      clearSession();
      if (typeof window !== "undefined") {
        window.dispatchEvent(new Event("techlane:storefront-session-expired"));
      }
    }
    throw new ApiError(
      body?.error?.message ?? res.statusText ?? "Request failed",
      res.status,
      body?.error?.code,
    );
  }
  if (res.status === 204) return undefined as T;
  return res.json() as Promise<T>;
}

export function onStorefrontSessionExpired(handler: () => void) {
  if (typeof window === "undefined") return () => undefined;
  const listener = () => handler();
  window.addEventListener("techlane:storefront-session-expired", listener);
  return () => window.removeEventListener("techlane:storefront-session-expired", listener);
}

export function getBootstrap() {
  return api<Bootstrap>("/commerce/public/bootstrap");
}

export function listCatalog(locationId: string) {
  return api<{ items: CatalogItem[] }>(
    `/commerce/public/catalog?location_id=${encodeURIComponent(locationId)}`,
  );
}

export function checkout(
  body: {
    branch_id: string;
    location_id: string;
    items: { variant_id: string; quantity: number }[];
    method: "mpesa_c2b" | "mpesa_stk";
    phone?: string;
  },
  token?: string,
) {
  return api<CheckoutResult>("/commerce/public/checkout", {
    method: "POST",
    body: JSON.stringify(body),
    token,
  });
}

export function getOrder(id: string) {
  return api<Order>(`/commerce/public/orders/${encodeURIComponent(id)}`);
}

export function registerAccount(body: {
  full_name: string;
  email: string;
  phone?: string;
  password: string;
}) {
  return api<CustomerSession>("/commerce/public/accounts/register", {
    method: "POST",
    body: JSON.stringify(body),
  });
}

export function loginAccount(body: { email: string; password: string }) {
  return api<CustomerSession>("/commerce/public/accounts/login", {
    method: "POST",
    body: JSON.stringify(body),
  });
}

export function getAccount(token: string) {
  return api<CustomerAccount>("/commerce/public/account", { token });
}

export function listAccountOrders(token: string) {
  return api<{ items: Order[] }>("/commerce/public/account/orders", { token });
}

/** Terminal / failure statuses that should stop payment polling. */
export function isOrderPaid(order: Order): boolean {
  return (
    order.status === "ready_for_pickup" ||
    order.status === "confirmed" ||
    order.status === "delivered" ||
    Boolean(order.collection_code)
  );
}

export function isOrderFailed(order: Order): boolean {
  return order.status === "expired" || order.status === "cancelled" || order.status === "failed";
}

export function orderStatusMessage(order: Order): string {
  switch (order.status) {
    case "pending_payment":
      return "Waiting for payment. Stock is held for about 15 minutes.";
    case "ready_for_pickup":
    case "confirmed":
      return "Paid — ready for branch pickup.";
    case "delivered":
      return "Order collected.";
    case "expired":
      return "Hold expired. Payment was not completed in time — stock was released. Place a new order.";
    case "cancelled":
      return "Order was cancelled.";
    case "failed":
      return "Order payment failed.";
    default:
      return `Status: ${order.status}`;
  }
}
