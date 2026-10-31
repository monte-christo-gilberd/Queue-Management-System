# Perintah manual setup QMS (tanpa `make` dan `scripts/`)

Semua perintah dijalankan dari root repo kecuali disebut lain. Domain: `qms.montechristo.my.id`, tunnel: `qms-ta`.
Jangan menempel isi `.env.prod`, kunci VAPID, token tunnel, atau `~/qms-demo-accounts.txt` ke chat atau git.

## 0. Prasyarat

```bash
docker --version && docker compose version
cloudflared --version
go version          # 1.27 (hanya untuk tes lokal)
node --version      # hanya untuk Playwright
sudo systemctl stop openvpn* 2>/dev/null; ip link show cscotun0 2>/dev/null   # VPN harus mati saat tes/demo
```

## 1. Cloudflare Tunnel (sekali saja)

```bash
cloudflared tunnel login                         # buka URL di browser, pilih zona montechristo.my.id
cloudflared tunnel create qms-ta                 # menghasilkan ~/.cloudflared/<ID>.json
cloudflared tunnel route dns qms-ta qms.montechristo.my.id
cloudflared tunnel list
```

Config `~/.cloudflared/config.yml` (ganti `<ID>`):

```yaml
tunnel: qms-ta
credentials-file: /etc/cloudflared/<ID>.json
ingress:
  - hostname: qms.montechristo.my.id
    service: http://localhost:8080
  - service: http_status:404
```

Pasang sebagai service systemd, lalu atur agar manual:

```bash
sudo mkdir -p /etc/cloudflared
sudo cp ~/.cloudflared/config.yml ~/.cloudflared/<ID>.json /etc/cloudflared/
sudo chown root:root /etc/cloudflared/* && sudo chmod 600 /etc/cloudflared/*
sudo cloudflared service install
sudo systemctl disable cloudflared               # tidak hidup otomatis saat boot
sudo systemctl stop cloudflared
```

Pemakaian harian (systemd otomatis menghentikannya saat laptop shutdown):

```bash
sudo systemctl start cloudflared                 # sebelum demo
systemctl is-active cloudflared
sudo systemctl stop cloudflared                  # setelah demo
journalctl -u cloudflared -n 30 --no-pager
```

Pengecekan:

```bash
curl -s https://qms.montechristo.my.id/health    # {"status":"ok"}
curl -sI http://qms.montechristo.my.id | head -1 # 301 ke https
```

`curl -I` ke `/health` memberi 404 karena rute hanya GET.

## 2. WAF Cloudflare (dashboard)

Domains → `montechristo.my.id` → Security → Security rules → Create rule (Custom):

- Expression: `(http.host eq "qms.montechristo.my.id" and not ip.src in {157.85.208.215 IP_KANTOR})`
- Action: Block
- Security → Settings → Bot fight mode: On

IP publik WiFi saat ini:

```bash
curl -4 https://ifconfig.me
ip -4 route        # di WiFi kantor: pastikan tidak memakai 172.28.x.x atau 172.29.x.x
```

## 3. Stack dev (opsional, port 8080 sama dengan prod)

```bash
cp .env.example .env && chmod 600 .env           # isi secret
IP=$(ip route get 1.1.1.1 | grep -oP 'src \K\S+')
QMS_PUBLIC_ORIGIN="http://$IP:8080" docker compose up -d --build --force-recreate
docker compose ps
docker compose logs -f --tail 50
docker compose restart backend qr-service notify-service   # setelah ubah kode Go
docker compose down
```

Seed dev (ditolak otomatis di DB prod):

```bash
docker compose exec -T db psql -U qms -d qms_db < db/scripts/reset_seed_dev.sql
```

## 4. Stack prod/demo

Buat `.env.prod` (sekali). Nilai rahasia dibuat baru:

```bash
cp .env.prod.example .env.prod && chmod 600 .env.prod
openssl rand -hex 32                             # jalankan per rahasia (JWT, DB, HMAC, dst.), isi manual
```

Kunci VAPID baru (isi `VAPID_PUBLIC_KEY`/`VAPID_PRIVATE_KEY`; `VAPID_SUBJECT=mailto:admin@montechristo.my.id`):

