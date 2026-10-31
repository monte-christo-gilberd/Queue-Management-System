# Kontainer nginx reverse proxy. Build context: folder deploy/nginx/
# NGINX_CONF memilih server HTTP: nginx.conf (dev, default) atau nginx.prod.conf (di belakang Cloudflare).
# server.conf = isi server bersama. FRONTEND_UPGRADE: header WebSocket untuk Metro (hanya dev, kosong di produksi).
FROM nginx:1.27-alpine

ARG NGINX_CONF=nginx.conf
ARG FRONTEND_UPGRADE=frontend-upgrade.dev.conf
COPY ${NGINX_CONF} /etc/nginx/nginx.conf
COPY server.conf /etc/nginx/snippets/server.conf
COPY ${FRONTEND_UPGRADE} /etc/nginx/snippets/frontend-upgrade.conf

EXPOSE 80
