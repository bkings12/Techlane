import { createContext, useCallback, useContext, useEffect, useState, type ReactNode } from "react";
import { useNavigate } from "react-router-dom";
import {
  clearSession,
  getToken,
  listRepairs,
  logout as apiLogout,
  me,
  onSessionExpired,
  type Customer,
  type Repair,
} from "./api";

type SessionState = {
  token: string | null;
  customer: Customer | null;
  repairs: Repair[];
  setToken: (token: string) => void;
  setCustomer: (customer: Customer | null) => void;
  refreshRepairs: () => Promise<Repair[]>;
  signOut: () => Promise<void>;
};

const SessionContext = createContext<SessionState | null>(null);

export function SessionProvider({ children }: { children: ReactNode }) {
  const navigate = useNavigate();
  const [token, setTokenState] = useState<string | null>(() => getToken());
  const [customer, setCustomer] = useState<Customer | null>(null);
  const [repairs, setRepairs] = useState<Repair[]>([]);

  const refreshRepairs = useCallback(async () => {
    const items = await listRepairs();
    setRepairs(items);
    return items;
  }, []);

  useEffect(() => {
    if (!token) return;
    me()
      .then(setCustomer)
      .catch(() => {
        clearSession();
        setTokenState(null);
      });
  }, [token]);

  useEffect(
    () =>
      onSessionExpired(() => {
        setTokenState(null);
        setCustomer(null);
        setRepairs([]);
        navigate("/login", { replace: true, state: { notice: "Your session expired. Please sign in again." } });
      }),
    [navigate],
  );

  async function signOut() {
    await apiLogout();
    setTokenState(null);
    setCustomer(null);
    setRepairs([]);
    navigate("/login", { replace: true });
  }

  return (
    <SessionContext.Provider
      value={{ token, customer, repairs, setToken: setTokenState, setCustomer, refreshRepairs, signOut }}
    >
      {children}
    </SessionContext.Provider>
  );
}

export function useSession() {
  const ctx = useContext(SessionContext);
  if (!ctx) throw new Error("useSession must be used within SessionProvider");
  return ctx;
}
