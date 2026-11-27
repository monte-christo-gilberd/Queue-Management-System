# =============================================================================
# Makefile: daily commands for the QMS stack (Docker Compose + nginx).
# -----------------------------------------------------------------------------
# Run from the queue-management-system/ root. Two stacks:
#   dev   -> docker-compose.yml       (hot reload, mounted source code)
#   prod  -> docker-compose.prod.yml  (demo/production, env from .env.prod)
#
# DEV commands (reads .env):
#   make up                         build + start; LAN URL http://<laptop IP>:8080 (mobile on same Wi-Fi).
#                                   Google Login: open http://localhost:8080 (not LAN IP), see notes below
#   make up IP=192.168.1.5          same, but uses the specified LAN IP
#   make tunnel                     public HTTPS via Cloudflare quick tunnel (mobile camera works)
#   make down                       stop stack and tunnel
#   make restart                    restart backend, auth-service, notify-service, mail-service, payment-service (if running)
#                                   (used after modifying Go code)
#   make seed                       reset dev database to seed data
#   make logs                       follow logs of all services (last 50 lines)
#   make ps                         container status
#   make mailpit                    show dev and prod Mailpit links without running anything
#   make verify-user EMAIL=a@b.com [STACK=prod]
#                                   mark a single email as verified without waiting for email delivery
#
# PROD commands (reads .env.prod, copied from .env.prod.example):
#   make prod-up [MAIL_MODE=smtp|mailpit]   start prod stack (defaults to MAIL_MODE=smtp; MAIL_MODE overrides .env.prod)
#   make prod-up-mailpit                    shorthand for prod-up with MAIL_MODE=mailpit
#   make prod-payment-check                 check BorderPay config and payment_rw role privileges (read-only)
#   make prod-up-smtp                       shorthand for prod-up with MAIL_MODE=smtp
#   make prod-down                          stop prod stack (including Mailpit container)
#   make prod-guard                         mark prod DB so seed/reset is rejected (run once)
#   make prod-backup                        backup prod DB to ~/qms-backups/
#   make prod-reset                         FULL RESET of prod database: all accounts and data wiped, start fresh (backs up first)
#   make prod-seed-penguji                  create/reset tester and seed customer accounts, new random password per account, reload test data (backs up first)
#
# `obs` keyword (optional): make up obs, make tunnel obs, make prod-up obs also starts observability (collector, Loki,
# Grafana) and attaches log collection to application services, without running a separate make obs-up. `obs=1` form is also
# accepted. Without it, the stack runs normally without observability. Requires GRAFANA_ADMIN_PASSWORD in the env file.
#
# OBSERVABILITY commands (optional, docker-compose.observability.yml overlay; OBS_TARGET=dev|prod, defaults to dev):
#   make obs-up [OBS_TARGET=prod]           start collector, Loki, Tempo, Prometheus, exporters, Grafana, and attach log & trace collection to application
#                                           services (application services are recreated once). Requires GRAFANA_ADMIN_PASSWORD and
#                                           METRICS_DB_PASSWORD in .env (dev) or $(PROD_ENV) (prod). Database role
#                                           metrics_ro is created/updated automatically.
#   make obs-down [OBS_TARGET=prod]         stop and remove observability containers; application keeps running.
#                                           Data volumes (logs, Grafana) are preserved.
#   make obs-url [OBS_TARGET=prod]          display Grafana URL and remote access instructions (SSH tunnel)
#   make obs-check [OBS_TARGET=prod]        verify: static overlay isolation (check-service-isolation.sh) and secret
#                                           leak test into logs, traces, metrics (check-observability-leaks.sh;
#                                           requires running stack with overlay, ~1.5 minutes)
#
# Notes:
#   - Browser camera requires HTTPS or localhost. QR scanning from mobile via LAN IP does not work on web;
#     use `make tunnel`. Tunnel requires cloudflared and internet access.
#   - Tunnel URL is randomized each session. Backend, auth-service, and mail-service read it on startup (QMS_PUBLIC_ORIGIN, for
#     CORS and links in emails).
#   - Google Login (optional; empty = button hidden). Variables: GOOGLE_CLIENT_IDS (server) and
#     EXPO_PUBLIC_GOOGLE_{WEB,ANDROID,IOS}_CLIENT_ID (frontend) in .env / .env.prod. Setup: see scripts/README.md under check-google-config.sh
#       make up / tunnel / prod-up checks its config first (scripts/check-google-config.sh) and aborts if out of
#       sync (login would definitely fail). make up and tunnel also automatically clear Metro cache if EXPO_PUBLIC_GOOGLE_* changes.
#       Google only accepts https or http://localhost origins: in dev use http://localhost:8080 (not LAN IP); in tunnel
#       the random URL must be registered in Google Cloud each session; in prod the static https origin must already be registered.
#   - Prod email mode:
#       MAIL_MODE=smtp     emails are sent via real SMTP (configure SMTP_* in .env.prod). DEFAULT when MAIL_MODE is unset.
#       MAIL_MODE=mailpit  emails are captured by Mailpit and NOT delivered to anyone (demo/testing).
# =============================================================================

