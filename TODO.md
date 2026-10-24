# Project Roadmap & Task Checklist

Reference Architecture: `plan.md`  
Reference Spec: `docs/superpowers/specs/2026-10-24-hotwheels-auction-api-design.md`

---

## Phase 1: Environment & Project Scaffolding
- [ ] Inisialisasi Go module (`go mod init github.com/ariefusry/lelang-hotwheels`)
- [ ] Buat file konfigurasi `.env.example` dan loader konfigurasi di `internal/config/config.go`
- [ ] Buat `docker-compose.yml` untuk PostgreSQL 15 dengan healthcheck
- [ ] Siapkan tool migrasi `golang-migrate` dan script runner

## Phase 2: Database Schema & Migrations
- [ ] Buat migrasi SQL tabel `users` (id, email, password_hash, username, created_at)
- [ ] Buat migrasi SQL tabel `wallets` (id, user_id, available_balance, locked_balance)
- [ ] Buat migrasi SQL tabel `items` (id, seller_id, brand, model_name, series, year, scale, condition, image_url, description)
- [ ] Buat migrasi SQL tabel `auctions` (id, item_id, start_price, min_increment, buy_now_price, current_highest_bid, highest_bidder_id, status, start_time, end_time, winner_id)
- [ ] Buat migrasi SQL tabel `bids` (id, auction_id, bidder_id, amount, created_at)
- [ ] Terapkan index untuk `auctions(status, end_time)`, `bids(auction_id, amount DESC)`, dan foreign keys

## Phase 3: Data Scraper & Database Seeder
- [ ] Buat CLI scraper di `cmd/scraper/main.go` untuk menarik data Hot Wheels, Mini GT, dan Inno64
- [ ] Generate file data riil ke `data/seeds.json`
- [ ] Buat script seeder di `cmd/seed/main.go` yang membaca `seeds.json` dan mengisi database dengan akun demo serta katalog

## Phase 4: Authentication & Virtual Wallet APIs
- [ ] Implementasi register, login dengan bcrypt & JWT di `internal/service/auth.go`
- [ ] Buat auth middleware untuk proteksi endpoint di `internal/middleware/auth.go`
- [ ] Implementasi endpoint `GET /wallet` dan `POST /wallet/deposit`
- [ ] Unit test transaksi deposit dan kalkulasi saldo dompet

## Phase 5: Auction Engine, Anti-Sniping & Bidding Logic
- [ ] Implementasi CRUD lelang: buat draf, publikasikan (`POST /auctions/{id}/publish`), batalkan jika belum ada bid
- [ ] Implementasi transaksi bid atomik dengan `SELECT ... FOR UPDATE` di PostgreSQL
- [ ] Validasi bid: lelang aktif, waktu belum habis, penawar bukan penjual, saldo mencukupi
- [ ] Mekanisme escrow: kunci saldo penawar baru, lepas saldo penawar lama dalam satu transaksi
- [ ] Implementasi Anti-Sniping: jika bid masuk di 2 menit terakhir, perpanjang `end_time` otomatis 2 menit
- [ ] Implementasi endpoint `POST /auctions/{id}/buy-now` untuk pembelian instan
- [ ] Unit & concurrency test untuk membuktikan anti-race-condition pada saat tawaran bersamaan

## Phase 6: Background Expiration Worker & Graceful Shutdown
- [ ] Buat background worker di `internal/worker/auction_worker.go` dengan `time.Ticker`
- [ ] Query dan update atomik untuk menutup lelang yang melewati `end_time`
- [ ] Pindahkan dana terkunci pemenang ke saldo aktif penjual
- [ ] Graceful shutdown pada `cmd/api/main.go` untuk menangani SIGINT/SIGTERM

## Phase 7: AI Natural Language Search
- [ ] Integrasi client LLM (Gemini Flash atau Groq) dengan format response JSON terstruktur
- [ ] Implementasi parser kalimat query ke parameter struct pencarian
- [ ] Buat dynamic query builder SQL aman dengan parameterized filter dan logika fallback
- [ ] Implementasi endpoint `POST /auctions/search/ai`

## Phase 8: Swagger Documentation & Middleware Quality
- [ ] Anotasi Swagger OpenAPI pada semua handler API
- [ ] Setup route `/swagger/index.html` via `http-swagger`
- [ ] Middleware rate limiting token bucket di endpoint bid dan auth
- [ ] Middleware CORS, structured logging, dan error handler seragam

## Phase 9: React Frontend (SPA)
- [ ] Inisialisasi frontend dengan Vite + React + Tailwind CSS di folder `frontend/`
- [ ] Halaman login dan register dengan penyimpanan token JWT
- [ ] Halaman katalog dengan filter brand (Hot Wheels, Mini GT, Inno64), kondisi, dan AI search bar
- [ ] Halaman detail lelang dengan hitung mundur waktu riil dan form penawaran harga
- [ ] Halaman dompet virtual untuk melihat saldo dan top-up simulasi
- [ ] Halaman buat lelang baru beserta upload gambar

## Phase 10: Docker Production & Deployment
- [ ] Buat multi-stage `Dockerfile.backend` (Go build -> Alpine runner)
- [ ] Buat multi-stage `Dockerfile.frontend` (Node build -> Nginx server)
- [ ] Konfigurasi `docker-compose.yml` lengkap (db, backend, frontend) siap jalan dengan satu perintah
- [ ] Siapkan GitHub Actions CI workflow untuk linting dan testing otomatis
