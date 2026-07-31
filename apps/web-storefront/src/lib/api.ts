import { clearSession } from "./storage";

const API_BASE = import.meta.env.VITE_API_BASE ?? "http://localhost:8080/api/v1";

export type PublicBranch = {
  id: string;
  name: string;
  location_id: string;
  location_name: string;
  address?: string;
  phone?: string;
  hours?: string;
  map_url?: string;
};

export type DeliveryLocation = {
  id: string;
  name: string;
  description?: string;
  fee: number;
  active: boolean;
  sort_order: number;
};

export type Bootstrap = {
  tenant_id: string;
  tenant_name: string;
  branch_id: string;
  branch_name: string;
  location_id: string;
  location_name: string;
  branches: PublicBranch[];
  delivery_locations?: DeliveryLocation[];
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
  has_image?: boolean;
  image_updated_at?: string;
  original_price?: number;
  deal_ends_at?: string;
  featured?: boolean;
  new_arrival?: boolean;
  bestseller?: boolean;
  rating_avg?: number;
  rating_count?: number;
};

export type StorefrontSettings = {
  shop_display_name?: string;
  page_title?: string;
  has_logo?: boolean;
  logo_content_type?: string;
  logo_updated_at?: string;
  color_primary?: string;
  color_secondary?: string;
  color_accent?: string;
  topbar_help_href?: string;
  topbar_support_href?: string;
  topbar_contact_href?: string;
  topbar_phone_label?: string;
  header_promo_text?: string;
  show_featured?: boolean;
  show_new_arrivals?: boolean;
  show_bestsellers?: boolean;
  show_deals?: boolean;
  show_most_viewed?: boolean;
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
  pay_label_stk?: string;
  pay_label_paybill?: string;
  pay_label_cash?: string;
  pay_hint_stk?: string;
  pay_hint_paybill?: string;
  pay_hint_cash?: string;
  pay_cta_stk?: string;
  pay_cta_paybill?: string;
  pay_cta_cash?: string;
};

export type StorefrontBanner = {
  id: string;
  headline: string;
  subtext: string;
  cta_label: string;
  cta_href: string;
  has_image: boolean;
  image_updated_at?: string;
  placement: "hero" | "side" | "mid" | "promo_tile";
  deal_id?: string;
  deal_variant_id?: string;
  deal_price?: number;
  deal_base_price?: number;
  sort_order: number;
  active: boolean;
};

export type StorefrontCategory = {
  id: string;
  name: string;
  parent_id?: string;
  path: string;
  depth: number;
};

export type StorefrontContent = {
  settings: StorefrontSettings;
  banners: StorefrontBanner[];
  categories: StorefrontCategory[];
  featured: CatalogItem[];
  new_arrivals: CatalogItem[];
  bestsellers: CatalogItem[];
  deals: CatalogItem[];
  most_viewed: CatalogItem[];
};

export type ProductReview = {
  id: string;
  product_id: string;
  rating: number;
  title?: string;
  body?: string;
  status: string;
  created_at: string;
};