SHELL := bash
.SHELLFLAGS := -euo pipefail -c
# Run recipes in a single shell (variables persist across lines).
.ONESHELL:
# Do not echo commands before execution. Explicit `echo` output is still shown.
.SILENT:
.PHONY: obs up tunnel down restart seed logs ps mailpit verify-user \
        prod-up prod-up-mailpit prod-up-smtp prod-down prod-guard prod-backup prod-reset \
        prod-seed-penguji prod-payment-check \
        obs-up obs-down obs-url obs-check

# -----------------------------------------------------------------------------
# Variables and shared snippets
# -----------------------------------------------------------------------------

# Marker of the last EXPO_PUBLIC_GOOGLE_* values built into Metro cache (gitignored). See google_metro_cache.
GOOGLE_STAMP := .metro-google.stamp

# Passing `obs` on the command line (make up obs) is equivalent to obs=1: the word enters MAKECMDGOALS
# and the `obs` target below is intentionally empty. Therefore, never use `obs` as a filename or another target.
ifneq ($(filter obs,$(MAKECMDGOALS)),)
obs := 1
endif

# obs=1 adds the observability overlay to DEV compose commands (up, tunnel). Empty unless obs is set to 1.
# OBS_SERVICES: overlay services started together during `make tunnel` (service list there is explicit).
OBS_DEV_FILES := $(if $(filter 1,$(obs)),-f docker-compose.yml -f docker-compose.observability.yml)
OBS_SERVICES := $(if $(filter 1,$(obs)),collector loki grafana prometheus cadvisor node-exporter postgres-exporter nginx-exporter tempo)

# With obs=1: abort if GRAFANA_ADMIN_PASSWORD or METRICS_DB_PASSWORD is empty in env file $(1), before recreating any container.
define obs_require_password
$(if $(filter 1,$(obs)),for v in GRAFANA_ADMIN_PASSWORD METRICS_DB_PASSWORD; do grep -qE "^$$v=.+" $(1) || { echo "obs: $$v kosong di $(1): isi dulu (tanpa nilai bawaan)" >&2; exit 1; }; done)
endef

# Dev JWT keys in keys-dev/ are regenerated on every `make up` / `make tunnel` and deleted on `make down`.
# Prod keys in keys/ are never touched by dev targets. rm -rf (not rm -f) also cleans up directories wrongly created by Docker.
# If keys-dev/ is owned by root (often created by Docker when bind-mounted key files do not exist yet), rm fails: abort with a clear
# message instead of proceeding to `docker compose up` with invalid keys (backend crash-loop: "is a directory").
# After generation, verify both key files are regular files.
define fresh_auth_keys
{ [ ! -e keys-dev ] || rm -rf keys-dev 2>/dev/null || { echo "GAGAL: keys-dev/ tidak bisa dihapus (milik root, biasanya dibuat Docker). Jalankan: sudo rm -rf keys-dev lalu ulangi." >&2; exit 1; }; } && AUTH_KEYS_DIR=keys-dev scripts/gen-auth-keys.sh && { [ -f keys-dev/auth-public.pem ] && [ -f keys-dev/auth-private.pem ] || { echo "GAGAL: keys-dev/ tidak berisi kedua berkas kunci." >&2; exit 1; }; }
endef

# Temporary files for the cloudflared process (tunnel).
PIDFILE := /tmp/qms-tunnel.pid
LOG := /tmp/qms-tunnel.log

# Prod env file. Can be overridden: make prod-up PROD_ENV=.env.other
PROD_ENV ?= .env.prod

