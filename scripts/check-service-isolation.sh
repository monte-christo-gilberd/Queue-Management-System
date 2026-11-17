#!/usr/bin/env bash
# Memastikan notify-service (Web Push) TIDAK bergantung pada mail-service, mailpit, atau SMTP, di compose dev
# dan prod. Jaminannya: email mati tidak boleh menghentikan push. Terakhir, memastikan auth-service (login) TERPISAH: tidak ada service lain yang bergantung padanya (jadi user yang
# sudah login tetap bisa memakai aplikasi saat ia mati), ia hanya bergantung pada db, dan kunci privat JWT hanya
# di-mount ke auth-service.
# Dijalankan dari akar repo:
#   scripts/check-service-isolation.sh
set -euo pipefail
cd "$(dirname "$0")/.."

check() { # <label> <perintah compose...>
  local label="$1"; shift
  "$@" config --format json | python3 -c '
import json, sys
label = sys.argv[1]
svc = json.load(sys.stdin)["services"]["notify-service"]
deps = set((svc.get("depends_on") or {}).keys())
env = set((svc.get("environment") or {}).keys())
problems = []
bad_deps = deps & {"mail-service", "mailpit"}
if bad_deps: problems.append(f"depends_on {sorted(bad_deps)}")
bad_env = sorted(k for k in env if k.startswith("SMTP_") or k.startswith("MAIL_"))
if bad_env: problems.append(f"env {bad_env}")
if problems:
    print(f"GAGAL [{label}] notify-service bergantung pada email: " + "; ".join(problems)); sys.exit(1)
print(f"OK [{label}] notify-service tidak bergantung pada mail-service/mailpit/SMTP (depends_on={sorted(deps)})")
' "$label"
}

check_auth() { # <label> <perintah compose...>
  local label="$1"; shift
  "$@" config --format json | python3 -c '
import json, sys
label = sys.argv[1]
svcs = json.load(sys.stdin)["services"]
problems = []
auth = svcs.get("auth-service")
if not auth:
    problems.append("auth-service tidak ada di compose")
else:
    extra = set((auth.get("depends_on") or {}).keys()) - {"db"}
    if extra: problems.append(f"auth-service depends_on {sorted(extra)} (hanya db yang boleh)")
for name, svc in svcs.items():
    if name == "auth-service": continue
    if "auth-service" in (svc.get("depends_on") or {}):
        problems.append(f"{name} depends_on auth-service")
    # Kunci privat penanda token hanya boleh dipegang auth-service (folder keys/ penuh juga membukanya).
    for v in svc.get("volumes") or []:
        src = str(v.get("source", ""))
        if "auth-private" in src or src.rstrip("/").endswith("/keys"):
            problems.append(f"{name} me-mount kunci privat JWT ({src})")
    bad_env = sorted(k for k in (svc.get("environment") or {}) if k.startswith("AUTH_JWT_PRIVATE"))
    if bad_env: problems.append(f"{name} env {bad_env}")
if problems:
    print(f"GAGAL [{label}] auth-service tidak terisolasi: " + "; ".join(problems)); sys.exit(1)
print(f"OK [{label}] auth-service terisolasi: tidak ada service yang bergantung padanya, ia hanya butuh db, kunci privat hanya di auth-service")
' "$label"
}

check dev docker compose
check_auth dev docker compose
# Prod mewajibkan variabel SMTP; nilai palsu cukup untuk menguraikan compose.
export SMTP_HOST=x SMTP_PORT=587 SMTP_FROM=a@b.co MAIL_SERVICE_TOKEN=0123456789abcdef0123456789abcdef
check prod docker compose -f docker-compose.prod.yml --env-file "${QMS_ENV_FILE:-.env}"
check_auth prod docker compose -f docker-compose.prod.yml --env-file "${QMS_ENV_FILE:-.env}"

# Kode notify-service juga tidak boleh menyebut email.
if grep -rniE 'smtp|mail-service|mailer' notify-service --include=*.go --include=go.mod -l | grep -q .; then
  echo "GAGAL [kode] notify-service menyebut smtp/mail-service/mailer:"; grep -rniE 'smtp|mail-service|mailer' notify-service --include=*.go --include=go.mod -l; exit 1
fi
echo "OK [kode] kode notify-service tidak menyebut smtp/mail-service/mailer"

# Backend tidak boleh memanggil auth-service: token diverifikasi lokal dengan kunci publik. Tidak ada klien HTTP,
# URL, atau kunci privat auth-service di kode backend (komentar boleh menyebut namanya).
if grep -rnE 'AUTH_SERVICE_URL|auth-service:[0-9]+|AUTH_JWT_PRIVATE' backend --include=*.go --include=go.mod | grep -q .; then
  echo "GAGAL [kode] backend memanggil/menyebut alamat auth-service atau kunci privat:"; grep -rnE 'AUTH_SERVICE_URL|auth-service:[0-9]+|AUTH_JWT_PRIVATE' backend --include=*.go --include=go.mod; exit 1
