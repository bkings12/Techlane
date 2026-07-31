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
    const token = getAccessToken();
    if (!token) {
      setLoading(false);
      return;
    }
    getMe()
      .then(setUser)
      .catch(() => {
        clearSession();
        setUser(null);
      })
      .finally(() => setLoading(false));
  }, []);

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
