#!/usr/bin/env bash
# Memastikan notify-service (Web Push) TIDAK bergantung pada mail-service, mailpit, atau SMTP, di compose dev
# dan prod. Jaminannya: email mati tidak boleh menghentikan push. Juga memastikan payment-service TERPISAH dari
# jalur antrian: backend, nginx, dan notify-service tidak bergantung padanya, dan ia hanya bergantung pada db.
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
    extra = set((pay.get("depends_on") or {}).keys()) - {"db"}
    if extra: problems.append(f"payment-service depends_on {sorted(extra)} (hanya db yang boleh)")
if problems:
    print(f"GAGAL [{label}] payment-service tidak terisolasi: " + "; ".join(problems)); sys.exit(1)
print(f"OK [{label}] payment-service terisolasi: backend/nginx/service lain tidak bergantung padanya, ia hanya butuh db")
' "$label"
}

check dev docker compose
check_payment dev docker compose
# Prod mewajibkan variabel SMTP; nilai palsu cukup untuk menguraikan compose.
export SMTP_HOST=x SMTP_PORT=587 SMTP_FROM=a@b.co MAIL_SERVICE_TOKEN=0123456789abcdef0123456789abcdef
check prod docker compose -f docker-compose.prod.yml --env-file "${QMS_ENV_FILE:-.env}"
check_payment prod docker compose -f docker-compose.prod.yml --env-file "${QMS_ENV_FILE:-.env}"

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
if grep -nE 'upstream[[:space:]]+payment|proxy_pass[[:space:]]+http://payment-service' deploy/nginx/*.conf | grep -q .; then
  echo "GAGAL [nginx] upstream payment-service harus lewat variabel (set \$qms_payment) agar nginx tetap start bila mati"; exit 1
fi
echo "OK [nginx] upstream payment-service lewat variabel + resolver"
