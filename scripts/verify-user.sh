#!/usr/bin/env bash
# Menandai email satu akun sebagai terverifikasi langsung di database, tanpa menunggu email. Jalan keluar saat
# verifikasi lewat email tidak mungkin: MAIL_MODE=mailpit (email tidak sampai ke siapa pun), SMTP belum disiapkan,
# atau email salah ketik. Idempoten: akun yang sudah terverifikasi tidak diubah.
#   scripts/verify-user.sh budi@mail.com          # stack dev (docker-compose.yml)
#   scripts/verify-user.sh budi@mail.com prod     # stack prod (docker-compose.prod.yml, env dari .env.prod)
# Lewat make: make verify-user EMAIL=budi@mail.com [STACK=prod]
set -euo pipefail
cd "$(dirname "$0")/.."
EMAIL="${1:-}"
[ -n "$EMAIL" ] || { echo "pakai: scripts/verify-user.sh <email> [prod]" >&2; exit 1; }
case "${2:-}" in
  prod) DC=(docker compose -f docker-compose.prod.yml --env-file "${QMS_ENV_FILE:-.env.prod}") ;;
  "")   DC=(docker compose) ;;
  *)    echo "argumen kedua hanya boleh 'prod'" >&2; exit 1 ;;
esac

# Email dikirim sebagai variabel psql (:'email'), bukan disisipkan ke teks SQL, supaya karakter aneh tidak jadi SQL.
# Perintah lewat stdin karena psql tidak menyisipkan variabel pada -c.
out=$(printf '%s\n' "UPDATE users SET email_verified_at = now() WHERE email = :'email' AND email_verified_at IS NULL RETURNING id;" \
  | "${DC[@]}" exec -T db psql -U qms -d qms_db -v ON_ERROR_STOP=1 -v email="$EMAIL" -tAq)
if [ -n "$out" ]; then
  echo "OK: email $EMAIL ditandai terverifikasi (user id $out)"
elif "${DC[@]}" exec -T db psql -U qms -d qms_db -tAq -v email="$EMAIL" <<<"SELECT 1 FROM users WHERE email = :'email'" | grep -q 1; then
  echo "Sudah terverifikasi: $EMAIL (tidak ada yang diubah)"
else
  echo "GAGAL: tidak ada akun dengan email $EMAIL" >&2
  exit 1
fi
