# =============================================================================
# Makefile: perintah harian untuk stack QMS (Docker Compose + nginx).
# -----------------------------------------------------------------------------
# Dijalankan dari akar queue-management-system/. Dua stack:
#   dev   -> docker-compose.yml       (hot reload, kode di-mount)
#   prod  -> docker-compose.prod.yml  (demo/produksi, env dari .env.prod)
#
# Perintah DEV (membaca .env):
#   make up                         build + jalankan; alamat LAN http://<IP laptop>:8080 (HP satu Wi-Fi).
#                                   Login Google: buka http://localhost:8080 (bukan IP LAN), lihat catatan di bawah
#   make up IP=192.168.1.5          sama, tetapi memakai IP LAN yang ditentukan
#   make tunnel                     HTTPS publik lewat Cloudflare quick tunnel (kamera HP jalan)
#   make down                       hentikan stack dan tunnel
#   make restart                    restart backend, auth-service, qr-service, notify-service, mail-service
#                                   (dipakai setelah mengubah kode Go)
#   make seed                       reset database dev ke data seed
#   make logs                       ikuti log semua service (50 baris terakhir)
#   make ps                         status container
#   make mailpit                    tampilkan link Mailpit dev dan prod tanpa menjalankan apa pun
#   make verify-user EMAIL=a@b.com [STACK=prod]
#                                   tandai satu email terverifikasi tanpa menunggu email
#
# Perintah PROD (membaca .env.prod, salin dari .env.prod.example):
#   make prod-up [MAIL_MODE=smtp|mailpit]   jalankan stack prod (MAIL_MODE menimpa nilai di .env.prod)
#   make prod-up-mailpit                    singkatan untuk prod-up dengan MAIL_MODE=mailpit
#   make prod-up-smtp                       singkatan untuk prod-up dengan MAIL_MODE=smtp
#   make prod-down                          hentikan stack prod (termasuk container Mailpit)
#   make prod-guard                         tandai DB prod agar seed/reset ditolak (jalankan sekali)
#   make prod-backup                        backup DB prod ke ~/qms-backups/
#   make prod-reset                         RESET TOTAL database prod: semua akun dan data dihapus, mulai kosong (backup dulu)
#   make prod-seed-penguji                  buat/setel akun penguji dan pelanggan seed, kata sandi acak baru per akun, isi ulang data uji (backup dulu)
#
# Kata `obs` (opsional): make up obs, make tunnel obs, make prod-up obs ikut menyalakan observability (collector, Loki,
# Grafana) dan memasang pengumpulan log di service aplikasi, tanpa menjalankan make obs-up terpisah. Bentuk obs=1 juga
# diterima. Tanpa itu, stack berjalan seperti biasa tanpa observability. Butuh GRAFANA_ADMIN_PASSWORD di env file.
#
# Perintah OBSERVABILITY (opsional, overlay docker-compose.observability.yml; OBS_TARGET=dev|prod, bawaan dev):
#   make obs-up [OBS_TARGET=prod]           jalankan collector, Loki, Tempo, Prometheus, exporter, Grafana, dan pasang pengumpulan log dan trace di service
#                                           aplikasi (service aplikasi dibuat ulang sekali). Butuh GRAFANA_ADMIN_PASSWORD dan
#                                           METRICS_DB_PASSWORD di .env (dev) atau $(PROD_ENV) (prod). Role database
#                                           metrics_ro dibuat/diperbarui otomatis.
#   make obs-down [OBS_TARGET=prod]         hentikan dan hapus container observability; aplikasi tetap jalan.
#                                           Volume data (log, Grafana) dipertahankan.
#   make obs-url [OBS_TARGET=prod]          tampilkan alamat Grafana dan cara akses jauh (SSH tunnel)
#   make obs-check [OBS_TARGET=prod]        verifikasi: isolasi statis overlay (check-service-isolation.sh) dan tes
#                                           kebocoran rahasia ke log, trace, metrik (check-observability-leaks.sh;
#                                           butuh stack jalan dengan overlay, ~1,5 menit)
#
# Catatan:
#   - Kamera di browser butuh HTTPS atau localhost. Scan QR dari HP lewat IP LAN tidak jalan di web;
#     pakai `make tunnel`. Tunnel butuh cloudflared dan internet.
#   - URL tunnel acak tiap sesi. Backend, auth-service, dan mail-service membacanya saat start (QMS_PUBLIC_ORIGIN, untuk
#     CORS dan tautan di email).
#   - Login Google (opsional; kosong = tombol tersembunyi). Variabel: GOOGLE_CLIENT_IDS (server) dan
#     EXPO_PUBLIC_GOOGLE_{WEB,ANDROID,IOS}_CLIENT_ID (frontend) di .env / .env.prod. Setup: lihat scripts/README.md bagian check-google-config.sh
#       make up / tunnel / prod-up memeriksa konfigurasinya dulu (scripts/check-google-config.sh) dan berhenti bila tidak
#       sinkron (login pasti gagal). make up dan tunnel juga membersihkan cache Metro otomatis bila EXPO_PUBLIC_GOOGLE_* berubah.
#       Google hanya menerima origin https atau http://localhost: di dev pakai http://localhost:8080 (bukan IP LAN); di tunnel
#       URL acak harus didaftarkan di Google Cloud tiap sesi; di prod origin https harus sudah terdaftar.
#   - Mode email prod:
#       MAIL_MODE=mailpit  email ditangkap Mailpit dan TIDAK terkirim ke siapa pun (demo/uji). BAWAAN bila MAIL_MODE kosong.
#       MAIL_MODE=smtp     email dikirim lewat SMTP nyata (isi SMTP_* di .env.prod).
# =============================================================================

