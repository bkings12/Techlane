import {
  createContext,
  useCallback,
  useContext,
  useEffect,
  useMemo,
  useState,
  type ReactNode,
} from "react";
import { useAuth } from "../auth/AuthContext";
import { listBranches, type Branch } from "../lib/api";

const BRANCH_KEY = "techlane.branch";

type BranchState = {
  branches: Branch[];
  branchId: string;
  setBranchId: (id: string) => void;
  loading: boolean;
};

const BranchContext = createContext<BranchState | null>(null);

export function BranchProvider({ children }: { children: ReactNode }) {
  const { user } = useAuth();
  const [branches, setBranches] = useState<Branch[]>([]);
  const [branchId, setBranchIdState] = useState("");
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    if (!user) {
      setBranches([]);
      setBranchIdState("");
      setLoading(false);
      return;
    }
    const allowed = user.branch_ids ?? [];
    setLoading(true);
    listBranches()
      .then((res) => {
        const all = res.items ?? [];
        const filtered = allowed.length
          ? all.filter((b) => allowed.includes(b.id))
          : all;
        setBranches(filtered.length ? filtered : all);
      })
      .catch(() => {
        setBranches(
          allowed.map((id) => ({ id, name: id.slice(0, 8), code: "" })),
        );
      })
      .finally(() => setLoading(false));
  }, [user]);

  useEffect(() => {
    if (!user) return;
    const allowed = user.branch_ids ?? [];
    const saved = localStorage.getItem(BRANCH_KEY) ?? "";
    const pick =
      (saved && allowed.includes(saved) && saved) ||
      (saved && branches.some((b) => b.id === saved) && saved) ||
      allowed[0] ||
      branches[0]?.id ||
      "";
    setBranchIdState(pick);
  }, [user, branches]);

  const setBranchId = useCallback((id: string) => {
    localStorage.setItem(BRANCH_KEY, id);
    setBranchIdState(id);
  }, []);

  const value = useMemo(
    () => ({ branches, branchId, setBranchId, loading }),
    [branches, branchId, setBranchId, loading],
  );

  return <BranchContext.Provider value={value}>{children}</BranchContext.Provider>;
}

export function useBranch() {
  const ctx = useContext(BranchContext);
  if (!ctx) throw new Error("useBranch outside provider");
  return ctx;
}
