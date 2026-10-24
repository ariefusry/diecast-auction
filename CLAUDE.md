# CLAUDE.md

Panduan ringkas untuk Claude Code CLI dalam mengembangkan platform lelang diecast skala 1:64 (Hot Wheels, Mini GT, Inno64).

---

## Prinsip & Alur Kerja
- Baca `TODO.md` sebelum mulai pengerjaan untuk mengambil tugas fase aktif dan perbarui status centang setelah selesai.
- Rujuk `plan.md` untuk arsitektur detail, spec Section 6 untuk keputusan final, dan `SCRATCHPAD.md` untuk log sesi.
- Prioritaskan Test-Driven Development (TDD): tulis failing test sebelum menulis logika bisnis baru.
- Dokumen dan komentar tanpa em-dash.

---

## Perintah Penting

```bash
# Jalankan seluruh service (Postgres, Go API, React)
docker compose up --build

# Pengujian wajib dengan deteksi data race (service test dibuat di Phase 1)
# Alasan: -race butuh CGO + gcc, dan mesin dev Windows tidak punya gcc.
docker compose --profile test run --rm test

# Reset testdb setelah mengedit migrasi yang sudah ada, lalu jalankan tanpa cache
# Alasan: testdb tetap hidup antar run, jadi skema lama tidak diterapkan ulang.
docker compose --profile test down
docker compose --profile test run --rm test go test -race -count=1 ./...

# Loop test cepat lokal tanpa race detector
go test ./...

# Scraping data riil diecast ke data/seeds.json
go run cmd/scraper/main.go

# Isi database dengan katalog dan akun demo (butuh DATABASE_URL, idempoten)
go run cmd/seed/main.go

# Regenerasi dokumentasi Swagger setelah mengubah anotasi handler atau tipe service
swag init -g handler.go -d internal/handler,internal/service -o internal/apidocs --ot go,json

# Jalankan frontend secara lokal (API di :8080, CORS default mengizinkan :5173)
cd frontend && npm install && npm run dev

# Test dan build frontend
cd frontend && npm test && npm run build
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

7. **Cancel dan Buy-Now Hanya Saat Belum Ada Bid (`highest_bidder_id IS NULL`)**  
   *Alasan:* Tidak ada escrow penawar lain yang perlu dilepas, jadi jalur buy-now tetap sederhana dan bebas bug refund.

8. **LLM Hanya Menghasilkan JSON Terstruktur**  
   *Alasan:* Model LLM (Gemini / Groq) hanya memecah kalimat pengguna menjadi parameter filter (brand, seri, warna, tags, kondisi, harga). Backend Go memvalidasi struct lalu menyusun parameterized query SQL untuk mencegah halusinasi data dan SQL injection.

9. **`data/seeds.json` Adalah Kontrak Data Seeder, Bid Seed Lewat Service**  
   *Alasan:* File kecil manual dipakai sampai fase scraper, lalu diganti data riil agar demo menampilkan katalog otentik. Bid simulasi lewat service bid agar invariant dompet dan ledger tetap benar.

10. **Data Test Selalu Unik, Jangan TRUNCATE**  
   *Alasan:* `go test ./...` menjalankan package secara paralel terhadap satu `testdb`. Pakai `testutil.Unique()` untuk email/username, dan ID negatif untuk fixture ber-ID tetap (sequence tidak pernah menghasilkan ID negatif), supaya test antar package tidak saling bentrok.
