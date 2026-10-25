import { useEffect, useRef, useState } from "react";
import { Link, useNavigate } from "react-router";
import { AuctionCard } from "../components/AuctionCard";
import { Button, ErrorNote } from "../components/ui";
import { api, errorMessage, imageSrc, type Auction, type MyAuctions, type Profile } from "../lib/api";
import { useAuth } from "../lib/auth";
import { castingNumber, rupiah, timeLeft, trackerStatus } from "../lib/format";
import { useNow } from "../lib/useNow";

// The API returns at most this many per list (service.myLimit).
const listLimit = 50;

const monthYear = new Intl.DateTimeFormat("id-ID", { month: "long", year: "numeric" });

type Tab = "won" | "bidding" | "lost" | "selling";

// One tab per part a user plays in an auction; the count doubles as a stat.
const tabs: { key: Tab; label: string; empty: string }[] = [
  { key: "won", label: "Menang", empty: "Belum ada yang dimenangkan." },
  { key: "bidding", label: "Sedang ditawar", empty: "Tidak ada lelang yang sedang Anda tawar." },
  { key: "lost", label: "Kalah", empty: "Belum pernah kalah tawar." },
  { key: "selling", label: "Dijual", empty: "Anda belum menjual apa pun." },
];

/** One row of the bid tracker: my best bid against the price now. */
function TrackerRow({ a, myBid, myId, now }: { a: Auction; myBid: number; myId: number | null; now: number }) {
  const leading = a.highest_bidder_id === myId;
  const left = timeLeft(a.end_time, now);
  const open = a.status === "active" && left.ms > 0;
  const status = trackerStatus(a, myId, open);
  const good = status === "Memimpin" || status === "Menang";
  return (
    <li className="flex flex-wrap items-center gap-x-4 gap-y-2 px-4 py-3">
      <Link to={`/lelang/${a.id}`} className="flex min-w-0 flex-1 basis-56 items-center gap-3">
        <span className="grid size-14 shrink-0 place-items-center overflow-hidden rounded-lg bg-paper">
          {a.item.image_url && <img src={imageSrc(a.item.image_url)} alt="" referrerPolicy="no-referrer" className="h-full w-full object-contain mix-blend-multiply" />}
        </span>
        <span className="min-w-0">
          <span className="block truncate font-semibold hover:underline">{castingNumber(a.item.model_name).name}</span>
          <span className={`text-sm font-semibold ${good ? "text-lead" : "text-signal"}`}>{status}</span>
          {open && <span className={`ml-2 text-sm tabular-nums ${left.closing ? "font-semibold text-signal" : "text-ink-soft"}`}>{left.label}</span>}
        </span>
      </Link>
      <dl className="flex gap-5 text-right">
        <div>
          <dt className="text-xs text-ink-soft">Tawaran saya</dt>
          <dd className="font-display text-lg font-bold tabular-nums">{rupiah(myBid)}</dd>
        </div>
        <div>
          <dt className="text-xs text-ink-soft">{open ? "Tertinggi" : "Akhir"}</dt>
          <dd className="font-display text-lg font-bold tabular-nums">{rupiah(a.current_highest_bid ?? a.start_price)}</dd>
        </div>
      </dl>
      {open && !leading && (
        <Link to={`/lelang/${a.id}`} className="press rounded-lg bg-track px-3 py-2 text-sm font-semibold text-white hover:bg-track-dark">
          Tawar lagi
        </Link>
      )}
    </li>
  );
}

