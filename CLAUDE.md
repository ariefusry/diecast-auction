# CLAUDE.md

Panduan ringkas untuk Claude Code CLI dalam mengembangkan platform lelang diecast skala 1:64 (Hot Wheels, Mini GT, Inno64).

---

## Prinsip & Alur Kerja
- Baca `TODO.md` sebelum mulai pengerjaan untuk mengambil tugas fase aktif dan perbarui status centang setelah selesai.
- Rujuk `plan.md` untuk arsitektur detail dan `SCRATCHPAD.md` untuk log sesi.
- Prioritaskan Test-Driven Development (TDD): tulis failing test sebelum menulis logika bisnis baru.

---

## Perintah Penting

```bash
# Jalankan seluruh service (Postgres, Go API, React)
docker compose up --build

# Jalankan pengujian backend wajib dengan deteksi data race (kritis untuk fitur bid)
go test -v -race ./...

# Scraping data riil diecast ke data/seeds.json
go run cmd/scraper/main.go

# Isi database dengan katalog riil dan akun demo
go run cmd/seed/main.go

# Jalankan frontend secara lokal
cd frontend && npm install && npm run dev
```

---

## Aturan Rekayasa Khusus Proyek (Beserta Alasan)

1. **Gunakan Raw SQL via `pgx`, Hindari ORM Berat**  
   *Alasan:* Sistem lelang menuntut kontrol transaksi atomik tingkat rendah dan penguncian baris database eksplisit (`SELECT ... FOR UPDATE`). ORM menambah lapisan abstraksi yang mempersulit diagnosis query lambat dan rawan race condition.

2. **Kunci Baris Berurutan Berdasarkan Resource ID**  
   *Alasan:* Saat banyak penawar menaruh tawaran pada lelang dan dompet yang sama, mengunci row lelang dan row dompet dengan urutan ID yang konsisten mencegah terjadinya database deadlock.

3. **Mekanisme Escrow Dompet Virtual**  
   *Alasan:* Dana penawar dikunci pada `locked_balance` saat memasang bid agar saldo tidak bisa dipakai ganda di lelang lain. Jika tawaran terlampaui (outbid), dana otomatis dikembalikan ke `available_balance` dalam transaksi atomik yang sama agar saldo pengguna selalu akurat.

4. **Anti-Sniping Waktu Lelang**  
   *Alasan:* Jika ada bid valid masuk di 2 menit terakhir sebelum lelang berakhir, perpanjang `end_time` sebanyak 2 menit di transaksi yang sama. Ini mencegah bot merebut lelang di detik terakhir dan memberi kesempatan adil bagi penawar manusia.

5. **LLM Hanya Menghasilkan JSON Terstruktur**  
   *Alasan:* Model LLM (Gemini / Groq) hanya bertugas memecah kalimat pengguna menjadi parameter filter (brand, seri, kondisi, harga). Backend Go memvalidasi struct lalu menyusun parameterized query SQL ke PostgreSQL untuk mencegah halusinasi data dan ancaman SQL injection.

6. **Gunakan Data Riil Hasil Scraper untuk Seeder**  
   *Alasan:* Mengisi database awal dari `data/seeds.json` memastikan portofolio langsung menampilkan katalog otentik dengan foto asli, seri rilisan nyata, dan terminologi kolektor yang valid saat didemokan.
