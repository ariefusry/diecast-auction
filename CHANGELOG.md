# Changelog Blister64

Catatan semua perubahan, dari fondasi sampai fitur terbaru. Hash commit ada di tiap baris agar mudah ditelusuri dengan `git show <hash>`. Detail keputusan teknis per sesi ada di `SCRATCHPAD.md`.

Status: Phase 1 sampai 10 selesai kecuali deploy (menunggu pilihan hosting). Belum di-push ke GitHub.

---

## Ringkasan produk

Blister64 adalah lelang diecast skala 1:64 dan diorama. Penjual memasang lelang, penawar memasang tawaran dengan dana yang dikunci di dompet (escrow), lelang diperpanjang 2 menit kalau ada tawaran di menit-menit terakhir (anti-sniping), dan worker menutup lelang yang habis waktunya lalu memindahkan dana ke penjual. Pencarian bisa memakai kalimat biasa yang diterjemahkan Gemini menjadi filter.

- Backend: Go 1.27, PostgreSQL 15, pgx (raw SQL), golang-migrate.
- Frontend: React 19, Vite 8, Tailwind 4, React Router 8.
- Infrastruktur: Docker Compose (db, seed, api, nginx web), GitHub Actions CI.

---

## Fondasi: Phase 1 sampai 9

| Phase | Isi | Commit |
|---|---|---|
| Dokumen | Spec desain, plan, TODO, SCRATCHPAD, CLAUDE.md, keputusan arsitektur dikunci | `7b7c143`..`5930023` |
| 1 | Loader konfigurasi, runner migrasi, compose `db` + `testdb` + `test` (race detector di container) | `df5c7dd` |
| 2 | Skema awal: users, wallets, ledger `wallet_transactions` append-only, items, auctions, bids, dengan CHECK di database | `8dd7a15` |
| 3 | Register/login (bcrypt + JWT), dompet dan deposit | `0eabbed` |
| 4 | Mesin lelang: escrow, anti-sniping, buy-now, urutan lock tetap (lelang lalu dompet) | `12662e4` |
| 5 | Worker penutup lelang (`SKIP LOCKED`), graceful shutdown | `91fd755` |
| 6 | Scraper wiki Fandom, katalog nyata, seeder idempoten | `9f0c814` |
| 7 | Pencarian AI (Gemini) dan katalog, lalu penyetelan dari uji nyata | `f635b16`, `440fcb9` |
| 8 | Swagger, rate limit per IP, CORS, log terstruktur | `69d8557` |
| 9 | Upload foto, frontend React (katalog, detail, jual, dompet, masuk) | `112e61b`, `14bab23` |

## Sesi QA (7 pemeriksa)

| Perubahan | Commit |
|---|---|
| Validasi field item dan batas uang Rp1 triliun agar tawaran tidak overflow (migrasi 000003) | `9c9002c` |
| Timeout server HTTP, rate limit AI search, tolak JWT_SECRET contoh | `059bc2d` |
| Filter katalog tetap ketat, normalisasi teks linear | `5118362` |
| Frontend: abaikan polling basi, cegah klik ganda dan listing ganda | `964c033` |
| Test race tutup lelang vs tawaran terakhir dibuat deterministik | `68c9ea3` |
| Scraper ikut pagination dan sampling merata; buy-now seed hanya di lot tanpa tawaran | `4ed8245` |
| Sinkron CLAUDE.md dan log QA | `059f1f4` |

## Desain dan merek

| Perubahan | Commit |
|---|---|
| Prinsip Apple: umpan balik saat ditekan, header buram (frosted), animasi harga berubah, validasi tawaran langsung | `8e6fe0a` |
| Kartu blister berwarna sesuai kemasan tiap merek, rak pegboard di beranda berisi lot yang segera berakhir, foto menyatu dengan gelembung | `8461b98` |
| Seeder menyelang-nyeling merek supaya katalog tidak berisi satu merek | `57cd17d` |
| Logo Blister64 (konsep "On the Peg"): master SVG, favicon, ikon aplikasi, panduan di `brand/README.md` | `ad623c6` |
| Membuka lot: kartu blister "diangkat dari peg" dan berubah menjadi pack di halaman detail (View Transition, router pindah ke `createBrowserRouter`), halaman baru mulai dari atas, reduced-motion tetap memberi umpan balik lewat warna dan opacity | `1bc6e7e` |