```bash
npx web-push generate-vapid-keys
```

Jalankan dan kelola:

```bash
PROD="docker compose -f docker-compose.prod.yml --env-file .env.prod"

docker compose down                              # hentikan dev dulu (port 8080)
$PROD up -d --build
$PROD ps
$PROD logs -f --tail 50
$PROD down                                       # volume db_data_prod tetap
curl -s http://localhost:8080/health
```

Tanda DB produksi (sekali, setelah akun demo dibuat; membuat seed dev ditolak):

```bash
$PROD exec -T db psql -U qms -d qms_db -c "CREATE TABLE IF NOT EXISTS _no_seed_guard (x int)"
```

## 5. Akun demo (1 owner + 4 customer)

Sandi acak ditulis ke `~/qms-demo-accounts.txt` (mode 600, di luar repo). Jangan ditampilkan.

```bash
PROD="docker compose -f docker-compose.prod.yml --env-file .env.prod"
API=http://localhost:8080/api/v1
OUT=~/qms-demo-accounts.txt
[ -e "$OUT" ] && echo "sudah ada, hapus dulu" || {
  ( umask 077; : > "$OUT" )
  pw() { openssl rand -base64 18 | tr -d '/+=' | cut -c1-16; }
  reg() { curl -s -o /dev/null -w '%{http_code}\n' -X POST -H 'Content-Type: application/json' \
          -d "{\"username\":\"$1\",\"email\":\"$2\",\"password\":\"$3\"}" $API/users/register; }

  p=$(pw); reg demo-owner demo-owner@montechristo.my.id "$p"
  $PROD exec -T db psql -U qms -d qms_db -c \
    "INSERT INTO owners (user_id, subscription_expires_at) SELECT id, now() + interval '30 days' FROM users WHERE username='demo-owner'"
  echo "OWNER     demo-owner@montechristo.my.id   $p" >> "$OUT"

  for i in 1 2 3 4; do
    p=$(pw); reg demo-user$i demo-user$i@montechristo.my.id "$p"
    echo "CUSTOMER  demo-user$i@montechristo.my.id   $p" >> "$OUT"
    sleep 1                                      # hindari rate limit login/register
  done
  chmod 600 "$OUT"
}
```

Kode HTTP harus 200 atau 201 untuk setiap pendaftaran.

## 6. Backup dan restore

```bash
PROD="docker compose -f docker-compose.prod.yml --env-file .env.prod"
mkdir -p ~/qms-backups

# backup
( umask 077; $PROD exec -T db pg_dump -U qms -d qms_db --clean --if-exists \
  | gzip > ~/qms-backups/qms-$(date +%Y%m%d-%H%M%S).sql.gz )
ls -lh ~/qms-backups | tail -3

# restore (MENIMPA database prod)
gunzip -c ~/qms-backups/<file>.sql.gz | $PROD exec -T db psql -U qms -d qms_db -v ON_ERROR_STOP=1 -q > /dev/null
```

## 7. Reset data demo (akun dipertahankan)

Backup dulu (bagian 6), lalu:

```bash
PROD="docker compose -f docker-compose.prod.yml --env-file .env.prod"
$PROD exec -T db psql -U qms -d qms_db -v ON_ERROR_STOP=1 <<'SQL'
BEGIN;
TRUNCATE TABLE push_subscriptions, ticket_notes, counters, tickets, queue_history, queue_settings, queues RESTART IDENTITY;
UPDATE owners SET subscription_expires_at = GREATEST(subscription_expires_at, now()) + interval '30 days'
  WHERE subscription_expires_at < now() + interval '30 days';
COMMIT;
SQL
$PROD exec -T db psql -U qms -d qms_db -tA -c "select 'queues='||(select count(*) from queues)||' users='||(select count(*) from users)"
```

## 8. Migrasi skema manual

Tidak ada migration runner. Pada DB yang sudah ada, ubah skema dengan `ALTER`. Contoh kolom prefix tiket:

