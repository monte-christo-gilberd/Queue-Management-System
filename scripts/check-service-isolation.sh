#!/usr/bin/env bash
# Memastikan notify-service (Web Push) TIDAK bergantung pada mail-service, mailpit, atau SMTP, di compose dev
# dan prod. Jaminannya: email mati tidak boleh menghentikan push. Dijalankan dari akar repo:
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

check dev docker compose
# Prod mewajibkan variabel SMTP; nilai palsu cukup untuk menguraikan compose.
SMTP_HOST=x SMTP_PORT=587 SMTP_FROM=a@b.co MAIL_SERVICE_TOKEN=0123456789abcdef0123456789abcdef \
  check prod docker compose -f docker-compose.prod.yml --env-file "${QMS_ENV_FILE:-.env}"

# Kode notify-service juga tidak boleh menyebut email.
if grep -rniE 'smtp|mail-service|mailer' notify-service --include=*.go --include=go.mod -l | grep -q .; then
  echo "GAGAL [kode] notify-service menyebut smtp/mail-service/mailer:"; grep -rniE 'smtp|mail-service|mailer' notify-service --include=*.go --include=go.mod -l; exit 1
fi
echo "OK [kode] kode notify-service tidak menyebut smtp/mail-service/mailer"