fi
echo "OK [kode] kode backend tidak memanggil auth-service"

# nginx tidak boleh memakai upstream auth-service di luar variabel (host tak ter-resolve = nginx gagal start).
if grep -hE 'upstream[[:space:]]+auth|proxy_pass[[:space:]]+http://auth-service' deploy/nginx/*.conf | grep -vE '^[[:space:]]*#' | grep -q .; then
  echo "GAGAL [nginx] upstream auth-service harus lewat variabel (set \$qms_auth) agar nginx tetap start bila mati"; exit 1
fi
echo "OK [nginx] upstream auth-service lewat variabel + resolver"


# =============================================================================
# Observability (Fase 6): overlay docker-compose.observability.yml tidak boleh mengganggu aplikasi.
# Pemeriksaan di bawah menjaga tujuan 5 di plan: observability mati tidak boleh mengganggu antrian.
# =============================================================================

# Nama service milik overlay observability. Semua service lain dianggap service aplikasi.
OBS_SERVICES='collector loki tempo prometheus grafana cadvisor node-exporter postgres-exporter nginx-exporter'
# Overlay butuh dua variabel wajib; nilai palsu cukup untuk menguraikan compose.
export GRAFANA_ADMIN_PASSWORD=x METRICS_DB_PASSWORD=x OBS_SERVICES

# Berkas sementara untuk JSON hasil `docker compose config` (dihapus saat skrip selesai).
OBS_TMP="$(mktemp)"
trap 'rm -f "$OBS_TMP"' EXIT

# Pemeriksa overlay (Python, dibaca dari berkas sementara supaya aman dari tanda kutip shell).
OBS_PY="$(mktemp)"
trap 'rm -f "$OBS_TMP" "$OBS_PY"' EXIT
cat > "$OBS_PY" <<'PY'
import json, os, sys

label, path = sys.argv[1], sys.argv[2]
obs = set(os.environ["OBS_SERVICES"].split())
svcs = json.load(open(path))["services"]
problems = []

# 1. Tidak ada depends_on dari service aplikasi ke komponen observability (aplikasi tidak boleh menunggu observability).
for name, svc in svcs.items():
    if name in obs:
        continue
    bad = sorted(set((svc.get("depends_on") or {}).keys()) & obs)
    if bad:
        problems.append(f"{name} depends_on {bad}")

# 2. Hanya Grafana yang membuka port ke host, dan hanya di 127.0.0.1. Port internal observability (metrik 9100,
#    status nginx 8081, OTLP 4317, Prometheus 9090, Loki 3100, Tempo 3200) tidak boleh dipublikasikan service mana pun.
internal_ports = {"9100", "8081", "4317", "9090", "3100", "3200"}
for name, svc in svcs.items():
    for p in svc.get("ports") or []:
        if name == "grafana":
            if p.get("host_ip") != "127.0.0.1":
                problems.append("grafana membuka port ke " + str(p.get("host_ip") or "semua antarmuka"))
        elif name in obs:
            problems.append(f"{name} membuka port ke host ({p.get('published')})")
        elif str(p.get("target")) in internal_ports:
            problems.append(f"{name} mempublikasikan port internal observability {p.get('target')}")

# 3. Tidak ada docker.sock di service mana pun (akses penuh ke Docker host).
for name, svc in svcs.items():
    for v in svc.get("volumes") or []:
        if "docker.sock" in str(v.get("source", "")):
            problems.append(f"{name} me-mount docker.sock")

# 4. Mount direktori host oleh service observability harus read-only.
for name in obs:
    for v in (svcs.get(name) or {}).get("volumes") or []:
        src = str(v.get("source", ""))
        if v.get("type") == "bind" and src.startswith("/") and not v.get("read_only"):
            problems.append(f"{name} me-mount {src} tanpa read-only")

# 5. Setiap komponen observability punya batas memori (RAM host terbatas).
for name in obs:
    if name in svcs and not svcs[name].get("mem_limit"):
        problems.append(f"{name} tanpa mem_limit")

# 6. Pengiriman trace (OTEL_*) hanya untuk service yang dipasangi SDK, dan hanya ke collector.
sdk = {"backend", "auth-service", "mail-service"}
for name, svc in svcs.items():
    ep = (svc.get("environment") or {}).get("OTEL_EXPORTER_OTLP_ENDPOINT")
    if ep is not None and name not in sdk:
        problems.append(f"{name} memakai OTEL_EXPORTER_OTLP_ENDPOINT tanpa SDK")
    if ep and ep != "http://collector:4317":
        problems.append(f"{name} mengirim trace ke {ep}, bukan ke collector")

if problems:
    print(f"GAGAL [{label}] overlay observability mengganggu isolasi: " + "; ".join(problems))
    sys.exit(1)
print(f"OK [{label}] overlay observability: tanpa depends_on dari aplikasi, hanya Grafana (127.0.0.1) yang membuka port, "
      "tanpa docker.sock, mount host read-only, semua komponen ber-mem_limit, trace hanya ke collector")
PY

# Pemeriksa compose dasar (tanpa overlay): tidak boleh membawa jejak observability.
BASE_PY="$(mktemp)"
trap 'rm -f "$OBS_TMP" "$OBS_PY" "$BASE_PY"' EXIT
cat > "$BASE_PY" <<'PY'
import json, os, sys

label, path = sys.argv[1], sys.argv[2]
obs = set(os.environ["OBS_SERVICES"].split())
svcs = json.load(open(path))["services"]
problems = []
for name, svc in svcs.items():
    if name in obs:
        problems.append(f"{name} ada di compose dasar")
    for k in (svc.get("environment") or {}):
        if k.startswith("OTEL_"):
            problems.append(f"{name} membawa {k} tanpa overlay")
if problems:
    print(f"GAGAL [{label}] compose dasar membawa jejak observability: " + "; ".join(problems))
    sys.exit(1)
print(f"OK [{label}] compose dasar bersih dari observability (tanpa service observability dan tanpa OTEL_*)")
PY

# Jalankan pemeriksaan overlay untuk dev dan prod, dan pemeriksaan kebersihan untuk compose dasar keduanya.
docker compose -f docker-compose.yml -f docker-compose.observability.yml config --format json > "$OBS_TMP"
python3 "$OBS_PY" dev "$OBS_TMP"
docker compose -f docker-compose.prod.yml -f docker-compose.observability.yml --env-file "${QMS_ENV_FILE:-.env}" config --format json > "$OBS_TMP"
python3 "$OBS_PY" prod "$OBS_TMP"
docker compose config --format json > "$OBS_TMP"
python3 "$BASE_PY" dev "$OBS_TMP"
docker compose -f docker-compose.prod.yml --env-file "${QMS_ENV_FILE:-.env}" config --format json > "$OBS_TMP"
python3 "$BASE_PY" prod "$OBS_TMP"

# nginx tidak boleh meneruskan /metrics atau port metrik (9100); metrik tidak boleh sampai ke internet lewat nginx.
if grep -hE 'proxy_pass[^;]*:9100|location[^{]*/metrics' deploy/nginx/server.conf | grep -vE '^[[:space:]]*#' | grep -q .; then
  echo "GAGAL [nginx] server.conf meneruskan /metrics atau port 9100"; exit 1
