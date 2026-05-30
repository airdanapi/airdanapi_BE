# Panduan Demo & Presentasi - API Gateway (Integrator)

Dokumen ini adalah skenario langkah-demi-langkah (skrip) yang bisa kamu ikuti saat mempresentasikan aplikasi API Gateway ke dosen atau audiens.

## Langkah 1: Persiapan Sistem (Sebelum Presentasi)
Untuk menjalankan ekosistem secara utuh, kamu perlu membuka **4 terminal PowerShell yang berbeda**.

1. **Terminal 1 (Backend Gateway)**:
   - Arahkan ke folder `airdanapi_BE`
   - Jalankan: `go run ./cmd/server`
2. **Terminal 2 (Frontend Console)**:
   - Arahkan ke folder `airdanapi_FE`
   - Jalankan: `npm run dev`
3. **Terminal 3 (Mock Downstream Servers)**:
   - Arahkan ke folder `airdanapi_BE`
   - Jalankan: `go run ./scripts/mock_downstream.go`
   - *(Ini akan menjalankan 6 server bayangan sekaligus, dari port 8101-8106)*
4. **Terminal 4 (Dummy Traffic Generator)**:
   - Arahkan ke folder `airdanapi_BE/scripts`
   - Jalankan: `.\demo_traffic.ps1`
   - *(Biarkan berjalan di background agar grafik di dashboard UI bisa terus bergerak dinamis)*

---

## Langkah 2: Mendemokan Console Integrator (UI Frontend)
Buka browser dan arahkan ke `http://localhost:3000`.

**Narasi saat presentasi:**
1. **Login**: Masuk menggunakan kredensial:
   - Email: `admin@airdanapi.local`
   - Password: `password123`
2. **Dashboard**: "Di halaman Dashboard, kita bisa melihat metrik realtime dari API Gateway. Grafik *Throughput* terus bergerak berkat trafik yang sedang berjalan."
3. **Routes**: Buka menu *Route Registry*. "Ini adalah daftar 30+ rute dari 6 domain aplikasi berbeda yang telah didaftarkan ke Gateway."
4. **Tokens**: Buka menu *Security & JWT*. "Gateway menggunakan otentikasi JWT terpusat yang bisa dicabut kapan saja jika terdeteksi aktivitas mencurigakan."
5. **Logs**: Buka menu *Request Logs*. "Setiap request yang melintasi Gateway akan dicatat seluruh daur hidupnya (STARTED, COMPLETED, FAILED) ke database."
6. **Fees**: Buka menu *Gateway Fees*. "Ini adalah laporan revenue dari potongan fee transaksi sebesar 0.5% yang otomatis dipungut oleh Gateway."

---

## Langkah 3: Mendemokan Transaksi & Keamanan (Backend API)
Buka **Terminal 5** (terminal baru yang bersih) untuk menjalankan request HTTP dari sudut pandang *client*.

### Tahap 3.1: Set Token Otentikasi
Pertama-tama, simpan token JWT valid ini ke dalam variabel terminal agar mudah dipakai di setiap skenario. **(Copy paste baris ini dan tekan Enter):**

```powershell
$TOKEN = "eyJhbGciOiJSUzI1NiIsInR5cCI6IkpXVCJ9.eyJhdWQiOlsiZWNvc3lzdGVtIl0sImV4cCI6MTgxMTIyMTc3NywiaWF0IjoxNzc5Njg1Nzc3LCJpc3MiOiJzbWFydGJhbmsiLCJqdGkiOiJkZW1vLXRva2VuLWxpdmUtdjMiLCJuYmYiOjE3Nzk2ODU3NzcsInJvbGVzIjpbIm9wZXJhdG9yIl0sInNjb3BlcyI6WyJtYXJrZXRwbGFjZTpyZWFkIiwibWFya2V0cGxhY2U6d3JpdGUiLCJhZG1pbjpyZWFkIiwiYWRtaW46d3JpdGUiXSwic3ViIjoiYWRtaW4ifQ.dIr4g6MEA4iOAvaKeUfNhPMDWjODUhQ7vi4PM2TAvIH0MpiJnXOZhvVNX6raV-SYlilm-9pfZl8dFylRXL7xC7azHy5WcVrcE9mzxxxK9yKYdBZlPpTNG16xuF90VbEtpPW2xk-eb0ZQzv-3ciaUfFjlU1NQOzS_bMS-wtFVL4DIaX68b7Atkch4sHA7eLafgdmTpqxZM5INQMaJI97Y1Xmuvb_eGZAMqazHk9kRZLr5xm9CCRNgZUid5iRRYy-wbPk1jz1k8pFtgeKm_Mas2ES-faEg9bWAVnMyN4sOhhwwRHzXqYCTapEWc7Bdvf0N3ox1CLBWvZr7b8T_qotrrg"
```

