import { createContext, useContext, useEffect, useMemo, useState, type ReactNode } from "react";
import {
  getBootstrap,
  getFXRates,
  getStorefrontContent,
  listAccountOrders,
  listAccountRepairs,
  listCatalog,
  loginAccount,
  onStorefrontSessionExpired,
  registerAccount,
  type AccountRepair,
  type Bootstrap,
  type CatalogItem,
  type CustomerSession,
  type FXRates,
  type Order,
  type PublicBranch,
  type StorefrontContent,
} from "../lib/api";
import {
  clearSession as clearStoredSession,
  loadBranch,
  loadCart,
  loadSession,
  saveBranch,
  saveCart,
  saveSession,
  type CartMap,
  type StoredBranch,
  type StoredSession,
} from "../lib/storage";

type CartLine = { item: CatalogItem; qty: number };

type StorefrontState = {
  boot: Bootstrap | null;
  content: StorefrontContent | null;
  catalog: CatalogItem[];
  loadingShop: boolean;
  error: string;
  pickup: StoredBranch | null;
  branches: PublicBranch[];
  changeBranch: (branch: PublicBranch) => Promise<void>;

  cart: CartMap;
  lines: CartLine[];
  total: number;
  cartCount: number;
  setQty: (variantId: string, next: number, maxQty: number) => void;
  addOne: (item: CatalogItem) => void;
  clearCart: () => void;

  session: StoredSession | null;
  accountOrders: Order[];
  accountRepairs: AccountRepair[];
  login: (email: string, password: string) => Promise<void>;
  register: (input: { full_name: string; email: string; phone?: string; password: string }) => Promise<void>;
  logout: () => void;
  refreshAccountOrders: () => void;

  // Display-only currency switcher — checkout always charges the real KES
  // amount server-side regardless of what's shown here.
  currency: string;
  setCurrency: (code: string) => void;
  availableCurrencies: string[];
  formatPrice: (kesAmount: number) => string;
};

const CURRENCY_KEY = "techlane.storefront.currency";

const StorefrontCtx = createContext<StorefrontState | null>(null);

function pickBranch(b: Bootstrap, stored: StoredBranch | null): StoredBranch {
  const branches = b.branches?.length
    ? b.branches
    : [{ id: b.branch_id, name: b.branch_name, location_id: b.location_id, location_name: b.location_name }];
  if (stored) {
    const match = branches.find((x) => x.id === stored.branch_id && x.location_id === stored.location_id);
    if (match) {
      return {
        branch_id: match.id,
        branch_name: match.name,
        location_id: match.location_id,
        location_name: match.location_name,
      };
    }
  }
  const first = branches[0]!;
  return {
    branch_id: first.id,
    branch_name: first.name,
    location_id: first.location_id,
    location_name: first.location_name,
  };
}

