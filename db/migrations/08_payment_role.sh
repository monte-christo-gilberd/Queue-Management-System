#!/bin/sh
# payment-service: role database terbatas payment_rw.
# Role ini hanya boleh membuat dan memperbarui pesanan pembayaran, memperpanjang langganan owner, dan mencatat
# pembayaran. Tidak ada akses ke tiket, antrian, akun, atau token.
# Idempoten: aman dijalankan ulang (membuat role atau mengganti passwordnya):
#   docker compose exec -T -e PAYMENT_DB_PASSWORD="$PAYMENT_DB_PASSWORD" db sh < db/migrations/08_payment_role.sh
# Bila PAYMENT_DB_PASSWORD kosong, langkah ini dilewati (BUKAN galat) supaya database tetap bisa start.
set -e
if [ -z "${PAYMENT_DB_PASSWORD:-}" ]; then
  echo "08_payment_role: PAYMENT_DB_PASSWORD kosong, dilewati (payment-service tidak bisa terhubung ke database)" >&2
  exit 0
fi
psql -v ON_ERROR_STOP=1 -v pw="$PAYMENT_DB_PASSWORD" -U "$POSTGRES_USER" -d "$POSTGRES_DB" <<'SQL'
SELECT format('CREATE ROLE payment_rw LOGIN PASSWORD %L', :'pw')
 WHERE NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'payment_rw') \gexec
SELECT format('ALTER ROLE payment_rw PASSWORD %L', :'pw') \gexec
DO $$
BEGIN
    EXECUTE format('GRANT CONNECT ON DATABASE %I TO payment_rw', current_database());
END $$;
GRANT USAGE ON SCHEMA public TO payment_rw;
-- owners: baca user_id dan langganan; buat baris baru (calon owner) dan perbarui subscription_expires_at.
GRANT SELECT (user_id, subscription_expires_at) ON owners TO payment_rw;
GRANT INSERT (user_id, subscription_expires_at) ON owners TO payment_rw;
GRANT UPDATE (subscription_expires_at) ON owners TO payment_rw;
-- payment_orders: buat pesanan, simpan pay_url, dan tandai lunas. Tidak ada DELETE.
GRANT SELECT, INSERT ON payment_orders TO payment_rw;
GRANT UPDATE (status, pay_url, paid_at, updated_at) ON payment_orders TO payment_rw;
-- payments: catat pembayaran lunas.
GRANT INSERT ON payments TO payment_rw;
GRANT USAGE ON SEQUENCE payments_id_seq TO payment_rw;
SQL
