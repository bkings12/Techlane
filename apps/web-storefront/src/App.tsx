import { Route, Routes } from "react-router-dom";
import { StorefrontProvider } from "./store/StorefrontContext";
import { Layout, NotFoundPage } from "./components/Layout";
import { HomePage } from "./pages/HomePage";
import { ShopPage } from "./pages/ShopPage";
import { ProductPage } from "./pages/ProductPage";
import { CartPage } from "./pages/CartPage";
import { CheckoutPage } from "./pages/CheckoutPage";
import { PayPage } from "./pages/PayPage";
import { DonePage } from "./pages/DonePage";
import { LookupPage } from "./pages/LookupPage";
import { AccountPage } from "./pages/AccountPage";
import { LocatorPage } from "./pages/LocatorPage";
import { SavedPage } from "./pages/SavedPage";
import { ContactPage } from "./pages/ContactPage";

export default function App() {
  return (
    <StorefrontProvider>
      <Routes>
        <Route element={<Layout />}>
          <Route index element={<HomePage />} />
          <Route path="shop" element={<ShopPage />} />
          <Route path="product/:variantId" element={<ProductPage />} />
          <Route path="cart" element={<CartPage />} />
          <Route path="checkout" element={<CheckoutPage />} />
          <Route path="pay/:orderId" element={<PayPage />} />
          <Route path="done/:orderId" element={<DonePage />} />
          <Route path="lookup" element={<LookupPage />} />
          <Route path="lookup/:orderId" element={<LookupPage />} />
          <Route path="account" element={<AccountPage />} />
          <Route path="stores" element={<LocatorPage />} />
          <Route path="saved" element={<SavedPage />} />
          <Route path="contact" element={<ContactPage />} />
          <Route path="*" element={<NotFoundPage />} />
        </Route>
      </Routes>
    </StorefrontProvider>
  );
}
