import { useEffect, useState, type FormEvent } from "react";
import { AuctionCard } from "../components/AuctionCard";
import { Button, ErrorNote } from "../components/ui";
import { api, errorMessage, type Filter, type SearchResult } from "../lib/api";
import { useAuth } from "../lib/auth";
import { conditionLabel, conditions, rupiah } from "../lib/format";
import { useNow } from "../lib/useNow";

const brands = ["Hot Wheels", "Mini GT", "Inno64"];
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
  const [brand, setBrand] = useState("");
  const [condition, setCondition] = useState("");
  const [sort, setSort] = useState("ending_soon");
  const [query, setQuery] = useState("");
  const [aiResult, setAiResult] = useState<SearchResult | null>(null);
  const [result, setResult] = useState<{ key: string; data: SearchResult } | null>(null);
  const [searching, setSearching] = useState(false);
  const [error, setError] = useState("");

  const params = new URLSearchParams({ sort });
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
      <form onSubmit={searchAI} className="space-y-3">
        <h1 className="font-display text-4xl leading-none font-extrabold italic sm:text-5xl">Cari diecast incaranmu</h1>
        <p className="max-w-prose text-ink-soft">Ketik seperti chat ke sesama kolektor. Sebut merek, warna, kondisi atau budget.</p>
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
            <span className="text-ink-soft">Contoh:</span>
            {examples.map((ex) => (
              <button key={ex} type="button" onClick={() => setQuery(ex)} className="press text-ink underline decoration-line underline-offset-4 hover:decoration-ink">
                {ex}
              </button>
            ))}
          </div>
        )}
      </form>

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
        <div className="flex flex-wrap items-center gap-2">
          {["", ...brands].map((b) => (
            <button
              key={b || "all"}
              onClick={() => setBrand(b)}
              aria-pressed={brand === b}
              className={`press rounded-full border px-4 py-1.5 font-semibold ${brand === b ? "border-ink bg-ink text-white" : "border-line bg-card hover:border-ink"}`}
            >
              {b || "Semua merek"}
            </button>
          ))}
          <div className="flex w-full gap-2 sm:ml-auto sm:w-auto">
            <select value={condition} onChange={(e) => setCondition(e.target.value)} aria-label="Kondisi" className="min-w-0 flex-1 rounded-lg border border-line bg-card px-3 py-1.5 sm:flex-none">
              <option value="">Semua kondisi</option>
              {Object.entries(conditions).map(([v, l]) => (
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
