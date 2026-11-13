#!/usr/bin/env bash
# Mengembalikan data uji akun penguji di stack prod ke kondisi seed (db/scripts/seed_prod_penguji.sql).
# Yang dikembalikan: antrian, tiket, counter, layar publik (token baru), catatan, riwayat, dan profil dasar
# (username, notifikasi email, foto profil, langganan 365 hari dari sekarang). Kata sandi TIDAK diubah.
# Akun pelanggan seed dibuat ulang. Akun lain dan data produksi lain tidak disentuh.
# Syarat: akun penguji sudah ada (scripts/seed-prod-penguji.sh). Backup database dibuat otomatis lebih dulu.
#   scripts/restore-prod-penguji.sh          # minta konfirmasi (ketik 'ya')
#   scripts/restore-prod-penguji.sh -y       # tanpa konfirmasi
# Variabel: QMS_ENV_FILE (bawaan .env.prod), PENGUJI_EMAIL (bawaan penguji@montechristo.my.id), PENGUJI_USERNAME.
set -euo pipefail
cd "$(dirname "$0")/.."
ENV_FILE="${QMS_ENV_FILE:-.env.prod}"
EMAIL="${PENGUJI_EMAIL:-penguji@montechristo.my.id}"
USERNAME="${PENGUJI_USERNAME:-penguji}"
SQL="db/scripts/seed_prod_penguji.sql"
DC=(docker compose -f docker-compose.prod.yml --env-file "$ENV_FILE")
DB_USER="${POSTGRES_USER:-qms}"; DB_NAME="${POSTGRES_DB:-qms_db}"
PSQL=("${DC[@]}" exec -T db psql -U "$DB_USER" -d "$DB_NAME" -v ON_ERROR_STOP=1)

[[ "$EMAIL" =~ ^[A-Za-z0-9._%+-]+@[A-Za-z0-9.-]+\.[A-Za-z]{2,}$ ]] || { echo "PENGUJI_EMAIL tidak valid: $EMAIL" >&2; exit 2; }
"${DC[@]}" ps --status running db 2>/dev/null | grep -q db || { echo "stack prod tidak berjalan (make prod-up)" >&2; exit 1; }

exists=$(printf "SELECT COUNT(*) FROM users WHERE email = :'e';" | "${PSQL[@]}" -tA -v e="$EMAIL")
[ "$exists" = "1" ] || { echo "akun $EMAIL belum ada. Buat dulu dengan scripts/seed-prod-penguji.sh." >&2; exit 1; }

if [ "${1:-}" != "-y" ]; then
  read -r -p "Kembalikan data uji akun $EMAIL ke kondisi seed? Semua antrian dan tiketnya diganti. Ketik 'ya': " ans
  [ "$ans" = "ya" ] || { echo "dibatalkan"; exit 1; }
fi

echo "backup dulu..."
QMS_ENV_FILE="$ENV_FILE" scripts/backup-prod.sh backup

"${PSQL[@]}" -q -v tester_email="$EMAIL" -v tester_username="$USERNAME" < "$SQL"

echo "OK: data uji penguji dikembalikan ke kondisi seed."
