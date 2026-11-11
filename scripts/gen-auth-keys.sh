#!/usr/bin/env bash
# Membuat pasangan kunci RSA untuk token login (JWT RS256) di keys/ (prod, bawaan) atau di folder dari AUTH_KEYS_DIR
# (dev memakai keys-dev/). Idempoten: bila kunci sudah ada, tidak ada yang ditimpa (menimpa kunci membuat semua token
# login yang beredar tidak sah).
#   <folder>/auth-private.pem : kunci privat. HANYA di-mount ke auth-service (penerbit token).
#   <folder>/auth-public.pem  : kunci publik. Di-mount ke backend (verifikasi lokal) dan auth-service.
# Dijalankan dari akar repo. Prod: otomatis oleh `make prod-up` (folder keys/). Dev: `make up` dan `make tunnel` menghapus
# keys-dev/ lalu memanggil script ini dengan AUTH_KEYS_DIR=keys-dev, dan `make down` menghapus keys-dev/, jadi kunci dev
# baru tiap sesi. keys/ tidak pernah disentuh target dev:
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
# Kunci privat dibaca user non-root di kontainer auth-service (UID 10001). Berkasnya dibuat 644 dan foldernya 700
# (hanya pemilik host yang bisa masuk); akses untuk UID 10001 diberikan lewat ACL di bawah.
chmod 644 "$priv"
chmod 700 "$dir"
# chmod 700 saja memblokir UID 10001 (user app di kontainer) sehingga auth-service crash-loop dengan "permission denied".
# Beri ACL khusus untuk UID 10001: traverse folder (x) dan baca kunci privat (r). Pemilik host tetap bisa mengakses.
# Pemilik berkas boleh mengatur ACL miliknya sendiri tanpa root.
if command -v setfacl >/dev/null; then
  setfacl -m u:10001:x "$dir"
  setfacl -m u:10001:r "$priv"
else
  echo "PERINGATAN: setfacl tidak ada; auth-service (UID 10001) tidak bisa membaca $priv. Pasang paket acl." >&2
fi
echo "Kunci auth dibuat di $dir/. Jangan commit folder ini (sudah ada di .gitignore)."
