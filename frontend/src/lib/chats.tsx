import { createContext, useCallback, useContext, useEffect, useState, useSyncExternalStore, type ReactNode } from "react";
import { useNavigate } from "react-router";
import { api, type Conversation } from "./api";
import { useAuth } from "./auth";

type Chats = {
  conversations: Conversation[] | null;
  unread: number;
  refresh: () => void;
  /** Opens a chat: the floating panel on wide screens, the chat page on phones. */
  openChat: (auctionId: number) => void;
  panel: { open: boolean; selected: number | null };
  setPanel: (p: { open: boolean; selected: number | null }) => void;
};

const ChatContext = createContext<Chats | null>(null);

// Wide enough for the two-pane inbox and the floating panel (Tailwind's lg).
const wideQuery = "(min-width: 64rem)";
const wide = () => window.matchMedia(wideQuery).matches;

/** Whether the screen is wide enough to show the inbox's list and conversation side by side. */
export function useWide(): boolean {
  return useSyncExternalStore(
    (change) => {
      const m = window.matchMedia(wideQuery);
      m.addEventListener("change", change);
      return () => m.removeEventListener("change", change);
    },
    wide,
  );
}

export function ChatProvider({ children }: { children: ReactNode }) {
  const { userId } = useAuth();
  const navigate = useNavigate();
  const [conversations, setConversations] = useState<Conversation[] | null>(null);
  const [panel, setPanel] = useState<{ open: boolean; selected: number | null }>({ open: false, selected: null });

  const refresh = useCallback(() => {
    if (userId === null) return;
    api<Conversation[]>("/chats").then(setConversations, () => undefined);
  }, [userId]);

  // The unread badge polls while signed in, and rests while the tab is hidden.
  useEffect(() => {
    if (userId === null) {
      setConversations(null);
      return;
    }
    refresh();
    const t = setInterval(() => document.visibilityState === "visible" && refresh(), 15_000);
    const back = () => document.visibilityState === "visible" && refresh();
    document.addEventListener("visibilitychange", back);
    return () => {
      clearInterval(t);
      document.removeEventListener("visibilitychange", back);
    };
  }, [userId, refresh]);

  const openChat = useCallback(
    (auctionId: number) => {
      if (wide()) setPanel({ open: true, selected: auctionId });
      else navigate(`/chat/${auctionId}`);
    },
    [navigate],
  );

  const unread = conversations?.reduce((n, c) => n + c.unread, 0) ?? 0;
  return <ChatContext.Provider value={{ conversations, unread, refresh, openChat, panel, setPanel }}>{children}</ChatContext.Provider>;
}

export function useChats(): Chats {
  const ctx = useContext(ChatContext);
  if (!ctx) throw new Error("useChats outside ChatProvider");
  return ctx;
}
