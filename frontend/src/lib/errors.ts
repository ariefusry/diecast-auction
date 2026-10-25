import { rupiah } from "./format";

// The API answers in English for any client; people here read Indonesian. Each
// message says what failed and, when there is one, how to recover.

const fields: Record<string, string> = {
  model_name: "Nama model",
  series: "Seri",
  color: "Warna",
  description: "Deskripsi",
  artist: "Artis",
  size: "Ukuran",
};

const statuses: Record<string, string> = { completed: "sudah selesai", cancelled: "sudah dibatalkan", active: "sudah tayang", draft: "masih draf" };

// Matched against the message without its "conflict: " style prefix.
const known: [RegExp, (m: RegExpMatchArray) => string][] = [
  [/^bid must be at least (\d+)$/, (m) => `Tawaran minimal sekarang ${rupiah(+m[1])}.`],
  [/^insufficient balance$/, () => "Saldo tidak cukup. Isi saldo di Dompet lalu coba lagi."],
  [/^you are already the highest bidder$/, () => "Anda sudah memimpin lelang ini."],
  [/^auction is not open$|^auction has ended$/, () => "Lelang ini sudah ditutup."],
  [/^auction is \w+, not draft$/, () => "Lelang ini sudah ditayangkan."],
  [/^auction is (\w+)$/, (m) => `Lelang ini ${statuses[m[1]] ?? "tidak bisa diubah lagi"}.`],
  [/^auction already has bids$/, () => "Lelang yang sudah punya tawaran tidak bisa dibatalkan."],
  [/^buy-now is closed once bidding starts$/, () => "Beli langsung tidak berlaku lagi karena sudah ada tawaran."],
  [/^auction has no buy-now price$/, () => "Lelang ini tidak punya harga beli langsung."],
  [/^sellers cannot bid on their own auction$/, () => "Anda tidak bisa menawar lelang sendiri."],
  [/^sellers cannot buy their own auction$/, () => "Anda tidak bisa membeli lelang sendiri."],
  [/^only the seller can cancel$/, () => "Hanya penjual yang bisa membatalkan lelang ini."],
  [/^only the seller can publish$/, () => "Hanya penjual yang bisa menayangkan lelang ini."],
  [/^only the seller and the winner can chat/, () => "Chat hanya untuk penjual dan pemenang, setelah lelang dimenangkan."],
  [/^a message is 1-1000 characters/, () => "Pesan harus berisi 1 sampai 1.000 karakter."],
  [/^amount must be 1-(\d+)$/, (m) => `Jumlah harus Rp1 sampai ${rupiah(+m[1])}.`],
  [/^prices must be at most (\d+)$/, (m) => `Harga maksimal ${rupiah(+m[1])}.`],
  [/^at most (\d+) photos a day$/, (m) => `Batas ${m[1]} foto per hari sudah tercapai. Coba lagi besok.`],
  [/^photo uploads are full for today/, () => "Kapasitas upload foto hari ini sudah penuh. Coba lagi besok."],
  [/^image must be JPEG, PNG or WebP$/, () => "Foto harus berformat JPEG, PNG atau WebP."],
  [/^image must be at most 5 MiB$|^send one image/, () => "Ukuran foto maksimal 5 MB."],
  [/^email is not valid$/, () => "Alamat email tidak valid."],
  [/^email or username is taken$/, () => "Email atau username sudah dipakai. Kalau ini akun Anda, silakan masuk."],
  [/^username must be/, () => "Username 3 sampai 30 karakter: huruf, angka atau garis bawah."],
  [/^password must be/, () => "Kata sandi 8 sampai 72 karakter."],
  [/^invalid email or password$/, () => "Email atau kata sandi salah."],
  [/^query must be/, () => "Pencarian maksimal 200 karakter."],
  [/^duration_minutes must be/, () => "Pilih durasi lelang dari daftar."],
  [/^violates .*buy_now/, () => "Harga beli langsung harus di atas harga awal."],
  [/^model_name is required$/, () => "Nama model wajib diisi."],
  [/^year must be (\d+)-(\d+)$/, (m) => `Tahun rilis harus antara ${m[1]} dan ${m[2]}.`],
  [/^at most (\d+) tags$/, (m) => `Maksimal ${m[1]} tag.`],
  [/^each tag must be at most (\d+) characters$/, (m) => `Setiap tag maksimal ${m[1]} karakter.`],
  [/^(\w+) must be at most (\d+) characters$/, (m) => `${fields[m[1]] ?? "Isian"} maksimal ${m[2]} karakter.`],
  [/^brand must be one of/, () => "Pilih merek dari daftar."],
  [/^unauthorized$/, () => "Sesi Anda berakhir. Masuk lagi untuk melanjutkan."],
  [/^too many requests$/, () => "Terlalu banyak permintaan. Tunggu sebentar lalu coba lagi."],
];

function byStatus(status: number): string {
  if (status === 401) return "Sesi Anda berakhir. Masuk lagi untuk melanjutkan.";
  if (status === 403) return "Anda tidak punya akses untuk tindakan ini.";
  if (status === 404) return "Data tidak ditemukan. Mungkin sudah dihapus.";
  if (status === 409) return "Tindakan ini tidak bisa dilakukan sekarang. Muat ulang halaman lalu coba lagi.";
  if (status === 413) return "Ukuran foto maksimal 5 MB.";
  if (status === 429) return "Terlalu banyak permintaan. Tunggu sebentar lalu coba lagi.";
  if (status >= 500) return "Server sedang bermasalah. Coba lagi sebentar lagi.";
  return "Isian tidak valid. Periksa lagi lalu coba lagi.";
}

/** The Indonesian message for an API error; unknown ones fall back on the status. */
export function serverError(status: number, message: string): string {
  const text = message.replace(/^(invalid input|conflict|forbidden|not found): /, "");
  for (const [pattern, say] of known) {
    const m = text.match(pattern);
    if (m) return say(m);
  }
  return byStatus(status);
}