fi
echo "OK [nginx] nginx tidak meneruskan /metrics atau :9100"

# Listener stub_status nginx (:8081) harus punya `deny all` (hanya IP privat yang diizinkan) di dev dan prod.
for f in deploy/nginx/nginx.conf deploy/nginx/nginx.prod.conf; do
  # awk: mulai dari baris `listen 8081;` sampai penutup blok server, cari `deny all;`
  if ! awk '/listen 8081;/{f=1} f&&/deny all;/{ok=1} f&&/^    }/{exit} END{exit !ok}' "$f"; then
    echo "GAGAL [nginx] listener stub_status :8081 di $f tanpa 'deny all'"; exit 1
  fi
done
echo "OK [nginx] listener stub_status :8081 membatasi akses (deny all) di dev dan prod"

# Kode aplikasi tidak boleh bergantung pada observability: tidak ada nama host collector/tempo/prometheus/loki/grafana
# yang ditulis keras di kode Go (alamat hanya datang dari OTEL_EXPORTER_OTLP_ENDPOINT yang diisi overlay).
HARD='"(http://)?(collector|tempo|prometheus|loki|grafana)(:[0-9]+)?"'
# baris komentar (diawali //) dikecualikan: komentar boleh menyebut nama host sebagai contoh
HITS="$(grep -rnE "$HARD" backend auth-service mail-service notify-service --include=*.go | grep -v _test.go | grep -vE '^[^:]*:[0-9]+:[[:space:]]*//' || true)"
if [ -n "$HITS" ]; then
  echo "GAGAL [kode] kode Go menulis alamat komponen observability secara keras:"
  echo "$HITS"; exit 1
fi
echo "OK [kode] kode Go tidak menulis alamat komponen observability secara keras"
