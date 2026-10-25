import { StrictMode, type ReactNode } from "react";
import { createRoot } from "react-dom/client";
import { BrowserRouter, Navigate, Route, Routes } from "react-router";
import { Layout } from "./components/Layout";
import "./index.css";
import { AuthProvider, useAuth } from "./lib/auth";
import { AuctionDetail } from "./pages/AuctionDetail";
import { AuthPage } from "./pages/Auth";
import { Catalog } from "./pages/Catalog";
import { ProfilePage } from "./pages/Profile";
import { Sell } from "./pages/Sell";
import { SellerPage } from "./pages/Seller";
import { Wallet } from "./pages/Wallet";

function RequireLogin({ children }: { children: ReactNode }) {
  return useAuth().userId ? children : <Navigate to="/masuk" replace />;
}

createRoot(document.getElementById("root")!).render(
  <StrictMode>
    <AuthProvider>
      <BrowserRouter>
        <Routes>
          <Route element={<Layout />}>
            <Route index element={<Catalog />} />
            <Route path="lelang/:id" element={<AuctionDetail />} />
            <Route path="penjual/:username" element={<SellerPage />} />
            <Route path="masuk" element={<AuthPage mode="login" />} />
            <Route path="daftar" element={<AuthPage mode="register" />} />
            <Route
              path="dompet"
              element={
                <RequireLogin>
                  <Wallet />
                </RequireLogin>
              }
            />
            <Route
              path="jual"
              element={
                <RequireLogin>
                  <Sell />
                </RequireLogin>
              }
            />
            <Route
              path="profil"
              element={
                <RequireLogin>
                  <ProfilePage />
                </RequireLogin>
              }
            />
            <Route path="*" element={<p className="text-ink-soft">Halaman tidak ditemukan.</p>} />
          </Route>
        </Routes>
      </BrowserRouter>
    </AuthProvider>
  </StrictMode>,
);