## Katalog dan merek

| Perubahan | Commit |
|---|---|
| Merek baru: Matchbox, Tomica, Tarmac Works, M2 Machines (total 7), lengkap dengan logo; `seeds.json` jadi 263 item | `b434008` |
| Kategori diorama: tanpa merek diecast, artis opsional, kondisi Baru/Bekas, ukuran, estimasi PO (migrasi 000004) | `c7036c4`, `2e8a21f` |

## Pengalaman menawar

| Perubahan | Commit |
|---|---|
| Riwayat tawaran dengan nama disamarkan (`GET /auctions/{id}/bids`) | `46e6ffc` |
| Menawar terasa seperti game: tombol cepat, stiker "MEMIMPIN!", alarm "Kamu disalip!" dengan tombol Balas, bonus "+2:00", timer berdetak; panel tawaran menempel di bawah layar HP; badge "Detik terakhir", label beli langsung, stiker nomor Tomica | `a6e6207` |

## Akun, profil, dan transaksi

| Perubahan | Commit |
|---|---|
| Halaman Profil: tab Menang, Sedang ditawar, Kalah, Dijual; tracker tawaran (tawaran saya vs tertinggi, Memimpin/Disalip, tombol Tawar lagi); konfirmasi sebelum keluar akun | `88ef932`, `c258534` |
| Riwayat dompet di halaman Dompet (isi saldo, dikunci, dikembalikan, pembayaran, hasil penjualan) | `88ef932`, `c258534` |
| Halaman penjual publik `/penjual/:username` dan tautan "Dijual oleh" di setiap lelang | `88ef932`, `c258534` |
| Chat penjual dan pemenang untuk mengatur pengiriman, hanya setelah lelang dimenangkan (migrasi 000005) | `88ef932`, `c258534` |
| Header tidak lagi bergeser antar halaman (`scrollbar-gutter`), kartu terangkat saat di-hover | `c258534` |

## Infrastruktur: Phase 10 (tanpa deploy)

| Perubahan | Commit |
|---|---|
| API membaca `X-Real-IP` hanya di belakang proxy tepercaya (`TRUST_PROXY`), supaya rate limit tetap per pengunjung | `76cfe93` |
| `Dockerfile.backend` (Alpine, 62 MB), `Dockerfile.frontend` (nginx, 75 MB), compose penuh: db, seed, api, web di port 8081 | `9767606` |
| GitHub Actions: gofmt, vet, Swagger mutakhir, test `-race` dengan Postgres, lint/test/build frontend, build image | `950562d` |
| Vite mem-proxy `/api` ke API, jadi port dev mana pun bisa terhubung (dulu gagal karena CORS saat Vite pindah ke 5174) | `aeb9d3e` |

## Sesi QA 2 (8 pemeriksa)

