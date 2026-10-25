# Product

<!-- impeccable:product-schema 1 -->

## Platform

web

## Users

- **Kolektor diecast 1:64 di Indonesia.** Mereka menawar lot satuan, sering dari HP sambil beraktivitas lain, lalu kembali di menit-menit terakhir lelang. Tugas utamanya: menemukan casting yang dicari, memasang tawaran, dan tahu seketika kalau disalip.
- **Penjual kecil.** Pemilik toko diecast, kolektor yang melepas koleksinya, dan seniman diorama yang memasang lelang secara rutin. Tugas utamanya: memasang lot dengan foto dan harga yang benar, lalu mengatur pengiriman dengan pemenang.

## Product Purpose

Blister64 adalah tempat lelang diecast skala 1:64 dan diorama. Penjual memasang lot, penawar menawar dengan dana yang dikunci di dompet, dan lelang ditutup otomatis dengan dana berpindah ke penjual. Produk ini berhasil kalau lelang berjalan adil, penawar yang menang benar-benar membayar, dan menawar terasa menyenangkan, bukan seperti mengisi formulir.

## Positioning

Dibanding lelang di grup Facebook atau WhatsApp dan marketplace umum:

- **Escrow dompet.** Dana penawar dikunci saat menawar dan dilepas saat disalip, jadi tidak ada penawar fiktif yang kabur setelah menang.
- **Anti-sniping.** Tawaran di 2 menit terakhir menambah waktu 2 menit, jadi lelang adil bagi penawar manusia, bukan bot.
- **Khusus 1:64 dan diorama.** Katalog per merek (Hot Wheels, Mini GT, Inno64, Matchbox, Tomica, Tarmac Works, M2 Machines) plus diorama, dengan pencarian kalimat biasa yang diterjemahkan AI menjadi filter.
- **Menawar terasa seperti game.** Stiker "MEMIMPIN!", alarm "Kamu disalip!" dengan tombol balas, bonus waktu "+2:00", dan timer yang berdetak di detik terakhir.

## Operating Context

- Lot punya siklus draf, tayang, selesai, atau dibatalkan. Penjual bisa membatalkan atau pembeli memakai beli langsung hanya selama belum ada tawaran.
- Setelah lelang dimenangkan, penjual dan pemenang mengatur alamat, kurir, dan nomor resi lewat chat di halaman lelang. Pengiriman sendiri terjadi di luar aplikasi.
- Halaman penjual publik di `/penjual/:username` menampilkan lot aktif dan jumlah lot yang terjual.
- Antarmuka berbahasa Indonesia dan memakai istilah kolektor (casting, mint on card, loose, blister).

## Capabilities and Constraints

- Uang selalu rupiah utuh. Saat ini saldo dompet masih simulasi (top-up mock, maksimal Rp10.000.000 per kali).
- **Keputusan terbuka:** dompet direncanakan terhubung ke pembayaran sungguhan (payment gateway). Penyedia dan waktunya belum diputuskan.
- Kondisi diecast: mint on card, box segel, akrilik, loose mulus, kemasan rusak. Diorama memakai kondisi baru atau bekas, artis opsional, ukuran, dan estimasi PO.
- Identitas penawar disamarkan: riwayat tawaran menampilkan nama yang disamarkan, dan respons API tidak membawa ID penawar maupun pemenang.
- Upload foto JPEG, PNG, atau WebP, maksimal 5 MiB, 20 foto per pengguna per hari.

## Brand Commitments

- Nama produk: **Blister64**.
- Logo konsep "On the Peg". Master SVG, favicon, ikon aplikasi, dan panduannya ada di `brand/README.md`.
- Suara: bahasa Indonesia sehari-hari yang hangat dan langsung, dengan istilah kolektor.

## Evidence on Hand

- Katalog nyata hasil scrape wiki Fandom: 263 item di `data/seeds.json`, foto dan data berlisensi CC-BY-SA, atribusi di `items.source_url`.
- Akun demo dari seeder (`cmd/seed/main.go`), hanya untuk demo.
- Belum ada testimoni, pengguna nyata, data transaksi, liputan media, maupun angka performa. Jangan dibuat-buat.

## Product Principles

1. **Uang tidak boleh salah.** Setiap rupiah yang dikunci, dilepas, atau dipindahkan harus bisa diaudit. Kenyamanan tidak boleh mengorbankan ketepatan saldo.
2. **Adil bagi penawar manusia.** Waktu ditentukan server dan anti-sniping melindungi detik terakhir. Antarmuka tidak boleh menjanjikan peluang yang ditolak server.
3. **Status selalu jelas.** Penawar harus tahu seketika apakah sedang memimpin, disalip, menang, atau kalah, dan berapa yang harus dibayar.
4. **Kegembiraan berasal dari hobi.** Rasa game dan dunia kemasan diecast adalah bagian dari produk, tetapi tidak boleh menutupi informasi harga dan waktu.
5. **Privasi penawar dijaga.** Penawar terlihat aktif tanpa terlihat siapa orangnya.
