#!/usr/bin/env bash
# Membuat/menyetel akun penguji (owner sekaligus customer) dan mengembalikan data uji antrian di stack prod
# ke kondisi seed. Dipakai juga sebagai "restore": jalankan ulang untuk mengembalikan data uji.
#
# Setiap run membuat kata sandi acak baru untuk SEMUA akun uji:
#   - akun penguji (owner + customer), dan
#   - akun pelanggan seed (pelanggan-1 sampai pelanggan-12).
# Semua email dan kata sandi ditulis ke berkas mode 600 di luar repo (bawaan ~/qms-penguji-account.txt),
# DITIMPA setiap run. Kata sandi diterapkan sebagai hash bcrypt, dan sesi login lama semua akun itu dicabut.
#
#   scripts/seed-prod-penguji.sh          # minta konfirmasi (ketik 'ya')
#   scripts/seed-prod-penguji.sh -y       # tanpa konfirmasi
# Variabel: QMS_ENV_FILE (bawaan .env.prod), QMS_API (bawaan http://localhost:8080),
#           PENGUJI_EMAIL (bawaan penguji@montechristo.my.id), PENGUJI_USERNAME (bawaan penguji), PENGUJI_FILE.
# Isi data uji: db/scripts/seed_prod_penguji.sql. Backup database dibuat otomatis sebelum perubahan apa pun.
set -euo pipefail
cd "$(dirname "$0")/.."
ENV_FILE="${QMS_ENV_FILE:-.env.prod}"
API="${QMS_API:-http://localhost:8080}/api/v1"
EMAIL="${PENGUJI_EMAIL:-penguji@montechristo.my.id}"
USERNAME="${PENGUJI_USERNAME:-penguji}"
OUT="${PENGUJI_FILE:-$HOME/qms-penguji-account.txt}"
SQL="db/scripts/seed_prod_penguji.sql"
DC=(docker compose -f docker-compose.prod.yml --env-file "$ENV_FILE")
DB_USER="${POSTGRES_USER:-qms}"; DB_NAME="${POSTGRES_DB:-qms_db}"
PSQL=("${DC[@]}" exec -T db psql -U "$DB_USER" -d "$DB_NAME" -v ON_ERROR_STOP=1)

[[ "$EMAIL" =~ ^[A-Za-z0-9._%+-]+@[A-Za-z0-9.-]+\.[A-Za-z]{2,}$ ]] || { echo "PENGUJI_EMAIL tidak valid: $EMAIL" >&2; exit 2; }
[[ "$USERNAME" =~ ^[A-Za-z0-9_.-]{3,30}$ ]] || { echo "PENGUJI_USERNAME tidak valid (3-30 karakter: huruf, angka, _ . -)" >&2; exit 2; }
python3 -c 'import bcrypt' 2>/dev/null || { echo "modul Python 'bcrypt' belum ada (pip install bcrypt)" >&2; exit 1; }
"${DC[@]}" ps --status running db 2>/dev/null | grep -q db || { echo "stack prod tidak berjalan (make prod-up)" >&2; exit 1; }

exists=$(printf "SELECT COUNT(*) FROM users WHERE email = :'e';" | "${PSQL[@]}" -tA -v e="$EMAIL")

if [ "$exists" = "1" ]; then
  echo "Akun $EMAIL sudah ada: kata sandinya DIGANTI dengan yang baru, dan data uji diisi ulang."
fi

if [ "${1:-}" != "-y" ]; then
  read -r -p "Ganti kata sandi dan isi ulang SEMUA antrian, tiket, layar, dan riwayat akun $EMAIL? Ketik 'ya': " ans
  [ "$ans" = "ya" ] || { echo "dibatalkan"; exit 1; }
fi

echo "backup dulu..."
QMS_ENV_FILE="$ENV_FILE" scripts/backup-prod.sh backup

# Kata sandi acak 16 karakter alfanumerik (tanpa / + =), dibuat tiap run.
new_password() { openssl rand -base64 18 | tr -d '/+=' | cut -c1-16; }

# Hash bcrypt dibuat di host. Kata sandi lewat stdin, tidak lewat argumen.
bcrypt_hash() { printf '%s' "$1" | python3 -c 'import sys, bcrypt; print(bcrypt.hashpw(sys.stdin.buffer.read(), bcrypt.gensalt(12)).decode())'; }

# Menyetel kata sandi untuk satu username/email di tabel users.
set_password() { # $1 = kolom (username|email), $2 = nilai, $3 = kata sandi
  local hash; hash=$(bcrypt_hash "$3")
  printf "UPDATE users SET password_hash = :'h', password_changed_at = now() WHERE %s = :'v';" "$1" \
    | "${PSQL[@]}" -q -v h="$hash" -v v="$2"
}

pw=$(new_password)
pen_created=0
if [ "$exists" = "0" ]; then
  code=$(curl -s -o /dev/null -w '%{http_code}' -X POST -H 'Content-Type: application/json' \
    -d "{\"username\":\"$USERNAME\",\"email\":\"$EMAIL\",\"password\":\"$pw\"}" "$API/users/register")
  [ "$code" = "200" ] || [ "$code" = "201" ] || { echo "daftar akun penguji gagal (HTTP $code)" >&2; exit 1; }
  pen_created=1
  echo "Akun $EMAIL dibuat."
fi

# Isi data uji. Ini juga menghapus lalu membuat ulang akun pelanggan seed (tanpa kata sandi).
"${PSQL[@]}" -q -v tester_email="$EMAIL" -v tester_username="$USERNAME" < "$SQL"

# Akun penguji yang sudah ada: kata sandinya diganti. Akun yang baru dibuat sudah punya kata sandi dari register.
if [ "$pen_created" = "0" ]; then
  set_password email "$EMAIL" "$pw"
fi

# Pelanggan seed: setiap akun dapat kata sandi acak sendiri.
mapfile -t pelanggan < <(printf '%s' "SELECT username FROM users WHERE google_id LIKE 'seed-pelanggan-%' ORDER BY length(username), username;" | "${PSQL[@]}" -tA)
pel_lines=()
for u in "${pelanggan[@]}"; do
  ppw=$(new_password)
  set_password username "$u" "$ppw"
  pel_lines+=("$u   $ppw")
done

# Tulis semua kredensial setelah semua kata sandi berlaku, supaya berkas selalu sesuai database.
( umask 077; : > "$OUT"
  {
    printf 'Akun penguji QMS  https://qms.montechristo.my.id\n'
    printf 'Dibuat ulang: %s\n\n' "$(date '+%Y-%m-%d %H:%M:%S')"
    printf 'PENGUJI (owner + customer)\n'
    printf 'Email    %s\nUsername %s\nPassword %s\n\n' "$EMAIL" "$USERNAME" "$pw"
    printf 'PELANGGAN SEED (username dan password, login pakai username)\n'
    for l in "${pel_lines[@]}"; do printf '%s\n' "$l"; done
  } >> "$OUT" )
chmod 600 "$OUT"

echo "OK: data uji penguji diisi ulang ke kondisi seed."
echo "Kredensial semua akun uji ada di: $OUT"