# Read QMS_HTTP_PORT from .env (default 8080) into shell variable `port`.
# Used by up, tunnel, and warm_metro.
define need_env
[ -f .env ] || { echo ".env belum ada (cp .env.example .env lalu isi secret)" >&2; exit 1; }
port=$$(grep -E '^QMS_HTTP_PORT=' .env | cut -d= -f2 || true); port=$${port:-8080}
endef

# Stop cloudflared recorded in PIDFILE, then remove the PID file.
define stop_tunnel
[ -f $(PIDFILE) ] && kill "$$(cat $(PIDFILE))" 2>/dev/null || true; rm -f $(PIDFILE)
endef

# Print dev Mailpit link. UI is bound only to 127.0.0.1 on this machine: not exposed via nginx, LAN, or tunnel.
# Other devices must use an SSH tunnel.
define mailpit_dev
mp=$$(grep -E '^MAILPIT_UI_PORT=' .env 2>/dev/null | tail -1 | cut -d= -f2 || true); mp=$${mp:-8025}
echo "Mailpit (email dev, hanya dari mesin ini): http://localhost:$$mp"
echo "  dari perangkat lain: ssh -L $$mp:127.0.0.1:$$mp <user>@<IP mesin ini>, lalu buka http://localhost:$$mp"
endef

# Metro caches EXPO_PUBLIC_* values in its transform cache: changing them in .env without clearing the cache causes the bundle
# to keep using OLD values (Google button missing or using old client ID). This macro compares EXPO_PUBLIC_GOOGLE_* values
# in .env against the last stamp, and if changed removes the metro_cache volume (frontend container is stopped first
# because the volume is in use). Also clears once on the first run with populated values.
define google_metro_cache
sig=$$(grep -E '^EXPO_PUBLIC_GOOGLE_[A-Z]+_CLIENT_ID=' .env 2>/dev/null | sort | sha256sum | cut -d' ' -f1 || true)
filled=$$(grep -cE '^EXPO_PUBLIC_GOOGLE_[A-Z]+_CLIENT_ID=.+' .env 2>/dev/null || true)
old=$$(cat $(GOOGLE_STAMP) 2>/dev/null || true)
if [ "$$sig" != "$$old" ] && { [ -n "$$old" ] || [ "$${filled:-0}" -gt 0 ]; }; then
  echo "EXPO_PUBLIC_GOOGLE_* berubah: membersihkan cache Metro agar bundle memakai nilai baru (sekali, +30-40 detik)..."
  docker compose rm -sf frontend >/dev/null 2>&1 || true
  vol=$$(docker volume ls -q --filter label=com.docker.compose.project=qms --filter label=com.docker.compose.volume=metro_cache || true)
  [ -z "$$vol" ] || docker volume rm -f $$vol >/dev/null
fi
printf '%s' "$$sig" > $(GOOGLE_STAMP)
endef

# Google Login hint after DEV stack starts. Shown only if Web client ID is populated.
define google_hint_dev
if grep -qE '^EXPO_PUBLIC_GOOGLE_WEB_CLIENT_ID=.+' .env 2>/dev/null; then
  echo "Login Google: buka http://localhost:$$port (BUKAN alamat IP LAN). Google dan WebCrypto hanya menerima https atau localhost."
  echo "  Pastikan http://localhost:$$port dan http://localhost:$$port/ terdaftar di Authorized JavaScript origins dan redirect URIs."
fi
endef

# Pre-warm Metro (dev) so bundling finishes before the browser opens the URL (cold cache takes ~30–40s).
# Warmup failures only emit warnings; they do not fail make.
define warm_metro
echo "Memanaskan Metro (cache kosong bisa ±30–40 detik)..."
curl -fsS -o /dev/null --retry 30 --retry-delay 2 --retry-all-errors "http://localhost:$$port/" || echo "Pemanasan halaman gagal; muat pertama bisa lambat." >&2
curl -fsS -o /dev/null --max-time 300 --retry 5 --retry-delay 2 --retry-all-errors "http://localhost:$$port/node_modules/expo-router/entry.bundle?platform=web&dev=true&hot=false&lazy=true&transform.routerRoot=src%2Fapp&transform.reactCompiler=true" || echo "Pemanasan bundle gagal." >&2
endef

# -----------------------------------------------------------------------------
# DEV Stack
# -----------------------------------------------------------------------------

