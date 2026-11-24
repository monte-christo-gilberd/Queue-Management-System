# auth-service

The login microservice for QMS. Its responsibilities:

- **registration and login** (by email or username, bcrypt), issuing **RS256 JWTs**,
- **forgot and reset password** through single-use links,
- **email verification** (send, resend, and verify through a link; after a successful verification, a `email-verified` confirmation email is sent in the background),
- **Google sign-in** (exchanging a Google ID token for a QMS token, and linking accounts through password confirmation),
- being the **only holder of the private key** that signs tokens (`AUTH_JWT_PRIVATE_KEY_FILE`).

The backend **does not call** this service. It verifies tokens locally with the public key (`keys/auth-public.pem`) and reads session revocation and verification status directly from the database. If auth-service is down, logged-in users can keep using the application; only new logins, registrations, password resets, and email verification fail (nginx answers 503 `AUTH_UNAVAILABLE`).

## Running

The keys are created once (automatically by `make up` and `make prod-up`):

```bash
scripts/gen-auth-keys.sh        # from the queue-management-system/ root
```

With Compose (the `auth-service` service, internal port 8094, not mapped to the host):

```bash
docker compose up -d --build auth-service
curl -s http://localhost:8080/api/v1/users/login -H 'Content-Type: application/json' \
  -d '{"identifier":"budi","password":"secret123"}'      # through nginx
```

Without Docker: `go run ./cmd/auth-service` (reads `../.env`; requires `DATABASE_URL`, `PUBLIC_APP_URL`, and `AUTH_JWT_PRIVATE_KEY_FILE`). Tests: `go test ./...`.

## Endpoints

The public paths are the same ones the backend used to serve (nginx forwards them to this service). Response shape: success `{"data": ...}`, failure `{"error": {"code", "message"}}`.

| Method | Path | Notes |
|---|---|---|
| GET | `/health` | Liveness (`{"status":"ok"}`), does not touch the database |
| POST | `/api/v1/users/register` | `{email, username, password}` → `{data:{token}}`. 400 `WEAK_PASSWORD`, 409 `CONFLICT` |
| POST | `/api/v1/users/login` | `{identifier, password}` (`email` accepted as an alias) → `{data:{token}}`. All failures return 401 `UNAUTHORIZED` |
| POST | `/api/v1/users/google` | `{id_token}` → `{data:{token}}`, or 409 `GOOGLE_LINK_REQUIRED` with `error.link_token`. For 401/403/503 see the API contract. Only when `GOOGLE_CLIENT_IDS` is set |
| POST | `/api/v1/users/google/link` | `{link_token, password}` → `{data:{token}}`. 400 `INVALID_TOKEN`, 401 `UNAUTHORIZED`, 409 `CONFLICT` |
| POST | `/api/v1/users/forgot-password` | Always 202 for a well-formed email (no account enumeration) |
| POST | `/api/v1/users/reset-password` | `{token, password}`. 400 `INVALID_TOKEN` / `WEAK_PASSWORD` |
| POST | `/api/v1/users/verify-email` | `{token}`. 400 `INVALID_TOKEN`. If this token verifies a newly verified account, the `email-verified` email is sent through mail-service (see the verification confirmation section) |
| POST | `/api/v1/users/me/verify-email/resend` | Requires login. 202, or 429 when more than 3 per hour |

Forgot, reset, and verify are only registered when `MAIL_SERVICE_URL` is set (otherwise 404). Profile, avatar, preferences, and tickets remain in the backend.

## Verification confirmation email

After `verify-email` consumes a token for an account that was **not** verified before, `service/verification_service.go` sends the `email-verified` template to that account through `mailclient` (mail-service). Delivery is best effort and asynchronous (a goroutine with its own timeout): a failure is only written to the log (without the email address) and never changes the HTTP response of `verify-email`. Nothing is sent if the account was already verified, or if `MAIL_SERVICE_URL` is empty (in that case the verification route is not registered at all). `TokenRepository.VerifyEmail` returns `userID` and `newlyVerified` (true only when `email_verified_at` was set by this token) for that decision.

## Tokens

```
header : {"alg":"RS256","typ":"JWT","kid":"qms-1"}
payload: {"sub":"<user id>","iat":<seconds>,"exp":<iat + 24 hours>}
```

