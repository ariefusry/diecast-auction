import { useCallback, useEffect, useRef, useState, type FormEvent } from "react";
import { Link, useParams } from "react-router";
import { Photo } from "../components/Photo";
import { Button, ErrorNote } from "../components/ui";
import { api, errorMessage, type Auction } from "../lib/api";
import { useAuth } from "../lib/auth";
import { conditionLabel, minBid, rupiah, timeLeft } from "../lib/format";
import { useNow } from "../lib/useNow";

const statusText: Record<Auction["status"], string> = {
  draft: "Draf, belum tayang",
  active: "Sedang berlangsung",
  completed: "Lelang selesai",
  cancelled: "Dibatalkan penjual",
};

export function AuctionDetail() {
  const { id } = useParams();
  const { userId } = useAuth();
  const now = useNow();
  const [auction, setAuction] = useState<Auction | null>(null);
  const [loadError, setLoadError] = useState("");
  // Bumped by every request; a response from an older request is dropped, so a
  // slow poll can never overwrite the result of a bid that finished after it.
  const seq = useRef(0);
  const [amount, setAmount] = useState("");
  const [error, setError] = useState("");
  const [shaking, setShaking] = useState(false);
  const [notice, setNotice] = useState("");
  const [busy, setBusy] = useState(false);
  // Counts price changes after the first load, so a new bid (ours or someone
  // else's) animates in while the initial render stays still.
  const price = auction ? (auction.current_highest_bid ?? auction.start_price) : null;
  const [seenPrice, setSeenPrice] = useState<number | null>(null);
  const [bumps, setBumps] = useState(0);
  if (price !== seenPrice) {
    if (seenPrice !== null) setBumps(bumps + 1);
    setSeenPrice(price);
  }

  const load = useCallback(async () => {
    const mine = ++seq.current;
    try {
      const fresh = await api<Auction>(`/auctions/${id}`);
      if (mine === seq.current) setAuction(fresh);
    } catch (e) {
      // A failed poll keeps the last good data; only a failed first load shows.
      if (mine === seq.current) setLoadError(errorMessage(e));
    }
  }, [id]);

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
  const leading = userId !== null && a.highest_bidder_id === userId;
  const floor = minBid(a);

  async function run(action: () => Promise<Auction>, done: string) {
    setBusy(true);
    setError("");
    setNotice("");
    const mine = ++seq.current;
    try {
      const updated = await action();
      if (mine === seq.current) setAuction(updated);
      setNotice(done);
      setAmount("");
    } catch (e) {
      setError(errorMessage(e));
      setShaking(true);
      load();
    } finally {
      setBusy(false);
    }
  }

  function bid(e: FormEvent) {
    e.preventDefault();
    const value = Number(amount || floor);
    run(() => api<Auction>(`/auctions/${a.id}/bids`, { method: "POST", body: { amount: value } }), `Tawaran ${rupiah(value)} terpasang.`);
  }

  return (
    <div className="grid gap-8 lg:grid-cols-[1.1fr_1fr]">
      <section className="blister self-start">
        <Photo url={a.item.image_url} alt={a.item.model_name} />
      </section>

      <section className="space-y-5">
        <div>
          <div className="text-ink-soft">
            {a.item.brand}
            {a.item.series && `, ${a.item.series}`}
            {a.item.year > 0 && ` (${a.item.year})`}
          </div>
          <h1 className="font-display text-4xl leading-tight font-extrabold italic">{a.item.model_name}</h1>
          <div className="mt-2 flex flex-wrap gap-2 text-sm">
            <span className="rounded-full bg-card px-3 py-1 font-semibold">{conditionLabel(a.item.condition)}</span>
            {a.item.color && <span className="rounded-full bg-card px-3 py-1">{a.item.color}</span>}
            {a.item.tags.map((t) => (
              <span key={t} className="rounded-full bg-card px-3 py-1">
                {t}
              </span>
            ))}
          </div>
        </div>

        <div className="rounded-xl border border-line bg-card p-5">
          <div className="flex flex-wrap items-end justify-between gap-4">
            <div>
              <div className="text-sm text-ink-soft">{a.current_highest_bid === null ? "Harga awal" : "Tawaran tertinggi"}</div>
              <div key={bumps} className={`font-display text-4xl font-bold tabular-nums ${bumps ? "land" : ""}`}>
                {rupiah(a.current_highest_bid ?? a.start_price)}
              </div>
            </div>
            <div className="sm:text-right">
              <div className="text-sm text-ink-soft">{open ? "Sisa waktu" : statusText[a.status]}</div>
              <div className={`font-display text-4xl font-bold tabular-nums ${left.closing ? "text-signal" : ""}`} aria-live="polite">
                {open ? left.label : a.status === "active" ? "Menunggu penutupan" : ""}
              </div>
            </div>
          </div>
          {left.closing && open && (
            <p className="mt-3 text-sm font-medium text-signal">Dua menit terakhir: setiap tawaran menambah waktu 2 menit.</p>
          )}
          {leading && open && <p className="mt-3 rounded-md bg-lead/10 px-3 py-2 font-semibold text-lead">Anda memimpin lelang ini.</p>}
          {a.status === "completed" && (
            <p className="mt-3 font-semibold">
              {a.winner_id === null ? "Selesai tanpa pemenang." : a.winner_id === userId ? "Anda memenangkan lelang ini." : "Sudah terjual."}
            </p>
          )}
        </div>

        <ErrorNote message={error} />
        {notice && (
          <p role="status" className="rounded-lg bg-lead/10 px-3 py-2 text-sm font-medium text-lead">
            {notice}
          </p>
        )}

        {open && !isSeller && userId !== null && !leading && (
          <form onSubmit={bid} onAnimationEnd={() => setShaking(false)} className={`space-y-3 ${shaking ? "refuse" : ""}`}>
            <label className="block">
              <span className="mb-1 block font-semibold">Tawaran Anda</span>
              <div className="flex gap-2">
                <input
                  type="number"
                  inputMode="numeric"
                  min={floor}
                  step="any"
                  value={amount}
                  onChange={(e) => setAmount(e.target.value)}
                  placeholder={String(floor)}
                  className="min-w-0 flex-1 rounded-lg border-2 border-ink bg-card px-3 py-2.5 font-display text-2xl font-bold tabular-nums focus:outline-none"
                />
                <Button type="submit" variant="track" disabled={busy} className="px-6 text-lg">
                  {busy ? "Memasang…" : "Pasang tawaran"}
                </Button>
              </div>
              {amount !== "" && Number(amount) < floor ? (
                <span className="mt-1 block text-sm font-medium text-signal">Kurang dari minimal {rupiah(floor)}.</span>
              ) : (
                <span className="mt-1 block text-sm text-ink-soft">
                  Minimal {rupiah(floor)}. Dana Anda terkunci selama memimpin dan kembali begitu ada tawaran lebih tinggi.
                </span>
              )}
            </label>
          </form>
        )}

        {open && !isSeller && userId !== null && a.buy_now_price !== null && a.highest_bidder_id === null && (
          <Button
            variant="ghost"
            disabled={busy}
            className="w-full"
            onClick={() => run(() => api<Auction>(`/auctions/${a.id}/buy-now`, { method: "POST" }), "Terbeli. Lelang selesai atas nama Anda.")}
          >
            Beli langsung {rupiah(a.buy_now_price)}
          </Button>
        )}

        {userId === null && open && (
          <p>
            <Link to="/masuk" className="font-semibold underline underline-offset-4">
              Masuk
            </Link>{" "}
            untuk menawar.
          </p>
        )}

        {isSeller && (
          <div className="flex flex-wrap gap-2">
            {a.status === "draft" && (
              <Button disabled={busy} onClick={() => run(() => api<Auction>(`/auctions/${a.id}/publish`, { method: "POST" }), "Lelang ditayangkan.")}>
                Tayangkan lelang
              </Button>
            )}
            {(a.status === "draft" || a.status === "active") && a.highest_bidder_id === null && (
              <Button variant="ghost" disabled={busy} onClick={() => run(() => api<Auction>(`/auctions/${a.id}/cancel`, { method: "POST" }), "Lelang dibatalkan.")}>
                Batalkan lelang
              </Button>
            )}
            {a.highest_bidder_id !== null && <p className="text-sm text-ink-soft">Lelang yang sudah punya tawaran tidak bisa dibatalkan.</p>}
          </div>
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
    </div>
  );
}
