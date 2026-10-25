import { NavLink, Outlet, useNavigate } from "react-router";
import { useAuth } from "../lib/auth";

const link = ({ isActive }: { isActive: boolean }) =>
  `press rounded-full px-2.5 py-1.5 text-[15px] font-semibold sm:text-base sm:px-4 ${isActive ? "bg-ink text-white" : "text-ink-soft hover:text-ink"}`;

export function Layout() {
  const { userId, logout } = useAuth();
  const navigate = useNavigate();
  return (
    <div className="min-h-screen">
      <header className="material sticky top-0 z-20">
        <nav className="mx-auto flex max-w-6xl flex-wrap items-center gap-x-0.5 gap-y-2 px-4 py-3 sm:gap-x-2">
          <NavLink to="/" className="mr-auto font-display text-xl font-extrabold italic sm:text-2xl">
            Lelang 1:64
          </NavLink>
          <NavLink to="/" end className={link}>
            Katalog
          </NavLink>
          {userId ? (
            <>
              <NavLink to="/jual" className={link}>
                Jual
              </NavLink>
              <NavLink to="/dompet" className={link}>
                Dompet
              </NavLink>
              <button
                className="press rounded-full px-2.5 py-1.5 text-[15px] font-semibold sm:text-base text-ink-soft hover:text-ink sm:px-4"
                onClick={() => {
                  logout();
                  navigate("/");
                }}
              >
                Keluar
              </button>
            </>
          ) : (
            <NavLink to="/masuk" className={link}>
              Masuk
            </NavLink>
          )}
        </nav>
      </header>
      <main className="mx-auto max-w-6xl px-4 py-6 sm:py-8">
        <Outlet />
      </main>
      <footer className="mx-auto max-w-6xl px-4 pb-8 text-sm text-ink-soft">
        Data katalog dari wiki Fandom Hot Wheels, Mini GT, Inno64, Matchbox, Tomica, Tarmac Works dan M2 Machines (CC-BY-SA). Logo dan merek dagang milik pemiliknya masing-masing; situs ini tidak berafiliasi dengan mereka. Saldo adalah uang simulasi.
      </footer>
    </div>
  );
}
