#!/bin/sh
# auth-service: role database terbatas auth_rw.
# Role ini hanya boleh menyentuh akun dan token sekali pakai: membuat user baru, membaca kolom login, mengubah
# kolom yang memang diubah alur auth (password, verifikasi email, tautan Google), dan memakai tabel
# user_action_tokens. Tidak ada akses ke antrian, tiket, pembayaran, atau data lain.
# Idempoten: aman dijalankan ulang pada DB yang sudah ada (membuat role atau mengganti passwordnya):
#   docker compose exec -T -e AUTH_DB_PASSWORD="$AUTH_DB_PASSWORD" db sh < db/migrations/06_auth_role.sh
# Bila AUTH_DB_PASSWORD kosong, langkah ini dilewati (BUKAN galat) supaya auth-service tidak pernah menghalangi
# database start; hanya auth-service yang belum bisa terhubung (login baru gagal, user yang sudah login tetap jalan).
set -e
if [ -z "${AUTH_DB_PASSWORD:-}" ]; then
  echo "06_auth_role: AUTH_DB_PASSWORD kosong, dilewati (auth-service tidak akan bisa terhubung ke database)" >&2
  exit 0
fi
psql -v ON_ERROR_STOP=1 -v pw="$AUTH_DB_PASSWORD" -U "$POSTGRES_USER" -d "$POSTGRES_DB" <<'SQL'
SELECT format('CREATE ROLE auth_rw LOGIN PASSWORD %L', :'pw')
 WHERE NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'auth_rw') \gexec
SELECT format('ALTER ROLE auth_rw PASSWORD %L', :'pw') \gexec
GRANT CONNECT ON DATABASE :"DBNAME" TO auth_rw;
GRANT USAGE ON SCHEMA public TO auth_rw;
-- users: baca kolom login dan verifikasi; tulis hanya kolom yang diubah alur auth.
GRANT SELECT (id, email, username, password_hash, google_id, created_at, password_changed_at, email_verified_at) ON users TO auth_rw;
GRANT INSERT (email, username, password_hash, google_id) ON users TO auth_rw;
GRANT UPDATE (password_hash, password_changed_at, email_verified_at, google_id) ON users TO auth_rw;
GRANT USAGE ON SEQUENCE users_id_seq TO auth_rw;
-- user_action_tokens: token reset password dan verifikasi email (hanya hash). Tidak ada DELETE.
GRANT SELECT, INSERT ON user_action_tokens TO auth_rw;
GRANT UPDATE (used_at) ON user_action_tokens TO auth_rw;
GRANT USAGE ON SEQUENCE user_action_tokens_id_seq TO auth_rw;
SQL
