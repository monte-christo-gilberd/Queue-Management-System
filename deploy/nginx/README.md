# nginx configuration (QMS)

This folder contains the reverse proxy configuration: the single entry point to the whole stack. Each file has a header card that explains its purpose and what to check when the domain or topology changes. Every directive has its own comment, and the points that need adjusting are marked with searchable tags:

```bash
grep -rn "\[GANTI" deploy/nginx
```

| Tag | Meaning |
|---|---|
| `[GANTI-DOMAIN]` | Value related to the domain or host. Change it when the domain moves. |
| `[GANTI-JARINGAN]` | Must match `docker-compose*.yml` (service names, ports, subnets). |
| `[SESUAIKAN]` | Tuning values (rate limits, burst, size limits, timeouts). Adjust according to load. |
| `[SINKRON]` | Duplicate in another file. Change it in both places. |
| `[PENTING]` | Security or contract. Do not change without understanding the consequences. |

> Note: the tags above are kept in their original Indonesian form because they are literal strings used by `grep`.

## Files

| File | Purpose |
|---|---|
| `nginx.conf` | **Dev** configuration (`docker-compose.yml`): http{}, rate-limit zones (`limit_req_status 429`), log format, `stub_status` listener on `:8081` for observability, `server_name localhost`. This is `/etc/nginx/nginx.conf` in the image. |
| `nginx.prod.conf` | **Prod/demo** configuration behind Cloudflare Tunnel: same as dev, plus `set_real_ip_from` and `real_ip_header CF-Connecting-IP`, and `server_tokens off`. |
| `server.conf` | Shared `server{}` body (routing, per-route rate limits, headers). Included by both configurations above. |
| `frontend-upgrade.dev.conf` | WebSocket headers for Metro HMR. **Dev only.** |
| `frontend-upgrade.prod.conf` | Intentionally empty. Do not delete it, because the Dockerfile still COPYs it. |
| `Dockerfile` | nginx 1.27 Alpine image. Copies the files above to `/etc/nginx/`. |
| `Dockerfile.dockerignore` | Allowlist of files included in the build context. New files must be added here. |
| `../../frontend/nginx.conf` | nginx inside the frontend container (serves the output of `expo export`). It is different from the files in this folder. |

## Build arguments (`Dockerfile`)

| ARG | Dev (default) | Prod |
|---|---|---|
| `NGINX_CONF` | `nginx.conf` | `nginx.prod.conf` |
| `FRONTEND_UPGRADE` | `frontend-upgrade.dev.conf` | `frontend-upgrade.prod.conf` |

These two ARGs must be paired correctly. Prod values are set in `docker-compose.prod.yml` (`build.args`).

## Upstream hostnames and ports

The hostnames in `server.conf` are the service names in docker-compose:

| Variable | Upstream | Notes |
|---|---|---|
| `$qms_backend` | `backend:8080` | API, SSE, health |
| `$qms_auth` | `auth-service:8094` | Login, registration, password reset, email verification |
| `$qms_frontend` | `frontend:80` | Web application |
| `$qms_payment` | `payment-service:8095` | BorderPay webhook only. This service runs only with the compose profile `payments` (always enabled by `make prod-up`) |

`notify-service`, `mail-service`, and `mailpit` have no public routes. `payment-service` has only one public route (the webhook). Its checkout (`/internal/...`) is called only by the backend. `/internal/` always returns 404, and the backend calls internal services directly over the Docker network. Upstreams use variables and `resolver 127.0.0.11`, so nginx can still start while a service is not yet running, and it does not keep returning 502 after a service is restarted.

## Routing (`server.conf`)

The order of `location` blocks matters: regex locations are checked before prefix locations, and `location /` is at the bottom.