SHELL := bash
.SHELLFLAGS := -euo pipefail -c
# Recipe dijalankan dalam satu shell (variabel antar baris bisa dipakai).
.ONESHELL:
# Perintah tidak ditampilkan sebelum dijalankan. Pesan yang memang dicetak (echo) tetap tampil.
.SILENT:
.PHONY: obs up tunnel down restart seed logs ps mailpit verify-user \
        prod-up prod-up-mailpit prod-up-smtp prod-down prod-guard prod-backup prod-reset \
        prod-seed-penguji \
        obs-up obs-down obs-url obs-check

# -----------------------------------------------------------------------------
# Variabel dan potongan bersama
# -----------------------------------------------------------------------------

# Penanda nilai EXPO_PUBLIC_GOOGLE_* terakhir yang dibangun ke cache Metro (tidak ikut git). Lihat google_metro_cache.
GOOGLE_STAMP := .metro-google.stamp

# Menulis kata `obs` di baris perintah (make up obs) sama dengan obs=1: kata itu masuk daftar target (MAKECMDGOALS)
# dan target `obs` di bawah sengaja kosong. Karena itu jangan memakai `obs` sebagai nama file atau target lain.
ifneq ($(filter obs,$(MAKECMDGOALS)),)
obs := 1
endif

# obs=1 menambahkan overlay observability ke perintah compose DEV (up, tunnel). Kosong bila obs tidak diisi 1.
# OBS_SERVICES: service overlay yang ikut dinaikkan pada `make tunnel` (daftar service di sana eksplisit).
OBS_DEV_FILES := $(if $(filter 1,$(obs)),-f docker-compose.yml -f docker-compose.observability.yml)
OBS_SERVICES := $(if $(filter 1,$(obs)),collector loki grafana prometheus cadvisor node-exporter postgres-exporter nginx-exporter tempo)

# Dengan obs=1: menolak jalan bila GRAFANA_ADMIN_PASSWORD kosong di env file $(1), sebelum ada container yang dibuat ulang.
define obs_require_password
$(if $(filter 1,$(obs)),for v in GRAFANA_ADMIN_PASSWORD METRICS_DB_PASSWORD; do grep -qE "^$$v=.+" $(1) || { echo "obs: $$v kosong di $(1): isi dulu (tanpa nilai bawaan)" >&2; exit 1; }; done)
endef

