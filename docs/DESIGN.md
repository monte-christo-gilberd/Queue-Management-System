# Design: Queue Management System

This document describes the architecture, design decisions, and constraints of the system. For request and response shapes
see [api-contract.md](api-contract.md). For user flows see [FLOW.md](FLOW.md).

## 1. System overview

A queue system for in-person services. There are two roles:

- **Owner**: creates queues, calls numbers (Next, Skip, Recall), manages counters, inserts or removes numbers, writes notes, turns on the public display, views and exports history, and closes queues. The owner must have an active subscription. The subscription is paid through QRIS BorderPay (`POST /api/v1/owners/subscription/checkout`, then the `payment.paid` webhook extends it by 30 days). A regular user's first payment also makes them an owner.
- **Customer**: logs in, joins a queue with a join code (from a QR code or a link), sees the number being served and their position in real time, cancels their ticket, and views their ticket history. Receives turn notifications (Web Push in the browser, FCM in the Android app) and emails.

The system consists of five Go services (the queue backend, `auth-service`, `notify-service`, `mail-service`, and `payment-service`), one PostgreSQL database, one Expo frontend, and nginx. The cross-service summary is in the Architecture section of [README.md](../README.md). `payment-service` handles subscription payments through QRIS BorderPay as a separate service.

```
   Browser (Expo web / native)
        │  HTTP (REST, JSON)  +  SSE (fetch stream)
        ▼
   nginx  ── frontend (Metro in dev, static in prod)
        │  /api/  ──────────────►  Backend Go (Gin) :8080 ──► PostgreSQL (qms_db)
        │  /api/v1/users/ (login, register, google, forgot/reset password, verify-email)
        │          ──────────────►  auth-service :8094 ──► PostgreSQL (auth_rw)
        │                              └──► mail-service :8092 (password reset and verification emails)
        │  /api/v1/webhooks/borderpay ─► payment-service :8095 ──► PostgreSQL (payment_rw) + BorderPay
        │     (the backend calls payment-service through /internal/v1/payments for checkout)

   PostgreSQL ─ pg_notify('queue_events') ─► notify-service :8091 ─► Web Push / FCM
   PostgreSQL ─ notification_outbox       ─► mail-service         ─► SMTP
```

The backend does not call auth-service: login tokens (RS256 JWTs) are issued by auth-service and verified locally by the backend with the public key. The other `/api/v1/users/me/*` paths (profile, avatar, preferences, history) stay in the backend.

In production nginx sits in front (see `deploy/nginx/`). The frontend and the API share one origin, so CORS is only needed when the frontend is opened from another origin.

## 2. Backend

### 2.1 Layers

```
handler  →  service  →  repository  →  PostgreSQL (sqlx + pgx)
   │           │
   │           └── notifier (SSE hub)
   └── dto (JSON shapes)
```

- **handler** (`internal/handler`): reads the request, calls a service, and maps errors to HTTP codes and error codes.
  It contains no business rules.
- **service** (`internal/service`): all business rules (ownership, subscription, queue order, rejoining).
- **repository** (`internal/repository`): SQL. It knows no business rules.
- **dto** (`internal/dto`): the JSON shapes sent to clients. Database models are never sent directly.

The same layer pattern (`handler`, `service`, `repository`, `dto`, `model`, `router`, `config`, `apperror`, `httpserver`, tests in `internal/test`) is used by the backend, `auth-service`, `notify-service`, and `mail-service`; packages that are not needed do not exist. The backend also has `app` (wiring), `logging`, `metrics`, `telemetry`, `requestid`, and `paymentclient` (the client for payment-service). `payment-service` is smaller: `borderpay`, `checkout`, `webhook`, `order`, `store`, `config`, using `net/http` without Gin and without JWT (only `X-Service-Token` and the webhook token). Specific packages: `notify-service` has `listener`, `pusher`, and `runner`; `mail-service` has `mailer` and `mailtemplate`; `auth-service` has `google` and `mailclient`.

Each layer defines the interfaces it needs itself (for example `TicketRepository` in the service package).
As a result, services can be tested with fake repositories and no database, and wiring is done manually in
`internal/app/wire.go` (`app.Wire`) and `internal/app/engine.go`, called from `cmd/server/main.go`.

### 2.2 Authentication and authorization

