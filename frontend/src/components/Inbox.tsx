import { useCallback, useEffect, useRef, useState, type FormEvent } from "react";
import { Link } from "react-router";
import { api, errorMessage, imageSrc, type Conversation, type Message } from "../lib/api";
import { useChats } from "../lib/chats";
import { rupiah, timeAgo } from "../lib/format";
import { useNow } from "../lib/useNow";
import { Icon } from "./Icon";

const clock = new Intl.DateTimeFormat("id-ID", { hour: "2-digit", minute: "2-digit" });
const day = new Intl.DateTimeFormat("id-ID", { day: "2-digit", month: "2-digit" });

/** "17:56" today, "11/08" before that: the inbox's time column. */
function when(iso: string | null): string {
  if (!iso) return "";
  const d = new Date(iso);
  return d.toDateString() === new Date().toDateString() ? clock.format(d) : day.format(d);
}

function Avatar({ name, className = "size-11 text-lg" }: { name: string; className?: string }) {
  return (
    <span aria-hidden className={`grid shrink-0 place-items-center rounded-full bg-ink font-display font-extrabold text-white uppercase ${className}`}>
      {name.charAt(0)}
    </span>
  );
}

/** The conversation list: search by name, all or unread only, newest activity first. */
function ConversationList({ selected, onSelect }: { selected: number | null; onSelect: (id: number) => void }) {
  const { conversations } = useChats();
  const [query, setQuery] = useState("");
  const [unreadOnly, setUnreadOnly] = useState(false);
  const q = query.trim().toLowerCase();
  const shown = (conversations ?? []).filter((c) => (!unreadOnly || c.unread > 0) && (!q || c.other.toLowerCase().includes(q) || c.title.toLowerCase().includes(q)));

  return (
    <div className="flex min-h-0 flex-col">
      <div className="flex gap-2 border-b border-line p-3">
        <label className="flex min-w-0 flex-1 items-center gap-2 rounded-lg border border-line bg-card px-2.5 focus-within:border-ink">
          <Icon name="search" className="size-4 shrink-0 text-ink-soft" />
          <input value={query} onChange={(e) => setQuery(e.target.value)} placeholder="Cari nama" aria-label="Cari nama atau lot" className="min-w-0 flex-1 bg-transparent py-2 text-sm focus:outline-none" />
        </label>
        <select value={unreadOnly ? "unread" : "all"} onChange={(e) => setUnreadOnly(e.target.value === "unread")} aria-label="Tampilkan" className="rounded-lg border border-line bg-card px-2 text-sm">
          <option value="all">Semua</option>
          <option value="unread">Belum dibaca</option>
        </select>
      </div>
      {conversations === null ? (
        <p className="p-6 text-center text-sm text-ink-soft">Memuat percakapan…</p>
      ) : shown.length === 0 ? (
        <p className="p-6 text-center text-sm text-ink-soft">
          {conversations.length === 0 ? "Belum ada percakapan. Chat terbuka setelah lelang dimenangkan." : "Tidak ada percakapan yang cocok."}
        </p>
      ) : (
        <ul className="min-h-0 flex-1 overflow-y-auto">
          {shown.map((c) => (
            <li key={c.auction_id}>
              <button
                onClick={() => onSelect(c.auction_id)}
                aria-current={selected === c.auction_id}
                className={`flex w-full items-center gap-3 px-3 py-3 text-left hover:bg-paper ${selected === c.auction_id ? "bg-paper" : ""}`}
              >
                <Avatar name={c.other} />
                <span className="min-w-0 flex-1">
                  <span className="flex items-baseline justify-between gap-2">
                    <span className="truncate font-semibold">{c.other}</span>
                    <span className="shrink-0 text-xs text-ink-soft tabular-nums">{when(c.last_at)}</span>
                  </span>
                  <span className="flex items-center justify-between gap-2">
                    <span className={`truncate text-sm ${c.unread ? "font-semibold text-ink" : "text-ink-soft"}`}>
                      {c.last_body ? `${c.last_mine ? "Anda: " : ""}${c.last_body}` : `${c.selling ? "Terjual" : "Menang"}: ${c.title}`}
                    </span>
                    {c.unread > 0 && (
                      <span className="grid h-5 min-w-5 shrink-0 place-items-center rounded-full bg-track px-1.5 text-xs font-bold text-white tabular-nums">{c.unread > 99 ? "99+" : c.unread}</span>
                    )}
                  </span>
                </span>
              </button>
            </li>
          ))}
        </ul>
      )}
    </div>
  );
}

