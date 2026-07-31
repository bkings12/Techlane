import {
  createContext,
  useContext,
  useEffect,
  useMemo,
  useState,
  type ReactNode,
} from "react";
import { useAuth } from "../auth/AuthContext";
import { getShopProfile } from "./api";

// i18n / multi-currency groundwork: the shop sets its currency + locale once
// (Settings > Shop profile) and every amount in the app formats through this
// context instead of hardcoding "KES". New pages should call useCurrency()
// rather than re-implementing their own formatter.
type CurrencyState = {
  currencyCode: string;
  locale: string;
  formatMoney: (amount: number, opts?: { withCode?: boolean }) => string;
};

const DEFAULT_CURRENCY = "KES";
const DEFAULT_LOCALE = "en-KE";

const CurrencyContext = createContext<CurrencyState | null>(null);

function buildFormatter(currencyCode: string, locale: string) {
  return (amount: number, opts?: { withCode?: boolean }) => {
    const value = Number.isFinite(amount) ? amount : 0;
    try {
      const formatted = new Intl.NumberFormat(locale, {
        style: "currency",
        currency: currencyCode,
        currencyDisplay: opts?.withCode === false ? "narrowSymbol" : "code",
        maximumFractionDigits: 2,
      }).format(value);
      return formatted;
    } catch {
      return `${currencyCode} ${value.toLocaleString()}`;
    }
  };
}

export function CurrencyProvider({ children }: { children: ReactNode }) {
  const { user } = useAuth();
  const [currencyCode, setCurrencyCode] = useState(DEFAULT_CURRENCY);
  const [locale, setLocale] = useState(DEFAULT_LOCALE);

  useEffect(() => {
    if (!user) {
      setCurrencyCode(DEFAULT_CURRENCY);
      setLocale(DEFAULT_LOCALE);
      return;
    }
    let cancelled = false;
    getShopProfile()
      .then((profile) => {
        if (cancelled) return;
        setCurrencyCode(profile.currency_code || DEFAULT_CURRENCY);
        setLocale(profile.locale || DEFAULT_LOCALE);
      })
      .catch(() => {
        /* keep defaults — shop just hasn't set a profile yet */
      });
    return () => {
      cancelled = true;
    };
  }, [user]);

  const value = useMemo<CurrencyState>(
    () => ({ currencyCode, locale, formatMoney: buildFormatter(currencyCode, locale) }),
    [currencyCode, locale],
  );

  return <CurrencyContext.Provider value={value}>{children}</CurrencyContext.Provider>;
}

export function useCurrency() {
  const ctx = useContext(CurrencyContext);
  if (!ctx) {
    // Pages rendered outside the provider (rare) still get sane KES defaults.
    return { currencyCode: DEFAULT_CURRENCY, locale: DEFAULT_LOCALE, formatMoney: buildFormatter(DEFAULT_CURRENCY, DEFAULT_LOCALE) };
  }
  return ctx;
}
