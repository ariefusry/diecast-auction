import { useEffect, useState, type FormEvent } from "react";
import { AuctionCard } from "../components/AuctionCard";
import { BrandLogo } from "../components/BrandLogo";
import { PegWall } from "../components/PegWall";
import { Button, ErrorNote } from "../components/ui";
import { api, errorMessage, type Auction, type Filter, type SearchResult } from "../lib/api";
import { useAuth } from "../lib/auth";
import { conditionLabel, conditions, dioramaConditions, rupiah } from "../lib/format";
import { useNow } from "../lib/useNow";

const categories: Record<string, string> = { "": "Semua", diecast: "Diecast", diorama: "Diorama" };
const brands = ["Hot Wheels", "Mini GT", "Inno64", "Matchbox", "Tomica", "Tarmac Works", "M2 Machines"];
const sorts: Record<string, string> = {
  ending_soon: "Segera berakhir",
  newest: "Terbaru",
  price_asc: "Termurah",
  price_desc: "Termahal",
};
const examples = ["jdm merah di bawah 100k", "inno64 skyline termurah", "mini gt putih kondisi segel"];

/** What the AI understood, as short chips the collector can check at a glance. */
function understood(f: Filter): string[] {
  const out: string[] = [];
  if (f.category) out.push(categories[f.category] ?? f.category);
  if (f.brand) out.push(f.brand);
  if (f.text) out.push(`"${f.text}"`);
  if (f.color) out.push(`warna ${f.color}`);
  for (const t of f.tags ?? []) out.push(t);
  if (f.condition) out.push(conditionLabel(f.condition));
  if (f.min_price) out.push(`min ${rupiah(f.min_price)}`);
  if (f.max_price) out.push(`maks ${rupiah(f.max_price)}`);
  if (f.sort) out.push(sorts[f.sort] ?? f.sort);
  return out;
}