# Kunci JWT dev di keys-dev/ dibuat ulang setiap `make up` / `make tunnel` dan dihapus `make down`.
# Kunci prod di keys/ tidak pernah disentuh target dev. rm -rf (bukan rm -f) juga membersihkan direktori salah-buat oleh Docker.
define fresh_auth_keys
rm -rf keys-dev && AUTH_KEYS_DIR=keys-dev scripts/gen-auth-keys.sh
endef

# File sementara untuk proses cloudflared (tunnel).
PIDFILE := /tmp/qms-tunnel.pid
LOG := /tmp/qms-tunnel.log

# Env file prod. Bisa diganti: make prod-up PROD_ENV=.env.lain
PROD_ENV ?= .env.prod

# Membaca QMS_HTTP_PORT dari .env (default 8080) ke variabel shell `port`.
# Dipakai oleh up, tunnel, dan warm_metro.
define need_env
[ -f .env ] || { echo ".env belum ada (cp .env.example .env lalu isi secret)" >&2; exit 1; }
port=$$(grep -E '^QMS_HTTP_PORT=' .env | cut -d= -f2 || true); port=$${port:-8080}
endef

# Menghentikan cloudflared yang tercatat di PIDFILE, lalu membersihkan file PID.
define stop_tunnel
[ -f $(PIDFILE) ] && kill "$$(cat $(PIDFILE))" 2>/dev/null || true; rm -f $(PIDFILE)
endef

# Mencetak link Mailpit dev. UI hanya diikat ke 127.0.0.1 mesin ini: tidak lewat nginx, LAN, atau tunnel.
# Perangkat lain harus memakai SSH tunnel.
define mailpit_dev
mp=$$(grep -E '^MAILPIT_UI_PORT=' .env 2>/dev/null | tail -1 | cut -d= -f2 || true); mp=$${mp:-8025}
echo "Mailpit (email dev, hanya dari mesin ini): http://localhost:$$mp"
echo "  dari perangkat lain: ssh -L $$mp:127.0.0.1:$$mp <user>@<IP mesin ini>, lalu buka http://localhost:$$mp"
endef

# Metro menyimpan nilai EXPO_PUBLIC_* di cache transformasi: mengubahnya di .env tanpa membersihkan cache membuat bundle
# tetap memakai nilai LAMA (tombol Google tidak muncul atau memakai client ID lama). Makro ini membandingkan nilai
# EXPO_PUBLIC_GOOGLE_* di .env dengan penanda terakhir, dan bila berbeda menghapus volume metro_cache (container frontend
# dihentikan dulu karena volume sedang dipakai). Pertama kali dengan nilai terisi juga membersihkan, sekali saja.
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

# Petunjuk login Google setelah stack DEV naik. Hanya bila client ID Web terisi.
define google_hint_dev
if grep -qE '^EXPO_PUBLIC_GOOGLE_WEB_CLIENT_ID=.+' .env 2>/dev/null; then
  echo "Login Google: buka http://localhost:$$port (BUKAN alamat IP LAN). Google dan WebCrypto hanya menerima https atau localhost."
  echo "  Pastikan http://localhost:$$port dan http://localhost:$$port/ terdaftar di Authorized JavaScript origins dan redirect URIs."
fi
endef

# Memanaskan Metro (dev) agar kompilasi selesai sebelum browser membuka link (cache kosong ±30–40 detik).
# Kegagalan pemanasan hanya memberi peringatan; tidak menggagalkan make.
define warm_metro
echo "Memanaskan Metro (cache kosong bisa ±30–40 detik)..."
curl -fsS -o /dev/null --retry 30 --retry-delay 2 --retry-all-errors "http://localhost:$$port/" || echo "Pemanasan halaman gagal; muat pertama bisa lambat." >&2
curl -fsS -o /dev/null --max-time 300 --retry 5 --retry-delay 2 --retry-all-errors "http://localhost:$$port/node_modules/expo-router/entry.bundle?platform=web&dev=true&hot=false&lazy=true&transform.routerRoot=src%2Fapp&transform.reactCompiler=true" || echo "Pemanasan bundle gagal." >&2
endef

