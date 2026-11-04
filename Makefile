# Perintah harian untuk stack dev (Docker Compose + nginx).
#   make up              # build + jalankan, alamat LAN: http://<IP laptop>:8080 (HP satu Wi-Fi)
#   make up IP=192.168.1.5   # mode LAN dengan IP tertentu
#   make tunnel          # HTTPS publik via Cloudflare quick tunnel (kamera HP jalan)
#   make down            # hentikan stack dan tunnel
#   make mailpit         # tampilkan link Mailpit (dev, tunnel, prod) tanpa menjalankan apa pun
#   make prod-up [MAIL_MODE=smtp|mailpit]   # stack prod; MAIL_MODE menimpa nilai di .env.prod
#   make prod-up-mailpit | prod-up-smtp     # singkatan untuk dua mode email prod
#   make restart         # restart backend, qr-service, notify-service setelah ubah kode Go
#   make seed            # reset database ke data seed dev
# Kamera di browser butuh HTTPS atau localhost, jadi scan QR dari HP lewat IP LAN
# tidak jalan di web; pakai `make tunnel`. Tunnel butuh cloudflared dan internet.
# URL tunnel acak tiap sesi dan dibaca backend/qr-service saat start (QMS_PUBLIC_ORIGIN).
SHELL := bash
.SHELLFLAGS := -euo pipefail -c
.ONESHELL:
.PHONY: up tunnel down restart seed logs ps mailpit prod-up prod-up-mailpit prod-up-smtp prod-down prod-guard prod-backup prod-reset

PIDFILE := /tmp/qms-tunnel.pid
LOG := /tmp/qms-tunnel.log

define need_env
[ -f .env ] || { echo ".env belum ada (cp .env.example .env lalu isi secret)" >&2; exit 1; }
port=$$(grep -E '^QMS_HTTP_PORT=' .env | cut -d= -f2 || true); port=$${port:-8080}
endef

define stop_tunnel
[ -f $(PIDFILE) ] && kill "$$(cat $(PIDFILE))" 2>/dev/null || true; rm -f $(PIDFILE)
endef

# Memanaskan Metro (dev) agar kompilasi selesai sebelum browser membuka link. Gagal pemanasan tidak menggagalkan make.
# Link Mailpit dev. UI hanya diikat ke 127.0.0.1 mesin ini dan tidak lewat nginx, jadi TIDAK ikut terbuka lewat
# IP LAN maupun tunnel Cloudflare. Perangkat lain memakai SSH tunnel.
define mailpit_dev
mp=$$(grep -E '^MAILPIT_UI_PORT=' .env 2>/dev/null | tail -1 | cut -d= -f2 || true); mp=$${mp:-8025}
echo "Mailpit (email dev, hanya dari mesin ini): http://localhost:$$mp"
echo "  dari perangkat lain: ssh -L $$mp:127.0.0.1:$$mp <user>@<IP mesin ini>, lalu buka http://localhost:$$mp"
endef

define warm_metro
echo "Memanaskan Metro (cache kosong bisa ±30–40 detik)..."
curl -fsS -o /dev/null --retry 30 --retry-delay 2 --retry-all-errors "http://localhost:$$port/" || echo "Pemanasan halaman gagal; muat pertama bisa lambat." >&2
curl -fsS -o /dev/null --max-time 300 --retry 5 --retry-delay 2 --retry-all-errors "http://localhost:$$port/node_modules/expo-router/entry.bundle?platform=web&dev=true&hot=false&lazy=true&transform.routerRoot=src%2Fapp&transform.reactCompiler=true" || echo "Pemanasan bundle gagal." >&2
endef

up:
	$(need_env)
	ip="$(IP)"
	[ -n "$$ip" ] || ip=$$(ip route get 1.1.1.1 2>/dev/null | grep -oP 'src \K\S+' || true)
	[ -n "$$ip" ] || { echo "IP LAN tidak terdeteksi; beri manual: make up IP=<ip>" >&2; exit 1; }
	export QMS_PUBLIC_ORIGIN="http://$$ip:$$port"
	docker compose up -d --build --force-recreate
	$(warm_metro)
	echo; echo "Link (satu Wi-Fi): $$QMS_PUBLIC_ORIGIN"
	$(mailpit_dev)

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
	echo; echo "Link: $$url"; echo "Hentikan: make down"
	$(mailpit_dev)
	echo "  (Mailpit tidak ikut lewat tunnel Cloudflare; email dari aplikasi tetap tertangkap di sini)"

down:
	$(stop_tunnel)
	docker compose down

