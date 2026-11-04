# Kontainer database (PostgreSQL). Skema dijalankan otomatis saat data directory masih kosong.
# Build context: folder db/
FROM postgres:16-alpine

# Init script di /docker-entrypoint-initdb.d dijalankan berurutan saat container pertama kali dibuat.
COPY migrations/01_Schema.sql /docker-entrypoint-initdb.d/01_Schema.sql
COPY migrations/02_Indexes.sql /docker-entrypoint-initdb.d/02_Indexes.sql
COPY migrations/03_notify_role.sh /docker-entrypoint-initdb.d/03_notify_role.sh
COPY migrations/04_mail_role.sh /docker-entrypoint-initdb.d/04_mail_role.sh
COPY migrations/05_payment_role.sh /docker-entrypoint-initdb.d/05_payment_role.sh

# Kredensial jangan dibakar ke image. Isi saat run: POSTGRES_USER, POSTGRES_PASSWORD, POSTGRES_DB.
EXPOSE 5432
