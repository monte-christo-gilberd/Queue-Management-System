#!/usr/bin/env bash
# tune-os.sh: menampilkan (dan bila diminta menerapkan) penyetelan OS untuk uji beban dengan banyak koneksi.
#
# Tanpa argumen: HANYA mencetak nilai sekarang dan nilai yang disarankan, tanpa mengubah apa pun.
#   perf/tune-os.sh
# Dengan --apply: menerapkan lewat `sysctl -w` (butuh sudo; berlaku sampai reboot, tidak ditulis ke /etc/sysctl.d).
#   sudo perf/tune-os.sh --apply
#
# Kenapa: tiap koneksi SSE memakai satu file descriptor di penahan dan dua di nginx; satu IP sumber hanya punya sekitar
# 28 ribu port lokal ke satu tujuan pada rentang bawaan Linux (32768-60999). Setelan ini dipakai di MESIN GENERATOR dan
# MESIN TARGET uji, bukan di server produksi.
set -euo pipefail

# Nilai yang disarankan: kunci sysctl -> nilai
declare -A WANT=(
  [fs.file-max]=2097152
  [net.ipv4.ip_local_port_range]="1024 65535"
  [net.core.somaxconn]=65535
  [net.ipv4.tcp_max_syn_backlog]=65535
  [net.ipv4.tcp_tw_reuse]=1
  [net.netfilter.nf_conntrack_max]=1048576
)

apply=0
[ "${1:-}" = "--apply" ] && apply=1

printf '%-36s %-22s %s\n' "SYSCTL" "SEKARANG" "DISARANKAN"
for key in "${!WANT[@]}"; do
  cur="$(sysctl -n "$key" 2>/dev/null | tr '\t' ' ' || echo '(tidak ada)')"
  printf '%-36s %-22s %s\n' "$key" "$cur" "${WANT[$key]}"
  if [ "$apply" = 1 ]; then
    # kunci yang tidak ada (mis. nf_conntrack tanpa modul) dilewati tanpa menggagalkan skrip
    sysctl -w "$key=${WANT[$key]}" >/dev/null 2>&1 || echo "  (dilewati: $key tidak bisa diatur di mesin ini)"
  fi
done

echo
echo "ulimit -n proses ini: $(ulimit -n) (penahan SSE perlu lebih besar dari jumlah koneksi; make perf-sse mencoba menaikkannya ke 1048576)"
[ "$apply" = 1 ] || echo "Hanya menampilkan. Terapkan dengan: sudo perf/tune-os.sh --apply"
