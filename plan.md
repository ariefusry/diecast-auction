# Architectural Plan: Scale 1:64 Diecast Auction Platform

Reference Spec: `docs/superpowers/specs/2026-10-24-hotwheels-auction-api-design.md` (keputusan final di Section 6)

---

## 1. Domain & Scope
Platform lelang online khusus koleksi diecast skala 1:64 multi-brand (Hot Wheels, Mini GT, Inno64). Menghubungkan sesama kolektor dalam ekosistem akun tunggal (bisa pasang lelang sekaligus menawar lelang orang lain, dilarang menawar lelang sendiri).

---

## 2. Tech Stack

| Layer | Technology | Key Responsibility |
|---|---|---|
| **Backend** | Go (stabil terbaru, minimum 1.22) + `net/http` ServeMux | REST API, middleware, graceful shutdown |
| **Database** | PostgreSQL 15+ via `pgx/v5` + `pgxpool` | ACID transactions, row-level locking (`SELECT ... FOR UPDATE`) |
| **Migrations** | `golang-migrate` (library + `embed`) | Migrasi SQL berversi, otomatis saat API start |
| **API Docs** | Swagger / OpenAPI (`swag`) | Interactive API docs at `/swagger/index.html` |
| **Rate Limit** | `golang.org/x/time/rate` | Token bucket per IP di endpoint bid dan auth |
| **AI Integration**| Gemini Flash / Groq LLM | Natural language search parsing into structured JSON filters |
| **Frontend** | React + Vite + Tailwind CSS | Responsive SPA for catalog, countdown, bid box, wallet UI |
| **DevOps** | Docker multi-stage & Docker Compose | Isolated multi-container environment (API, DB, UI, test) |
| **Deployment** | Linux VPS generik + Docker Compose | Provider dipilih di fase deployment, CI via GitHub Actions |

---

## 3. Core Architectural Decisions

### 3.1 Financial Ledger & Virtual Wallet Escrow
- Setiap user memiliki akun dompet (`wallets`) berisi `available_balance` dan `locked_balance`, bertipe `BIGINT` rupiah dengan `CHECK (>= 0)`.
- Saat bid dipasang: validasi saldo cukup, pindahkan `bid_amount` dari available ke locked.
- Saat tertimpa penawar lain (outbid): dana penawar lama otomatis dikembalikan dari locked ke available dalam transaksi atomik yang sama.
- Setiap mutasi saldo menulis baris `wallet_transactions` (deposit, lock, release, settle, buy_now) dalam transaksi yang sama.
- Urutan lock: row lelang dulu, lalu row dompet dengan `user_id` naik, untuk mencegah database deadlock.

### 3.2 Bidding, Anti-Sniping & Lifecycle Engine
- Validasi bid: lelang `active`, `now()` DB `< end_time`, bid pertama `>= start_price`, bid berikutnya `>= current_highest_bid + min_increment`, penawar bukan penjual dan bukan penawar tertinggi saat ini.
- Waktu selalu dari `now()` PostgreSQL di dalam transaksi, bukan jam aplikasi.
- Jika tawaran valid masuk di 2 menit terakhir sebelum `end_time`, `end_time = end_time + 2 menit` di transaksi yang sama.
- Publish: draf menyimpan `duration`, publish menetapkan `start_time = now()` dan `end_time = now() + duration`.
- Background worker internal (`time.Ticker`) memindai lelang aktif yang melewati `end_time` dengan `FOR UPDATE SKIP LOCKED`, menetapkan pemenang, dan memindahkan dana terkunci ke dompet penjual.
- Lelang tanpa penawar bertransisi ke `completed` dengan `winner_id = NULL`.
- Cancel dan buy-now hanya diizinkan saat belum ada bid (`highest_bidder_id IS NULL`). Buy-now langsung menuntaskan lelang tanpa jalur refund.

### 3.3 Data Scraping & Auto-Seeding Pipeline
- `data/seeds.json` adalah kontrak data dan di-commit. Sampai fase scraper, isinya file kecil yang ditulis manual.
- CLI terpisah `cmd/scraper` (dijalankan sekali) menarik puluhan model asli lewat MediaWiki API (`api.php`) di hotwheels.fandom.com, minigt.fandom.com, dan inno64.fandom.com ke `data/seeds.json`. Konten berlisensi CC-BY-SA, jadi URL halaman sumber disimpan sebagai atribusi. Inno64 dilengkapi manual karena wikinya tipis.
- CLI `cmd/seed` menyuntikkan katalog, akun demo penjual/penawar, dan saldo awal. Bid simulasi dibuat lewat service bid agar invariant dompet dan ledger tetap terjaga.

### 3.4 AI Natural Language Search
- Endpoint `POST /auctions/search/ai` menerima query bebas pengguna (contoh: "carikan minigt nissan gtr merah di bawah 200k").
- LLM memetakan input ke JSON skema terstruktur (brand, model, series, color, tags, condition, min_price, max_price, sort).
- Backend Go memvalidasi struct dan mengeksekusi parameterized query SQL ke PostgreSQL dengan mekanisme fallback jika hasil query kosong.

### 3.5 Frontend
- JWT dikirim sebagai Bearer token.
- Halaman detail lelang polling sekitar 5 detik agar perpanjangan anti-sniping terlihat di countdown (tanpa WebSocket di MVP).

---

## 4. Repository Structure

Tidak ada lapisan repository terpisah: service menjalankan SQL langsung agar operasi multi-row bisa memegang transaksi dan urutan lock sendiri.

```
LelangHotwheels/
├── cmd/
│   ├── api/          # Entrypoint REST API server (menjalankan migrasi embed saat start)
│   ├── scraper/      # CLI data scraping diecast wiki
│   └── seed/         # CLI database seeder
├── internal/
│   ├── config/       # Environment configuration (12-Factor)
│   ├── handler/      # HTTP handlers & ServeMux routes
│   ├── middleware/   # Auth JWT, Rate limit, CORS, Logging
│   ├── model/        # Domain entities & structs
│   ├── service/      # Business logic + raw SQL via pgx (auth, wallet, auction, bidding)
│   ├── testutil/     # Helper test integrasi (pool testdb, data unik)
│   └── worker/       # Background expiration ticker
├── migrations/       # SQL migration files (di-embed ke binary)
├── data/             # seeds.json (di-commit)
├── frontend/         # React + Vite + Tailwind CSS SPA
├── docs/             # Specs and architecture plans
├── docker-compose.yml  # db, api, frontend, test (profile test untuk go test -race)
├── Dockerfile.backend
└── Dockerfile.frontend
```
