#!/usr/bin/env bash
# Memastikan rahasia TIDAK bocor ke observability (tujuan 6 di docs/plan-observability-tracing.md).
#
# Cara kerja: skrip mengirim request ke stack yang sedang jalan lewat nginx, dengan rahasia PALSU yang unik
# (token layar publik, password login, JWT palsu, token verifikasi email, token reset password, dan email penuh).
# Setelah pipeline sempat memproses, skrip mencari rahasia itu di tiga tempat: log (Loki), trace (Tempo), dan label
# metrik (Prometheus). Bila satu saja ditemukan, skrip gagal (kode keluar 1).
#
# Prasyarat: stack jalan dengan overlay observability (`make up obs` atau `make obs-up`), `curl`, `openssl`, `python3`.
# Dijalankan dari akar queue-management-system/:
#   scripts/check-observability-leaks.sh
#   QMS_ENV_FILE=.env.prod scripts/check-observability-leaks.sh
# Variabel (opsional):
#   QMS_ENV_FILE  berkas env untuk membaca GRAFANA_ADMIN_PASSWORD, GRAFANA_PORT, QMS_HTTP_PORT (bawaan .env)
#   QMS_BASE_URL  alamat nginx (bawaan http://localhost:<QMS_HTTP_PORT>)
#   LEAK_EXTRA_CANARY  teks tambahan yang sengaja dicari (kontrol positif untuk menguji pemeriksa ini)
#   LEAK_ROUNDS   berapa kali tiap request dikirim (bawaan 40); lebih banyak = lebih pasti ada trace yang terekam
#                 saat sampling 10% (peluang tidak ada trace sama sekali di 40 putaran sekitar 1,5%)
#
# Efek samping: hanya request yang ditolak (404/401/400); tidak ada data yang dibuat. Menambah beberapa angka di metrik
# login gagal dan 404. Hanya Grafana (127.0.0.1) yang dipakai untuk membaca Loki, Tempo, dan Prometheus.
#
# Kode keluar: 0 bersih, 1 ada kebocoran, 2 stack observability tidak terjangkau.
set -euo pipefail
cd "$(dirname "$0")/.."

ENV_FILE="${QMS_ENV_FILE:-.env}"
[ -f "$ENV_FILE" ] || { echo "$ENV_FILE tidak ada" >&2; exit 2; }

# envval membaca satu nilai dari berkas env (tanpa tanda kutip), atau mengembalikan bawaan.
envval() { # <NAMA> <bawaan>
  local v
  v="$(grep -E "^$1=" "$ENV_FILE" | tail -1 | cut -d= -f2- | tr -d '"' | tr -d "'" || true)"
  echo "${v:-$2}"
}

GRAFANA_PW="$(envval GRAFANA_ADMIN_PASSWORD '')"
GRAFANA_URL="http://127.0.0.1:$(envval GRAFANA_PORT 3000)"
BASE="${QMS_BASE_URL:-http://localhost:$(envval QMS_HTTP_PORT 8080)}"
ROUNDS="${LEAK_ROUNDS:-40}"
[ -n "$GRAFANA_PW" ] || { echo "GRAFANA_ADMIN_PASSWORD kosong di $ENV_FILE" >&2; exit 2; }

# Grafana harus terjangkau, dan sumber data Loki, Tempo, Prometheus harus ada.
if ! curl -fsS -u "admin:$GRAFANA_PW" "$GRAFANA_URL/api/health" >/dev/null 2>&1; then
  echo "Grafana tidak terjangkau di $GRAFANA_URL: jalankan stack dengan overlay (make up obs)" >&2; exit 2
fi

# Rahasia palsu yang unik per eksekusi, supaya hasil lama di Loki/Tempo tidak tercampur.
RUN="$(openssl rand -hex 5)"
C_DISPLAY="canarydisplay${RUN}"       # token layar publik (path)
C_QUERY="canaryquery${RUN}"           # nilai query string
C_PASSWORD="canarypw${RUN}Xx9!"       # password login
C_EMAIL="canary-${RUN}@leaktest.invalid"  # email penuh
C_TOKEN="canaryverify${RUN}"          # token verifikasi email (body)
C_RESET="canaryreset${RUN}"           # token reset password (query)
C_JWT="eyJhbGciOiJSUzI1NiJ9.canaryjwt${RUN}.c2lnbmF0dXJl"  # JWT palsu di header Authorization
CANARIES=("$C_DISPLAY" "$C_QUERY" "$C_PASSWORD" "$C_EMAIL" "$C_TOKEN" "$C_RESET" "$C_JWT" "canaryjwt${RUN}")
# Kontrol positif (opsional): LEAK_EXTRA_CANARY menambah satu teks yang SENGAJA dicari. Dipakai untuk membuktikan bahwa
# pemeriksa ini benar-benar bisa mendeteksi kebocoran (mis. teks itu ditulis manual ke log sebuah container).
if [ -n "${LEAK_EXTRA_CANARY:-}" ]; then CANARIES+=("$LEAK_EXTRA_CANARY"); fi

