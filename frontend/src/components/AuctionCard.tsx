import type { CSSProperties } from "react";
import { Link } from "react-router";
import type { Auction } from "../lib/api";
import { cardBack } from "../lib/brands";
import { castingNumber, conditionLabel, rupiah, timeLeft } from "../lib/format";
import { BrandLogo } from "./BrandLogo";
import { Photo } from "./Photo";

/** One lot as a blister pack: brand-colored card back, photo in the bubble,
 *  and a white price label stuck on the bottom. */
export function AuctionCard({ auction: a, now, className = "" }: { auction: Auction; now: number; className?: string }) {
  const left = timeLeft(a.end_time, now);
  const price = a.current_highest_bid ?? a.start_price;
  const leading = a.leading && a.status === "active";
  // Finished lots say how they ended instead of who leads.
  const outcome =
    a.status === "completed" ? (a.won ? "Anda menang" : a.sold ? "Terjual" : "Tidak terjual") : a.status === "cancelled" ? "Dibatalkan" : null;
  const { no, name } = castingNumber(a.item.model_name);
  const live = a.status === "active" && left.ms > 0;
  const closing = live && left.closing;
  // Buy-now only works until the first bid.
  const buyNow = live && a.buy_now_price !== null && a.current_highest_bid === null ? a.buy_now_price : null;
  return (
    <Link
      to={`/lelang/${a.id}`}
      viewTransition
      // Only the clicked pack gets the shared name: the same lot can hang on the
      // pegboard and in the grid at once, and two names would cancel the morph.
      onClick={(e) => e.currentTarget.style.setProperty("view-transition-name", "lot")}
      style={{ "--back": cardBack(a.item) } as CSSProperties}
      // A container: the pack reads its own width, so the same card is compact in a
      // two-up phone grid or the pegboard strip and roomy in a wide desktop grid.
      className={`blister press @container flex flex-col gap-2 [--press:0.985] ${className}`}
    >
      <div className="relative">
        <Photo url={a.item.image_url} alt={a.item.model_name} />
        {no && (
          // Tomica's red number sticker, top corner of the pack
          <span className="absolute top-1.5 right-1.5 grid size-8 rotate-6 place-items-center rounded-full border-2 border-white bg-[#e1001a] font-display text-sm leading-none font-extrabold text-white shadow @min-[14rem]:top-2 @min-[14rem]:right-2 @min-[14rem]:size-11 @min-[14rem]:text-lg">
            {no}
          </span>
        )}
        {closing && (
          <span className="absolute top-1.5 left-1.5 rounded-full bg-signal px-2 py-0.5 text-[11px] font-bold text-white shadow @min-[14rem]:top-2 @min-[14rem]:left-2 @min-[14rem]:px-2.5 @min-[14rem]:py-1 @min-[14rem]:text-xs">
            Detik terakhir
          </span>
        )}
      </div>
      <div className="flex flex-1 flex-col gap-1 rounded-xl bg-card p-2.5 @min-[14rem]:gap-2 @min-[14rem]:p-3">
        {/* Wraps rather than truncates: a cut-off condition ("Box sege…") reads as a typo. */}
        <div className="flex flex-wrap items-center justify-between gap-x-2 gap-y-0.5 text-xs @min-[14rem]:text-sm">
          {a.item.category === "diorama" ? (
            <span className="rounded-md bg-[#3f5e3a] px-1.5 py-0.5 text-[11px] font-bold text-white @min-[14rem]:px-2 @min-[14rem]:text-xs">Diorama</span>
          ) : (
            <BrandLogo brand={a.item.brand} className="h-4 max-w-20 @min-[14rem]:h-5 @min-[14rem]:max-w-24" />
          )}
          <span className="text-ink-soft">{conditionLabel(a.item.condition)}</span>
        </div>
        <h3 className="line-clamp-2 font-display text-base leading-tight font-bold @min-[14rem]:text-xl">{name}</h3>
        {(a.item.artist || a.item.ready_weeks) && (
          <div className="flex flex-wrap items-center gap-x-2 gap-y-1 text-xs text-ink-soft @min-[14rem]:text-sm">
            {a.item.artist && <span className="truncate">oleh {a.item.artist}</span>}
            {a.item.ready_weeks ? <span className="rounded-md bg-ink px-1.5 py-0.5 text-[11px] font-bold text-white @min-[14rem]:px-2 @min-[14rem]:text-xs">PO ±{a.item.ready_weeks} minggu</span> : null}
          </div>
        )}
        {buyNow !== null && (
          <span className="self-start rounded-md bg-track/12 px-1.5 py-0.5 text-xs font-semibold text-track-dark @min-[14rem]:px-2 @min-[14rem]:text-sm">
            Beli <span className="hidden @min-[14rem]:inline">langsung </span>
            {rupiah(buyNow)}
          </span>
        )}
        {/* Narrow packs stack price over time; wide ones put them side by side. */}
        <div className="mt-auto flex flex-col pt-1 @min-[14rem]:flex-row @min-[14rem]:items-end @min-[14rem]:justify-between @min-[14rem]:gap-2">
          <div>
            <div className={`text-xs @min-[14rem]:text-sm ${leading || a.won ? "font-semibold text-lead" : "text-ink-soft"}`}>
              {outcome ?? (leading ? "Anda memimpin" : a.current_highest_bid === null ? "Harga awal" : "Tawaran tertinggi")}
            </div>
            <div className="font-display text-lg leading-tight font-bold tabular-nums @min-[14rem]:text-2xl">{rupiah(price)}</div>
          </div>
          <div className={`font-display text-sm font-bold tabular-nums @min-[14rem]:text-right @min-[14rem]:text-lg ${closing ? "text-signal" : "text-ink-soft @min-[14rem]:text-ink"}`}>
            {left.label}
          </div>
        </div>
      </div>
    </Link>
  );
}
