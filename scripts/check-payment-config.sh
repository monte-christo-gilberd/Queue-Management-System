#!/usr/bin/env bash
# Memeriksa konfigurasi pembayaran BorderPay di file env sebelum stack prod dijalankan. Dipanggil oleh `make prod-up`
# dan `make prod-payment-check`.
#   scripts/check-payment-config.sh [file-env]      # bawaan: $QMS_ENV_FILE, lalu .env.prod
#
# Hanya memeriksa keberadaan, panjang, dan prefix. Nilai rahasia TIDAK pernah dicetak.
#
# Hasil:
#   variabel wajib kosong atau salah format -> GAGAL (exit 1), stack tidak dinyalakan
#   kunci test (bp_test_) di prod           -> PERINGATAN (exit 0): pembayaran sungguhan tidak akan diterima
#
# Galat yang dicegah (lihat README, bagian "Pembayaran BorderPay"):
#   - payment-service crash-loop karena variabel wajib kosong (config.Load gagal).
#   - checkout 503 karena PAYMENT_SERVICE_TOKEN di backend dan payment-service tidak sama, atau terlalu pendek.
#   - webhook ditolak 401 karena BORDERPAY_WEBHOOK_TOKEN terlalu pendek atau kosong.
set -euo pipefail
cd "$(dirname "$0")/.."
ENV_FILE="${1:-${QMS_ENV_FILE:-.env.prod}}"
[ -f "$ENV_FILE" ] || { echo "GAGAL: $ENV_FILE tidak ada" >&2; exit 1; }

# Nilai variabel terakhir di file, tanpa tanda kutip dan spasi di tepi.
get() { grep -E "^$1=" "$ENV_FILE" | tail -1 | cut -d= -f2- | sed -e 's/^["'\'' ]*//' -e 's/["'\'' ]*$//' || true; }

errors=0
fail() { echo "GAGAL ($ENV_FILE): $*" >&2; errors=$((errors + 1)); }
warn() { echo "PERINGATAN ($ENV_FILE): $*" >&2; }

# Kunci API BorderPay: prefix menentukan mode (test atau live).
key="$(get BORDERPAY_API_KEY)"
case "$key" in
  "")
    fail "BORDERPAY_API_KEY kosong: checkout langganan akan membalas 503." ;;
  bp_live_*)
    echo "BorderPay: kunci LIVE (bp_live_)." ;;
  bp_test_*)
    warn "BORDERPAY_API_KEY adalah kunci TEST (bp_test_). Pembayaran sungguhan tidak akan diterima. Pakai bp_live_ untuk produksi." ;;
  *)
    fail "BORDERPAY_API_KEY harus berawalan bp_test_ atau bp_live_." ;;
esac

# Token yang wajib minimal 32 karakter dan tidak boleh berisi nilai contoh.
for name in BORDERPAY_WEBHOOK_TOKEN PAYMENT_SERVICE_TOKEN; do
  val="$(get "$name")"
  if [ -z "$val" ]; then
    fail "$name kosong."
  elif [ "${#val}" -lt 32 ]; then
    fail "$name kurang dari 32 karakter (${#val} karakter). Buat dengan: openssl rand -hex 32"
  elif [[ "$val" == *GANTI* ]]; then
    fail "$name masih berisi nilai contoh (GANTI)."
  fi
done

# Token webhook dari dashboard BorderPay berawalan bpt_. Hanya peringatan: layanan tidak memeriksa prefix ini.
wh="$(get BORDERPAY_WEBHOOK_TOKEN)"
if [ -n "$wh" ] && [[ "$wh" != bpt_* ]]; then
  warn "BORDERPAY_WEBHOOK_TOKEN biasanya berawalan bpt_ (verification token dari dashboard BorderPay). Pastikan nilainya sama dengan dashboard."
fi

# Kata sandi role database payment_rw: dipakai `08_payment_role.sh` dan payment-service.
pw="$(get PAYMENT_DB_PASSWORD)"
if [ -z "$pw" ]; then
  fail "PAYMENT_DB_PASSWORD kosong: role payment_rw tidak bisa dibuat dan payment-service tidak bisa konek ke database."
elif [[ "$pw" == *GANTI* ]]; then
  fail "PAYMENT_DB_PASSWORD masih berisi nilai contoh (GANTI)."
fi

if [ "$errors" -gt 0 ]; then
  echo "Konfigurasi pembayaran TIDAK siap: perbaiki $errors hal di atas, lalu jalankan ulang." >&2
  exit 1
fi
echo "Konfigurasi pembayaran: lengkap (nilai rahasia tidak ditampilkan)."
