# Project Scratchpad & Session Notes

## Current Status
- Project Name: Scale 1:64 Diecast Auction Platform (Hot Wheels, Mini GT, Inno64)
- Phase: Keputusan arsitektur dikunci (spec Section 6)
- Active Focus: Phase 9 selesai, siap Phase 10 (Docker production & deployment)

---

## Key Context & Constraints
- Language & Runtime: Go stabil terbaru, minimum 1.22 (Root repo)
- Frontend: React + Vite + Tailwind CSS (`/frontend`)
- Database: PostgreSQL 15 via Docker Compose
- Deployment: Linux VPS generik + Docker Compose
- Design Spec: `docs/superpowers/specs/2026-10-24-hotwheels-auction-api-design.md`
- Roadmap: `TODO.md`
- Architecture: `plan.md`

## Environment Notes (mesin dev Windows)
- Go lokal sudah 1.27.0 (upgrade dari 1.19.1 via winget, 2026-10-25).
- `gcc` tidak terpasang, jadi `go test -race` dijalankan di container.
- Remote: `github.com/ariefusry/diecast-auction` (private).

## Open Items
- Provider VPS dipilih di Phase 10.
- Phase 10: rate limit memakai `RemoteAddr`. Di belakang reverse proxy (nginx) semua request terlihat dari IP proxy, jadi baca `X-Real-IP` dari proxy tepercaya.
- Phase 10: CI sebaiknya menjalankan `swag init` lalu `git diff --exit-code internal/apidocs` agar dokumentasi tidak basi.

---

## Helpful Commands
```bash
# Start all local containers (Database, API, Frontend)
docker compose up --build

# Run backend tests with race detector (service test dibuat di Phase 1)
docker compose --profile test run --rm test

# Quick local tests without race detector
go test ./...

# Run real data scraper
go run cmd/scraper/main.go

# Run database seeder
go run cmd/seed/main.go

# Start frontend development server
cd frontend && npm install && npm run dev
```
Migrasi tidak punya perintah terpisah: di-embed dan dijalankan saat API start.

---

