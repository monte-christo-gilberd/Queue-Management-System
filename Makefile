# =============================================================================
# Makefile: perintah harian untuk stack QMS (Docker Compose + nginx).
# -----------------------------------------------------------------------------
# Dijalankan dari akar queue-management-system/. Dua stack:
#   dev   -> docker-compose.yml       (hot reload, kode di-mount)
#   prod  -> docker-compose.prod.yml  (demo/produksi, env dari .env.prod)
#
# Perintah DEV (membaca .env):
#   make up                         build + jalankan; alamat LAN http://<IP laptop>:8080 (HP satu Wi-Fi)
#   make up IP=192.168.1.5          sama, tetapi memakai IP LAN yang ditentukan
#   make tunnel                     HTTPS publik lewat Cloudflare quick tunnel (kamera HP jalan)
#   make down                       hentikan stack dan tunnel
#   make restart                    restart backend, qr-service, notify-service, mail-service, payment-service
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
#   make prod-reset                         kosongkan antrian dan tiket demo (akun dipertahankan, backup dulu)
#
# Catatan:
#   - Kamera di browser butuh HTTPS atau localhost. Scan QR dari HP lewat IP LAN tidak jalan di web;
#     pakai `make tunnel`. Tunnel butuh cloudflared dan internet.
#   - URL tunnel acak tiap sesi. Backend dan qr-service membacanya saat start (QMS_PUBLIC_ORIGIN).
#   - Mode email prod:
#       MAIL_MODE=smtp     email dikirim lewat SMTP nyata (bawaan).
#       MAIL_MODE=mailpit  email ditangkap Mailpit dan TIDAK terkirim (demo/uji).
# =============================================================================

SHELL := bash
.SHELLFLAGS := -euo pipefail -c
# Recipe dijalankan dalam satu shell (variabel antar baris bisa dipakai).
.ONESHELL:
# Perintah tidak ditampilkan sebelum dijalankan. Pesan yang memang dicetak (echo) tetap tampil.
.SILENT:
.PHONY: up tunnel down restart seed logs ps mailpit verify-user \
        prod-up prod-up-mailpit prod-up-smtp prod-down prod-guard prod-backup prod-reset

# -----------------------------------------------------------------------------
# Variabel dan potongan bersama
# -----------------------------------------------------------------------------

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

# Build dan jalankan stack dev. IP LAN dibaca otomatis dari rute default, atau dari IP=... bila diberikan.
up:
	$(need_env)
	ip="$(IP)"
	[ -n "$$ip" ] || ip=$$(ip route get 1.1.1.1 2>/dev/null | grep -oP 'src \K\S+' || true)
	[ -n "$$ip" ] || { echo "IP LAN tidak terdeteksi; beri manual: make up IP=<ip>" >&2; exit 1; }
	export QMS_PUBLIC_ORIGIN="http://$$ip:$$port"
	docker compose up -d --build --force-recreate
	$(warm_metro)
	echo "Link (satu Wi-Fi): $$QMS_PUBLIC_ORIGIN"
	$(mailpit_dev)

# Stack dev dengan HTTPS publik lewat Cloudflare quick tunnel. URL diambil dari log cloudflared.
# Service yang di-recreate membawa QMS_PUBLIC_ORIGIN = URL tunnel (daftar service ada di perintah compose).
tunnel:
	command -v cloudflared >/dev/null || { echo "cloudflared belum terpasang" >&2; exit 1; }
	$(need_env)
	$(stop_tunnel); rm -f $(LOG)
	cloudflared tunnel --config /dev/null --protocol http2 --no-autoupdate --url "http://localhost:$$port" >$(LOG) 2>&1 &
	echo $$! > $(PIDFILE)
	url=""
	for _ in $$(seq 1 40); do
	  url=$$(grep -aoE 'https://[a-z0-9-]+\.trycloudflare\.com' $(LOG) | head -1 || true)
	  [ -n "$$url" ] && break; sleep 1
	done
	[ -n "$$url" ] || { echo "URL tunnel tidak muncul, lihat $(LOG)" >&2; $(stop_tunnel); exit 1; }
	QMS_PUBLIC_ORIGIN="$$url" docker compose up -d --build --force-recreate backend qr-service notify-service mail-service mailpit nginx frontend db
	$(warm_metro)
	echo "Link: $$url"
	echo "Hentikan: make down"
	$(mailpit_dev)
	echo "  (Mailpit tidak ikut lewat tunnel Cloudflare; email dari aplikasi tetap tertangkap di sini)"

