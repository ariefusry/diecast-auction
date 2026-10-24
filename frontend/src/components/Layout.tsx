import { NavLink, Outlet, useNavigate } from "react-router";
import { useAuth } from "../lib/auth";

const link = ({ isActive }: { isActive: boolean }) =>
  `rounded-md px-2 py-1.5 font-semibold sm:px-3 ${isActive ? "bg-white/15 text-white" : "text-white/75 hover:text-white"}`;

export function Layout() {
  const { userId, logout } = useAuth();
  const navigate = useNavigate();
  return (
    <div className="min-h-screen">
      <header className="bg-ink text-white">
        <nav className="mx-auto flex max-w-6xl flex-wrap items-center gap-x-1 gap-y-2 px-4 py-3 sm:gap-x-2">
          <NavLink to="/" className="mr-auto font-display text-xl font-extrabold italic tracking-tight sm:text-2xl">
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
                className="rounded-md px-2 py-1.5 font-semibold text-white/75 hover:text-white sm:px-3"
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
        Data katalog dari wiki Fandom Hot Wheels, Mini GT dan Inno64 (CC-BY-SA). Saldo adalah uang simulasi.
      </footer>
    </div>
  );
}
