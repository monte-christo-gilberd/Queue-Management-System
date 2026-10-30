# Kontainer nginx reverse proxy. Build context: folder deploy/nginx/
# nginx.conf = server HTTP; server.conf = isi server bersama; https.conf dipasang opsional (lihat docker-compose.https.yml).
FROM nginx:1.27-alpine

COPY nginx.conf /etc/nginx/nginx.conf
COPY server.conf /etc/nginx/snippets/server.conf
RUN mkdir -p /etc/nginx/https.d

EXPOSE 80
