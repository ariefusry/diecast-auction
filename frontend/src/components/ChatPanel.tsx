import { Link, useLocation } from "react-router";
import { useAuth } from "../lib/auth";
import { useChats } from "../lib/chats";
import { Icon } from "./Icon";
import { Inbox } from "./Inbox";

/**
 * Chat on wide screens, docked bottom right on every page like a shop's chat:
 * a pill with the unread count that opens into the two-pane inbox.
 * Phones use the Chat tab and page instead.
 */
export function ChatPanel() {
  const { userId } = useAuth();
  const { unread, panel, setPanel } = useChats();
  const { pathname } = useLocation();
  if (userId === null || pathname.startsWith("/chat")) return null;

  if (!panel.open) {
    return (
      <button
        onClick={() => setPanel({ ...panel, open: true })}
        className="press fixed right-6 bottom-6 z-40 hidden items-center gap-2 rounded-full bg-ink py-3 pr-5 pl-4 font-semibold text-white shadow-[0_10px_30px_-10px_rgb(20_33_61/0.6)] lg:flex"
      >
        <Icon name="chat" className="size-5" />
        Chat
        {unread > 0 && <span className="grid h-5 min-w-5 place-items-center rounded-full bg-track px-1.5 text-xs font-bold tabular-nums">{unread > 99 ? "99+" : unread}</span>}
      </button>
    );
  }
  return (
    <section
      aria-label="Chat"
      className="chat-panel fixed right-6 bottom-0 z-40 hidden h-[34rem] max-h-[calc(100dvh-5rem)] w-[48rem] max-w-[calc(100vw-3rem)] flex-col overflow-hidden rounded-t-2xl border border-line bg-card shadow-[0_-12px_40px_-16px_rgb(20_33_61/0.45)] lg:flex"
    >
      <header className="flex items-center gap-2 border-b border-line px-4 py-2.5">
        <h2 className="font-display text-2xl font-extrabold text-track-dark italic">Chat</h2>
        {unread > 0 && <span className="text-sm font-semibold text-track-dark">({unread})</span>}
        <Link
          to={panel.selected ? `/chat/${panel.selected}` : "/chat"}
          onClick={() => setPanel({ ...panel, open: false })}
          className="ml-auto rounded-lg px-2 py-1 text-sm font-semibold text-ink-soft hover:bg-paper hover:text-ink"
        >
          Buka halaman penuh
        </Link>
        <button onClick={() => setPanel({ ...panel, open: false })} aria-label="Kecilkan chat" className="press grid size-8 place-items-center rounded-lg text-ink-soft hover:bg-paper hover:text-ink">
          <Icon name="down" className="size-5" />
        </button>
      </header>
      <Inbox split selected={panel.selected} onSelect={(id) => setPanel({ open: true, selected: id })} />
    </section>
  );
}