### Tahap 3.2: Transaksi Marketplace (Mendemokan Potongan Fee)
**Narasi**: "Sekarang kita simulasikan *checkout* pada aplikasi Marketplace. Gateway akan meneruskan request ke server Marketplace, lalu saat respons kembali, Gateway akan otomatis memotong fee 0.5%."

**(Copy paste perintah ini):**
```powershell
Invoke-RestMethod -Uri http://localhost:8080/api/v1/marketplace/checkout -Method Post -Headers @{"Authorization"="Bearer $TOKEN"; "X-Idempotency-Key"="demo-idem-101"; "Content-Type"="application/json"} -Body '{"order_id": "ORD-001"}'
```
*(Perhatikan di hasil output, akan ada object `fee` sebesar `amount: 500` karena transaksinya diasumsikan bernilai 100000).*

### Tahap 3.3: Mencegah Double-Charge (Idempotency)
**Narasi**: "Jika pengguna tidak sengaja menekan tombol bayar 2 kali, Gateway akan menggunakan `X-Idempotency-Key` untuk mengembalikan respons *cache* yang sama persis, dan TIDAK AKAN menagih fee untuk kedua kalinya."

**Jalankan ulang perintah Tahap 3.2 tadi (tekan Panah Atas di keyboard lalu Enter)**. Kamu akan melihat hasilnya dikembalikan instan dari Cache (tanpa memanggil mock server), lengkap dengan sisa limit Rate Limit.

### Tahap 3.4: Transaksi Lintas Aplikasi (POS / Point of Sales)
**Narasi**: "Tentu saja Gateway ini melayani banyak domain, misalnya aplikasi Point of Sales (POS)."

**(Copy paste perintah ini):**
```powershell
Invoke-RestMethod -Uri http://localhost:8080/api/v1/pos/pembayaran -Method Post -Headers @{"Authorization"="Bearer $TOKEN"; "X-Idempotency-Key"="demo-idem-pos-202"; "Content-Type"="application/json"} -Body '{"invoice_id": "INV-002"}'
```
*(Hasil output akan berasal dari server bayangan POS dan fee otomatis dikenakan).*

### Tahap 3.5: Transaksi LogistiKita (Pengiriman)
**Narasi**: "Begitu juga dengan aplikasi ekosistem lainnya seperti LogistiKita."

**(Copy paste perintah ini):**
```powershell
Invoke-RestMethod -Uri http://localhost:8080/api/v1/logistikita/pembayaran_logistik -Method Post -Headers @{"Authorization"="Bearer $TOKEN"; "X-Idempotency-Key"="demo-idem-log-303"; "Content-Type"="application/json"} -Body '{"resi": "RESI-003"}'
```

### Tahap 3.6: Rate Limiting (Mencegah Spam/Serangan)
**Narasi**: "Gateway juga dilengkapi pertahanan *Rate Limiter* untuk mencegah spam atau serangan DDoS. Mari kita coba men-spam API SmartBank."

**(Jalankan perintah di bawah ini dan tekan ENTER berkali-kali dengan cepat di terminal):**
```powershell
Invoke-RestMethod -Uri http://localhost:8080/api/v1/smartbank/manajemen_saldo -Method Get -Headers @{"Authorization"="Bearer $TOKEN"}
```
*(Jika kamu mengeksekusinya terlalu cepat, terminal akan mulai menampilkan warna merah karena ditolak dengan pesan error `429 Too Many Requests`).*

---

## Langkah 4: Penutup
1. Buka kembali halaman Dashboard UI untuk memperlihatkan lonjakan **Error Rate** akibat simulasi serangan Spam (Rate Limiter) yang baru saja kita lakukan.
2. Pindah ke menu **Request Logs**, dan tunjukkan kepada audiens bahwa request yang terblokir memiliki status `FAILED`.
3. "Sebagai infrastruktur pusat, API Gateway kami telah dirancang dengan stabil, di-cover oleh 15 Skenario Uji Otomatis (GW-T01 s/d GW-T15), mengelola perlindungan DoS, serta sanggup me-*routing* puluhan *endpoints*."
4. **Selesai**. Aplikasi API Gateway siap beroperasi menopang seluruh Ekosistem!
