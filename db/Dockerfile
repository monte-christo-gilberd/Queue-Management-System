# Kontainer database (PostgreSQL). Skema dijalankan otomatis saat data directory masih kosong.
# Build context: folder db/
FROM postgres:16-alpine

# Init script di /docker-entrypoint-initdb.d dijalankan berurutan saat container pertama kali dibuat.
COPY migrations/01_Schema.sql /docker-entrypoint-initdb.d/01_Schema.sql

# Kredensial jangan dibakar ke image. Isi saat run: POSTGRES_USER, POSTGRES_PASSWORD, POSTGRES_DB.
EXPOSE 5432
