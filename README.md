# airdanapi_BE

Backend API Gateway / Integrator untuk Ekosistem Ekonomi UMKM. Service ini menjadi fondasi Sprint 0 untuk routing, validasi, logging, fee integrasi, dan console API pada sprint berikutnya.

## Status Sprint 0

Fitur yang sudah tersedia:

- Go HTTP server dengan `chi`.
- Config loader dari environment variable dan `.env`.
- Standard JSON response envelope.
- Request ID middleware dengan header `X-Request-Id`.
- Basic panic recovery.
- `GET /health`.
- `GET /ready`.
- Unit test dasar untuk health dan readiness endpoint.

Belum termasuk Sprint 0:

- Database dan migration.
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

## Menjalankan Aplikasi

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

## Testing

```bash
go test ./...
```

Atau:

```bash
make test
```

## Struktur Project

```text
cmd/server/          Entry point HTTP server
internal/config/     Environment config loader
internal/handler/    HTTP handlers
internal/middleware/ HTTP middleware
internal/response/   JSON response envelope
internal/domain/     Domain model placeholder untuk sprint berikutnya
internal/repository/ Repository placeholder untuk sprint berikutnya
internal/service/    Service layer placeholder untuk sprint berikutnya
internal/store/      In-memory store placeholder untuk sprint berikutnya
migrations/          SQL migration placeholder untuk Sprint 1
```

## Catatan Penting

- Jangan commit file `.env`.
- Jangan menambahkan Redis atau Docker tanpa perubahan scope eksplisit.
- Semua transaksi moneter pada sprint berikutnya harus tetap melalui SmartBank.
- Gateway tidak boleh melakukan mutasi saldo langsung.
- Response API harus tetap memakai envelope standar.