# --renew-anon-volumes: repopulates the anonymous /app/node_modules volume from the image on every `up`. Without this, stale volumes
# shadow image node_modules and new packages in package.json go missing (Metro: "Unable to resolve module").
# Build and start dev stack. LAN IP is auto-detected from the default route, or taken from IP=... if provided.
up:
	$(need_env)
	$(call obs_require_password,.env)
	$(fresh_auth_keys)
	scripts/check-google-config.sh .env
	$(google_metro_cache)
	ip="$(IP)"
	[ -n "$$ip" ] || ip=$$(ip route get 1.1.1.1 2>/dev/null | grep -oP 'src \K\S+' || true)
	[ -n "$$ip" ] || { echo "IP LAN tidak terdeteksi; beri manual: make up IP=<ip>" >&2; exit 1; }
	export QMS_PUBLIC_ORIGIN="http://$$ip:$$port"
	$(if $(filter 1,$(obs)),$(obs_dev_sampling))
	docker compose $(OBS_DEV_FILES) up -d --build --force-recreate --renew-anon-volumes
	$(if $(filter 1,$(obs)),$(call obs_db_role_run,docker compose $(OBS_DEV_FILES),.env))
	$(warm_metro)
	echo "Link (satu Wi-Fi): $$QMS_PUBLIC_ORIGIN"
	$(google_hint_dev)
	$(mailpit_dev)
	$(if $(filter 1,$(obs)),$(call obs_url,.env,dev))

# Dev stack with public HTTPS via Cloudflare quick tunnel. URL is parsed from cloudflared logs.
# Recreated services receive QMS_PUBLIC_ORIGIN = tunnel URL (service list is explicit in the compose command).
tunnel:
	command -v cloudflared >/dev/null || { echo "cloudflared belum terpasang" >&2; exit 1; }
	$(need_env)
	$(call obs_require_password,.env)
	$(fresh_auth_keys)
	scripts/check-google-config.sh .env
	$(google_metro_cache)
	$(stop_tunnel); rm -f $(LOG)
	cloudflared tunnel --config /dev/null --protocol http2 --no-autoupdate --url "http://localhost:$$port" >$(LOG) 2>&1 &
	echo $$! > $(PIDFILE)
	url=""
	for _ in $$(seq 1 40); do
	  url=$$(grep -aoE 'https://[a-z0-9-]+\.trycloudflare\.com' $(LOG) | head -1 || true)
	  [ -n "$$url" ] && break; sleep 1
	done
	[ -n "$$url" ] || { echo "URL tunnel tidak muncul, lihat $(LOG)" >&2; $(stop_tunnel); exit 1; }
	$(if $(filter 1,$(obs)),$(obs_dev_sampling))
	QMS_PUBLIC_ORIGIN="$$url" docker compose $(OBS_DEV_FILES) up -d --build --force-recreate --renew-anon-volumes backend auth-service notify-service mail-service mailpit nginx frontend db $(OBS_SERVICES)
	$(if $(filter 1,$(obs)),$(call obs_db_role_run,docker compose $(OBS_DEV_FILES),.env))
	$(warm_metro)
	echo "Link: $$url"
	if grep -qE '^EXPO_PUBLIC_GOOGLE_WEB_CLIENT_ID=.+' .env 2>/dev/null; then
	  echo "Login Google lewat tunnel: daftarkan origin ini di Google Cloud (Credentials > client Web) SEBELUM menekan tombol:"
	  echo "  Authorized JavaScript origins : $$url"
	  echo "  Authorized redirect URIs      : $$url  dan  $$url/"
	  echo "  URL tunnel berubah tiap sesi, jadi harus didaftarkan ulang setiap make tunnel; perubahan berlaku 1-5 menit."
	  echo "  Untuk URL tetap, pakai stack prod (domain https tetap)."
	fi
	echo "Hentikan: make down"
	$(mailpit_dev)
	echo "  (Mailpit tidak ikut lewat tunnel Cloudflare; email dari aplikasi tetap tertangkap di sini)"
	$(if $(filter 1,$(obs)),$(call obs_url,.env,dev))

# Stop tunnel (if running) and all dev containers.
down:
	$(stop_tunnel)
	# --remove-orphans: also removes observability containers (collector, loki, grafana) if the overlay was previously started.
	docker compose --profile '*' down --remove-orphans
	rm -f $(GOOGLE_STAMP)
	rm -rf keys-dev 2>/dev/null || { echo "PERINGATAN: keys-dev/ tidak terhapus (milik root). Jalankan: sudo rm -rf keys-dev" >&2; exit 1; }

# Restart services that need reloading after Go code changes.
restart:
	docker compose --profile payments restart backend auth-service notify-service mail-service payment-service

