# Hot Wheels Auction Platform - Design Document

**Date:** 2026-10-24  
**Author:** AI Assistant & ariefusry  
**Status:** In Review (Brainstorming Stage)  

---

## 1. Intent & The Why

### Background & Motivation
The user is a fullstack developer with cum laude graduation from Telkom University whose existing portfolio heavily leverages serverless stacks (Next.js, Vercel, Supabase). To target competitive backend and fullstack engineering roles, the user wants a production-grade system featuring a **Go** backend, a separate **React** frontend, and unified **Docker** orchestration.

### Problem Domain
Collectible auctions (specifically Hot Wheels) present classic, non-trivial backend and frontend engineering challenges:
1. High concurrency and race conditions when multiple users bid simultaneously in closing seconds.
2. Financial consistency via a virtual wallet with escrow holds, atomic transfers, and instant outbid refunds.
3. Lifecycle state management (manual publish, scheduled expiration, and winner finalization).
4. Containerization and cloud deployment readiness using GitHub Student Developer Pack resources (Heroku Eco Dynos atau DigitalOcean Droplet VPS).
5. Functional collector-oriented user interface for listing, bidding, and tracking wallet balances.

This project showcases clean Go architecture, idiomatic database transactions, Docker multi-stage builds, clean React component design, and thorough OpenAPI/Swagger documentation.

---

## 2. Goals, Non-Goals & Anti-Goals

### Goals
- **Unified User Management:** Secure registration, authentication (JWT + bcrypt), and profile retrieval. Any authenticated user can list Hot Wheels items for auction and bid on others' items.
- **Hot Wheels Catalog & Item Metadata:** Rich item attributes tailored to collector terminology (model name, series, release year, scale 1:64, packaging condition: mint on card / loose / blister crack, opening bid, minimum increment, optional buy-now price).
- **Virtual Wallet & Escrow Ledger:**
  - Each user has a virtual wallet balance.
  - Endpoints: `GET /wallet`, `POST /wallet/deposit` (mock top-up).
  - Escrow hold: When a bid is placed, the bid amount is locked in escrow from the user's available balance.
  - Outbid refund: When a higher bid arrives, the previous highest bidder's locked funds are immediately released back to their available balance in the same database transaction.
  - Strict resource ordering during transactions (e.g. order by resource ID) to prevent database deadlocks.
- **Auction Engine & Lifecycle:**
  - States: `draft`, `active`, `completed`, `cancelled`.
  - Manual publication: `POST /auctions/{id}/publish` activates a draft auction.
  - Safe cancellation: `POST /auctions/{id}/cancel` allowed only when bid count is zero.
  - Configurable duration with strict `start_time` and `end_time`.
- **Atomic Bidding Mechanism:**
  - Bids must be validated atomically inside a PostgreSQL transaction using row-level locking (`SELECT ... FOR UPDATE`) on the auction and wallet rows to eliminate race conditions.
  - Validation rules: auction must be `active`, current time must be `< end_time`, bid amount must be `>= current_highest_bid + min_increment`, bidder cannot be the item seller, bidder must have sufficient wallet balance.
- **Instant Buy-Now Option:**
  - Dedicated endpoint `POST /auctions/{id}/buy-now` if `buy_now_price` is configured.
  - Immediately ends the auction, charges buyer wallet, refunds previous highest bidder (if any), and marks status as `completed`.
- **Background Expiration Worker:**
  - A background Go worker (goroutine with `time.Ticker`) scanning for expired active auctions (`end_time <= now()`), locking them, setting status to `completed`, and assigning `winner_id` (or `NULL` if zero bids). If a winner exists, the escrow funds are transferred to the seller's wallet balance.
- **Functional React Frontend:**
  - Dedicated SPA in `/frontend` built with React + Vite + Tailwind CSS.
  - Screens: Auth (Login/Register), Catalog listing with condition/price filters and AI search bar, Auction detail with countdown timer and bid placement form, Create auction form with image upload, and Wallet management (view balance, locked escrow, deposit simulation).
- **Natural Language Search (LLM Integration):**
  - Endpoint `POST /auctions/search/ai` accepting freeform natural language query (e.g. "carikan jdm merah kondisi mint di bawah 100k").
  - Backend Go invokes lightweight LLM (Google Gemini Flash / Groq) requesting strict JSON structured output representing query filters (series, color, condition, min/max price, sort).
  - Go validates the parsed JSON and constructs parameterized SQL queries against PostgreSQL. Fallback logic automatically broadens search if zero results match strictly.
