import type { Auction } from "../lib/api";
import { AuctionCard } from "./AuctionCard";

// Slight, fixed tilts: cards on a real peg never hang perfectly straight.
const tilts = ["-rotate-1", "rotate-[0.6deg]", "-rotate-[0.4deg]", "rotate-1"];

/** The lots closing soonest, hanging from chrome pegs. On phones a strip of small packs that scrolls sideways. */
export function PegWall({ auctions, now }: { auctions: Auction[]; now: number }) {
  if (auctions.length === 0) return null;
  return (
    <div className="-mx-4 flex snap-x snap-mandatory gap-3 overflow-x-auto px-4 [scrollbar-width:none] pb-2 sm:-mx-8 sm:gap-4 sm:px-8 lg:mx-0 lg:grid lg:grid-cols-4 lg:overflow-visible lg:px-0">
      {auctions.map((a, i) => (
        <div key={a.id} className="relative w-44 shrink-0 snap-start pt-4 sm:w-56 lg:w-auto">
          <span className="peg" aria-hidden />
          <AuctionCard auction={a} now={now} className={`h-full origin-top shadow-[0_14px_22px_-12px_rgb(60_40_20/0.55)] ${tilts[i % tilts.length]}`} />
        </div>
      ))}
    </div>
  );
}
