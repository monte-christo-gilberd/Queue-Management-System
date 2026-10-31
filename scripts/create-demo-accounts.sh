#!/usr/bin/env bash
# Membuat akun demo di stack prod: 1 owner (langganan 30 hari) + 4 customer, sandi acak kuat.
# Daftar akun ditulis ke ~/qms-demo-accounts.txt (mode 600, di luar repo). Tidak menimpa berkas yang ada.
#   scripts/create-demo-accounts.sh
set -euo pipefail
cd "$(dirname "$0")/.."
ENV_FILE="${QMS_ENV_FILE:-.env.prod}"
API="${QMS_API:-http://localhost:8080}/api/v1"
OUT="${QMS_DEMO_FILE:-$HOME/qms-demo-accounts.txt}"
DC=(docker compose -f docker-compose.prod.yml --env-file "$ENV_FILE")
[ -e "$OUT" ] && { echo "sudah ada: $OUT (hapus dulu bila ingin membuat ulang)" >&2; exit 1; }

pw() { openssl rand -base64 18 | tr -d '/+=' | cut -c1-16; }
reg() { # username email password
  code=$(curl -s -o /dev/null -w '%{http_code}' -X POST -H 'Content-Type: application/json' \
    -d "{\"username\":\"$1\",\"email\":\"$2\",\"password\":\"$3\"}" "$API/users/register")
  [ "$code" = "200" ] || [ "$code" = "201" ] || { echo "daftar $1 gagal (HTTP $code)" >&2; exit 1; }
}

( umask 077; : > "$OUT" )
printf 'Akun demo QMS  https://qms.montechristo.my.id\n\n' >> "$OUT"

p=$(pw); reg demo-owner demo-owner@montechristo.my.id "$p"
"${DC[@]}" exec -T db psql -U qms -d qms_db -v ON_ERROR_STOP=1 -q -c \
  "INSERT INTO owners (user_id, subscription_expires_at) SELECT id, now() + interval '30 days' FROM users WHERE username='demo-owner'"
printf 'OWNER     demo-owner@montechristo.my.id   %s\n' "$p" >> "$OUT"
for i in 1 2 3 4; do
  p=$(pw); reg "demo-user$i" "demo-user$i@montechristo.my.id" "$p"
  printf 'CUSTOMER  demo-user%s@montechristo.my.id   %s\n' "$i" "$p" >> "$OUT"
  sleep 1
done
chmod 600 "$OUT"
echo "OK: 5 akun dibuat, daftar di $OUT"