`iat` is used for session revocation: a token issued before `users.password_changed_at` is rejected. Verification only accepts RS256 (HS256 and `alg=none` are rejected).

**Link token** (Google account linking): a 5-minute RS256 JWT with `purpose: "google_link"`, `gid`, and a random `jti`. It is signed with the same key as login tokens, but it is **not a login token**: `purpose` makes `auth-service` and the backend reject it. It is single-use, and it expires after 5 wrong passwords (the counter is in memory; one instance only).

## Google sign-in

`internal/google` verifies the ID token itself (without `google.golang.org/api`): RS256 signature checked against Google's JWKS (cached according to `Cache-Control`; an unknown `kid` triggers a refresh at most once every 1 minute; expired keys are not used when Google is unavailable), the `iss` must be Google's, `aud` must be one of `GOOGLE_CLIENT_IDS`, `exp`/`iat` with a 60-second tolerance, and `email_verified` must be true. Decisions in `service/google_service.go`: a linked account (matched by `google_id`) → token; an email that does not exist yet → new account with no password, already verified; an email that exists with a password (case-insensitive) → 409, and it is **not** linked automatically.

## Configuration

| Variable | Required | Notes |
|---|---|---|
| `DATABASE_URL` (or `AUTH_DATABASE_URL`) | yes | Connection using the `auth_rw` role |
| `PUBLIC_APP_URL` | yes | Public address used in email links. In production: https is required |
| `AUTH_JWT_PRIVATE_KEY_FILE` | yes | RSA private key (PEM, PKCS#8 or PKCS#1) |
| `AUTH_JWT_KID` | no | `kid` header value (default `qms-1`) |
| `MAIL_SERVICE_URL`, `MAIL_SERVICE_TOKEN` | paired | Both or neither. In production: the token must be at least 32 characters |
| `EMAIL_VERIFICATION_REQUIRED` | no | Empty = follows whether mail-service is configured. **Must match the backend** |
| `GOOGLE_CLIENT_IDS` | no | Valid OAuth client IDs (web, Android, iOS) for the `aud` claim, comma-separated. Empty = Google sign-in disabled (404). See [setup-google-login.md](../../documentation/docs/setup-google-login.md) |
| `GOOGLE_CERTS_URL` | no | Google public key endpoint; for local tests only. In production: https is required |
| `PORT` / `AUTH_PORT` | no | Default `8094` |
| `TRUSTED_PROXIES` | no | CIDRs of proxies allowed to set the client IP |
| `SHUTDOWN_TIMEOUT` | no | Seconds, default 8 |
| `APP_ENV` | no | `production` enables the security checks |

## Database

The `auth_rw` role (`db/migrations/06_auth_role.sh`): reads the login columns of `users`, writes `password_hash`, `password_changed_at`, `email_verified_at`, and `google_id`, and `INSERT`s new users. On `user_action_tokens` it can read, insert, and update `used_at` (no `DELETE`). It has no access to queues, tickets, or payments. Only SHA-256 hashes of single-use tokens are stored, never the raw token.

## Structure

Layered architecture: `cmd` only wires things together (configuration, database, repository, service, handler, router, server).

```
cmd/auth-service/    main: main() calls run(); configuration → database → repository → service → handler → router, graceful shutdown
internal/apperror/   sentinel errors that handlers map to HTTP codes
internal/config/     env, private key, production validation, TRUSTED_PROXIES
internal/database/   PostgreSQL connection (Connect)
internal/dto/        request bodies and JSON response payloads
internal/google/     Google ID token verifier (JWKS, iss, aud, exp, email_verified)
internal/handler/    Gin: AuthHandler (8 account handlers), respondData/respondError
internal/httpserver/ http.Server with timeouts
internal/mailclient/ HTTP client for mail-service
internal/middleware/ Auth: RS256 JWT check and session revocation
internal/model/      User and GoogleIdentity
internal/repository/ PostgreSQL: UserRepository and TokenRepository
internal/router/     AppRouter, NewAppRouter + WithEmail/WithGoogle, SetupRouter, per-feature routes (auth_routes.go)
internal/service/    Service (register, login, Authenticate, reset, verification, Google) and TokenIssuer (RS256)
internal/validator/  no_html_tags rule
internal/test/       external tests per layer: config_test, google_test, handler_test, service_test,
                     repository_test (sqlmock for SQL); testutil holds in-memory fakes
```