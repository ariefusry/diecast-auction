# Architectural Plan: Scale 1:64 Diecast Auction Platform

Reference Spec: `docs/superpowers/specs/2026-10-24-hotwheels-auction-api-design.md`

---

## 1. Domain & Scope
Platform lelang online khusus koleksi diecast skala 1:64 multi-brand (Hot Wheels, Mini GT, Inno64). Menghubungkan sesama kolektor dalam ekosistem akun tunggal (bisa pasang lelang sekaligus menawar lelang orang lain, dilarang menawar lelang sendiri).

---

## 2. Tech Stack

| Layer | Technology | Key Responsibility |
|---|---|---|
| **Backend** | Go (1.19+) + Chi router | High performance REST API, middleware, graceful shutdown |
| **Database** | PostgreSQL 15+ via `pgx` | ACID transactions, row-level locking (`SELECT ... FOR UPDATE`) |
| **Migrations** | `golang-migrate` | Version-controlled SQL migrations |
| **API Docs** | Swagger / OpenAPI (`swag`) | Interactive API docs at `/swagger/index.html` |
| **AI Integration**| Gemini Flash / Groq LLM | Natural language search parsing into structured JSON filters |
| **Frontend** | React + Vite + Tailwind CSS | Responsive SPA for catalog, countdown, bid box, wallet UI |
| **DevOps** | Docker multi-stage & Docker Compose | Isolated multi-container environment (API, DB, UI) |
| **Deployment** | Heroku Eco / DigitalOcean Droplet VPS | Containerized cloud deployment via GitHub Actions CI/CD |

---

## 3. Core Architectural Decisions

### 3.1 Financial Ledger & Virtual Wallet Escrow
- Setiap user memiliki akun dompet (`wallets`) berisi `available_balance` dan `locked_balance`.
- Saat bid dipasang: validasi saldo cukup, pindahkan `bid_amount` dari available ke locked.
- Saat tertimpa penawar lain (outbid): dana penawar lama otomatis dikembalikan dari locked ke available dalam transaksi atomik yang sama.
- Penguncian baris (`FOR UPDATE`) pada tabel lelang dan dompet dilakukan berurutan berdasarkan ID untuk mencegah database deadlock.

### 3.2 Anti-Sniping & Lifecycle Engine
- Jika tawaran valid masuk di 2 menit terakhir sebelum `end_time`, kolom `end_time` otomatis diperpanjang 2 menit di transaksi yang sama.
- Background worker internal (`time.Ticker`) memindai lelang aktif yang melewati `end_time`, mengunci lelang, menetapkan pemenang, dan memindahkan dana terkunci ke dompet penjual.
- Lelang tanpa penawar bertransisi ke `completed` dengan `winner_id = NULL`.
- Endpoint buy-now langsung menuntaskan lelang seketika dan mengembalikan dana penawar sebelumnya.

### 3.3 Data Scraping & Auto-Seeding Pipeline
- CLI terpisah `cmd/scraper` mengekstrak puluhan model Hot Wheels, Mini GT, dan Inno64 asli ke `data/seeds.json`.
- CLI `cmd/seed` (dieksekusi otomatis saat container database pertama kali aktif) menyuntikkan data katalog riil, akun demo penjual/penawar, dan saldo awal.

### 3.4 AI Natural Language Search
- Endpoint `POST /auctions/search/ai` menerima query bebas pengguna (contoh: "carikan minigt nissan gtr merah di bawah 200k").
- LLM memetakan input ke JSON skema terstruktur (brand, model, series, color, condition, min_price, max_price).
- Backend Go memvalidasi struct dan mengeksekusi parameterized query SQL ke PostgreSQL dengan mekanisme fallback jika hasil query kosong.

---

## 4. Repository Structure

```
LelangHotwheels/
├── cmd/
│   ├── api/          # Entrypoint REST API server
│   ├── scraper/      # CLI data scraping diecast wiki
│   └── seed/         # CLI database seeder
├── internal/
│   ├── config/       # Environment configuration (12-Factor)
│   ├── handler/      # HTTP handlers & Chi routes
│   ├── middleware/   # Auth JWT, Rate limit, CORS, Logging
│   ├── model/        # Domain entities & structs
│   ├── repository/   # Database queries via pgx (raw SQL)
│   ├── service/      # Business logic (wallet, auction, bidding)
│   └── worker/       # Background expiration ticker
├── migrations/       # SQL migration files
├── data/             # seeds.json scraped data
├── frontend/         # React + Vite + Tailwind CSS SPA
├── docs/             # Specs and architecture plans
├── docker-compose.yml
├── Dockerfile.backend
└── Dockerfile.frontend
```
