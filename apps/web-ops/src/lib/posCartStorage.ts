/** Client-side POS cart persistence + held (parked) sales. */

export type PersistedCartLine = {
  id: string;
  description: string;
  sku?: string;
  unitPrice: number;
  listPrice?: number;
  overrideReason?: string;
  qty: number;
  variantId?: string;
  availableQty?: number;
  unitCost?: number;
  supplierId?: string;
  supplierName?: string;
};

export type HeldSale = {
  id: string;
  note: string;
  heldAt: string;
  items: PersistedCartLine[];
};

const CART_KEY = "techlane.pos.cart";
const HELD_KEY = "techlane.pos.held";

function readJSON<T>(key: string, fallback: T): T {
  try {
    const raw = localStorage.getItem(key);
    if (!raw) return fallback;
    return JSON.parse(raw) as T;
  } catch {
    return fallback;
  }
}

export function loadPersistedCart(): PersistedCartLine[] {
  const items = readJSON<PersistedCartLine[]>(CART_KEY, []);
  return Array.isArray(items) ? items.filter((l) => l && l.id && l.description) : [];
}

export function savePersistedCart(items: PersistedCartLine[]) {
  try {
    localStorage.setItem(CART_KEY, JSON.stringify(items));
  } catch {
    /* ignore quota */
  }
}

export function clearPersistedCart() {
  try {
    localStorage.removeItem(CART_KEY);
  } catch {
    /* ignore */
  }
}

export function loadHeldSales(): HeldSale[] {
  const items = readJSON<HeldSale[]>(HELD_KEY, []);
  return Array.isArray(items) ? items : [];
}

export function saveHeldSales(items: HeldSale[]) {
  try {
    localStorage.setItem(HELD_KEY, JSON.stringify(items));
  } catch {
    /* ignore */
  }
}
