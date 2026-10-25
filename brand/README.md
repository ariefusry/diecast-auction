# Blister64: panduan logo

**Ide:** kemasan blister yang tergantung miring di pasak rak toko. Kolektor langsung mengenali bendanya, dan logo memakai bahasa visual yang sama dengan kartu katalog dan rak pegboard di website.

## File (`brand/logo/`)

| File | Pakai untuk |
|---|---|
| `blister64-horizontal.svg` | Logo utama: header, dokumen, presentasi |
| `blister64-stacked.svg` | Ruang sempit dan tinggi: poster, splash |
| `blister64-symbol.svg` | Simbol di samping teks lain, 32 px ke atas |
| `blister64-symbol-64.svg` | Simbol berdiri sendiri: avatar sosial, ikon aplikasi |
| `blister64-symbol-small.svg` | 16 sampai 31 px: favicon, tab browser |
| `*-reversed.svg` | Di atas latar gelap (navy atau foto gelap) |
| `*-black.svg` | Cetak satu warna, stempel, emboss |

Ikon web siap pakai ada di `frontend/public/` (favicon.ico, favicon.svg, apple-touch-icon.png, icon-192/512, maskable-512, site.webmanifest). Mockup ada di `brand/slides/`.

## Warna

| Nama | HEX | RGB | CMYK (perkiraan) | Pantone (terdekat) | Peran |
|---|---|---|---|---|---|
| Track | `#FF6A13` | 255 106 19 | 0 58 93 0 | 1585 C | Kartu blister |
| Ink | `#14213D` | 20 33 61 | 67 46 0 76 | 2767 C | Pasak dan wordmark |
| Paper | `#EEF1F5` | 238 241 245 | 3 2 0 4 | Cool Gray 1 C | Latar terang |

Nilai CMYK dan Pantone hanya perkiraan. Minta proof cetak sebelum produksi.

## Tipografi

Wordmark memakai Barlow Condensed ExtraBold Italic (SIL Open Font License, boleh untuk logo) yang sudah diubah jadi outline. Jangan mengetik ulang wordmark dengan font. UI memakai Barlow dan Barlow Condensed.

## Ruang kosong dan ukuran minimum

- Ruang kosong di sekeliling logo minimal setinggi lubang gantung dikali dua (sekitar 1/7 tinggi simbol).
- Logo horizontal minimal 96 px lebar di layar, 25 mm di cetak.
- Di bawah 32 px pakai `blister64-symbol-small.svg`: lubang gantungnya dihilangkan karena akan menutup.

## Latar yang boleh

- Putih, Paper, atau abu terang: versi berwarna.
- Navy Ink atau foto gelap: versi `-reversed`.
- Di atas warna kartu merek (biru Hot Wheels, merah Tomica, dan lainnya): versi `-reversed` atau `-black`, jangan versi berwarna.

## Jangan

- Meluruskan kartu. Kemiringan 9 derajat adalah bagian dari logo.
- Mengganti warna kartu dengan warna lain selain Track, atau memberi gradien dan bayangan.
- Memakai `symbol-64` di samping wordmark (angka 64 jadi tertulis dua kali).
- Meregangkan, memutar, atau memberi outline pada logo.
- Menaruh versi berwarna di atas latar oranye atau foto yang ramai.

## Catatan

- Nama "Blister64" belum dicek di pangkalan data merek. Cek di PDKI (pdki-indonesia.dgip.go.id) sebelum dipakai komersial.
- Master dibuat dari satu definisi geometri. Simbol berukuran 256 x 256, kartunya diputar -9 derajat pada titik (128, 64).
