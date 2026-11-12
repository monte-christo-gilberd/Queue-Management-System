#!/bin/sh
# observability: role database metrics_ro untuk postgres_exporter (Prometheus).
# Role ini hanya membaca statistik dan pengaturan server lewat pg_monitor (pg_stat_*, pg_settings, ukuran database).
# TIDAK ada hak SELECT ke tabel data aplikasi (users, tickets, ...), jadi kebocoran kredensialnya tidak membuka data.
# Idempoten: aman dijalankan ulang pada DB yang sudah ada (membuat role atau mengganti passwordnya):
#   docker compose exec -T -e METRICS_DB_PASSWORD="$METRICS_DB_PASSWORD" db sh < db/migrations/07_metrics_role.sh
# `make obs-up` menjalankannya otomatis. Saat database dibuat pertama kali, skrip ini ikut berjalan tetapi
# METRICS_DB_PASSWORD biasanya belum tersedia di container db, jadi dilewati (BUKAN galat) supaya observability tidak
# pernah menghalangi database start.
set -e
if [ -z "${METRICS_DB_PASSWORD:-}" ]; then
  echo "07_metrics_role: METRICS_DB_PASSWORD kosong, dilewati (postgres_exporter tidak akan bisa terhubung)" >&2
  exit 0
fi
psql -v ON_ERROR_STOP=1 -v pw="$METRICS_DB_PASSWORD" -U "$POSTGRES_USER" -d "$POSTGRES_DB" <<'SQL'
SELECT format('CREATE ROLE metrics_ro LOGIN PASSWORD %L', :'pw')
 WHERE NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'metrics_ro') \gexec
SELECT format('ALTER ROLE metrics_ro PASSWORD %L', :'pw') \gexec
-- pg_monitor: baca pg_stat_*, pg_settings, dan fungsi pemantauan. Tidak memberi akses ke isi tabel.
GRANT pg_monitor TO metrics_ro;
GRANT CONNECT ON DATABASE :"DBNAME" TO metrics_ro;
SQL