- **Real Data Scraper & Auto-Seeder Pipeline:**
  - Standalone Go CLI tool in `cmd/scraper` that crawls authentic Hot Wheels models, release years, series, and official photo URLs from the public Hot Wheels Wiki.
  - Output stored as structured JSON in `data/seeds.json`.
  - Database seeder tool in `cmd/seed` (and executed during Docker startup) that populates PostgreSQL with real item catalogs, demo seller/bidder accounts, and simulated initial bids.
- **Containerization & Local Dev:**
  - Multi-stage `Dockerfile` for backend (Go binary on Alpine).
  - Multi-stage `Dockerfile` for frontend (Node build stage, Nginx production server).
  - `docker-compose.yml` spinning up PostgreSQL, Go API, and React frontend with automated migrations and health checks.
- **API Documentation & Quality:**
  - Interactive Swagger / OpenAPI documentation served at `/swagger/index.html`.
  - Structured logging, standardized JSON error envelopes, and environment-based configuration (12-Factor App).
- **CI/CD Pipeline:**
  - GitHub Actions workflow running tests, linting, and Docker container build, ready for deployment to Heroku Eco Dynos or DigitalOcean Droplet VPS.

### Non-Goals
- **No Complex Mobile App:** Web browser responsive UI only.
- **No Real Third-Party Payment Gateway:** Uses virtual wallet escrow to avoid external sandbox friction while demonstrating financial ledger integrity.
- **No Real-Time WebSockets in Phase 1:** Keep the API strictly RESTful with countdown on frontend. WebSocket or SSE for live bid streams is intentionally deferred to Phase 2.
- **No Complex Microservices:** Single monorepo containing `/backend` and `/frontend`.

### Anti-Goals
- **Over-engineering with Distributed Queues (Kafka/RabbitMQ) or Kubernetes:** Unnecessary operational overhead for this project scale.
- **In-memory-only State:** Bids, auction states, wallets, and timers must be persisted and transactionally safe in PostgreSQL.

---

## 3. Constraints

- **Language:** Go (1.19+ standard library compatibility).
- **Database:** PostgreSQL 15+.
- **Portability:** Fully reproducible via Docker Compose (`docker compose up --build`).
- **Deployment Budget:** GitHub Student Developer Pack eligible (Heroku Eco Dynos / DigitalOcean Droplet VPS).
- **Zero Em-Dash & Clean Tone:** Project documentation and comments adhere to clean, direct formatting.

---

## 4. User-Decided Decisions

1. **Domain:** Hot Wheels collector auction platform.
2. **Architecture:** Decoupled architecture with Go REST API and separate React frontend (Vite + Tailwind CSS).
3. **Role Structure:** Unified account model. A single user account can act as both seller and bidder, but is strictly prohibited from bidding on their own listings.
4. **Financial Model:** Virtual wallet system with escrow holds during active bids and automated refunds when outbid.
5. **AI Integration:** Natural language auction search via LLM structured output mapping to PostgreSQL queries.
6. **Data Pipeline:** CLI scraper in Go targeting authentic Hot Wheels wiki data feeding an automated database seeder.
7. **Target Deployment:** Dockerized multi-container deployment (backend, frontend, PostgreSQL) leveraging GitHub Student benefits.

---

## 5. Builder Decisions (Resolved by Reviewer Check)

1. **Escrow & Settlement:**
   - Placing a bid locks `bid_amount` into `locked_balance`.
   - When outbid, the previous bidder's `locked_balance` is refunded back to `available_balance` atomically.
   - When auction completes, worker moves winner's `locked_balance` into seller's `available_balance`.
2. **Buy-Now Mechanics:**
   - `POST /auctions/{id}/buy-now` terminates the auction instantly, sets winner, transfers funds, and releases any previous bidder's escrow hold.
3. **Lifecycle Rules:**
   - Seller creates auction in `draft` state and triggers `POST /auctions/{id}/publish` to make it `active`.
   - Seller can cancel with `POST /auctions/{id}/cancel` only if no bids have been placed yet.
   - Zero-bid auctions transitioning past `end_time` are marked `completed` with `winner_id = NULL`.
4. **Framework & Driver:**
   - Router: Standard library compliant router (`chi` router recommended for idiomatic lightweight middleware).
   - Database: `pgx` driver or `sqlx` with SQL migrations (`golang-migrate`).
   - Image uploads: Local multipart upload stored in `./uploads` with static file serving, container volume mounted for persistence.
