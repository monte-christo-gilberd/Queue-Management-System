# Queue Management System (QMS)

<p align="center">
  <a href="https://go.dev/"><img alt="Go 1.27" src="https://img.shields.io/badge/Go-1.27-00ADD8?style=for-the-badge&logo=go&logoColor=white"></a>
  <a href="https://gin-gonic.com/"><img alt="Gin 1.12" src="https://img.shields.io/badge/Gin-1.12-008ECF?style=for-the-badge"></a>
  <a href="https://www.postgresql.org/"><img alt="PostgreSQL 16" src="https://img.shields.io/badge/PostgreSQL-16-4169E1?style=for-the-badge&logo=postgresql&logoColor=white"></a>
  <a href="https://reactnative.dev/"><img alt="React Native 0.86" src="https://img.shields.io/badge/React%20Native-0.86-61DAFB?style=for-the-badge&logo=react&logoColor=black"></a>
  <a href="https://expo.dev/"><img alt="Expo SDK 57" src="https://img.shields.io/badge/Expo%20SDK-57-000020?style=for-the-badge&logo=expo&logoColor=white"></a>
  <a href="https://www.typescriptlang.org/"><img alt="TypeScript 6.0" src="https://img.shields.io/badge/TypeScript-6.0-3178C6?style=for-the-badge&logo=typescript&logoColor=white"></a>
  <a href="https://www.docker.com/"><img alt="Docker" src="https://img.shields.io/badge/Docker-Containerized-2496ED?style=for-the-badge&logo=docker&logoColor=white"></a>
  <a href="https://nginx.org/"><img alt="nginx 1.27" src="https://img.shields.io/badge/nginx-1.27-009639?style=for-the-badge&logo=nginx&logoColor=white"></a>
  <a href="https://opentelemetry.io/"><img alt="OpenTelemetry" src="https://img.shields.io/badge/OpenTelemetry-Enabled-425CC7?style=for-the-badge&logo=opentelemetry&logoColor=white"></a>
  <a href="https://grafana.com/"><img alt="Grafana" src="https://img.shields.io/badge/Grafana-Enabled-F46800?style=for-the-badge&logo=grafana&logoColor=white"></a>
  <a href="https://playwright.dev/"><img alt="Playwright" src="https://img.shields.io/badge/Playwright-Tested-2EAD33?style=for-the-badge&logo=playwright&logoColor=white"></a>
  <img alt="Web Push" src="https://img.shields.io/badge/Web%20Push-VAPID-FF6B00?style=for-the-badge">
</p>

<p align="center">
  <a href="https://github.com/monte-christo-gilberd/Queue-Management-System/releases"><img alt="Version 1.0.0" src="https://img.shields.io/badge/Version-1.0.0-blue?style=for-the-badge"></a>
  <a href="https://github.com/monte-christo-gilberd/Queue-Management-System/actions"><img alt="CI: GitHub Actions" src="https://img.shields.io/badge/CI-GitHub%20Actions-2088FF?style=for-the-badge&logo=githubactions&logoColor=white"></a>
  <img alt="Platform: Web and Android" src="https://img.shields.io/badge/Platform-Web%20%7C%20Android-blue?style=for-the-badge&logo=android&logoColor=white">
  <img alt="Status: Active" src="https://img.shields.io/badge/Status-Active-brightgreen?style=for-the-badge">
  <a href="https://github.com/monte-christo-gilberd/Queue-Management-System/pulls"><img alt="PRs Welcome" src="https://img.shields.io/badge/PRs-welcome-brightgreen?style=for-the-badge&logo=github&logoColor=white"></a>
  <a href="LICENSE"><img alt="License: MIT" src="https://img.shields.io/badge/License-MIT-yellow?style=for-the-badge"></a>
  <a href="docs/"><img alt="Docs" src="https://img.shields.io/badge/Docs-available-blue?style=for-the-badge&logo=readthedocs&logoColor=white"></a>
</p>

A queue management system for in-person services. Owners create a queue, share a QR code or join link, and call the next number. Customers join by scanning the QR code or opening the link, then follow their position and the number currently being served in real time, without refreshing the page.

