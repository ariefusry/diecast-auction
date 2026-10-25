import { useEffect, useState } from "react";
import { useParams } from "react-router";
import { AuctionCard } from "../components/AuctionCard";
import { ErrorNote } from "../components/ui";
import { api, errorMessage, type SellerProfile } from "../lib/api";
import { useNow } from "../lib/useNow";

// The API returns at most this many listings (service.myLimit).
const listLimit = 50;

const monthYear = new Intl.DateTimeFormat("id-ID", { month: "long", year: "numeric" });

/** A seller's public page: who they are and everything they have on offer now. */
export function SellerPage() {
  const { username } = useParams();
  const now = useNow();
  const [seller, setSeller] = useState<{ name: string; data: SellerProfile } | null>(null);
  const [error, setError] = useState("");

  useEffect(() => {
    let stale = false;
    api<SellerProfile>(`/sellers/${encodeURIComponent(username ?? "")}`).then(
      (data) => !stale && setSeller({ name: username ?? "", data }),
      (e) => !stale && setError(errorMessage(e)),
    );
    return () => {
      stale = true;
    };
  }, [username]);

  if (!seller || seller.name !== username) {
    return error ? <ErrorNote message={error} /> : <p className="text-ink-soft">Memuat penjual…</p>;
  }
  const p = seller.data;
  return (
    <div className="space-y-6">
      <section className="flex flex-wrap items-center gap-4 rounded-2xl border border-line bg-card p-5">
        <span aria-hidden className="grid size-16 shrink-0 place-items-center rounded-full bg-ink font-display text-3xl font-extrabold text-white uppercase">
          {p.username.charAt(0)}
        </span>
        <div className="min-w-48 flex-1">
          <h1 className="truncate font-display text-3xl font-extrabold italic">{p.username}</h1>
          <p className="text-ink-soft">Bergabung {monthYear.format(new Date(p.created_at))}</p>
        </div>
        <dl className="flex gap-6 sm:text-right">
          <div>
            <dt className="text-sm text-ink-soft">Sedang dilelang</dt>
            <dd className="font-display text-3xl font-bold tabular-nums">{p.active.length >= listLimit ? `${listLimit}+` : p.active.length}</dd>
          </div>
          <div>
            <dt className="text-sm text-ink-soft">Terjual</dt>
            <dd className="font-display text-3xl font-bold tabular-nums">{p.sold}</dd>
          </div>
        </dl>
      </section>

      <h2 className="font-display text-2xl font-bold italic">Lelang dari {p.username}</h2>
      {p.active.length === 0 ? (
        <p className="rounded-xl border border-dashed border-line px-4 py-10 text-center text-ink-soft">Tidak ada lelang yang sedang berjalan.</p>
      ) : (
        <div className="grid grid-cols-1 gap-4 sm:grid-cols-2 lg:grid-cols-3 xl:grid-cols-4">
          {p.active.map((a) => (
            <AuctionCard key={a.id} auction={a} now={now} />
          ))}
        </div>
      )}
    </div>
  );
}
