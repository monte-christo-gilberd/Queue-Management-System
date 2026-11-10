#!/usr/bin/env bash
# Memastikan notify-service (Web Push) TIDAK bergantung pada mail-service, mailpit, atau SMTP, di compose dev
# dan prod. Jaminannya: email mati tidak boleh menghentikan push. Juga memastikan payment-service TERPISAH dari
# jalur antrian: backend, nginx, dan notify-service tidak bergantung padanya, dan ia hanya bergantung pada db.
# Terakhir, memastikan auth-service (login) TERPISAH: tidak ada service lain yang bergantung padanya (jadi user yang
# sudah login tetap bisa memakai aplikasi saat ia mati), ia hanya bergantung pada db, dan kunci privat JWT hanya
# di-mount ke auth-service.
# Dijalankan dari akar repo:
#   scripts/check-service-isolation.sh
set -euo pipefail
cd "$(dirname "$0")/.."

check() { # <label> <perintah compose...>
  local label="$1"; shift
  "$@" config --format json | python3 -c '
import json, sys
label = sys.argv[1]
svc = json.load(sys.stdin)["services"]["notify-service"]
deps = set((svc.get("depends_on") or {}).keys())
env = set((svc.get("environment") or {}).keys())
problems = []
bad_deps = deps & {"mail-service", "mailpit"}
if bad_deps: problems.append(f"depends_on {sorted(bad_deps)}")
bad_env = sorted(k for k in env if k.startswith("SMTP_") or k.startswith("MAIL_"))
if bad_env: problems.append(f"env {bad_env}")
if problems:
    print(f"GAGAL [{label}] notify-service bergantung pada email: " + "; ".join(problems)); sys.exit(1)
print(f"OK [{label}] notify-service tidak bergantung pada mail-service/mailpit/SMTP (depends_on={sorted(deps)})")
' "$label"
}

check_payment() { # <label> <perintah compose...>
  local label="$1"; shift
  "$@" config --format json | python3 -c '
import json, sys
label = sys.argv[1]
svcs = json.load(sys.stdin)["services"]
problems = []
for name in ("backend", "nginx", "notify-service", "qr-service", "mail-service", "frontend"):
    svc = svcs.get(name)
    if not svc: continue
    deps = set((svc.get("depends_on") or {}).keys())
    if "payment-service" in deps: problems.append(f"{name} depends_on payment-service")
    bad_env = sorted(k for k in (svc.get("environment") or {}) if k.startswith(("PAYMENT_", "STRIPE_")))
    if bad_env: problems.append(f"{name} env {bad_env}")
pay = svcs.get("payment-service")
if not pay: problems.append("payment-service tidak ada di compose")
else:
    if label == "prod" and "DEV_PAYMENT_SIMULATOR" in (pay.get("environment") or {}):
        problems.append("payment-service prod meneruskan DEV_PAYMENT_SIMULATOR (pembayaran palsu tidak boleh ada di prod)")
    extra = set((pay.get("depends_on") or {}).keys()) - {"db"}
    if extra: problems.append(f"payment-service depends_on {sorted(extra)} (hanya db yang boleh)")
if problems:
    print(f"GAGAL [{label}] payment-service tidak terisolasi: " + "; ".join(problems)); sys.exit(1)
print(f"OK [{label}] payment-service terisolasi: backend/nginx/service lain tidak bergantung padanya, ia hanya butuh db")
' "$label"
}


check_auth() { # <label> <perintah compose...>
  local label="$1"; shift
  "$@" config --format json | python3 -c '
import json, sys
label = sys.argv[1]
svcs = json.load(sys.stdin)["services"]
problems = []
auth = svcs.get("auth-service")
if not auth:
    problems.append("auth-service tidak ada di compose")
else:
    extra = set((auth.get("depends_on") or {}).keys()) - {"db"}
    if extra: problems.append(f"auth-service depends_on {sorted(extra)} (hanya db yang boleh)")
for name, svc in svcs.items():
    if name == "auth-service": continue
    if "auth-service" in (svc.get("depends_on") or {}):
        problems.append(f"{name} depends_on auth-service")
    # Kunci privat penanda token hanya boleh dipegang auth-service (folder keys/ penuh juga membukanya).
    for v in svc.get("volumes") or []:
        src = str(v.get("source", ""))
        if "auth-private" in src or src.rstrip("/").endswith("/keys"):
            problems.append(f"{name} me-mount kunci privat JWT ({src})")
    bad_env = sorted(k for k in (svc.get("environment") or {}) if k.startswith("AUTH_JWT_PRIVATE"))
    if bad_env: problems.append(f"{name} env {bad_env}")
if problems:
    print(f"GAGAL [{label}] auth-service tidak terisolasi: " + "; ".join(problems)); sys.exit(1)
print(f"OK [{label}] auth-service terisolasi: tidak ada service yang bergantung padanya, ia hanya butuh db, kunci privat hanya di auth-service")
' "$label"
}

