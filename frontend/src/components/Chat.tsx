import { useCallback, useEffect, useRef, useState, type FormEvent } from "react";
import { api, errorMessage, type Message } from "../lib/api";
import { timeAgo } from "../lib/format";
import { Button, ErrorNote } from "./ui";

/** Seller and winner arrange shipping here once the auction is won. Polls like the auction page. */
export function Chat({ auctionId, myId, otherName, now }: { auctionId: number; myId: number; otherName: string; now: number }) {
  const [messages, setMessages] = useState<Message[]>([]);
  const [draft, setDraft] = useState("");
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");
  const end = useRef<HTMLDivElement>(null);

  const load = useCallback(() => api<Message[]>(`/auctions/${auctionId}/messages`).then(setMessages, () => undefined), [auctionId]);

  useEffect(() => {
    load();
    const t = setInterval(load, 5000);
    return () => clearInterval(t);
  }, [load]);

  // Keep the newest message in view as the conversation grows.
  const count = messages.length;
  useEffect(() => {
    end.current?.scrollIntoView({ block: "nearest" });
  }, [count]);

  async function send(e: FormEvent) {
    e.preventDefault();
    if (!draft.trim()) return;
    setBusy(true);
    setError("");
    try {
      const m = await api<Message>(`/auctions/${auctionId}/messages`, { method: "POST", body: { body: draft } });
      setMessages((ms) => [...ms, m]);
      setDraft("");
    } catch (err) {
      setError(errorMessage(err));
    } finally {
      setBusy(false);
    }
  }

  return (
    <section id="chat" className="space-y-3 rounded-xl border border-line bg-card p-4">
      <div>
        <h2 className="font-display text-2xl font-bold italic">Atur pengiriman</h2>
        <p className="text-sm text-ink-soft">Chat dengan {otherName}. Sepakati alamat, kurir dan nomor resi di sini.</p>
      </div>
      <div className="max-h-80 space-y-2 overflow-y-auto rounded-lg bg-paper p-3" aria-live="polite">
        {messages.length === 0 && <p className="py-6 text-center text-sm text-ink-soft">Belum ada pesan. Mulai dengan alamat pengiriman.</p>}
        {messages.map((m) => {
          const mine = m.sender_id === myId;
          return (
            <div key={m.id} className={`flex ${mine ? "justify-end" : "justify-start"}`}>
              <div className={`max-w-[80%] rounded-2xl px-3.5 py-2 ${mine ? "rounded-br-sm bg-ink text-white" : "rounded-bl-sm bg-card"}`}>
                <p className="break-words whitespace-pre-line">{m.body}</p>
                <p className={`mt-0.5 text-xs ${mine ? "text-white/60" : "text-ink-soft"}`}>
                  {mine ? "Anda" : m.sender}, {timeAgo(m.created_at, now)}
                </p>
              </div>
            </div>
          );
        })}
        <div ref={end} />
      </div>
      <ErrorNote message={error} />
      <form onSubmit={send} className="flex gap-2">
        <input
          value={draft}
          onChange={(e) => setDraft(e.target.value)}
          maxLength={1000}
          placeholder="Tulis pesan"
          aria-label="Pesan"
          className="min-w-0 flex-1 rounded-lg border border-line bg-card px-3 py-2.5 focus:border-ink focus:outline-none"
        />
        <Button type="submit" disabled={busy || !draft.trim()}>
          Kirim
        </Button>
      </form>
    </section>
  );
}
