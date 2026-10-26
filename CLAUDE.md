# CLAUDE.md

Panduan ringkas untuk Claude Code CLI dalam mengembangkan platform lelang diecast skala 1:64 (Hot Wheels, Mini GT, Inno64, Matchbox, Tomica, Tarmac Works, M2 Machines).

---

## Prinsip & Alur Kerja
- Baca `TODO.md` sebelum mulai pengerjaan untuk mengambil tugas fase aktif dan perbarui status centang setelah selesai.
- Rujuk `plan.md` untuk arsitektur detail, spec Section 6 untuk keputusan final, dan `SCRATCHPAD.md` untuk log sesi.
- Catat setiap fitur atau perbaikan yang di-commit di `CHANGELOG.md` (ringkas, dengan hash commit).
- Prioritaskan Test-Driven Development (TDD): tulis failing test sebelum menulis logika bisnis baru.
- Dokumen dan komentar tanpa em-dash.

---

## Perintah Penting

```bash
# Jalankan seluruh stack (Postgres, Go API, React di nginx) lalu buka http://localhost:8081
# Butuh POSTGRES_PASSWORD dan JWT_SECRET (>= 32 byte) di .env; compose menolak jalan tanpa POSTGRES_PASSWORD.
# Untuk development cukup `docker compose up -d db` lalu jalankan API dan Vite lokal
docker compose up --build

# Kalau build berhenti dengan "additional privileges requested: pass --allow=fs.read=...":
# regresi buildx >= 0.37.2 (Compose hanya memberi izin baca untuk context). Matikan cek itu
# hanya untuk satu perintah, karena compose file ini milik kita sendiri. PowerShell: $env:BUILDX_BAKE_ENTITLEMENTS_FS=0
BUILDX_BAKE_ENTITLEMENTS_FS=0 docker compose up --build

# Sama, plus katalog dan akun demo. Di server publik isi DEMO_PASSWORD dengan secret, karena default-nya password publik
docker compose --profile demo up --build

# Server: HTTPS lewat Caddy di 80/443. Isi DOMAIN (DNS mengarah ke server) dan WEB_PORT=127.0.0.1:8081 di .env
# Lokal: DOMAIN=localhost memakai CA Caddy sendiri (curl -k https://localhost/api/healthz)
docker compose --profile https up -d

# Pengujian wajib dengan deteksi data race (service test dibuat di Phase 1)
# Alasan: -race butuh CGO + gcc, dan mesin dev Windows tidak punya gcc.
docker compose --profile test run --rm test

# Reset testdb setelah mengedit migrasi yang sudah ada, lalu jalankan tanpa cache
# Alasan: testdb tetap hidup antar run, jadi skema lama tidak diterapkan ulang.
docker compose --profile test down
docker compose --profile test run --rm test go test -race -count=1 ./...

# Loop test cepat lokal tanpa race detector. Tanpa TEST_DATABASE_URL semua test DB di-skip, jadi "ok" di sini belum berarti logika DB teruji
go test ./...

# Scraping data riil diecast ke data/seeds.json
go run cmd/scraper/main.go

# Isi database dengan katalog dan akun demo (butuh DATABASE_URL, idempoten)
go run cmd/seed/main.go

# Regenerasi dokumentasi Swagger setelah mengubah anotasi handler atau tipe service.
# Lewat go run dengan versi yang sama seperti CI: binary swag lokal bisa lebih baru (dan mengaku v1.16.4) lalu menulis output berbeda
go run github.com/swaggo/swag/cmd/swag@v1.16.4 init -g handler.go -d internal/handler,internal/service -o internal/apidocs --ot go,json

# API lokal di :8080 untuk `npm run dev` (API di Docker tidak dipublish ke host, jadi tanpa ini katalog kosong).
# API tidak membaca .env sendiri, jadi muat dulu (Git Bash)
set -a; . ./.env; set +a; go run ./cmd/api

# Jalankan frontend secara lokal. Vite mem-proxy /api ke API di :8080 (seperti nginx di produksi), jadi port Vite bebas
cd frontend && npm install && npm run dev

# Test dan build frontend
cd frontend && npm test && npm run build

# Update produksi (EC2 i-0126a724165a954d4, https://blister64.ariefusry.me) setelah push ke main.
# Server tidak punya port masuk: perintah dikirim lewat SSM dan berjalan sebagai root di /opt/blister64.
# Kredensial AWS berlaku 12 jam, perbarui dengan `aws login --profile blister64`. Detail di docs/deploy.md langkah 10.
aws ssm send-command --profile blister64 --region ap-southeast-2 --instance-ids i-0126a724165a954d4 \n  --document-name AWS-RunShellScript --parameters 'commands=["cd /opt/blister64 && git pull && BUILDX_BAKE_ENTITLEMENTS_FS=0 docker compose --profile demo --profile tunnel up -d --build"]'
```

