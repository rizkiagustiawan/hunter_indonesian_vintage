# FBM Vintage Monitor — Anti-Ban Edition (Go Rewrite)

Bot monitoring otomatis untuk Facebook Marketplace yang difokuskan pada pencarian barang-barang *vintage* (pakaian, jaket, denim, dll.) di seluruh Indonesia.

Bot ini telah ditulis ulang sepenuhnya menggunakan **Go** dengan fokus pada **Anti-Ban** dan **Human Emulation** untuk menghindari deteksi AI Facebook di tahun 2026.

## 🚀 Fitur Utama

- **Go Native Performance:** Sangat ringan, hemat RAM, dan bisa berjalan 24/7 di VPS terkecil sekalipun. CGO-free menggunakan `modernc.org/sqlite`.
- **Advanced TLS Impersonation:** Menggunakan library HTTP yang menyamar secara identik dengan Google Chrome (JA4 Fingerprint) di tingkat jaringan, bukan sekadar mengganti User-Agent.
- **HTTP/3 (QUIC) Support:** Menggunakan protokol modern yang membuat traffic terlihat identik dengan browsing manusia biasa.
- **Cookie Rotation & Pool:** Mendukung banyak akun sekaligus. Bot akan memutar akun secara *round-robin* dan otomatis melewati (auto-skip) akun yang terkena limit atau checkpoint.
- **Human Behavioral Emulation:**
  - **Pre-flight:** Mengunjungi homepage Facebook secara berkala sebelum melakukan pencarian.
  - **Coffee Breaks:** Mengambil jeda panjang (3-5 menit) secara acak setiap 15-20 request untuk meniru kelelahan manusia.
  - **Sleep Hours:** Otomatis jeda pada jam 00:00 - 06:00 WIB.
- **Telegram Integration:** Notifikasi langsung dikirim ke Telegram lengkap dengan estimasi harga pasar (Grailed estimate).
- **Standalone Cookie Checker:** Dilengkapi dengan tool CLI terpisah untuk mengecek kesehatan (Live/Die) ratusan cookie dalam hitungan detik.

## 📦 Instalasi & Build

Pastikan Anda sudah menginstal [Go](https://go.dev/dl/) (versi 1.21+ disarankan).

1. Clone repository ini:
   ```bash
   git clone https://github.com/rizkiagustiawan/fbm-vintage-monitor.git
   cd fbm-vintage-monitor
   ```

2. Build bot utama dan tool checker:
   ```bash
   go mod tidy
   go build -o fbm-monitor main.go
   go build -o check-cookies cmd/check_cookies/main.go
   ```

## ⚙️ Konfigurasi

1. Salin template konfigurasi:
   ```bash
   cp config.example.toml config.toml
   ```

2. Edit `config.toml`:
   - Masukkan `telegram_token` dan `telegram_chat_id` bot Anda.
   - (Sangat Disarankan) Isi `proxy_url` dengan **Residential Proxy** format `http://user:pass@host:port` untuk menghindari ban IP, terutama jika dijalankan di VPS.

3. Siapkan Cookies:
   - Buat folder `cookies/` di root direktori project.
   - Export cookies Facebook Anda (menggunakan ekstensi browser seperti EditThisCookie) dalam format JSON.
   - Simpan file ke dalam folder `cookies/` (contoh: `01.json`, `02.json`, dst). Pastikan json tersebut memiliki key `c_user`.

## 💻 Penggunaan

### Mengecek Kesehatan Cookies
Sebelum menjalankan bot utama, pastikan cookies Anda valid:
```bash
./check-cookies
```

### Menjalankan Bot
```bash
./fbm-monitor
```

## ☁️ Deploy ke VPS (Systemd)

Untuk menjalankan bot 24/7 di VPS Linux:

1. Edit file `fbm-monitor.service`, sesuaikan `WorkingDirectory` dan `ExecStart` dengan path absolut di VPS Anda.
2. Salin dan aktifkan service:
   ```bash
   sudo cp fbm-monitor.service /etc/systemd/system/
   sudo systemctl daemon-reload
   sudo systemctl enable fbm-monitor
   sudo systemctl start fbm-monitor
   ```
3. Cek log:
   ```bash
   tail -f data/monitor.log
   ```

## ⚠️ Disclaimer
Bot ini dibuat untuk tujuan edukasi dan penggunaan pribadi. Melakukan scraping dengan intensitas tinggi mungkin melanggar Terms of Service (ToS) Facebook. Gunakan dengan bijak dan gunakan delay yang wajar. Pengembang tidak bertanggung jawab atas akun yang terblokir.