# Reset dev database to seed data (db/scripts/reset_seed_dev.sql).
seed:
	docker compose exec -T db psql -U qms -d qms_db < db/scripts/reset_seed_dev.sql

# Follow logs for all services, last 50 lines.
logs:
	docker compose logs -f --tail 50

# Dev container status.
ps:
	docker compose ps

# Mark a single email as verified directly in the database. Useful when MAIL_MODE=mailpit or emails are not delivered.
# STACK=prod targets the prod database (env from $(PROD_ENV)).
verify-user:
	[ -n "$(EMAIL)" ] || { echo "pakai: make verify-user EMAIL=alamat@contoh.com [STACK=prod]" >&2; exit 1; }
	QMS_ENV_FILE=$(PROD_ENV) scripts/verify-user.sh "$(EMAIL)" $(if $(filter prod,$(STACK)),prod)

# -----------------------------------------------------------------------------
# PROD Stack (docker-compose.prod.yml, env from $(PROD_ENV))
# -----------------------------------------------------------------------------

# MAIL_MODE is read from $(PROD_ENV). Command-line override (make prod-up MAIL_MODE=...) takes precedence over the file.
# Empty in file = smtp (default): emails are sent via real SMTP (configure SMTP_* in .env.prod). Use mailpit for local demo/testing.
MAIL_MODE := $(or $(strip $(shell grep -E '^MAIL_MODE=' $(PROD_ENV) 2>/dev/null | tail -1 | cut -d= -f2 | tr -d '"' | tr -d "'")),smtp)

# mailpit mode: activates the `mailpit` profile and forces SMTP_* to Mailpit. These `export` variables override .env.prod
# for every command invoked in this Makefile.
ifeq ($(MAIL_MODE),mailpit)
export SMTP_HOST := mailpit
export SMTP_PORT := 1025
export SMTP_TLS := none
export SMTP_USER :=
export SMTP_PASSWORD :=
export SMTP_FROM := Antrian <no-reply@qms.local>
PROD_PROFILE := --profile mailpit
endif

# Full prod compose command (files, env, and profiles).
# obs=1 adds the observability overlay; OBS_PROJECT tells the collector which project to scrape.
# The `payments` profile is always enabled in prod: payment-service is rebuilt and started on every prod-up.
# Its configuration is validated first by scripts/check-payment-config.sh so it will not start in a broken state.
PROD := $(if $(filter 1,$(obs)),OBS_PROJECT=qms-prod )docker compose -f docker-compose.prod.yml $(if $(filter 1,$(obs)),-f docker-compose.observability.yml )--env-file $(PROD_ENV) $(PROD_PROFILE) --profile payments

# Create or update the payment_rw role via db/migrations/08_payment_role.sh (idempotent), then verify the privileges
# required by payment-service (db/scripts/check_payment_grants.sql). Unlike obs_db_role_run, errors here
# ABORT prod-up: missing grants cause BorderPay webhooks to fail with 500 without any other hint in the app.
# Script 08 only runs on an empty database volume, so existing prod databases require this step.
define payment_role_run
pw=$$(grep -E '^PAYMENT_DB_PASSWORD=' $(PROD_ENV) | tail -1 | cut -d= -f2- | tr -d '"' | tr -d "'" || true)
ready=
for _ in $$(seq 1 30); do
  if $(PROD) exec -T db sh -c 'pg_isready -q -d "$$POSTGRES_DB"' >/dev/null 2>&1; then ready=1; break; fi
  sleep 1
done
if [ -z "$$ready" ]; then echo "GAGAL: database prod belum siap; role payment_rw tidak diperbarui." >&2; exit 1; fi
$(PROD) exec -T -e PAYMENT_DB_PASSWORD="$$pw" db sh < db/migrations/08_payment_role.sh
$(PROD) exec -T db sh -c 'psql -v ON_ERROR_STOP=1 -U "$$POSTGRES_USER" -d "$$POSTGRES_DB"' < db/scripts/check_payment_grants.sql
endef