## Session Log & Decision Notes
- 2026-10-24: Inisialisasi spesifikasi arsitektur platform lelang diecast 1:64 multi-brand (Hot Wheels, Mini GT, Inno64).
- 2026-10-24: Menetapkan model dompet virtual dengan sistem escrow, anti-sniping 2 menit, AI natural language search, Go scraper CLI, dan fullstack Docker Compose.
- 2026-10-24: Menyiapkan `plan.md`, `TODO.md`, dan `SCRATCHPAD.md` agar siap dieksekusi secara terstruktur oleh Claude Code CLI atau developer.
- 2026-10-25: Review dokumen. Keputusan user: upgrade Go + race test di Docker, buy-now hanya saat 0 bid, tambah ledger `wallet_transactions`, deploy ke VPS generik. Heroku dicoret (dyno tidur menghentikan worker, filesystem ephemeral). Kredit DigitalOcean di Student Pack dilaporkan berakhir 2026-07-31.
- 2026-10-25: Module path `github.com/ariefusry/diecast-auction`. Sumber scraper: MediaWiki API Fandom (hotwheels ~wiki besar, minigt 699 artikel, inno64 hanya 24 artikel), lisensi CC-BY-SA. Halaman HTML Fandom diblok Cloudflare, API tidak.
- 2026-10-25: Default builder dikunci di spec Section 6: uang BIGINT + CHECK, lock lelang lalu dompet `user_id` naik, waktu dari `now()` DB, `SKIP LOCKED` di worker, migrasi embed, items `color`/`tags` tanpa `scale`, polling frontend. Fase diurutkan ulang: dompet, bid, worker sebelum seeder dan scraper.
- 2026-10-25: Phase 1 selesai. `internal/config` (DATABASE_URL wajib, PORT default 8080), `internal/db.Migrate` (golang-migrate + pgx5, idempoten), compose `db` + `testdb` (tmpfs) + `test` (golang:1.27, -race). Test DB di-skip jika `TEST_DATABASE_URL` kosong. Embed migrasi dan wiring `cmd/api` ditunda ke Phase 2 karena `//go:embed *.sql` butuh minimal satu file SQL.
- 2026-10-25: Phase 2 selesai. Satu migrasi `000001_init_schema` (6 tabel). `wallets` memakai `user_id` sebagai PK (lebih sederhana dari rencana `id` + `user_id UNIQUE`, cocok dengan aturan lock), ledger merujuk `user_id`. Ledger append-only ditegakkan trigger. Test skema hanya menerima SQLSTATE 23xxx/P0001, dan sudah dibuktikan dengan uji mutasi (hapus CHECK + trigger lalu test gagal). Catatan: `testdb` persisten antar run, jadi edit migrasi lama butuh `down` + `-count=1`.
- 2026-10-25: Phase 3 selesai. Auth (bcrypt, JWT HS256 dikunci via WithValidMethods, dummy hash untuk login email tak dikenal), wallet Get/Deposit + ledger, middleware Bearer, handler dengan envelope `{"error": ...}` dan 500 generik. Tidak ada package repository: service menjalankan SQL langsung. Regresi ditemukan: fixture skema Phase 2 memakai ID tetap 1/2 yang bentrok dengan user dari package lain, diperbaiki dengan ID negatif (aturan CLAUDE.md #10).
- 2026-10-25: Phase 4 selesai. `service.Auctions` (Create, Get, Publish, Cancel, Bid, BuyNow), semua mutasi lewat `mutate`: lock row lelang `FOR UPDATE OF a`, lalu `lockWallets` (ORDER BY user_id FOR UPDATE). Endpoint: `POST /auctions` (item + draf sekaligus, `duration_minutes` 1-43200), `GET /auctions/{id}` publik, `POST /auctions/{id}/{publish,cancel,bids,buy-now}`. `ErrForbidden` (403) baru, teks `ErrConflict` jadi "conflict" (409 untuk semua pelanggaran state, termasuk bid terlalu rendah dan saldo kurang). Keputusan ledger: untuk `buy_now` dan `settle`, penjual lelang dikredit dan user lain didebit (aman karena penjual tidak bisa bid/beli lelangnya sendiri); replay ada di `testutil.AssertLedger` untuk dipakai ulang di Phase 5. Buy-now set `end_time = now()`. Test race: banyak bidder di dua lelang, buy-now vs bid pertama, buy-now ganda, cancel vs bid pertama; uji mutasi (hapus release, hapus lock lelang, hapus anti-sniping) masing-masing membuat test gagal. Belum ada `GET /auctions` list (katalog), ditunda ke Phase 7/9.
- 2026-10-25: Phase 5 selesai. `service.Auctions.CloseExpired`: satu transaksi per lelang, `FOR UPDATE OF a SKIP LOCKED`, settle pemenang (ledger `settle` untuk pemenang dari locked dan penjual ke available), tanpa bid jadi `completed` dengan `winner_id NULL`. Lelang yang gagal settle di-rollback dan dilewati sisa pass (`id <> ALL($1)`, slice tidak boleh nil karena pgx mengirim NULL), errornya dikembalikan. Ini ditemukan saat uji manual: data rusak sisa uji mutasi memblokir semua settlement karena `ORDER BY end_time LIMIT 1` selalu memilih lelang rusak yang sama. `internal/worker.Run` hanya ticker 1 detik yang memanggil fungsi close. `cmd/api`: `signal.NotifyContext` SIGINT/SIGTERM, `http.Server.Shutdown` 10 detik, tunggu worker selesai. Pembatalan di tengah pass hanya me-rollback lelang yang sedang diproses. Diverifikasi: binary di container exit 0 setelah SIGTERM. Catatan: uji mutasi bisa meninggalkan data rusak di testdb, jalankan `docker compose --profile test down` sesudahnya.
- 2026-10-25: Phase 6 selesai. Scraper: satu item per halaman casting (HW `Category:JDM Cars` disampling merata, Mini GT `Category:Models`, Inno64 semua halaman), foto dari `pageimages`, tahun/seri dari infobox, warna dari baris pertama tabel versi (kolom Color/Livery, header = baris pertama yang punya sel), fallback warna dari kata di judul, tag = merek (alias seperti Skyline->nissan) + `jdm`. Hasil 103 item (HW 40, Mini GT 40, Inno64 23). Halaman Mini GT kebanyakan hanya galeri foto, jadi tahun kosong. `items.source_url` (migrasi 000002) menyimpan atribusi CC-BY-SA. PENTING untuk Phase 9: CDN Fandom (static.wikia.nocookie.net) membalas 404 jika Referer dari domain lain, pakai `<img referrerpolicy="no-referrer">`. Seeder: akun `demo_seller1-2`, `demo_bidder1-3` (password `demo-password`, Rp20.000.000 per bidder), harga dasar per brand, kondisi/durasi/buy-now bergantung indeks, tiap item ke-10 tetap draf, bid lewat `Auctions.Bid`. Idempoten: no-op jika `demo_seller1` sudah ada (kalau seeding gagal di tengah, reset DB lalu ulangi).
- 2026-10-25: Phase 7 selesai. Provider Gemini (`internal/llm`), model default `gemini-3.8-flash` (dicek dari docs, bisa diganti via `GEMINI_MODEL`), REST `generationConfig.responseFormat.text` dengan `mimeType: APPLICATION_JSON` + schema berenum untuk brand/condition/sort. Prompt ada di bagian `contents` (tidak memakai systemInstruction). `service.Filter.Normalize` membuang nilai di luar allowlist, `Auctions.Search` menyusun SQL berparameter (ORDER BY dari map allowlist, LIKE di-escape), hanya lelang `active`, harga = `coalesce(current_highest_bid, start_price)`. Fallback: strict, lalu buang color/series/tags/condition, lalu buang harga; brand dan kata kunci selalu dipertahankan, respons memberi `relaxed`. Tanpa key atau saat LLM error, query utuh jadi pencarian kata kunci (respons `ai: false`). Belum diuji ke Gemini sungguhan karena belum ada API key; client diuji dengan server palsu.
- 2026-10-25: Phase 7 diuji ke Gemini sungguhan (key di `.env`, gitignored; API tidak membaca `.env` sendiri, jadi export dulu). `gemini-3.8-flash` membalas 429 quota exceeded untuk key free tier ini, `gemini-2.5-flash-lite` 404 (tidak tersedia untuk user baru), `gemini-3.5-flash-lite` stabil 1-2 detik, jadi itu default baru. Perbaikan dari uji nyata: (1) teks dan kata kunci dibandingkan hanya huruf+angka (`gtr` cocok `GT-R`), (2) `text` dari LLM hanya boleh berisi kata yang memang diketik user (model sempat menulis prosa), (3) relaksasi terakhir: cukup satu kata (>= 3 huruf) cocok, supaya kalimat utuh pada fallback tanpa LLM tetap memberi hasil. Prompt injection ("abaikan instruksi...") hanya menghasilkan filter kosong, yaitu katalog publik.
- 2026-10-25: Phase 8 selesai. `middleware.RateLimit` (token bucket per IP, `x/time/rate`, bucket idle > 10 menit dibersihkan, 429 + Retry-After): auth 1 per 6 detik burst 10 (register+login berbagi bucket), bid 2/detik burst 5. `middleware.CORS` dari `CORS_ORIGINS` (default Vite `http://localhost:5173`). `middleware.Log`: slog JSON satu baris per request, panic jadi JSON 500. `slog.SetDefault` juga menyalurkan `log.Printf` lama. Route tak dikenal: JSON 404 lewat catch-all `/` (method salah pada path yang ada juga jadi 404, bukan 405). Swagger: anotasi swaggo di semua handler, request/response bertipe nama, hasil generate di `internal/apidocs` (di-commit), UI di `/swagger/index.html`. Binary `swag` lokal melaporkan v1.16.4 meski install v1.16.6.
- 2026-10-25: Phase 9 selesai. Backend: `POST /uploads` (multipart `image`, 5 MiB, tipe dari sniffing byte: JPEG/PNG/WebP, nama acak `rand.Text()`), `GET /uploads/{name}` tanpa listing direktori, `UPLOAD_DIR`. Frontend `frontend/`: Vite 8 + React 19 + TypeScript + Tailwind 4 (`@tailwindcss/vite`) + React Router 8 (mode deklaratif, `react-router-dom` sudah tidak ada di v8) + Vitest 5. Arah desain: kartu lelang berbentuk blister card (lubang gantungan + gelembung plastik) sebagai satu-satunya elemen dekoratif; palet card-white #EEF1F5, navy #14213D, oranye track #FF6A13 (aksi bid), merah #D62828 (jendela anti-sniping), mint #2A9D8F (memimpin); font Barlow Condensed + Barlow. UI berbahasa Indonesia. Token JWT di localStorage, 401 memicu event `auth:expired`. Detail lelang polling 5 detik. Foto Fandom memakai `referrerPolicy="no-referrer"`. Diuji di browser (Playwright): katalog, AI search, login, bid, jual dengan upload, batal, dompet, lebar 390px; konsol bersih. Sisa peringatan lint: fast-refresh di main.tsx/auth.tsx (khusus dev) dan false positive set-state-in-effect di AuctionDetail.
- 2026-10-25: QA swarm (7 explorer Haiku read-only, verifikasi dan perbaikan oleh Opus). Diperbaiki: (1) overflow int64 `minBid` lewat `min_increment` raksasa: `MaxMoney` Rp1 triliun di Create/Bid + migrasi 000003 (CHECK batas uang dan tahun 1950-2100); (2) validasi item: brand dari daftar, panjang teks, tanpa NUL, tag lowercase maks 10, `image_url` hanya http(s) atau `/uploads/<nama>`, `source_url` hanya http(s) (React 19 sudah memblok href `javascript:`, tapi data tetap dibersihkan di server); (3) `http.Server` timeout; (4) rate limit AI search 1 per 3 detik burst 5; (5) JWT_SECRET placeholder `.env.example` ditolak; (6) filter katalog eksplisit tidak lagi direlaksasi diam-diam (hanya pencarian teks/AI); (7) `Normalize` linear + buang NUL; (8) frontend: poll basi tidak menimpa hasil bid, tombol isi saldo anti klik ganda, gagal tayang tidak membuat listing ganda, salah password di /masuk tidak menghapus sesi, input bid `step=any`; (9) test race close vs bid deterministik dan wajib melihat kedua hasil; (10) scraper mengikuti `continue` MediaWiki, sampling merata setelah filter, buang halaman non-casting; seeder buy-now hanya di lot tanpa bid; `seeds.json` di-scrape ulang. Sengaja tidak diubah (keputusan user): draf bisa dibaca siapa pun lewat ID, kuota upload per user, index trigram untuk pencarian, password demo statis.
