# airdanapi_BE

Backend API Gateway / Integrator untuk Ekosistem Ekonomi UMKM. Service ini menjadi fondasi Sprint 0 untuk routing, validasi, logging, fee integrasi, dan console API pada sprint berikutnya.

Dokumen akhir proyek tersedia di [DOKUMENTASI_AKHIR.md](DOKUMENTASI_AKHIR.md). Panduan demo tersedia di [DEMO.md](DEMO.md).

## Status Sprint 1

Fitur yang sudah tersedia:

- Go HTTP server dengan `chi`.
- Config loader dari environment variable dan `.env`.
- Standard JSON response envelope.
- Request ID middleware dengan header `X-Request-Id`.
- Basic panic recovery.
- `GET /health`.
- `GET /ready`.
- Unit test dasar untuk health dan readiness endpoint.
- SQL schema manual untuk MySQL.
- Domain model dan repository dasar untuk tabel utama Gateway.
- Seed route registry minimal.

Belum termasuk Sprint 1:

- JWT validation.
- Routing proxy.
- Audit logging ke MySQL.
- Fee Gateway 0.5%.
- Rate limiting, idempotency, dan circuit breaker.

## Requirements

- Go 1.22 atau lebih baru.
- Git.
- MySQL 8 akan dibutuhkan mulai Sprint 1, tetapi belum wajib untuk menjalankan Sprint 0.

## Instalasi

```bash
git clone https://github.com/airdanapi/airdanapi_BE.git
cd airdanapi_BE
go mod tidy
```

Salin contoh environment:

```bash
cp .env.example .env
```

Pada Windows PowerShell:

```powershell
Copy-Item .env.example .env
```

## Environment Variable

| Variable | Default | Keterangan |
|---|---|---|
| `APP_ENV` | `development` | Environment aplikasi. |
| `APP_PORT` | `8080` | Port HTTP server. |
| `APP_NAME` | `airdanapi-integrator` | Nama aplikasi pada response health. |
| `APP_VERSION` | `0.1.0` | Versi aplikasi pada response health. |
| `DB_HOST` | `localhost` | Host MySQL. |
| `DB_PORT` | `3306` | Port MySQL. |
| `DB_USER` | `root` | User MySQL. |
| `DB_PASS` | kosong | Password MySQL. |
| `DB_NAME` | `airdanapi_gateway` | Nama database aplikasi. |
| `DB_PARSE_TIME` | `true` | Mengaktifkan parsing kolom waktu MySQL. |
| `DB_LOC` | `Local` | Zona waktu driver MySQL untuk kolom waktu. |
| `JWT_ISSUER` | `smartbank` | Issuer JWT yang dipercaya Gateway. |
| `JWT_AUDIENCE` | `ecosystem` | Audience JWT yang wajib ada pada token. |
| `JWT_PUBLIC_KEY_PEM` | kosong | Public key RSA PEM untuk validasi JWT RS256 di mode dev. |
| `JWT_CLOCK_SKEW_SECONDS` | `30` | Toleransi clock skew validasi JWT. |
| `CORS_ALLOWED_ORIGINS` | `*` | Daftar origin CORS, dipisahkan koma. |
| `SMARTBANK_BASE_URL` | `http://localhost:8101` | Base URL SmartBank jika `SMARTBANK_MODE=http`. |
| `SMARTBANK_MODE` | `mock_success` | Mode SmartBank: `mock_success`, `mock_failure`, `mock_timeout`, atau `http`. |
| `SMARTBANK_TIMEOUT_MS` | `5000` | Timeout call SmartBank. |
| `GATEWAY_REVENUE_USER` | `GATEWAY_REVENUE` | Akun tujuan fee Gateway di SmartBank. |
| `GATEWAY_FEE_RATE` | `0.005` | Rate fee Gateway. |
| `RATE_LIMIT_READ_PER_MINUTE` | `60` | Rate limit route read per user. |
| `RATE_LIMIT_TRANSACTIONAL_PER_MINUTE` | `10` | Rate limit route transactional per user. |
| `TRANSACTION_COOLDOWN_SECONDS` | `10` | Jeda antar transaksi per user. |
| `TRANSACTION_DAILY_LIMIT` | `10` | Maksimal transaksi harian per user. |
| `IDEMPOTENCY_TTL_HOURS` | `24` | TTL cache idempotency transactional. |
| `CIRCUIT_OPEN_SECONDS` | `60` | Durasi circuit breaker berada di state OPEN. |

