#!/usr/bin/env bash
# Memeriksa MAIL_MODE di env produksi sebelum stack dijalankan. Dipanggil oleh `make prod-up`.
#   MAIL_MODE=smtp     email dikirim lewat SMTP nyata ke penerima sebenarnya (bawaan bila MAIL_MODE kosong)
#   MAIL_MODE=mailpit  email ditangkap Mailpit dan TIDAK dikirim ke siapa pun (demo/uji); UI hanya 127.0.0.1
# Keluar 1 bila konfigurasi salah. Dijalankan dari akar repo: scripts/check-mail-mode.sh [file-env]
set -euo pipefail
cd "$(dirname "$0")/.."
ENV_FILE="${1:-${QMS_ENV_FILE:-.env.prod}}"
[ -f "$ENV_FILE" ] || { echo "GAGAL: $ENV_FILE tidak ada" >&2; exit 1; }

get() { grep -E "^$1=" "$ENV_FILE" | tail -1 | cut -d= -f2- | sed -e 's/^["'\'']//' -e 's/["'\'']$//' || true; }
fail() { echo "GAGAL ($ENV_FILE): $*" >&2; exit 1; }

# Variabel lingkungan MAIL_MODE (mis. dari `make prod-up MAIL_MODE=mailpit`) menimpa nilai di file.
mode="${MAIL_MODE:-$(get MAIL_MODE)}"; mode="${mode:-smtp}"
case "$mode" in
  mailpit)
    echo "PERINGATAN: MAIL_MODE=mailpit. Email ditangkap Mailpit dan TIDAK dikirim ke penerima sebenarnya."
    echo "            Buka UI lewat SSH tunnel: ssh -L 8025:127.0.0.1:8025 <user>@<host>, lalu http://localhost:8025"
    evr="${EMAIL_VERIFICATION_REQUIRED:-$(get EMAIL_VERIFICATION_REQUIRED)}"
    case "$evr" in
      false|FALSE|False|0|f|F) ;;
      *)
        echo "PERINGATAN: verifikasi email menyala. Pengguna baru TIDAK menerima tautan verifikasi dan tidak bisa bergabung/membuat antrian"
        echo "            sebelum diverifikasi. Ambil tautan dari Mailpit, atau jalankan: make verify-user EMAIL=<email> STACK=prod"
        echo "            (akun dari scripts/create-demo-accounts.sh sudah terverifikasi). Matikan lewat EMAIL_VERIFICATION_REQUIRED=false."
        ;;
    esac
    ;;
  smtp)
    host="$(get SMTP_HOST)"; port="$(get SMTP_PORT)"; from="$(get SMTP_FROM)"; tls="$(get SMTP_TLS)"; tls="${tls:-starttls}"
    for v in SMTP_HOST SMTP_PORT SMTP_FROM; do
      val="$(get $v)"
      [ -n "$val" ] || fail "MAIL_MODE=smtp tetapi $v kosong. Isi SMTP nyata, atau set MAIL_MODE=mailpit untuk demo."
      case "$val" in *GANTI*) fail "$v masih berisi nilai contoh (GANTI). Isi dengan SMTP nyata, atau set MAIL_MODE=mailpit.";; esac
    done
    case "$host" in
      mailpit) fail "SMTP_HOST=mailpit tetapi MAIL_MODE=smtp: email tidak akan terkirim. Set MAIL_MODE=mailpit atau isi SMTP nyata." ;;
    esac
    if [ "$tls" = "none" ] && [ "$host" != "localhost" ] && [ "$host" != "127.0.0.1" ]; then
      fail "SMTP_TLS=none ke host jauh ($host) mengirim email dan kata sandi tanpa enkripsi. Pakai starttls (587) atau tls (465)."
    fi
    [ -n "$(get MAIL_SERVICE_TOKEN)" ] || fail "MAIL_SERVICE_TOKEN kosong"
    echo "OK: MAIL_MODE=smtp, email dikirim lewat $host:$port ($tls)"
    ;;
  *) fail "MAIL_MODE harus 'smtp' atau 'mailpit', bukan '$mode'" ;;
esac
tok="$(get MAIL_SERVICE_TOKEN)"
case "$tok" in *GANTI*|"") fail "MAIL_SERVICE_TOKEN kosong atau masih contoh. Buat: openssl rand -hex 32" ;; esac
[ "${#tok}" -ge 32 ] || fail "MAIL_SERVICE_TOKEN kurang dari 32 karakter"
dbpw="$(get MAIL_DB_PASSWORD)"
case "$dbpw" in *GANTI*|"") fail "MAIL_DB_PASSWORD kosong atau masih contoh. Buat: openssl rand -hex 24" ;; esac
exit 0