echo "== Mengirim $ROUNDS putaran request berisi rahasia uji (run $RUN) ke $BASE"
for _ in $(seq 1 "$ROUNDS"); do
  # token layar publik di path dan query (API dan halaman)
  curl -s -o /dev/null "$BASE/api/v1/display/$C_DISPLAY?token=$C_QUERY" || true
  curl -s -o /dev/null "$BASE/display/$C_DISPLAY" || true
  # login dengan password dan email penuh di body
  curl -s -o /dev/null -X POST "$BASE/api/v1/users/login" -H 'content-type: application/json' \
    -d "{\"identifier\":\"$C_EMAIL\",\"password\":\"$C_PASSWORD\"}" || true
  # verifikasi email dengan token di body
  curl -s -o /dev/null -X POST "$BASE/api/v1/users/verify-email" -H 'content-type: application/json' \
    -d "{\"token\":\"$C_TOKEN\"}" || true
  # token reset password di query halaman
  curl -s -o /dev/null "$BASE/reset-password?token=$C_RESET" || true
  # JWT palsu di header Authorization
  curl -s -o /dev/null "$BASE/api/v1/users/me" -H "Authorization: Bearer $C_JWT" || true
  # jeda kecil supaya tidak terkena rate limit nginx (10 request/detik per IP)
  sleep 0.25
done

# Beri waktu pipeline: log (collector batch 2 detik lalu Loki), trace (batch 5 detik di aplikasi, lalu collector dan Tempo).
echo "== Menunggu pipeline observability (45 detik)"
sleep 45

# Pemeriksa (Python) dijalankan dari berkas sementara; daftar rahasia dikirim lewat variabel lingkungan.
CHECK_PY="$(mktemp)"
trap 'rm -f "$CHECK_PY"' EXIT
cat > "$CHECK_PY" <<'PY'
import base64, json, os, sys, time, urllib.parse, urllib.request

grafana, pw = os.environ["GRAFANA_URL"], os.environ["GRAFANA_PW"]
canaries = [c for c in os.environ["CANARIES"].split("\n") if c]
auth = "Basic " + base64.b64encode(f"admin:{pw}".encode()).decode()
now = int(time.time())
start = now - 1800  # jendela 30 menit cukup untuk run ini


def get(path, **params):
    # GET ke sumber data lewat proxy Grafana; mengembalikan JSON
    url = f"{grafana}/api/datasources/proxy/uid/{path}"
    if params:
        url += "?" + urllib.parse.urlencode(params)
    req = urllib.request.Request(url, headers={"Authorization": auth, "Accept": "application/json"})
    return json.load(urllib.request.urlopen(req, timeout=60))


found = []  # (tempat, rahasia)

# --- Loki: cari setiap rahasia sebagai teks bebas di semua service, dan hitung baris yang ditemukan ---
loki_lines = 0
for c in canaries:
    q = '{service=~".+"} |= "' + c.replace('"', '\\"') + '"'
    r = get("loki/loki/api/v1/query_range", query=q, start=start * 10**9, end=now * 10**9, limit=5)
    hits = sum(len(s["values"]) for s in r["data"]["result"])
    if hits:
        found.append(("log (Loki)", c))
# baris log milik run ini harus ADA (membuktikan pipeline hidup): cari route /display/:token atau login 401 baru-baru ini
r = get("loki/loki/api/v1/query_range", query='{service="nginx"} | uri=~"/(api/v1/)?display/.*"', start=start * 10**9, end=now * 10**9, limit=5)
loki_lines = sum(len(s["values"]) for s in r["data"]["result"])

# --- Tempo: ambil trace terbaru lalu cari rahasia di seluruh isi span (atribut, event, status) ---
trace_ids = set()
for q in ["{ }", '{ resource.service.name = "backend" }', '{ resource.service.name = "auth-service" }']:
    try:
        res = get("tempo/api/search", q=q, start=start, end=now, limit=500)
    except Exception:
        continue
    trace_ids.update(t["traceID"] for t in res.get("traces", []))
spans = 0
for tid in trace_ids:
    try:
        trace = get("tempo/api/traces/" + tid, start=start, end=now)
    except Exception:
        continue
    text = json.dumps(trace)
    spans += text.count('"spanId"')
    for c in canaries:
        if c in text:
            found.append(("trace (Tempo)", c))

# --- Prometheus: cari rahasia di semua label metrik http_* dan auth_* ---
series_text = json.dumps(get("prometheus/api/v1/series", **{"match[]": '{__name__=~"http_.*|auth_.*|nginx_.*"}', "start": start, "end": now}))
series_text += json.dumps(get("prometheus/api/v1/label/route/values"))
for c in canaries:
    if c in series_text:
        found.append(("metrik (Prometheus)", c))

print(f"== Diperiksa: {len(trace_ids)} trace ({spans} span), log {len(canaries)} pola, label metrik")
print(f"   baris log nginx untuk /display/ yang terlihat di Loki (bukti pipeline hidup): {loki_lines}")
if loki_lines == 0:
    print("PERINGATAN: tidak ada log nginx baru di Loki; pemeriksaan log tidak berarti. Pastikan overlay observability jalan.")
    sys.exit(3)
if found:
    print("GAGAL: rahasia uji ditemukan di observability:")
    for where, c in found:
        print(f"   - {where}: {c[:12]}...")
    sys.exit(1)
print(f"OK: tidak ada rahasia uji (token layar, password, email, JWT, token verifikasi/reset) di log, trace, maupun label metrik")
if len(trace_ids) == 0:
    print("CATATAN: belum ada trace yang terekam (sampling rendah atau tracing mati); pemeriksaan trace belum berarti.")
PY

GRAFANA_URL="$GRAFANA_URL" GRAFANA_PW="$GRAFANA_PW" CANARIES="$(printf '%s\n' "${CANARIES[@]}")" python3 "$CHECK_PY"
