# Scale 1:64 Diecast Auction Platform - Design Document

**Date:** 2026-10-24  
**Author:** AI Assistant & ariefusry  
**Status:** Approved (2026-10-25, see Section 6)  

---

## 1. Intent & The Why

### Background & Motivation
The user is a fullstack developer with cum laude graduation from Telkom University whose existing portfolio heavily leverages serverless stacks (Next.js, Vercel, Supabase). To target competitive backend and fullstack engineering roles, the user wants a production-grade system featuring a **Go** backend, a separate **React** frontend, and unified **Docker** orchestration.

### Problem Domain
Collectible diecast scale 1:64 auctions (Hot Wheels, Mini GT, Inno64) present classic, non-trivial backend and frontend engineering challenges:
1. High concurrency and race conditions when multiple users bid simultaneously in closing seconds.
2. Financial consistency via a virtual wallet with escrow holds, atomic transfers, and instant outbid refunds.
3. Anti-sniping dynamic time extension when bids land in the final minutes of an auction.
4. Multi-brand diecast metadata and collector condition nuances (blister card, sealed box, acrylic case, loose).
5. Lifecycle state management (manual publish, scheduled expiration, and winner finalization).
6. Containerization and cloud deployment readiness using GitHub Student Developer Pack resources, deployed as Docker Compose on a generic Linux VPS.
7. Functional collector-oriented user interface for listing, bidding, and tracking wallet balances.

This project showcases clean Go architecture, idiomatic database transactions, Docker multi-stage builds, clean React component design, and thorough OpenAPI/Swagger documentation.

---

## 2. Goals, Non-Goals & Anti-Goals

### Goals
- **Unified User Management:** Secure registration, authentication (JWT + bcrypt), and profile retrieval. Any authenticated user can list diecast items for auction and bid on others' items.
- **Multi-Brand Diecast Catalog & Metadata:** Rich item attributes tailored to 1:64 diecast collector terminology:
  - Brand: Hot Wheels, Mini GT, Inno64, or custom maker.
  - Model name, series (e.g. Kaido House, Car Culture, Special Edition), release year, casting details.
  - Packaging condition: mint on card / blister card, sealed box, acrylic case, loose mint, damaged packaging.
  - Financial parameters: opening bid, minimum increment, optional buy-now price.
- **Virtual Wallet & Escrow Ledger:**
  - Each user has a virtual wallet balance.
  - Endpoints: `GET /wallet`, `POST /wallet/deposit` (mock top-up).
  - Escrow hold: When a bid is placed, the bid amount is locked in escrow from the user's available balance.
  - Outbid refund: When a higher bid arrives, the previous highest bidder's locked funds are immediately released back to their available balance in the same database transaction.
  - Strict lock ordering during transactions (auction row first, then wallet rows by ascending `user_id`) to prevent database deadlocks.
  - Every balance mutation also appends a row to the `wallet_transactions` ledger in the same transaction.
- **Auction Engine & Lifecycle:**
  - States: `draft`, `active`, `completed`, `cancelled`.
  - Manual publication: `POST /auctions/{id}/publish` activates a draft auction.
  - Safe cancellation: `POST /auctions/{id}/cancel` allowed only when bid count is zero.
  - Configurable duration with strict `start_time` and `end_time`.
- **Atomic Bidding Mechanism & Anti-Sniping:**
  - Bids must be validated atomically inside a PostgreSQL transaction using row-level locking (`SELECT ... FOR UPDATE`) on the auction and wallet rows to eliminate race conditions.
  - Validation rules: auction must be `active`, database `now()` must be `< end_time`, first bid must be `>= start_price` and later bids `>= current_highest_bid + min_increment`, bidder cannot be the item seller, bidder cannot already be the highest bidder, bidder must have sufficient available balance.
  - Anti-sniping rule: If a valid bid arrives within the final 2 minutes before `end_time`, automatically extend `end_time` by 2 minutes in the same transaction to guarantee fair bidding.
- **Instant Buy-Now Option:**
  - Dedicated endpoint `POST /auctions/{id}/buy-now` if `buy_now_price` is configured, valid only while the auction has zero bids (`highest_bidder_id IS NULL`).
  - Immediately ends the auction, transfers `buy_now_price` from buyer to seller, and marks status as `completed`. No refund path is needed.
- **Background Expiration Worker:**
  - A background Go worker (goroutine with `time.Ticker`) scanning for expired active auctions (`end_time <= now()`), locking them, setting status to `completed`, and assigning `winner_id` (or `NULL` if zero bids). If a winner exists, the escrow funds are transferred to the seller's wallet balance.
