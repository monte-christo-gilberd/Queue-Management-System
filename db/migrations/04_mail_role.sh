#!/bin/sh
# Role untuk mail-service (pengirim email): baca outbox dan sebagian kolom users, ubah status kirim outbox.
# Idempoten: aman dijalankan ulang pada DB yang sudah ada untuk membuat role atau mengganti passwordnya:
#   docker compose exec -T -e MAIL_DB_PASSWORD="$MAIL_DB_PASSWORD" db sh < db/migrations/04_mail_role.sh
# (di DB lama, jalankan juga db/scripts/apply_schema.sql lebih dulu agar tabel outbox ada).
set -e
: "${MAIL_DB_PASSWORD:?MAIL_DB_PASSWORD belum diisi}"
psql -v ON_ERROR_STOP=1 -v pw="$MAIL_DB_PASSWORD" -U "$POSTGRES_USER" -d "$POSTGRES_DB" <<'SQL'
SELECT format('CREATE ROLE mail_rw LOGIN PASSWORD %L', :'pw')
 WHERE NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'mail_rw') \gexec
SELECT format('ALTER ROLE mail_rw PASSWORD %L', :'pw') \gexec
GRANT CONNECT ON DATABASE :"DBNAME" TO mail_rw;
GRANT USAGE ON SCHEMA public TO mail_rw;
GRANT SELECT ON notification_outbox TO mail_rw;
GRANT UPDATE (sent_at, attempts, last_error, next_attempt_at, failed_at) ON notification_outbox TO mail_rw;
GRANT SELECT (id, email, username, email_notifications) ON users TO mail_rw;
SQL
