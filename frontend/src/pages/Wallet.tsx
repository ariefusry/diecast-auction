import { useEffect, useState, type FormEvent } from "react";
import { Button, ErrorNote } from "../components/ui";
import { api, errorMessage, type Wallet as WalletT } from "../lib/api";
import { rupiah } from "../lib/format";

const quick = [100_000, 500_000, 1_000_000, 5_000_000];

export function Wallet() {
  const [wallet, setWallet] = useState<WalletT | null>(null);
  const [amount, setAmount] = useState("");
  const [error, setError] = useState("");
  const [notice, setNotice] = useState("");
  const [busy, setBusy] = useState(false);

  useEffect(() => {
    api<WalletT>("/wallet").then(setWallet, (e) => setError(errorMessage(e)));
  }, []);

  async function deposit(e: FormEvent) {
    e.preventDefault();
    setError("");
    setNotice("");
    setBusy(true);
    try {
      const value = Number(amount);
      setWallet(await api<WalletT>("/wallet/deposit", { method: "POST", body: { amount: value } }));
      setNotice(`Saldo bertambah ${rupiah(value)}.`);
      setAmount("");
    } catch (err) {
      setError(errorMessage(err));
    } finally {
      setBusy(false);
    }
  }

  return (
    <div className="max-w-2xl space-y-6">
      <h1 className="font-display text-4xl font-extrabold italic">Dompet</h1>
      {wallet && (
        <div className="grid gap-4 sm:grid-cols-2">
          <div className="rounded-xl border border-line bg-card p-5">
            <div className="text-ink-soft">Bisa dipakai</div>
            <div className="font-display text-4xl font-bold tabular-nums">{rupiah(wallet.available_balance)}</div>
          </div>
          <div className="rounded-xl border border-line bg-card p-5">
            <div className="text-ink-soft">Terkunci di tawaran</div>
            <div className="font-display text-4xl font-bold tabular-nums">{rupiah(wallet.locked_balance)}</div>
            <p className="mt-1 text-sm text-ink-soft">Kembali otomatis saat Anda kalah tawar.</p>
          </div>
        </div>
      )}

      <form onSubmit={deposit} className="space-y-3 rounded-xl border border-line bg-card p-5">
        <h2 className="font-display text-2xl font-bold">Isi saldo</h2>
        <p className="text-sm text-ink-soft">Simulasi top-up, tanpa uang sungguhan. Maksimal Rp10.000.000 per pengisian.</p>
        <div className="flex flex-wrap gap-2">
          {quick.map((q) => (
            <button
              key={q}
              type="button"
              onClick={() => setAmount(String(q))}
              aria-pressed={amount === String(q)}
              className={`rounded-full border px-3 py-1.5 font-semibold ${amount === String(q) ? "border-ink bg-ink text-white" : "border-line hover:border-ink"}`}
            >
              {rupiah(q)}
            </button>
          ))}
        </div>
        <div className="flex gap-2">
          <input
            type="number"
            inputMode="numeric"
            min={1}
            max={10_000_000}
            required
            value={amount}
            onChange={(e) => setAmount(e.target.value)}
            placeholder="Jumlah lain"
            aria-label="Jumlah"
            className="min-w-0 flex-1 rounded-lg border border-line px-3 py-2.5 tabular-nums focus:border-ink focus:outline-none"
          />
          <Button type="submit" disabled={busy}>
            Isi saldo
          </Button>
        </div>
        <ErrorNote message={error} />
        {notice && (
          <p role="status" className="text-sm font-medium text-lead">
            {notice}
          </p>
        )}
      </form>
    </div>
  );
}
