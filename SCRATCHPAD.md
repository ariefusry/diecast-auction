# Project Scratchpad & Session Notes

## Current Status
- Project Name: Scale 1:64 Diecast Auction Platform (Hot Wheels, Mini GT, Inno64)
- Phase: Keputusan arsitektur dikunci (spec Section 6)
- Active Focus: Phase 1 selesai, siap Phase 2 (schema & migrations)

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
