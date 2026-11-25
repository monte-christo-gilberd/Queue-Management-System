# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

## [1.0.0] - 2026-11-24

### Added

- End-to-end queue management for owners, customers, and public displays.
- Real-time updates over Server-Sent Events (SSE) without page refresh.
- Queue creation with custom ticket prefixes, QR join links (static and rotating), and multi-counter support.
- Owner controls: Next, Skip, Recall, insert/remove numbers, cancel queue, notes, and CSV/XLSX export.
- Customer flow: join by QR/link, live position, cancel ticket, and queue history.
- Authentication: JWT RS256, registration, login, password reset, email verification, and Google sign-in.
- Subscription billing: 1-month QRIS checkout through BorderPay with idempotent webhook handling.
- Notifications: Web Push (VAPID) and FCM for Android.
- Microservices for auth, mail, notify, and payment, with per-service PostgreSQL roles.
- PostgreSQL outbox and `LISTEN/NOTIFY` for email and push events.
- Docker Compose stacks for development and production, nginx reverse proxy, and optional Cloudflare Tunnel.
- Observability overlay: OpenTelemetry Collector, Loki, Prometheus, Tempo, Grafana, dashboards, and alert rules.
- Unit tests for Go services and frontend, plus Playwright end-to-end tests.
- Technical documentation: design, flows, API contract, ERD, and product overview.

### Changed

- Backend serves all queue and QR functionality; the separate QR service was removed.
- Frontend uses Expo Router and Zustand for routing and state.
- Owner route group renamed to user-facing routes.
- README rewritten in English with a clearer project overview and setup guide.

### Fixed

- Prevented duplicate subscription extension after successful BorderPay payments.
- Fixed push notifications on Brave and Android.
- Fixed owner history showing a placeholder instead of the customer name.
- Fixed dark mode across web and Android.
- Hardened configuration checks, nginx webhook handling, and production seed scripts.

### Security

- JWT private key isolated to auth-service; other services verify with the public key.
- Minimal-privilege PostgreSQL roles per service.
- Secrets, `.env`, and key directories excluded from version control.
- Webhook verification token and amount checks for payment callbacks.

### Known Limitations

- Backend runs as a single instance because the SSE hub is in memory.
- Push notifications do not use an outbox and are not retried after notify-service downtime.
- No migration runner; schema changes use idempotent SQL scripts.
- No scheduled backups; backups are manual.
- Email verification does not validate the email domain.
- Rate limiting is only configured in nginx.
- Export returns all numbers of a closed queue.

[Unreleased]: https://github.com/monte-christo-gilberd/Queue-Management-System/compare/v1.0.0...HEAD
[1.0.0]: https://github.com/monte-christo-gilberd/Queue-Management-System/releases/tag/v1.0.0