# -----------------------------------------------------------------------------
# Stack DEV
# -----------------------------------------------------------------------------

# --renew-anon-volumes: volume anonim /app/node_modules diisi ulang dari image tiap `up`. Tanpa ini, volume lama menimpa
# node_modules image dan paket baru di package.json tidak terpasang (Metro: "Unable to resolve module").
# Build dan jalankan stack dev. IP LAN dibaca otomatis dari rute default, atau dari IP=... bila diberikan.
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

# Stack dev dengan HTTPS publik lewat Cloudflare quick tunnel. URL diambil dari log cloudflared.
# Service yang di-recreate membawa QMS_PUBLIC_ORIGIN = URL tunnel (daftar service ada di perintah compose).
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
	QMS_PUBLIC_ORIGIN="$$url" docker compose $(OBS_DEV_FILES) up -d --build --force-recreate --renew-anon-volumes backend auth-service qr-service notify-service mail-service mailpit nginx frontend db $(OBS_SERVICES)
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

# Hentikan tunnel (bila ada) dan semua container dev.
down:
	$(stop_tunnel)
	# --remove-orphans: ikut membersihkan container observability (collector, loki, grafana) bila overlay pernah dinyalakan.
	docker compose --profile '*' down --remove-orphans
	rm -f $(GOOGLE_STAMP)
	rm -rf keys-dev

# Restart service yang perlu dimuat ulang setelah kode Go berubah.
restart:
	docker compose restart backend auth-service qr-service notify-service mail-service

# Reset database dev ke data seed (db/scripts/reset_seed_dev.sql).
seed:
	docker compose exec -T db psql -U qms -d qms_db < db/scripts/reset_seed_dev.sql

# Ikuti log semua service, 50 baris terakhir.
logs:
	docker compose logs -f --tail 50

# Status container dev.
ps:
	docker compose ps

# Tandai satu email terverifikasi langsung di database. Berguna saat MAIL_MODE=mailpit atau email tidak sampai.
# STACK=prod memakai database prod (env dari $(PROD_ENV)).
verify-user:
	[ -n "$(EMAIL)" ] || { echo "pakai: make verify-user EMAIL=alamat@contoh.com [STACK=prod]" >&2; exit 1; }
	QMS_ENV_FILE=$(PROD_ENV) scripts/verify-user.sh "$(EMAIL)" $(if $(filter prod,$(STACK)),prod)

# -----------------------------------------------------------------------------
# Stack PROD (docker-compose.prod.yml, env dari $(PROD_ENV))
# -----------------------------------------------------------------------------

# MAIL_MODE dibaca dari $(PROD_ENV). Nilai di baris perintah (make prod-up MAIL_MODE=...) menimpa file.
# Kosong di file = mailpit (bawaan): email TIDAK dikirim ke penerima sebenarnya. Pakai smtp untuk SMTP nyata.
MAIL_MODE := $(or $(strip $(shell grep -E '^MAIL_MODE=' $(PROD_ENV) 2>/dev/null | tail -1 | cut -d= -f2 | tr -d '"' | tr -d "'")),mailpit)

# Mode mailpit: profile `mailpit` aktif dan SMTP_* dipaksa ke Mailpit. Variabel `export` ini menimpa .env.prod
# untuk setiap perintah di Makefile ini.
ifeq ($(MAIL_MODE),mailpit)
export SMTP_HOST := mailpit
export SMTP_PORT := 1025
export SMTP_TLS := none
export SMTP_USER :=
export SMTP_PASSWORD :=
export SMTP_FROM := Antrian <no-reply@qms.local>
PROD_PROFILE := --profile mailpit
endif