check dev docker compose
check_payment dev docker compose
check_auth dev docker compose
# Prod mewajibkan variabel SMTP; nilai palsu cukup untuk menguraikan compose.
export SMTP_HOST=x SMTP_PORT=587 SMTP_FROM=a@b.co MAIL_SERVICE_TOKEN=0123456789abcdef0123456789abcdef
check prod docker compose -f docker-compose.prod.yml --env-file "${QMS_ENV_FILE:-.env}"
check_payment prod docker compose -f docker-compose.prod.yml --env-file "${QMS_ENV_FILE:-.env}"
check_auth prod docker compose -f docker-compose.prod.yml --env-file "${QMS_ENV_FILE:-.env}"

# Kode notify-service juga tidak boleh menyebut email.
if grep -rniE 'smtp|mail-service|mailer' notify-service --include=*.go --include=go.mod -l | grep -q .; then
  echo "GAGAL [kode] notify-service menyebut smtp/mail-service/mailer:"; grep -rniE 'smtp|mail-service|mailer' notify-service --include=*.go --include=go.mod -l; exit 1
fi
echo "OK [kode] kode notify-service tidak menyebut smtp/mail-service/mailer"

# Kode backend tidak boleh memanggil/menyebut payment-service atau Stripe: status langganan hanya dibaca dari DB.
if grep -rniE 'payment-service|stripe|PAYMENT_SERVICE' backend --include=*.go --include=go.mod -l | grep -q .; then
  echo "GAGAL [kode] backend menyebut payment-service/stripe:"; grep -rniE 'payment-service|stripe|PAYMENT_SERVICE' backend --include=*.go --include=go.mod -l; exit 1
fi
echo "OK [kode] kode backend tidak menyebut payment-service/stripe"

# nginx tidak boleh memakai upstream payment di luar variabel (host tak ter-resolve = nginx gagal start).
# Baris komentar diabaikan: komentar boleh menyebut kata "upstream payment-service".
if grep -hE 'upstream[[:space:]]+payment|proxy_pass[[:space:]]+http://payment-service' deploy/nginx/*.conf | grep -vE '^[[:space:]]*#' | grep -q .; then
  echo "GAGAL [nginx] upstream payment-service harus lewat variabel (set \$qms_payment) agar nginx tetap start bila mati"; exit 1
fi
echo "OK [nginx] upstream payment-service lewat variabel + resolver"

# Backend tidak boleh memanggil auth-service: token diverifikasi lokal dengan kunci publik. Tidak ada klien HTTP,
# URL, atau kunci privat auth-service di kode backend (komentar boleh menyebut namanya).
if grep -rnE 'AUTH_SERVICE_URL|auth-service:[0-9]+|AUTH_JWT_PRIVATE' backend --include=*.go --include=go.mod | grep -q .; then
  echo "GAGAL [kode] backend memanggil/menyebut alamat auth-service atau kunci privat:"; grep -rnE 'AUTH_SERVICE_URL|auth-service:[0-9]+|AUTH_JWT_PRIVATE' backend --include=*.go --include=go.mod; exit 1
fi
echo "OK [kode] kode backend tidak memanggil auth-service"

# nginx tidak boleh memakai upstream auth-service di luar variabel (host tak ter-resolve = nginx gagal start).
if grep -hE 'upstream[[:space:]]+auth|proxy_pass[[:space:]]+http://auth-service' deploy/nginx/*.conf | grep -vE '^[[:space:]]*#' | grep -q .; then
  echo "GAGAL [nginx] upstream auth-service harus lewat variabel (set \$qms_auth) agar nginx tetap start bila mati"; exit 1
fi
echo "OK [nginx] upstream auth-service lewat variabel + resolver"
