#!/usr/bin/env bash
# Memeriksa konfigurasi login Google di file env sebelum stack dijalankan. Dipanggil oleh `make up`, `make tunnel`,
# dan `make prod-up`.
#   scripts/check-google-config.sh [file-env]      # bawaan: .env
#
# Variabel yang diperiksa:
#   GOOGLE_CLIENT_IDS                       auth-service: client ID yang sah (klaim `aud`), dipisah koma
#   EXPO_PUBLIC_GOOGLE_{WEB,ANDROID,IOS}_CLIENT_ID   frontend: client ID per platform (dibakar ke bundle)
#
# Hasil:
#   semua kosong                  -> login Google mati (tombol tersembunyi, rute 404). Bukan galat.
#   konfigurasi tidak sinkron     -> GAGAL (exit 1): login pasti rusak.
#   hanya peringatan              -> exit 0
# Galat yang dicegah:
#   - EXPO_PUBLIC_* terisi tetapi GOOGLE_CLIENT_IDS kosong: tombol tampil, server menjawab 404.
#   - client ID frontend TIDAK ada di GOOGLE_CLIENT_IDS: setiap ID token ditolak server (401, `aud` tidak cocok).
set -euo pipefail
cd "$(dirname "$0")/.."
ENV_FILE="${1:-.env}"
[ -f "$ENV_FILE" ] || { echo "GAGAL: $ENV_FILE tidak ada" >&2; exit 1; }

# Nilai variabel terakhir di file, tanpa tanda kutip dan spasi di tepi.
get() { grep -E "^$1=" "$ENV_FILE" | tail -1 | cut -d= -f2- | sed -e 's/^["'\'' ]*//' -e 's/["'\'' ]*$//' || true; }

server="$(get GOOGLE_CLIENT_IDS)"
web="$(get EXPO_PUBLIC_GOOGLE_WEB_CLIENT_ID)"
android="$(get EXPO_PUBLIC_GOOGLE_ANDROID_CLIENT_ID)"
ios="$(get EXPO_PUBLIC_GOOGLE_IOS_CLIENT_ID)"

errors=0
fail() { echo "GAGAL ($ENV_FILE): $*" >&2; errors=$((errors + 1)); }
warn() { echo "PERINGATAN ($ENV_FILE): $*" >&2; }

# Tidak ada yang dikonfigurasi: fitur mati, bukan galat.
if [ -z "$server$web$android$ios" ]; then
  echo "Login Google: mati (GOOGLE_CLIENT_IDS dan EXPO_PUBLIC_GOOGLE_* kosong). Lihat documentation/docs/setup-google-login.md"
  exit 0
fi

# Daftar client ID server, satu per baris (koma dipisah, spasi dibuang).
server_list="$(printf '%s' "$server" | tr ',' '\n' | sed -e 's/^ *//' -e 's/ *$//' | grep -v '^$' || true)"

if [ -z "$server_list" ]; then
  fail "EXPO_PUBLIC_GOOGLE_* terisi tetapi GOOGLE_CLIENT_IDS kosong: tombol Google tampil, tetapi server menjawab 404. Isi GOOGLE_CLIENT_IDS dengan client ID yang sama."
fi

# Setiap client ID frontend harus ada di daftar server; kalau tidak, server menolak semua ID token (aud tidak cocok).
check_id() { # <nama variabel> <nilai>
  local name="$1" value="$2"
  [ -n "$value" ] || return 0
  if [ -n "$server_list" ] && ! printf '%s\n' "$server_list" | grep -Fxq "$value"; then
    fail "$name tidak ada di GOOGLE_CLIENT_IDS: setiap login Google akan ditolak server (401). Tambahkan client ID itu ke GOOGLE_CLIENT_IDS."
  fi
  case "$value" in
    *.apps.googleusercontent.com) ;;
    *) warn "$name tidak berakhiran .apps.googleusercontent.com; pastikan itu client ID dari Google Cloud, bukan nilai contoh." ;;
  esac
}
check_id EXPO_PUBLIC_GOOGLE_WEB_CLIENT_ID "$web"
check_id EXPO_PUBLIC_GOOGLE_ANDROID_CLIENT_ID "$android"
check_id EXPO_PUBLIC_GOOGLE_IOS_CLIENT_ID "$ios"

# Server siap tetapi frontend tidak punya client ID mana pun: tombol tersembunyi.
if [ -n "$server_list" ] && [ -z "$web$android$ios" ]; then
  warn "GOOGLE_CLIENT_IDS terisi tetapi EXPO_PUBLIC_GOOGLE_* kosong: tombol Google tidak akan tampil."
fi
# Web tanpa client ID Web: tombol tidak tampil di browser (native saja).
if [ -n "$web$android$ios" ] && [ -z "$web" ]; then
  warn "EXPO_PUBLIC_GOOGLE_WEB_CLIENT_ID kosong: tombol Google tidak akan tampil di browser (hanya di aplikasi native)."
fi

if [ "$errors" -gt 0 ]; then exit 1; fi
platforms=""
[ -n "$web" ] && platforms="web"
[ -n "$android" ] && platforms="${platforms:+$platforms, }android"
[ -n "$ios" ] && platforms="${platforms:+$platforms, }ios"
if [ -z "$platforms" ]; then
  echo "Login Google: server siap ($(printf '%s\n' "$server_list" | wc -l) client ID), tetapi frontend belum punya client ID: tombol tersembunyi."
else
  echo "Login Google: aktif ($platforms), $(printf '%s\n' "$server_list" | wc -l) client ID di server."
fi
