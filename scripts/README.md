# scripts/

Helper scripts to run, check, and maintain the QMS stack. All scripts are **run from the repository root `queue-management-system/`**. They change to the repo root themselves with `cd "$(dirname "$0")/.."`, so relative paths are safe. Most are called through `make`; the rest are run directly.

Common prerequisites: `bash` and `docker` with `docker compose`. `check-service-isolation.sh` also needs `python3`. `gen-auth-keys.sh` and `seed-prod-penguji.sh` need `openssl`. `seed-prod-penguji.sh` also needs `curl` and the Python module `bcrypt`.

| Script | Purpose | Stack | Called by |
|---|---|---|---|
| [`gen-auth-keys.sh`](#gen-auth-keyssh) | Generates the RSA key pair for login tokens | dev and prod | `make up`, `make tunnel`, `make prod-up` |
| [`check-service-isolation.sh`](#check-service-isolationsh) | Keeps services isolated from each other in Compose (including the observability overlay) | dev and prod | `make obs-check`, manual |
| [`check-observability-leaks.sh`](#check-observability-leakssh) | Secret-leak test against logs, traces, and metrics | dev and prod (observability overlay) | `make obs-check` |
| [`check-google-config.sh`](#check-google-configsh) | Checks Google sign-in configuration (server and frontend are in sync) | dev and prod | `make up`, `make tunnel`, `make prod-up` |
| [`check-mail-mode.sh`](#check-mail-modesh) | Checks `MAIL_MODE` and `SMTP_*` before the prod stack starts | prod | `make prod-up` |
| [`check-payment-config.sh`](#check-payment-configsh) | Checks the BorderPay key, webhook token, service token, and the `payment_rw` role password (only presence and length; values are never printed) | prod | `make prod-up`, `make prod-payment-check` |
| [`verify-user.sh`](#verify-usersh) | Marks one account's email as verified directly in the database | dev or prod | `make verify-user` |
| [`reset-demo.sh`](#reset-demosh) | FULL RESET of the prod database (all accounts are deleted) | prod | `make prod-reset` |
| [`seed-prod-penguji.sh`](#seed-prod-pengujish) | Creates 1 tester account (owner + customer) and fills in test queue data | prod | `make prod-seed-penguji` |
| [`backup-prod.sh`](#backup-prodsh) | Backs up and restores the prod database | prod | `make prod-backup`, `make prod-reset`, `seed-prod-penguji.sh` |

Environment variables read by some scripts:

| Variable | Meaning | Default |
|---|---|---|
| `QMS_ENV_FILE` | Env file used by the prod Compose stack | `.env.prod` (`.env` for `backup-prod.sh` and `check-service-isolation.sh`) |
| `POSTGRES_USER`, `POSTGRES_DB` | Database user and name (read by `backup-prod.sh`, `reset-demo.sh`, `seed-prod-penguji.sh`) | `qms`, `qms_db` |

---

## gen-auth-keys.sh

Creates an RSA 2048-bit key pair for login tokens (JWT RS256). Prod uses the `keys/` folder: it is idempotent, and `make prod-up` never replaces the keys. Dev uses a separate `keys-dev/` folder (`AUTH_KEYS_DIR=keys-dev`): `make up` and `make tunnel` create new keys (deleting `keys-dev/` first), and `make down` removes it. Prod keys are never touched by dev targets.

| File | Contents | Mounted into |
|---|---|---|
| `auth-private.pem` | Private key | **Only** `auth-service` (the token issuer) |
| `auth-public.pem` | Public key | `backend` (local verification) and `auth-service` |

The files are in `keys/` (prod) or `keys-dev/` (dev).

```bash
scripts/gen-auth-keys.sh
```

- **Idempotent.** If both keys already exist, nothing is overwritten and the script exits with code 0. Replacing the keys would make every token currently in use invalid.
- If only one key exists, the script stops with an error (a half-finished state is never guessed).
- Called automatically by `make up`, `make tunnel`, and `make prod-up`, so you normally do not need to run it manually.
- The `keys/` and `keys-dev/` folders are in `.gitignore`. Do not commit them.
- `AUTH_KEYS_DIR=<folder>` changes the location (default `keys`).

**Key rotation:** delete both files, run the script again, then `make restart` (or restart `backend` and `auth-service`). All users must log in again.

---

## check-service-isolation.sh

Checks `docker-compose.yml` and `docker-compose.prod.yml` and the code, to protect the guarantees below. Exits with code 1 and a `GAGAL [...]` message (Indonesian for "FAILED") when a guarantee is violated.

| Check | Guarantee |
|---|---|
| `notify-service` | Does not depend on `mail-service`, Mailpit, or SMTP. A dead email service must not stop push notifications |
| `auth-service` | No other service has `depends_on` on it. It depends only on `db`, and the **JWT private key is mounted only into `auth-service`**. This means logged-in users can keep using the app while it is down |
| Code | `notify-service` does not mention smtp or mailer. `backend` does not call `auth-service` |
| nginx | The `auth-service` upstream uses a variable plus a resolver, so nginx still starts if one upstream is down |
| Observability: overlay | With `docker-compose.observability.yml` (dev and prod): no `depends_on` from application services to observability components. Only Grafana publishes a port to the host (and only on `127.0.0.1`). Internal ports (9100, 8081, 4317, 9090, 3100, 3200) are not published. No `docker.sock`. Host directory mounts are read-only. Every component has a `mem_limit`. `OTEL_EXPORTER_OTLP_ENDPOINT` is set only for backend, auth, and mail, and only points to the collector |
| Observability: base Compose | Without the overlay, the base Compose file has no observability services and no `OTEL_*` variables, so the application starts as usual |
| Observability: nginx and code | nginx does not forward `/metrics` or `:9100`. The `stub_status` listener on `:8081` has `deny all`. The Go code does not hard-code observability component addresses |

```bash
scripts/check-service-isolation.sh
QMS_ENV_FILE=.env.prod scripts/check-service-isolation.sh
```

Run it every time you change `docker-compose*.yml` or `deploy/nginx/*.conf`. It is also run by `make obs-check`. The prod Compose file requires variables to be set. The script uses `${QMS_ENV_FILE:-.env}` and fills in fake SMTP values for parsing only.

---

## check-observability-leaks.sh

A secret-leak test for observability (goal 6 in the observability plan document, which is not included in this repository). It sends rejected requests (404, 401, 400) through nginx with **fake secrets that are unique for each run**: a public display token (in the path and query string), the password and full email in the login body, an email verification token, a password reset token, and a fake JWT in the `Authorization` header. After the pipeline has had time to process them (about 45 seconds), the script searches for those secrets in:

| Location | Method |
|---|---|
| Logs (Loki) | Free-text search across all services |
| Traces (Tempo) | Fetches the latest traces and scans all span content (attributes, events, status) |
| Metrics (Prometheus) | Scans all label values of `http_*`, `auth_*`, and `nginx_*` metrics |

Exit codes: `0` clean, `1` a leak was found (the secret and where it was found are printed), `2` the observability stack is not reachable, `3` no new nginx logs in Loki (the check is not meaningful). The script does not create data. It only adds a few counts to the failed-login and 404 metrics.

```bash
scripts/check-observability-leaks.sh
QMS_ENV_FILE=.env.prod scripts/check-observability-leaks.sh
LEAK_ROUNDS=80 scripts/check-observability-leaks.sh                 # more likely to capture a trace at 10% sampling
LEAK_EXTRA_CANARY="test-text" scripts/check-observability-leaks.sh  # positive control: must FAIL if "test-text" is in the logs
```

Prerequisites: the stack is running with the overlay (`make up obs`), plus `curl`, `openssl`, and `python3`. It reads `GRAFANA_ADMIN_PASSWORD`, `GRAFANA_PORT`, and `QMS_HTTP_PORT` from `.env` (or `QMS_ENV_FILE`). Other variables: `QMS_BASE_URL` (nginx address, default `http://localhost:<QMS_HTTP_PORT>`) and `LEAK_ROUNDS` (default 40). It is also run by `make obs-check`.

---

## check-google-config.sh

Checks the Google sign-in configuration in the env file before the stack starts. It is called automatically by `make up` and `make tunnel` (reads `.env`) and by `make prod-up` (reads `.env.prod`).

| Situation | Result |
|---|---|
| `GOOGLE_CLIENT_IDS` and all `EXPO_PUBLIC_GOOGLE_*` are empty | Google sign-in is disabled (button hidden, route returns 404). **This is not an error** |
| A frontend client ID is **missing** from `GOOGLE_CLIENT_IDS` | **FAILED (exit 1)**: every ID token is rejected by the server (401, `aud` does not match) |
| `EXPO_PUBLIC_GOOGLE_*` is set but `GOOGLE_CLIENT_IDS` is empty | **FAILED (exit 1)**: the button appears, but the server answers 404 |
| `GOOGLE_CLIENT_IDS` is set but the frontend values are empty | Warning: the button will not appear |
| A client ID does not end with `.apps.googleusercontent.com` | Warning: possibly a sample value or a typo |

```bash
scripts/check-google-config.sh            # checks .env
scripts/check-google-config.sh .env.prod  # explicit env file
```

The script only reads the file. It does not run anything and does not print client ID values.

### What the Makefile also does for Google sign-in

| Target | Additional behavior |
|---|---|
| `make up` | Runs the check. **Clears the Metro cache automatically** if `EXPO_PUBLIC_GOOGLE_*` in `.env` changed since the last build (marker file `.metro-google.stamp`, not committed to git). Prints a reminder that Google sign-in only works at `http://localhost:8080`, not at a LAN IP address |
| `make tunnel` | Same, and prints the tunnel origin that must be registered in Google Cloud (the URL changes every session, so it must be registered again each time) |
| `make prod-up` | Checks `.env.prod`. Prints the https origin that must be registered, and a reminder to update the `auth_rw` role on an existing database |

The Metro cache has to be cleared because Metro stores `EXPO_PUBLIC_*` values in its transform cache. Changing them in `.env` without clearing the cache makes the bundle use the old values. Prod builds are not affected: the values are passed as Docker build arguments, which rebuilds the layer automatically.

---

## check-mail-mode.sh

Checks `MAIL_MODE` in the prod env file before the stack starts. It is called automatically by `make prod-up`.

| `MAIL_MODE` | Meaning | Checks |
|---|---|---|
| `mailpit` (**default when empty**) | Email is captured by Mailpit and is **not** sent to anyone (demo or testing) | Warning only: the Mailpit UI is bound to `127.0.0.1`, and if email verification is enabled, new users will not receive links |
| `smtp` | Email is sent through a real SMTP server | `SMTP_HOST`, `SMTP_PORT`, `SMTP_FROM` must be set and must not be sample values (`GANTI`). The host must not be `mailpit`. `SMTP_TLS=none` is allowed only for localhost |

Checks for both modes: `MAIL_SERVICE_TOKEN` is set, is not a sample value, and is at least 32 characters. `MAIL_DB_PASSWORD` is set and is not a sample value.

```bash
scripts/check-mail-mode.sh                 # uses $QMS_ENV_FILE or .env.prod
scripts/check-mail-mode.sh .env.prod       # explicit env file
MAIL_MODE=smtp scripts/check-mail-mode.sh .env.prod
```

A `MAIL_MODE` set in the environment overrides the value in the file. Exit code 1 when the configuration is wrong.

---

## check-payment-config.sh

Checks the BorderPay payment configuration in the prod env file. It is run by `make prod-up` (before the stack starts) and by `make prod-payment-check` (at any time, without changing anything). The script only prints presence, length, and prefix. Secret values are never shown.

| Variable | Check | Result when failing |
|---|---|---|
| `BORDERPAY_API_KEY` | Set, starts with `bp_test_` or `bp_live_` | Error if empty or another prefix. `bp_test_` is only a warning: real payments will not be accepted |
| `BORDERPAY_WEBHOOK_TOKEN` | Set, at least 32 characters, not a sample value. Recommended to start with `bpt_` | Error. `bpt_` is only a warning |
| `PAYMENT_SERVICE_TOKEN` | Set, at least 32 characters, not a sample value | Error (checkout and webhook will fail) |
| `PAYMENT_DB_PASSWORD` | Set and not a sample value | Error (payment-service cannot connect to the database) |

```bash
scripts/check-payment-config.sh                # uses $QMS_ENV_FILE or .env.prod
scripts/check-payment-config.sh .env.prod      # explicit env file
```

Exit code 1 if there is an error, so `make prod-up` stops before the payment container is rebuilt.

The database privilege check (`db/scripts/check_payment_grants.sql`) is run separately by `make prod-up` and `make prod-payment-check`, because it needs a running database.

---

## verify-user.sh

Sets `users.email_verified_at` for one account directly in the database, without waiting for an email. Useful when the verification email cannot be delivered: `MAIL_MODE=mailpit`, SMTP is not set up yet, or the email address was mistyped. Idempotent.

```bash
scripts/verify-user.sh budi@mail.com          # dev stack
scripts/verify-user.sh budi@mail.com prod     # prod stack (env from $QMS_ENV_FILE, default .env.prod)

make verify-user EMAIL=budi@mail.com          # via make
make verify-user EMAIL=budi@mail.com STACK=prod
```

Result: `OK` (changed), `Sudah terverifikasi` (already verified, nothing changed), or `GAGAL` (email not found, exit code 1). The script prints these messages in Indonesian. The email is passed as a psql variable, not inserted into the SQL text. The second argument may only be `prod`; any other value makes the script exit with code 1.

---

## reset-demo.sh

**Full reset** of the prod database. The volume `qms-prod_db_data_prod` is deleted, then `make prod-up` starts the stack again with an empty database. The `db` container recreates the schema and roles from `db/migrations/`. **All accounts (owners, customers, testers), queues, tickets, history, and payments are lost.**

Script steps: check that the stack is running, ask for confirmation (type `hapus-prod`), back up (`backup-prod.sh`), `down`, delete the volume, `make prod-up`, wait for `db` to become healthy, then create `_no_seed_guard` so that dev seeds and resets are refused on this database.

```bash
scripts/reset-demo.sh        # asks for confirmation (type 'hapus-prod')
scripts/reset-demo.sh -y     # no confirmation
make prod-reset
```

After a reset, recreate the tester account and test data with `make prod-seed-penguji`. To restore earlier data: `scripts/backup-prod.sh restore <file>` (overwrites the prod database).

---

## seed-prod-penguji.sh

Test data for a tester who has only **one account** (`tester@example.com`, username `tester`). That account is both owner and customer, so every queue feature can be tested from one login.

Each feature has its own queue, and each active queue contains 5 to 12 waiting customers:

| Prefix | Queue | What to test |
|---|---|---|
| N | Demo NEXT | Repeated NEXT (10 waiting), public display |
| U | Demo Recall | RECALL on the number being served |
| S | Demo Skip | SKIP, skip setting "return to the back" |
| H | Demo Remove Number | Owner removes a number (customer gets an email and their screen updates) |
| B | Demo Cancel Number | Customer cancels. The tester account already has a number here (number 3) |
| I | Demo Insert | Owner inserts customers `customer-7` through `customer-12` by username |
| C | Demo Notes | Per-number notes (already present on numbers 1, 3, and 5) |
| K | Demo Counter | Counter CRUD. Counter 2 is serving number 1 |
| T | Demo Close Queue | Closes the queue (moves it to history) |
| R | Long History | Closed queue with 25 numbers, for History pagination and export |

The tester account can also join by scanning its own queue's QR code with a phone (same account) to test the customer side: My Queues, My History, and notifications.

The test data is in [`db/scripts/seed_prod_penguji.sql`](../db/scripts/seed_prod_penguji.sql). The seeded customers (`customer-1` through `customer-12`) get their own random password on each run and can log in with their username. Their email notifications are turned off.

Each run creates a new random password for the tester account and for **all** seeded customers, then writes the email, username, and password to `~/qms-penguji-account.txt` (mode 600, outside the repository, overwritten on each run). All existing login sessions of these accounts are revoked. Run the same script again to bring the test data back to its seeded state.

```bash
make prod-seed-penguji        # or: scripts/seed-prod-penguji.sh [-y]
```

- **Seed:** each run creates a random 16-character password for the tester account. If the account does not exist yet, it is registered through the API. If it exists, its password is replaced using a bcrypt hash. Credentials are written to `~/qms-penguji-account.txt` (mode 600, outside the repository) and **overwritten** on each run. Each run also revokes the tester's existing login sessions. The email and username can be changed with `PENGUJI_EMAIL` and `PENGUJI_USERNAME`. The verification email from registration is sent according to `MAIL_MODE`. `QMS_API` (default `http://localhost:8080`) sets the API address for registration, and `PENGUJI_FILE` changes the credentials file location. This script also resets the test data to its seeded state, so no separate restore script is needed.
- The script makes a database backup first (`scripts/backup-prod.sh`) and asks for confirmation (type `ya`, Indonesian for "yes") unless `-y` is used. It needs the Python module `bcrypt` on the host.
- Only the tester account, its tickets, and the seeded customer accounts are changed. Other accounts and other production data are not touched.
- Run the seed **after** the testing session is finished. If `make prod-reset` is run, the tester account and data are deleted too, so run `make prod-seed-penguji` afterwards. The new password is written to the credentials file.
- QR codes and public display tokens change every time the data is seeded. Print the QR code again from the owner screen, and use the public display link shown when the script finishes.

---

## backup-prod.sh

Backs up and restores the prod database using `pg_dump` and `psql` inside the `db` container.

```bash
scripts/backup-prod.sh backup                  # ~/qms-backups/qms-<timestamp>.sql.gz (outside the repository)
scripts/backup-prod.sh backup /path/file.sql.gz
scripts/backup-prod.sh restore /path/file.sql.gz
make prod-backup
```

- Backup files are created with `umask 077` (readable only by the owner). An empty backup is treated as a failure and deleted.
- The dump uses `--clean --if-exists`, so **restore overwrites the contents of the prod database**. The script asks you to type `ya`.
- The env file is read from `$QMS_ENV_FILE` (default `.env`; for prod it is usually `QMS_ENV_FILE=.env.prod`, which `make prod-backup` sets).
- The tables owned by `auth-service` (`users`, `user_action_tokens`) are included in the backup because they live in the same database. The JWT keys (`keys/`) are **not** included. Store a backup of the keys separately: without the same keys, old tokens are invalid, but users can log in again and no accounts are lost.

---

## SQL scripts in db/

These are not in this folder, but they are often needed together with the scripts above: `db/scripts/apply_schema.sql` (idempotent schema adjustments for an existing database), `db/scripts/reset_seed_dev.sql` (dev seed data, `make seed`), `db/scripts/backfill_currently_served.sql`, and `db/scripts/check_payment_grants.sql` (read-only: checks that the `payment_rw` role has all privileges payment-service needs; called by `make prod-up` and `make prod-payment-check`). The per-service roles are created by `db/migrations/03_notify_role.sh`, `04_mail_role.sh`, `06_auth_role.sh`, and `07_metrics_role.sh` (`metrics_ro` for postgres_exporter, re-run by the observability targets). `08_payment_role.sh` for `payment_rw` is also re-run by `make prod-up`. For an older database that does not have the `auth_rw` role yet, run `06_auth_role.sh` as described in `.env.example`.

---

## Related Makefile targets

Summary of targets that call the scripts above or replace manual steps. `make up obs` (or `obs=1`) adds the observability overlay.

| Target | Purpose |
|---|---|
| `make up`, `make tunnel` | Dev stack (LAN or Cloudflare quick tunnel). Creates `keys-dev/`, checks Google configuration, clears the Metro cache if needed |
| `make down` | Stops the tunnel and all dev containers, removes `keys-dev/` and the Metro marker |
| `make restart`, `make logs`, `make ps` | Restarts `backend`, `auth-service`, `notify-service`, `mail-service`, and `payment-service` (profile `payments`); follows logs; dev container status |
| `make seed` | Resets the dev database to seed data (`db/scripts/reset_seed_dev.sql`) |
| `make verify-user EMAIL=... [STACK=prod]` | See [`verify-user.sh`](#verify-usersh) |
| `make prod-up` | Runs `gen-auth-keys.sh`, `check-mail-mode.sh`, `check-google-config.sh`, and `check-payment-config.sh`. Starts `db`, runs `08_payment_role.sh` and `check_payment_grants.sql` (an error stops the target), then starts the whole stack (the `payments` profile is always enabled) |
| `make prod-up-mailpit`, `make prod-up-smtp` | Shortcuts for `prod-up` with a specific `MAIL_MODE` |
| `make prod-payment-check` | `check-payment-config.sh`, `check_payment_grants.sql`, and `payment-service` status, without changing anything |
| `make prod-down` | Stops the prod stack (including Mailpit and orphans) |
| `make prod-guard` | Creates the `_no_seed_guard` table in the prod database so that dev seeds and resets are refused (already done by `reset-demo.sh`) |
| `make prod-backup`, `make prod-reset`, `make prod-seed-penguji` | See [`backup-prod.sh`](#backup-prodsh), [`reset-demo.sh`](#reset-demosh), [`seed-prod-penguji.sh`](#seed-prod-pengujish) |
| `make obs-up`, `make obs-down`, `make obs-url`, `make obs-check` | Observability: starts the overlay and the `metrics_ro` role, stops only its components, prints the Grafana address, runs `check-service-isolation.sh` and `check-observability-leaks.sh`. `OBS_TARGET=prod` for the prod stack |
| `make mailpit` | Prints the dev and prod Mailpit links and the active prod `MAIL_MODE` (default `mailpit`) |

`PROD_ENV` (default `.env.prod`) replaces the prod env file for all `prod-*` targets.