---

## Aturan Rekayasa Khusus Proyek (Beserta Alasan)

1. **Gunakan Raw SQL via `pgx`, Hindari ORM Berat**  
   *Alasan:* Sistem lelang menuntut kontrol transaksi atomik tingkat rendah dan penguncian baris database eksplisit (`SELECT ... FOR UPDATE`). ORM menambah lapisan abstraksi yang mempersulit diagnosis query lambat dan rawan race condition.

2. **Urutan Lock Tetap: Row Lelang Dulu, Lalu Dompet dengan `user_id` Naik**  
   *Alasan:* Lock lelang menyerialkan semua bid pada satu lelang. Urutan dompet yang konsisten mencegah deadlock saat dua transaksi menyentuh dompet yang sama dari lelang berbeda.

3. **Mekanisme Escrow Dompet Virtual dengan Ledger**  
   *Alasan:* Dana penawar dikunci pada `locked_balance` agar tidak bisa dipakai ganda di lelang lain, dan dikembalikan ke `available_balance` saat outbid dalam transaksi yang sama. Setiap mutasi juga menulis `wallet_transactions` di transaksi yang sama agar saldo bisa diaudit dan diuji terhadap riwayatnya.

4. **Uang Selalu `BIGINT` Rupiah dengan `CHECK (>= 0)`**  
   *Alasan:* Float menghasilkan error pembulatan. CHECK constraint membuat database sendiri menolak saldo negatif, jadi bug di kode aplikasi tidak bisa merusak data.

5. **Waktu dari `now()` PostgreSQL di Dalam Transaksi**  
   *Alasan:* Validasi `end_time` dan anti-sniping harus memakai satu jam yang sama. Jam aplikasi bisa melenceng antar instance.

6. **Anti-Sniping Waktu Lelang**  
   *Alasan:* Bid valid di 2 menit terakhir membuat `end_time = end_time + 2 menit` di transaksi yang sama. Ini mencegah bot merebut lelang di detik terakhir dan memberi kesempatan adil bagi penawar manusia.

7. **Cancel Hanya Saat Belum Ada Bid; Buy-Now Selama Tawaran di Bawah Harganya (model Yahoo Auctions)**  
   *Alasan:* Cancel tanpa bid berarti tidak ada escrow yang perlu dilepas. Buy-now tetap terbuka selama `current_highest_bid < buy_now_price`, dan bid yang mencapai harga itu dihitung sebagai buy-now di harga itu, jadi lelang aktif selalu di bawah harga buy-now. Keduanya lewat `buyOut`: pemimpin (bisa pembelinya sendiri) di-release dulu, lalu harga pindah dari pembeli ke penjual, semua di transaksi yang sama dengan lock lelang.

8. **LLM Hanya Menghasilkan JSON Terstruktur**  
   *Alasan:* Model LLM (Gemini, default `gemini-3.5-flash-lite`) hanya memecah kalimat pengguna menjadi parameter filter (brand, seri, warna, tags, kondisi, harga). Backend Go memvalidasi struct lalu menyusun parameterized query SQL untuk mencegah halusinasi data dan SQL injection.

9. **`data/seeds.json` Adalah Kontrak Data Seeder, Bid Seed Lewat Service**  
   *Alasan:* File kecil manual dipakai sampai fase scraper, lalu diganti data riil agar demo menampilkan katalog otentik. Bid simulasi lewat service bid agar invariant dompet dan ledger tetap benar.

10. **Data Test Selalu Unik, Jangan TRUNCATE**  
   *Alasan:* `go test ./...` menjalankan package secara paralel terhadap satu `testdb`. Pakai `testutil.Unique()` untuk email/username, dan ID negatif untuk fixture ber-ID tetap (sequence tidak pernah menghasilkan ID negatif), supaya test antar package tidak saling bentrok.

---

<!-- BEGIN AWS Agent Toolkit rules -->
# AWS Guidance for the new AWS experience

This user has signed up for the new AWS experience. This experience lets you sign into AWS using a social provider and requires the following additional context.

Where this guidance conflicts with the project's own instructions, the project's instructions take precedence.

## Context

### Terminology:

