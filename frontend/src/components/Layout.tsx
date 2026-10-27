import { useLayoutEffect } from "react";
import { NavLink, Outlet, useLocation, useNavigationType } from "react-router";
import { useAuth } from "../lib/auth";
import { useChats } from "../lib/chats";
import { ChatPanel } from "./ChatPanel";
import { Icon, type IconName } from "./Icon";

const link = ({ isActive }: { isActive: boolean }) =>
  `press inline-flex min-h-11 items-center rounded-full px-2.5 text-[15px] font-semibold sm:text-base sm:px-4 ${isActive ? "bg-ink text-white" : "text-ink-soft hover:text-ink"}`;

function Badge({ count }: { count: number }) {
  if (count === 0) return null;
  return (
    <span className="absolute -top-1 -right-2 grid h-[18px] min-w-[18px] place-items-center rounded-full bg-track px-1 text-[11px] font-bold text-white tabular-nums ring-2 ring-paper">
      {count > 99 ? "99+" : count}
    </span>
  );
}

/**
 * Phones navigate from the bottom, in thumb reach, like a shop app. Hidden
 * where the bottom of the screen already belongs to an action: the bid form
 * on a lot, the composer in a conversation.
 */
function BottomNav({ unread }: { unread: number }) {
  const { userId } = useAuth();
  const tabs: [string, string, IconName][] = userId
    ? [
        ["/", "Katalog", "home"],
        ["/jual", "Jual", "tag"],
        ["/chat", "Chat", "chat"],
        ["/dompet", "Dompet", "wallet"],
        ["/profil", "Profil", "user"],
      ]
    : [
        ["/", "Katalog", "home"],
        ["/masuk", "Masuk", "login"],
      ];
  return (
    <nav aria-label="Navigasi" className="fixed inset-x-0 bottom-0 z-30 border-t border-line bg-paper/90 pb-[env(safe-area-inset-bottom)] backdrop-blur-xl sm:hidden">
      <div className="flex">
        {tabs.map(([to, label, icon]) => (
          <NavLink
            key={to}
            to={to}
            end={to === "/"}
            className={({ isActive }) => `press flex min-h-14 flex-1 flex-col items-center justify-center gap-0.5 text-[11px] font-semibold ${isActive ? "text-track-dark" : "text-ink-soft"}`}
          >
            <span className="relative">
              <Icon name={icon} className="size-6" />
              {to === "/chat" && <Badge count={unread} />}
            </span>
            {label}
          </NavLink>
        ))}
      </div>
    </nav>
  );
}

export function Layout() {
  const { userId } = useAuth();
  const { unread } = useChats();
  // A new page starts at the top, before an opened lot's pack morphs into place.
  // Back (POP) is left to the browser.
  const { pathname } = useLocation();
  const navigation = useNavigationType();
  useLayoutEffect(() => {
    if (navigation !== "POP") window.scrollTo(0, 0);
  }, [pathname, navigation]);
  const chat = pathname.startsWith("/chat");
  const bottomNav = !/^\/lelang\/|^\/chat\/./.test(pathname);
  return (
    <div className="min-h-screen">
      <header className="material sticky top-0 z-20">
        <nav className="mx-auto flex max-w-6xl flex-wrap items-center gap-x-0.5 gap-y-2 px-4 py-3 sm:gap-x-2">
          <NavLink to="/" className="press mr-auto flex min-h-11 min-w-11 items-center">
            <img src="/logo-symbol.svg" alt="Blister64" className="h-9 w-auto sm:hidden" />
            <img src="/logo.svg" alt="Blister64" className="hidden h-9 w-auto sm:block" />
          </NavLink>
          {/* On phones these live in the bottom bar. */}
          <div className="flex items-center gap-x-0.5 max-sm:hidden sm:gap-x-2">
            <NavLink to="/" end className={link}>
              Katalog
            </NavLink>
            {userId ? (
              <>
                <NavLink to="/jual" className={link}>
                  Jual
                </NavLink>
                <NavLink to="/chat" className={link}>
                  <span className="relative">
                    Chat
                    <Badge count={unread} />
                  </span>
                </NavLink>
                <NavLink to="/dompet" className={link}>
                  Dompet
                </NavLink>
                {/* Keluar lives on the profile page, so the bar still fits a phone. */}
                <NavLink to="/profil" className={link}>
                  Profil
                </NavLink>
              </>
            ) : (
              <NavLink to="/masuk" className={link}>
                Masuk
              </NavLink>
            )}
          </div>
        </nav>
      </header>
      <main className={`mx-auto max-w-6xl px-4 py-6 sm:py-8 ${bottomNav ? "max-sm:pb-24" : ""}`}>
        <Outlet />
      </main>
      <footer className={`mx-auto max-w-6xl px-4 pb-8 text-sm text-ink-soft ${chat ? "hidden" : ""} ${bottomNav ? "max-sm:pb-24" : ""}`}>
        Data katalog dari wiki Fandom Hot Wheels, Mini GT, Inno64, Matchbox, Tomica, Tarmac Works dan M2 Machines (CC-BY-SA). Logo dan merek dagang milik pemiliknya masing-masing; situs ini tidak berafiliasi dengan mereka. Saldo adalah uang simulasi.
      </footer>
      {bottomNav && <BottomNav unread={unread} />}
      <ChatPanel />
    </div>
  );
}
