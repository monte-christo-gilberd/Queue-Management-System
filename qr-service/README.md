# qr-service

Microservice QR untuk QMS. Tanggung jawabnya:

- membuat dan memverifikasi **token join** bertanda tangan (statis & dinamis),
- merender **PNG QR** untuk tautan join dan kebutuhan internal lain (mis. QR pembayaran),
- menjadi **satu-satunya pemegang** `JOIN_LINK_SECRET`.

Service ini **stateless**: tidak mengakses database, tidak menyimpan token, dan tidak tahu
apakah sebuah antrian masih aktif. Keputusan bisnis (kepemilikan antrian, penggantian kode, join tiket)
tetap milik backend.

## Menjalankan lokal

```bash
cp ../.env.example .env   # lalu pakai bagian [qr-service/.env] di dalamnya
go run ./cmd/qr-service
```

Atau lewat Docker:

```bash
docker build -t qms/qr-service .
docker run --rm -p 8090:8090 --env-file .env qms/qr-service
```

Health check: `GET http://localhost:8090/health`.

## Endpoint

### Publik

| Method | Path | Keterangan |
|---|---|---|
| GET | `/health` | Liveness, `{"status":"ok"}` |
| GET | `/ready` | Readiness, `{"status":"ready"}` |

Tidak ada endpoint gambar publik. QR digambar di frontend dari `join_url` yang dikembalikan backend.

### Internal (`X-Service-Token` wajib)

| Method | Path | Body |
|---|---|---|
| POST | `/internal/v1/join-token` | `{queue_id, type, ttl_seconds?}` |
| POST | `/internal/v1/join-token/verify` | `{token}` |
| POST | `/internal/v1/qr/render` | `{data, size?, level?, fg?, bg?}` → `image/png` |

Endpoint internal **tidak** diproksikan nginx dan hanya boleh diakses dari jaringan internal.

Contoh:

```bash
curl -s -X POST http://localhost:8090/internal/v1/join-token \
  -H "X-Service-Token: $QR_SERVICE_TOKEN" \
  -H 'Content-Type: application/json' \
  -d '{"queue_id":12,"type":"static"}'
```

## Format token

```
v1.<base64url(payload)>.<base64url(HMAC-SHA256(secret, "v1."+payload))>
```

Payload:

```json
{ "qid": 12, "typ": "static", "jti": "…", "exp": 1760000000 }
```

- `qid` — id antrian (> 0)
- `typ` — `static` atau `dynamic`
- `jti` — id acak 16 byte; membuat token hasil penggantian selalu berbeda
- `exp` — hanya token dinamis (Unix detik)

Token statis **tidak** punya `exp`. Backend wajib mencocokkannya dengan
`queue_settings.qr_current_code`, sehingga penggantian otomatis mencabut token lama.

## Konfigurasi

| Env | Wajib | Default | Keterangan |
|---|---|---|---|
| `PORT` | tidak | `8090` | Port HTTP |
| `JOIN_LINK_SECRET` | ya | — | HMAC secret, minimal 32 byte |
| `QR_DYNAMIC_TTL_SECONDS` | tidak | `120` | TTL token dinamis (30–600) |
| `QR_SERVICE_TOKEN` | ya | — | Token internal untuk backend |
| `LOG_LEVEL` | tidak | `info` | `debug`/`info`/`warn`/`error` |


## Kode error

| HTTP | Code | Arti |
|---|---|---|
| 400 | `QR_MALFORMED` | Token rusak/format tidak dikenal |
| 400 | `QR_TYPE_INVALID` | `type` bukan `static`/`dynamic` |
| 401 | `QR_INVALID` | Tanda tangan salah |
| 401 | `UNAUTHORIZED` | `X-Service-Token` salah/kosong |
| 410 | `QR_EXPIRED` | Token dinamis kedaluwarsa |
| 413 | `BODY_TOO_LARGE` | Body melebihi 16 KB |

## Pengujian

```bash
go vet ./...
go test -race ./...
```

## Catatan keamanan

- Jangan mencatat token utuh di log. Logger hanya mencatat path, bukan query string.
- Render hanya memeriksa tanda tangan. Token statis yang sudah diganti masih bisa
  dirender gambarnya, tetapi **ditolak saat join** karena backend membandingkannya
  dengan token aktif.
- `/internal/*` harus diblokir di reverse proxy (`location /internal/ { return 404; }`).