export function Catalog() {
  const { userId } = useAuth();
  const now = useNow();
  const [category, setCategory] = useState("");
  const [brand, setBrand] = useState("");
  const [condition, setCondition] = useState("");
  // The pegboard already shows what closes soonest, so the grid starts with what is new.
  const [sort, setSort] = useState("newest");
  const [query, setQuery] = useState("");
  const [aiResult, setAiResult] = useState<SearchResult | null>(null);
  const [result, setResult] = useState<{ key: string; data: SearchResult } | null>(null);
  const [searching, setSearching] = useState(false);
  const [error, setError] = useState("");
  const [closing, setClosing] = useState<Auction[]>([]);

  useEffect(() => {
    api<SearchResult>("/auctions?sort=ending_soon").then(
      (data) => setClosing(data.auctions.slice(0, 4)),
      () => undefined, // the grid below reports connection errors
    );
  }, []);

  const params = new URLSearchParams({ sort });
  if (category) params.set("category", category);
  if (brand) params.set("brand", brand);
  if (condition) params.set("condition", condition);
  const key = params.toString();

  useEffect(() => {
    let stale = false;
    api<SearchResult>(`/auctions?${key}`)
      .then((data) => !stale && (setResult({ key, data }), setError("")))
      .catch((e) => !stale && setError(errorMessage(e)));
    return () => {
      stale = true;
    };
  }, [key]);

  // Loading until the result for the current filters arrives.
  const loading = searching || (!aiResult && result?.key !== key && !error);

  async function searchAI(e: FormEvent) {
    e.preventDefault();
    if (!query.trim()) return;
    setSearching(true);
    try {
      setAiResult(await api<SearchResult>("/auctions/search/ai", { method: "POST", body: { query } }));
      setError("");
    } catch (err) {
      setError(errorMessage(err));
    } finally {
      setSearching(false);
    }
  }

  function clearAI() {
    setAiResult(null);
    setQuery("");
  }

  const shown = aiResult ?? result?.data;
  return (
    <div className="space-y-6">
      <section className="pegboard space-y-8 rounded-3xl p-5 sm:p-8">
        <form onSubmit={searchAI} className="space-y-3">
          <h1 className="font-display text-4xl leading-none font-extrabold italic sm:text-5xl">Cari diecast incaranmu</h1>
          <p className="max-w-prose font-medium">Ketik seperti chat ke sesama kolektor. Sebut merek, warna, kondisi atau budget.</p>
          <div className="flex flex-col gap-2 sm:flex-row">
            <input
              value={query}
              onChange={(e) => setQuery(e.target.value)}
              maxLength={200}
              placeholder={examples[0]}
              aria-label="Pencarian"
              className="min-w-0 flex-1 rounded-xl border-2 border-ink bg-card px-4 py-3 text-lg focus:outline-none"
            />
            <Button type="submit" className="px-6 text-lg" disabled={loading}>
              Cari
            </Button>
          </div>
          {!aiResult && (
            <div className="flex flex-wrap gap-2 text-sm">
              <span className="font-medium">Contoh:</span>
              {examples.map((ex) => (
                <button key={ex} type="button" onClick={() => setQuery(ex)} className="press rounded-full bg-card/70 px-3 py-1 text-ink hover:bg-card">
                  {ex}
                </button>
              ))}
            </div>
          )}
        </form>
        {closing.length > 0 && (
          <div className="space-y-1">
            <h2 className="font-display text-2xl font-bold italic">Segera berakhir</h2>
            <PegWall auctions={closing} now={now} myId={userId} />
          </div>
        )}
      </section>

      {aiResult ? (
        <div className="space-y-2 rounded-xl border border-line bg-card p-4">
          <div className="flex flex-wrap items-center gap-2">
            <span className="text-sm font-semibold">{aiResult.ai ? "Dipahami sebagai" : "Dicari sebagai kata kunci"}</span>
            {understood(aiResult.filter).map((c) => (
              <span key={c} className="rounded-full bg-paper px-3 py-1 text-sm">
                {c}
              </span>
            ))}
            <button onClick={clearAI} className="ml-auto text-sm font-semibold underline underline-offset-4">
              Hapus pencarian
            </button>
          </div>
          {aiResult.relaxed && aiResult.auctions.length > 0 && (
            <p className="text-sm text-ink-soft">Tidak ada yang persis cocok. Ini yang paling mendekati.</p>
          )}
        </div>
      ) : (
        <div className="space-y-3">
          <div role="tablist" aria-label="Kategori" className="inline-flex rounded-full border border-line bg-card p-1">
            {Object.entries(categories).map(([v, l]) => (
              <button
                key={v || "all"}
                role="tab"
                aria-selected={category === v}
                onClick={() => {
                  setCategory(v);
                  // Brands and packaging conditions do not apply to dioramas.
                  setBrand("");
                  setCondition("");
                }}
                className={`press rounded-full px-5 py-1.5 font-semibold ${category === v ? "bg-ink text-white" : "text-ink-soft hover:text-ink"}`}
              >
                {l}
              </button>
            ))}
          </div>
          <div className="flex flex-wrap items-center gap-2">
            {category !== "diorama" && ["", ...brands].map((b) => (
              <button
                key={b || "all"}
                onClick={() => setBrand(b)}
                aria-pressed={brand === b}
                className={`press flex h-10 items-center rounded-full border bg-card px-4 font-semibold ${brand === b ? "border-ink ring-2 ring-ink" : "border-line hover:border-ink"}`}
              >
                {b ? <BrandLogo brand={b} className="h-6 max-w-28" /> : "Semua merek"}
              </button>
            ))}
            <div className="flex w-full gap-2 sm:ml-auto sm:w-auto">
              <select value={condition} onChange={(e) => setCondition(e.target.value)} aria-label="Kondisi" className="min-w-0 flex-1 rounded-lg border border-line bg-card px-3 py-1.5 sm:flex-none">
                <option value="">Semua kondisi</option>
                {Object.entries({ ...(category !== "diorama" && conditions), ...(category !== "diecast" && dioramaConditions) }).map(([v, l]) => (
                  <option key={v} value={v}>
                    {l}
                  </option>
                ))}
              </select>
              <select value={sort} onChange={(e) => setSort(e.target.value)} aria-label="Urutkan" className="min-w-0 flex-1 rounded-lg border border-line bg-card px-3 py-1.5 sm:flex-none">
                {Object.entries(sorts).map(([v, l]) => (
                  <option key={v} value={v}>
                    {l}
                  </option>
                ))}
              </select>
            </div>
          </div>
        </div>
      )}

      <ErrorNote message={error} />
      {shown && shown.auctions.length === 0 && !loading && (
        <p className="py-12 text-center text-ink-soft">Belum ada lelang yang cocok. Coba kata lain atau hapus filter.</p>
      )}
      <div className={`grid grid-cols-1 gap-4 sm:grid-cols-2 lg:grid-cols-3 xl:grid-cols-4 ${loading ? "opacity-60" : ""}`}>
        {shown?.auctions.map((a) => <AuctionCard key={a.id} auction={a} now={now} myId={userId} />)}
      </div>
    </div>
  );
}