- **auth-service** (`auth-service/`, internal port 8094) handles registration, login (email or username), Google login, forgot and reset password, and email verification. Only this service holds the private key that signs tokens. Public paths stay `/api/v1/users/...`; nginx forwards `login`, `register`, `google`, `forgot-password`, `reset-password`, `verify-email`, and `me/verify-email/resend` there, and answers with a JSON 503 `AUTH_UNAVAILABLE` when it is down. The database role `auth_rw` only touches `users`, `user_action_tokens`, and the columns that the auth flows change. The backend does not call it, and `scripts/check-service-isolation.sh` keeps any `depends_on` to it out, so users who are already logged in can keep using the app while auth-service is down.
- Passwords are hashed with bcrypt (cost 10). Responses never contain the password.
- RS256 JWT tokens are valid for 24 hours (`kid` `qms-1`). The `sub` claim holds the user id and `iat` the issue time. The backend only holds the public key (`JWT_PUBLIC_KEY_FILE`); HS256 tokens are rejected. The `RequireAuthWithRevocation` middleware checks tokens on all `/api/v1/queues` and `/api/v1/users/me` routes, and rejects a token whose `iat` is older than `users.password_changed_at` (sessions are revoked after a password reset; the comparison is in seconds). Tokens without `iat` (issued before this change) remain valid until `exp` and do not trigger a query. The cost is one query per authenticated request, measured in item 13.
- A password must be between 8 characters and 72 bytes (`service.ValidatePassword`, used by registration and reset).
- **Password reset (item 9):** `POST /users/forgot-password` always returns 202 (no account enumeration; sending happens in a goroutine, at most 32 at a time, the rest get 503). The token is 32 random bytes, and only its SHA-256 hash is stored in `user_action_tokens` (30 minutes, single use, a new token cancels the old one, at most 3 per account per hour). auth-service calls `mail-service` (`POST /internal/v1/send`, template `password-reset`) through `internal/mailclient`; `MAIL_SERVICE_URL` and `MAIL_SERVICE_TOKEN` are set as a pair, and when empty the forgot, reset, and verify routes are not registered. `POST /users/reset-password` consumes the token, changes the password, sets `password_changed_at`, and cancels the other tokens in one transaction; the password is checked before the token is touched. nginx: zone `reset_zone` (1 r/s, burst 5), the log for the `/reset-password` page uses a format without the query (`qms_masked`) so the token is not recorded, and `Referrer-Policy: no-referrer`. `notify_ro` and `mail_rw` have no privileges on `user_action_tokens`. The same table is prepared for verifying an email change (`purpose = 'email_change'`), which is not built yet.
- Login accepts an email or a username. A failed login message does not say which part was wrong.
- **Google login:** `POST /users/google {id_token}` exchanges a Google ID token (checked by auth-service itself against Google's JWKS; active only when `GOOGLE_CLIENT_IDS` is set) for a QMS token. An account is matched by `google_id`; a new email creates an account without a password that is already verified; an email that is already registered with a password gets 409 `GOOGLE_LINK_REQUIRED` together with a `link_token` (a 5-minute single-use JWT) and is **not** linked automatically: the owner must prove the password through `POST /users/google/link`. In the frontend, the Google button is hidden when the client ID is not configured.
- **Email verification:** `EMAIL_VERIFICATION_REQUIRED` (default: on when `MAIL_SERVICE_URL` is set; the value must be the same in auth-service and the backend; turning it on without mail-service makes the service refuse to start). Registration sends a link (`email-verify`, `/verify-email?token=`, valid 24 hours, at most 3 per account per hour) using `user_action_tokens` (`purpose = 'email_verify'`) and the column `users.email_verified_at`. `POST /users/verify-email {token}` (public; a token that is invalid, expired, or used gives 400 `INVALID_TOKEN`), and `POST /users/me/verify-email/resend` (neutral 202, 429 when too frequent). After a successful verification, auth-service sends the `email-verified` confirmation email in a goroutine (best effort: a failure is only logged, and the verification result does not change). `GET /users/me` (backend) includes `email_verified` (always `true` when the feature is off). The `RequireVerified` guard in the backend reads the database on each call (not the JWT claims) and rejects 403 `EMAIL_NOT_VERIFIED` for creating a queue, subscription checkout, and joining with a new number (rejoining that returns the old ticket is not rejected). Frontend: the public page `/verify-email` and a banner with a resend button on the Profile page. The verification email is sent by a fire-and-forget pattern: registration sends it in a goroutine (a failure is only logged, and registration still returns 200). Development: emails go to Mailpit. Production: follows `MAIL_MODE` (`mailpit` by default, or `smtp` for a real SMTP server, e.g. Gmail; `scripts/check-mail-mode.sh` warns that new users cannot verify in `mailpit` mode, and `make verify-user` marks one account). The older DB migration (`db/scripts/apply_schema.sql`) adds the columns, widens the `purpose` CHECK, and verifies all existing accounts once (the column did not exist before), so no one is locked out. Seed and demo accounts are created as verified. There is no MX or domain allowlist check: proof of ownership is clicking the link.
- Profile photo: see 6c.

Access rules:

| Operation | Who may do it |
|---|---|
| Create, change, set permissions, list queues | The owner with an active subscription (the list remains available without a subscription, but only active queues) |
| Close, details, ticket list, Next, Skip, Recall, insert, remove number, counters, notes, public display settings | The queue's owner |
| Cancel a ticket | The ticket holder |
| Queue history and export (`/history/*`) | The queue's owner; customers use `/users/me/history` |
| Public display (`/display/:token`) | Anyone who holds the token (no login) |
| Queue information (`/info`) | Any logged-in user |
| Queue status and SSE | The owner or a ticket holder in that queue |
| Join | Any logged-in user while the queue is active |

Requests that are not allowed get **404**, not 403. This way the existence of another person's queue cannot be guessed.

### 2.3 Data model

All tables in the schema are used now (16 tables; the map per file is in `db/migrations/README.md`):

| Table | Purpose |
|---|---|
| `users` | Accounts (email, username, password hash, `google_id`, `email_notifications`, `password_changed_at`, `email_verified_at`) |
| `owners` | Owner marker and subscription expiry (`subscription_expires_at`) |
| `payments` | History of each subscription extension (written by payment-service when the `payment.paid` webhook is applied) |
| `queues`, `queue_settings` | Queues (name, description, status, owner, `ticket_prefix`) and Skip settings (`skip_fallback`, `skip_fallback_n`, `skip_max_returns`) |
| `counters` | Counters per queue (one default, at most 10) |
| `tickets`, `ticket_notes`, `queue_history` | Tickets (`waiting`, `currently_served`, `completed`, `cancelled`, `skipped`, `deleted`), owner notes, and history |
| `push_subscriptions` | Web Push subscriptions per browser |
| `push_devices` | FCM tokens per Android device (one token per row, moved to the account that logged in last) |
| `notification_outbox` | Queue of ticket and `queue-closed` emails sent by mail-service |
| `user_action_tokens` | Hashes of password reset and email verification tokens (managed by auth-service) |
| `user_avatars` | Profile photos |
| `queue_displays` | Token and settings of the public display |

Important constraints on `tickets`:
- Partial unique index `tickets_queue_id_user_id_active_key` on `(queue_id, user_id) WHERE status IN ('waiting','currently_served')`: a user has only one active ticket per queue; after the ticket is completed or cancelled, the user may join again and receive a new number.
- Partial unique index `tickets_one_serving_per_counter`: a counter serves only one number.
- `UNIQUE (queue_id, sort_key)`: two tickets must not have the same order, including completed ones.

### 2.4 Queue order (sort_key)

- Each new ticket gets a `sort_key` = the maximum `sort_key` in that queue + **65536** (the gap).
- The maximum is computed from **all** tickets in the queue, not only the `waiting` ones. The reason: `UNIQUE (queue_id, sort_key)`
  also applies to completed tickets. If only `waiting` tickets were counted, a queue that had been drained would produce a
  colliding key and joins would keep failing (this bug happened once).
- The large gap allows inserting in the middle (`POST /insert`) without rewriting many rows.
- Next selects the `waiting` ticket with the smallest `sort_key`.

### 2.5 Ticket lifecycle

```
            join                 next
  (new) ───────────► waiting ───────────► currently_served ───────► completed
                         │  ▲                     │      (next Next / queue closed)
                         │  └── skip with fallback (back | after_n) or counter deleted
                         ├── cancel (customer)            ──► cancelled
                         ├── remove (owner)               ──► deleted
                         └── permanent skip (fallback `remove`) ──► skipped
```

Skip rules are set per queue with `queue_settings.skip_fallback` (`remove|back|after_n`), `skip_fallback_n`, and `skip_max_returns`. Defaults for a new queue: `skip_fallback = back` (return to the very back) and `skip_max_returns = 1`; existing queues keep their own values. Each counter serves one number (`currently_served`). Deleting a counter that is serving returns its number to the front of the queue.

- **Rejoining** (a user who already has a ticket in that queue): the backend returns the **existing ticket** with 200.
  No second number and no new event. This answers the case of scanning a QR code twice or double-tapping.
- **Next** on a queue with no `waiting` ticket: 409 `QUEUE_EMPTY`, with no data change.

### 2.6 Displayed numbers

- `current_number` = the `ticket_number` of the `currently_served` ticket (one per counter; customers see the number being served in that queue).
- `previous_number` = the most recently completed (`completed`) ticket.
- `next_number` = the first `waiting` ticket.
- `waiting_ahead` = the number of `waiting` tickets with a smaller `sort_key` than the requester's.

The customer view uses these numbers directly from `GET /status`. The owner's board is different: it loads the ticket list
(`GET /tickets`) and derives the same positions in the frontend (`utils/queue-view.ts`), using the same rules as above.

### 2.7 Real-time (SSE)

- The backend has **one hub** in memory (`sse.NewHub()` in `app.Wire`) that keeps a *room* per queue (keyed by `queue_id`; the public display uses the room `-queueID`). A client connected to `/queues/:id/event` is registered in that queue's room.
- Services call `notifier` after the transaction is complete (join, next, skip, cancel, remove, counter, update, close, QR). Events: `ticket_joined`, `ticket_next`, `ticket_skipped`, `ticket_cancelled`, `ticket_removed`, `counter_updated`, `queue_closed`, `queue_updated`, `queue_qr_updated`, and `display_changed` (only to the public display room, see 6d). The hub sends the event to every client in that queue's room.
- A `: ping` heartbeat is sent about every 25 seconds so that the connection is not dropped by a proxy. The header `X-Accel-Buffering: no` turns off buffering in nginx.
- A disconnected client is removed from the hub (`defer Unsubscribe`).
- Events only carry the data needed for display. `user_id` and `sort_key` are removed from `TicketEvent`, because the recipients can be other customers in the same queue.

**Limitation**: the hub lives in the memory of one process. Running more than one backend instance requires a shared pub/sub mechanism
(e.g. PostgreSQL LISTEN/NOTIFY or Redis) so that events reach every instance.

### 2.8 Response format

- Snake_case and `null` for empty fields, shaped through DTOs (`NewXxxResponse`).
- Success is wrapped as `{"data": ...}`; failure is wrapped as `{"error": {"code", "message"}}`.
- Error codes are fixed strings (`NOT_FOUND`, `QUEUE_EMPTY`, etc.) so that clients can handle them without reading the message text.

## 3. Frontend

### 3.1 Technology

- **Expo SDK 57** with Expo Router (file-based routing). The main target is web (`output: static`); native follows.

- **NativeWind** (Tailwind for React Native) for styling.
- **Zustand** with `persist` for the login session (token and display name). Stored in AsyncStorage, and in localStorage on web.
- **SSE** through a `fetch` stream (not `EventSource`), because `EventSource` cannot send the `Authorization` header.

### 3.2 Structure

```
src/
  app/          routes (Expo Router). Read the store, call hooks/lib, pass data to components.
  components/   presentational components: props only, no fetch and no store.
  hooks/        server data (use-queue-board, use-queue-spot, use-my-tickets, use-queue-events, ...).
  lib/          HTTP client (api.ts), per-endpoint functions (tickets.ts, queues.ts), pure utilities.
  store/        authStore (login session).
  utils/        pure logic (queue-view.ts: previous/current/next numbers).
  constants/    tokens.js: one source of colors and radii.
```

Rule: components in `components/` do not import the store and do not call the API. This way the UI can be tested
and reworked without touching data.

### 3.3 Routing and guards

- `(user)/` is guarded by `AuthGate` in `required` mode. Without a token, the user is sent to login with a `next` parameter.
- `join` (`/join?t=<token>`) also uses `required`: the user must log in before joining. `AuthGate`
  accepts an explicit `nextPath` so that the token query is not lost when redirecting to login.
- `display/[token]` is public (without `AuthGate`): the screen for a waiting-room monitor.
- Login is at `/login` (`guest` mode); `/` is only an entry point that forwards to `/login` (guests) or to `next`/`/dashboard` (logged in). `forgot-password`, `reset-password`, and `verify-email` are public (without `AuthGate`, because their links are often opened on a device that is not logged in); `link-google` uses `guest` and can only be used when a link token is waiting in memory (otherwise it returns to login). `(user)/` also contains history, profile, `subscription`, and `queues/[queueId]/live`.
- `ticket/[queueId]` shows the status of a ticket the user already has (opened from My Queues or History).
- Login and register use `guest`: a logged-in user is sent directly to `next` or the dashboard.
- `subscription`: the subscription screen calls `POST /api/v1/owners/subscription/checkout` (method `qris`), then opens BorderPay's `pay_url`. A regular user sees a "Become an Owner" card; the first payment makes them an owner. The subscription status is reloaded with the REFRESH button after the webhook is applied.
- `next` is checked with `sanitizeNext` so that it cannot be used to open another site (open redirect).
- A 401 response from the API removes the local session and sends the user to login.

### 3.4 Responsive layout

One shell component (`ResponsiveScreenLayout`) is used for two views:
- Width ≥ 1024 px: a header with the profile panel, content limited to a maximum width, and the queue details page uses
  a two-column layout that does not scroll as a page (only the right column scrolls).
- Narrower: a phone header with a back button, a bottom tab bar, and the profile panel slides in from the right.

Root tabs (Home, My Queues, History, Profile) have no back button. Child pages do.

### 3.5 Theme

- Semantic colors (`background`, `surface`, `foreground`, `line`, etc.) are defined once in `tokens.js` for light and dark mode.
- The mode is set through NativeWind's `colorScheme`. The user's choice is stored under the key `qms-theme`.
- Fixed colors (for example the dark queue board and header) do not follow the theme, because they are designed to be dark in both modes.
- Colors passed as non-style props (icons, spinners) use `useThemeColor`.

### 3.6 API address configuration

`EXPO_PUBLIC_API_URL` is used at build time. If it is empty, the frontend uses the host of the opened page with port 8080.
With nginx, `EXPO_PUBLIC_API_URL` must be set to the nginx origin. Otherwise the frontend calls port 8080 directly and bypasses the proxy.

### 3.7 Feedback and popups

- **Toast** (`components/ui/Toast.tsx`): temporary errors through `ToastProvider` in the root layout and `useErrorToast`. It disappears by itself after 5 seconds, identical messages are not stacked, and when a popup is open the toast is shown inside the topmost popup so that it is not hidden by the dark backdrop.
- **Popup** (`components/ui/ModalCard.tsx`): a shared frame with a dark backdrop; tapping the backdrop or the back button calls `onRequestClose`. `ConfirmModal` is a two-button confirmation popup on top of it (Next, Skip, delete photo, public display settings). `compact` mode reduces the card padding from 20 to 8 px.
- **Password confirmation** (sign up and reset password): the message "Passwords do not match" appears inline under the confirmation field, not as a toast, and only after that field loses focus or the submit button is pressed (`utils/confirm-password.ts`), not while typing.

## 4. Deployment

- The stack runs through Docker Compose: `docker-compose.yml` (dev, code mounted, hot reload; includes Mailpit) and `docker-compose.prod.yml` (demo/production behind Cloudflare Tunnel, environment from `.env.prod`, nginx only on `127.0.0.1:8080`, static frontend). Services: `db`, `notify-service`, `mail-service`, `auth-service`, `payment-service`, `backend`, `frontend`, `nginx` (and `mailpit` in some modes). The two cannot run at the same time (port 8080).
- Only nginx publishes a port. Internal services have no `ports:`; `auth-service` is only reachable through certain `/api/v1/users/...` routes in nginx (JSON 503 `AUTH_UNAVAILABLE` when it is down), and it deliberately has no `depends_on` from the backend or nginx. The JWT public key (`keys/` for prod, `keys-dev/` for dev) is mounted into the backend, and the private key only into auth-service (`scripts/gen-auth-keys.sh`).
- nginx forwards `/api/` to the backend, uses unbuffered SSE routes (queues and the public display), rate limits (login and registration, join, password reset, avatar, public display), masks tokens in the log, and blocks `/internal/`. Its configuration is in `deploy/nginx/`. TLS is handled by Cloudflare; nginx only serves HTTP.
- The backend refuses to start with `APP_ENV=production` if `ALLOWED_ORIGINS` contains `*` or a non-https origin, or if `PUBLIC_APP_URL` is not https (the weak-secret check in the backend is currently commented out; auth-service checks that `PUBLIC_APP_URL` is https and that `MAIL_SERVICE_TOKEN` is at least 32 characters). Graceful shutdown: SIGINT/SIGTERM close SSE, finish requests (`SHUTDOWN_TIMEOUT`), and then close the database.
- DB schema: `db/migrations/01*.sql` (one file per domain; map in `db/migrations/README.md`) only runs on an empty data directory; an existing database uses `db/scripts/apply_schema.sql` (idempotent) and the role scripts (`03_notify_role.sh`, `04_mail_role.sh`, `06_auth_role.sh`, `07_metrics_role.sh`, `08_payment_role.sh`). Daily commands go through the `Makefile` and `scripts/` (backup, demo accounts, `check-service-isolation.sh`, `check-mail-mode.sh`).
- All sensitive variables are supplied when the container runs. There are no secrets inside the image.

## 5. Design decisions

| Decision | Reason |
|---|---|
| Rejoining returns the existing ticket, not an error | The specification: a customer who scans again must see their number. A double tap still produces one ticket. |
| Only the number (no customer name) on the public display | Privacy. Names are not part of the display data. |
| 404 for forbidden access | The existence of other people's data does not leak. |
| SSE through a `fetch` stream | The `Authorization` header is still sent. The token does not go into the URL. |
| SSE events without `user_id` and `sort_key` | The recipients can be other customers. |
| The customer receives the number computed by the backend; the owner derives it from the ticket list | Customers do not need the full ticket list. The owner needs all tickets for the board anyway, so deriving the positions in the frontend adds no load. The rules are the same on both sides. |
| Presentational components separated from data | The UI can be tested and reworked without touching data. |
| The queue list stays available without a subscription, but only active queues are shown | The owner can still see queues that are still running. |
| A gap of 65536 for `sort_key` | Insertion in the middle can be done without rewriting other rows. |
| Go libraries standardized: `gin` for HTTP, `sqlx` with the `pgx` driver for queries, pgx v5.12.0 in every module | One way to write handlers, middleware, and repositories in every service, so the logging, request ID, and metrics patterns do not need two versions. Applies to the backend, auth-service, mail-service (gin and sqlx), and notify-service (sqlx). |
| LISTEN/NOTIFY keeps using a direct `pgx` connection, not `sqlx` (mail-service `LISTEN mail_outbox`, notify-service `LISTEN queue_events`) | `WaitForNotification` exists only in the native pgx API; `database/sql` has no equivalent. The LISTEN connection is opened separately (`pgx.Connect`), not taken from the sqlx pool, so a connection waiting for notifications is not reused by ordinary queries. This is why `pgx` remains a direct dependency in mail-service and notify-service. |
| notify-service does not use `gin`; `/health` stays on `net/http` | Its only HTTP endpoint is `GET /health` (48 lines, no middleware). Adding gin for one route would add dependencies and binary size with no benefit. The `net/http` in `internal/pusher` is a Web Push client, not a server. |
| Go modules do not share a `go.work` or a shared module; library versions are kept in sync manually | Each service is built and deployed on its own (its own Dockerfile). Small shared packages (for example logging and metrics) are deliberately copied into each service. When a shared library is upgraded, upgrade it in every module that uses it. |

## 6. Testing strategy

| Layer | Tool | Location |
|---|---|---|
| Backend unit | `go test` with fake repositories, stub services, spy notifiers, and mocks | `backend/internal/test/<layer>_test` (dto, handler, service, repository, router, middleware, sse, config, and others) |
| Backend integration | `go test -tags integration` on PostgreSQL (temporary schema) | `backend/internal/test/integration_test` |
| Frontend unit | `node --test` (`npm run test:unit`) for pure logic | `frontend/tests/unit` |
| Unit tests of the other services | `go test ./...` in each service | `auth-service`, `mail-service`, `notify-service`, `payment-service` |
| End-to-end | Playwright (not run by default; email tests need Mailpit) | `frontend/playwright` |

Test expectations are derived from the project's product overview (a document outside this repository), not from the code.

## 6b. notify-service (Web Push and FCM)

A separate Go service, internal, not published by nginx. It only exposes `GET :8091/health` (200 when LISTEN is active) for the Compose healthcheck; JSON logs; push uses an 8-second timeout and 3 retries for temporary failures. The trigger `tickets_notify_trg` (`AFTER UPDATE OF status, called_at, recalled_at, skip_count`) calls `pg_notify('queue_events', {kind: called|recalled|skipped|requeued, queue_id, ticket_id})`; `requeued` also carries `reason` (`skip` or `counter_closed`) and `counter` (the counter name, read by the trigger because a deleted counter no longer exists when notify-service reads it). The trigger checks `requeued` before `skipped`. NOTIFY is sent on COMMIT. `notify-service` LISTENs (it must connect to the primary), reads data with the role `notify_ro` (SELECT on `tickets`, `queues`, `counters`, `push_subscriptions`, `push_devices`; DELETE on `push_subscriptions` and `push_devices`), and then sends through webpush-go (browsers) or FCM HTTP v1 (the Android app). `pusher.Dispatcher` picks the channel per device, so `service` does not know which channel is used. Recipients: `called` → the owner of the number and the first waiting ticket; `recalled` → the owner of the number; `skipped` (permanent skip) → the owner of the skipped number; `requeued` (the number returned to `waiting` after a skip with fallback, or because a counter was deleted) → the owner of the number, with the new position computed from `sort_key` (`PositionOf`). Subscriptions that return 404 or 410 (Web Push) and tokens that are `UNREGISTERED` or invalid (FCM) are deleted; an `INVALID_ARGUMENT` that is not about a token deletes nothing. The text of a number is produced by one function (`service.Label`). Layered structure: `internal/{config,model,dto,repository,service,listener,pusher,runner,httpserver,apperror}` (`listener` receives NOTIFY, `pusher` sends Web Push with retries and selects the channel, `fcm` sends to FCM with a cached OAuth access token from the service account, `runner` connects them); tests in `internal/test/`. There is no outbox yet: a push sent while the service is down is lost.

The backend: `POST /queues/:id/recall` (`tickets.recalled_at`, 3 seconds, once per call) and `GET /push/public-key`, `POST /push/subscribe|unsubscribe`. A subscription is unique per `endpoint`; subscribing moves it to the logged-in user. For the Android app: `POST /push/fcm/register|unregister` (a token is unique per device and moves to the logged-in account; the routes only exist when `FCM_PROJECT_ID` is set). FCM is optional: without credentials, notify-service does not read `push_devices`, and Web Push keeps working.

### mail-service (email, its own microservice)

Emails are sent by **`mail-service`**, not notify-service. The separation is deliberate: `notify-service` handles push only (Web Push and FCM), with no code, environment, or `depends_on` for email, so an SMTP or mail-service outage does not stop push notifications (proven with a real event while mail-service was stopped; guarded by `scripts/check-service-isolation.sh`).

- **Shape:** its own Go module (`mail-service/`; dependencies: `gin` for the API, `sqlx` with the `pgx` driver for outbox queries, and `pgx` directly for LISTEN), internal port 8092 (outbox worker: see the "Email outbox" section below; without `DATABASE_URL` the worker is off and only the send API runs), no nginx route. `GET /health` does not check SMTP. `POST /internal/v1/send` with the header `X-Service-Token` (`MAIL_SERVICE_TOKEN`, at least 32 characters): `{"to","template","data"}`. The content of an email is defined by the templates inside mail-service; callers only supply a name and data. Responses: 200 `{"data":{"status":"sent"}}`, 400 (broken JSON, unknown field, invalid recipient, missing template, incomplete data), 401, 413 (body larger than 64 KB), 502 `SMTP_FAILED` (SMTP details are not leaked), 503 `BUSY` (more than 16 concurrent sends). Sending is synchronous with internal retries; there is no in-memory queue, so a caller that needs durability uses the outbox (item 8, below). Recipient addresses are not written to the log.
- **Package `internal/mailer`** (called by `service`): MIME message `multipart/alternative` (text and HTML, quoted-printable, UTF-8 subject), one deadline per SMTP conversation (dial 5 seconds, total 15 seconds, cancelled together with the `context`), TLS modes `none`/`starttls`/`tls` (`PlainAuth` refuses passwords over a connection without TLS to hosts other than localhost), 3 retries with a backoff of 1 and 2 seconds for temporary failures (dial, 4xx); 5xx and invalid input are not retried (`ErrPermanent`); `To` and `Subject` containing CR or LF are rejected.
- **Templates** (`internal/mailtemplate/templates`, embedded): the existing templates are `joined`, `cancelled`, `removed`, `completed`, `queue-closed`, `email-verify`, `email-verified`, `password-reset`, and `test`. Each template consists of `<name>.subject.tmpl`, `.txt.tmpl`, `.html.tmpl`, plus the shared `layout.html.tmpl`. HTML goes through `html/template` (user values are escaped automatically); `missingkey=error` turns missing data into an error instead of a "<no value>" that would be sent; template names are validated (`^[a-z0-9][a-z0-9-]*$`).
- **Configuration** is validated at startup (`config.Load`): `SMTP_HOST`, `SMTP_PORT`, `SMTP_FROM`, `SMTP_USER`/`SMTP_PASSWORD` (as a pair), `SMTP_TLS`, `MAIL_SERVICE_TOKEN`; connectivity is not checked at startup. `mail-service send-test <address>` sends a test email.
- **Development:** Mailpit (`axllent/mailpit`, pinned version): SMTP 1025 and UI 8025 are bound to 127.0.0.1 on the host (change them with `MAILPIT_SMTP_PORT` and `MAILPIT_UI_PORT`), not through nginx; mail-service uses `mailpit:1025` over the internal network. **Production** chooses through `MAIL_MODE` (in `.env.prod` or `make prod-up MAIL_MODE=...`): `smtp` uses a real SMTP server and requires `SMTP_HOST`, `SMTP_PORT`, `SMTP_FROM`, and `MAIL_SERVICE_TOKEN`; `mailpit` (**default** when `MAIL_MODE` is empty) starts Mailpit through the compose profile `mailpit` (UI only on 127.0.0.1, messages in a volume) and forces `SMTP_*` to Mailpit, for demos or testing without sending real email. `scripts/check-mail-mode.sh` rejects a wrong configuration before the stack starts.

### Email outbox (item 8)

Four emails go to the ticket's owner: `joined` (on INSERT of a ticket; joining again returns the same ticket without an INSERT, so there is no duplicate; an owner's insert also produces `joined`), `cancelled`, `removed` (status `deleted`), and `completed` (only from `currently_served`, including when the queue is closed). One more email goes to the **owner**: `queue-closed`, when a queue changes from `active` to `closed` (the trigger `queues_email_outbox_trg`, `AFTER UPDATE OF status ON queues`; `ticket_id` is NULL, `queue_id` is set, one email per queue through the partial unique index `(kind, queue_id)`; the owner's `email_notifications` switch applies). Its link goes to `/riwayat`, not to `/ticket/:queueId`.

Outside the outbox, `auth-service` calls `POST /internal/v1/send` directly for `email-verify`, `password-reset`, and `email-verified` (sent once after a successful verification, best effort).

- **Atomic write.** The trigger `tickets_email_outbox` (`AFTER INSERT OR UPDATE OF status`, separate from `tickets_notify` so that push is not affected) writes `notification_outbox` in the same transaction as the ticket change, and then `pg_notify('mail_outbox')` wakes the worker. `UNIQUE (kind, ticket_id)` together with `ON CONFLICT DO NOTHING` guarantees one email per event. The payload is a snapshot (`queue_id`, `queue_name`, `prefix`, `number`, `name`); it **never contains the owner's note**, and the number label is formatted in one place (`service.Label` in mail-service, the same as in notify-service). When a ticket is deleted, its outbox rows are deleted too (`ON DELETE CASCADE`), which also avoids id collisions after `TRUNCATE ... RESTART IDENTITY` in the seed.
- **The switch** `users.email_notifications` (default `TRUE`, changed through `PUT /users/me/preferences`) is read **at the time of the event** by the trigger: turning it off does not cancel emails already in the queue, but later events do not enter the outbox, and turning it back on does not send the missed ones.
- **The worker lives in mail-service**, not in notify-service: notify-service remains pure push (Web Push and FCM) with no dependency on email (guarded by `scripts/check-service-isolation.sh`). The worker claims rows with a single statement `UPDATE ... WHERE id IN (SELECT ... FOR UPDATE SKIP LOCKED)` that increments `attempts` and moves `next_attempt_at` forward by the lease (`SendTimeout + 1 minute`); the row lock is not held during SMTP. Delivery is **at-least-once**: if the process dies right after SMTP accepts the message and before `sent_at` is written, the email can be sent twice after the lease expires.
- **Retries:** a temporary failure schedules `next_attempt_at` with an increasing backoff (30 seconds, 1, 2, ... up to a maximum of 30 minutes; `OUTBOX_MAX_ATTEMPTS` defaults to 8). A permanent failure (`mailer.ErrPermanent`: 5xx, a rejected address), a broken payload or template, or exhausted attempts marks `failed_at` and stops; `last_error` keeps the cause (at most 500 characters). One broken row does not block other rows. A **rate limit** `OUTBOX_RATE` (default 5 emails per second) keeps a large queue closure from flooding SMTP. Polling every 5 seconds is a safety net in case a NOTIFY is missed (a dropped LISTEN connection is reconnected).
- **Privileges:** role `mail_rw` (`db/migrations/04_mail_role.sh`, password `MAIL_DB_PASSWORD`; idempotent): `SELECT` on the outbox, `UPDATE` only on the delivery status columns (`sent_at`, `attempts`, `last_error`, `next_attempt_at`, `failed_at`), and `SELECT (id, email, username, email_notifications)` on `users`. It cannot read `password_hash`, tickets, or other tables, and it cannot INSERT or DELETE (tested with `SET ROLE`). `notify_ro` is unchanged. The init script only runs on an empty data directory; for an existing database, run `db/scripts/apply_schema.sql` and then `04_mail_role.sh` (see the README).
- **Links** in ticket emails: `QMS_PUBLIC_ORIGIN` + `/ticket/:queueId`; the `queue-closed` email: `QMS_PUBLIC_ORIGIN` + `/riwayat` (an absolute http or https URL is required while the worker is running). The content is in Indonesian, HTML and text; the subject includes the queue name (and the number, for ticket emails).

## 6d. Public display (feature 4.5)

A login-free screen for a waiting-room monitor: one card per counter, the next numbers, and the skipped numbers. Contract: the section "Public display" in `api-contract.md`.

- **Credential = a token in the URL.** `queue_displays` (1:1 with `queues`, `ON DELETE CASCADE`): `token` is unique, 24 random bytes (`crypto/rand`, base64url), and `upcoming_count` and `skipped_count` are between 0 and 10 (CHECK). The token is stored in plain text (not hashed) because the owner needs to see the URL again; for that reason `notify_ro` and `mail_rw` have no privileges on this table. A token in the wrong format is rejected without touching the database.
- **Minimal public data.** `DisplaySnapshotResponse` (`dto/display_dto.go`) only contains display-ready text. The contract test `TestDisplaySnapshot_PublicResponseKeysAreLocked` locks the list of keys, so that a new field cannot leak by accident.
- **Separate SSE room.** The hub sends a full event payload (user ids and so on) to every client in a room, so the public display must not join the queue room. `sse.DisplayRoom(queueID)` = `-queueID`; `Hub.Notify` sends the full payload to the queue room and a **`display_changed` signal without data** to the display room (except for `queue_qr_updated`). No existing event emitter has changed: `counter_updated` (create, rename, delete counter), `ticket_*`, and `queue_*` all go through `Hub.Notify`.
- **Snapshot cache** (`service/display_cache.go`): 1 second per token, with one load shared by many concurrent screens. `Hub.SetBeforeNotify(cache.Invalidate)` clears the cache **before** the event is distributed, so a client that refetches after the signal does not get old data. A load that started before the invalidation is not stored (an `epoch` counter). Replacing the token or turning the display off clears the cache, so the old token returns 404 immediately.
- **Limits.** 20 display SSE connections per queue (`Hub.RoomSize(DisplayRoom(id))`); beyond that, 429. nginx: `display_zone`, unbuffered SSE, and the log format `qms_masked` that masks the token in the path (`map $uri $uri_masked`) for `/display/<token>` and `/api/v1/display/<token>[/event]`; the display page uses `Referrer-Policy: no-referrer` and `Cache-Control: no-store`.
- **Frontend.** The public route `app/display/[token].tsx` (without `AuthGate`), the hook `use-display-screen` (snapshot plus SSE without login, a 150 ms debounce for bursts of signals, resync every 45 seconds while the stream beats and every 3 seconds when it does not, Wake Lock), `DisplayCounterCard` (the style of the customer's "Your Number" card; empty counters are dimmed), and a dynamic layout from 1 to 10 cards (`utils/display.ts`). The status "Reconnecting..." appears when the stream has not beaten for 30 seconds AND the last snapshot is more than 10 seconds old; the last numbers stay visible. On the owner side: `DisplaySettingsCard` in the queue details.
- **Not built yet:** a QR join on the screen (the column `show_qr` is deliberately not present yet; add it through `apply_schema.sql`).

## 6c. Profile photo

`PUT/GET/DELETE /users/me/avatar` (api-contract.md). The photo is **private**: only `GET /users/me/avatar` exists, always for the caller, so there is no public URL and a queue owner cannot see a customer's photo.

- **Processing** (`backend/internal/imageproc`): the type is determined from the file content (`http.DetectContentType`), and only JPEG, PNG, and WebP are accepted; the dimensions are read from the header first and limited to 24 megapixels before decoding (protection against decompression bombs); a center square crop, resized to 512x512 (CatmullRom; small photos are not enlarged), re-encoded as JPEG with the quality lowered step by step until the result fits 150 KB. Re-encoding removes EXIF and GPS data; transparent PNG areas are flattened onto white.
- **Storage**: the table `user_avatars` (`user_id` as PK and FK `ON DELETE CASCADE`, `data BYTEA` with a CHECK of 150 KB, `updated_at`). It lives in Postgres because it is automatically included in `scripts/backup-prod.sh` (which backs up the database only), needs no new volume or service, and writes atomically. `GET /users/me` only reads `updated_at` (`avatar_updated_at`), never the `data` column. `notify_ro` has no privileges on this table.
- **The interface `service.AvatarStore`** (`Put/Get/Delete/UpdatedAt`) is the path to move to object storage. Size: about 50 KB per photo, so 100 thousand photos are about 5 GB, and 1 million photos (every user) about 50 GB. **Threshold to move: about 100 thousand photos (5 GB in the database), or when the backend runs more than one instance that needs to share files.** Object storage means a private bucket (no versioning), credentials in the environment, its own backup policy, and explicit object deletion when an account is deleted.
- **nginx**: the location `= /api/v1/users/me/avatar` raises `client_max_body_size` to 6m only here (the global limit stays 1m) and uses the zone `avatar_zone` at 5 requests per second per IP. A 413 rejection from nginx is HTML, so the client maps the error by status.
- **Frontend**: `expo-image-picker`. Because the endpoint requires a token, the web fetches the photo and creates an object URL (`hooks/use-avatar-source.ts`); native uses `expo-image` with the `Authorization` header. The web shrinks the photo in the browser first (`utils/avatar-image.ts`, canvas) so that uploads are small and the EXIF orientation is correct; the server still validates. On the Profile page the photo is shown at 88 px (the header avatar is 36 px through `UserAvatar`); when a photo exists, tapping it opens an enlarged photo popup (`ModalCard` in `compact` mode, with the size computed by `utils/avatar-preview.ts` to fit the screen, at most 464 px), closed with a small X button in the corner of the photo, a tap outside the card, or the back button. Without a photo (icon only), there is no popup. Buttons: "Upload Photo" or "Change Photo", and "Delete Photo" (through `ConfirmModal`).
- **Development**: the frontend container keeps `node_modules` in an anonymous volume, so after adding a package run `docker compose up -d --build -V frontend`.

## 7. Limitations and what is not built yet

Not built yet:
- Changing the email address (`email_change`) is not built; the `purpose` column is already prepared for it.
- Load testing. (Observability exists: OpenTelemetry, Prometheus, Loki, Tempo, Grafana, and 11 alert rules, as the overlay `docker-compose.observability.yml`; the alerts do not send notifications yet. payment-service does not yet have JSON logs, metrics, or traces.)
- An outbox for push (Web Push and FCM), native iOS notifications, a migration runner, automatic backups, HSTS and Cloudflare Access.
- A QR join on the public display (the column `show_qr` does not exist yet); subscription plans other than one month (30 days) through QRIS BorderPay, and recurring billing.
- Removed from the product (31 October 2026): `registration_limit`, the daily QR limit, the history limit, and permanent deletion (history always remains).

Built since the previous version of this document: customer cancellation, owner removal of a number, custom counters, insertion at a position, the requeue fallback for Skip, notes, history and export, the public display, the profile photo, password reset, email notifications, Google login (requires `GOOGLE_CLIENT_IDS`), email verification, and the 30-day subscription through QRIS BorderPay (replacing the MOCK subscription).

Technical limitations:
- The SSE hub is in memory (see 2.7): the backend runs as a single instance.
- Demo deployment: `docker-compose.prod.yml` behind Cloudflare Tunnel; the backend with `APP_ENV=production` validates secrets and https origins.
- Push has no outbox; email has one (at-least-once, so it can be sent twice if the process dies right after SMTP accepts the message).
- `ALREADY_JOINED` is no longer returned by join (joining again returns the existing ticket). The join handler still maps `ErrAlreadyJoined` and the frontend still handles it, so that older clients are not broken; that code path is only active for an owner's insert (`POST /insert`, 409 `ALREADY_JOINED`).