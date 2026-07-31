import type { ReactNode } from "react";
import { Navigate, Route, Routes, useLocation } from "react-router-dom";
import { AuthPage } from "./pages/AuthPage";
import { GuestPage } from "./pages/GuestPage";
import { ProfilePage } from "./pages/ProfilePage";
import { RepairDetailPage } from "./pages/RepairDetailPage";
import { RepairsPage } from "./pages/RepairsPage";
import { SessionProvider, useSession } from "./session";

function RequireAuth({ children }: { children: ReactNode }) {
  const { token } = useSession();
  const location = useLocation();
  if (!token) return <Navigate to="/login" replace state={{ from: location }} />;
  return <>{children}</>;
}

function AppRoutes() {
  return (
    <Routes>
      <Route path="/login" element={<AuthPage />} />
      <Route path="/guest" element={<GuestPage />} />
      <Route
        path="/"
        element={
          <RequireAuth>
            <RepairsPage />
          </RequireAuth>
        }
      />
      <Route
        path="/repairs/:id"
        element={
          <RequireAuth>
            <RepairDetailPage />
          </RequireAuth>
        }
      />
      <Route
        path="/profile"
        element={
          <RequireAuth>
            <ProfilePage />
          </RequireAuth>
        }
      />
      <Route path="*" element={<Navigate to="/" replace />} />
    </Routes>
  );
}

function App() {
  return (
    <SessionProvider>
      <AppRoutes />
    </SessionProvider>
  );
}

export default App;
