import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { useNavigate } from "react-router-dom";
import { globalSearch, type SearchResult } from "../lib/api";
import { Icon, ICONS } from "./ui";

type PaletteItem = {
  key: string;
  title: string;
  subtitle?: string;
  icon: React.ReactNode;
  onSelect: () => void;
};

const QUICK_LINKS: { label: string; path: string; icon: string; keywords: string }[] = [
  { label: "Home", path: "/", icon: "reports", keywords: "home dashboard" },
  { label: "Repairs", path: "/repairs", icon: "repairs", keywords: "repairs jobs workshop records board" },
  { label: "Job POS", path: "/repairs/pos", icon: "repairs", keywords: "intake new job take in device ticket" },
  { label: "Customers", path: "/customers", icon: "customers", keywords: "customers clients" },
  { label: "Part requests", path: "/parts", icon: "parts", keywords: "parts requests" },
  { label: "Inventory", path: "/inventory", icon: "inventory", keywords: "inventory stock" },
  { label: "Suppliers", path: "/suppliers", icon: "suppliers", keywords: "suppliers vendors" },
  { label: "POS", path: "/pos", icon: "pos", keywords: "pos point of sale checkout sell counter" },
  { label: "Same-day fix", path: "/counter/fix", icon: "pos", keywords: "quick repair same day counter fix" },
  { label: "Pickup", path: "/counter/pickup", icon: "pos", keywords: "pickup collect pk code order" },
  { label: "Orders", path: "/orders", icon: "orders", keywords: "orders online" },
  { label: "Payments", path: "/payments", icon: "cash", keywords: "payments cash mpesa" },
  { label: "Risk", path: "/risk", icon: "risk", keywords: "risk alerts fraud" },
  { label: "Reports", path: "/reports", icon: "reports", keywords: "reports analytics charts" },
  { label: "Audit & alerts", path: "/audit", icon: "audit", keywords: "audit errors log" },
  { label: "Sync", path: "/sync", icon: "sync", keywords: "sync offline" },
  { label: "Settings", path: "/settings", icon: "settings", keywords: "settings preferences" },
  { label: "Security settings (MFA)", path: "/settings/security", icon: "settings", keywords: "mfa 2fa security password" },
  { label: "Notifications", path: "/notifications", icon: "inbox", keywords: "notifications alerts" },
];

function resultIcon(type: SearchResult["type"]) {
  if (type === "customer") return ICONS.customers;
  if (type === "repair") return ICONS.repairs;
  return ICONS.orders;
}

export function useCommandPalette() {
  const [open, setOpen] = useState(false);
  useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      const isMod = e.metaKey || e.ctrlKey;
      if (isMod && e.key.toLowerCase() === "k") {
        e.preventDefault();
        setOpen((v) => !v);
      } else if (e.key === "Escape") {
        setOpen(false);
      }
    };
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  }, []);
  return { open, setOpen };
}