# Perintah compose prod lengkap (file, env, dan profile).
# obs=1 menambahkan overlay observability; OBS_PROJECT memberi tahu collector project mana yang dibaca.
PROD := $(if $(filter 1,$(obs)),OBS_PROJECT=qms-prod )docker compose -f docker-compose.prod.yml $(if $(filter 1,$(obs)),-f docker-compose.observability.yml )--env-file $(PROD_ENV) $(PROD_PROFILE)

# Jalankan stack prod. Memeriksa MAIL_MODE dulu (scripts/check-mail-mode.sh) dan menolak konfigurasi salah.
# Bila berpindah dari mailpit ke smtp, sisa container Mailpit dihentikan supaya tidak membingungkan.
prod-up:
	scripts/gen-auth-keys.sh
	MAIL_MODE=$(MAIL_MODE) QMS_ENV_FILE=$(PROD_ENV) scripts/check-mail-mode.sh
	scripts/check-google-config.sh $(PROD_ENV)
	$(call obs_require_password,$(PROD_ENV))
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

# Pintasan: prod-up dengan MAIL_MODE=mailpit.
prod-up-mailpit:
	$(MAKE) prod-up MAIL_MODE=mailpit

# Pintasan: prod-up dengan MAIL_MODE=smtp.
prod-up-smtp:
	$(MAKE) prod-up MAIL_MODE=smtp

# Hentikan stack prod. Memakai --profile mailpit agar container Mailpit ikut berhenti walau mode saat ini smtp.
# Nilai SMTP palsu hanya supaya compose bisa menguraikan file; tidak memengaruhi `down`.
prod-down:
	SMTP_HOST=$${SMTP_HOST:-x} SMTP_PORT=$${SMTP_PORT:-1} SMTP_FROM=$${SMTP_FROM:-x@x.y} MAIL_SERVICE_TOKEN=$${MAIL_SERVICE_TOKEN:-x} \
	docker compose -f docker-compose.prod.yml --env-file $(PROD_ENV) --profile '*' down --remove-orphans

# Tandai DB prod agar seed/reset ditolak. Jalankan sekali setelah akun demo dibuat.
prod-guard:
	$(PROD) exec -T db psql -U qms -d qms_db -c "CREATE TABLE IF NOT EXISTS _no_seed_guard (x int)"

# Backup DB prod ke ~/qms-backups/ (scripts/backup-prod.sh).
prod-backup:
	QMS_ENV_FILE=$(PROD_ENV) scripts/backup-prod.sh backup

# RESET TOTAL database prod: hapus volume DB, nyalakan ulang stack dengan DB kosong. Semua akun ikut hilang. Backup dulu (scripts/reset-demo.sh).
prod-reset:
	QMS_ENV_FILE=$(PROD_ENV) scripts/reset-demo.sh

# Akun penguji (owner + customer) dan pelanggan seed: kata sandi acak baru per akun (ditulis ke ~/qms-penguji-account.txt),
# dan mengisi ulang data uji antrian ke kondisi seed. Dipakai juga untuk mengembalikan data uji (scripts/seed-prod-penguji.sh).
prod-seed-penguji:
	QMS_ENV_FILE=$(PROD_ENV) scripts/seed-prod-penguji.sh

# -----------------------------------------------------------------------------
# OBSERVABILITY (overlay docker-compose.observability.yml, lihat berkas itu dan bagian Observability di README.md)
# -----------------------------------------------------------------------------

# Target overlay: dev (bawaan) atau prod. Menentukan berkas compose, env file, dan nama project yang dibaca collector.
OBS_TARGET ?= dev
ifeq ($(OBS_TARGET),prod)
OBS_ENV_FILE := $(PROD_ENV)
export OBS_PROJECT := qms-prod
# `=` (bukan :=) agar PROD_PROFILE yang ditentukan di atas ikut terbaca.
OBS_COMPOSE = docker compose -f docker-compose.prod.yml -f docker-compose.observability.yml --env-file $(PROD_ENV) $(PROD_PROFILE)
else
OBS_ENV_FILE := .env
export OBS_PROJECT := qms
OBS_COMPOSE = docker compose -f docker-compose.yml -f docker-compose.observability.yml
endif