| Perubahan | Commit |
|---|---|
| Form jual menampilkan harga yang terbaca (`125.000` di field angka jadi Rp125), beli langsung wajib di atas harga awal | `05c1950` |
| Tracker profil: lelang yang waktunya habis tapi belum ditutup worker tidak lagi tampil "Kalah" bagi pemimpin | `7c07488` |
| Cancel ditolak (409) setelah `end_time` lewat, tombol batal disembunyikan | `bf51553` |
| Chat mengembalikan 500 pesan terbaru, bukan 500 terlama | `efefd1f` |
| Chat: poll basi tidak menyembunyikan pesan baru, tanpa duplikat | `5667c22` |
| Banner "Kamu disalip!" menampilkan tawaran tertinggi terkini | `710fdc2` |
| Rate limit bid, chat, dan upload memakai bucket terpisah | `48f89de` |
| Keamanan: Postgres hanya dipublish ke `127.0.0.1` (port Docker melewati `ufw` di Linux) | `12fd1cf` |
| Keamanan: `POSTGRES_PASSWORD` wajib tanpa default, seed demo hanya lewat `docker compose --profile demo up` | `c72ee91` |
| Keamanan: username unik tanpa peduli huruf besar/kecil (migrasi 000006), halaman penjual mencari tanpa peduli huruf | `fecc315` |
| Keamanan: kuota upload 20 foto per user dan 2 GiB untuk semua per 24 jam (migrasi 000007, 409 kalau penuh) | `df46e5d` |
| Keamanan: ID penawar dan pemenang tidak lagi dikirim; respons memakai `leading`, `won`, `mine`, `sold` relatif ke penonton | `549e17a` |
| Keamanan: HTTPS lewat Caddy (`--profile https`), HSTS, redirect HTTP, nginx realip supaya rate limit tetap per pengunjung | `5a28d75` |

---

## Referensi cepat

### Migrasi database

| No | Isi |
|---|---|
| 000001 | Skema awal dan semua CHECK uang/status |
| 000002 | `items.source_url` untuk atribusi CC-BY-SA |
| 000003 | Batas uang dan tahun |
| 000004 | Diorama: `category`, `artist`, `size`, `ready_weeks` |
| 000005 | Tabel `messages` untuk chat |
| 000006 | Username unik tanpa peduli huruf besar/kecil |
| 000007 | Tabel `uploads` untuk kuota foto |

### Endpoint API

| Endpoint | Akses |
|---|---|
| `POST /auth/register`, `POST /auth/login` | publik, rate limit |
| `GET /wallet`, `POST /wallet/deposit`, `GET /wallet/history` | login |
| `GET /me`, `GET /me/auctions` | login |
| `GET /auctions` (filter `category`, `brand`, `condition`, harga, `sort`, `q`), `POST /auctions/search/ai` | publik, token opsional untuk `leading`/`won` |
| `POST /auctions`, `POST /auctions/{id}/publish`, `/cancel`, `/buy-now`, `/bids` | login |
| `GET /auctions/{id}`, `GET /auctions/{id}/bids`, `GET /sellers/{username}` | publik, token opsional untuk `leading`/`won`/`mine` |
| `GET`/`POST /auctions/{id}/messages` | login, hanya penjual dan pemenang |
| `POST /uploads`, `GET /uploads/{nama}` | login untuk upload, 20 per hari |

### Akun demo

`demo_seller1`, `demo_seller2`, `demo_bidder1` sampai `demo_bidder3`, email `<nama>@demo.local`, password `demo-password`.

---

## Belum dikerjakan

- Deploy (hosting sedang dipilih: Oracle Always Free, DigitalOcean, atau Cloudflare Tunnel dari laptop).
- Notifikasi pesan chat yang belum dibaca.
- Scraper kadang memilih foto yang salah (poster promo M2, placeholder "image not available" Hot Wheels).
- Migrasi `down` 000004 sampai 000007 belum pernah dijalankan.
- Index trigram untuk pencarian, draf yang bisa dibaca siapa pun lewat ID.
- Dua listing diorama uji tanpa foto di database dev.
- `docker compose up --build` gagal sejak buildx 0.37.2 (di mesin dev: 0.38.1, Compose v5.6.1) dengan `additional privileges requested: --allow=fs.read=...`, juga untuk `Dockerfile.backend` yang ada di dalam context. Regresi upstream, bukan di repo ini. Workaround per perintah: `BUILDX_BAKE_ENTITLEMENTS_FS=0` (lihat CLAUDE.md). Hapus begitu rilis Compose dengan perbaikan izin baca Dockerfile keluar.
- Temuan QA 2 yang menunggu keputusan: pesan error mentah berbahasa Inggris, login tidak kembali ke halaman asal, countdown memakai jam klien, settlement gagal tanpa alert, field isi saldo punya jebakan titik ribuan yang sama.