| Path | Destination | Rate limit / notes |
|---|---|---|
| `/api/v1/queues/:id/event` | backend | SSE: unbuffered, timeout 86400s |
| `/api/v1/users/login`, `/register` | auth-service | `auth_zone` 10 r/s, burst 20. If auth-service is down: JSON 503 `AUTH_UNAVAILABLE` |
| `/api/v1/users/google`, `/google/link` | auth-service | `auth_zone` 10 r/s, burst 20 |
| `/api/v1/users/verify-email`, `/me/verify-email/resend` | auth-service | `auth_zone` 10 r/s, burst 20 |
| `/api/v1/users/forgot-password`, `/reset-password` | auth-service | `reset_zone` 1 r/s, burst 5 (per-account limit is in auth-service) |
| `/reset-password` (page) | frontend | Token masked in logs, `Referrer-Policy no-referrer`, `no-store` |
| `/api/v1/webhooks/borderpay` | payment-service | Exact match, no rate limit. Verified with `x-borderpay-token` in payment-service. If payment-service is down (502/503/504): `@payment_unavailable` returns JSON 503 `PAYMENT_UNAVAILABLE` with `Retry-After: 30` (a non-2xx status so BorderPay retries) |
| `/api/v1/display/:token/event` | backend | Unbuffered SSE, `display_zone` 20 r/s, burst 40, token masked in logs |
| `/api/v1/display/:token` | backend | `display_zone`, token masked in logs |
| `/display/:token` (page) | frontend | Same as `/reset-password` |
| `/api/v1/join`, `/api/v1/join/preview` | backend | `join_zone` 10 r/s, burst 20 |
| `/api/v1/users/me/avatar` | backend | Body up to 6 MB (this route only), `avatar_zone` 5 r/s, burst 10 |
| `/health` | backend | Access log disabled |
| `/api/` | backend | Timeout 60s. Includes `POST /api/v1/owners/subscription/checkout` (the backend calls payment-service; if it is down, the backend itself returns 503 `PAYMENT_UNAVAILABLE`). This route has no special rule |
| `/internal/` | - | Always 404 |
| `/` | frontend | Web application (fallback) |

The `@auth_unavailable` fallback (502/503/504 from auth-service) returns JSON 503 `AUTH_UNAVAILABLE` with `Retry-After: 30`. This code and `PAYMENT_UNAVAILABLE` (for the BorderPay webhook only) are generated by nginx. `PAYMENT_UNAVAILABLE` for checkout is generated by the backend. Requests over the rate limit get 429. The queue and SSE paths are not affected, and logged-in users keep working because the backend verifies the JWT locally.

The default body size limit `client_max_body_size 1m` applies to all routes except the avatar route.

Common proxy headers (`Host`, `X-Real-IP`, `X-Forwarded-For`, `X-Forwarded-Proto`) and basic security headers (`X-Content-Type-Options`, `Referrer-Policy`) are set in `server.conf`, which sits inside `server{}`.

## Access logs and request IDs

All locations use one global JSON `access_log` (`qms_json`, defined in `nginx.conf` and `nginx.prod.conf`). Fields: `time`, `service` (`nginx`), `request_id`, `remote_addr`, `method`, `uri`, `status`, `bytes`, `request_time`, `upstream_response_time`. All values are strings. `/health` is not logged (`access_log off`).

Sensitive values are masked: the password reset token is in the query string (`?token=`), and the public display token is in the path. Logs record `$uri_masked` (the token is replaced with `***`) and never record the query string, so no location needs a special log format.

Request IDs: nginx creates `$request_id` for every request (any value sent by the client is always overwritten), forwards it to upstreams as `X-Request-Id`, and returns it in the response header. The backend and auth-service log the same ID as `request_id`. Locations that have their own `proxy_set_header` or `add_header` (SSE, display pages, password reset, `@auth_unavailable`) repeat these headers, because a directive inside a location replaces the one in `server{}`.

## `stub_status` listener (observability)

In addition to the main `server{}` (port 80), `nginx.conf` and `nginx.prod.conf` define a second `server{}` on port `8081` that serves only `/stub_status` for `nginx-exporter` (the `docker-compose.observability.yml` overlay). Only private networks (`10/8`, `172.16/12`, `192.168/16`) are allowed; everything else gets `deny all`, and other paths return 404. This port is not mapped to the host and is not in `server.conf`, so there is no public route to `/metrics` or `stub_status`. `scripts/check-service-isolation.sh` checks this.

## When moving the domain or topology