# Start prod stack. Validates MAIL_MODE first (scripts/check-mail-mode.sh) and rejects invalid configurations.
# When switching from mailpit to smtp, any leftover Mailpit container is stopped to avoid confusion.
prod-up:
	scripts/gen-auth-keys.sh
	MAIL_MODE=$(MAIL_MODE) QMS_ENV_FILE=$(PROD_ENV) scripts/check-mail-mode.sh
	scripts/check-google-config.sh $(PROD_ENV)
	scripts/check-payment-config.sh $(PROD_ENV)
	$(call obs_require_password,$(PROD_ENV))
	$(PROD) up -d --build db
	$(call payment_role_run)
	$(PROD) up -d --build
	$(if $(filter 1,$(obs)),$(call obs_db_role_run,$(PROD),$(PROD_ENV)))
	$(if $(PROD_PROFILE),,docker compose -f docker-compose.prod.yml --env-file $(PROD_ENV) --profile mailpit rm -sf mailpit >/dev/null 2>&1 || true)
	pmp=$$(grep -E '^MAILPIT_UI_PORT=' $(PROD_ENV) 2>/dev/null | tail -1 | cut -d= -f2 || true); pmp=$${pmp:-8025}
	if [ "$(MAIL_MODE)" = mailpit ]; then
	  echo "Mailpit (prod, MAIL_MODE=mailpit): http://localhost:$$pmp  (hanya dari server ini; email TIDAK terkirim ke penerima sebenarnya)"
	  echo "  dari jauh: ssh -L $$pmp:127.0.0.1:$$pmp <user>@<server>, lalu buka http://localhost:$$pmp"
	else
	  echo "Email prod: SMTP nyata (MAIL_MODE=smtp), tanpa Mailpit. Lihat hasil kirim di inbox penerima atau dashboard penyedia SMTP."
	fi
	$(if $(filter 1,$(obs)),$(call obs_url,$(PROD_ENV),prod))
	if grep -qE '^EXPO_PUBLIC_GOOGLE_WEB_CLIENT_ID=.+' $(PROD_ENV) 2>/dev/null; then
	  origin=$$(grep -E '^QMS_PUBLIC_ORIGIN=' $(PROD_ENV) | tail -1 | cut -d= -f2- | tr -d '"' | tr -d "'" || true)
	  echo "Login Google prod: pastikan $$origin dan $$origin/ terdaftar di Authorized JavaScript origins dan redirect URIs (client Web)."
	  echo "  Role database auth_rw harus punya izin terbaru: bila DB prod sudah ada, jalankan ulang db/migrations/06_auth_role.sh (lihat db/migrations/README.md)."
	fi

# Shortcut: prod-up with MAIL_MODE=mailpit.
prod-up-mailpit:
	$(MAKE) prod-up MAIL_MODE=mailpit $(if $(filter 1,$(obs)),obs=1)

# Shortcut: prod-up with MAIL_MODE=smtp. `obs` keyword is forwarded to sub-make.
prod-up-smtp:
	$(MAKE) prod-up MAIL_MODE=smtp $(if $(filter 1,$(obs)),obs=1)

# Check payment setup without modifying anything: env file config, payment_rw privileges in database, and container status.
prod-payment-check:
	scripts/check-payment-config.sh $(PROD_ENV)
	$(PROD) exec -T db sh -c 'psql -v ON_ERROR_STOP=1 -U "$$POSTGRES_USER" -d "$$POSTGRES_DB"' < db/scripts/check_payment_grants.sql
	$(PROD) ps payment-service

# Stop prod stack. Uses --profile '*' so Mailpit containers also stop even when currently in smtp mode.
# Dummy SMTP values are only needed so compose can parse the file; they do not affect `down`.
prod-down:
	SMTP_HOST=$${SMTP_HOST:-x} SMTP_PORT=$${SMTP_PORT:-1} SMTP_FROM=$${SMTP_FROM:-x@x.y} MAIL_SERVICE_TOKEN=$${MAIL_SERVICE_TOKEN:-x} \
	docker compose -f docker-compose.prod.yml --env-file $(PROD_ENV) --profile '*' down --remove-orphans

# Mark prod DB so seed/reset is rejected. Run once after demo accounts are created.
prod-guard:
	$(PROD) exec -T db psql -U qms -d qms_db -c "CREATE TABLE IF NOT EXISTS _no_seed_guard (x int)"

# Backup prod DB to ~/qms-backups/ (scripts/backup-prod.sh).
prod-backup:
	QMS_ENV_FILE=$(PROD_ENV) scripts/backup-prod.sh backup

# FULL RESET of prod database: wipe DB volume, restart stack with an empty DB. All accounts are deleted. Backs up first (scripts/reset-demo.sh).
prod-reset:
	QMS_ENV_FILE=$(PROD_ENV) scripts/reset-demo.sh

