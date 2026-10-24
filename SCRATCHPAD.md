# Project Scratchpad & Session Notes

## Current Status
- Project Name: Scale 1:64 Diecast Auction Platform (Hot Wheels, Mini GT, Inno64)
- Phase: Preparation & Architecture Finalized
- Active Focus: Ready for Phase 1 Execution

---

## Key Context & Constraints
- Language & Runtime: Go 1.19+ (Root repo)
- Frontend: React + Vite + Tailwind CSS (`/frontend`)
- Database: PostgreSQL 15 via Docker Compose
- Design Spec: `docs/superpowers/specs/2026-10-24-hotwheels-auction-api-design.md`
- Roadmap: `TODO.md`
- Architecture: `plan.md`

---

## Helpful Commands
```bash
# Start all local containers (Database, API, Frontend)
docker compose up --build

# Run backend tests with race condition detector
go test -race ./...

# Run database migrations
migrate -path migrations -database "postgres://user:password@localhost:5432/lelang_db?sslmode=disable" up

# Run real data scraper
go run cmd/scraper/main.go

# Run database seeder
go run cmd/seed/main.go

# Start frontend development server
cd frontend && npm install && npm run dev
```

---

## Session Log & Decision Notes
- 2026-10-24: Inisialisasi spesifikasi arsitektur platform lelang diecast 1:64 multi-brand (Hot Wheels, Mini GT, Inno64).
- 2026-10-24: Menetapkan model dompet virtual dengan sistem escrow, anti-sniping 2 menit, AI natural language search, Go scraper CLI, dan fullstack Docker Compose.
- 2026-10-24: Menyiapkan `plan.md`, `TODO.md`, dan `SCRATCHPAD.md` agar siap dieksekusi secara terstruktur oleh Claude Code CLI atau developer.
