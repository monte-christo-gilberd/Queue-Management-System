#!/usr/bin/env bash
# Membuat pasangan kunci RSA untuk token login (JWT RS256) di keys/. Idempoten: bila kunci sudah ada, tidak ada
# yang ditimpa (menimpa kunci membuat semua token login yang beredar tidak sah).
#   keys/auth-private.pem : kunci privat. HANYA di-mount ke auth-service (penerbit token).
#   keys/auth-public.pem  : kunci publik. Di-mount ke backend (verifikasi lokal) dan auth-service.
# Dijalankan dari akar repo (otomatis oleh `make prod-up`; untuk dev, `make up` dan `make tunnel` menghapus keys/
# lalu memanggil script ini, dan `make down` menghapus keys/, jadi kunci dev baru tiap sesi):
#   scripts/gen-auth-keys.sh
# Rotasi kunci: hapus kedua berkas, jalankan ulang, lalu restart backend dan auth-service. Semua user harus login ulang.
set -euo pipefail
cd "$(dirname "$0")/.."

dir="${AUTH_KEYS_DIR:-keys}"
priv="$dir/auth-private.pem"
pub="$dir/auth-public.pem"

# Kunci sudah lengkap: tidak ada yang dikerjakan.
if [ -f "$priv" ] && [ -f "$pub" ]; then
  echo "Kunci auth sudah ada di $dir/ (tidak ditimpa)."
  exit 0
fi
# Hanya salah satu yang ada berarti keadaan setengah jadi; jangan menebak mana yang benar.
if [ -f "$priv" ] || [ -f "$pub" ]; then
  echo "GAGAL: hanya salah satu kunci ada di $dir/. Hapus keduanya lalu jalankan ulang." >&2
  exit 1
fi

command -v openssl >/dev/null || { echo "openssl belum terpasang" >&2; exit 1; }
mkdir -p "$dir"
# umask 077: berkas kunci privat hanya terbaca pemilik.
( umask 077; openssl genpkey -algorithm RSA -pkeyopt rsa_keygen_bits:2048 -out "$priv" 2>/dev/null )
openssl pkey -in "$priv" -pubout -out "$pub"
# Kunci publik harus terbaca oleh user non-root di kontainer backend (UID 10001 pada image prod).
chmod 644 "$pub"
# Kunci privat dibaca user non-root di kontainer auth-service (UID 10001), jadi dibuat terbaca grup/lainnya
# di dalam folder yang hanya bisa dimasuki pemilik host.
chmod 644 "$priv"
chmod 700 "$dir"
echo "Kunci auth dibuat di $dir/. Jangan commit folder ini (sudah ada di .gitignore)."
