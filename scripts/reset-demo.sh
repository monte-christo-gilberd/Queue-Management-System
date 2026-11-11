#!/usr/bin/env bash
# Reset cepat data demo di stack prod: menghapus semua antrian, tiket, catatan, riwayat, layar publik,
# email outbox, dan langganan push. Akun (users, owners, payments, foto profil) TETAP, dan langganan owner
# diperpanjang 30 hari.
# Backup otomatis dibuat dulu ke ~/qms-backups/. Tidak menyentuh stack dev.
#   scripts/reset-demo.sh        # minta konfirmasi
#   scripts/reset-demo.sh -y     # tanpa konfirmasi
set -euo pipefail
cd "$(dirname "$0")/.."
ENV_FILE="${QMS_ENV_FILE:-.env.prod}"
DC=(docker compose -f docker-compose.prod.yml --env-file "$ENV_FILE")
psql_() { "${DC[@]}" exec -T db psql -U "${POSTGRES_USER:-qms}" -d "${POSTGRES_DB:-qms_db}" -v ON_ERROR_STOP=1 "$@"; }

"${DC[@]}" ps --status running db 2>/dev/null | grep -q db || { echo "stack prod tidak berjalan (make prod-up)" >&2; exit 1; }

if [ "${1:-}" != "-y" ]; then
  read -r -p "Hapus SEMUA antrian dan tiket demo (akun dipertahankan)? Ketik 'ya': " ans
  [ "$ans" = "ya" ] || { echo "dibatalkan"; exit 1; }
fi

echo "backup dulu..."
QMS_ENV_FILE="$ENV_FILE" scripts/backup-prod.sh backup

psql_ -q <<'SQL'
BEGIN;
-- Semua tabel yang mereferensi queues, tickets, atau counters wajib ikut di daftar ini (queue_displays punya FK ke
-- queues). Tanpa CASCADE, tabel yang terlewat membuat TRUNCATE ditolak dan transaksi dibatalkan.
TRUNCATE TABLE notification_outbox, push_subscriptions, ticket_notes, counters, tickets, queue_history, queue_settings, queue_displays, queues RESTART IDENTITY;
UPDATE owners SET subscription_expires_at = GREATEST(subscription_expires_at, now()) + interval '30 days'
  WHERE subscription_expires_at < now() + interval '30 days';
COMMIT;
SQL

echo "OK: antrian, tiket, layar publik, dan langganan push dikosongkan."
psql_ -tA -c "select 'akun tersisa: ' || (select count(*) from users) || ', owner: ' || (select count(*) from owners)"