export function ProfilePage() {
  const { userId, logout } = useAuth();
  const navigate = useNavigate();
  const now = useNow();
  const [profile, setProfile] = useState<Profile | null>(null);
  const [mine, setMine] = useState<MyAuctions | null>(null);
  const [tab, setTab] = useState<Tab>("won");
  const [error, setError] = useState("");
  const confirm = useRef<HTMLDialogElement>(null);

  useEffect(() => {
    Promise.all([api<Profile>("/me"), api<MyAuctions>("/me/auctions")]).then(
      ([p, m]) => {
        setProfile(p);
        setMine(m);
      },
      (e) => setError(errorMessage(e)),
    );
  }, []);

  if (!profile || !mine) {
    return error ? <ErrorNote message={error} /> : <p className="text-ink-soft">Memuat profil…</p>;
  }

  const current = tabs.find((t) => t.key === tab)!;
  const list = mine[tab];
  const tracker = tab === "bidding" || tab === "lost";
  return (
    <div className="space-y-6">
      <section className="flex flex-wrap items-center gap-4 rounded-2xl border border-line bg-card p-5">
        <span aria-hidden className="grid size-16 shrink-0 place-items-center rounded-full bg-track font-display text-3xl font-extrabold text-white uppercase">
          {profile.username.charAt(0)}
        </span>
        <div className="min-w-48 flex-1">
          <h1 className="truncate font-display text-3xl font-extrabold italic">{profile.username}</h1>
          <p className="truncate text-ink-soft">
            {profile.email}, bergabung {monthYear.format(new Date(profile.created_at))}
          </p>
          <Link to={`/penjual/${profile.username}`} className="text-sm font-semibold underline underline-offset-4">
            Lihat halaman penjual Anda
          </Link>
        </div>
        <Button variant="ghost" onClick={() => confirm.current?.showModal()}>
          Keluar
        </Button>
      </section>

      {/* Logging out drops the session, so ask first instead of acting on one stray tap. */}
      <dialog ref={confirm} className="m-auto w-[min(92vw,24rem)] rounded-2xl p-6 text-ink backdrop:bg-ink/40">
        <h2 className="font-display text-2xl font-bold italic">Keluar dari akun?</h2>
        <p className="mt-1 text-ink-soft">Tawaran Anda tetap berjalan. Masuk lagi kapan saja untuk memantaunya.</p>
        <form method="dialog" className="mt-5 flex justify-end gap-2">
          <Button variant="ghost">Batal</Button>
          <Button
            type="button"
            variant="track"
            onClick={() => {
              logout();
              navigate("/");
            }}
          >
            Keluar
          </Button>
        </form>
      </dialog>

      <div role="tablist" aria-label="Lelang saya" className="grid grid-cols-2 gap-2 sm:grid-cols-4">
        {tabs.map((t) => (
          <button
            key={t.key}
            role="tab"
            aria-selected={tab === t.key}
            onClick={() => setTab(t.key)}
            className={`press rounded-xl border px-4 py-3 text-left ${tab === t.key ? "border-ink bg-ink text-white" : "border-line bg-card hover:border-ink"}`}
          >
            <span className="block font-display text-3xl font-bold tabular-nums">{mine[t.key].length >= listLimit ? `${listLimit}+` : mine[t.key].length}</span>
            <span className={`text-sm font-semibold ${tab === t.key ? "text-white/80" : "text-ink-soft"}`}>{t.label}</span>
          </button>
        ))}
      </div>

      {list.length === 0 ? (
        <p className="rounded-xl border border-dashed border-line px-4 py-10 text-center text-ink-soft">
          {current.empty}{" "}
          <Link to={tab === "selling" ? "/jual" : "/"} className="font-semibold text-ink underline underline-offset-4">
            {tab === "selling" ? "Jual sekarang" : "Lihat katalog"}
          </Link>
        </p>
      ) : tracker ? (
        <ol className="divide-y divide-line overflow-hidden rounded-xl border border-line bg-card">
          {list.map((a) => (
            <TrackerRow key={a.id} a={a} myBid={mine.my_bids[a.id] ?? 0} myId={userId} now={now} />
          ))}
        </ol>
      ) : (
        <div className="grid grid-cols-1 gap-4 sm:grid-cols-2 lg:grid-cols-3 xl:grid-cols-4">
          {list.map((a) => (
            <div key={a.id} className="space-y-2">
              <AuctionCard auction={a} now={now} myId={userId} />
              {a.status === "completed" && a.winner_id !== null && (
                <Link to={`/lelang/${a.id}#chat`} className="press block rounded-lg bg-ink py-2 text-center text-sm font-semibold text-white">
                  {tab === "won" ? "Chat penjual, atur pengiriman" : "Chat pembeli, atur pengiriman"}
                </Link>
              )}
            </div>
          ))}
        </div>
      )}
    </div>
  );
}
