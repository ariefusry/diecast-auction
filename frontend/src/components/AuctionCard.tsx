import type { CSSProperties } from "react";
import { Link } from "react-router";
import type { Auction } from "../lib/api";
import { brandBack } from "../lib/brands";
import { conditionLabel, rupiah, timeLeft } from "../lib/format";
import { BrandLogo } from "./BrandLogo";
import { Photo } from "./Photo";

/** One lot as a blister pack: brand-colored card back, photo in the bubble,
 *  and a white price label stuck on the bottom. */
export function AuctionCard({ auction: a, now, myId, className = "" }: { auction: Auction; now: number; myId: number | null; className?: string }) {
  const left = timeLeft(a.end_time, now);
  const price = a.current_highest_bid ?? a.start_price;
  const leading = myId !== null && a.highest_bidder_id === myId;
  return (
    <Link
      to={`/lelang/${a.id}`}
      style={{ "--back": brandBack[a.item.brand] } as CSSProperties}
      className={`blister press flex flex-col gap-3 [--press:0.985] ${className}`}
    >
      <Photo url={a.item.image_url} alt={a.item.model_name} />
      <div className="flex flex-1 flex-col gap-2 rounded-xl bg-card p-3">
        <div className="flex items-center justify-between gap-2 text-sm">
          <BrandLogo brand={a.item.brand} className="h-5 max-w-24" />
          <span className="text-ink-soft">{conditionLabel(a.item.condition)}</span>
        </div>
        <h3 className="font-display text-xl leading-tight font-bold">{a.item.model_name}</h3>
        <div className="mt-auto flex items-end justify-between gap-2">
          <div>
            <div className={`text-sm ${leading ? "font-semibold text-lead" : "text-ink-soft"}`}>
              {leading ? "Anda memimpin" : a.current_highest_bid === null ? "Harga awal" : "Tawaran tertinggi"}
            </div>
            <div className="font-display text-2xl font-bold tabular-nums">{rupiah(price)}</div>
          </div>
          <div className={`text-right font-display text-lg font-bold tabular-nums ${left.closing ? "text-signal" : ""}`}>
            {left.label}
          </div>
        </div>
      </div>
    </Link>
  );
}
