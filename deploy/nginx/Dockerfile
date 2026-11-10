# =============================================================================
# Dockerfile: kontainer nginx reverse proxy QMS.
# -----------------------------------------------------------------------------
# Build context: folder deploy/nginx/ (lihat docker-compose*.yml, bagian build.context).
# Dipakai oleh : docker-compose.yml (dev, nilai ARG bawaan) dan docker-compose.prod.yml
#                (prod, ARG diisi sendiri lewat build.args).
#
# ARG yang bisa diubah:
#   NGINX_CONF        [GANTI-JARINGAN] berkas konfigurasi http{} yang dipakai.
#                     Bawaan nginx.conf (dev). Prod: nginx.prod.conf.
#   FRONTEND_UPGRADE  berkas header WebSocket untuk location /. Bawaan frontend-upgrade.dev.conf.
#                     Prod: frontend-upgrade.prod.conf (kosong).
# Dua ARG ini harus dipasangkan benar: dev memakai pasangan bawaan, prod memakai pasangan prod.
#
# Yang di-COPY ke image:
#   ${NGINX_CONF}       -> /etc/nginx/nginx.conf (http{} dan server_name)
#   server.conf         -> /etc/nginx/snippets/server.conf (isi server{} bersama)
#   ${FRONTEND_UPGRADE} -> /etc/nginx/snippets/frontend-upgrade.conf
# Tujuan folder snippets: nginx.conf dan server.conf memakai path /etc/nginx/snippets/ tetap,
# sehingga nama berkas di host bisa berbeda tanpa mengubah include.
#
# [GANTI-JARINGAN] EXPOSE 80 harus sama dengan port yang di-listen nginx (listen 80 di nginx*.conf).
# Pemetaan port ke host dilakukan di docker-compose*.yml, bukan di sini.
# =============================================================================

# Image dasar: nginx versi 1.27 berbasis Alpine (ukuran kecil). Versi dikunci agar build selalu sama.
FROM nginx:1.27-alpine

# Nilai bawaan untuk ARG NGINX_CONF: konfigurasi dev. Bisa ditimpa lewat build.args.
ARG NGINX_CONF=nginx.conf
# Nilai bawaan untuk ARG FRONTEND_UPGRADE: header WebSocket dev. Prod menimpanya dengan frontend-upgrade.prod.conf.
ARG FRONTEND_UPGRADE=frontend-upgrade.dev.conf
# Menyalin konfigurasi http{} yang dipilih ke lokasi standar nginx, menimpa nginx.conf bawaan image.
COPY ${NGINX_CONF} /etc/nginx/nginx.conf
# Menyalin isi server{} bersama ke folder snippets, tempat include server.conf mencarinya.
COPY server.conf /etc/nginx/snippets/server.conf
# Menyalin berkas header WebSocket yang dipilih, dengan nama tetap frontend-upgrade.conf.
COPY ${FRONTEND_UPGRADE} /etc/nginx/snippets/frontend-upgrade.conf

# Memberi tahu Docker bahwa kontainer mendengarkan port 80. Ini dokumentasi saja: pemetaan ke host
# dilakukan di docker-compose, bukan oleh EXPOSE.
EXPOSE 80