> Capstone project. All services (backend, frontend, auth, notifications, email, and payments) are connected end to end. For deeper technical documentation, see [Documentation](#documentation).

## Table of Contents

- [About the Project](#about-the-project)
- [Features](#features)
- [Tech Stack](#tech-stack)
- [Architecture](#architecture)
- [Getting Started](#getting-started)
- [Configuration](#configuration)
- [Testing](#testing)
- [Production Deployment](#production-deployment)
- [Operations](#operations)
- [Project Structure](#project-structure)
- [Documentation](#documentation)
- [Known Limitations](#known-limitations)
- [Next Steps](#next-steps)
- [Disclaimer](#disclaimer)
- [License](#license)

## About the Project

QMS is designed for services whose customers are physically present and carry their own phones, such as clinics, counters, or cafés. The owner calls numbers one by one, while customers only scan a QR code and wait for a notification when their turn is near.

Key assumptions:

- Customers must log in and verify their email before joining. Anonymous joins are not supported.
- Email verification only proves ownership of the address through a link. The email domain is not checked.
- Joining is only possible with the 6-character join code from the QR code or link, which the backend generates.
- Creating a queue requires an active subscription. Regular users become owners by paying a 1-month (30-day) subscription via QRIS.
- The public display shows numbers only, never customer names.
- Queues and history are never permanently deleted.
- The backend runs as a single instance, because the SSE hub is kept in process memory.

## Features

**Owner**

- Create a queue, and change its name, description, and ticket prefix (e.g. `A`, `B1`).
- Share a QR code or join link. Static QR codes hold a fixed code that can be replaced. Dynamic QR codes rotate periodically and expire.
- Call the next number with **Next**, skip it with **Skip**, and repeat a call with **Recall**.
- Configure Skip behavior: return the number to the queue, return it after *n* numbers, or drop it, with a cap on returns.
- Manage multiple counters, each with its own name. Each counter serves one number at a time.
- Insert or remove numbers, cancel a queue, and write a note for each number.
- View queue history and export numbers to CSV or XLSX.
- Enable a public waiting-room display with a rotatable link.
- Subscribe for 1 month via QRIS to be able to create queues.

**Customer**

- Join by scanning a QR code or opening a join link, then tapping Join. Scanning again does not create a second ticket.
- View position, the number being served, and the next number on the Home or My Queues page.
- Cancel a ticket and view history.
- Receive a "Your turn" notification via Web Push (browser) or FCM (Android app).

**Public display**

- Shows the number being served per counter and the next number, without personal data.

**Account**

- Registration, login, password reset, and email verification.
- Google sign-in. An account with a password is never linked automatically. The owner must enter their password first.
- Profile photo and email notification settings.

## Tech Stack

| Layer | Technology |
|---|---|
| Backend | Go 1.27, Gin 1.12, sqlx + pgx v5 |
| Auth | JWT RS256 (golang-jwt), bcrypt |
| Database | PostgreSQL 16 (triggers, `LISTEN/NOTIFY`, outbox, per-service roles) |
| Real-time | Server-Sent Events (SSE), in-memory hub |
| Frontend | Expo SDK 57, React Native 0.86, React 19, expo-router, Zustand |
| Push | Web Push (VAPID), FCM HTTP v1 |
| Email | SMTP, `html/template` templates, Mailpit for development |
| Payments | BorderPay (QRIS), idempotent webhook |
| Infrastructure | Docker Compose, nginx 1.27, Cloudflare Tunnel (optional) |
| Observability | OpenTelemetry Collector, Loki, Prometheus, Tempo, Grafana (optional) |
| Testing | `go test`, Node test runner, Playwright |

## Architecture

A single entry point (nginx) routes each request to the right service. Supporting services are separated for security and failure isolation, not for scale.

```mermaid
flowchart LR
    Client["Browser or Android app"] --> Nginx["nginx: the only public port"]
    Nginx --> FE["frontend (Expo web)"]
    Nginx --> BE["backend (Go)"]
    Nginx --> AUTH["auth-service"]
    Nginx --> PAY["payment-service (webhook)"]
    BE --> DB["PostgreSQL 16"]
    AUTH --> DB
    PAY --> DB
    NOTIFY["notify-service"] --> DB
    MAIL["mail-service"] --> DB
    BE --> PAY
    AUTH --> MAIL
```

Principles:

- **Failure isolation.** If the email, push, or payment services go down, the queue keeps working. Queues, login, and SSE stay available.
- **Events originate in the database.** PostgreSQL triggers write email outbox rows and send `pg_notify` inside the same transaction as the data change, so there is no dual write.
- **Database privilege separation.** Each service has its own role with minimal privileges.
- **The JWT private key lives only in auth-service.** The backend verifies tokens locally with the public key.

Diagrams and flow details are in [docs/FLOW.md](docs/FLOW.md) and [docs/DESIGN.md](docs/DESIGN.md).

## Getting Started

### Prerequisites

- Docker and Docker Compose, to run the full stack.
- Or, for local development: Go 1.27, Node.js 22, PostgreSQL 16, and `psql`.
- `openssl` for generating keys.

### Run with Docker Compose (recommended)

```bash
cp .env.example .env         # fill in the secrets; QMS_PUBLIC_ORIGIN is the origin you open from other devices
docker compose up --build
```

Open `http://localhost:8080` (or the value of `QMS_PUBLIC_ORIGIN`). Only nginx publishes a port to the host. Other services are reachable only from the internal Docker network.

With the Makefile:

| Command | Purpose |
|---|---|
| `make up` | Build and start the dev stack. URLs use the LAN IP |
| `make up obs` | Same, with observability |
| `make tunnel` | Start with a temporary public HTTPS URL (the phone camera works) |
| `make down` | Stop the stack |
| `make restart` | Restart backend, auth, notify, mail, and payment |
| `make seed` | Reset the dev database to seed data |
| `make logs` / `make ps` | Logs and container status |
| `make mailpit` | Show the Mailpit UI address |
| `make verify-user EMAIL=<email>` | Mark one account as verified (when the verification email does not arrive) |

The JWT keys are generated automatically by `make up` and `make prod-up` through `scripts/gen-auth-keys.sh`. The keys are not committed to git.

### Run locally (without Docker)

**1. Database**

```bash
createdb qms_db
for f in db/migrations/01*.sql db/migrations/02_Indexes.sql; do
  psql -d qms_db -v ON_ERROR_STOP=1 -f "$f"
done
```

The schema is split by domain (`01a_accounts.sql` through `01j`), followed by `02_Indexes.sql`. File order reflects dependencies. The file map and the rules for adding schema changes are in [db/migrations/README.md](db/migrations/README.md). The per-service roles are created by the `db/migrations/*.sh` scripts, which only run automatically under Docker.

**2. JWT keys**

```bash
scripts/gen-auth-keys.sh     # creates keys/auth-private.pem and keys/auth-public.pem
```

**3. auth-service**

```bash
cd auth-service
go run ./cmd/auth-service    # http://localhost:8094
```

**4. Backend**

```bash
cd backend
go run ./cmd/server
```

**5. Frontend**

```bash
cd frontend
npm install
npm run web                  # http://localhost:8081
npm run web:lan              # reachable from other devices on the same network
```

The API address is set with `EXPO_PUBLIC_API_URL`. If it is empty, the frontend uses the host it was opened from, with port 8080.

## Configuration

Variables are read from the environment. A `.env` file at the repository root is loaded if present. Complete examples are in `.env.example` and `.env.prod.example`.

| Variable | Purpose |
|---|---|
| `DATABASE_URL` | PostgreSQL DSN, e.g. `postgres://user:pass@localhost:5432/qms_db?sslmode=disable` |
| `JWT_PUBLIC_KEY_FILE` | RSA public key used to verify tokens (backend). Required |
| `AUTH_JWT_PRIVATE_KEY_FILE` | RSA private key used to issue tokens (auth-service). Required |
| `PORT` | Backend HTTP port (default 8080) |
| `ALLOWED_ORIGINS` | Allowed frontend origins, comma-separated |
| `PUBLIC_APP_URL` | Origin that devices scanning the QR code can open. Must be HTTPS in production |
| `EMAIL_VERIFICATION_REQUIRED` | Enables email verification. Empty means enabled. Must have the same value in the backend and auth-service |
| `MAIL_SERVICE_URL`, `MAIL_SERVICE_TOKEN` | mail-service address and token. The token must be at least 32 characters in production |
| `GOOGLE_CLIENT_IDS` | Google client IDs. Google sign-in is enabled only when set |
| `SUBSCRIPTION_PRICE_IDR` | Monthly subscription price (default 15000) |
| `PAYMENT_HTTP_TIMEOUT` | Timeout in seconds for backend calls to payment-service (default 20; must be greater than 15) |
| `VAPID_PUBLIC_KEY`, `VAPID_PRIVATE_KEY`, `VAPID_SUBJECT` | VAPID keys for Web Push. Generate with `npx web-push generate-vapid-keys` |
| `FCM_PROJECT_ID`, `FCM_CREDENTIALS_FILE` | Optional, for Android app push |
| `SHUTDOWN_TIMEOUT` | Seconds to wait for in-flight requests on shutdown (default 8) |
| `LOG_LEVEL` | `debug`, `info`, `warn`, or `error` (default `info`) |
| `GRAFANA_ADMIN_PASSWORD`, `METRICS_DB_PASSWORD` | Required when observability is enabled |

Notes:

- `PUBLIC_APP_URL` must not be `localhost` when testing from a phone.
- With `APP_ENV=production`, the backend refuses to start if `ALLOWED_ORIGINS` contains `*` or a non-HTTPS origin. auth-service refuses to start if `PUBLIC_APP_URL` is not HTTPS.
- Secrets are not baked into Docker images.
- Variables prefixed with `EXPO_PUBLIC_` are included in the frontend bundle. Never put secrets there.

## Testing

```bash
# Backend unit tests (integration packages are skipped without a build tag)
cd backend && go test ./... -count=1

# Backend integration tests (requires an accessible DATABASE_URL)
cd backend && go test -tags integration ./internal/test/integration_test/ -count=1 -v

# Other services, each a separate Go module
cd auth-service && go test ./...
cd notify-service && go test ./...
cd mail-service && go test ./...
cd payment-service && go test ./...

# Frontend unit tests
cd frontend && npm run test:unit

# End-to-end tests (requires the backend and frontend to be running)
cd frontend && npm run e2e
```

Number of top-level `Test` functions per module, counted from the code: backend 518, auth-service 158, mail-service 83, notify-service 60, payment-service 22. Frontend unit tests: 208. End-to-end tests use Playwright under `frontend/playwright/` (33 spec files).

## Production Deployment

Production uses `docker-compose.prod.yml` behind a reverse proxy and an HTTPS tunnel.

```bash
cp .env.prod.example .env.prod && chmod 600 .env.prod   # fill in the secrets (openssl rand -hex 32) and new VAPID keys
make prod-up                 # check configuration, prepare database roles, then docker compose up -d --build
make prod-payment-check      # read-only: payment configuration and payment_rw role privileges
make prod-seed-penguji       # test owner and customer accounts
make prod-backup             # database backup (pg_dump, gzip)
make prod-down               # stop the production stack
```

Things to keep in mind:

- nginx binds only to `127.0.0.1:8080`. The tunnel (e.g. `cloudflared`) points to `http://localhost:8080`.
- Client IPs are read from `CF-Connecting-IP`.
- The production database uses its own volume and is not seeded.
- `make prod-reset` deletes all production data after an automatic backup. Use it only when you really need to.
- The backend runs as a single instance because the SSE hub lives in memory.
- To build the frontend as a static web app: `EXPO_PUBLIC_API_URL=https://your-domain npx expo export --platform web --clear`. The output is in `frontend/dist/`.

## Operations

### Scripts

All scripts are in `scripts/` and `db/scripts/`. Details are in [scripts/README.md](scripts/README.md).

| Script | Purpose |
|---|---|
| `scripts/gen-auth-keys.sh` | Generates the RSA key pair for tokens. Idempotent |
| `scripts/backup-prod.sh` | Backs up and restores the production database. Restore overwrites data |
| `scripts/reset-demo.sh` | Full reset of the production database, with an automatic backup first |
| `scripts/verify-user.sh` | Marks one email as verified directly in the database |
| `scripts/check-mail-mode.sh` | Checks the email configuration before the production stack starts |
| `scripts/check-google-config.sh` | Ensures the server and frontend Google sign-in settings match |
| `scripts/check-service-isolation.sh` | Checks service isolation and observability configuration |
| `scripts/check-observability-leaks.sh` | Secret-leak test across logs, traces, and metric labels |
| `db/scripts/apply_schema.sql` | Idempotent schema adjustments for an existing database. Does not touch data |
| `db/scripts/reset_seed_dev.sql` | Resets the dev database to seed data |
| `db/scripts/backfill_currently_served.sql` | One-off backfill. Ends with `ROLLBACK`: review the output, then switch to `COMMIT` |

The role scripts in `db/migrations/*.sh` run automatically in Docker when the database volume is empty. For an existing database, `make prod-up` re-runs `08_payment_role.sh`, which is idempotent.

### Push notifications

The trigger `tickets_notify_trg` sends `pg_notify('queue_events')` when a number is called, recalled, skipped, or returned to the queue. `notify-service` listens for these events, determines the recipients, and sends:

- **Web Push** to browsers, using VAPID, with a TTL of 60 seconds. Subscriptions are stored in `push_subscriptions`.
- **FCM** to the Android app, using a Firebase service account. Tokens are stored in `push_devices`.

Invalid subscriptions are removed automatically. Push does not use an outbox, so notifications sent while the service is down are not retried.

Notes:

- Web Push only works over HTTPS in a supporting browser. On iPhone and iPad, the site must be added to the Home Screen.
- The Android app requires an APK built with EAS Build. Expo Go does not support these notifications.
- One browser or phone serves one account at a time. Logging out removes its subscription.

To enable FCM, create a Firebase project, add an Android app with your own package name (e.g. `com.example.qms`), enable the Firebase Cloud Messaging API V1, then:

1. Place `google-services.json` in `frontend/` for local builds. For EAS, register it as the secret file `GOOGLE_SERVICES_JSON`.
2. Store the service account credentials as `keys/fcm/fcm-sa.json`. Grant read access only to the notify-service user, without changing the folder owner.
3. Set `FCM_PROJECT_ID` and `FCM_CREDENTIALS_FILE=/run/fcm/fcm-sa.json`, run `apply_schema.sql`, then restart the backend and notify-service.
4. Check that the notify-service log shows `FCM push enabled`.

### Email

`mail-service` is the only service that sends email. Callers, mainly auth-service, use `POST /internal/v1/send` with the `X-Service-Token` header.

Ticket notification emails (joined the queue, cancelled, removed, served, and queue closed) are written by triggers into `notification_outbox` in the same transaction. A worker sends them with retries and increasing backoff. If SMTP or mail-service is down, emails wait in the outbox and are sent after recovery.

Production email mode is set with `MAIL_MODE`:

| Mode | Where email goes | Account verification |
|---|---|---|
| `mailpit` (production default) | Mailpit, not delivered to anyone | Get the link from Mailpit, or use `make verify-user` |
| `smtp` | Real recipients via SMTP | The user clicks the link in their inbox |

When `MAIL_MODE=smtp`, set `SMTP_HOST`, `SMTP_PORT`, `SMTP_FROM`, `SMTP_USER`, `SMTP_PASSWORD`, and `SMTP_TLS` (`starttls` or `tls`). `scripts/check-mail-mode.sh` rejects invalid configurations.

To test in development without waiting for a queue event: `docker compose exec mail-service go run ./cmd/mail-service send-test you@example.com`.

### Subscription payments

Owners pay for a 1-month subscription via QRIS through BorderPay:

1. The frontend calls `POST /api/v1/owners/subscription/checkout`. The backend sets the price and creates a `reference_id`.
2. The backend calls payment-service, which records a `pending` order and creates a transaction at BorderPay.
3. The user pays using the `pay_url`.
4. BorderPay sends `payment.paid` to `/api/v1/webhooks/borderpay`. In a single transaction, payment-service marks the order as paid and extends `owners.subscription_expires_at` by 30 days. Duplicate webhooks do not apply the payment twice.

The backend only reads the subscription expiry date. All payment state changes happen in payment-service.

On the payment provider dashboard, make sure:

- The webhook endpoint points to `https://<domain>/api/v1/webhooks/borderpay`.
- The verification token matches `BORDERPAY_WEBHOOK_TOKEN`.
- The firewall or WAF allows the webhook path. If it is blocked, paid transactions are not recorded.

| Webhook response status | Meaning |
|---|---|
| 200 | Applied, already applied, or unknown reference (ignored) |
| 401 | Webhook token mismatch |
| 409 | Amount does not match the order. Check manually |
| 500 | Storage failed. The provider retries after the cause is fixed |

### Observability

The `docker-compose.observability.yml` overlay adds OpenTelemetry Collector, Loki, Prometheus, Tempo, and Grafana. The application does not depend on it: if the overlay is off, the queue, login, SSE, and subscriptions keep working.

```bash
make obs-up                  # dev
make obs-up OBS_TARGET=prod  # production
make obs-url                 # Grafana address, bound to 127.0.0.1 only
make obs-check               # isolation checks and secret-leak tests
```

- **Logs**: one JSON line per event, including `request_id`. Collected from Docker logs and shipped to Loki.
- **Metrics**: Prometheus scrapes `/metrics` from each Go service on a separate port that is not published to the host. Dashboards cover overview, queues, subscriptions and email, auth, infrastructure, PostgreSQL, and nginx.
- **Traces**: backend, auth, and mail send spans to the collector when `OTEL_EXPORTER_OTLP_ENDPOINT` is set. Sampling is 10% in production.
- **Alerts**: 11 rules defined as code in `observability/grafana/provisioning/alerting/rules.yaml`. Their state is visible in the UI, but they do not send notifications yet.

Grafana is reachable only from 127.0.0.1. From another device, use an SSH tunnel: `ssh -L 3000:127.0.0.1:3000 <user>@<server>`.

Every log line includes `request_id`. To trace one request across services, filter by `request_id` in Grafana.

## Project Structure

```text
.
├── auth-service/        Login, registration, Google sign-in, password reset, email verification, JWT issuer
├── backend/             Main API: queues, tickets, counters, public display, SSE
│   └── internal/        app, config, handler, service, repository, sse, middleware, etc.
├── db/                  Database Dockerfile, per-domain migrations, operational scripts
├── deploy/nginx/        Reverse proxy configuration (dev and prod)
├── docs/                Design, flow, API contract, ERD, and product documents
├── frontend/            Expo: web and Android app
│   ├── playwright/      End-to-end tests
│   └── src/             app (routing), components, hooks, lib, store
├── mail-service/        The only email sender: internal API, outbox, templates, SMTP
├── notify-service/      Web Push and FCM push driven by Postgres LISTEN
├── observability/       Collector, Loki, Prometheus, Tempo, and Grafana configuration
├── payment-service/     Subscription checkout and BorderPay webhook
├── scripts/             Operational scripts
├── docker-compose.yml                Dev stack
├── docker-compose.prod.yml           Production stack
├── docker-compose.observability.yml  Observability overlay
└── Makefile             Everyday commands
```

## Documentation

- [docs/DESIGN.md](docs/DESIGN.md): architecture, design decisions, and constraints.
- [docs/FLOW.md](docs/FLOW.md): user flows and sequence diagrams.
- [docs/api-contract.md](docs/api-contract.md): HTTP and SSE contract.
- [docs/product-overview.md](docs/product-overview.md): product requirements.
- [docs/ERD.md](docs/ERD.md) and [docs/Architecture_Summary.md](docs/Architecture_Summary.md): schema and architecture summary.
- [deploy/nginx/README.md](deploy/nginx/README.md): nginx setup.
- [scripts/README.md](scripts/README.md): script reference.

If the documentation and the code disagree, the code is correct.

## Known Limitations

- **Single backend instance.** The SSE hub lives in process memory. Scaling out requires shared pub/sub.
- **Push without an outbox.** Notifications sent while notify-service is down are not retried. Email uses an outbox and is not affected.
- **No migration runner.** The schema is applied with idempotent SQL scripts, and the rules for changes are in `db/migrations/README.md`.
- **No scheduled backups.** Backups are manual, using `make prod-backup`.
- **Google sign-in attempt counter is in memory.** It resets when auth-service restarts and is not shared between instances.
- **Email verification does not check the domain.** There is no MX check or allowlist.
- **No rate limiting outside nginx.** Endpoints under `/api/` without a `limit_req` are not rate-limited.
- **No history limit.** Export returns all numbers of a closed queue.
- **Not all SSE events come from triggers.** Some changes are still sent from Go code after commit.

## Next Steps

- Add nginx rate limiting for the email verification and resend endpoints.
- Fix the demo reset script so it also clears `queue_displays`.
- Narrow the `tickets_notify` trigger to fire only on `currently_served`, after the backfill has been verified.
- Add concurrency tests for Remove and Cancel against Skip, and for deleting a counter against Next.
- Features: change password and email from the profile, QR code on the public display, push for cancellations and removals, and an outbox for Web Push.
- A migration runner, scheduled backups, and load testing.

## Disclaimer

This project was created for learning and portfolio purposes only. It is not affiliated with any company, client, or organization. All names, data, and assets used in this project are fictionalized or sanitized for public use.

## License

This project is licensed under the MIT License. See [LICENSE](LICENSE) for details.