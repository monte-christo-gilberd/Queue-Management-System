#!/bin/sh
# payment-service: tabel pencatat webhook + role database terbatas payment_rw.
# Role ini hanya boleh: membaca user_id/subscription_expires_at dan mengubah subscription_expires_at di owners,
# membaca email di users, menambah baris payments, dan memakai dua tabel milik payment-service. Tidak ada akses
# ke antrian, tiket, atau data lain.
# Idempoten: aman dijalankan ulang pada DB yang sudah ada (membuat tabel/role atau mengganti passwordnya):
#   docker compose exec -T -e PAYMENT_DB_PASSWORD="$PAYMENT_DB_PASSWORD" db sh < db/migrations/05_payment_role.sh
# Bila PAYMENT_DB_PASSWORD kosong, langkah ini dilewati (BUKAN galat) supaya pembayaran tidak pernah menghalangi
# database dan seluruh sistem antrian start; payment-service saja yang belum bisa dipakai.
set -e
if [ -z "${PAYMENT_DB_PASSWORD:-}" ]; then
  echo "05_payment_role: PAYMENT_DB_PASSWORD kosong, dilewati (payment-service tidak akan bisa terhubung ke database)" >&2
  exit 0
fi
psql -v ON_ERROR_STOP=1 -v pw="$PAYMENT_DB_PASSWORD" -U "$POSTGRES_USER" -d "$POSTGRES_DB" <<'SQL'
-- Event webhook yang sudah diproses: Stripe mengirim ulang event, perpanjangan tidak boleh ganda.
CREATE TABLE IF NOT EXISTS payment_webhook_events (
    event_id     TEXT PRIMARY KEY,
    type         TEXT NOT NULL,
    processed_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
-- Satu sesi Checkout hanya boleh memperpanjang sekali, walau datang dari dua event berbeda.
CREATE TABLE IF NOT EXISTS payment_session_credits (
    session_id TEXT PRIMARY KEY,
    payment_id BIGINT REFERENCES payments(id),
    credited_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

SELECT format('CREATE ROLE payment_rw LOGIN PASSWORD %L', :'pw')
 WHERE NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'payment_rw') \gexec
SELECT format('ALTER ROLE payment_rw PASSWORD %L', :'pw') \gexec
GRANT CONNECT ON DATABASE :"DBNAME" TO payment_rw;
GRANT USAGE ON SCHEMA public TO payment_rw;
GRANT SELECT (user_id, subscription_expires_at) ON owners TO payment_rw;
GRANT UPDATE (subscription_expires_at) ON owners TO payment_rw;
GRANT SELECT (id, email) ON users TO payment_rw;
GRANT SELECT, INSERT ON payments TO payment_rw;
GRANT USAGE ON SEQUENCE payments_id_seq TO payment_rw;
GRANT SELECT, INSERT ON payment_webhook_events TO payment_rw;
GRANT SELECT, INSERT, UPDATE ON payment_session_credits TO payment_rw;
SQL