export function StorefrontProvider({ children }: { children: ReactNode }) {
  const [boot, setBoot] = useState<Bootstrap | null>(null);
  const [content, setContent] = useState<StorefrontContent | null>(null);
  const [catalog, setCatalog] = useState<CatalogItem[]>([]);
  const [cart, setCart] = useState<CartMap>(() => loadCart());
  const [session, setSession] = useState<StoredSession | null>(() => loadSession());
  const [pickup, setPickup] = useState<StoredBranch | null>(() => loadBranch());
  const [accountOrders, setAccountOrders] = useState<Order[]>([]);
  const [accountRepairs, setAccountRepairs] = useState<AccountRepair[]>([]);
  const [error, setError] = useState("");
  const [loadingShop, setLoadingShop] = useState(true);
  const [fxRates, setFxRates] = useState<FXRates | null>(null);
  const [currency, setCurrencyState] = useState<string>(() => {
    try {
      return localStorage.getItem(CURRENCY_KEY) || "KES";
    } catch {
      return "KES";
    }
  });

  useEffect(() => {
    saveCart(cart);
  }, [cart]);

  useEffect(() => {
    getFXRates()
      .then(setFxRates)
      .catch(() => setFxRates(null));
  }, []);

  function setCurrency(code: string) {
    setCurrencyState(code);
    try {
      localStorage.setItem(CURRENCY_KEY, code);
    } catch {
      // Best-effort — a currency display preference isn't worth failing over.
    }
  }

  const availableCurrencies = fxRates?.enabled?.length ? fxRates.enabled : ["KES"];

  function formatPrice(kesAmount: number): string {
    if (currency === "KES" || !fxRates?.rates?.[currency]) {
      return `KES ${kesAmount.toLocaleString()}`;
    }
    const converted = kesAmount * fxRates.rates[currency];
    try {
      return new Intl.NumberFormat(undefined, { style: "currency", currency }).format(converted);
    } catch {
      return `${currency} ${converted.toLocaleString(undefined, { maximumFractionDigits: 2 })}`;
    }
  }

  useEffect(() => {
    let cancelled = false;
    setLoadingShop(true);
    getBootstrap()
      .then(async (b) => {
        if (cancelled) return;
        setBoot(b);
        const stored = loadBranch();
        const branch = pickBranch(b, stored);
        setPickup(branch);
        saveBranch(branch);
        const [cat, cms] = await Promise.all([
          listCatalog(branch.location_id),
          getStorefrontContent(branch.location_id).catch(() => null),
        ]);
        if (cancelled) return;
        setCatalog(cat.items ?? []);
        if (cms) setContent(cms);
      })
      .catch((e) => {
        if (!cancelled) setError(e instanceof Error ? e.message : "Failed to load shop");
      })
      .finally(() => {
        if (!cancelled) setLoadingShop(false);
      });
    return () => {
      cancelled = true;
    };
  }, []);

  useEffect(() => {
    if (!session?.token) {
      setAccountOrders([]);
      setAccountRepairs([]);
      return;
    }
    listAccountOrders(session.token)
      .then((res) => setAccountOrders(res.items ?? []))
      .catch(() => setAccountOrders([]));
    listAccountRepairs(session.token)
      .then((res) => setAccountRepairs(res.items ?? []))
      .catch(() => setAccountRepairs([]));
  }, [session?.token]);

  useEffect(
    () =>
      onStorefrontSessionExpired(() => {
        setSession(null);
        setAccountOrders([]);
        setAccountRepairs([]);
      }),
    [],
  );

  async function changeBranch(branch: PublicBranch) {
    const next: StoredBranch = {
      branch_id: branch.id,
      branch_name: branch.name,
      location_id: branch.location_id,
      location_name: branch.location_name,
    };
    setPickup(next);
    saveBranch(next);
    setCart({});
    setLoadingShop(true);
    setError("");
    try {
      const [cat, cms] = await Promise.all([
        // TODO: move to server-side pagination if catalog size becomes a performance problem
        listCatalog(branch.location_id),
        getStorefrontContent(branch.location_id).catch(() => null),
      ]);
      setCatalog(cat.items ?? []);
      if (cms) setContent(cms);
    } catch (e) {
      setError(e instanceof Error ? e.message : "Failed to load catalog");
    } finally {
      setLoadingShop(false);
    }
  }

  function setQty(variantId: string, next: number, maxQty: number) {
    setCart((prev) => {
      const qty = Math.max(0, Math.min(maxQty, Math.floor(next)));
      if (qty <= 0) {
        const { [variantId]: _, ...rest } = prev;
        return rest;
      }
      return { ...prev, [variantId]: qty };
    });
  }

  function addOne(item: CatalogItem) {
    const current = cart[item.variant_id] ?? 0;
    if (current >= item.available_qty) return;
    setQty(item.variant_id, current + 1, item.available_qty);
  }

  const lines = useMemo(() => {
    return Object.entries(cart)
      .filter(([, q]) => q > 0)
      .map(([variantId, qty]) => {
        const item = catalog.find((c) => c.variant_id === variantId);
        return item ? { item, qty } : null;
      })
      .filter((l): l is CartLine => Boolean(l));
  }, [cart, catalog]);

  const total = lines.reduce((s, l) => s + l.item.sell_price * l.qty, 0);
  const cartCount = lines.reduce((s, l) => s + l.qty, 0);

  function applySession(s: CustomerSession) {
    const stored: StoredSession = { token: s.token, expires_at: s.expires_at, customer: s.customer };
    saveSession(stored);
    setSession(stored);
  }

  async function login(email: string, password: string) {
    const s = await loginAccount({ email, password });
    applySession(s);
  }

  async function register(input: { full_name: string; email: string; phone?: string; password: string }) {
    const s = await registerAccount(input);
    applySession(s);
  }

  function logout() {
    clearStoredSession();
    setSession(null);
    setAccountOrders([]);
    setAccountRepairs([]);
  }

  function refreshAccountOrders() {
    if (!session?.token) return;
    listAccountOrders(session.token)
      .then((res) => setAccountOrders(res.items ?? []))
      .catch(() => undefined);
    listAccountRepairs(session.token)
      .then((res) => setAccountRepairs(res.items ?? []))
      .catch(() => undefined);
  }

  const value: StorefrontState = {
    boot,
    content,
    catalog,
    loadingShop,
    error,
    pickup,
    branches: boot?.branches?.length
      ? boot.branches
      : pickup
        ? [
            {
              id: pickup.branch_id,
              name: pickup.branch_name,
              location_id: pickup.location_id,
              location_name: pickup.location_name,
            },
          ]
        : [],
    changeBranch,
    cart,
    lines,
    total,
    cartCount,
    setQty,
    addOne,
    clearCart: () => setCart({}),
    session,
    accountOrders,
    accountRepairs,
    login,
    register,
    logout,
    refreshAccountOrders,
    currency,
    setCurrency,
    availableCurrencies,
    formatPrice,
  };

  return <StorefrontCtx.Provider value={value}>{children}</StorefrontCtx.Provider>;
}

export function useStorefront() {
  const ctx = useContext(StorefrontCtx);
  if (!ctx) throw new Error("useStorefront must be used within StorefrontProvider");
  return ctx;
}
