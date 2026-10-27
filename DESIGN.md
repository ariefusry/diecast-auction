# DESIGN.md

Arah desain Blister64. Dipakai sebagai acuan sebelum mengubah UI. Sumber: keputusan yang sudah ada di `frontend/src/index.css` dan komponen.

Dial: ENERGY 2 / RHYTHM 2 / MOTION 2

## Identitas

Rak toko mainan. Setiap lot adalah kemasan blister: kartu cetak berwarna merek, foto di gelembung plastik, label harga putih di bawah. Katalog tergantung di pegboard cokelat dengan peg krom. Motif ini satu-satunya dekorasi di antarmuka, dan hanya muncul di tempat yang masuk akal (kartu lot dan area pencarian).

Alasan: pengguna adalah kolektor diecast 1:64. Mereka sudah akrab dengan kemasan blister, jadi motifnya menandai produk tanpa penjelasan.

## Warna

| Token | Nilai | Peran |
|---|---|---|
| paper | #eef1f5 | latar halaman |
| card | #ffffff | permukaan kartu dan input |
| ink | #14213d | teks, border aktif, tombol utama |
| ink-soft | #4a5672 | teks sekunder, placeholder |
| line | #cfd6e2 | border tipis |
| track | #ff6a13 | satu-satunya aksen: tawar (bid), fokus keyboard, penanda seleksi |
| signal | #d62828 | jendela anti-sniping (2 menit terakhir) |
| lead | #2a9d8f | "Anda memimpin", harga baru masuk |

Warna kartu per merek diambil dari kemasan asli (`lib/brands.ts`), bukan dari palet ini.

Aturan: oranye hanya untuk tawar dan fokus. Merah hanya untuk hitung mundur akhir. Mint hanya untuk status memimpin.

## Tipografi

- Barlow Condensed (miring, tebal) untuk judul, harga, dan hitung mundur. Alasan: huruf condensed dipakai di kemasan Hot Wheels dan Tomica, dan angka harga tetap muat di kartu sempit.
- Barlow untuk teks isi. Alasan: pasangan natural Barlow Condensed, terbaca di ukuran kecil.
- Angka memakai `tabular-nums` agar harga dan timer tidak bergeser.

## Bentuk dan permukaan

- Radius: kartu 14px, gelembung foto 26px, input dan tombol 8 sampai 12px, pil untuk hal yang bisa dipilih (merek, chip, tab). Variasi radius menandai peran.
- Blur (`material`, bottom nav, dock) hanya di chrome yang menempel (header, bottom nav, bid dock). Tidak pernah di kartu atau modal.
- Bayangan menandai kartu yang bisa diangkat (hover) dan kartu yang digantung di peg. Selebihnya datar.
- Tanpa gradien dekoratif dan tanpa glow.

## Gerak

Setiap animasi punya sebab di data: harga baru masuk (`land`), bid berhasil (`stamp`), bonus anti-sniping (`bonus`), digit timer berganti (`roll`), kartu dibuka lalu morph ke halaman detail (view transition). Tidak ada gerak yang berjalan terus tanpa pemicu, kecuali detak `tick` di dua menit terakhir. `prefers-reduced-motion` mempertahankan penyebab perubahan tanpa pergerakan.

## Aksesibilitas

- Fokus keyboard: outline oranye 2px global lewat `:focus-visible`. Jangan memakai `focus:outline-none`.
- Placeholder memakai `ink-soft` penuh agar kontras di atas putih tetap di atas 4.5:1.
- Foto yang gagal dimuat jatuh ke tulisan "Tanpa foto".

## Bahasa

Indonesia, informal tapi jelas ("Cari diecast incaranmu"). Tanpa em dash. Tanpa klaim statistik atau testimoni.
