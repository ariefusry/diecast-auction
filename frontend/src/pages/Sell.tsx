import { useEffect, useMemo, useState, type FormEvent } from "react";
import { useNavigate } from "react-router";
import { Button, ErrorNote, Field, Input, Select } from "../components/ui";
import { api, errorMessage, type Auction } from "../lib/api";
import { conditions } from "../lib/format";

const durations: Record<string, string> = { "60": "1 jam", "360": "6 jam", "1440": "1 hari", "4320": "3 hari", "10080": "7 hari" };

export function Sell() {
  const navigate = useNavigate();
  const [form, setForm] = useState({
    brand: "Hot Wheels",
    model_name: "",
    series: "",
    year: "",
    color: "",
    tags: "",
    condition: "mint_on_card",
    description: "",
    start_price: "",
    min_increment: "5000",
    buy_now_price: "",
    duration_minutes: "1440",
  });
  const [photo, setPhoto] = useState<File | null>(null);
  const [publish, setPublish] = useState(true);
  const [error, setError] = useState("");
  const [busy, setBusy] = useState(false);

  const preview = useMemo(() => (photo ? URL.createObjectURL(photo) : ""), [photo]);
  useEffect(() => () => URL.revokeObjectURL(preview), [preview]);

  const set = (k: keyof typeof form) => (e: { target: { value: string } }) => setForm({ ...form, [k]: e.target.value });

  async function submit(e: FormEvent) {
    e.preventDefault();
    setBusy(true);
    setError("");
    try {
      let image_url = "";
      if (photo) {
        const body = new FormData();
        body.append("image", photo);
        image_url = (await api<{ url: string }>("/uploads", { method: "POST", body })).url;
      }
      const created = await api<Auction>("/auctions", {
        method: "POST",
        body: {
          item: {
            brand: form.brand,
            model_name: form.model_name,
            series: form.series,
            year: Number(form.year) || 0,
            color: form.color,
            tags: form.tags
              .split(",")
              .map((t) => t.trim().toLowerCase())
              .filter(Boolean),
            condition: form.condition,
            image_url,
            description: form.description,
          },
          start_price: Number(form.start_price),
          min_increment: Number(form.min_increment),
          buy_now_price: form.buy_now_price ? Number(form.buy_now_price) : null,
          duration_minutes: Number(form.duration_minutes),
        },
      });
      // The auction exists from here on. If publishing fails it stays a draft,
      // which the detail page can publish, so never resubmit the form (duplicate).
      if (publish) await api(`/auctions/${created.id}/publish`, { method: "POST" }).catch(() => undefined);
      navigate(`/lelang/${created.id}`);
    } catch (err) {
      setError(errorMessage(err));
    } finally {
      setBusy(false);
    }
  }

  return (
    <form onSubmit={submit} className="grid gap-8 lg:grid-cols-[1fr_1.3fr]">
      <div className="space-y-3">
        <h1 className="font-display text-4xl font-extrabold italic">Jual diecast</h1>
        <div className="blister">
          <div className="bubble flex aspect-[4/3] items-center justify-center">
            {preview ? (
              <img src={preview} alt="Pratinjau foto" className="h-full w-full object-contain p-3" />
            ) : (
              <span className="px-6 text-center text-ink-soft">Foto tampil di sini, seperti di dalam blister.</span>
            )}
          </div>
        </div>
        <Field label="Foto" hint="JPEG, PNG atau WebP, maksimal 5 MB.">
          <input
            type="file"
            accept="image/jpeg,image/png,image/webp"
            onChange={(e) => setPhoto(e.target.files?.[0] ?? null)}
            className="block w-full text-sm file:mr-3 file:rounded-lg file:border-0 file:bg-ink file:px-4 file:py-2 file:font-semibold file:text-white"
          />
        </Field>
      </div>

      <div className="space-y-4">
        <div className="grid gap-4 sm:grid-cols-2">
          <Field label="Merek">
            <Select value={form.brand} onChange={set("brand")}>
              <option>Hot Wheels</option>
              <option>Mini GT</option>
              <option>Inno64</option>
            </Select>
          </Field>
          <Field label="Kondisi">
            <Select value={form.condition} onChange={set("condition")}>
              {Object.entries(conditions).map(([v, l]) => (
                <option key={v} value={v}>
                  {l}
                </option>
              ))}
            </Select>
          </Field>
        </div>
        <Field label="Nama model">
          <Input required value={form.model_name} onChange={set("model_name")} placeholder="Nissan Skyline GT-R (R34)" />
        </Field>
        <div className="grid gap-4 sm:grid-cols-3">
          <Field label="Seri">
            <Input value={form.series} onChange={set("series")} placeholder="Car Culture" />
          </Field>
          <Field label="Tahun rilis">
            <Input type="number" min={1968} max={2100} value={form.year} onChange={set("year")} />
          </Field>
          <Field label="Warna">
            <Input value={form.color} onChange={set("color")} placeholder="Bayside Blue" />
          </Field>
        </div>
        <Field label="Tag" hint="Pisahkan dengan koma, misalnya jdm, nissan.">
          <Input value={form.tags} onChange={set("tags")} />
        </Field>
        <Field label="Deskripsi">
          <textarea
            value={form.description}
            onChange={set("description")}
            rows={3}
            className="w-full rounded-lg border border-line bg-card px-3 py-2.5 focus:border-ink focus:outline-none"
          />
        </Field>
        <div className="grid gap-4 sm:grid-cols-3">
          <Field label="Harga awal (Rp)">
            <Input type="number" inputMode="numeric" min={1} required value={form.start_price} onChange={set("start_price")} />
          </Field>
          <Field label="Kelipatan (Rp)">
            <Input type="number" inputMode="numeric" min={1} required value={form.min_increment} onChange={set("min_increment")} />
          </Field>
          <Field label="Beli langsung (Rp)" hint="Opsional, di atas harga awal.">
            <Input type="number" inputMode="numeric" min={1} value={form.buy_now_price} onChange={set("buy_now_price")} />
          </Field>
        </div>
        <Field label="Durasi">
          <Select value={form.duration_minutes} onChange={set("duration_minutes")}>
            {Object.entries(durations).map(([v, l]) => (
              <option key={v} value={v}>
                {l}
              </option>
            ))}
          </Select>
        </Field>
        <label className="flex items-center gap-2">
          <input type="checkbox" checked={publish} onChange={(e) => setPublish(e.target.checked)} className="size-4 accent-ink" />
          <span>Langsung tayangkan. Jika tidak, lelang disimpan sebagai draf.</span>
        </label>
        <ErrorNote message={error} />
        <Button type="submit" disabled={busy} className="w-full text-lg">
          {publish ? "Tayangkan lelang" : "Simpan draf"}
        </Button>
      </div>
    </form>
  );
}