# Tester accounts (owner + customer) and seed customers: new random password per account (written to ~/qms-penguji-account.txt),
# and reloads queue test data back to seed state. Also used to restore test data (scripts/seed-prod-penguji.sh).
prod-seed-penguji:
	QMS_ENV_FILE=$(PROD_ENV) scripts/seed-prod-penguji.sh

# -----------------------------------------------------------------------------
# OBSERVABILITY (docker-compose.observability.yml overlay, see that file and the Observability section in README.md)
# -----------------------------------------------------------------------------

# Overlay target: dev (default) or prod. Determines compose files, env file, and project name read by the collector.
OBS_TARGET ?= dev
ifeq ($(OBS_TARGET),prod)
OBS_ENV_FILE := $(PROD_ENV)
export OBS_PROJECT := qms-prod
# `=` (not :=) so PROD_PROFILE defined above is evaluated properly.
OBS_COMPOSE = docker compose -f docker-compose.prod.yml -f docker-compose.observability.yml --env-file $(PROD_ENV) $(PROD_PROFILE)
else
OBS_ENV_FILE := .env
export OBS_PROJECT := qms
OBS_COMPOSE = docker compose -f docker-compose.yml -f docker-compose.observability.yml
endif

# Create or update the metrics_ro database role (postgres_exporter) via db/migrations/07_metrics_role.sh, which is
# idempotent. Invoked via $(call obs_db_role_run,<compose command>,<env file>). Retries for up to 30 seconds because
# the database may have just started. Failure only prints a warning: the application does not depend on this role.
define obs_db_role_run
pw=$$(grep -E '^METRICS_DB_PASSWORD=' $(2) | tail -1 | cut -d= -f2- | tr -d '"' | tr -d "'" || true)
ok=
for _ in $$(seq 1 30); do
  if $(1) exec -T -e METRICS_DB_PASSWORD="$$pw" db sh < db/migrations/07_metrics_role.sh >/dev/null 2>&1; then ok=1; break; fi
  sleep 1
done
if [ -n "$$ok" ]; then echo "Role database metrics_ro siap."; else echo "Peringatan: role metrics_ro belum dibuat (database belum siap?). Jalankan make obs-up lagi." >&2; fi
endef

# Tracing in DEV: overlay sends traces with a default 10% sampling rate (suited for prod). In dev traffic is low, so 10%
# leaves the Trace dashboard looking empty; this macro uses 100% (all traces) if OTEL_TRACES_SAMPLER_ARG is not
# set in .env or the shell. Explicitly set values are still respected. Prod does not use this macro (stays at 10%).
# `|| true` is REQUIRED on grep: recipes run with `-e -o pipefail`, so a non-matching grep (variable unset in .env)
# would silently abort make.
define obs_dev_sampling
r=$$(grep -E '^OTEL_TRACES_SAMPLER_ARG=.+' .env 2>/dev/null | tail -1 | cut -d= -f2- | tr -d '"' | tr -d "'" || true)
export OTEL_TRACES_SAMPLER_ARG="$${OTEL_TRACES_SAMPLER_ARG:-$${r:-1}}"
echo "Tracing aktif (dev): sampling $$OTEL_TRACES_SAMPLER_ARG (1 = semua trace; ubah lewat OTEL_TRACES_SAMPLER_ARG di .env)"
endef

# Dev: `make up` determines QMS_PUBLIC_ORIGIN from the LAN IP or tunnel URL. Recreating services with the default value
# (localhost) would alter CORS and email links, so the running backend's value is reused if available.
define obs_keep_origin
if [ "$(OBS_TARGET)" = dev ] && [ -z "$${QMS_PUBLIC_ORIGIN:-}" ]; then
  cid=$$(docker compose ps -q backend 2>/dev/null | head -1 || true)
  if [ -n "$$cid" ]; then
    o=$$(docker inspect -f '{{range .Config.Env}}{{println .}}{{end}}' "$$cid" | grep '^PUBLIC_APP_URL=' | cut -d= -f2- || true)
    [ -z "$$o" ] || export QMS_PUBLIC_ORIGIN="$$o"
  fi
fi
endef