- Say "project" instead of "account": a project contains an AWS account and settings for sharing with other collaborators
- Say "team member" instead of "IAM user": users are invited by email, not created or federated in IAM
- Say "AWS Settings" when referring to management tasks at [settings.aws.com](https://settings.aws.com/) (project management, billing, team members, spend limits). Users view their actual AWS resources in the AWS Management Console.
- Say "selected Region" when referring to the user's Region: not "home Region"
- The user has a managed IAM experience. This includes a managed service control policies (SCP) and resource control policies (RCP) that govern the use of AWS. They will still need to use IAM to create policies to let services work with each other. If there are questions about the SCPs or RCPs, go to the documentation at https://docs.aws.amazon.com/accounts/latest/reference/scps-and-rcps-for-projects.html

### Constraints:

- All projects share a single AWS Region determined by the user's contact address. Resources cannot be created in other Regions
- When developing:
  - MUST create all Regional resources in the project's assigned Region
  - You CAN create AWS WAF and Cloudwatch Logs resources in us-east-1 when there are global resources (like a global WAF instance) that require a connection to dependencies in us-east-1. You should not use these for any other reason, because resources in the selected Region will provide lower cost (due to no cross-Region traffic), increased availability (due to no cross-Region traffic), and easier manageability (due to not needing to look in another Region). When you need to do an inventory of resources, you need to look in both the selected Region and us-east-1 for Cloudwatch Logs or WAF resources.
  - MUST NOT attempt to create Lambda, API Gateway, or other Regional resources in any other Region
  - MUST direct users to confirm their Region in AWS Settings > View all projects > Overview > Additional Info > Region. If the user cannot confirm their Region, check in ~/.aws/config
  - MUST NOT use Lambda@Edge: excluded from both Lambda and CloudFront
  - MUST NOT use CloudFormation StackSets: no multi-account or multi-Region deployments
  - MUST NOT attempt cross-Region actions: no cross-Region replication for DynamoDB/S3/RDS, no multi-Region KMS keys
  - MUST NOT use Route 53 cross-Region routing: geolocation, latency-based, and failover routing policies are not available
  - CloudFront is a global service and its actions ARE allowed in `us-east-1`. A user can create a CloudFront distribution pointing to their project-region Lambda function URL or API Gateway. However, Lambda and API Gateway themselves MUST NOT be created in `us-east-1`: they must be in the project Region.
  - Reduced availability in `eu-north-1` specifically: Amazon Rekognition, Amazon Textract, Amazon Personalize, AWS App Runner are not available in that Region.
- IAM permissions for human access are managed by AWS. Don't assign roles to team members unless absolutely necessary
- The user may have a spend limit if they are on the paid plan. The limit that pauses their project if it's exceeded. If resources suddenly become inaccessible, ask if they have a spend limit configured. Only project owners can modify a spend limit.
- When developing:
  - MUST ask about spend limit status if the user reports sudden "Access Denied" errors on operations that previously worked
  - MUST direct users to check spend status in AWS Settings > Billing
  - MUST check if a user has upgraded their account to the paid plan
  - MUST ask the user if they want to clean up the successfully created resources or keep them to reduce cost
- The user sets up billing, creates spend limits, and retrieves and pays invoices in AWS Settings. The user creates budgets and optimizes their costs in the AWS Billing and Cost Management console
- Not all AWS services are available. If a service isn't working, do the following:
  1. Run the command `aws freetier get-account-plan-state`
  2. If accountPlanType": "FREE", check the [Free Tier supported services list](https://docs.aws.amazon.com/accounts/latest/reference/supported-services-sign-up-new.html#supported-services-free-tier) next,
  3. If accountPlanType": "PAID", check the [Paid Tier supported services list](https://docs.aws.amazon.com/accounts/latest/reference/supported-services-sign-up-new.html#supported-services-paid-plan).
  4. If neither list shows the service, check the [Not supported for this experience list](https://docs.aws.amazon.com/accounts/latest/reference/supported-services-sign-up-new.html#unsupported-services). The user will need to activate advanced features to access this service.
- Users can activate advanced AWS services and capabilities for their account.
- Before starting a task, check whether a relevant AWS skill is available. Load the skill with retrieve_skill and prefer its guidance over general knowledge.

### Help level

- help_level (required): LOW, MEDIUM, or HIGH. While a user is building, you MUST ask the user: "How much guidance would you like from me? Low (I only flag security risks), medium (I ask a couple of clarifying questions if something seems off), or high (I explain what I'm doing, suggest alternatives, and flag best practices)."

You CAN update this rule file to save a user's help_level.

help_level saat ini: **HIGH** (dipilih user 2026-10-25).

Constraints for each level:

**LOW:**

- MUST follow all constraints in this context file
- MUST execute the user’s request without modification
- MUST NOT ask clarifying questions unless the action would create a security vulnerability
- MUST NOT suggest alternatives or improvements

**MEDIUM:**

- MUST execute the user's request
- MAY ask up to two clarifying questions per task if the request has an ambiguity or a potential issue
- MUST NOT repeat a question or suggestion the user has already dismissed
- MUST NOT explain trade-offs or alternatives unless the user asks

**HIGH:**

- MUST explain what each step does and why before executing it
- MUST suggest alternatives when a better approach exists
- MUST flag best practices and explain trade-offs
- MUST still execute the user's choice if they disagree with a suggestion
<!-- END AWS Agent Toolkit rules -->
