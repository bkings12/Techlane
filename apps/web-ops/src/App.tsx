import { Navigate, Route, Routes } from "react-router-dom";
import { lazy, Suspense, useEffect } from "react";
import { AuthProvider, useAuth } from "./auth/AuthContext";
import { BranchProvider } from "./branch/BranchContext";
import { CurrencyProvider } from "./lib/currency";
import { AppShell } from "./layout/AppShell";
import { CustomersPage } from "./pages/CustomersPage";
import { CustomerDetailPage } from "./pages/CustomerDetailPage";
import { HomePage } from "./pages/HomePage";
import { LoginPage } from "./pages/LoginPage";
import { SignupPage } from "./pages/SignupPage";
import { ForgotPasswordPage } from "./pages/ForgotPasswordPage";
import { ResetPasswordPage } from "./pages/ResetPasswordPage";
import { NotificationsPage } from "./pages/NotificationsPage";
import { OrdersPage } from "./pages/OrdersPage";
import { PartRequestsPage } from "./pages/PartRequestsPage";
import { PaymentsPage } from "./pages/PaymentsPage";
import { AuditPage } from "./pages/AuditPage";
import { RiskPage } from "./pages/RiskPage";
import { SuppliersPage } from "./pages/SuppliersPage";
import { SyncPage } from "./pages/SyncPage";
import { TrashPage } from "./pages/TrashPage";
import { CommissionsPage } from "./pages/settings/CommissionsPage";
import { BranchesPage } from "./pages/settings/BranchesPage";
import { PaymentSettingsPage } from "./pages/settings/PaymentSettingsPage";
import { ReceiptsPage } from "./pages/settings/ReceiptsPage";
import { StorefrontSettingsPage } from "./pages/settings/StorefrontSettingsPage";
import { StorefrontBannersPage } from "./pages/settings/StorefrontBannersPage";
import { StorefrontDealsPage } from "./pages/settings/StorefrontDealsPage";
import { DeliveryLocationsPage } from "./pages/settings/DeliveryLocationsPage";
import { StorefrontSubscribersPage } from "./pages/settings/StorefrontSubscribersPage";
import { StorefrontReviewsPage } from "./pages/settings/StorefrontReviewsPage";
import { ShopProfilePage } from "./pages/settings/ShopProfilePage";
import { SMSSettingsPage } from "./pages/settings/SMSSettingsPage";
import { WhatsAppSettingsPage } from "./pages/settings/WhatsAppSettingsPage";
import { LoyaltyMarketingPage } from "./pages/settings/LoyaltyMarketingPage";
import { RolesPage } from "./pages/settings/RolesPage";
import { IntakePresetsPage } from "./pages/settings/IntakePresetsPage";
import { SecuritySettingsPage } from "./pages/settings/SecuritySettingsPage";
import { SettingsHomePage } from "./pages/settings/SettingsHomePage";
import { SettingsLayout } from "./pages/settings/SettingsLayout";
import { StaffDetailPage } from "./pages/settings/StaffDetailPage";
import { StaffListPage } from "./pages/settings/StaffListPage";

const RepairsPage = lazy(() => import("./pages/RepairsPage").then((module) => ({ default: module.RepairsPage })));
const RepairDetailPage = lazy(() => import("./pages/RepairDetailPage").then((module) => ({ default: module.RepairDetailPage })));
const InventoryPage = lazy(() => import("./pages/InventoryPage").then((module) => ({ default: module.InventoryPage })));
const POSPage = lazy(() => import("./pages/POSPage").then((module) => ({ default: module.POSPage })));
const QuickFixPage = lazy(() => import("./pages/QuickFixPage").then((module) => ({ default: module.QuickFixPage })));
const CounterPickupPage = lazy(() => import("./pages/CounterPickupPage").then((module) => ({ default: module.CounterPickupPage })));
const ReportsPage = lazy(() => import("./pages/ReportsPage").then((module) => ({ default: module.ReportsPage })));

/** /downloads (plural) used to fall through to the SPA with no route → blank page. */
function DownloadsAliasRedirect() {
  useEffect(() => {
    window.location.replace("/download/");
  }, []);
  return <div className="boot">Opening downloads…</div>;
}

function Protected({ children }: { children: React.ReactNode }) {
  const { user, loading } = useAuth();
  if (loading) return <div className="boot">Loading…</div>;
  if (!user) return <Navigate to="/login" replace />;
  return (
    <BranchProvider>
      <CurrencyProvider>{children}</CurrencyProvider>
    </BranchProvider>
  );
}

export default function App() {
  return (
    <AuthProvider>
      <Suspense fallback={<div className="boot">Opening workspace…</div>}>
      <Routes>
        <Route path="/downloads" element={<DownloadsAliasRedirect />} />
        <Route path="/downloads/" element={<DownloadsAliasRedirect />} />
        <Route path="/login" element={<LoginPage />} />
        <Route path="/signup" element={<SignupPage />} />
        <Route path="/forgot-password" element={<ForgotPasswordPage />} />
        <Route path="/reset-password" element={<ResetPasswordPage />} />
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
          <Route path="orders" element={<OrdersPage />} />
          <Route path="payments" element={<PaymentsPage />} />
          <Route path="pos" element={<POSPage />} />
          <Route path="counter/fix" element={<QuickFixPage />} />
          <Route path="counter/pickup" element={<CounterPickupPage />} />
          <Route path="notifications" element={<NotificationsPage />} />
          <Route path="risk" element={<RiskPage />} />
          <Route path="reports" element={<ReportsPage />} />
          <Route path="audit" element={<AuditPage />} />
          <Route path="sync" element={<SyncPage />} />
          <Route path="trash" element={<TrashPage />} />
          <Route path="settings" element={<SettingsLayout />}>
            <Route index element={<SettingsHomePage />} />
            <Route path="branches" element={<BranchesPage />} />
            <Route path="staff" element={<StaffListPage />} />
            <Route path="staff/:id" element={<StaffDetailPage />} />
            <Route path="roles" element={<RolesPage />} />
            <Route path="intake-presets" element={<IntakePresetsPage />} />
            <Route path="commissions" element={<CommissionsPage />} />
            <Route path="payments" element={<PaymentSettingsPage />} />
            <Route path="shop" element={<ShopProfilePage />} />
            <Route path="receipts" element={<ReceiptsPage />} />
            <Route path="storefront" element={<StorefrontSettingsPage />} />
            <Route path="storefront/banners" element={<StorefrontBannersPage />} />
            <Route path="storefront/deals" element={<StorefrontDealsPage />} />
            <Route path="storefront/delivery" element={<DeliveryLocationsPage />} />
            <Route path="storefront/subscribers" element={<StorefrontSubscribersPage />} />
            <Route path="storefront/reviews" element={<StorefrontReviewsPage />} />
            <Route path="sms" element={<SMSSettingsPage />} />
            <Route path="whatsapp" element={<WhatsAppSettingsPage />} />
            <Route path="loyalty" element={<LoyaltyMarketingPage />} />
            <Route path="security" element={<SecuritySettingsPage />} />
          </Route>
        </Route>
      </Routes>
      </Suspense>
    </AuthProvider>
  );
}
