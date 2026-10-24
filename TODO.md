# Project Roadmap & Task Checklist

Reference Architecture: `plan.md`  
Reference Spec: `docs/superpowers/specs/2026-10-24-hotwheels-auction-api-design.md`

Urutan fase sengaja menaruh inti paling berisiko (dompet, bid, worker) sebelum scraper.

---

## Phase 1: Environment & Project Scaffolding
- [ ] Upgrade Go lokal ke versi stabil terbaru (lokal masih 1.19.1)
- [ ] Buat repo GitHub `ariefusry/diecast-auction` dan set git remote
- [ ] Inisialisasi Go module (`go mod init github.com/ariefusry/diecast-auction`), kunci versi Go di `go.mod`
- [ ] Buat file konfigurasi `.env.example` dan loader konfigurasi di `internal/config/config.go`
- [ ] Buat `docker-compose.yml` untuk PostgreSQL 15 dengan healthcheck
- [ ] Tambah service compose `test` (profile `test`, image `golang` sesuai versi `go.mod`, `CGO_ENABLED=1`) untuk `go test -race ./...` terhadap service `db`
- [ ] Siapkan migrasi `golang-migrate` (library + `embed`) yang dijalankan saat API start

## Phase 2: Database Schema & Migrations
- [ ] Tabel `users` (id, email, password_hash, username, created_at)
- [ ] Tabel `wallets` (id, user_id UNIQUE, available_balance BIGINT, locked_balance BIGINT, CHECK keduanya >= 0)
- [ ] Tabel `wallet_transactions` append-only (id, wallet_id, type, amount BIGINT, auction_id NULL, created_at), type: deposit, lock, release, settle, buy_now
- [ ] Tabel `items` (id, seller_id, brand, model_name, series, year, color, tags TEXT[], condition, image_url, description)
- [ ] Tabel `auctions` (id, item_id, start_price, min_increment, buy_now_price NULL, current_highest_bid, highest_bidder_id NULL, status, duration, start_time NULL, end_time NULL, winner_id NULL), semua uang BIGINT
- [ ] Tabel `bids` (id, auction_id, bidder_id, amount BIGINT, created_at)
- [ ] Index `auctions(status, end_time)`, `bids(auction_id, amount DESC)`, dan foreign keys

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