- **Functional React Frontend:**
  - Dedicated SPA in `/frontend` built with React + Vite + Tailwind CSS.
  - Screens: Auth (Login/Register), Catalog listing with condition/price filters and AI search bar, Auction detail with countdown timer and bid placement form, Create auction form with image upload, and Wallet management (view balance, locked escrow, deposit simulation).
- **Natural Language Search (LLM Integration):**
  - Endpoint `POST /auctions/search/ai` accepting freeform natural language query (e.g. "carikan jdm merah kondisi mint di bawah 100k").
  - Backend Go invokes lightweight LLM (Google Gemini Flash / Groq) requesting strict JSON structured output representing query filters (brand, series, color, tags, condition, min/max price, sort).
  - Go validates the parsed JSON and constructs parameterized SQL queries against PostgreSQL. Fallback logic automatically broadens search if zero results match strictly.
- **Real Data Scraper & Auto-Seeder Pipeline:**
  - Standalone Go CLI tool in `cmd/scraper` that crawls authentic diecast models across major brands (Hot Wheels, Mini GT, Inno64) with release years, series, and official photo URLs from collector reference catalogs.
  - Output stored as structured JSON in `data/seeds.json`.
  - Database seeder tool in `cmd/seed` (and executed during Docker startup) that populates PostgreSQL with real item catalogs, demo seller/bidder accounts, and simulated initial bids.
- **Containerization & Local Dev:**
  - Multi-stage `Dockerfile` for backend (Go binary on Alpine).
  - Multi-stage `Dockerfile` for frontend (Node build stage, Nginx production server).
  - `docker-compose.yml` spinning up PostgreSQL, Go API, and React frontend with automated migrations and health checks.
- **API Documentation & Quality Engineering:**
  - Interactive Swagger / OpenAPI documentation served at `/swagger/index.html`.
  - Structured logging, standardized JSON error envelopes, and environment-based configuration (12-Factor App).
  - Graceful shutdown intercepting OS signals (SIGINT, SIGTERM) to allow in-flight database transactions to finish cleanly before process exit.
  - In-memory per-IP rate limiting using `golang.org/x/time/rate` token bucket to prevent bot spam on bidding and authentication endpoints.
- **CI/CD Pipeline:**
  - GitHub Actions workflow running tests, linting, and Docker container build, ready for deployment to a generic Linux VPS running Docker Compose.

### Non-Goals
- **No Complex Mobile App:** Web browser responsive UI only.
- **No Real Third-Party Payment Gateway:** Uses virtual wallet escrow to avoid external sandbox friction while demonstrating financial ledger integrity.
- **No Real-Time WebSockets in Phase 1:** Keep the API strictly RESTful with countdown on frontend. WebSocket or SSE for live bid streams is intentionally deferred until after the MVP. The auction detail page polls instead.
- **No Complex Microservices:** Single monorepo with the Go module at the repo root and the SPA in `/frontend`.

### Anti-Goals
- **Over-engineering with Distributed Queues (Kafka/RabbitMQ) or Kubernetes:** Unnecessary operational overhead for this project scale.
- **In-memory-only State:** Bids, auction states, wallets, and timers must be persisted and transactionally safe in PostgreSQL.

---

## 3. Constraints

- **Language:** Go, latest stable at upgrade time (minimum 1.22 for `net/http` ServeMux method and path patterns). Exact version is pinned in `go.mod` and the `golang` image tag in Phase 1.
- **Database:** PostgreSQL 15+.
- **Portability:** Fully reproducible via Docker Compose (`docker compose up --build`).
- **Deployment Budget:** Low-cost generic Linux VPS running Docker Compose. Provider chosen in the deployment phase.
- **Zero Em-Dash & Clean Tone:** Project documentation and comments adhere to clean, direct formatting.

---

## 4. User-Decided Decisions

