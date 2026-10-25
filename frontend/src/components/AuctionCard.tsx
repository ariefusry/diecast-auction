import type { CSSProperties } from "react";
import { Link } from "react-router";
import type { Auction } from "../lib/api";
import { cardBack } from "../lib/brands";
import { castingNumber, conditionLabel, rupiah, timeLeft } from "../lib/format";
import { BrandLogo } from "./BrandLogo";
import { Photo } from "./Photo";

/** One lot as a blister pack: brand-colored card back, photo in the bubble,
 *  and a white price label stuck on the bottom. */
export function AuctionCard({ auction: a, now, myId, className = "" }: { auction: Auction; now: number; myId: number | null; className?: string }) {
  const left = timeLeft(a.end_time, now);
  const price = a.current_highest_bid ?? a.start_price;
  const leading = myId !== null && a.highest_bidder_id === myId && a.status === "active";
  const won = myId !== null && a.winner_id === myId;
  // Finished lots say how they ended instead of who leads.
  const outcome =
    a.status === "completed" ? (won ? "Anda menang" : a.winner_id === null ? "Tidak terjual" : "Terjual") : a.status === "cancelled" ? "Dibatalkan" : null;
  const { no, name } = castingNumber(a.item.model_name);
  const live = a.status === "active" && left.ms > 0;
  const closing = live && left.closing;
  // Buy-now only works until the first bid.
  const buyNow = live && a.buy_now_price !== null && a.highest_bidder_id === null ? a.buy_now_price : null;
  return (
    <Link
      to={`/lelang/${a.id}`}
      style={{ "--back": cardBack(a.item) } as CSSProperties}
      className={`blister press flex flex-col gap-3 [--press:0.985] ${className}`}
    >
      <div className="relative">
        <Photo url={a.item.image_url} alt={a.item.model_name} />
        {no && (
          // Tomica's red number sticker, top corner of the pack
          <span className="absolute top-2 right-2 grid size-11 rotate-6 place-items-center rounded-full border-2 border-white bg-[#e1001a] font-display text-lg leading-none font-extrabold text-white shadow">
            {no}
          </span>
        )}
        {closing && (
          <span className="absolute top-2 left-2 rounded-full bg-signal px-2.5 py-1 text-xs font-bold text-white shadow">Detik terakhir</span>
        )}
      </div>
      <div className="flex flex-1 flex-col gap-2 rounded-xl bg-card p-3">
        <div className="flex items-center justify-between gap-2 text-sm">
          {a.item.category === "diorama" ? (
            <span className="rounded-md bg-[#3f5e3a] px-2 py-0.5 text-xs font-bold text-white">Diorama</span>
          ) : (
            <BrandLogo brand={a.item.brand} className="h-5 max-w-24" />
          )}
          <span className="text-ink-soft">{conditionLabel(a.item.condition)}</span>
        </div>
        <h3 className="font-display text-xl leading-tight font-bold">{name}</h3>
        {(a.item.artist || a.item.ready_weeks) && (
          <div className="flex flex-wrap items-center gap-2 text-sm text-ink-soft">
            {a.item.artist && <span>oleh {a.item.artist}</span>}
            {a.item.ready_weeks ? <span className="rounded-md bg-ink px-2 py-0.5 text-xs font-bold text-white">PO ±{a.item.ready_weeks} minggu</span> : null}
          </div>
        )}
        {buyNow !== null && (
          <span className="self-start rounded-md bg-track/12 px-2 py-0.5 text-sm font-semibold text-track-dark">Beli langsung {rupiah(buyNow)}</span>
        )}
        <div className="mt-auto flex items-end justify-between gap-2">
          <div>
            <div className={`text-sm ${leading || won ? "font-semibold text-lead" : "text-ink-soft"}`}>
              {outcome ?? (leading ? "Anda memimpin" : a.current_highest_bid === null ? "Harga awal" : "Tawaran tertinggi")}
            </div>
            <div className="font-display text-2xl font-bold tabular-nums">{rupiah(price)}</div>
          </div>
          <div className={`text-right font-display text-lg font-bold tabular-nums ${closing ? "text-signal" : ""}`}>
            {left.label}
          </div>
        </div>
      </div>
    </Link>
  );
}