| What changes | Change it in |
|---|---|
| Public domain (browsers and phones) | `QMS_PUBLIC_ORIGIN` in `.env`. It must match the address that is opened. The backend uses it for CORS and join links (`PUBLIC_APP_URL`), and auth-service and mail-service use it for links in emails. |
| Production domain | The Cloudflare Tunnel configuration, and `server_name _` in `nginx.prod.conf` if you want to reject other Host headers. |
| Service names or ports | `set $qms_*` in `server.conf`, and the services in `docker-compose*.yml`. |
| Network subnet | `set_real_ip_from` in `nginx.prod.conf`, the backend `TRUSTED_PROXIES` in `docker-compose*.yml` (dev `172.28.0.0/16`, prod `172.29.0.0/16`), and the `ipam` block in compose. All three must match. |
| Ports mapped to the host | `EXPOSE` in `Dockerfile` (must stay 80) and `QMS_HTTP_PORT` in compose. |
| Rate limits or body size | `limit_req_zone` in `nginx.conf` and `nginx.prod.conf`, and `limit_req` and `client_max_body_size` in `server.conf`. Zone names must match. |

## Running

```bash
cp .env.example .env        # fill in the secrets and QMS_PUBLIC_ORIGIN
docker compose up --build   # nginx is exposed on ${QMS_HTTP_PORT:-8080}
```

`docker-compose.yml` is the development mode (hot reload). The prod stack uses `docker-compose.prod.yml` (`make prod-up`), and `docker-compose.observability.yml` is an optional overlay. Only nginx publishes a port to the host (besides Mailpit, which is bound to `127.0.0.1` only). `db`, `backend`, `auth-service`, `notify-service`, `mail-service`, and `frontend` are reachable only over the Docker network. To open psql: `docker compose exec db psql -U qms -d qms_db`.

`QMS_PUBLIC_ORIGIN` must match the address the browser and phone use. The frontend calls the API on the page's origin, so nothing is baked into the bundle.

The `backend` uses `TRUSTED_PROXIES` (the compose network subnet), so client IPs from `X-Forwarded-For` are trusted only when they come from nginx.

## Frontend nginx

One HTML file per Expo route (e.g. `dashboard.html`), so `try_files $uri $uri.html $uri/ /index.html` is used. Static assets are cached for one year, HTML is not cached. The file is at `frontend/nginx.conf`.

## Production/demo: `nginx.prod.conf`

Behind Cloudflare Tunnel (`cloudflared` on the host, target `http://localhost:8080`). The nginx port is bound only to `127.0.0.1`, so only `cloudflared` can send the `CF-Connecting-IP` header that is trusted. The domain is configured in Cloudflare. The Metro WebSocket headers are not used in prod (`frontend-upgrade.prod.conf` is empty). The tunnel is managed manually; see `documentation/docs/setup-perintah-manual.md` (one level above `queue-management-system/`, a document that is not part of this repository).

## Public HTTPS for phone testing (temporary quick tunnel)

Phones do not need to install anything: a Cloudflare quick tunnel gives a `https://*.trycloudflare.com` URL with a publicly trusted certificate, so the QR scanner camera works directly.

```bash
# install cloudflared: https://github.com/cloudflare/cloudflared/releases
make tunnel   # open the tunnel, start the stack, print the URL
make down     # stop
make mailpit  # print the Mailpit links (dev) without starting anything
```

- The URL is random and changes every session. The script passes it as `QMS_PUBLIC_ORIGIN` to the backend, auth-service, mail-service, and other services that need it, which read it at startup.
- Requires internet access. Do not use it for real data, because anyone who knows the URL can reach the application.
- On networks that block QUIC, use `--protocol http2`.

## Not yet implemented

- HSTS (postponed, decision of 31 Oct; if added, start with `max-age=300` and only in `nginx.prod.conf`). Certificates are handled by Cloudflare, not nginx.
- Mailpit through nginx (`/mail/`). Mailpit is reachable only on `127.0.0.1` (see `make mailpit`). `mail-service` sends email to Mailpit over the internal network (`SMTP_HOST: mailpit`).

## Quick checks

- `/` shows the login page, and `/dashboard` does not return 404.
- `/api/v1/queues` without a token returns 401 (502 means the backend is down).
- `/internal/v1/join-token` returns 404.
- `POST /api/v1/webhooks/borderpay` without payment-service returns 502 (the `payments` profile is not running).
- Many quick requests to `/api/v1/users/login` eventually return 429.
- The owner presses Next and the customer screen updates without a refresh (SSE is not held by buffering).