1. **Domain:** Multi-brand scale 1:64 diecast auction platform (Hot Wheels, Mini GT, Inno64).
2. **Architecture:** Decoupled architecture with Go REST API and separate React frontend (Vite + Tailwind CSS).
3. **Role Structure:** Unified account model. A single user account can act as both seller and bidder, but is strictly prohibited from bidding on their own listings.
4. **Financial Model:** Virtual wallet system with escrow holds during active bids and automated refunds when outbid.
5. **AI Integration:** Natural language auction search via LLM structured output mapping to PostgreSQL queries.
6. **Data Pipeline:** CLI scraper in Go targeting authentic diecast collector catalogs (Hot Wheels, Mini GT, Inno64) feeding an automated database seeder.
7. Anti-Sniping Protection: Automatic 2-minute dynamic auction extension when valid bids arrive in the closing 2 minutes.
8. Target Deployment: Dockerized multi-container deployment (backend, frontend, PostgreSQL) leveraging GitHub Student benefits (Heroku Eco / DigitalOcean Droplet VPS). *Amended 2026-10-25: generic Linux VPS + Docker Compose. Heroku Eco dynos sleep (stopping the expiration worker) and have an ephemeral filesystem (losing `./uploads`); the DigitalOcean Student Pack credit reportedly ended 2026-07-31.*

---

## 5. Builder Decisions (Resolved by Reviewer Check)

1. **Escrow & Settlement:**
   - Placing a bid locks `bid_amount` into `locked_balance`.
   - When outbid, the previous bidder's `locked_balance` is refunded back to `available_balance` atomically.
   - When auction completes, worker moves winner's `locked_balance` into seller's `available_balance`.
2. **Buy-Now Mechanics:**
   - `POST /auctions/{id}/buy-now` is allowed only while the auction has zero bids. It terminates the auction instantly, sets winner, and transfers funds. No escrow release is needed.
3. **Lifecycle Rules:**
   - Seller creates auction in `draft` state and triggers `POST /auctions/{id}/publish` to make it `active`.
   - Seller can cancel with `POST /auctions/{id}/cancel` only if no bids have been placed yet (`highest_bidder_id IS NULL`).
   - Zero-bid auctions transitioning past `end_time` are marked `completed` with `winner_id = NULL`.
4. **Framework & Driver:**
   - Router: standard library `net/http` ServeMux (Go 1.22+ patterns). Add `chi` only if middleware grouping becomes messy.
   - Database: `pgx/v5` with `pgxpool`, raw SQL. Migrations via the `golang-migrate` library with `embed`, applied on API startup.
   - Image uploads: Local multipart upload stored in `./uploads` with static file serving, container volume mounted for persistence.

---

## 6. Decisions Resolved (2026-10-25)

1. **Toolchain:** Upgrade local Go to latest stable. `go test -race` runs inside a `golang` container (has gcc, `CGO_ENABLED=1`) via a compose `test` service, because the Windows dev machine has no C compiler.
2. **Deployment:** Generic Linux VPS + Docker Compose. Provider chosen in the deployment phase.
3. **Money:** `BIGINT` whole rupiah, never floats. `CHECK (available_balance >= 0 AND locked_balance >= 0)` on `wallets`.
4. **Ledger:** Append-only `wallet_transactions` (`id, wallet_id, type, amount, auction_id NULL, created_at`), types `deposit`, `lock`, `release`, `settle`, `buy_now`. Written in the same transaction as the balance change.
5. **Lock order:** Auction row first (serializes bids per auction), then wallet rows by ascending `user_id`.
6. **Bid rules:** First bid `>= start_price`; later bids `>= current_highest_bid + min_increment`. The current highest bidder cannot bid again.
7. **Time source:** PostgreSQL `now()` inside the transaction, not the application clock.
8. **Anti-sniping:** `end_time = end_time + interval '2 minutes'` when a valid bid lands within 2 minutes of `end_time`.
9. **Publish:** Draft stores `duration`. Publish sets `start_time = now()` and `end_time = now() + duration`.
10. **Buy-now:** Only while `highest_bidder_id IS NULL`.
11. **Worker:** `SELECT ... FOR UPDATE SKIP LOCKED` so multiple instances never settle the same auction.
12. **Seeder:** Initial bids go through the bid service, never direct INSERTs, so wallet and ledger invariants hold. `data/seeds.json` is committed; a small hand-written file is used until the scraper phase.
13. **Items schema:** Add `color` and `tags TEXT[]` (e.g. `jdm`) for AI search filters. Drop `scale` (always 1:64).
14. **Frontend:** JWT sent as Bearer token. Auction detail polls about every 5 seconds so anti-sniping extensions are visible.
15. **Repo layout:** Go module at repo root (per `plan.md`), SPA in `frontend/`. Module path `github.com/ariefusry/diecast-auction`.
16. **Scraper source:** MediaWiki API (`api.php`) of the Hot Wheels, Mini GT, and Inno64 Fandom wikis (CC-BY-SA, source page URL kept as attribution). HTML pages sit behind a Cloudflare challenge; the API does not. The Inno64 wiki is thin (24 articles), so Inno64 entries are topped up by hand.
