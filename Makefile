# Perintah harian untuk stack dev (Docker Compose + nginx).
#   make up              # build + jalankan, alamat LAN: http://<IP laptop>:8080 (HP satu Wi-Fi)
#   make up IP=192.168.1.5   # mode LAN dengan IP tertentu
#   make tunnel          # HTTPS publik via Cloudflare quick tunnel (kamera HP jalan)
#   make down            # hentikan stack dan tunnel
#   make restart         # restart backend, qr-service, notify-service setelah ubah kode Go
#   make seed            # reset database ke data seed dev
# Kamera di browser butuh HTTPS atau localhost, jadi scan QR dari HP lewat IP LAN
# tidak jalan di web; pakai `make tunnel`. Tunnel butuh cloudflared dan internet.
# URL tunnel acak tiap sesi dan dibaca backend/qr-service saat start (QMS_PUBLIC_ORIGIN).
SHELL := bash
.SHELLFLAGS := -euo pipefail -c
.ONESHELL:
.PHONY: up tunnel down restart seed logs ps prod-up prod-down prod-guard prod-backup prod-reset

PIDFILE := /tmp/qms-tunnel.pid
LOG := /tmp/qms-tunnel.log

define need_env
[ -f .env ] || { echo ".env belum ada (cp .env.example .env lalu isi secret)" >&2; exit 1; }
port=$$(grep -E '^QMS_HTTP_PORT=' .env | cut -d= -f2 || true); port=$${port:-8080}
endef

define stop_tunnel
[ -f $(PIDFILE) ] && kill "$$(cat $(PIDFILE))" 2>/dev/null || true; rm -f $(PIDFILE)
endef

up:
	$(need_env)
	ip="$(IP)"
	[ -n "$$ip" ] || ip=$$(ip route get 1.1.1.1 2>/dev/null | grep -oP 'src \K\S+' || true)
	[ -n "$$ip" ] || { echo "IP LAN tidak terdeteksi; beri manual: make up IP=<ip>" >&2; exit 1; }
	export QMS_PUBLIC_ORIGIN="http://$$ip:$$port"
	docker compose up -d --build --force-recreate
	echo; echo "Buka dari HP (satu Wi-Fi): $$QMS_PUBLIC_ORIGIN"

tunnel:
	command -v cloudflared >/dev/null || { echo "cloudflared belum terpasang" >&2; exit 1; }
	$(need_env)
	$(stop_tunnel); rm -f $(LOG)
	cloudflared tunnel --protocol http2 --no-autoupdate --url "http://localhost:$$port" >$(LOG) 2>&1 &
	echo $$! > $(PIDFILE)
	url=""
	for _ in $$(seq 1 40); do
	  url=$$(grep -aoE 'https://[a-z0-9-]+\.trycloudflare\.com' $(LOG) | head -1 || true)
	  [ -n "$$url" ] && break; sleep 1
	done
	[ -n "$$url" ] || { echo "URL tunnel tidak muncul, lihat $(LOG)" >&2; $(stop_tunnel); exit 1; }
	QMS_PUBLIC_ORIGIN="$$url" docker compose up -d --build --force-recreate backend qr-service notify-service nginx frontend db
	echo; echo "Buka dari HP mana pun: $$url"; echo "Hentikan: make down"

down:
	$(stop_tunnel)
	docker compose down

restart:
	docker compose restart backend qr-service notify-service

seed:
	docker compose exec -T db psql -U qms -d qms_db < db/scripts/reset_seed_dev.sql

logs:
	docker compose logs -f --tail 50

ps:
	docker compose ps

# --- Stack demo/produksi (docker-compose.prod.yml, env dari .env.prod; salin dari .env.prod.example) ---
PROD_ENV ?= .env.prod
PROD := docker compose -f docker-compose.prod.yml --env-file $(PROD_ENV)

prod-up:
	$(PROD) up -d --build

prod-down:
	$(PROD) down

# Tandai DB prod agar seed/reset ditolak. Jalankan sekali setelah akun demo dibuat.
prod-guard:
	$(PROD) exec -T db psql -U qms -d qms_db -c "CREATE TABLE IF NOT EXISTS _no_seed_guard (x int)"

prod-backup:
	QMS_ENV_FILE=$(PROD_ENV) scripts/backup-prod.sh backup

# Reset data demo: hapus antrian/tiket/push, akun dipertahankan (backup otomatis dulu).
prod-reset:
	QMS_ENV_FILE=$(PROD_ENV) scripts/reset-demo.sh