## Setup Database Manual

Sprint 1 memakai import SQL manual, bukan migration tool otomatis.

Buat database:

```bash
mysql -u root -e "CREATE DATABASE IF NOT EXISTS airdanapi_gateway;"
```

Jika MySQL memakai password:

```bash
mysql -u root -p -e "CREATE DATABASE IF NOT EXISTS airdanapi_gateway;"
```

Import schema dan seed:

```bash
mysql -u root airdanapi_gateway < migrations/001_init_schema.up.sql
mysql -u root airdanapi_gateway < migrations/002_seed_routes.up.sql
```

Rollback manual:

```bash
mysql -u root airdanapi_gateway < migrations/002_seed_routes.down.sql
mysql -u root airdanapi_gateway < migrations/001_init_schema.down.sql
```

Urutan file penting: jalankan `001_init_schema.up.sql` sebelum `002_seed_routes.up.sql`.

Jika database lokal sudah pernah di-seed sebelum alignment CSV, reload seed route:

```powershell
mysql -u root airdanapi_gateway < migrations/002_seed_routes.down.sql
mysql -u root airdanapi_gateway < migrations/002_seed_routes.up.sql
```

Jika MySQL memakai password, gunakan `mysql -u root -p ...`.

## Menjalankan Aplikasi

Untuk menjalankan backend di lokal dari workspace tugas besar:

```powershell
cd "D:\Kuli Ah S4\RPL_II new\Tugas_Besar\airdanapi_BE"
go mod tidy
Copy-Item .env.example .env
go run ./cmd/server
```

Jika file `.env` sudah ada, langkah `Copy-Item .env.example .env` tidak perlu diulang.

Backend tetap bisa berjalan tanpa MySQL aktif. Jika MySQL belum siap, aplikasi akan menampilkan warning dan endpoint `/ready` tidak melakukan ping database.

```bash
go run ./cmd/server
```

Atau memakai Makefile:

```bash
make run
```

Server default berjalan di:

```text
http://localhost:8080
```

Verifikasi cepat:

```powershell
Invoke-RestMethod http://localhost:8080/health
Invoke-RestMethod http://localhost:8080/ready
```

Jika ingin menjalankan backend dengan database lokal, siapkan MySQL lebih dulu:

```powershell
mysql -u root -e "CREATE DATABASE IF NOT EXISTS airdanapi_gateway;"
mysql -u root airdanapi_gateway < migrations/001_init_schema.up.sql
mysql -u root airdanapi_gateway < migrations/002_seed_routes.up.sql
go run ./cmd/server
```

Jika MySQL memakai password, gunakan `mysql -u root -p ...`.

Backend dan frontend dijalankan di terminal terpisah. Setelah backend aktif di `http://localhost:8080`, jalankan frontend dari folder `airdanapi_FE`.

## Endpoint Sprint 0

### Health Check

```http
GET /health
```

Contoh response:

```json
{
  "success": true,
  "request_id": "generated-request-id",
  "data": {
    "env": "development",
    "name": "airdanapi-integrator",
    "status": "ok",
    "version": "0.1.0"
  },
  "timestamp": "2026-05-23T20:00:00+07:00"
}
```

### Readiness Check

```http
GET /ready
```

Contoh response:

```json
{
  "success": true,
  "request_id": "generated-request-id",
  "data": {
    "status": "ready"
  },
  "timestamp": "2026-05-23T20:00:00+07:00"
}
```

## Endpoint Sprint 2

### Validasi Request

```http
POST /integrator/validasi_request
Authorization: Bearer <jwt-rs256>
Content-Type: application/json
```

Request:

```json
{
  "service": "marketplace",
  "feature": "checkout",
  "method": "POST"
}
```

Response sukses:

```json
{
  "success": true,
  "request_id": "generated-request-id",
  "data": {
    "valid": true,
    "user_id": "user_123",
    "roles": ["umkm_owner"],
    "scopes": ["marketplace:write"],
    "exp": 1716300000
  },
  "timestamp": "2026-05-24T10:00:00+07:00"
}
```

