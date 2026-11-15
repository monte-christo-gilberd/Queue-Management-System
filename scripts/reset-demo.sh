#!/usr/bin/env bash
# RESET TOTAL database prod: menghapus SELURUH data, termasuk semua akun (owner, pelanggan, penguji), antrian,
# tiket, riwayat, dan pembayaran. Volume db_data_prod dihapus, lalu stack dinyalakan lagi dengan database kosong.
# Skema dan role dibuat ulang otomatis dari db/migrations/ oleh container db (volume kosong).
#
# Backup otomatis dibuat dulu ke ~/qms-backups/ (scripts/backup-prod.sh). Untuk memulihkan:
#   scripts/backup-prod.sh restore <file>
#
# Setelah reset, jalankan:
#   make prod-seed-penguji    # membuat akun penguji dan pelanggan seed, kata sandi acak, data uji
#
#   scripts/reset-demo.sh          # minta konfirmasi (ketik 'hapus-prod')
#   scripts/reset-demo.sh -y       # tanpa konfirmasi
set -euo pipefail
cd "$(dirname "$0")/.."
ENV_FILE="${QMS_ENV_FILE:-.env.prod}"
DC=(docker compose -f docker-compose.prod.yml --env-file "$ENV_FILE")
# Nama volume = nama project (name: qms-prod di docker-compose.prod.yml) + nama volume di compose.
VOLUME="qms-prod_db_data_prod"
DB_USER="${POSTGRES_USER:-qms}"; DB_NAME="${POSTGRES_DB:-qms_db}"

"${DC[@]}" ps --status running db 2>/dev/null | grep -q db || { echo "stack prod tidak berjalan (db harus hidup untuk backup)" >&2; exit 1; }

echo "PERINGATAN: ini menghapus SELURUH database prod, termasuk semua akun owner dan pelanggan."
if [ "${1:-}" != "-y" ]; then
  read -r -p "Hapus seluruh database prod dan mulai dari kosong? Ketik 'hapus-prod': " ans
  [ "$ans" = "hapus-prod" ] || { echo "dibatalkan"; exit 1; }
fi

echo "backup dulu..."
QMS_ENV_FILE="$ENV_FILE" scripts/backup-prod.sh backup

echo "menghentikan dan menghapus container prod..."
"${DC[@]}" --profile '*' down --remove-orphans

echo "menghapus volume database..."
docker volume rm "$VOLUME"

echo "menyalakan ulang stack dengan database kosong..."
make prod-up PROD_ENV="$ENV_FILE"

# Tunggu database siap (healthcheck pg_isready).
db_id=$("${DC[@]}" ps -q db)
for _ in $(seq 1 60); do
  status=$(docker inspect --format '{{.State.Health.Status}}' "$db_id" 2>/dev/null || echo starting)
  [ "$status" = "healthy" ] && break
  sleep 2
done
[ "$status" = "healthy" ] || { echo "database belum sehat setelah reset (status: $status)" >&2; exit 1; }

# Tandai DB prod agar seed/reset dev ditolak (sama dengan make prod-guard).
"${DC[@]}" exec -T db psql -U "$DB_USER" -d "$DB_NAME" -v ON_ERROR_STOP=1 \
  -c "CREATE TABLE IF NOT EXISTS _no_seed_guard (x int)" >/dev/null

echo "OK: database prod sudah kosong dan stack berjalan."
echo "Langkah berikutnya: make prod-seed-penguji"
