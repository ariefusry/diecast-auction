import { useCallback, useEffect, useRef, useState, type CSSProperties, type FormEvent } from "react";
import { Link, useLocation, useParams } from "react-router";
import { BrandLogo } from "../components/BrandLogo";
import { Photo } from "../components/Photo";
import { Button, buttonClass, Clock, ErrorNote } from "../components/ui";
import { api, errorMessage, type Auction, type BidEntry } from "../lib/api";
import { useAuth } from "../lib/auth";
import { useChats } from "../lib/chats";
import { cardBack } from "../lib/brands";
import { castingNumber, conditionLabel, endsAtWIB, minBid, quickBids, rupiah, timeAgo, timeLeft } from "../lib/format";
import { useNow } from "../lib/useNow";

const statusText: Record<Auction["status"], string> = {
  draft: "Draf, belum tayang",
  active: "Sedang berlangsung",
  completed: "Lelang selesai",
  cancelled: "Dibatalkan penjual",
};

function ChatButton({ label, unread, onClick }: { label: string; unread: number; onClick: () => void }) {
  return (
    <Button variant="ink" size="lg" icon="chat" className="w-full" onClick={onClick}>
      {label}
      {unread > 0 && <span className="grid h-5 min-w-5 place-items-center rounded-full bg-track px-1.5 text-xs font-bold tabular-nums">{unread}</span>}
    </Button>
  );
}

/** A short buzz on phones that support it; feedback for winning or losing the lead. */
const buzz = (pattern: number | number[]) => navigator.vibrate?.(pattern);

