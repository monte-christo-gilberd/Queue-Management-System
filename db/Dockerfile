# =============================================================================
# Dockerfile: kontainer database PostgreSQL QMS.
# -----------------------------------------------------------------------------
# Build context : folder db/ (docker-compose*.yml: build.context).
# Dipakai oleh  : docker-compose.yml dan docker-compose.prod.yml.
# Tujuan        : image PostgreSQL 16 yang membuat skema dan role secara otomatis saat
#                 data directory masih kosong. Script di /docker-entrypoint-initdb.d
#                 hanya jalan pada start pertama, bukan setiap kali container dinyalakan.
# Berkas init   : db/migrations/ (*.sql dan *.sh). Lihat db/migrations/README.md.
#
# Saat ada perubahan, periksa:
#   [PENTING] Urutan nama berkas menentukan urutan eksekusi. Jangan mengganti nama berkas
#             sembarangan, dan pastikan role di 03..06 dibuat sebelum dipakai.
#   [PENTING] Data lama tidak berubah. Perubahan skema di sini hanya berlaku pada database
#             baru (volume kosong). Untuk database yang sudah ada, pakai migrasi terpisah.
# =============================================================================

# Image dasar: PostgreSQL 16 di atas Alpine. Versi dikunci agar skema konsisten dengan yang diuji.
FROM postgres:16-alpine

# Init script di /docker-entrypoint-initdb.d dijalankan berurutan saat container pertama kali dibuat.
# Urutan nama = urutan jalan: 01a..01h (skema per domain), 02 (index), 03..06 (role per service). Lihat migrations/README.md.
# Menyalin semua script SQL dan shell dari migrations/ ke folder init. Entrypoint image postgres menjalankannya
# berurutan (alfabetis) saat data directory kosong.
COPY migrations/*.sql migrations/*.sh /docker-entrypoint-initdb.d/

# Kredensial jangan dibakar ke image. Isi saat run: POSTGRES_USER, POSTGRES_PASSWORD, POSTGRES_DB.
# Dokumentasi: port PostgreSQL. Hanya terjangkau di jaringan Docker, tidak dipetakan ke host.
EXPOSE 5432
