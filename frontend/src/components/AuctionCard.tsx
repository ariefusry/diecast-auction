import { Link } from "react-router";
import type { Auction } from "../lib/api";
import { conditionLabel, rupiah, timeLeft } from "../lib/format";
import { Photo } from "./Photo";

export function AuctionCard({ auction: a, now, myId }: { auction: Auction; now: number; myId: number | null }) {
  const left = timeLeft(a.end_time, now);
  const price = a.current_highest_bid ?? a.start_price;
  const leading = myId !== null && a.highest_bidder_id === myId;
  return (
    <Link to={`/lelang/${a.id}`} className="blister group flex flex-col gap-3 hover:border-ink">
      <Photo url={a.item.image_url} alt={a.item.model_name} />
      <div className="flex items-center justify-between gap-2 text-sm">
        <span className="font-semibold text-ink-soft">{a.item.brand}</span>
        <span className="text-ink-soft">{conditionLabel(a.item.condition)}</span>
      </div>
      <h3 className="font-display text-xl leading-tight font-bold group-hover:underline">{a.item.model_name}</h3>
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
    </Link>
  );
}
