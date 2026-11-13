#!/usr/bin/env bash
# Membuat 1 akun penguji (owner sekaligus customer) dan mengisi data uji antrian di stack prod.
# Akun dibuat lewat API register (sandi acak, hash bcrypt benar). Bila akun sudah ada, sandinya TIDAK diubah;
# hanya data ujinya yang diisi ulang (sama seperti restore-prod-penguji.sh).
# Kredensial ditulis ke berkas mode 600 di luar repo (bawaan ~/qms-penguji-account.txt). Tidak menimpa berkas yang ada.
#   scripts/seed-prod-penguji.sh          # minta konfirmasi (ketik 'ya')
#   scripts/seed-prod-penguji.sh -y       # tanpa konfirmasi
# Variabel: QMS_ENV_FILE (bawaan .env.prod), QMS_API (bawaan http://localhost:8080),
#           PENGUJI_EMAIL (bawaan penguji@montechristo.my.id), PENGUJI_USERNAME (bawaan penguji), PENGUJI_FILE.
# Isi data uji: lihat db/scripts/seed_prod_penguji.sql. Backup database dibuat otomatis lebih dulu.
set -euo pipefail
cd "$(dirname "$0")/.."
ENV_FILE="${QMS_ENV_FILE:-.env.prod}"
API="${QMS_API:-http://localhost:8080}/api/v1"
EMAIL="${PENGUJI_EMAIL:-penguji@montechristo.my.id}"
USERNAME="${PENGUJI_USERNAME:-penguji}"
OUT="${PENGUJI_FILE:-$HOME/qms-penguji-account.txt}"
SQL="db/scripts/seed_prod_penguji.sql"
DC=(docker compose -f docker-compose.prod.yml --env-file "$ENV_FILE")
DB_USER="${POSTGRES_USER:-qms}"; DB_NAME="${POSTGRES_DB:-qms_db}"
PSQL=("${DC[@]}" exec -T db psql -U "$DB_USER" -d "$DB_NAME" -v ON_ERROR_STOP=1)

[[ "$EMAIL" =~ ^[A-Za-z0-9._%+-]+@[A-Za-z0-9.-]+\.[A-Za-z]{2,}$ ]] || { echo "PENGUJI_EMAIL tidak valid: $EMAIL" >&2; exit 2; }
[[ "$USERNAME" =~ ^[A-Za-z0-9_.-]{3,30}$ ]] || { echo "PENGUJI_USERNAME tidak valid (3-30 karakter: huruf, angka, _ . -)" >&2; exit 2; }
"${DC[@]}" ps --status running db 2>/dev/null | grep -q db || { echo "stack prod tidak berjalan (make prod-up)" >&2; exit 1; }

exists=$(printf "SELECT COUNT(*) FROM users WHERE email = :'e';" | "${PSQL[@]}" -tA -v e="$EMAIL")

if [ "$exists" = "1" ]; then
  echo "Akun $EMAIL sudah ada: sandinya tidak diubah. Hanya data uji yang diisi ulang."
fi

if [ "${1:-}" != "-y" ]; then
  read -r -p "Isi ulang SEMUA antrian, tiket, layar, dan riwayat akun $EMAIL? Ketik 'ya': " ans
  [ "$ans" = "ya" ] || { echo "dibatalkan"; exit 1; }
fi

created=0
if [ "$exists" = "0" ]; then
  [ -e "$OUT" ] && { echo "sudah ada: $OUT, tetapi akun $EMAIL belum ada. Pindahkan berkas itu dulu." >&2; exit 1; }
  pw=$(openssl rand -base64 18 | tr -d '/+=' | cut -c1-16)
  code=$(curl -s -o /dev/null -w '%{http_code}' -X POST -H 'Content-Type: application/json' \
    -d "{\"username\":\"$USERNAME\",\"email\":\"$EMAIL\",\"password\":\"$pw\"}" "$API/users/register")
  [ "$code" = "200" ] || [ "$code" = "201" ] || { echo "daftar akun penguji gagal (HTTP $code)" >&2; exit 1; }
  ( umask 077; : > "$OUT"
    printf 'Akun penguji QMS  https://qms.montechristo.my.id\n\n' >> "$OUT"
    printf 'Email    %s\nPassword %s\n\n' "$EMAIL" "$pw" >> "$OUT"
    printf 'Akun ini owner (langganan 365 hari) dan customer sekaligus.\n' >> "$OUT" )
  chmod 600 "$OUT"
  created=1
  echo "Akun $EMAIL dibuat."
fi

echo "backup dulu..."
QMS_ENV_FILE="$ENV_FILE" scripts/backup-prod.sh backup

"${PSQL[@]}" -q -v tester_email="$EMAIL" -v tester_username="$USERNAME" < "$SQL"

echo "OK: data uji penguji diisi."
if [ "$created" = "1" ]; then
  echo "Akun baru dibuat. Email dan sandi ada di: $OUT"
fi
echo "Setelah pengujian, kembalikan data dengan: make prod-restore-penguji (atau scripts/restore-prod-penguji.sh)."