# Print Grafana URL. Invoked via $(call obs_url,<env file>,<dev|prod>). Bound only to 127.0.0.1 on this machine;
# remote devices use an SSH tunnel. Defined as a recipe snippet (rather than `$(MAKE) obs-url`) so `make -n` does not
# execute the caller's recipe for real.
define obs_url
gp=$$(grep -E '^GRAFANA_PORT=' $(1) 2>/dev/null | tail -1 | cut -d= -f2 || true); gp=$${gp:-3000}
echo "Grafana ($(2), hanya dari mesin ini): http://localhost:$$gp  (user admin, password = GRAFANA_ADMIN_PASSWORD di $(1))"
echo "  dari perangkat lain: ssh -L $$gp:127.0.0.1:$$gp <user>@<IP mesin ini>, lalu buka http://localhost:$$gp"
echo "  log: Explore > Loki, contoh {service=\"backend\"} | json"
endef

# Marker target for `make up obs`, etc. Does nothing on its own; its effect lives in the `obs` variable above.
obs:
	:

# Start collector, Loki, Grafana. Application services are recreated if the overlay logging block is not yet attached
# (volumes and database remain intact). Aborts if GRAFANA_ADMIN_PASSWORD or METRICS_DB_PASSWORD is empty.
obs-up:
	[ -f $(OBS_ENV_FILE) ] || { echo "$(OBS_ENV_FILE) belum ada" >&2; exit 1; }
	for v in GRAFANA_ADMIN_PASSWORD METRICS_DB_PASSWORD; do grep -qE "^$$v=.+" $(OBS_ENV_FILE) || { echo "$$v kosong di $(OBS_ENV_FILE): isi dulu (tanpa nilai bawaan)" >&2; exit 1; }; done
	$(obs_keep_origin)
	$(if $(filter dev,$(OBS_TARGET)),$(obs_dev_sampling))
	$(OBS_COMPOSE) up -d
	$(call obs_db_role_run,$(OBS_COMPOSE),$(OBS_ENV_FILE))
	$(call obs_url,$(OBS_ENV_FILE),$(OBS_TARGET))

# Stop and remove observability containers only. Application services are untouched (isolation). Dummy values below are only
# so compose can parse the file; they do not affect container removal.
obs-down:
	GRAFANA_ADMIN_PASSWORD=$${GRAFANA_ADMIN_PASSWORD:-x} SMTP_HOST=$${SMTP_HOST:-x} SMTP_PORT=$${SMTP_PORT:-1} SMTP_FROM=$${SMTP_FROM:-x@x.y} MAIL_SERVICE_TOKEN=$${MAIL_SERVICE_TOKEN:-x} \
	$(OBS_COMPOSE) rm -sf collector loki grafana prometheus cadvisor node-exporter postgres-exporter nginx-exporter tempo
	echo "Observability dihentikan. Data (log, Grafana) tersimpan di volume obs_*; hapus dengan: docker volume rm <project>_obs_loki_data ..."

# Verify observability: (1) static compose and nginx isolation, (2) secret leak test into Loki, Tempo, Prometheus.
# The second test sends rejected requests containing fake secrets to the running stack; no data is persisted.
obs-check:
	scripts/check-service-isolation.sh
	QMS_ENV_FILE=$(OBS_ENV_FILE) scripts/check-observability-leaks.sh

# Display Grafana URL without running anything.
obs-url:
	$(call obs_url,$(OBS_ENV_FILE),$(OBS_TARGET))

# -----------------------------------------------------------------------------
# Cross-mode Mailpit link summary. Does not run anything.
# -----------------------------------------------------------------------------

# Display dev and prod Mailpit links, along with the active prod email mode.
mailpit:
	$(mailpit_dev)
	pmp=$$(grep -E '^MAILPIT_UI_PORT=' $(PROD_ENV) 2>/dev/null | tail -1 | cut -d= -f2 || true); pmp=$${pmp:-8025}
	echo "Prod (MAIL_MODE=$(MAIL_MODE)): $(if $(filter mailpit,$(MAIL_MODE)),Mailpit di server prod: http://localhost:$$pmp (hanya dari server itu; jauh: ssh -L $$pmp:127.0.0.1:$$pmp <user>@<server>),tanpa Mailpit: email dikirim lewat SMTP nyata)"
	# Switch prod mode: make prod-up MAIL_MODE=mailpit   or   make prod-up MAIL_MODE=smtp   (or set MAIL_MODE in $(PROD_ENV))
"""

with open("Makefile", "w", newline="\n") as f:
    f.write(makefile_content)

with open("Makefile.txt", "w", newline="\n") as f:
    f.write(makefile_content)

print("Done!")