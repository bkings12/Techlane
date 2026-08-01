import {
  createContext,
  useCallback,
  useContext,
  useEffect,
  useMemo,
  useState,
  type ReactNode,
} from "react";
import {
  clearSession,
  getAccessToken,
  getMe,
  login as apiLogin,
  onSessionExpired,
  refreshSession,
  SessionExpiredError,
  signup as apiSignup,
  verifyMfaLogin,
  type LoginResult,
  type UserProfile,
} from "../lib/api";
import { closeRealtimeConnection } from "../lib/realtime";

type AuthState = {
  user: UserProfile | null;
  loading: boolean;
  login: (email: string, password: string) => Promise<LoginResult>;
  registerTenant: (input: { company_name: string; owner_name: string; email: string; password: string }) => Promise<void>;
  completeMfaLogin: (challenge: string, code: string) => Promise<void>;
  logout: () => void;
  primaryRole: string;
  sessionExpired: boolean;
  clearSessionExpired: () => void;
};

const AuthContext = createContext<AuthState | null>(null);

export function AuthProvider({ children }: { children: ReactNode }) {
  const [user, setUser] = useState<UserProfile | null>(null);
  const [loading, setLoading] = useState(true);
  const [sessionExpired, setSessionExpired] = useState(false);

  useEffect(() => {
    let cancelled = false;
    const token = getAccessToken();
    if (!token) {
      setLoading(false);
      return;
    }
    (async () => {
      try {
        const me = await getMe();
        if (!cancelled) setUser(me);
      } catch (err) {
        if (err instanceof SessionExpiredError) {
          clearSession();
          if (!cancelled) {
            setUser(null);
            setSessionExpired(true);
          }
          return;
        }
        // Deploy blip / brief 502: keep refresh token, try one rotation, then me.
        try {
          if (await refreshSession()) {
            const me = await getMe();
            if (!cancelled) setUser(me);
            return;
          }
        } catch {
          /* fall through */
        }
        if (!cancelled) setUser(null);
      } finally {
        if (!cancelled) setLoading(false);
      }
    })();
    return () => {
      cancelled = true;
    };
  }, []);

  // Keep the short-lived access token fresh while the tab is open so a brief
  // API restart does not strand the cashier on a wave of 401s.
  useEffect(() => {
    if (!user) return;
    const id = window.setInterval(() => {
      void refreshSession().catch(() => undefined);
    }, 10 * 60 * 1000);
    const onVisible = () => {
      if (document.visibilityState === "visible") {
        void refreshSession().catch(() => undefined);
      }
    };
    document.addEventListener("visibilitychange", onVisible);
    return () => {
      window.clearInterval(id);
      document.removeEventListener("visibilitychange", onVisible);
    };
  }, [user]);

  useEffect(
    () =>
      onSessionExpired(() => {
        closeRealtimeConnection();
        setUser((prev) => {
          if (prev) setSessionExpired(true);
          return null;
        });
      }),
    [],
  );

  const login = useCallback(async (email: string, password: string) => {
    const result = await apiLogin(email, password);
    if (result.status === "ok") {
      setUser(await getMe());
    }
    return result;
  }, []);

  const registerTenant = useCallback(
    async (input: { company_name: string; owner_name: string; email: string; password: string }) => {
      await apiSignup(input);
      setUser(await getMe());
    },
    [],
  );

  const completeMfaLogin = useCallback(async (challenge: string, code: string) => {
    await verifyMfaLogin(challenge, code);
    setUser(await getMe());
  }, []);

  const logout = useCallback(() => {
    clearSession();
    closeRealtimeConnection();
    setUser(null);
  }, []);

  const clearSessionExpired = useCallback(() => setSessionExpired(false), []);

  const primaryRole = user?.roles?.[0] ?? "guest";

  const value = useMemo(
    () => ({
      user,
      loading,
      login,
      registerTenant,
      completeMfaLogin,
      logout,
      primaryRole,
      sessionExpired,
      clearSessionExpired,
    }),
    [user, loading, login, registerTenant, completeMfaLogin, logout, primaryRole, sessionExpired, clearSessionExpired],
  );

  return <AuthContext.Provider value={value}>{children}</AuthContext.Provider>;
}

export function useAuth() {
  const ctx = useContext(AuthContext);
  if (!ctx) throw new Error("useAuth outside provider");
  return ctx;
}