# Tampilkan link Mailpit untuk semua mode tanpa menjalankan apa pun.
mailpit:
	@$(mailpit_dev)
	pmp=$$(grep -E '^MAILPIT_UI_PORT=' $(PROD_ENV) 2>/dev/null | tail -1 | cut -d= -f2 || true); pmp=$${pmp:-8025}
	echo "Prod (MAIL_MODE=$(or $(MAIL_MODE),smtp)): $(if $(filter mailpit,$(MAIL_MODE)),Mailpit di server prod: http://localhost:$$pmp (hanya dari server itu; jauh: ssh -L $$pmp:127.0.0.1:$$pmp <user>@<server>),tanpa Mailpit: email dikirim lewat SMTP nyata)"
	echo "Ubah mode prod: make prod-up MAIL_MODE=mailpit  atau  MAIL_MODE=smtp  (atau set MAIL_MODE di $(PROD_ENV))"

restart:
	docker compose restart backend qr-service notify-service mail-service

seed:
	docker compose exec -T db psql -U qms -d qms_db < db/scripts/reset_seed_dev.sql

logs:
	docker compose logs -f --tail 50

ps:
	docker compose ps

# --- Stack demo/produksi (docker-compose.prod.yml, env dari .env.prod; salin dari .env.prod.example) ---
PROD_ENV ?= .env.prod
# MAIL_MODE di .env.prod: smtp (bawaan, SMTP nyata) atau mailpit (email ditangkap Mailpit, tidak terkirim).
# Mode mailpit menyalakan profile `mailpit` dan memaksa SMTP_* ke Mailpit (env shell menimpa .env.prod).
MAIL_MODE := $(shell grep -E '^MAIL_MODE=' $(PROD_ENV) 2>/dev/null | tail -1 | cut -d= -f2 | tr -d '"' | tr -d "'")
ifeq ($(MAIL_MODE),mailpit)
export SMTP_HOST := mailpit
export SMTP_PORT := 1025
export SMTP_TLS := none
export SMTP_USER :=
export SMTP_PASSWORD :=
export SMTP_FROM := Antrian <no-reply@qms.local>
PROD_PROFILE := --profile mailpit
endif
PROD := docker compose -f docker-compose.prod.yml --env-file $(PROD_ENV) $(PROD_PROFILE)

# MAIL_MODE=mailpit: email ditangkap Mailpit dan TIDAK terkirim (demo/uji). MAIL_MODE=smtp: SMTP nyata.
# Nilai di baris perintah (make prod-up MAIL_MODE=mailpit) menimpa .env.prod untuk satu kali jalan.
prod-up:
	MAIL_MODE=$(or $(MAIL_MODE),smtp) QMS_ENV_FILE=$(PROD_ENV) scripts/check-mail-mode.sh
	$(PROD) up -d --build
	# Pindah dari mailpit ke smtp: hentikan sisa container Mailpit supaya tidak membingungkan.
	$(if $(PROD_PROFILE),,docker compose -f docker-compose.prod.yml --env-file $(PROD_ENV) --profile mailpit rm -sf mailpit >/dev/null 2>&1 || true)
	echo
	pmp=$$(grep -E '^MAILPIT_UI_PORT=' $(PROD_ENV) 2>/dev/null | tail -1 | cut -d= -f2 || true); pmp=$${pmp:-8025}
	if [ "$(MAIL_MODE)" = mailpit ]; then
	  echo "Mailpit (prod, MAIL_MODE=mailpit): http://localhost:$$pmp  (hanya dari server ini; email TIDAK terkirim ke penerima sebenarnya)"
	  echo "  dari jauh: ssh -L $$pmp:127.0.0.1:$$pmp <user>@<server>, lalu buka http://localhost:$$pmp"
	else
	  echo "Email prod: SMTP nyata (MAIL_MODE=smtp), tanpa Mailpit. Lihat hasil kirim di inbox penerima atau dashboard penyedia SMTP."
	fi

prod-up-mailpit:
	$(MAKE) prod-up MAIL_MODE=mailpit

prod-up-smtp:
	$(MAKE) prod-up MAIL_MODE=smtp

# --profile mailpit agar container Mailpit ikut berhenti walau mode saat ini smtp. Nilai SMTP palsu hanya
# supaya compose bisa menguraikan file; tidak memengaruhi `down`.
prod-down:
	SMTP_HOST=$${SMTP_HOST:-x} SMTP_PORT=$${SMTP_PORT:-1} SMTP_FROM=$${SMTP_FROM:-x@x.y} MAIL_SERVICE_TOKEN=$${MAIL_SERVICE_TOKEN:-x} \
	docker compose -f docker-compose.prod.yml --env-file $(PROD_ENV) --profile mailpit down

# Tandai DB prod agar seed/reset ditolak. Jalankan sekali setelah akun demo dibuat.
prod-guard:
	$(PROD) exec -T db psql -U qms -d qms_db -c "CREATE TABLE IF NOT EXISTS _no_seed_guard (x int)"

prod-backup:
	QMS_ENV_FILE=$(PROD_ENV) scripts/backup-prod.sh backup

# Reset data demo: hapus antrian/tiket/push, akun dipertahankan (backup otomatis dulu).
prod-reset:
	QMS_ENV_FILE=$(PROD_ENV) scripts/reset-demo.sh
