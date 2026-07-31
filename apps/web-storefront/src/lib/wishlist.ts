// Wishlist is a local, browser-only convenience — not a backend feature.
const WISHLIST_KEY = "techlane.storefront.wishlist";
const CHANGE_EVENT = "techlane:wishlist-changed";

export function loadWishlist(): string[] {
  try {
    const raw = localStorage.getItem(WISHLIST_KEY);
    if (!raw) return [];
    const parsed = JSON.parse(raw) as unknown;
    return Array.isArray(parsed) ? parsed.filter((x): x is string => typeof x === "string") : [];
  } catch {
    return [];
  }
}

function save(ids: string[]) {
  localStorage.setItem(WISHLIST_KEY, JSON.stringify(ids));
  if (typeof window !== "undefined") {
    window.dispatchEvent(new Event(CHANGE_EVENT));
  }
}

export function toggleWishlist(variantId: string): string[] {
  const current = loadWishlist();
  const next = current.includes(variantId) ? current.filter((id) => id !== variantId) : [...current, variantId];
  save(next);
  return next;
}

export function onWishlistChanged(handler: () => void) {
  if (typeof window === "undefined") return () => undefined;
  window.addEventListener(CHANGE_EVENT, handler);
  window.addEventListener("storage", handler);
  return () => {
    window.removeEventListener(CHANGE_EVENT, handler);
    window.removeEventListener("storage", handler);
  };
}