Error auth utama:

- `401 AUTH_TOKEN_MISSING` jika header Authorization tidak ada.
- `401 AUTH_INVALID_TOKEN` jika token invalid, expired, signature salah, atau JTI aktif di blacklist.
- `403 AUTH_SCOPE_DENIED` jika scope token tidak memenuhi `required_scope` route.

Audit logging Sprint 2 mencatat lifecycle `STARTED` lalu `COMPLETED` atau `FAILED` ke tabel `request_logs` jika database tersedia. Jika database tidak tersedia, request tetap diproses dan kegagalan insert log dicatat sebagai warning.

**Dokumentasi lengkap proyek ini ada di [DOKUMENTASI_AKHIR.md](./DOKUMENTASI_AKHIR.md)**.

## Pengujian (Test Scenarios)
Terdapat 15 skenario pengujian utama (GW-T01 - GW-T15) untuk memverifikasi fungsionalitas Gateway.
Untuk menjalankan seluruh test (dengan data race detector):
```bash
go test -race -v -run TestGW ./internal/handler/...
go test -race ./...
```

## Endpoint Sprint 3

### Transparent Routing

```http
POST /api/v1/marketplace/checkout?order=123
Authorization: Bearer <jwt-rs256>
Content-Type: application/json
X-Idempotency-Key: demo-key
```

Request body diteruskan apa adanya ke `downstream_url` route aktif pada tabel `routes_registry`. Query string juga diteruskan tanpa modifikasi.

### Envelope Routing

```http
POST /integrator/routing_api
Authorization: Bearer <jwt-rs256>
Content-Type: application/json
```

Request:

```json
{
  "target_service": "marketplace",
  "target_feature": "checkout",
  "method": "POST",
  "payload": {
    "amount": 10000
  }
}
```

Transparent proxy mengembalikan response downstream secara raw: status code, `Content-Type`, dan body dari downstream diteruskan apa adanya. Error yang dibuat Gateway tetap memakai envelope standar.

`POST /integrator/routing_api` selalu mengembalikan envelope Gateway. Pada route non-transactional, body downstream masuk ke `data`. Pada route transactional, envelope juga memuat `fee`.

Mulai Sprint 4, response sukses route transactional (`transactional=true`) dibungkus envelope Gateway agar field `fee` bisa dikembalikan. Route non-transactional tetap raw downstream.

Error routing utama:

- `404 ROUTE_NOT_FOUND` jika route aktif tidak ditemukan di registry.
- `403 AUTH_SCOPE_DENIED` jika token tidak memiliki scope route.
- `508 LOOP_DETECTED` jika `X-Hop-Count >= 3`.
- `502 UPSTREAM_TIMEOUT` jika downstream melewati timeout route.
- `502 UPSTREAM_FAILED` jika downstream gagal karena error transport non-timeout.
- `503 DATABASE_UNAVAILABLE` jika route registry tidak tersedia.

## Endpoint Sprint 4

### Fee Transactional

Untuk route dengan `transactional=true`, Gateway membaca `transaction_amount` dari response downstream:

```json
{
  "transaction_amount": 100000
}
```

atau:

```json
{
  "data": {
    "transaction_amount": 100000
  }
}
```

Gateway menghitung fee `ROUND(transaction_amount * 0.005)` dan memanggil SmartBank dengan idempotency key `fee-{request_id}`. Response sukses transactional:

```json
{
  "success": true,
  "request_id": "generated-request-id",
  "data": {
    "transaction_amount": 100000,
    "status": "paid"
  },
  "fee": {
    "amount": 500,
    "status": "success"
  },
  "timestamp": "2026-05-24T10:00:00+07:00"
}
```

Jika SmartBank gagal atau timeout, transaksi downstream tidak di-rollback. Gateway menyimpan fee sebagai `PENDING` dan mengembalikan:

```json
{
  "fee": {
    "amount": 500,
    "status": "deferred"
  }
}
```

### Query Gateway Fees

```http
GET /integrator/biaya_layanan_integrasi?status=PENDING&page=1&per_page=20
Authorization: Bearer <jwt-rs256-admin-read>
```

Scope wajib: `admin:read`.

### Query Request Logs