export function AuctionDetail() {
  const { id } = useParams();
  const here = useLocation().pathname;
  const { userId } = useAuth();
  const { openChat, conversations } = useChats();
  const now = useNow();
  const [auction, setAuction] = useState<Auction | null>(null);
  const [bids, setBids] = useState<BidEntry[]>([]);
  const [loadError, setLoadError] = useState("");
  // Bumped by every request; a response from an older request is dropped, so a
  // slow poll can never overwrite the result of a bid that finished after it.
  const seq = useRef(0);
  // The last auction state we showed, to notice what changed between polls.
  const last = useRef<Auction | null>(null);
  const [amount, setAmount] = useState("");
  const [error, setError] = useState("");
  const [shaking, setShaking] = useState(false);
  const [notice, setNotice] = useState("");
  // Which action is waiting on the server, so only its button shows a spinner.
  const [busy, setBusy] = useState<"bid" | "buy" | "publish" | "cancel" | null>(null);
  // Game feedback: our bid landed (sticker), someone took the lead from us,
  // anti-sniping added time. Counters remount the element so it animates again.
  const [stamps, setStamps] = useState(0);
  const [outbidBy, setOutbidBy] = useState<number | null>(null);
  const [bonuses, setBonuses] = useState(0);
  // Counts price changes after the first load, so a new bid (ours or someone
  // else's) animates in while the initial render stays still.
  const price = auction ? (auction.current_highest_bid ?? auction.start_price) : null;
  const [seenPrice, setSeenPrice] = useState<number | null>(null);
  const [bumps, setBumps] = useState(0);
  if (price !== seenPrice) {
    if (seenPrice !== null) setBumps(bumps + 1);
    setSeenPrice(price);
  }

  const show = useCallback(
    (fresh: Auction) => {
      const prev = last.current;
      last.current = fresh;
      setAuction(fresh);
      if (!prev) return;
      if (prev.leading && !fresh.leading && fresh.status === "active") {
        setOutbidBy(fresh.current_highest_bid);
        setNotice(""); // "your bid is placed" is no longer true
        buzz([40, 60, 40]);
      }
      if (prev.end_time && fresh.end_time && Date.parse(fresh.end_time) > Date.parse(prev.end_time)) setBonuses((n) => n + 1);
    },
    [],
  );

  const load = useCallback(async () => {
    const mine = ++seq.current;
    try {
      const [fresh, history] = await Promise.all([api<Auction>(`/auctions/${id}`), api<BidEntry[]>(`/auctions/${id}/bids`)]);
      if (mine !== seq.current) return;
      show(fresh);
      setBids(history);
    } catch (e) {
      // A failed poll keeps the last good data; only a failed first load shows.
      if (mine === seq.current) setLoadError(errorMessage(e));
    }
  }, [id, show]);

  // Poll so other people's bids and anti-sniping extensions show up.
  useEffect(() => {
    load();
    const t = setInterval(load, 5000);
    return () => clearInterval(t);
  }, [load]);

  if (!auction) {
    return loadError ? <ErrorNote message={loadError} /> : <p className="text-ink-soft">Memuat lelang…</p>;
  }

  const a = auction;
  const left = timeLeft(a.end_time, now);
  const open = a.status === "active" && left.ms > 0;
  const isSeller = userId === a.seller_id;
  const leading = a.leading;
  const won = a.status === "completed" && a.won;
  const floor = minBid(a);
  const canBid = open && !isSeller && userId !== null && !leading;
  const { no, name } = castingNumber(a.item.model_name);
  const bidValue = Number(amount || floor);
  // A bid at the buy-now price buys the lot (the server does the same), so the button says so.
  const buys = a.buy_now_price !== null && bidValue >= a.buy_now_price;
  const tooLow = amount !== "" && bidValue < floor && !buys;

  async function run(key: NonNullable<typeof busy>, action: () => Promise<Auction>, done: string, landed?: () => void) {
    setBusy(key);
    setError("");
    setNotice("");
    const mine = ++seq.current;
    try {
      const updated = await action();
      if (mine === seq.current) show(updated);
      setNotice(done);
      setAmount("");
      landed?.();
      load(); // refresh the history with our own row
    } catch (e) {
      setError(errorMessage(e));
      setShaking(true);
      load();
    } finally {
      setBusy(null);
    }
  }

  function placeBid(value: number) {
    const buying = a.buy_now_price !== null && value >= a.buy_now_price;
    const done = buying ? "Terbeli. Lelang selesai atas nama Anda." : `Tawaran ${rupiah(value)} terpasang.`;
    run("bid", () => api<Auction>(`/auctions/${a.id}/bids`, { method: "POST", body: { amount: value } }), done, () => {
      setStamps((n) => n + 1);
      setOutbidBy(null);
      buzz(20);
    });
  }

  function bid(e: FormEvent) {
    e.preventDefault();
    placeBid(bidValue);
  }

  const feedback = (
    <>
      <ErrorNote message={error} />
      {notice && (
        <p role="status" className="rounded-lg bg-lead/10 px-3 py-2 text-sm font-medium text-lead">
          {notice}
        </p>
      )}
    </>
  );

  return (
    <div className={`grid gap-8 lg:grid-cols-[1.1fr_1fr] lg:grid-rows-[auto_1fr] ${canBid ? "max-lg:pb-64" : userId === null && open ? "max-lg:pb-40" : ""}`}>
      <section className="blister relative self-start [view-transition-name:lot] lg:col-start-1 lg:row-start-1" style={{ "--back": cardBack(a.item) } as CSSProperties}>
        <Photo url={a.item.image_url} alt={a.item.model_name} />
        {no && (
          <span className="absolute top-12 right-6 grid size-16 rotate-6 place-items-center rounded-full border-[3px] border-white bg-[#e1001a] font-display text-3xl font-extrabold text-white shadow-md">
            {no}
          </span>
        )}
        {(leading && open) || won ? (
          // Slapped onto the pack when our bid lands; stays while we lead.
          <span
            key={stamps}
            className={`absolute bottom-10 left-6 -rotate-[8deg] rounded-xl border-[3px] border-white px-4 py-1.5 font-display text-3xl font-extrabold text-white italic shadow-lg ${won ? "bg-track" : "bg-lead"} ${stamps ? "stamp" : ""}`}
          >
            {won ? "MENANG!" : "MEMIMPIN!"}
          </span>
        ) : null}
      </section>

      <section className="space-y-5 lg:col-start-2 lg:row-span-2 lg:row-start-1">
        <div>
          <div className="flex flex-wrap items-center gap-x-3 gap-y-1 text-ink-soft">
            {a.item.category === "diorama" ? (
              <span className="rounded-lg bg-[#3f5e3a] px-2.5 py-1 font-bold text-white">Diorama</span>
            ) : (
              <BrandLogo brand={a.item.brand} className="h-7 max-w-32" />
            )}
            {a.item.artist && <span className="font-semibold text-ink">oleh {a.item.artist}</span>}
            {(a.item.series || a.item.year > 0) && (
              <span>
                {a.item.series}
                {a.item.series && a.item.year > 0 && ", "}
                {a.item.year > 0 && a.item.year}
              </span>
            )}
          </div>
          <h1 className="font-display text-4xl leading-tight font-extrabold italic">{name}</h1>
          <div className="mt-2 flex flex-wrap gap-2 text-sm">
            <span className="rounded-full bg-card px-3 py-1 font-semibold">{conditionLabel(a.item.condition)}</span>
            {a.item.size && <span className="rounded-full bg-card px-3 py-1">{a.item.size}</span>}
            {a.item.ready_weeks ? (
              <span className="rounded-full bg-ink px-3 py-1 font-semibold text-white">PO, selesai ±{a.item.ready_weeks} minggu</span>
            ) : null}
            {a.item.color && <span className="rounded-full bg-card px-3 py-1">{a.item.color}</span>}
            {a.item.tags.map((t) => (
              <span key={t} className="rounded-full bg-card px-3 py-1">
                {t}
              </span>
            ))}
          </div>
          <p className="mt-3 text-sm text-ink-soft">
            Dijual oleh{" "}
            <Link to={`/penjual/${a.seller}`} className="font-semibold text-ink underline underline-offset-4">
              {a.seller}
            </Link>
          </p>
        </div>

        <div className="rounded-xl border border-line bg-card p-5">
          <div className="flex flex-wrap items-end justify-between gap-4">
            <div>
              <div className="text-sm text-ink-soft">{a.current_highest_bid === null ? "Harga awal" : "Tawaran tertinggi"}</div>
              <div key={bumps} className={`font-display text-4xl font-bold tabular-nums ${bumps ? "land" : ""}`}>
                {rupiah(a.current_highest_bid ?? a.start_price)}
              </div>
            </div>
            <div className="relative sm:text-right">
              <div className="text-sm text-ink-soft">{open ? "Sisa waktu" : statusText[a.status]}</div>
              <div
                className={`origin-right font-display text-4xl font-bold tabular-nums ${left.closing && open ? "tick text-signal" : ""}`}
                aria-live="polite"
              >
                {open ? <Clock label={left.label} closing={left.closing} /> : a.status === "active" ? "Menunggu penutupan" : ""}
              </div>
              {bonuses > 0 && (
                <span key={bonuses} className="bonus pointer-events-none absolute -top-3 right-0 font-display text-2xl font-extrabold text-lead">
                  +2:00
                </span>
              )}
            </div>
          </div>
          {a.end_time && (a.status === "active" || a.status === "completed") && (
            // An absolute end in the market's zone; anti-sniping moves it, and this line follows.
            <p className="mt-3 text-sm text-ink-soft">
              {open ? "Berakhir" : "Berakhir pada"} <span className="font-semibold text-ink">{endsAtWIB(a.end_time)}</span>
            </p>
          )}
          {left.closing && open && (
            <p className="mt-1 text-sm font-medium text-signal">Detik terakhir: setiap tawaran menambah waktu 2 menit.</p>
          )}
          {a.status === "completed" && (
            <p className="mt-3 font-semibold">
              {!a.sold ? "Selesai tanpa pemenang." : won ? "Anda memenangkan lelang ini." : "Sudah terjual."}
            </p>
          )}
        </div>

        {!canBid && feedback}

        {canBid && (
          <form
            onSubmit={bid}
            onAnimationEnd={() => setShaking(false)}
            className={`dock space-y-3 lg:space-y-3 ${shaking ? "refuse" : ""}`}
            aria-label="Pasang tawaran"
          >
            {outbidBy !== null && (
              <div role="alert" className="refuse flex items-center justify-between gap-3 rounded-xl bg-signal px-4 py-2.5 text-white">
                <span className="font-display text-xl font-extrabold italic">Kamu disalip! {rupiah(a.current_highest_bid ?? outbidBy)}</span>
                <button type="button" disabled={busy !== null} onClick={() => placeBid(floor)} className="press shrink-0 rounded-lg bg-white px-3 py-1.5 font-bold text-signal">
                  Balas {rupiah(floor)}
                </button>
              </div>
            )}
            {feedback}
            <div className="flex items-baseline justify-between gap-2 lg:hidden">
              <span className="text-sm text-ink-soft">
                Tertinggi <b className="font-display text-lg text-ink tabular-nums">{rupiah(a.current_highest_bid ?? a.start_price)}</b>
              </span>
              <Clock label={left.label} closing={left.closing} className={`font-display text-lg font-bold tabular-nums ${left.closing ? "text-signal" : ""}`} />
            </div>
            <div className="flex gap-2">
              {quickBids(a).map((v) => (
                <button
                  key={v}
                  type="button"
                  onClick={() => setAmount(String(v))}
                  aria-pressed={amount === String(v)}
                  className={`press min-h-9 flex-1 rounded-full border px-2 text-sm font-semibold tabular-nums ${(amount || String(floor)) === String(v) ? "border-ink bg-ink text-white" : "border-line bg-card hover:border-ink"}`}
                >
                  {rupiah(v)}
                </button>
              ))}
            </div>
            <label className="block">
              <span className="sr-only">Jumlah lain</span>
              <div className="flex gap-2">
                <input
                  type="number"
                  inputMode="numeric"
                  min={floor}
                  step="any"
                  value={amount}
                  onChange={(e) => setAmount(e.target.value)}
                  placeholder="Jumlah lain"
                  className="w-0 min-w-0 flex-[2] rounded-xl border-2 border-line bg-card px-3 font-display text-xl font-bold tabular-nums placeholder:font-sans placeholder:text-base placeholder:font-medium focus:border-ink"
                />
                {/* The button names the exact amount it will place, so a tap never surprises. */}
                <Button type="submit" variant="track" size="lg" icon={buys ? "zap" : "gavel"} loading={busy === "bid"} disabled={tooLow} className="flex-[3] whitespace-nowrap tabular-nums max-sm:px-3 max-sm:text-lg">
                  {buys ? `Beli ${rupiah(a.buy_now_price!)}` : `Tawar ${rupiah(tooLow ? floor : bidValue)}`}
                </Button>
              </div>
              {tooLow ? (
                <span className="mt-1 block text-sm font-medium text-signal">Kurang dari minimal {rupiah(floor)}.</span>
              ) : (
                <span className="mt-1 block text-sm text-ink-soft max-lg:hidden">
                  Minimal {rupiah(floor)}. Dana Anda terkunci selama memimpin dan kembali begitu ada tawaran lebih tinggi.
                </span>
              )}
            </label>
          </form>
        )}

        {/* Open while the highest bid is under the buy-now price; the leader may buy out their own lead. */}
        {open && !isSeller && userId !== null && a.buy_now_price !== null && (a.current_highest_bid ?? 0) < a.buy_now_price && (
          <Button
            variant="ghost"
            icon="zap"
            loading={busy === "buy"}
            disabled={busy !== null}
            className="w-full"
            onClick={() => run("buy", () => api<Auction>(`/auctions/${a.id}/buy-now`, { method: "POST" }), "Terbeli. Lelang selesai atas nama Anda.")}
          >
            Beli langsung {rupiah(a.buy_now_price)}
          </Button>
        )}

        {userId === null && open && (
          // Bidding is the point of this page: logged out, the way in gets the bid form's weight and dock.
          <div className="dock space-y-2 lg:rounded-xl lg:border lg:border-line lg:bg-card lg:p-5">
            <p className="font-semibold max-lg:hidden">Mau menawar lot ini?</p>
            <Link to="/masuk" state={{ from: here }} className={buttonClass({ variant: "track", size: "lg", className: "w-full" })}>
              Masuk untuk menawar
            </Link>
            <p className="text-center text-sm text-ink-soft">
              Belum punya akun?{" "}
              <Link to="/daftar" state={{ from: here }} className="font-semibold text-ink underline underline-offset-4">
                Daftar gratis
              </Link>
            </p>
          </div>
        )}

        {isSeller && (
          <div className="flex flex-wrap gap-2">
            {a.status === "draft" && (
              <Button loading={busy === "publish"} disabled={busy !== null} onClick={() => run("publish", () => api<Auction>(`/auctions/${a.id}/publish`, { method: "POST" }), "Lelang ditayangkan.")}>
                Tayangkan lelang
              </Button>
            )}
            {(a.status === "draft" || open) && a.current_highest_bid === null && (
              <Button variant="ghost" loading={busy === "cancel"} disabled={busy !== null} onClick={() => run("cancel", () => api<Auction>(`/auctions/${a.id}/cancel`, { method: "POST" }), "Lelang dibatalkan.")}>
                Batalkan lelang
              </Button>
            )}
            {a.current_highest_bid !== null && <p className="text-sm text-ink-soft">Lelang yang sudah punya tawaran tidak bisa dibatalkan.</p>}
          </div>
        )}

        {a.status === "completed" && a.sold && userId !== null && (isSeller || won) && (
          // Shipping is arranged in the inbox, where every chat lives; this opens this lot's.
          <ChatButton unread={conversations?.find((c) => c.auction_id === a.id)?.unread ?? 0} label={isSeller ? "Chat pemenang, atur pengiriman" : "Chat penjual, atur pengiriman"} onClick={() => openChat(a.id)} />
        )}

        {a.item.description && <p className="max-w-prose whitespace-pre-line">{a.item.description}</p>}
        {/^https?:\/\//.test(a.item.source_url) && (
          <p className="text-sm text-ink-soft">
            Foto dan data dari{" "}
            <a href={a.item.source_url} target="_blank" rel="noreferrer" className="underline underline-offset-4">
              wiki Fandom
            </a>{" "}
            (CC-BY-SA).
          </p>
        )}
      </section>

      <section className="space-y-3 lg:col-start-1 lg:row-start-2">
        <h2 className="font-display text-2xl font-bold italic">
          Riwayat tawaran <span className="text-ink-soft">{bids.length > 0 && bids.length}</span>
        </h2>
        {bids.length === 0 ? (
          <p className="rounded-xl border border-dashed border-line px-4 py-6 text-center text-ink-soft">
            {open ? "Belum ada tawaran. Pasang yang pertama." : "Tidak ada tawaran."}
          </p>
        ) : (
          <ol className="divide-y divide-line overflow-hidden rounded-xl border border-line bg-card">
            {bids.map((b, i) => {
              const mine = b.mine;
              return (
                <li key={b.amount} className={`flex items-center gap-3 px-4 py-3 ${i === 0 && bumps ? "land" : ""} ${i === 0 ? "bg-lead/8" : ""}`}>
                  <span
                    aria-hidden
                    className={`grid size-9 shrink-0 place-items-center rounded-full font-display text-lg font-bold uppercase ${mine ? "bg-track text-white" : "bg-paper text-ink"}`}
                  >
                    {b.bidder.charAt(0)}
                  </span>
                  <div className="min-w-0 flex-1">
                    <div className="font-semibold">
                      {mine ? "Anda" : b.bidder}
                      {i === 0 && <span className="ml-2 rounded-full bg-lead px-2 py-0.5 text-xs font-bold text-white">Tertinggi</span>}
                    </div>
                    <div className="text-sm text-ink-soft">{timeAgo(b.created_at, now)}</div>
                  </div>
                  <span className={`font-display text-xl font-bold tabular-nums ${i === 0 ? "" : "text-ink-soft"}`}>{rupiah(b.amount)}</span>
                </li>
              );
            })}
          </ol>
        )}
      </section>
    </div>
  );
}