/** One conversation: the lot it is about, the messages, and the composer. Polls like the auction page. */
function ConversationView({ auctionId, conversation, onBack }: { auctionId: number; conversation?: Conversation; onBack?: () => void }) {
  const { refresh } = useChats();
  const now = useNow();
  const [messages, setMessages] = useState<Message[]>([]);
  const [draft, setDraft] = useState("");
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");
  const end = useRef<HTMLDivElement>(null);
  // Bumped by every request: a poll that started before a send must not land
  // after it and hide the message just sent.
  const seq = useRef(0);
  const marked = useRef(0);

  const load = useCallback(() => {
    const mine = ++seq.current;
    return api<Message[]>(`/auctions/${auctionId}/messages`).then(
      (list) => mine === seq.current && setMessages(list),
      (e) => mine === seq.current && setError(errorMessage(e)),
    );
  }, [auctionId]);

  useEffect(() => {
    setMessages([]);
    setError("");
    marked.current = 0;
    load();
    const t = setInterval(load, 5000);
    return () => clearInterval(t);
  }, [load]);

  // Read means seen: mark only while this conversation is on screen.
  const newest = messages.at(-1)?.id ?? 0;
  useEffect(() => {
    if (newest <= marked.current || document.visibilityState !== "visible") return;
    marked.current = newest;
    api(`/auctions/${auctionId}/read`, { method: "POST", body: { up_to: newest } }).then(refresh, () => undefined);
  }, [auctionId, newest, refresh]);

  const count = messages.length;
  useEffect(() => {
    end.current?.scrollIntoView({ block: "nearest" });
  }, [count]);

  async function send(e: FormEvent) {
    e.preventDefault();
    if (!draft.trim()) return;
    setBusy(true);
    setError("");
    ++seq.current;
    try {
      const m = await api<Message>(`/auctions/${auctionId}/messages`, { method: "POST", body: { body: draft } });
      setMessages((ms) => (ms.some((x) => x.id === m.id) ? ms : [...ms, m]));
      setDraft("");
      refresh();
    } catch (err) {
      setError(errorMessage(err));
    } finally {
      setBusy(false);
    }
  }

  return (
    <div className="flex min-h-0 flex-1 flex-col">
      <div className="flex items-center gap-2 border-b border-line px-3 py-2.5">
        {onBack && (
          <button onClick={onBack} aria-label="Kembali ke daftar chat" className="press -ml-1 grid size-9 place-items-center rounded-full hover:bg-paper">
            <Icon name="back" />
          </button>
        )}
        {conversation && <Avatar name={conversation.other} className="size-8 text-sm" />}
        <div className="min-w-0">
          <div className="truncate font-semibold">{conversation?.other ?? "Chat"}</div>
          {conversation && <div className="text-xs text-ink-soft">{conversation.selling ? "Pemenang lelang Anda" : "Penjual"}</div>}
        </div>
      </div>

      {conversation && (
        // The lot this chat is about, pinned on top like a shop's product card.
        <div className="flex items-center gap-3 border-b border-line bg-card px-3 py-2.5">
          <span className="grid size-12 shrink-0 place-items-center overflow-hidden rounded-lg bg-paper">
            {conversation.image_url && <img src={imageSrc(conversation.image_url)} alt="" referrerPolicy="no-referrer" className="h-full w-full object-contain mix-blend-multiply" />}
          </span>
          <div className="min-w-0 flex-1">
            <div className="truncate text-sm font-semibold">{conversation.title}</div>
            <div className="text-sm">
              <span className="font-display text-base font-bold text-track-dark tabular-nums">{rupiah(conversation.price)}</span>
              <span className="text-ink-soft"> · {conversation.selling ? "Terjual" : "Anda menang"}</span>
            </div>
          </div>
          <Link to={`/lelang/${auctionId}`} className="press shrink-0 rounded-lg border border-track px-3 py-1.5 text-sm font-semibold text-track-dark hover:bg-track/10">
            Lihat lot
          </Link>
        </div>
      )}

      <div className="min-h-0 flex-1 space-y-2 overflow-y-auto bg-paper p-3" aria-live="polite">
        {messages.length === 0 && !error && <p className="py-8 text-center text-sm text-ink-soft">Belum ada pesan. Mulai dengan alamat pengiriman.</p>}
        {messages.map((m) =>
          m.system ? (
            // The sale notice that opens every chat: a note from the site, not either party.
            <p key={m.id} className="mx-auto max-w-[90%] rounded-lg border border-line bg-card px-3 py-2 text-center text-sm text-ink-soft">
              {m.body}
            </p>
          ) : (
            <div key={m.id} className={`flex ${m.mine ? "justify-end" : "justify-start"}`}>
              <div className={`max-w-[80%] rounded-2xl px-3.5 py-2 ${m.mine ? "rounded-br-sm bg-ink text-white" : "rounded-bl-sm bg-card"}`}>
                <p className="break-words whitespace-pre-line">{m.body}</p>
                <p className={`mt-0.5 text-right text-xs ${m.mine ? "text-white/60" : "text-ink-soft"}`}>{timeAgo(m.created_at, now)}</p>
              </div>
            </div>
          ),
        )}
        <div ref={end} />
      </div>

      {error && <p className="border-t border-line bg-signal/10 px-3 py-2 text-sm text-signal">{error}</p>}
      <form onSubmit={send} className="flex items-center gap-2 border-t border-line bg-card p-2.5 pb-[max(0.625rem,env(safe-area-inset-bottom))]">
        <input
          value={draft}
          onChange={(e) => setDraft(e.target.value)}
          maxLength={1000}
          placeholder="Tulis pesan"
          aria-label="Pesan"
          className="min-w-0 flex-1 rounded-full border border-line bg-paper px-4 py-2.5 text-base focus:border-ink focus:outline-none"
        />
        <button type="submit" disabled={busy || !draft.trim()} aria-label="Kirim" className="press grid size-11 shrink-0 place-items-center rounded-full bg-track text-white disabled:opacity-40">
          <Icon name="send" className="size-5" />
        </button>
      </form>
    </div>
  );
}

/**
 * The inbox: list and conversation. Side by side when there is room
 * (`split`), otherwise one at a time with a back button.
 */
export function Inbox({ selected, onSelect, split }: { selected: number | null; onSelect: (id: number | null) => void; split: boolean }) {
  const { conversations } = useChats();
  const current = conversations?.find((c) => c.auction_id === selected);
  const list = <ConversationList selected={selected} onSelect={onSelect} />;
  if (!split) {
    return selected === null ? list : <ConversationView auctionId={selected} conversation={current} onBack={() => onSelect(null)} />;
  }
  return (
    <div className="grid min-h-0 flex-1 grid-cols-[18rem_1fr]">
      <div className="flex min-h-0 flex-col border-r border-line">{list}</div>
      {selected === null ? (
        <div className="grid place-items-center bg-paper p-6 text-center text-sm text-ink-soft">Pilih percakapan untuk mulai chat.</div>
      ) : (
        <ConversationView key={selected} auctionId={selected} conversation={current} />
      )}
    </div>
  );
}
