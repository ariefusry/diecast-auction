# Project Roadmap & Task Checklist

Reference Architecture: `plan.md`  
Reference Spec: `docs/superpowers/specs/2026-10-24-hotwheels-auction-api-design.md`

Urutan fase sengaja menaruh inti paling berisiko (dompet, bid, worker) sebelum scraper.

---

## Phase 1: Environment & Project Scaffolding
- [x] Upgrade Go lokal ke versi stabil terbaru (1.27.0, dari 1.19.1)
- [x] Buat repo GitHub `ariefusry/diecast-auction` (private) dan set git remote
- [x] Inisialisasi Go module (`go mod init github.com/ariefusry/diecast-auction`), kunci versi Go di `go.mod` (1.27.0)
- [x] Buat file konfigurasi `.env.example` dan loader konfigurasi di `internal/config/config.go`
- [x] Buat `docker-compose.yml` untuk PostgreSQL 15 dengan healthcheck
- [x] Tambah service compose `test` (profile `test`, image `golang:1.27`, `CGO_ENABLED=1`) untuk `go test -race ./...` terhadap service `testdb` (tmpfs, terpisah dari data dev)
- [x] Siapkan runner migrasi `golang-migrate` di `internal/db/migrate.go` (`Migrate(url, fs.FS)`, idempoten)

## Phase 2: Database Schema & Migrations
Semua tabel ada di satu migrasi `migrations/000001_init_schema.{up,down}.sql`, constraint diuji di `internal/db/schema_test.go`.
- [x] Buat `migrations/embed.go` (`//go:embed *.sql`) bersama migrasi pertama, lalu panggil `db.Migrate` saat start di `cmd/api/main.go` (plus `GET /healthz`)
- [x] Tabel `users` (id, email unik case-insensitive via `lower(email)`, password_hash, username unik, created_at)
- [x] Tabel `wallets` (user_id sebagai PK, available_balance BIGINT, locked_balance BIGINT, CHECK keduanya >= 0)
- [x] Tabel `wallet_transactions` append-only via trigger (id, user_id, type, amount BIGINT > 0, auction_id NULL, created_at), type: deposit, lock, release, settle, buy_now
- [x] Tabel `items` (id, seller_id, brand, model_name, series, year, color, tags TEXT[], condition CHECK, image_url, description)
- [x] Tabel `auctions` (id, item_id, start_price, min_increment, buy_now_price NULL > start_price, current_highest_bid, highest_bidder_id NULL berpasangan via CHECK, status CHECK, duration, start_time NULL, end_time NULL, winner_id NULL), semua uang BIGINT
- [x] Tabel `bids` (id, auction_id, bidder_id, amount BIGINT > 0, created_at)
- [x] Index `auctions(status, end_time)`, `bids(auction_id, amount DESC)`, `items(seller_id)`, `wallet_transactions(user_id)`, dan foreign keys

## Phase 3: Authentication & Virtual Wallet APIs
- [ ] Implementasi register, login dengan bcrypt & JWT di `internal/service/auth.go`
- [ ] Buat auth middleware untuk proteksi endpoint di `internal/middleware/auth.go`
- [ ] Implementasi endpoint `GET /wallet` dan `POST /wallet/deposit` (tulis ledger `deposit`)
- [ ] Test transaksi deposit, kalkulasi saldo, dan baris ledger

## Phase 4: Auction Engine, Anti-Sniping & Bidding Logic
- [ ] CRUD lelang: buat draf (simpan `duration`), publish (`start_time = now()`, `end_time = now() + duration`), cancel hanya jika `highest_bidder_id IS NULL`
- [ ] Transaksi bid atomik: lock row lelang dulu, lalu dompet dengan `user_id` naik
- [ ] Validasi bid: lelang aktif, `now()` DB `< end_time`, bid pertama `>= start_price`, berikutnya `>= current_highest_bid + min_increment`, bukan penjual, bukan penawar tertinggi, saldo cukup
- [ ] Escrow: kunci saldo penawar baru, lepas saldo penawar lama, tulis ledger `lock`/`release`, semua dalam satu transaksi
- [ ] Anti-sniping: bid di 2 menit terakhir membuat `end_time = end_time + 2 menit`
- [ ] Endpoint `POST /auctions/{id}/buy-now`, hanya saat `highest_bidder_id IS NULL`, tanpa refund, ledger `buy_now`
- [ ] Concurrency test (via service compose `test` dengan `-race`) untuk membuktikan tidak ada race condition dan saldo tidak pernah negatif

## Phase 5: Background Expiration Worker & Graceful Shutdown
- [ ] Buat background worker di `internal/worker/auction_worker.go` dengan `time.Ticker`
- [ ] Tutup lelang yang melewati `end_time` dengan `FOR UPDATE SKIP LOCKED`
- [ ] Pindahkan dana terkunci pemenang ke saldo aktif penjual (ledger `settle`)
- [ ] Graceful shutdown pada `cmd/api/main.go` untuk menangani SIGINT/SIGTERM

## Phase 6: Seeder & Data Scraper
- [ ] Tulis `data/seeds.json` kecil secara manual (beberapa item per brand) sebagai kontrak data
- [ ] Buat seeder `cmd/seed/main.go`: akun demo, saldo awal, katalog, bid simulasi lewat service bid
- [ ] Buat CLI scraper `cmd/scraper/main.go` via MediaWiki API (`api.php`) di hotwheels/minigt/inno64.fandom.com, rate limit sopan + User-Agent jelas
- [ ] Simpan atribusi CC-BY-SA (URL halaman sumber) per item, lengkapi Inno64 secara manual (wiki-nya tipis), lalu commit `data/seeds.json`

## Phase 7: AI Natural Language Search
- [ ] Integrasi client LLM (Gemini Flash atau Groq) dengan format response JSON terstruktur
- [ ] Parser kalimat query ke struct filter (brand, series, color, tags, condition, harga, sort)
- [ ] Dynamic query builder SQL aman dengan parameterized filter dan logika fallback
- [ ] Implementasi endpoint `POST /auctions/search/ai`

## Phase 8: Swagger Documentation & Middleware Quality
- [ ] Anotasi Swagger OpenAPI pada semua handler API
- [ ] Setup route `/swagger/index.html` via `http-swagger`
- [ ] Middleware rate limiting per IP (`golang.org/x/time/rate`) di endpoint bid dan auth
- [ ] Middleware CORS, structured logging, dan error handler seragam

## Phase 9: React Frontend (SPA)
- [ ] Inisialisasi frontend dengan Vite + React + Tailwind CSS di folder `frontend/`
- [ ] Halaman login dan register, JWT dikirim sebagai Bearer token
- [ ] Halaman katalog dengan filter brand (Hot Wheels, Mini GT, Inno64), kondisi, dan AI search bar
- [ ] Halaman detail lelang dengan countdown, polling sekitar 5 detik, dan form penawaran
- [ ] Halaman dompet virtual untuk melihat saldo dan top-up simulasi
- [ ] Halaman buat lelang baru beserta upload gambar

## Phase 10: Docker Production & Deployment
- [ ] Buat multi-stage `Dockerfile.backend` (Go build -> Alpine runner)
- [ ] Buat multi-stage `Dockerfile.frontend` (Node build -> Nginx server)
- [ ] Konfigurasi `docker-compose.yml` lengkap (db, backend, frontend) siap jalan dengan satu perintah
- [ ] Siapkan GitHub Actions CI workflow untuk linting dan testing otomatis
- [ ] Pilih provider Linux VPS dan deploy via Docker Compose (volume untuk `uploads` dan data Postgres)