# Membuat atau memperbarui role database metrics_ro (postgres_exporter) lewat db/migrations/07_metrics_role.sh, yang
# idempoten. Dipanggil dengan $(call obs_db_role_run,<perintah compose>,<env file>). Mencoba sampai 30 detik karena
# database mungkin baru dinyalakan. Gagal hanya memberi peringatan: aplikasi tidak bergantung pada role ini.
define obs_db_role_run
pw=$$(grep -E '^METRICS_DB_PASSWORD=' $(2) | tail -1 | cut -d= -f2- | tr -d '"' | tr -d "'" || true)
ok=
for _ in $$(seq 1 30); do
  if $(1) exec -T -e METRICS_DB_PASSWORD="$$pw" db sh < db/migrations/07_metrics_role.sh >/dev/null 2>&1; then ok=1; break; fi
  sleep 1
done
if [ -n "$$ok" ]; then echo "Role database metrics_ro siap."; else echo "Peringatan: role metrics_ro belum dibuat (database belum siap?). Jalankan make obs-up lagi." >&2; fi
endef

# Tracing di DEV: overlay mengirim trace dengan sampling bawaan 10% (cocok untuk prod). Di dev trafik kecil, jadi 10%
# membuat dashboard Trace tampak kosong; makro ini memakai 100% (semua trace) bila OTEL_TRACES_SAMPLER_ARG tidak
# diisi di .env maupun di shell. Nilai yang diisi eksplisit tetap dihormati. Prod tidak memakai makro ini (tetap 10%).
# `|| true` WAJIB pada grep: recipe berjalan dengan `-e -o pipefail`, jadi grep tanpa hasil (variabel belum ada di .env) akan
# menghentikan make secara diam-diam.
define obs_dev_sampling
r=$$(grep -E '^OTEL_TRACES_SAMPLER_ARG=.+' .env 2>/dev/null | tail -1 | cut -d= -f2- | tr -d '"' | tr -d "'" || true)
export OTEL_TRACES_SAMPLER_ARG="$${OTEL_TRACES_SAMPLER_ARG:-$${r:-1}}"
echo "Tracing aktif (dev): sampling $$OTEL_TRACES_SAMPLER_ARG (1 = semua trace; ubah lewat OTEL_TRACES_SAMPLER_ARG di .env)"
endef

# Dev: `make up` menentukan QMS_PUBLIC_ORIGIN dari IP LAN atau URL tunnel. Membuat ulang service dengan nilai bawaan
# (localhost) akan mengubah CORS dan tautan email, jadi nilai backend yang sedang jalan dipakai ulang bila ada.
define obs_keep_origin
if [ "$(OBS_TARGET)" = dev ] && [ -z "$${QMS_PUBLIC_ORIGIN:-}" ]; then
  cid=$$(docker compose ps -q backend 2>/dev/null | head -1 || true)
  if [ -n "$$cid" ]; then
    o=$$(docker inspect -f '{{range .Config.Env}}{{println .}}{{end}}' "$$cid" | grep '^PUBLIC_APP_URL=' | cut -d= -f2- || true)
    [ -z "$$o" ] || export QMS_PUBLIC_ORIGIN="$$o"
  fi
fi
endef

# Mencetak alamat Grafana. Dipanggil dengan $(call obs_url,<env file>,<dev|prod>). Hanya diikat ke 127.0.0.1 mesin ini;
# perangkat lain memakai SSH tunnel. Berupa potongan resep (bukan `$(MAKE) obs-url`) supaya `make -n` tidak
# menjalankan resep pemanggilnya sungguhan.
define obs_url
gp=$$(grep -E '^GRAFANA_PORT=' $(1) 2>/dev/null | tail -1 | cut -d= -f2 || true); gp=$${gp:-3000}
echo "Grafana ($(2), hanya dari mesin ini): http://localhost:$$gp  (user admin, password = GRAFANA_ADMIN_PASSWORD di $(1))"
echo "  dari perangkat lain: ssh -L $$gp:127.0.0.1:$$gp <user>@<IP mesin ini>, lalu buka http://localhost:$$gp"
echo "  log: Explore > Loki, contoh {service=\"backend\"} | json"
endef

