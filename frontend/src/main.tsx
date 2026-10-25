import { StrictMode, type ReactNode } from "react";
import { createRoot } from "react-dom/client";
import { createBrowserRouter, createRoutesFromElements, Navigate, Route, RouterProvider } from "react-router";
import { Layout } from "./components/Layout";
import "./index.css";
import { AuthProvider, useAuth } from "./lib/auth";
import { ChatProvider } from "./lib/chats";
import { ChatPage } from "./pages/ChatPage";
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

// A data router, because only it runs <Link viewTransition> (opening a lot morphs its card).
const router = createBrowserRouter(
  createRoutesFromElements(
    <Route
      element={
        <ChatProvider>
          <Layout />
        </ChatProvider>
      }
    >
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
        path="chat/:id?"
        element={
          <RequireLogin>
            <ChatPage />
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
    </Route>,
  ),
);

createRoot(document.getElementById("root")!).render(
  <StrictMode>
    <AuthProvider>
      <RouterProvider router={router} />
    </AuthProvider>
  </StrictMode>,
);
