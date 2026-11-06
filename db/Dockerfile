# Kontainer database (PostgreSQL). Skema dijalankan otomatis saat data directory masih kosong.
# Build context: folder db/
FROM postgres:16-alpine

# Init script di /docker-entrypoint-initdb.d dijalankan berurutan saat container pertama kali dibuat.
# Urutan nama = urutan jalan: 01a..01h (skema per domain), 02 (index), 03..05 (role per service). Lihat migrations/README.md.
COPY migrations/*.sql migrations/*.sh /docker-entrypoint-initdb.d/

# Kredensial jangan dibakar ke image. Isi saat run: POSTGRES_USER, POSTGRES_PASSWORD, POSTGRES_DB.
EXPOSE 5432
