#!/bin/sh
# Role untuk notify-service: hanya baca data antrian, dan hapus langganan push yang kedaluwarsa.
# Dijalankan otomatis saat data directory kosong (bersama 01 dan 02). Dev saja.
set -e
psql -v ON_ERROR_STOP=1 -U "$POSTGRES_USER" -d "$POSTGRES_DB" <<SQL
CREATE ROLE notify_ro LOGIN PASSWORD '$NOTIFY_DB_PASSWORD';
GRANT CONNECT ON DATABASE "$POSTGRES_DB" TO notify_ro;
GRANT USAGE ON SCHEMA public TO notify_ro;
GRANT SELECT ON tickets, queues, push_subscriptions TO notify_ro;
GRANT DELETE ON push_subscriptions TO notify_ro;
SQL
