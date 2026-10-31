# Kontainer nginx reverse proxy. Build context: folder deploy/nginx/
# NGINX_CONF memilih server HTTP: nginx.conf (dev, default) atau nginx.prod.conf (di belakang Cloudflare).
# server.conf = isi server bersama.
FROM nginx:1.27-alpine

ARG NGINX_CONF=nginx.conf
COPY ${NGINX_CONF} /etc/nginx/nginx.conf
COPY server.conf /etc/nginx/snippets/server.conf

EXPOSE 80