# Penanda untuk `make up obs` dkk. Tidak melakukan apa pun sendiri; efeknya ada di variabel obs di atas.
obs:
	:

# Jalankan collector, Loki, Grafana. Service aplikasi dibuat ulang bila blok logging overlay belum terpasang di sana
# (volume dan database tetap). Menolak jalan bila GRAFANA_ADMIN_PASSWORD kosong.
obs-up:
	[ -f $(OBS_ENV_FILE) ] || { echo "$(OBS_ENV_FILE) belum ada" >&2; exit 1; }
	for v in GRAFANA_ADMIN_PASSWORD METRICS_DB_PASSWORD; do grep -qE "^$$v=.+" $(OBS_ENV_FILE) || { echo "$$v kosong di $(OBS_ENV_FILE): isi dulu (tanpa nilai bawaan)" >&2; exit 1; }; done
	$(obs_keep_origin)
	$(if $(filter dev,$(OBS_TARGET)),$(obs_dev_sampling))
	$(OBS_COMPOSE) up -d
	$(call obs_db_role_run,$(OBS_COMPOSE),$(OBS_ENV_FILE))
	$(call obs_url,$(OBS_ENV_FILE),$(OBS_TARGET))

# Hentikan dan hapus container observability saja. Service aplikasi tidak disentuh (isolasi). Nilai palsu di bawah hanya
# supaya compose bisa menguraikan file; tidak memengaruhi penghapusan.
obs-down:
	GRAFANA_ADMIN_PASSWORD=$${GRAFANA_ADMIN_PASSWORD:-x} SMTP_HOST=$${SMTP_HOST:-x} SMTP_PORT=$${SMTP_PORT:-1} SMTP_FROM=$${SMTP_FROM:-x@x.y} MAIL_SERVICE_TOKEN=$${MAIL_SERVICE_TOKEN:-x} \
	$(OBS_COMPOSE) rm -sf collector loki grafana prometheus cadvisor node-exporter postgres-exporter nginx-exporter tempo
	echo "Observability dihentikan. Data (log, Grafana) tersimpan di volume obs_*; hapus dengan: docker volume rm <project>_obs_loki_data ..."

# Verifikasi observability: (1) isolasi statis compose dan nginx, (2) tes kebocoran rahasia ke Loki, Tempo, Prometheus.
# Tes kedua mengirim request ditolak berisi rahasia palsu ke stack yang sedang jalan; tidak membuat data apa pun.
obs-check:
	scripts/check-service-isolation.sh
	QMS_ENV_FILE=$(OBS_ENV_FILE) scripts/check-observability-leaks.sh

# Tampilkan alamat Grafana tanpa menjalankan apa pun.
obs-url:
	$(call obs_url,$(OBS_ENV_FILE),$(OBS_TARGET))

# -----------------------------------------------------------------------------
# Ringkasan link Mailpit lintas mode. Tidak menjalankan apa pun.
# -----------------------------------------------------------------------------

# Menampilkan link Mailpit dev dan prod, serta mode email prod yang sedang aktif.
mailpit:
	$(mailpit_dev)
	pmp=$$(grep -E '^MAILPIT_UI_PORT=' $(PROD_ENV) 2>/dev/null | tail -1 | cut -d= -f2 || true); pmp=$${pmp:-8025}
	echo "Prod (MAIL_MODE=$(MAIL_MODE)): $(if $(filter mailpit,$(MAIL_MODE)),Mailpit di server prod: http://localhost:$$pmp (hanya dari server itu; jauh: ssh -L $$pmp:127.0.0.1:$$pmp <user>@<server>),tanpa Mailpit: email dikirim lewat SMTP nyata)"
	# Ubah mode prod: make prod-up MAIL_MODE=mailpit   atau   make prod-up MAIL_MODE=smtp   (atau set MAIL_MODE di $(PROD_ENV))
