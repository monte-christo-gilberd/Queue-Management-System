#!/usr/bin/env bash
# Backup dan restore database stack prod (docker-compose.prod.yml).
#   scripts/backup-prod.sh backup [file]     # default: ~/qms-backups/qms-<waktu>.sql.gz (di luar repo)
#   scripts/backup-prod.sh restore <file>    # MENIMPA isi database prod
# Env file dibaca dari $QMS_ENV_FILE (default .env).
set -euo pipefail
cd "$(dirname "$0")/.."
ENV_FILE="${QMS_ENV_FILE:-.env}"
DC=(docker compose -f docker-compose.prod.yml --env-file "$ENV_FILE")
DB_USER="${POSTGRES_USER:-qms}"; DB_NAME="${POSTGRES_DB:-qms_db}"

case "${1:-}" in
  backup)
    out="${2:-$HOME/qms-backups/qms-$(date +%Y%m%d-%H%M%S).sql.gz}"
    mkdir -p "$(dirname "$out")"
    ( umask 077; "${DC[@]}" exec -T db pg_dump -U "$DB_USER" -d "$DB_NAME" --clean --if-exists | gzip > "$out" )
    [ -s "$out" ] || { echo "backup kosong, gagal" >&2; rm -f "$out"; exit 1; }
    echo "OK: $out ($(du -h "$out" | cut -f1))"
    ;;
  restore)
    f="${2:?pakai: restore <file.sql.gz>}"
    [ -f "$f" ] || { echo "file tidak ada: $f" >&2; exit 1; }
    read -r -p "Ini MENIMPA database prod. Ketik 'ya' untuk lanjut: " ans
    [ "$ans" = "ya" ] || { echo "dibatalkan"; exit 1; }
    gunzip -c "$f" | "${DC[@]}" exec -T db psql -U "$DB_USER" -d "$DB_NAME" -v ON_ERROR_STOP=1 -q >/dev/null
    echo "OK: restore dari $f"
    ;;
  *) echo "pakai: $0 backup [file] | restore <file>" >&2; exit 2 ;;
esac