```bash
PROD="docker compose -f docker-compose.prod.yml --env-file .env.prod"
$PROD exec -T db psql -U qms -d qms_db -c "
ALTER TABLE queues ADD COLUMN IF NOT EXISTS ticket_prefix VARCHAR(3) NOT NULL DEFAULT 'A';
ALTER TABLE queues DROP CONSTRAINT IF EXISTS queues_ticket_prefix_check;
ALTER TABLE queues ADD CONSTRAINT queues_ticket_prefix_check CHECK (ticket_prefix ~ '^[A-Z0-9]{1,3}\$');"
$PROD exec -T db psql -U qms -d qms_db -c "\d queues"
```

## 9. Tes

```bash
(cd backend && go vet ./... && go test ./...)
(cd qr-service && go vet ./... && go test ./...)
(cd notify-service && go vet ./... && go test ./...)
(cd frontend && npx tsc --noEmit)

# e2e di stack dev (jalankan seed dev dulu, bagian 3)
cd frontend
QMS_WEB_URL=http://localhost:8080 QMS_API_URL=http://localhost:8080 \
QMS_OWNER_EMAIL=owner1@test.com QMS_OWNER_PASSWORD=password npx playwright test
```

## 10. Verifikasi cepat prod

```bash
PROD="docker compose -f docker-compose.prod.yml --env-file .env.prod"
docker ps --format '{{.Names}} {{.Status}}' | grep qms-prod
curl -s http://localhost:8080/health
curl -s https://qms.montechristo.my.id/health
docker exec qms-prod-nginx-1 nginx -t
curl -s -o /dev/null -w "%{http_code}\n" -X POST -H 'Content-Type: application/json' \
  -d '{"identifier":"owner1@test.com","password":"password"}' http://localhost:8080/api/v1/users/login   # harus 401
```

Skenario kegagalan (antrian harus tetap jalan):

```bash
$PROD stop notify-service      # push mati, antrian tetap
$PROD start notify-service
$PROD stop qr-service          # QR/join mati, owner tetap bisa Next/Skip
$PROD start qr-service
```

## 11. Hari demo

```bash
# H-1
( umask 077; docker compose -f docker-compose.prod.yml --env-file .env.prod exec -T db pg_dump -U qms -d qms_db --clean --if-exists | gzip > ~/qms-backups/qms-$(date +%Y%m%d-%H%M%S).sql.gz )

# pagi
docker compose down 2>/dev/null
docker compose -f docker-compose.prod.yml --env-file .env.prod up -d --build
sudo systemctl start cloudflared
curl -s https://qms.montechristo.my.id/health

# cegah laptop tidur saat demo
systemd-inhibit --what=sleep:idle --why="demo QMS" sleep 14400 &

# selesai
sudo systemctl stop cloudflared
```

## 12. Matikan dan hentikan semua

Urutan: tunnel dulu (menutup akses publik), baru stack.

```bash
# 1. Hentikan tunnel (akses publik langsung tertutup)
sudo systemctl stop cloudflared
systemctl is-active cloudflared                  # inactive

# 2. Hentikan stack prod (data tetap di volume db_data_prod)
docker compose -f docker-compose.prod.yml --env-file .env.prod down

# 3. Hentikan stack dev bila ikut jalan
docker compose down

# 4. Hentikan penahan tidur laptop dari bagian 11
pkill -f "systemd-inhibit --what=sleep:idle" 2>/dev/null

# 5. Pastikan tidak ada yang tersisa
docker ps --format '{{.Names}}' | grep qms || echo "tidak ada kontainer qms"
pgrep -af "cloudflared tunnel" || echo "tidak ada proses cloudflared"
ss -ltn | grep ':8080' || echo "port 8080 bebas"
```

Jika ada quick tunnel liar (bukan service systemd):

```bash
pkill -f "cloudflared tunnel --url"
rm -f /tmp/qms-tunnel.pid /tmp/qms-tunnel.log
```

Menghapus data (HANYA bila memang ingin mulai dari nol, backup dulu di bagian 6):

```bash
docker compose -f docker-compose.prod.yml --env-file .env.prod down -v   # menghapus volume DB prod
```

Matikan laptop: tunnel ikut berhenti otomatis dan tidak hidup lagi saat boot (sudah `disabled`). Stack Docker dengan `restart: unless-stopped` akan hidup lagi setelah boot kecuali sudah di-`down`.
