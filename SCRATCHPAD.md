# Project Scratchpad & Session Notes

## Current Status
- Project Name: Scale 1:64 Diecast Auction Platform (Hot Wheels, Mini GT, Inno64)
- Phase: Keputusan arsitektur dikunci (spec Section 6)
- Active Focus: Phase 5 selesai, siap Phase 6 (seeder & scraper)

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