```http
GET /integrator/logging?target_app=marketplace&page=1&per_page=20
Authorization: Bearer <jwt-rs256-admin-read>
```

Scope wajib: `admin:read`.

Filter yang tersedia:

- `user_id`
- `request_id`
- `from` dan `to` dalam format RFC3339
- `status_code`
- `target_app`
- `page`
- `per_page`

### Manual Retry Fee

```http
POST /integrator/biaya_layanan_integrasi/retry/{id}
Authorization: Bearer <jwt-rs256-admin-write>
```

Scope wajib: `admin:write`.

## Endpoint Sprint 5

Protection middleware berlaku hanya untuk route routing:

- `/api/v1/{service}/{feature}`
- `/integrator/routing_api`

Proteksi yang aktif:

- Rate limit per `(user_id, route_class)`.
- Cooldown transaksi per user.
- Maksimal transaksi harian per user.
- Idempotency wajib untuk route transactional.
- Circuit breaker per downstream service.

Route transactional wajib mengirim:

```http
X-Idempotency-Key: unique-key-per-transaction
```

Error protection utama:

- `400 VALIDATION_FAILED` jika route transactional tidak memiliki `X-Idempotency-Key`.
- `409 IDEMPOTENCY_CONFLICT` jika key sama dipakai dengan body berbeda.
- `429 RATE_LIMITED` jika rate limit, cooldown, atau limit harian terlampaui.
- `503 CIRCUIT_OPEN` jika downstream circuit sedang open.

Replay dengan `X-Idempotency-Key` dan body yang sama mengembalikan cached response, tidak memanggil downstream ulang, dan tidak memungut fee ulang.

## Testing

```bash
go test ./...
```

Atau:

```bash
make test
```

Integration test repository MySQL hanya berjalan jika `INTEGRATION_DB_TEST=1`.

### Test Scenarios Sprint 8

Sprint 8 menambahkan test terpadu `internal/handler/gateway_scenarios_test.go` untuk 15 skenario GW-T01 sampai GW-T15:

- `GW-T01` read request sukses.
- `GW-T02` transactional request plus fee.
- `GW-T03` missing JWT.
- `GW-T04` expired JWT.
- `GW-T05` revoked token.
- `GW-T06` unknown route.
- `GW-T07` rate limit exceeded.
- `GW-T08` idempotency replay.
- `GW-T09` idempotency conflict.
- `GW-T10` downstream timeout.
- `GW-T11` circuit open.
- `GW-T12` fee charge failed.
- `GW-T13` scope denied.
- `GW-T14` log query.
- `GW-T15` concurrent calls.

Jalankan test skenario:

```bash
go test -race -v -run TestGW ./internal/handler/...
```

Jalankan seluruh backend test dengan race detector:

```bash
go test -race ./...
```

Buat database test dan import SQL:

```bash
mysql -u root -e "CREATE DATABASE IF NOT EXISTS airdanapi_gateway_test;"
mysql -u root airdanapi_gateway_test < migrations/001_init_schema.up.sql
mysql -u root airdanapi_gateway_test < migrations/002_seed_routes.up.sql
```

Jalankan integration test di Windows PowerShell:

```powershell
$env:INTEGRATION_DB_TEST="1"
$env:DB_NAME="airdanapi_gateway_test"
go test ./...
```

Jika `INTEGRATION_DB_TEST` tidak di-set, integration test akan otomatis skip.

## Struktur Project

```text
cmd/server/          Entry point HTTP server
internal/config/     Environment config loader
internal/handler/    HTTP handlers
internal/middleware/ HTTP middleware
internal/response/   JSON response envelope
internal/domain/     Domain model Gateway
internal/repository/ MySQL connection dan repository dasar
internal/service/    Service layer placeholder untuk sprint berikutnya
internal/store/      In-memory store placeholder untuk sprint berikutnya
migrations/          SQL schema dan seed manual
```

## Catatan Penting

- Jangan commit file `.env`.
- Jangan menambahkan Redis atau Docker tanpa perubahan scope eksplisit.
- Sprint 1 memakai import SQL manual; tidak ada `cmd/migrate`.
- Semua transaksi moneter pada sprint berikutnya harus tetap melalui SmartBank.
- Gateway tidak boleh melakukan mutasi saldo langsung.
- Response API harus tetap memakai envelope standar.