# Hentikan tunnel (bila ada) dan semua container dev.
down:
	$(stop_tunnel)
	docker compose down

# Restart service yang perlu dimuat ulang setelah kode Go berubah.
restart:
	docker compose restart backend qr-service notify-service mail-service payment-service

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
MAIL_MODE := $(shell grep -E '^MAIL_MODE=' $(PROD_ENV) 2>/dev/null | tail -1 | cut -d= -f2 | tr -d '"' | tr -d "'")

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
PROD := docker compose -f docker-compose.prod.yml --env-file $(PROD_ENV) $(PROD_PROFILE)

# Jalankan stack prod. Memeriksa MAIL_MODE dulu (scripts/check-mail-mode.sh) dan menolak konfigurasi salah.
# Bila berpindah dari mailpit ke smtp, sisa container Mailpit dihentikan supaya tidak membingungkan.
prod-up:
	MAIL_MODE=$(or $(MAIL_MODE),smtp) QMS_ENV_FILE=$(PROD_ENV) scripts/check-mail-mode.sh
	$(PROD) up -d --build
	$(if $(PROD_PROFILE),,docker compose -f docker-compose.prod.yml --env-file $(PROD_ENV) --profile mailpit rm -sf mailpit >/dev/null 2>&1 || true)
	pmp=$$(grep -E '^MAILPIT_UI_PORT=' $(PROD_ENV) 2>/dev/null | tail -1 | cut -d= -f2 || true); pmp=$${pmp:-8025}
	if [ "$(MAIL_MODE)" = mailpit ]; then
	  echo "Mailpit (prod, MAIL_MODE=mailpit): http://localhost:$$pmp  (hanya dari server ini; email TIDAK terkirim ke penerima sebenarnya)"
	  echo "  dari jauh: ssh -L $$pmp:127.0.0.1:$$pmp <user>@<server>, lalu buka http://localhost:$$pmp"
	else
	  echo "Email prod: SMTP nyata (MAIL_MODE=smtp), tanpa Mailpit. Lihat hasil kirim di inbox penerima atau dashboard penyedia SMTP."
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
	docker compose -f docker-compose.prod.yml --env-file $(PROD_ENV) --profile mailpit down

# Tandai DB prod agar seed/reset ditolak. Jalankan sekali setelah akun demo dibuat.
prod-guard:
	$(PROD) exec -T db psql -U qms -d qms_db -c "CREATE TABLE IF NOT EXISTS _no_seed_guard (x int)"

# Backup DB prod ke ~/qms-backups/ (scripts/backup-prod.sh).
prod-backup:
	QMS_ENV_FILE=$(PROD_ENV) scripts/backup-prod.sh backup

# Reset data demo: hapus antrian, tiket, dan push. Akun dipertahankan. Backup otomatis dibuat dulu (scripts/reset-demo.sh).
prod-reset:
	QMS_ENV_FILE=$(PROD_ENV) scripts/reset-demo.sh

# -----------------------------------------------------------------------------
# Ringkasan link Mailpit lintas mode. Tidak menjalankan apa pun.
# -----------------------------------------------------------------------------

# Menampilkan link Mailpit dev dan prod, serta mode email prod yang sedang aktif.
mailpit:
	$(mailpit_dev)
	pmp=$$(grep -E '^MAILPIT_UI_PORT=' $(PROD_ENV) 2>/dev/null | tail -1 | cut -d= -f2 || true); pmp=$${pmp:-8025}
	echo "Prod (MAIL_MODE=$(or $(MAIL_MODE),smtp)): $(if $(filter mailpit,$(MAIL_MODE)),Mailpit di server prod: http://localhost:$$pmp (hanya dari server itu; jauh: ssh -L $$pmp:127.0.0.1:$$pmp <user>@<server>),tanpa Mailpit: email dikirim lewat SMTP nyata)"
	# Ubah mode prod: make prod-up MAIL_MODE=mailpit   atau   make prod-up MAIL_MODE=smtp   (atau set MAIL_MODE di $(PROD_ENV))
