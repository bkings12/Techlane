import { Navigate, Route, Routes } from "react-router-dom";
import { AuthProvider, useAuth } from "./auth/AuthContext";
import { BranchProvider } from "./branch/BranchContext";
import { AppShell } from "./layout/AppShell";
import { CustomersPage } from "./pages/CustomersPage";
import { CustomerDetailPage } from "./pages/CustomerDetailPage";
import { HomePage } from "./pages/HomePage";
import { LoginPage } from "./pages/LoginPage";
import { InventoryPage } from "./pages/InventoryPage";
import { NotificationsPage } from "./pages/NotificationsPage";
import { OrdersPage } from "./pages/OrdersPage";
import { PartRequestsPage } from "./pages/PartRequestsPage";
import { PaymentsPage } from "./pages/PaymentsPage";
import { POSPage } from "./pages/POSPage";
import { RepairDetailPage } from "./pages/RepairDetailPage";
import { RepairsPage } from "./pages/RepairsPage";
import { ReportsPage } from "./pages/ReportsPage";
import { AuditPage } from "./pages/AuditPage";
import { RiskPage } from "./pages/RiskPage";
import { SuppliersPage } from "./pages/SuppliersPage";
import { SyncPage } from "./pages/SyncPage";
import { CommissionsPage } from "./pages/settings/CommissionsPage";
import { BranchesPage } from "./pages/settings/BranchesPage";
import { PaymentSettingsPage } from "./pages/settings/PaymentSettingsPage";
import { ShopProfilePage } from "./pages/settings/ShopProfilePage";
import { SMSSettingsPage } from "./pages/settings/SMSSettingsPage";
import { RolesPage } from "./pages/settings/RolesPage";
import { SettingsHomePage } from "./pages/settings/SettingsHomePage";
import { StaffDetailPage } from "./pages/settings/StaffDetailPage";
import { StaffListPage } from "./pages/settings/StaffListPage";

function Protected({ children }: { children: React.ReactNode }) {
  const { user, loading } = useAuth();
  if (loading) return <div className="boot">Loading…</div>;
  if (!user) return <Navigate to="/login" replace />;
  return <BranchProvider>{children}</BranchProvider>;
}

export default function App() {
  return (
    <AuthProvider>
      <Routes>
        <Route path="/login" element={<LoginPage />} />
        <Route
          path="/"
          element={
            <Protected>
              <AppShell />
            </Protected>
          }
        >
          <Route index element={<HomePage />} />
          <Route path="repairs" element={<RepairsPage />} />
          <Route path="repairs/:id" element={<RepairDetailPage />} />
          <Route path="customers" element={<CustomersPage />} />
          <Route path="customers/:id" element={<CustomerDetailPage />} />
          <Route path="parts" element={<PartRequestsPage />} />
          <Route path="inventory" element={<InventoryPage />} />
          <Route path="suppliers" element={<SuppliersPage />} />
          <Route path="pos" element={<POSPage />} />
          <Route path="orders" element={<OrdersPage />} />
          <Route path="payments" element={<PaymentsPage />} />
          <Route path="notifications" element={<NotificationsPage />} />
          <Route path="risk" element={<RiskPage />} />
          <Route path="reports" element={<ReportsPage />} />
          <Route path="audit" element={<AuditPage />} />
          <Route path="sync" element={<SyncPage />} />
          <Route path="settings" element={<SettingsHomePage />} />
          <Route path="settings/branches" element={<BranchesPage />} />
          <Route path="settings/staff" element={<StaffListPage />} />
          <Route path="settings/staff/:id" element={<StaffDetailPage />} />
          <Route path="settings/roles" element={<RolesPage />} />
          <Route path="settings/commissions" element={<CommissionsPage />} />
          <Route path="settings/payments" element={<PaymentSettingsPage />} />
          <Route path="settings/shop" element={<ShopProfilePage />} />
          <Route path="settings/sms" element={<SMSSettingsPage />} />
        </Route>
      </Routes>
    </AuthProvider>
  );
}