export type FXRates = {
  base: string;
  rates: Record<string, number>;
  enabled: string[];
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
  delivery_fee?: number;
  branch_id?: string;
  fulfilment_type?: string;
  created_at?: string;
  payment?: Payment;
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

export function getStorefrontContent(locationId?: string) {
  const q = locationId ? `?location_id=${encodeURIComponent(locationId)}` : "";
  return api<StorefrontContent>(`/commerce/public/storefront-content${q}`);
}

export function subscribeNewsletter(email: string) {
  return api<{ status: string }>("/commerce/public/newsletter-subscribe", {
    method: "POST",
    body: JSON.stringify({ email }),
  });
}

export function storefrontBannerImageURL(id: string, cacheBust?: string) {
  const base = `${API_BASE}/storefront/public/banners/${id}/image`;
  return cacheBust ? `${base}?v=${encodeURIComponent(cacheBust)}` : base;
}

export function productImageURL(productId: string, cacheBust?: string) {
  const base = `${API_BASE}/inventory/public/products/${encodeURIComponent(productId)}/image`;
  return cacheBust ? `${base}?v=${encodeURIComponent(cacheBust)}` : base;
}

/** Prefer uploaded product photo, then any absolute/text image_url. */
export function catalogItemImageURL(item: {
  product_id: string;
  image_url?: string;
  has_image?: boolean;
  image_updated_at?: string;
}): string | undefined {
  if (item.has_image) return productImageURL(item.product_id, item.image_updated_at);
  return item.image_url || undefined;
}

export function storefrontLogoURL(tenantId: string, cacheBust?: string) {
  const base = `${API_BASE}/storefront/public/logo/${tenantId}`;
  return cacheBust ? `${base}?v=${encodeURIComponent(cacheBust)}` : base;
}

/** Fire-and-forget product-page view tracking for the "Most Viewed" rail. */
export function recordProductView(variantId: string) {
  return api<{ status: string }>(`/commerce/public/products/${encodeURIComponent(variantId)}/view`, {
    method: "POST",
  });
}

export function listProductReviews(productId: string) {
  return api<{ items: ProductReview[] }>(`/commerce/public/products/${encodeURIComponent(productId)}/reviews`);
}

export function submitProductReview(
  productId: string,
  body: { rating: number; title?: string; body?: string },
  token: string,
) {
  return api<ProductReview>(`/commerce/public/products/${encodeURIComponent(productId)}/reviews`, {
    method: "POST",
    body: JSON.stringify(body),
    token,
  });
}

export function getFXRates() {
  return api<FXRates>("/commerce/public/fx-rates");
}

export function checkout(
  body: {
    branch_id: string;
    location_id: string;
    items: { variant_id: string; quantity: number }[];
    method: "mpesa_c2b" | "mpesa_stk" | "cash_on_pickup";
    phone?: string;
    fulfilment_type?: "branch_pickup" | "delivery";
    customer_name?: string;
    customer_email?: string;
    customer_notes?: string;
    delivery_location_id?: string;
    delivery_address_line1?: string;
    delivery_address_line2?: string;
    delivery_landmark?: string;
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

export type TrackOrderHit = {
  id: string;
  ref: string;
  status: string;
  total: number;
  fulfilment_type?: string;
  created_at: string;
};

export type TrackRepairHit = {
  id: string;
  job_code: string;
  job_number: number;
  status: string;
  problem_summary?: string;
  device?: { kind?: string; brand?: string; model?: string };
  created_at: string;
};

export type TrackResult = {
  query: string;
  orders: TrackOrderHit[];
  repairs: TrackRepairHit[];
};

export type AccountRepair = {
  id: string;
  job_code: string;
  job_number: number;
  status: string;
  problem_summary?: string;
  created_at: string;
};

export type PublicRepairStatus = {
  id?: string;
  job_code: string;
  job_number?: number;
  status: string;
  problem_summary?: string;
  device?: { kind?: string; brand?: string; model?: string };
  timeline?: Array<{ status: string; at?: string; note?: string }>;
  created_at?: string;
};

export function trackLookup(q: string) {
  return api<TrackResult>(`/commerce/public/track?q=${encodeURIComponent(q.trim())}`);
}

export function getPublicRepairStatus(jobCode: string, phone?: string) {
  const params = new URLSearchParams({ job_code: jobCode.trim() });
  if (phone?.trim()) params.set("phone", phone.trim());
  return api<PublicRepairStatus>(`/public/repairs/status?${params.toString()}`);
}

export function listAccountRepairs(token: string) {
  return api<{ items: AccountRepair[] }>("/commerce/public/account/repairs", { token });
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

/** Terminal success statuses that should stop payment polling. */
export function isOrderPaid(order: Order): boolean {
  return order.status === "ready_for_pickup" || order.status === "confirmed" || order.status === "delivered";
}

export function isOrderFailed(order: Order): boolean {
  return order.status === "expired" || order.status === "cancelled" || order.status === "failed";
}

export function orderStatusMessage(order: Order, opts?: { cashOnPickup?: boolean }): string {
  switch (order.status) {
    case "pending_payment":
      return opts?.cashOnPickup
        ? "Order reserved. Pay cash when you collect at the branch (hold lasts a few days)."
        : "Waiting for payment. Stock is held for about 15 minutes.";
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