export function CommandPalette({ open, onClose }: { open: boolean; onClose: () => void }) {
  const [query, setQuery] = useState("");
  const [results, setResults] = useState<SearchResult[]>([]);
  const [loading, setLoading] = useState(false);
  const [activeIndex, setActiveIndex] = useState(0);
  const inputRef = useRef<HTMLInputElement>(null);
  const navigate = useNavigate();
  const debounceRef = useRef<number | undefined>(undefined);

  useEffect(() => {
    if (open) {
      setQuery("");
      setResults([]);
      setActiveIndex(0);
      window.setTimeout(() => inputRef.current?.focus(), 20);
    }
  }, [open]);

  useEffect(() => {
    if (!open) return;
    window.clearTimeout(debounceRef.current);
    const q = query.trim();
    if (q.length < 2) {
      setResults([]);
      setLoading(false);
      return;
    }
    setLoading(true);
    debounceRef.current = window.setTimeout(async () => {
      try {
        const res = await globalSearch(q);
        setResults(res.results ?? []);
      } catch {
        setResults([]);
      } finally {
        setLoading(false);
      }
    }, 220);
    return () => window.clearTimeout(debounceRef.current);
  }, [query, open]);

  const filteredLinks = useMemo(() => {
    const q = query.trim().toLowerCase();
    if (!q) return QUICK_LINKS;
    return QUICK_LINKS.filter((l) => l.label.toLowerCase().includes(q) || l.keywords.includes(q));
  }, [query]);

  const items: PaletteItem[] = useMemo(() => {
    const searchItems: PaletteItem[] = results.map((r) => ({
      key: `${r.type}:${r.id}`,
      title: r.title,
      subtitle: r.subtitle,
      icon: resultIcon(r.type),
      onSelect: () => navigate(r.url),
    }));
    const linkItems: PaletteItem[] = filteredLinks.map((l) => ({
      key: `link:${l.path}`,
      title: l.label,
      subtitle: "Go to page",
      icon: ICONS[l.icon],
      onSelect: () => navigate(l.path),
    }));
    return query.trim().length >= 2 ? [...searchItems, ...linkItems] : linkItems;
  }, [results, filteredLinks, query, navigate]);

  useEffect(() => {
    setActiveIndex(0);
  }, [items.length]);

  const select = useCallback(
    (item: PaletteItem | undefined) => {
      if (!item) return;
      item.onSelect();
      onClose();
    },
    [onClose],
  );

  const onKeyDown = (e: React.KeyboardEvent) => {
    if (e.key === "ArrowDown") {
      e.preventDefault();
      setActiveIndex((i) => Math.min(i + 1, items.length - 1));
    } else if (e.key === "ArrowUp") {
      e.preventDefault();
      setActiveIndex((i) => Math.max(i - 1, 0));
    } else if (e.key === "Enter") {
      e.preventDefault();
      select(items[activeIndex]);
    } else if (e.key === "Escape") {
      onClose();
    }
  };

  if (!open) return null;

  return (
    <div className="cmdk-backdrop" onClick={onClose}>
      <div className="cmdk-panel" onClick={(e) => e.stopPropagation()} role="dialog" aria-modal="true" aria-label="Command palette">
        <div className="cmdk-input-row">
          <Icon d="m21 21-4.34-4.34" extra={<circle cx="11" cy="11" r="7" />} size={18} />
          <input
            ref={inputRef}
            className="cmdk-input"
            placeholder="Search customers, jobs, orders, or jump to a page…"
            aria-label="Search customers, jobs, orders, or jump to a page"
            role="combobox"
            aria-expanded={items.length > 0}
            aria-controls="cmdk-listbox"
            aria-activedescendant={items[activeIndex] ? `cmdk-option-${items[activeIndex].key}` : undefined}
            value={query}
            onChange={(e) => setQuery(e.target.value)}
            onKeyDown={onKeyDown}
          />
          <kbd className="cmdk-esc">Esc</kbd>
        </div>
        <div className="cmdk-results" id="cmdk-listbox" role="listbox" aria-label="Search results and pages">
          {loading ? <div className="cmdk-hint">Searching…</div> : null}
          {!loading && items.length === 0 ? <div className="cmdk-hint">No matches. Try a name, phone, job code, or order code.</div> : null}
          {items.map((item, idx) => (
            <button
              key={item.key}
              id={`cmdk-option-${item.key}`}
              type="button"
              role="option"
              aria-selected={idx === activeIndex}
              className={`cmdk-item ${idx === activeIndex ? "active" : ""}`}
              onMouseEnter={() => setActiveIndex(idx)}
              onClick={() => select(item)}
            >
              <span className="cmdk-item-icon">{item.icon}</span>
              <span className="cmdk-item-text">
                <strong>{item.title}</strong>
                {item.subtitle ? <span>{item.subtitle}</span> : null}
              </span>
            </button>
          ))}
        </div>
        <div className="cmdk-footer">
          <span><kbd>↑</kbd><kbd>↓</kbd> navigate</span>
          <span><kbd>Enter</kbd> open</span>
          <span><kbd>Ctrl</kbd>+<kbd>K</kbd> toggle</span>
        </div>
      </div>
    </div>
  );
}
